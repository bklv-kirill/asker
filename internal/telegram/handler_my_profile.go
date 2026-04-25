package telegram

import (
	"context"
	"errors"
	"html"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"

	"github.com/bklv-kirill/asker/internal/models"
	usersRepo "github.com/bklv-kirill/asker/internal/repository/users"
)

// handleMyProfile — обработчик reply-кнопки «👤 Мой профиль» из
// profileSettingsKeyboard. Шлёт сообщение со всеми полями users
// (имя, телефон, пол, возраст, инфо) в формате HTML; пустые поля
// помечаются как «не указано», чтобы юзер видел, что заполнить.
//
// Кнопка появляется только у привязанных юзеров (клавиатура отдаётся
// после Contact-flow), но защита `user_id IS NULL` всё равно стоит —
// клавиатура могла кэшироваться в клиенте.
func (t *TelegramBot) handleMyProfile(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	var from *tgmodels.User = update.Message.From
	var chatID int64 = update.Message.Chat.ID
	var inText string = update.Message.Text
	var inMessageID int64 = int64(update.Message.ID)

	t.clearPendingInput(from.ID)

	t.CreateNewTelegramUserIfNotExists(ctx, from)

	t.LogTelegramEvent(ctx, from, telegramEventPayload{
		Event:             eventMessageIn,
		ChatID:            chatID,
		TelegramMessageID: inMessageID,
		Text:              inText,
	})

	tgUser, err := t.telegramUsers.GetByTelegramUserID(ctx, from.ID)
	if err != nil {
		t.logger.Error("telegram_users get on my_profile", "err", err, "telegram_user_id", from.ID)
		t.sendMyProfileReply(ctx, b, from, chatID, "❌ Не получилось загрузить профиль. Попробуй позже.")

		return
	}

	if tgUser.UserID == nil {
		t.sendMyProfileReply(ctx, b, from, chatID, "⚠️ Сначала привяжи номер телефона. Используй /start")

		return
	}

	user, err := t.users.GetByID(ctx, *tgUser.UserID)
	if err != nil {
		if errors.Is(err, usersRepo.ErrNotFound) {
			t.logger.Error("users get by id: not found", "telegram_user_id", from.ID, "users_id", *tgUser.UserID)
			t.sendMyProfileReply(ctx, b, from, chatID, "❌ Профиль не найден. Попробуй /start.")

			return
		}

		t.logger.Error("users get by id", "err", err, "telegram_user_id", from.ID, "users_id", *tgUser.UserID)
		t.sendMyProfileReply(ctx, b, from, chatID, "❌ Не получилось загрузить профиль. Попробуй позже.")

		return
	}

	t.sendMyProfileReply(ctx, b, from, chatID, formatMyProfile(user))
}

// sendMyProfileReply — локальный хелпер для ответов из my-profile flow.
// Отправляет с ParseMode=HTML (чтобы работали <b>...</b>), журнал
// message_out пишется только при успехе SendMessage.
func (t *TelegramBot) sendMyProfileReply(ctx context.Context, b *bot.Bot, from *tgmodels.User, chatID int64, text string) {
	msg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: tgmodels.ParseModeHTML,
	})
	if err != nil {
		t.logger.Error("send my_profile reply", "err", err, "chat_id", chatID)

		return
	}

	t.LogTelegramEvent(ctx, from, telegramEventPayload{
		Event:             eventMessageOut,
		ChatID:            chatID,
		TelegramMessageID: int64(msg.ID),
		Text:              text,
	})
}

// formatMyProfile собирает HTML-сообщение со всеми полями профиля.
// Свободные строки (имя, инфо, gender) экранируются через html.EscapeString —
// юзер мог написать `<` или `&` в «о себе», иначе TG отвалит ParseMode.
// Phone хранится только цифрами (CHECK), age — int, gender — enum: им
// экранирование не нужно, но gender прогоняется через escape для единообразия.
func formatMyProfile(u *models.User) string {
	var lines []string = []string{
		"<b>👤 Твой профиль</b>",
		"",
		"Имя: " + formatProfileField(u.Name, html.EscapeString),
		"Телефон: " + html.EscapeString(u.Phone),
		"Пол: " + formatProfileField(u.Gender, func(g models.Gender) string { return html.EscapeString(string(g)) }),
		"Возраст: " + formatProfileField(u.Age, strconv.Itoa),
		"О себе: " + formatProfileField(u.Info, html.EscapeString),
	}

	return strings.Join(lines, "\n")
}

// formatProfileField — generic-хелпер для опциональных полей профиля.
// nil → «не указано»; иначе значение прогоняется через format-функцию
// (для строк — html.EscapeString, для int — strconv.Itoa, для enum'ов —
// adapter с конвертацией). Отделяет «отсутствие значения» от «как форматировать
// значение» — последнее решает caller, плейсхолдер фиксирован.
func formatProfileField[T any](p *T, format func(T) string) string {
	if p == nil {
		return "не указано"
	}

	return format(*p)
}
