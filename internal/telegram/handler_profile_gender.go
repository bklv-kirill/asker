package telegram

import (
	"context"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"

	"github.com/bklv-kirill/asker/internal/models"
)

// handleProfileGender — обработчик inline-кнопок «Мужчина»/«Женщина»
// (callback_data с префиксом profile_gender_). Шаги:
//  1. Журнал callback_in (Text = query.Data, чтобы было видно выбранное
//     значение в БД).
//  2. AnswerCallbackQuery — убирает спиннер.
//  3. Lookup tgUser → users.id (через GetByTelegramUserID). Если у юзера
//     ещё нет привязки (user_id IS NULL) — отвечаем «Сначала привяжи
//     номер.» и выходим.
//  4. Маппим query.Data → значение models.Gender (Мужчина/Женщина).
//     Невалидный data — логируем и выходим (теоретически недостижимо,
//     MatchTypePrefix отбрасывает чужие).
//  5. users.SetGender(ctx, *tgUser.UserID, gender).
//  6. DeleteMessage — удаляет исходное сообщение «Укажи свой пол» целиком,
//     чтобы не захламлять чат и исключить повторное нажатие.
//  7. Ответ «✅ Пол сохранён.» отдельным сообщением + журнал message_out.
//
// Импорт TG-пакета моделей переименован в tgmodels, чтобы не конфликтовать
// с нашим internal/models (Gender, User и т.д.) в этом файле.
func (t *TelegramBot) handleProfileGender(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
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
		t.logger.Error("profile_gender callback without accessible message", "telegram_user_id", from.ID, "data", query.Data)

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
		t.logger.Error("answer profile_gender callback", "err", err, "telegram_user_id", from.ID, "data", query.Data)
	}

	tgUser, err := t.telegramUsers.GetByTelegramUserID(ctx, from.ID)
	if err != nil {
		t.logger.Error("telegram_users get on profile_gender", "err", err, "telegram_user_id", from.ID)
		t.sendProfileGenderReply(ctx, b, from, chatID, "❌ Не получилось сохранить. Попробуй позже.")

		return
	}

	if tgUser.UserID == nil {
		t.sendProfileGenderReply(ctx, b, from, chatID, "⚠️ Сначала привяжи номер телефона. Используй /start")

		return
	}

	var gender models.Gender
	switch query.Data {
	case profileGenderMaleCallback:
		gender = models.GenderMale
	case profileGenderFemaleCallback:
		gender = models.GenderFemale
	default:
		t.logger.Error("profile_gender unexpected callback data", "telegram_user_id", from.ID, "data", query.Data)

		return
	}

	err = t.users.SetGender(ctx, *tgUser.UserID, gender)
	if err != nil {
		t.logger.Error("users set gender", "err", err, "telegram_user_id", from.ID, "users_id", *tgUser.UserID, "gender", string(gender))
		t.sendProfileGenderReply(ctx, b, from, chatID, "❌ Не получилось сохранить. Попробуй позже.")

		return
	}

	t.logger.Info("users gender updated",
		"users_id", *tgUser.UserID,
		"telegram_user_id", from.ID,
		"gender", string(gender),
	)

	_, err = b.DeleteMessage(ctx, &bot.DeleteMessageParams{
		ChatID:    chatID,
		MessageID: messageID,
	})
	if err != nil {
		// Не критично: сообщение может быть слишком старым, без права edit
		// и т.п. — UX не блокируем, идём отвечать.
		t.logger.Error("delete profile_gender message", "err", err, "chat_id", chatID, "message_id", messageID)
	}

	t.sendProfileGenderReply(ctx, b, from, chatID, "✅ Пол сохранён.")
}

// sendProfileGenderReply шлёт ответ и пишет message_out в журнал при
// успехе. Все ветки handleProfileGender используют один и тот же шаблон
// «текст без клавиатуры», поэтому хелпер локальный.
func (t *TelegramBot) sendProfileGenderReply(ctx context.Context, b *bot.Bot, from *tgmodels.User, chatID int64, text string) {
	msg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	})
	if err != nil {
		t.logger.Error("send profile_gender reply", "err", err, "chat_id", chatID, "text", text)

		return
	}

	t.LogTelegramEvent(ctx, from, telegramEventPayload{
		Event:             eventMessageOut,
		ChatID:            chatID,
		TelegramMessageID: int64(msg.ID),
		Text:              text,
	})
}
