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
// Структура: жирный заголовок, затем список полей (эмодзи-маркер + жирный
// label + значение); телефон в `<code>` — моно-шрифт читается аккуратнее
// плоского текста; «о себе» вынесено в `<blockquote>` отдельной секцией,
// потому что текст свободный и многострочный, в общий список не лезет.
// Свободные строки (имя, инфо, gender) экранируются через html.EscapeString —
// юзер мог написать `<` или `&` в «о себе», иначе TG отвалит ParseMode.
// Phone хранится только цифрами (CHECK), age — int, gender — enum: им
// экранирование не нужно, но gender прогоняется через escape для единообразия.
func formatMyProfile(u *models.User) string {
	var lines []string = []string{
		"<b>👤 Твой профиль</b>",
		"",
		"🪪 <b>Имя:</b> " + formatProfileField(u.Name, html.EscapeString),
		"📱 <b>Телефон:</b> <code>" + html.EscapeString(u.Phone) + "</code>",
		"⚧ <b>Пол:</b> " + formatProfileField(u.Gender, formatGenderWithEmoji),
		"🎂 <b>Возраст:</b> " + formatProfileField(u.Age, strconv.Itoa),
		"",
		"✏️ <b>О себе:</b>",
		formatInfoBlock(u.Info),
	}

	return strings.Join(lines, "\n")
}

// formatProfileField — generic-хелпер для опциональных полей профиля.
// nil → курсивное «<i>не указано</i>» (визуально отделяет пропущенные
// поля от заполненных); иначе значение прогоняется через format-функцию
// (для строк — html.EscapeString, для int — strconv.Itoa, для enum'ов —
// adapter с конвертацией).
func formatProfileField[T any](p *T, format func(T) string) string {
	if p == nil {
		return "<i>не указано</i>"
	}

	return format(*p)
}

// formatGenderWithEmoji подмешивает иконку перед русским значением gender'а.
// Для известных констант (мужчина/женщина) — соответствующий символ;
// неизвестное значение (теоретически возможно, если в БД оказалась левая
// строка вне CHECK) выводится без эмодзи.
func formatGenderWithEmoji(g models.Gender) string {
	switch g {
	case models.GenderMale:
		return "👨 " + html.EscapeString(string(g))
	case models.GenderFemale:
		return "👩 " + html.EscapeString(string(g))
	}

	return html.EscapeString(string(g))
}

// formatInfoBlock — отдельный хелпер для info, потому что обёртка отличается
// от обычных полей: заполненный текст оборачивается в <blockquote> (TG рисует
// его серой полосой слева, читается как цитата), пустое значение — обычный
// курсивный плейсхолдер без блока.
func formatInfoBlock(p *string) string {
	if p == nil {
		return "<i>не указано</i>"
	}

	return "<blockquote>" + html.EscapeString(*p) + "</blockquote>"
}
