package telegram

import (
	"context"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// handleProfileAgeInput — конкретный обработчик ввода возраста, в который
// диспетчеризует handleProfileFieldInput, когда pending == profilePendingFieldAge.
// Парсит число, валидирует диапазон 1..120 (соответствует CHECK схемы),
// вызывает users.SetAge.
//
// Контракт state: ставится в handler_profile_set_age.go при отправке
// prompt'а; здесь — clearPendingProfileField на любом терминальном исходе
// (успех, нет user_id, ошибка БД), кроме невалидного ввода — в этом случае
// state остаётся, юзер пробует снова.
func (t *TelegramBot) handleProfileAgeInput(ctx context.Context, b *bot.Bot, from *models.User, chatID int64, inText string) {
	age, err := strconv.Atoi(strings.TrimSpace(inText))
	if err != nil || age < 1 || age > 120 {
		t.sendProfileAgeInputReply(ctx, b, from, chatID, "⚠️ Введи число от 1 до 120.")

		return
	}

	tgUser, err := t.telegramUsers.GetByTelegramUserID(ctx, from.ID)
	if err != nil {
		t.logger.Error("telegram_users get on profile_age input", "err", err, "telegram_user_id", from.ID)
		t.clearPendingProfileField(from.ID)
		t.sendProfileAgeInputReply(ctx, b, from, chatID, "❌ Не получилось сохранить. Попробуй позже.")

		return
	}

	if tgUser.UserID == nil {
		t.clearPendingProfileField(from.ID)
		t.sendProfileAgeInputReply(ctx, b, from, chatID, "⚠️ Сначала привяжи номер телефона. Используй /start")

		return
	}

	err = t.users.SetAge(ctx, *tgUser.UserID, age)
	if err != nil {
		t.logger.Error("users set age", "err", err, "telegram_user_id", from.ID, "users_id", *tgUser.UserID, "age", age)
		t.clearPendingProfileField(from.ID)
		t.sendProfileAgeInputReply(ctx, b, from, chatID, "❌ Не получилось сохранить. Попробуй позже.")

		return
	}

	t.logger.Info("users age updated",
		"users_id", *tgUser.UserID,
		"telegram_user_id", from.ID,
		"age", age,
	)

	t.clearPendingProfileField(from.ID)
	t.sendProfileAgeInputReply(ctx, b, from, chatID, "✅ Возраст сохранён.")
}

// sendProfileAgeInputReply — локальный хелпер для ответов из age-flow.
// Один шаблон «текст без клавиатуры» + журнал message_out при успехе.
// Семантика других полей профиля может потребовать другой формат ответа,
// поэтому хелпер приватный для age, а не общий по всем pending-полям.
func (t *TelegramBot) sendProfileAgeInputReply(ctx context.Context, b *bot.Bot, from *models.User, chatID int64, text string) {
	msg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	})
	if err != nil {
		t.logger.Error("send profile_age_input reply", "err", err, "chat_id", chatID, "text", text)

		return
	}

	t.LogTelegramEvent(ctx, from, telegramEventPayload{
		Event:             eventMessageOut,
		ChatID:            chatID,
		TelegramMessageID: int64(msg.ID),
		Text:              text,
	})
}
