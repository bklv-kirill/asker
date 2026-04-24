package usersRepo

import (
	"context"
	"database/sql"
	"errors"
)

type usersSQLiteRepo struct {
	db *sql.DB
}

// NewUsersSQLiteRepo принимает готовое соединение с SQLite и возвращает
// реализацию Repository. Жизненный цикл соединения управляется вызывающим
// (main.go) — репозиторий его не закрывает.
func NewUsersSQLiteRepo(db *sql.DB) Repository {
	return &usersSQLiteRepo{db: db}
}

func (r *usersSQLiteRepo) Create(ctx context.Context, name string) (int64, error) {
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
