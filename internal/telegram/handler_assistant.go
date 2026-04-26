package telegram

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"

	"github.com/bklv-kirill/asker/internal/models"
	telegramUsersRepo "github.com/bklv-kirill/asker/internal/repository/telegram_users"
	usersRepo "github.com/bklv-kirill/asker/internal/repository/users"
	"github.com/bklv-kirill/asker/internal/services/ai"
)

// assistantHistoryLimit — сколько последних сообщений диалога подаём в LLM
// как контекст. 10 шагов (5 пар user/assistant) — достаточно для связности
// коротких диалогов без раздувания токенов. Когда захочется тонкой
// настройки под нагрузку — переедет в Config (env AI_HISTORY_LIMIT).
const assistantHistoryLimit = 20

// chunkMaxRunes / chunkMinRunes — границы окна для chunked-отправки
// длинных ответов LLM. Telegram режет SendMessage на 4096 UTF-16 единиц;
// мы работаем с []rune (chars), что даёт верную верхнюю границу для
// кириллицы (один char = 2 байта в UTF-8, byte-based slicing бы рвал
// символы). Нижняя граница — точка, начиная с которой ищем ближайший
// `\n` или `. `, чтобы не рвать слова и предложения.
const (
	chunkMaxRunes = 4096
	chunkMinRunes = 3500
)

// typingRefreshInterval — как часто переотправлять `sendChatAction:typing`
// пока ждём ответ LLM. Telegram держит индикатор «печатает...» 5 секунд
// после каждого вызова — обновляем чуть раньше, чтобы анимация не моргала
// при долгих запросах.
const typingRefreshInterval = 4 * time.Second

// handleAssistant — default-обработчик: на любое текстовое сообщение,
// не пойманное специализированными хендлерами, отвечает через LLM.
// Шаги:
//  1. CreateNewTelegramUserIfNotExists — страховка на случай общения без /start.
//  2. message_in — журнал входящего.
//  3. Lookup telegram_users → если user_id IS NULL, юзер не привязал номер:
//     отвечаем подсказкой «привяжи номер» с inline-кнопкой и выходим.
//     Ассистент работает только для пользователей с привязанным номером
//     (запись в users появляется только после контакта; chat_messages
//     завязан на users.id).
//  4. История диалога (chat_messages.GetLast) ДО записи текущего вопроса —
//     вариант A контракта: история без current question.
//  5. Запись текущего вопроса (chat_messages.Create, role=user).
//  6. Сборка prompt'а (история + вопрос) и вызов llm.Prompt — таймаут
//     соблюдает реализация провайдера (внутри ставит context.WithTimeout
//     по AITimeoutSec).
//  7. Запись ответа (chat_messages.Create, role=assistant). Best-effort:
//     если запись упала — отдаём ответ юзеру и логируем сбой; на следующем
//     запросе history будет короче, но UX не блокируется.
//  8. Чанкованная отправка (splitForTelegram режет >4096 рун по \n / ". "
//     в окне [3500..4096]); message_out пишется ОДИН раз с id последнего
//     отправленного сообщения и полным текстом — концептуально это один
//     логический ответ ассистента, даже если технически пришло несколько частей.
func (t *TelegramBot) handleAssistant(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	if update.Message == nil || update.Message.From == nil || update.Message.Text == "" {
		return
	}

	t.CreateNewTelegramUserIfNotExists(ctx, update.Message.From)

	var from *tgmodels.User = update.Message.From
	var chatID int64 = update.Message.Chat.ID
	var text string = update.Message.Text
	var messageID int64 = int64(update.Message.ID)

	t.logger.Info("incoming message",
		"chat_id", chatID,
		"telegram_user_id", from.ID,
		"username", from.Username,
		"text", text,
	)

	t.LogTelegramEvent(ctx, from, telegramEventPayload{
		Event:             eventMessageIn,
		ChatID:            chatID,
		TelegramMessageID: messageID,
		Text:              text,
	})

	tgUser, err := t.telegramUsers.GetByTelegramUserID(ctx, from.ID)
	if err != nil {
		// ErrNotFound тут невозможен после CreateNewTelegramUserIfNotExists;
		// реальный сбой хранилища — отвечаем пользователю и выходим.
		if !errors.Is(err, telegramUsersRepo.ErrNotFound) {
			t.logger.Error("telegram_users get on assistant", "err", err, "telegram_user_id", from.ID)
		}

		t.sendAssistantText(ctx, b, from, chatID, "❌ Не получилось обработать сообщение. Попробуй позже.", nil)

		return
	}

	if tgUser.UserID == nil {
		t.sendAssistantText(ctx, b, from, chatID, "📱 Чтобы я мог помогать, привяжи номер телефона.", attachPhoneInlineMarkup())

		return
	}

	var userID int64 = *tgUser.UserID

	user, err := t.users.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, usersRepo.ErrNotFound) {
			t.logger.Error("users get by id: not found", "telegram_user_id", from.ID, "user_id", userID)
			t.sendAssistantText(ctx, b, from, chatID, "❌ Профиль не найден. Попробуй /start.", nil)

			return
		}

		t.logger.Error("users get by id", "err", err, "user_id", userID)
		t.sendAssistantText(ctx, b, from, chatID, "❌ Не получилось загрузить профиль. Попробуй позже.", nil)

		return
	}

	history, err := t.chatMessages.GetLast(ctx, userID, assistantHistoryLimit)
	if err != nil {
		t.logger.Error("chat_messages get last", "err", err, "user_id", userID)
		t.sendAssistantText(ctx, b, from, chatID, "❌ Не получилось загрузить историю диалога. Попробуй позже.", nil)

		return
	}

	_, err = t.chatMessages.Create(ctx, models.ChatMessageCreate{
		UserID:  userID,
		Role:    models.ChatMessageRoleUser,
		Content: text,
	})
	if err != nil {
		t.logger.Error("chat_messages create user", "err", err, "user_id", userID)
		t.sendAssistantText(ctx, b, from, chatID, "❌ Не получилось сохранить сообщение. Попробуй позже.", nil)

		return
	}

	var prompt ai.Prompt = buildAssistantPrompt(user, history, text)

	var stopTyping context.CancelFunc = t.startTyping(ctx, b, chatID)
	defer stopTyping()

	llmAnswer, err := t.llm.Prompt(ctx, prompt)
	stopTyping()
	if err != nil {
		t.logger.Error("llm prompt", "err", err, "user_id", userID)
		t.sendAssistantText(ctx, b, from, chatID, "❌ Не получилось получить ответ от ассистента. Попробуй позже.", nil)

		return
	}

	_, err = t.chatMessages.Create(ctx, models.ChatMessageCreate{
		UserID:  userID,
		Role:    models.ChatMessageRoleAssistant,
		Content: llmAnswer,
	})
	if err != nil {
		// Ответ всё равно отдаём — потеря записи в истории не должна
		// блокировать UX.
		t.logger.Error("chat_messages create assistant", "err", err, "user_id", userID)
	}

	t.sendAssistantChunked(ctx, b, from, chatID, llmAnswer)
}

// sendAssistantText — короткий ответ (служебные сообщения, ошибки,
// напоминание привязать номер). Один SendMessage + один message_out.
// replyMarkup — nil или, например, attachPhoneInlineMarkup().
func (t *TelegramBot) sendAssistantText(ctx context.Context, b *bot.Bot, from *tgmodels.User, chatID int64, text string, replyMarkup any) {
	msg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ReplyMarkup: replyMarkup,
	})
	if err != nil {
		t.logger.Error("send assistant reply", "err", err, "chat_id", chatID)

		return
	}

	t.LogTelegramEvent(ctx, from, telegramEventPayload{
		Event:             eventMessageOut,
		ChatID:            chatID,
		TelegramMessageID: int64(msg.ID),
		Text:              text,
	})
}

// sendAssistantChunked отправляет ответ LLM пачками, если он длиннее
// chunkMaxRunes; каждая пачка — отдельный SendMessage. В журнале
// telegram_events пишем ОДНО событие message_out с id последнего
// отправленного сообщения и полным текстом ответа. Сообщение шлётся
// без ParseMode — system prompt запрещает LLM любую разметку, поэтому
// текст уходит «как есть» и Telegram ничего не парсит.
func (t *TelegramBot) sendAssistantChunked(ctx context.Context, b *bot.Bot, from *tgmodels.User, chatID int64, text string) {
	var chunks []string = splitForTelegram(text)
	if len(chunks) == 0 {
		return
	}

	var lastMessageID int
	for _, chunk := range chunks {
		msg, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   chunk,
		})
		if err != nil {
			t.logger.Error("send assistant chunk", "err", err, "chat_id", chatID)

			return
		}

		lastMessageID = msg.ID
	}

	t.LogTelegramEvent(ctx, from, telegramEventPayload{
		Event:             eventMessageOut,
		ChatID:            chatID,
		TelegramMessageID: int64(lastMessageID),
		Text:              text,
	})
}

// startTyping запускает фоновую горутину, которая шлёт `sendChatAction:typing`
// в чат сразу и затем каждые typingRefreshInterval, пока возвращаемый
// CancelFunc не будет вызван. Это даёт пользователю индикатор «печатает...»
// пока ассистент думает над ответом. Telegram держит анимацию ~5 секунд
// после каждого вызова — переобновляем за секунду до истечения.
//
// Возврат — context.CancelFunc; вызывающий обязан вызвать её (через defer
// или явно), иначе горутина продолжит дёргать Telegram до отмены родительского
// ctx. CancelFunc идемпотентна — двойной вызов безопасен.
//
// Ошибки SendChatAction логируются (кроме отмены контекста — это штатная
// остановка); UX-индикатор не критичен, чтобы из-за него ронять основной
// сценарий.
func (t *TelegramBot) startTyping(ctx context.Context, b *bot.Bot, chatID int64) context.CancelFunc {
	typingCtx, cancel := context.WithCancel(ctx)

	go func() {
		t.sendTypingAction(typingCtx, b, chatID)

		var ticker *time.Ticker = time.NewTicker(typingRefreshInterval)
		defer ticker.Stop()

		for {
			select {
				case <-typingCtx.Done():
					return
				case <-ticker.C:
					t.sendTypingAction(typingCtx, b, chatID)
			}
		}
	}()

	return cancel
}

// sendTypingAction шлёт один `sendChatAction:typing` и проглатывает
// context.Canceled (штатная остановка startTyping). Прочие ошибки
// логируются — но индикатор не критичен, ничего больше не делаем.
func (t *TelegramBot) sendTypingAction(ctx context.Context, b *bot.Bot, chatID int64) {
	_, err := b.SendChatAction(ctx, &bot.SendChatActionParams{
		ChatID: chatID,
		Action: tgmodels.ChatActionTyping,
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		t.logger.Error("send chat action typing", "err", err, "chat_id", chatID)
	}
}

// buildAssistantPrompt сериализует профиль пользователя + историю диалога
// + текущий вопрос в один Prompt: контракт ai.LLM.Prompt принимает одну
// строку, поэтому всю структуру передаём через форматирование. System
// prompt передаётся провайдером отдельно (характеристика инстанса LLM,
// см. NewOpenRouter). Когда контракт LLM расширим до messages-array,
// этот хелпер заменится на сборку структуры — сейчас сознательное упрощение.
//
// Профиль пользователя кладётся секцией «Информация о пользователе»
// перед историей, чтобы ассистент мог персонализировать ответ (имя,
// возраст, пол, «о себе»). Опциональные поля выводятся только если
// заполнены. Phone включается — пользователь общается со своим ботом,
// но в логи бот этот prompt намеренно не пишет.
func buildAssistantPrompt(user models.User, history []models.ChatMessage, question string) ai.Prompt {
	var b strings.Builder

	b.WriteString("Информация о пользователе:\n")
	if user.Name != nil {
		b.WriteString("- Имя: ")
		b.WriteString(*user.Name)
		b.WriteString("\n")
	}
	b.WriteString("- Телефон: ")
	b.WriteString(user.Phone)
	b.WriteString("\n")
	if user.Gender != nil {
		b.WriteString("- Пол: ")
		b.WriteString(string(*user.Gender))
		b.WriteString("\n")
	}
	if user.Age != nil {
		b.WriteString("- Возраст: ")
		b.WriteString(strconv.Itoa(*user.Age))
		b.WriteString("\n")
	}
	if user.Info != nil {
		b.WriteString("- О себе: ")
		b.WriteString(*user.Info)
		b.WriteString("\n")
	}
	b.WriteString("\n")

	if len(history) > 0 {
		b.WriteString("История диалога:\n\n")

		for _, m := range history {
			var role string
			switch m.Role {
				case models.ChatMessageRoleUser:
					role = "Пользователь"
				case models.ChatMessageRoleAssistant:
					role = "Ассистент"
			}

			b.WriteString(role)
			b.WriteString(": ")
			b.WriteString(m.Content)
			b.WriteString("\n\n")
		}
	}

	b.WriteString("Текущий вопрос пользователя:\n")
	b.WriteString(question)

	return ai.Prompt(b.String())
}

// splitForTelegram режет text на пачки <= chunkMaxRunes рун. В окне
// [chunkMinRunes..chunkMaxRunes] ищем последний `\n`, затем `. ` —
// режем по нему, чтобы не рвать слова и предложения. Если ни одного
// маркера в окне нет — режем жёстко по chunkMaxRunes (для человеческого
// текста крайне редкий случай). Работаем с []rune, не []byte: для
// кириллицы один char = 2 байта в UTF-8, byte-based slicing рвал бы
// символы.
func splitForTelegram(text string) []string {
	var runes []rune = []rune(text)
	if len(runes) <= chunkMaxRunes {
		return []string{text}
	}

	var chunks []string
	var rest []rune = runes

	for len(rest) > chunkMaxRunes {
		var window []rune = rest[:chunkMaxRunes]
		var cutAt int = -1

		for i := len(window) - 1; i >= chunkMinRunes; i-- {
			if window[i] == '\n' {
				cutAt = i + 1

				break
			}
		}

		if cutAt == -1 {
			for i := len(window) - 2; i >= chunkMinRunes; i-- {
				if window[i] == '.' && window[i+1] == ' ' {
					cutAt = i + 2

					break
				}
			}
		}

		if cutAt == -1 {
			cutAt = chunkMaxRunes
		}

		chunks = append(chunks, string(rest[:cutAt]))
		rest = rest[cutAt:]
	}

	if len(rest) > 0 {
		chunks = append(chunks, string(rest))
	}

	return chunks
}
