package telegram

import (
	"context"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
)

// handleProfileSetGender — обработчик inline-кнопки «Указать пол» из меню
// «Настроить профиль» (callback_data = profileSetGenderCallback). Шлёт
// сообщение «Укажи свой пол.» с inline-кнопками «Мужчина»/«Женщина»;
// фактическое обновление users.gender — в handleProfileGender по
// нажатию одной из них.
//
// Inline-кнопки исходного меню «Настроить профиль» НЕ убираем: юзер
// может вернуться и выбрать другое поле без повторного нажатия
// «Настроить профиль».
func (t *TelegramBot) handleProfileSetGender(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	if update.CallbackQuery == nil {
		return
	}

	var query *tgmodels.CallbackQuery = update.CallbackQuery
	var from *tgmodels.User = &query.From

	t.clearPendingInput(from.ID)
	t.dropUserDebounce(from.ID)

	var chatID int64
	var messageID int
	if query.Message.Message != nil {
		chatID = query.Message.Message.Chat.ID
		messageID = query.Message.Message.ID
	} else {
		t.logger.Error("profile_set_gender callback without accessible message", "telegram_user_id", from.ID)

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
		t.logger.Error("answer profile_set_gender callback", "err", err, "telegram_user_id", from.ID)
	}

	var replyText string = "👤 Укажи свой пол."
	msg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        replyText,
		ReplyMarkup: profileGenderInlineMarkup(),
	})
	if err != nil {
		t.logger.Error("send profile_set_gender prompt", "err", err, "chat_id", chatID)

		return
	}

	t.LogTelegramEvent(ctx, from, telegramEventPayload{
		Event:             eventMessageOut,
		ChatID:            chatID,
		TelegramMessageID: int64(msg.ID),
		Text:              replyText,
	})
}
