package telegram

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// handleProfileSetAge — обработчик inline-кнопки «🎂 Указать возраст» из
// меню «Настроить профиль» (callback_data = profileSetAgeCallback). Шлёт
// сообщение «🎂 Введи свой возраст числом.» и помечает юзера как ждущего
// ввод возраста через setPendingProfileField. Следующее текстовое
// сообщение от него ловится matchProfileFieldInput → handleProfileFieldInput.
//
// Кнопки исходного меню «Настроить профиль» НЕ убираются — юзер может
// передумать и выбрать другое поле; в этом случае их хендлеры (handleProfile*)
// сами вызовут clearPendingProfileField и age-flow забудется.
func (t *TelegramBot) handleProfileSetAge(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}

	var query *models.CallbackQuery = update.CallbackQuery
	var from *models.User = &query.From

	t.clearPendingProfileField(from.ID)

	var chatID int64
	var sourceMessageID int
	if query.Message.Message != nil {
		chatID = query.Message.Message.Chat.ID
		sourceMessageID = query.Message.Message.ID
	} else {
		t.logger.Error("profile_set_age callback without accessible message", "telegram_user_id", from.ID)

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
		t.logger.Error("answer profile_set_age callback", "err", err, "telegram_user_id", from.ID)
	}

	var replyText string = "🎂 Введи свой возраст числом."
	msg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   replyText,
	})
	if err != nil {
		t.logger.Error("send profile_set_age prompt", "err", err, "chat_id", chatID)

		return
	}

	t.LogTelegramEvent(ctx, from, telegramEventPayload{
		Event:             eventMessageOut,
		ChatID:            chatID,
		TelegramMessageID: int64(msg.ID),
		Text:              replyText,
	})

	// Помечаем юзера как ждущего ввод возраста ПОСЛЕ успешной отправки
	// prompt'а: если SendMessage упал, нет смысла включать state — юзер
	// не увидел вопрос.
	t.setPendingProfileField(from.ID, profilePendingFieldAge)
}
