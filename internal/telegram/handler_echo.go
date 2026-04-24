package telegram

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// handleEcho срабатывает на любое текстовое сообщение, которое не было поймано
// специализированными хендлерами (например, /start). Бот повторяет текст
// пользователя дословно и логирует и входящее сообщение, и свой ответ.
func (t *TelegramBot) handleEcho(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil || update.Message.Text == "" {
		return
	}

	chatID := update.Message.Chat.ID
	text := update.Message.Text

	t.logger.Info("incoming message",
		"chat_id", chatID,
		"user_id", update.Message.From.ID,
		"username", update.Message.From.Username,
		"text", text,
	)

	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	}); err != nil {
		t.logger.Error("send echo reply", "err", err, "chat_id", chatID)
		return
	}

	t.logger.Info("outgoing reply", "chat_id", chatID, "text", text)
}
