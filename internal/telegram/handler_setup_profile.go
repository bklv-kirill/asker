package telegram

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// handleSetupProfile — заглушка для reply-кнопки «Настроить профиль».
// Кнопка появляется у пользователя после успешной привязки номера и шлёт
// при нажатии обычное Message.Text с подписью кнопки. Реализация формы
// настройки профиля (gender/age/info) — в отдельной фазе; пока просто
// сообщаем «в разработке» и оставляем клавиатуру висеть.
func (t *TelegramBot) handleSetupProfile(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	var from *models.User = update.Message.From
	var chatID int64 = update.Message.Chat.ID
	var inText string = update.Message.Text
	var inMessageID int64 = int64(update.Message.ID)

	t.CreateNewTelegramUserIfNotExists(ctx, from)

	t.LogTelegramEvent(ctx, from, telegramEventPayload{
		Event:             eventMessageIn,
		ChatID:            chatID,
		TelegramMessageID: inMessageID,
		Text:              inText,
	})

	var replyText string = "🚧 В разработке..."
	msg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   replyText,
	})
	if err != nil {
		t.logger.Error("send setup_profile reply", "err", err, "chat_id", chatID)

		return
	}

	t.LogTelegramEvent(ctx, from, telegramEventPayload{
		Event:             eventMessageOut,
		ChatID:            chatID,
		TelegramMessageID: int64(msg.ID),
		Text:              replyText,
	})
}
