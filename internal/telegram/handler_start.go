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
	var chatID int64 = update.Message.Chat.ID

	// Если пользователь уже есть — здороваемся и выходим. Ошибку exists-check
	// не считаем фатальной: в худшем случае пойдём по new-user ветке и
	// CreateNewTelegramUserIfNotExists внутри повторит проверку.
	exists, err := t.telegramUsers.ExistsByTelegramUserID(ctx, from.ID)
	if err != nil {
		t.logger.Error("telegram_users exists check", "err", err, "telegram_user_id", from.ID)
	} else if exists {
		var text string = fmt.Sprintf("Рад тебя снова видеть, %s!", from.FirstName)
		if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   text,
		}); err != nil {
			t.logger.Error("send /start welcome-back reply", "err", err, "chat_id", chatID)
		}

		return
	}

	t.CreateNewTelegramUserIfNotExists(ctx, from)

	var text string = fmt.Sprintf("Привет, %s! Я %s.", from.FirstName, t.botName)
	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	}); err != nil {
		t.logger.Error("send /start reply", "err", err, "chat_id", chatID)
	}
}
