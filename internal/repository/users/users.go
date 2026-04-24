// Package users содержит репозиторий доменной таблицы users.
// Потребители (хендлеры, сервисы) зависят от интерфейса Repository,
// конкретная реализация поверх *sql.DB живёт в этом же пакете.
package users

import (
	"context"
	"database/sql"
	"errors"
)

// ErrCreate — ошибка вставки строки в users. Оборачивается причиной через
// errors.Join (правило проекта: fmt.Errorf в return запрещён).
var ErrCreate = errors.New("users: create")

// Repository — интерфейс доступа к таблице users.
type Repository interface {
	// Create вставляет запись с заданным именем и возвращает id созданной строки.
	// Остальные поля профиля (gender, age, info) опциональны — на момент создания
	// они NULL, заполняются позже отдельными методами.
	Create(ctx context.Context, name string) (int64, error)
}

type repository struct {
	db *sql.DB
}

// NewRepository принимает готовое соединение с SQLite и возвращает реализацию
// Repository. Жизненный цикл соединения управляется вызывающим (main.go) —
// репозиторий его не закрывает.
func NewRepository(db *sql.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, name string) (int64, error) {
	result, err := r.db.ExecContext(ctx, `INSERT INTO users (name) VALUES (?)`, name)
	if err != nil {
		return 0, errors.Join(ErrCreate, err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, errors.Join(ErrCreate, err)
	}

	return id, nil
}
