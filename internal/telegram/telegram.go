// Package telegram инкапсулирует жизненный цикл Telegram-бота: инициализацию клиента,
// регистрацию обработчиков команд и запуск long-polling.
//
// Соглашение: каждый обработчик команды живёт в отдельном файле handler_<name>.go
// (например, handler_start.go) как приватный метод *TelegramBot. Регистрация всех
// обработчиков выполняется в Start. В telegram.go, помимо типа/конструктора/Start,
// живут доменные методы *TelegramBot, переиспользуемые несколькими хендлерами
// (например, CreateNewTelegramUserIfNotExists, LogTelegramEvent), а также
// общие константы (типы событий журнала, callback-data) и хелперы.
package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"

	"github.com/bklv-kirill/asker/internal/models"
	chatMessagesRepo "github.com/bklv-kirill/asker/internal/repository/chat_messages"
	telegramEventsRepo "github.com/bklv-kirill/asker/internal/repository/telegram_events"
	telegramUsersRepo "github.com/bklv-kirill/asker/internal/repository/telegram_users"
	usersRepo "github.com/bklv-kirill/asker/internal/repository/users"
	"github.com/bklv-kirill/asker/internal/services/ai"
	"github.com/bklv-kirill/asker/internal/services/stt"
)

var ErrInitBot = errors.New("telegram: init bot")

// Типы событий журнала telegram_events. Команда логируется одним событием
// типа eventCommandIn (а не парой command_in + message_in). Для contact_in
// в поле Text payload'а пишется сырой Contact.PhoneNumber от TG. Для voice_in
// в Text — расшифровка от STT-провайдера; событие пишется только при
// успешной транскрибации (см. handler_voice.go).
const (
	eventCommandIn  = "command_in"
	eventMessageIn  = "message_in"
	eventMessageOut = "message_out"
	eventCallbackIn = "callback_in"
	eventContactIn  = "contact_in"
	eventVoiceIn    = "voice_in"
)

// setupProfileButtonText / myProfileButtonText — подписи reply-кнопок
// клавиатуры профиля (показывается уже-привязанным юзерам). Эти же значения
// приходят как Message.Text при нажатии — ловим через
// RegisterHandler(HandlerTypeMessageText, MatchTypeExact). Кнопки лежат
// в profileSettingsKeyboard, по одной в строке для удобного тапа.
const (
	setupProfileButtonText = "⚙️ Настроить профиль"
	myProfileButtonText    = "👤 Мой профиль"
)

// attachPhoneCallbackData — значение callback_data для inline-кнопки
// «Привязать номер». При нажатии TG присылает CallbackQuery с этим data —
// ловим через RegisterHandler(HandlerTypeCallbackQueryData, MatchTypeExact).
const attachPhoneCallbackData = "attach_phone"

// callback_data inline-кнопок меню «Настроить профиль». Каждая
// регистрируется отдельным RegisterHandler с MatchTypeExact: gender
// открывает второе меню (выбор пола), age и info — каждое свой
// pending-state flow с текстовым вводом.
const (
	profileSetGenderCallback = "profile_set_gender"
	profileSetAgeCallback    = "profile_set_age"
	profileSetInfoCallback   = "profile_set_info"
)

// callback_data inline-кнопок выбора пола (вторая ступень меню,
// открываемая по profile_set_gender). Регистрируются одним
// RegisterHandler через MatchTypePrefix profileGenderCallbackPrefix —
// общий хендлер handleProfileGender различает значение по query.Data.
const profileGenderCallbackPrefix = "profile_gender_"

const (
	profileGenderMaleCallback   = "profile_gender_male"
	profileGenderFemaleCallback = "profile_gender_female"
)

// pendingInput* — значения in-memory state «жду текстовый ответ от юзера X».
// Хранится в TelegramBot.pendingInput; matchFunc matchPendingInput ловит
// сообщения от юзеров, у которых state выставлен. Значения namespace'нуты
// префиксом домена (`profile.*` для полей профиля), чтобы при появлении
// нового сценария (опрос, модерация, etc.) state не пересекался по семантике.
const (
	pendingInputProfileAge  = "profile.age"
	pendingInputProfileInfo = "profile.info"
)

// telegramEventPayload — единая плоская структура payload для журнала
// telegram_events. Опциональные поля помечены omitempty, чтобы в JSON
// попадали только реально заполненные. Тип события определяется полем Event;
// сама строка команды (например, "/start"), эхо-текст или сырой номер
// контакта (Contact.PhoneNumber) — всё кладётся в Text.
//
// Структура приватная: схема событий — деталь продьюсера (этого пакета),
// репозиторий хранит сырой JSON. Если позже понадобится разбирать payload
// в других слоях — переедет в internal/models.
type telegramEventPayload struct {
	Event             string `json:"event"`
	ChatID            int64  `json:"chat_id"`
	TelegramMessageID int64  `json:"telegram_message_id,omitempty"`
	Text              string `json:"text,omitempty"`
}

type TelegramBot struct {
	token   string
	botName string

	logger *slog.Logger

	users          usersRepo.Repository
	telegramUsers  telegramUsersRepo.Repository
	telegramEvents telegramEventsRepo.Repository
	chatMessages   chatMessagesRepo.Repository

	llm ai.LLM
	stt stt.STT

	// sttMaxDurationSec — лимит длительности голосового сообщения (в
	// секундах), длиннее которого STT-провайдер не вызывается: бот отвечает
	// просьбой сократить или написать текстом. Берётся из STT_MAX_DURATION_SEC,
	// инжектится конструктором как обычное доменное значение (внутренние
	// пакеты не зависят от *config.Config).
	sttMaxDurationSec int

	// pendingInput — in-memory state «жду текстовый ответ от юзера X
	// на сценарий Y». Ключ — telegram_user_id (from.ID), значение —
	// одна из pendingInput* констант (с namespace-префиксом домена,
	// напр. `profile.age`). Один state per юзер: если юзер был в
	// одном flow и ушёл в другой, старое значение затирается чисткой
	// в начале целевого хендлера. State теряется при рестарте бота —
	// для текущего масштаба приемлемо.
	pendingMu    sync.RWMutex
	pendingInput map[int64]string
}

func NewTelegramBot(
	token, botName string,

	logger *slog.Logger,

	users usersRepo.Repository,
	telegramUsers telegramUsersRepo.Repository,
	telegramEvents telegramEventsRepo.Repository,
	chatMessages chatMessagesRepo.Repository,

	llm ai.LLM,
	speechToText stt.STT,

	sttMaxDurationSec int,
) *TelegramBot {
	return &TelegramBot{
		token:   token,
		botName: botName,

		logger: logger,

		users:          users,
		telegramUsers:  telegramUsers,
		telegramEvents: telegramEvents,
		chatMessages:   chatMessages,

		llm: llm,
		stt: speechToText,

		sttMaxDurationSec: sttMaxDurationSec,

		pendingInput: make(map[int64]string),
	}
}

// setPendingInput помечает, что от юзера ждём текстовый ответ на сценарий
// kind (например, pendingInputProfileAge). matchPendingInput по этому state
// маршрутизирует следующее текстовое сообщение в обработчик ввода.
func (t *TelegramBot) setPendingInput(telegramUserID int64, kind string) {
	t.pendingMu.Lock()
	defer t.pendingMu.Unlock()

	t.pendingInput[telegramUserID] = kind
}

// getPendingInput возвращает kind, на который ждём ответ, и флаг присутствия.
// Используется в matchPendingInput и в общем диспетчере handlePendingInput.
func (t *TelegramBot) getPendingInput(telegramUserID int64) (string, bool) {
	t.pendingMu.RLock()
	defer t.pendingMu.RUnlock()

	kind, ok := t.pendingInput[telegramUserID]

	return kind, ok
}

// clearPendingInput сбрасывает state. Вызывается всеми exact-match
// и prefix-match хендлерами в начале — это «отмена» ожидания: юзер пошёл
// в другой сценарий (/start, кнопка меню и т.п.).
func (t *TelegramBot) clearPendingInput(telegramUserID int64) {
	t.pendingMu.Lock()
	defer t.pendingMu.Unlock()

	delete(t.pendingInput, telegramUserID)
}

// Start инициализирует клиента Bot API, регистрирует обработчики и запускает
// long-polling. Блокирует вызывающего, пока ctx не будет отменён.
func (t *TelegramBot) Start(ctx context.Context) error {
	b, err := bot.New(t.token, bot.WithDefaultHandler(t.handleAssistant))
	if err != nil {
		return errors.Join(ErrInitBot, err)
	}

	b.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeExact, t.handleStart)

	b.RegisterHandler(bot.HandlerTypeMessageText, setupProfileButtonText, bot.MatchTypeExact, t.handleSetupProfile)
	b.RegisterHandler(bot.HandlerTypeMessageText, myProfileButtonText, bot.MatchTypeExact, t.handleMyProfile)

	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, attachPhoneCallbackData, bot.MatchTypeExact, t.handleAttachPhone)

	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, profileSetGenderCallback, bot.MatchTypeExact, t.handleProfileSetGender)
	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, profileGenderCallbackPrefix, bot.MatchTypePrefix, t.handleProfileGender)

	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, profileSetAgeCallback, bot.MatchTypeExact, t.handleProfileSetAge)
	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, profileSetInfoCallback, bot.MatchTypeExact, t.handleProfileSetInfo)

	b.RegisterHandlerMatchFunc(t.matchPendingInput, t.handlePendingInput)

	b.RegisterHandlerMatchFunc(matchMessageContact, t.handleContact)

	b.RegisterHandlerMatchFunc(matchMessageVoice, t.handleVoice)

	b.Start(ctx)

	return nil
}

// matchMessageContact срабатывает на любое сообщение, в котором есть
// поле Contact (пользователь поделился номером через кнопку
// request_contact). Default-хендлер (handleEcho) на такие апдейты не
// сработает — RegisterHandlerMatchFunc проверяется раньше дефолтного.
func matchMessageContact(update *tgmodels.Update) bool {
	return update.Message != nil && update.Message.Contact != nil
}

// matchMessageVoice срабатывает на любое сообщение с голосовой записью
// (Telegram voice — записанное в самом клиенте, MIME audio/ogg, кодек
// opus). Audio-файлы (загруженные из галереи), video/video_note и
// прочие медиа намеренно НЕ ловим — это отдельные сценарии, и точка
// принятия решения должна быть явной.
func matchMessageVoice(update *tgmodels.Update) bool {
	return update.Message != nil && update.Message.Voice != nil
}

// matchPendingInput срабатывает на любое текстовое сообщение от юзера,
// для которого установлен pending state (например, ждём profile.age).
// Регистрируется ПОСЛЕ всех exact-match хендлеров, так что /start,
// «⚙️ Настроить профиль» и т.п. имеют приоритет — они сами чистят state
// в начале и идут по своему сценарию.
func (t *TelegramBot) matchPendingInput(update *tgmodels.Update) bool {
	if update.Message == nil || update.Message.From == nil || update.Message.Text == "" {
		return false
	}

	_, ok := t.getPendingInput(update.Message.From.ID)

	return ok
}

// CreateNewTelegramUserIfNotExists сохраняет запись о TG-аккаунте в telegram_users,
// если её там ещё нет. Идемпотентен: повторный вызов для того же from.ID — no-op.
// Ошибки хранилища логируются и наружу не пробрасываются: вызывающий сценарий
// (хендлер) не должен блокироваться, если БД временно недоступна.
func (t *TelegramBot) CreateNewTelegramUserIfNotExists(ctx context.Context, from *tgmodels.User) {
	exists, err := t.telegramUsers.ExistsByTelegramUserID(ctx, from.ID)
	if err != nil {
		t.logger.Error("telegram_users exists check", "err", err, "telegram_user_id", from.ID)

		return
	} else if exists {
		return
	}

	var lastName *string = optionalString(from.LastName)
	var username *string = optionalString(from.Username)

	id, err := t.telegramUsers.Create(ctx, models.TelegramUserCreate{
		TelegramUserID: from.ID,
		FirstName:      from.FirstName,
		LastName:       lastName,
		Username:       username,
	})
	if err != nil {
		t.logger.Error("telegram_users create", "err", err, "telegram_user_id", from.ID)

		return
	}

	t.logger.Info("telegram_users created",
		"id", id,
		"telegram_user_id", from.ID,
		"first_name", from.FirstName,
		"last_name", from.LastName,
		"username", from.Username,
	)
}

// LogTelegramEvent сохраняет одно событие журнала telegram_events для
// указанного TG-пользователя. Делает lookup внутреннего id через
// telegramUsersRepo.GetByTelegramUserID — два запроса к БД на событие
// (lookup + insert); кэш id'шников добавим отдельным шагом, когда увидим
// нагрузку. Ошибки хранилища и маршалинга логируются и наружу не
// пробрасываются — хендлеры не должны блокироваться из-за сбоя журнала.
func (t *TelegramBot) LogTelegramEvent(ctx context.Context, from *tgmodels.User, payload telegramEventPayload) {
	user, err := t.telegramUsers.GetByTelegramUserID(ctx, from.ID)
	if err != nil {
		t.logger.Error("telegram_events lookup user", "err", err, "telegram_user_id", from.ID, "event", payload.Event)

		return
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.logger.Error("telegram_events marshal payload", "err", err, "telegram_user_id", from.ID, "event", payload.Event)

		return
	}

	id, err := t.telegramEvents.Create(ctx, models.TelegramEventCreate{
		TelegramUserID: user.ID,
		Payload:        raw,
	})
	if err != nil {
		t.logger.Error("telegram_events create", "err", err, "telegram_user_id", from.ID, "event", payload.Event)

		return
	}

	t.logger.Info("telegram_events created",
		"id", id,
		"telegram_user_id", from.ID,
		"event", payload.Event,
	)
}

// profileSettingsKeyboard собирает persistent reply-клавиатуру с двумя
// кнопками — «Настроить профиль» и «Мой профиль» (по одной в строке для
// удобного тапа на узких экранах). Используется для уже-привязанных
// юзеров — после первичной привязки номера и при /start, чтобы клавиатура
// не исчезала. IsPersistent=true просит TG не сворачивать её,
// ResizeKeyboard уменьшает высоту.
func profileSettingsKeyboard() tgmodels.ReplyKeyboardMarkup {
	return tgmodels.ReplyKeyboardMarkup{
		Keyboard: [][]tgmodels.KeyboardButton{
			{
				{Text: setupProfileButtonText},
			},
			{
				{Text: myProfileButtonText},
			},
		},
		IsPersistent:   true,
		ResizeKeyboard: true,
	}
}

// attachPhoneInlineMarkup собирает inline-клавиатуру с одной кнопкой
// «Привязать номер». При нажатии TG присылает CallbackQuery с
// data = attachPhoneCallbackData — его ловит handleAttachPhone.
func attachPhoneInlineMarkup() tgmodels.InlineKeyboardMarkup {
	return tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: "📱 Привязать номер", CallbackData: attachPhoneCallbackData},
			},
		},
	}
}

// profileFieldsInlineMarkup собирает inline-клавиатуру меню «Настроить
// профиль» — три кнопки на каждое поле доменного users (gender / age /
// info), каждая в своей строке для лучшего UX.
func profileFieldsInlineMarkup() tgmodels.InlineKeyboardMarkup {
	return tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: "👤 Указать пол", CallbackData: profileSetGenderCallback},
			},
			{
				{Text: "🎂 Указать возраст", CallbackData: profileSetAgeCallback},
			},
			{
				{Text: "✏️ Рассказать о себе", CallbackData: profileSetInfoCallback},
			},
		},
	}
}

// profileGenderInlineMarkup — вторая ступень меню «Настроить профиль» под
// «Указать пол»: две кнопки в одну строку для компактности. Подписи
// согласованы со значениями users.gender (CHECK 'мужчина'/'женщина'),
// чтобы юзеру не казалось, что варианты не совпадают с тем, что попадёт
// в БД. callback_data — отдельный префикс profile_gender_, чтобы общий
// handleProfileGender различил их.
func profileGenderInlineMarkup() tgmodels.InlineKeyboardMarkup {
	return tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: "👨 Мужчина", CallbackData: profileGenderMaleCallback},
				{Text: "👩 Женщина", CallbackData: profileGenderFemaleCallback},
			},
		},
	}
}

// optionalString отличает «TG не прислал поле» от «прислал, но пусто»: в
// tgmodels.User опциональные поля приходят как пустая строка — маппим её в nil,
// чтобы в БД легла NULL, а не "".
func optionalString(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}

// normalizePhone оставляет в строке только цифры. Contact.PhoneNumber от
// Telegram обычно приходит в формате "79991234567", но иногда содержит
// "+", пробелы или скобки — приводим к виду, который пройдёт CHECK на
// users.phone (length > 0 AND NOT GLOB '*[^0-9]*').
func normalizePhone(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}

		return -1
	}, s)
}
