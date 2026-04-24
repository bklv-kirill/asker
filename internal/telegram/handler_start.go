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

	t.CreateNewTelegramUserIfNotExists(ctx, update.Message.From)

	var text string = fmt.Sprintf("Привет, %s! Я %s.", update.Message.From.FirstName, t.botName)
	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   text,
	}); err != nil {
		t.logger.Error("send /start reply", "err", err, "chat_id", update.Message.Chat.ID)
	}
}
