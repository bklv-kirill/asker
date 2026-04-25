// Package telegramEventsRepo содержит контракт и реализации репозитория
// таблицы telegram_events — журнала событий Telegram-бота (входящие
// сообщения/команды/callback'и от пользователя и исходящие сообщения бота).
// Записи иммутабельны: только запись и (в будущем) чтение, без update.
//
// Привязка к telegram_users — через FK telegram_user_id (внутренний id
// строки telegram_users, не Telegram-овский from.ID).
//
// Интерфейс Repository описывает доступные операции; конкретные реализации
// (SQLite, файловое хранилище и т.п.) живут в отдельных файлах этого пакета.
// Доменная структура лежит в пакете internal/models.
package telegramEventsRepo

import (
	"context"
	"encoding/json"
	"errors"
)

// ErrCreate — ошибка вставки записи в хранилище. Оборачивается причиной
// через errors.Join (правило проекта: fmt.Errorf в return запрещён).
var ErrCreate = errors.New("telegram_events: create")

// Repository — интерфейс доступа к хранилищу telegram_events. Контракт
// намеренно свободен от деталей конкретной реализации. Сейчас есть только
// запись — методы чтения добавим, когда появится сценарий (аналитика,
// экспорт и т.п.).
type Repository interface {
	// Create сохраняет одно событие журнала и возвращает id созданной записи.
	// telegramUserID — внутренний id из telegram_users (FK), payload —
	// готовый сериализованный JSON. Валидность JSON проверяет CHECK в схеме;
	// при невалидном payload SQLite вернёт ошибку, обёрнутую ErrCreate.
	Create(
		ctx context.Context,
		telegramUserID int64,
		payload json.RawMessage,
	) (int64, error)
}
