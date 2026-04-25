package telegram

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// handleProfileField — заглушка для оставшихся inline-кнопок меню
// «Настроить профиль» (profile_set_age и profile_set_info). Каждая
// регистрируется отдельным RegisterHandler с MatchTypeExact; gender
// уже имеет свой реальный хендлер handleProfileSetGender и сюда не
// приходит. Когда у age/info появится реальная логика — заведём
// отдельные хендлеры по аналогии.
//
// Шаги: AnswerCallbackQuery (убирает спиннер), журнал callback_in
// (Text = query.Data, чтобы в БД было видно, какое поле жали),
// SendMessage с заглушкой, журнал message_out.
func (t *TelegramBot) handleProfileField(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}

	var query *models.CallbackQuery = update.CallbackQuery
	var from *models.User = &query.From

	var chatID int64
	var sourceMessageID int
	if query.Message.Message != nil {
		chatID = query.Message.Message.Chat.ID
		sourceMessageID = query.Message.Message.ID
	} else {
		t.logger.Error("profile_field callback without accessible message", "telegram_user_id", from.ID, "data", query.Data)

		return
	}

	t.CreateNewTelegramUserIfNotExists(ctx, from)

	t.LogTelegramEvent(ctx, from, telegramEventPayload{
		Event:             eventCallbackIn,
		ChatID:            chatID,
		TelegramMessageID: int64(sourceMessageID),
		Text:              query.Data,
	})

	_, err := b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: query.ID,
	})
	if err != nil {
		t.logger.Error("answer profile_field callback", "err", err, "telegram_user_id", from.ID, "data", query.Data)
	}

	var replyText string = "🚧 В разработке..."
	msg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   replyText,
	})
	if err != nil {
		t.logger.Error("send profile_field reply", "err", err, "chat_id", chatID, "data", query.Data)

		return
	}

	t.LogTelegramEvent(ctx, from, telegramEventPayload{
		Event:             eventMessageOut,
		ChatID:            chatID,
		TelegramMessageID: int64(msg.ID),
		Text:              replyText,
	})
}
