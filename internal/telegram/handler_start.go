package telegram

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (t *TelegramBot) handleStart(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	var from *models.User = update.Message.From

	// Сохраняем TG-аккаунт один раз. Повторный /start от того же юзера не
	// должен падать в UNIQUE — предварительно проверяем существование.
	// Ошибки хранилища логируем, но ответ пользователю не блокируем: бот
	// должен здороваться, даже если БД временно недоступна.
	exists, err := t.telegramUsers.ExistsByTelegramUserID(ctx, from.ID)
	if err != nil {
		t.logger.Error("telegram_users exists check", "err", err, "telegram_user_id", from.ID)
	} else if !exists {
		var lastName *string = optionalString(from.LastName)
		var username *string = optionalString(from.Username)

		if _, err := t.telegramUsers.Create(ctx, from.ID, from.FirstName, lastName, username); err != nil {
			t.logger.Error("telegram_users create", "err", err, "telegram_user_id", from.ID)
		}
	}

	var text string = fmt.Sprintf("Привет, %s! Я %s.", from.FirstName, t.botName)
	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   text,
	}); err != nil {
		t.logger.Error("send /start reply", "err", err, "chat_id", update.Message.Chat.ID)
	}
}

// optionalString отличает «TG не прислал поле» от «прислал, но пусто»: в
// tgmodels.User опциональные поля приходят как пустая строка — маппим её в nil,
// чтобы в БД легла NULL, а не "".
func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
