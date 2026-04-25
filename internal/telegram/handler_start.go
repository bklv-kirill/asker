package telegram

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// handleStart — обработчик /start. Отвечает разным приветствием в зависимости
// от того, видим мы этого TG-пользователя впервые или уже знаем, и фиксирует
// в журнале telegram_events входящую команду + исходящий ответ.
func (t *TelegramBot) handleStart(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	var from *models.User = update.Message.From
	var chatID int64 = update.Message.Chat.ID
	var inText string = update.Message.Text
	var inMessageID int64 = int64(update.Message.ID)

	// exists-check ДО CreateNewTelegramUserIfNotExists — иначе после создания
	// «новый» неотличим от «возвратившегося». Ошибку считаем не фатальной:
	// в худшем случае поздороваемся как с новым.
	wasExisting, err := t.telegramUsers.ExistsByTelegramUserID(ctx, from.ID)
	if err != nil {
		t.logger.Error("telegram_users exists check", "err", err, "telegram_user_id", from.ID)

		wasExisting = false
	}

	t.CreateNewTelegramUserIfNotExists(ctx, from)

	t.LogTelegramEvent(ctx, from, telegramEventPayload{
		Event:             eventCommandIn,
		ChatID:            chatID,
		TelegramMessageID: inMessageID,
		Text:              inText,
	})

	var replyText string
	if wasExisting {
		replyText = fmt.Sprintf("Рад тебя снова видеть, %s!", from.FirstName)
	} else {
		replyText = fmt.Sprintf("Привет, %s! Я %s.", from.FirstName, t.botName)
	}

	msg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   replyText,
	})
	if err != nil {
		t.logger.Error("send /start reply", "err", err, "chat_id", chatID)

		return
	}

	t.LogTelegramEvent(ctx, from, telegramEventPayload{
		Event:             eventMessageOut,
		ChatID:            chatID,
		TelegramMessageID: int64(msg.ID),
		Text:              replyText,
	})
}
