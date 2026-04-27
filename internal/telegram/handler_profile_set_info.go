package telegram

import (
	"context"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
)

// handleProfileSetInfo — обработчик inline-кнопки «✏️ Рассказать о себе»
// из меню «Настроить профиль» (callback_data = profileSetInfoCallback).
// Шлёт сообщение «✏️ Расскажи о себе одним сообщением.» и помечает юзера
// как ждущего ввод info через setPendingInput. Следующее текстовое
// сообщение от него ловится matchPendingInput → handlePendingInput.
//
// Кнопки исходного меню «Настроить профиль» НЕ убираются — юзер может
// передумать и выбрать другое поле; в этом случае их хендлеры (handleProfile*)
// сами вызовут clearPendingInput и info-flow забудется.
func (t *TelegramBot) handleProfileSetInfo(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	if update.CallbackQuery == nil {
		return
	}

	var query *tgmodels.CallbackQuery = update.CallbackQuery
	var from *tgmodels.User = &query.From

	t.clearPendingInput(from.ID)

	var chatID int64
	var messageID int
	if query.Message.Message != nil {
		chatID = query.Message.Message.Chat.ID
		messageID = query.Message.Message.ID
	} else {
		t.logger.Error("profile_set_info callback without accessible message", "telegram_user_id", from.ID)

		return
	}

	t.CreateNewTelegramUserIfNotExists(ctx, from)

	t.LogTelegramEvent(ctx, from, telegramEventPayload{
		Event:             eventCallbackIn,
		ChatID:            chatID,
		TelegramMessageID: int64(messageID),
		Text:              query.Data,
	})

	_, err := b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: query.ID,
	})
	if err != nil {
		t.logger.Error("answer profile_set_info callback", "err", err, "telegram_user_id", from.ID)
	}

	var replyText string = "✏️ Расскажи о себе одним сообщением."
	msg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   replyText,
	})
	if err != nil {
		t.logger.Error("send profile_set_info prompt", "err", err, "chat_id", chatID)

		return
	}

	t.LogTelegramEvent(ctx, from, telegramEventPayload{
		Event:             eventMessageOut,
		ChatID:            chatID,
		TelegramMessageID: int64(msg.ID),
		Text:              replyText,
	})

	t.setPendingInput(from.ID, pendingInputProfileInfo)
}
