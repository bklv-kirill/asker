package telegram

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"

	telegramUsersRepo "github.com/bklv-kirill/asker/internal/repository/telegram_users"
)

// handleStart — обработчик /start. Различает три состояния:
//  1. Пользователь новый или возвратившийся, но НЕ привязал номер →
//     приветствие + предложение привязать номер с inline-кнопкой
//     «Привязать номер» (callback_data = attachPhoneCallbackData).
//  2. Возвратившийся, номер уже привязан → короткое «Рад тебя снова видеть!».
//  3. Получение записи telegram_users упало с реальным сбоем → идём по
//     ветке «новый пользователь» (без блокировки UX), повторно создаём
//     идемпотентно.
//
// Все шаги фиксируются в журнале telegram_events: command_in для самой
// команды и message_out для ответа бота (после успешной отправки).
func (t *TelegramBot) handleStart(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	var from *tgmodels.User = update.Message.From
	var chatID int64 = update.Message.Chat.ID
	var text string = update.Message.Text
	var messageID int64 = int64(update.Message.ID)

	t.clearPendingInput(from.ID)

	// Сначала пробуем достать существующую запись — она нужна, чтобы
	// различить «новый», «возвратившийся без номера» и «возвратившийся
	// с привязанным номером». Ошибка ErrNotFound — валидное состояние
	// «новый пользователь» (found=false). Любая другая — лог + идём по
	// ветке нового (found=false).
	tgUser, err := t.telegramUsers.GetByTelegramUserID(ctx, from.ID)
	var found bool = err == nil
	if err != nil && !errors.Is(err, telegramUsersRepo.ErrNotFound) {
		t.logger.Error("telegram_users get on /start", "err", err, "telegram_user_id", from.ID)
	}

	t.CreateNewTelegramUserIfNotExists(ctx, from)

	t.LogTelegramEvent(ctx, from, telegramEventPayload{
		Event:             eventCommandIn,
		ChatID:            chatID,
		TelegramMessageID: messageID,
		Text:              text,
	})

	var (
		replyText   string
		replyMarkup any
	)

	switch {
	case found && tgUser.UserID != nil:
		replyText = fmt.Sprintf("👋 Рад тебя снова видеть, %s!\n\nℹ️ Номер телефона уже привязан.", from.FirstName)
		replyMarkup = profileSettingsKeyboard()
	case found:
		replyText = fmt.Sprintf("👋 Рад тебя снова видеть, %s!\n\n📱 Чтобы начать общение привяжи свой номер телефона.", from.FirstName)
		replyMarkup = attachPhoneInlineMarkup()
	default:
		replyText = fmt.Sprintf("👋 Привет, %s! Я %s.\n\n📱 Чтобы начать общение привяжи свой номер телефона.", from.FirstName, t.botName)
		replyMarkup = attachPhoneInlineMarkup()
	}

	msg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        replyText,
		ReplyMarkup: replyMarkup,
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
