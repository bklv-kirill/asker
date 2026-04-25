package telegram

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// handleEcho срабатывает на любое текстовое сообщение, которое не было поймано
// специализированными хендлерами (например, /start). Бот повторяет текст
// пользователя дословно и фиксирует в журнале telegram_events входящее
// сообщение + исходящий ответ.
func (t *TelegramBot) handleEcho(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil || update.Message.Text == "" {
		return
	}

	// Пользователь мог начать общение без /start — всё равно фиксируем TG-аккаунт.
	// Метод идемпотентен: если запись уже есть, это no-op.
	t.CreateNewTelegramUserIfNotExists(ctx, update.Message.From)

	var from *models.User = update.Message.From
	var chatID int64 = update.Message.Chat.ID
	var text string = update.Message.Text
	var inMessageID int64 = int64(update.Message.ID)

	t.logger.Info("incoming message",
		"chat_id", chatID,
		"telegram_user_id", from.ID,
		"username", from.Username,
		"text", text,
	)

	t.LogTelegramEvent(ctx, from, telegramEventPayload{
		Event:             eventMessageIn,
		ChatID:            chatID,
		TelegramMessageID: inMessageID,
		Text:              text,
	})

	msg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	})
	if err != nil {
		t.logger.Error("send echo reply", "err", err, "chat_id", chatID)

		return
	}

	t.LogTelegramEvent(ctx, from, telegramEventPayload{
		Event:             eventMessageOut,
		ChatID:            chatID,
		TelegramMessageID: int64(msg.ID),
		Text:              text,
	})
}
