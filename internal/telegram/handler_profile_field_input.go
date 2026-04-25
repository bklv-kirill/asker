package telegram

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// handleProfileFieldInput — общий matchFunc-обработчик текстового ответа
// от юзера, для которого выставлен pending state (см. matchProfileFieldInput
// в telegram.go). Файл маршрутизирует вызов в конкретный per-field
// обработчик (handleProfileAgeInput и т.п.) по значению pending-поля.
// Сами per-field обработчики живут в отдельных файлах
// (handler_profile_age_input.go и т.п.) — у каждого поля своя логика
// валидации, форматов ответа и взаимодействия с репозиторием.
//
// Логирование message_in и проверку наличия pending state делаем здесь —
// это общий код для всех полей.
func (t *TelegramBot) handleProfileFieldInput(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil || update.Message.Text == "" {
		return
	}

	var from *models.User = update.Message.From
	var chatID int64 = update.Message.Chat.ID
	var inText string = update.Message.Text
	var inMessageID int64 = int64(update.Message.ID)

	field, ok := t.getPendingProfileField(from.ID)
	if !ok {
		return
	}

	t.CreateNewTelegramUserIfNotExists(ctx, from)

	t.LogTelegramEvent(ctx, from, telegramEventPayload{
		Event:             eventMessageIn,
		ChatID:            chatID,
		TelegramMessageID: inMessageID,
		Text:              inText,
	})

	switch field {
	case profilePendingFieldAge:
		t.handleProfileAgeInput(ctx, b, from, chatID, inText)
	default:
		t.logger.Error("profile field input: unknown pending field", "telegram_user_id", from.ID, "field", field)
		t.clearPendingProfileField(from.ID)
	}
}
