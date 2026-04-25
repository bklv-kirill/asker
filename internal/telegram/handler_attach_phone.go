package telegram

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// handleAttachPhone — обработчик нажатия inline-кнопки «Привязать номер»
// (callback_data = attachPhoneCallbackData). Шаги:
//   1) AnswerCallbackQuery — обязателен, иначе у юзера на кнопке висит
//      спиннер до таймаута TG.
//   2) EditMessageReplyMarkup с пустой клавиатурой — убирает inline-кнопку
//      из исходного сообщения, чтобы её нельзя было нажать повторно.
//   3) Отправка нового сообщения с reply-keyboard, в которой одна кнопка
//      request_contact — TG нарисует юзеру «Поделиться номером».
//   4) Журнал: callback_in (нажатие) и message_out (ответ бота).
func (t *TelegramBot) handleAttachPhone(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}

	var query *models.CallbackQuery = update.CallbackQuery
	var from *models.User = &query.From

	t.clearPendingProfileField(from.ID)

	// chat_id и message_id из исходного сообщения с inline-кнопкой —
	// они нужны и для EditMessageReplyMarkup, и для нового SendMessage.
	// Поле Message в callback может быть либо обычным сообщением (Message),
	// либо «недоступным» (InaccessibleMessage) — для нашего сценария это
	// всегда обычный Message, так как кнопку шлёт сам бот.
	var chatID int64
	var sourceMessageID int

	if query.Message.Message != nil {
		chatID = query.Message.Message.Chat.ID
		sourceMessageID = query.Message.Message.ID
	} else {
		t.logger.Error("attach_phone callback without accessible message", "telegram_user_id", from.ID)

		return
	}

	t.CreateNewTelegramUserIfNotExists(ctx, from)

	t.LogTelegramEvent(ctx, from, telegramEventPayload{
		Event:             eventCallbackIn,
		ChatID:            chatID,
		TelegramMessageID: int64(sourceMessageID),
		Text:              attachPhoneCallbackData,
	})

	_, err := b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: query.ID,
	})
	if err != nil {
		// Не блокируем сценарий — спиннер у пользователя через какое-то
		// время сам пропадёт по таймауту TG.
		t.logger.Error("answer attach_phone callback", "err", err, "telegram_user_id", from.ID)
	}

	// Пустой InlineKeyboardMarkup убирает кнопку из исходного сообщения.
	// Если правка упала (сообщение слишком старое, право edit недоступно
	// и т.п.) — логируем и идём дальше, для UX это не критично.
	_, err = b.EditMessageReplyMarkup(ctx, &bot.EditMessageReplyMarkupParams{
		ChatID:      chatID,
		MessageID:   sourceMessageID,
		ReplyMarkup: models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{}},
	})
	if err != nil {
		t.logger.Error("edit attach_phone source message", "err", err, "chat_id", chatID, "message_id", sourceMessageID)
	}

	var replyText string = "📱 Нажми кнопку ниже, чтобы поделиться номером."
	msg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   replyText,
		ReplyMarkup: models.ReplyKeyboardMarkup{
			Keyboard: [][]models.KeyboardButton{
				{
					{Text: "📱 Поделиться номером", RequestContact: true},
				},
			},
			OneTimeKeyboard: true,
			ResizeKeyboard:  true,
		},
	})
	if err != nil {
		t.logger.Error("send attach_phone request_contact prompt", "err", err, "chat_id", chatID)

		return
	}

	t.LogTelegramEvent(ctx, from, telegramEventPayload{
		Event:             eventMessageOut,
		ChatID:            chatID,
		TelegramMessageID: int64(msg.ID),
		Text:              replyText,
	})
}
