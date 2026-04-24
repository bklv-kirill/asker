// Package usersRepo содержит контракт и реализации репозитория доменной
// таблицы users. Интерфейс Repository описывает доступные операции;
// конкретная реализация поверх SQLite лежит в sqlite.go.
package usersRepo

import (
	"context"
	"errors"
)

// ErrCreate — ошибка вставки строки в users. Оборачивается причиной через
// errors.Join (правило проекта: fmt.Errorf в return запрещён).
var ErrCreate = errors.New("users: create")

// Repository — интерфейс доступа к таблице users. Потребители (хендлеры,
// сервисы) зависят от этого интерфейса, а не от конкретной реализации.
type Repository interface {
	// Create вставляет запись с заданным именем и возвращает id созданной строки.
	// Остальные поля профиля (gender, age, info) опциональны — на момент создания
	// они NULL, заполняются позже отдельными методами.
	Create(ctx context.Context, name string) (int64, error)
}
