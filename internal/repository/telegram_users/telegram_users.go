// Package telegramUsersRepo содержит контракт и реализации репозитория
// таблицы telegram_users — привязки доменного пользователя (users) к его
// Telegram-аккаунту. Интерфейс Repository описывает доступные операции;
// конкретная реализация поверх SQLite лежит в sqlite.go.
package telegramUsersRepo

import (
	"context"
	"errors"
)

// ErrCreate — ошибка вставки строки в telegram_users. Оборачивается причиной
// через errors.Join (правило проекта: fmt.Errorf в return запрещён).
var ErrCreate = errors.New("telegram_users: create")

// Repository — интерфейс доступа к таблице telegram_users.
type Repository interface {
	// Create вставляет привязку пользователя к Telegram-аккаунту и возвращает
	// id созданной строки. Оба параметра обязательны: userID ссылается на
	// users.id (FK), telegramUserID — ID аккаунта в Telegram. Схема требует
	// уникальности обоих (1-к-1), поэтому повторная привязка существующего
	// users.id или telegram_user_id приводит к ошибке UNIQUE-констрейнта.
	Create(ctx context.Context, userID int64, telegramUserID int64) (int64, error)
}
