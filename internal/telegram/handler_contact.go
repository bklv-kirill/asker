package telegram

import (
	"context"
	"errors"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"

	usersRepo "github.com/bklv-kirill/asker/internal/repository/users"
)

// handleContact срабатывает на любое сообщение с полем Contact (юзер
// поделился номером через кнопку request_contact). Сценарий привязки:
//  1. Журнал contact_in (Phone в Text payload).
//  2. Защита: Contact.UserID должен совпадать с From.ID — иначе номер
//     чужой (например, пересланный контакт), отказываем.
//  3. Защита: если у этого telegram_users уже есть user_id — отвечаем
//     «у тебя уже привязан номер» и оставляем клавиатуру с «Настроить
//     профиль» (юзер уже считается привязанным).
//  4. users.Create(FirstName, normalizedPhone). UNIQUE-конфликт по phone
//     различается через ErrPhoneTaken.
//  5. telegramUsers.SetUserIDByTelegramUserID — проставляем связь.
//  6. Ответ «Спасибо! Номер привязан.» с reply-keyboard «Настроить профиль».
//
// Все ветки пишут message_out при успешной отправке ответа. Транзакция
// между шагами 4 и 5 не используется — если 5 упадёт, у нас останется
// users без связи; на следующем /start пользователь снова увидит кнопку,
// и шаг 4 упадёт с ErrPhoneTaken (его номер уже в users). Логика
// автоматического восстановления orphan'ов не предусмотрена сейчас.
func (t *TelegramBot) handleContact(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	if update.Message == nil || update.Message.From == nil || update.Message.Contact == nil {
		return
	}

	var from *tgmodels.User = update.Message.From
	var chatID int64 = update.Message.Chat.ID
	var contact *tgmodels.Contact = update.Message.Contact
	var messageID int64 = int64(update.Message.ID)

	t.clearPendingInput(from.ID)
	t.dropUserDebounce(from.ID)

	t.CreateNewTelegramUserIfNotExists(ctx, from)

	t.LogTelegramEvent(ctx, from, telegramEventPayload{
		Event:             eventContactIn,
		ChatID:            chatID,
		TelegramMessageID: messageID,
		Text:              contact.PhoneNumber,
	})

	if contact.UserID != from.ID {
		t.sendContactReply(ctx, b, from, chatID, "⚠️ Нужен твой собственный номер.", nil)

		return
	}

	tgUser, err := t.telegramUsers.GetByTelegramUserID(ctx, from.ID)
	if err != nil {
		// ErrNotFound тут невозможен после CreateNewTelegramUserIfNotExists,
		// но если случится реальный сбой — сообщаем юзеру и выходим.
		t.logger.Error("telegram_users get on contact", "err", err, "telegram_user_id", from.ID)

		t.sendContactReply(ctx, b, from, chatID, "❌ Не получилось сохранить номер. Попробуй позже.", nil)

		return
	}

	if tgUser.UserID != nil {
		t.sendContactReply(ctx, b, from, chatID, "ℹ️ У тебя уже привязан номер.", profileSettingsKeyboard())

		return
	}

	var normalizedPhone string = normalizePhone(contact.PhoneNumber)
	if normalizedPhone == "" {
		// CHECK на users.phone отбросит пустую строку — выясним сразу,
		// чтобы дать осмысленный ответ.
		t.sendContactReply(ctx, b, from, chatID, "❌ Не удалось разобрать номер. Попробуй ещё раз.", nil)

		return
	}

	newUserID, err := t.users.Create(ctx, from.FirstName, normalizedPhone)
	if err != nil {
		if errors.Is(err, usersRepo.ErrPhoneTaken) {
			t.sendContactReply(ctx, b, from, chatID, "⚠️ Этот номер уже привязан к другому аккаунту.", tgmodels.ReplyKeyboardRemove{RemoveKeyboard: true})

			return
		}

		t.logger.Error("users create on contact", "err", err, "telegram_user_id", from.ID)
		t.sendContactReply(ctx, b, from, chatID, "❌ Не получилось сохранить номер. Попробуй позже.", nil)

		return
	}

	t.logger.Info("users created",
		"id", newUserID,
		"telegram_user_id", from.ID,
		"name", from.FirstName,
	)

	err = t.telegramUsers.SetUserIDByTelegramUserID(ctx, from.ID, newUserID)
	if err != nil {
		// users уже создан, но связь не проставилась — записываем в лог
		// для дальнейшего разбора. Юзеру говорим попробовать позже;
		// на следующем /start он снова нажмёт «Привязать», но users.Create
		// тогда упадёт с ErrPhoneTaken (его номер уже в users).
		t.logger.Error("telegram_users set user_id on contact", "err", err, "telegram_user_id", from.ID, "users_id", newUserID)
		t.sendContactReply(ctx, b, from, chatID, "❌ Не получилось сохранить номер. Попробуй позже.", nil)

		return
	}

	t.logger.Info("telegram_users linked to users",
		"telegram_user_id", from.ID,
		"telegram_users_id", tgUser.ID,
		"users_id", newUserID,
	)

	t.sendContactReply(ctx, b, from, chatID, "✅ Спасибо! Номер привязан.", profileSettingsKeyboard())
}

// sendContactReply шлёт ответ на contact_in и пишет message_out в журнал
// при успехе. replyMarkup — произвольная клавиатура: nil (оставить
// текущую — например, request_contact для повторной попытки),
// tgmodels.ReplyKeyboardRemove (снять висящую) или
// profileSettingsKeyboard() (поставить «Настроить профиль» после привязки).
func (t *TelegramBot) sendContactReply(ctx context.Context, b *bot.Bot, from *tgmodels.User, chatID int64, text string, replyMarkup any) {
	msg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ReplyMarkup: replyMarkup,
	})
	if err != nil {
		t.logger.Error("send contact reply", "err", err, "chat_id", chatID, "text", text)

		return
	}

	t.LogTelegramEvent(ctx, from, telegramEventPayload{
		Event:             eventMessageOut,
		ChatID:            chatID,
		TelegramMessageID: int64(msg.ID),
		Text:              text,
	})
}
