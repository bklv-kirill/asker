package telegram

import (
	"context"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
)

// handleSetupProfile срабатывает на нажатие persistent reply-кнопки
// «Настроить профиль» (Message.Text совпадает с setupProfileButtonText).
// Шлёт сообщение-меню с inline-кнопками выбора поля для редактирования
// (пол / возраст / о себе). Сами действия за каждой кнопкой пока заглушка —
// см. handler_profile_field.go.
func (t *TelegramBot) handleSetupProfile(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	var from *tgmodels.User = update.Message.From
	var chatID int64 = update.Message.Chat.ID
	var text string = update.Message.Text
	var messageID int64 = int64(update.Message.ID)

	t.clearPendingInput(from.ID)
	t.dropUserDebounce(from.ID)

	t.CreateNewTelegramUserIfNotExists(ctx, from)

	t.LogTelegramEvent(ctx, from, telegramEventPayload{
		Event:             eventMessageIn,
		ChatID:            chatID,
		TelegramMessageID: messageID,
		Text:              text,
	})

	var replyText string = "⚙️ Что хочешь настроить?"
	msg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        replyText,
		ReplyMarkup: profileFieldsInlineMarkup(),
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
