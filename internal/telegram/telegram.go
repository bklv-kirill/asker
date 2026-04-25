// Package telegram инкапсулирует жизненный цикл Telegram-бота: инициализацию клиента,
// регистрацию обработчиков команд и запуск long-polling.
//
// Соглашение: каждый обработчик команды живёт в отдельном файле handler_<name>.go
// (например, handler_start.go) как приватный метод *TelegramBot. Регистрация всех
// обработчиков выполняется в Start. В telegram.go, помимо типа/конструктора/Start,
// живут доменные методы *TelegramBot, переиспользуемые несколькими хендлерами
// (например, CreateNewTelegramUserIfNotExists).
package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	telegramEventsRepo "github.com/bklv-kirill/asker/internal/repository/telegram_events"
	telegramUsersRepo "github.com/bklv-kirill/asker/internal/repository/telegram_users"
)

var ErrInitBot = errors.New("telegram: init bot")

// Типы событий журнала telegram_events. Команда логируется одним событием
// типа eventCommandIn (а не парой command_in + message_in).
const (
	eventCommandIn  = "command_in"
	eventMessageIn  = "message_in"
	eventMessageOut = "message_out"
)

// telegramEventPayload — единая плоская структура payload для журнала
// telegram_events. Опциональные поля помечены omitempty, чтобы в JSON
// попадали только реально заполненные. Тип события определяется полем Event;
// сама строка команды (например, "/start") живёт в Text — отдельное поле
// Command не нужно, оно дублировало бы Text при текущем наборе команд.
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

	telegramUsers  telegramUsersRepo.Repository
	telegramEvents telegramEventsRepo.Repository
}

func NewTelegramBot(
	token, botName string,

	logger *slog.Logger,

	telegramUsers telegramUsersRepo.Repository,
	telegramEvents telegramEventsRepo.Repository,
) *TelegramBot {
	return &TelegramBot{
		token:   token,
		botName: botName,

		logger: logger,

		telegramUsers:  telegramUsers,
		telegramEvents: telegramEvents,
	}
}

// Start инициализирует клиента Bot API, регистрирует обработчики и запускает
// long-polling. Блокирует вызывающего, пока ctx не будет отменён.
func (t *TelegramBot) Start(ctx context.Context) error {
	b, err := bot.New(t.token, bot.WithDefaultHandler(t.handleEcho))
	if err != nil {
		return errors.Join(ErrInitBot, err)
	}

	b.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeExact, t.handleStart)

	b.Start(ctx)

	return nil
}

// CreateNewTelegramUserIfNotExists сохраняет запись о TG-аккаунте в telegram_users,
// если её там ещё нет. Идемпотентен: повторный вызов для того же from.ID — no-op.
// Ошибки хранилища логируются и наружу не пробрасываются: вызывающий сценарий
// (хендлер) не должен блокироваться, если БД временно недоступна.
func (t *TelegramBot) CreateNewTelegramUserIfNotExists(ctx context.Context, from *models.User) {
	exists, err := t.telegramUsers.ExistsByTelegramUserID(ctx, from.ID)
	if err != nil {
		t.logger.Error("telegram_users exists check", "err", err, "telegram_user_id", from.ID)

		return
	} else if exists {
		return
	}

	var lastName *string = optionalString(from.LastName)
	var username *string = optionalString(from.Username)

	id, err := t.telegramUsers.Create(ctx, from.ID, from.FirstName, lastName, username)
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
func (t *TelegramBot) LogTelegramEvent(ctx context.Context, from *models.User, payload telegramEventPayload) {
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

	id, err := t.telegramEvents.Create(ctx, user.ID, raw)
	if err != nil {
		t.logger.Error("telegram_events create", "err", err, "telegram_user_id", from.ID, "event", payload.Event)

		return
	}

	t.logger.Info("telegram_events created",
		"id", id,
		"event", payload.Event,
		"telegram_user_id", from.ID,
	)
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
