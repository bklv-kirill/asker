package telegram

import (
	"context"
	"strings"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
)

// handleProfileInfoInput — конкретный обработчик ввода info, в который
// диспетчеризует handlePendingInput, когда pending == pendingInputProfileInfo.
// Принимает свободный текст: TrimSpace + проверка на непустоту, остальное
// уходит в users.SetInfo как есть. CHECK'а на колонке нет — длиной и
// форматом не ограничиваем (TG сам режет сообщения по 4096 символов).
//
// Контракт state: ставится в handler_profile_set_info.go при отправке
// prompt'а; здесь — clearPendingInput на любом терминальном исходе
// (успех, нет user_id, ошибка БД), кроме невалидного ввода (пустой текст) —
// в этом случае state остаётся, юзер пробует снова.
func (t *TelegramBot) handleProfileInfoInput(ctx context.Context, b *bot.Bot, from *tgmodels.User, chatID int64, text string) {
	var info string = strings.TrimSpace(text)
	if info == "" {
		t.sendProfileInfoInputReply(ctx, b, from, chatID, "⚠️ Текст не может быть пустым.")

		return
	}

	tgUser, err := t.telegramUsers.GetByTelegramUserID(ctx, from.ID)
	if err != nil {
		t.logger.Error("telegram_users get on profile_info input", "err", err, "telegram_user_id", from.ID)
		t.clearPendingInput(from.ID)
		t.sendProfileInfoInputReply(ctx, b, from, chatID, "❌ Не получилось сохранить. Попробуй позже.")

		return
	}

	if tgUser.UserID == nil {
		t.clearPendingInput(from.ID)
		t.sendProfileInfoInputReply(ctx, b, from, chatID, "⚠️ Сначала привяжи номер телефона. Используй /start")

		return
	}

	err = t.users.SetInfo(ctx, *tgUser.UserID, info)
	if err != nil {
		t.logger.Error("users set info", "err", err, "telegram_user_id", from.ID, "users_id", *tgUser.UserID)
		t.clearPendingInput(from.ID)
		t.sendProfileInfoInputReply(ctx, b, from, chatID, "❌ Не получилось сохранить. Попробуй позже.")

		return
	}

	t.logger.Info("users info updated",
		"users_id", *tgUser.UserID,
		"telegram_user_id", from.ID,
	)

	t.clearPendingInput(from.ID)
	t.sendProfileInfoInputReply(ctx, b, from, chatID, "✅ Информация сохранена.")
}

// sendProfileInfoInputReply — локальный хелпер для ответов из info-flow.
// Один шаблон «текст без клавиатуры» + журнал message_out при успехе.
// Семантика других полей профиля может потребовать другой формат ответа,
// поэтому хелпер приватный для info, а не общий по всем pending-полям.
func (t *TelegramBot) sendProfileInfoInputReply(ctx context.Context, b *bot.Bot, from *tgmodels.User, chatID int64, text string) {
	msg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	})
	if err != nil {
		t.logger.Error("send profile_info_input reply", "err", err, "chat_id", chatID, "text", text)

		return
	}

	t.LogTelegramEvent(ctx, from, telegramEventPayload{
		Event:             eventMessageOut,
		ChatID:            chatID,
		TelegramMessageID: int64(msg.ID),
		Text:              text,
	})
}
