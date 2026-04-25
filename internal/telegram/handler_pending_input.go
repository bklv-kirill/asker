package telegram

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// handlePendingInput — общий matchFunc-обработчик текстового ответа
// от юзера, для которого выставлен pending state (см. matchPendingInput
// в telegram.go). Файл маршрутизирует вызов в конкретный per-kind
// обработчик (handleProfileAgeInput и т.п.) по значению pending-kind.
// Сами per-kind обработчики живут в отдельных файлах
// (handler_profile_age_input.go и т.п.) — у каждого сценария своя логика
// валидации, форматов ответа и взаимодействия с репозиторием.
//
// Логирование message_in и проверку наличия pending state делаем здесь —
// это общий код для всех сценариев. Значения kind namespace'нуты префиксом
// домена (`profile.*`), чтобы при добавлении новых сценариев (опросы,
// модерация) роутинг не путался.
func (t *TelegramBot) handlePendingInput(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil || update.Message.Text == "" {
		return
	}

	var from *models.User = update.Message.From
	var chatID int64 = update.Message.Chat.ID
	var inText string = update.Message.Text
	var inMessageID int64 = int64(update.Message.ID)

	kind, ok := t.getPendingInput(from.ID)
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

	switch kind {
	case pendingInputProfileAge:
		t.handleProfileAgeInput(ctx, b, from, chatID, inText)
	case pendingInputProfileInfo:
		t.handleProfileInfoInput(ctx, b, from, chatID, inText)
	default:
		t.logger.Error("pending input: unknown kind", "telegram_user_id", from.ID, "kind", kind)
		t.clearPendingInput(from.ID)
	}
}
