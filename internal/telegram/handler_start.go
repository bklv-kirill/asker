package telegram

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	telegramUsersRepo "github.com/bklv-kirill/asker/internal/repository/telegram_users"
)

// handleStart — обработчик /start. Различает три состояния:
//   1) Пользователь новый или возвратившийся, но НЕ привязал номер →
//      приветствие + предложение привязать номер с inline-кнопкой
//      «Привязать номер» (callback_data = attachPhoneCallbackData).
//   2) Возвратившийся, номер уже привязан → короткое «Рад тебя снова видеть!».
//   3) Получение записи telegram_users упало с реальным сбоем → идём по
//      ветке «новый пользователь» (без блокировки UX), повторно создаём
//      идемпотентно.
//
// Все шаги фиксируются в журнале telegram_events: command_in для самой
// команды и message_out для ответа бота (после успешной отправки).
func (t *TelegramBot) handleStart(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	var from *models.User = update.Message.From
	var chatID int64 = update.Message.Chat.ID
	var inText string = update.Message.Text
	var inMessageID int64 = int64(update.Message.ID)

	t.clearPendingProfileField(from.ID)

	// Сначала пробуем достать существующую запись — она нужна, чтобы
	// различить «новый», «возвратившийся без номера» и «возвратившийся
	// с привязанным номером». Ошибка ErrNotFound — валидное состояние
	// «новый пользователь». Любая другая — лог + продолжаем как с новым.
	tgUser, err := t.telegramUsers.GetByTelegramUserID(ctx, from.ID)
	if err != nil && !errors.Is(err, telegramUsersRepo.ErrNotFound) {
		t.logger.Error("telegram_users get on /start", "err", err, "telegram_user_id", from.ID)

		tgUser = nil
	}

	t.CreateNewTelegramUserIfNotExists(ctx, from)

	t.LogTelegramEvent(ctx, from, telegramEventPayload{
		Event:             eventCommandIn,
		ChatID:            chatID,
		TelegramMessageID: inMessageID,
		Text:              inText,
	})

	var (
		replyText   string
		replyMarkup any
	)

	switch {
		case tgUser != nil && tgUser.UserID != nil:
			replyText = fmt.Sprintf("👋 Рад тебя снова видеть, %s!\n\nℹ️ Номер телефона уже привязан.", from.FirstName)
			replyMarkup = profileSettingsKeyboard()
		case tgUser != nil:
			replyText = fmt.Sprintf("👋 Рад тебя снова видеть, %s!\n\n📱 Для более точных ответов можешь привязать свой номер телефона и настроить профиль.", from.FirstName)
			replyMarkup = attachPhoneInlineMarkup()
		default:
			replyText = fmt.Sprintf("👋 Привет, %s! Я %s.\n\n📱 Для более точных ответов можешь привязать свой номер телефона и настроить профиль.", from.FirstName, t.botName)
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

// attachPhoneInlineMarkup собирает inline-клавиатуру с одной кнопкой
// «Привязать номер». При нажатии TG присылает CallbackQuery с
// data = attachPhoneCallbackData — его ловит handleAttachPhone.
func attachPhoneInlineMarkup() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "📱 Привязать номер", CallbackData: attachPhoneCallbackData},
			},
		},
	}
}
