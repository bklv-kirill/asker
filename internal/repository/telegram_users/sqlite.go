package telegramUsersRepo

import (
	"context"
	"database/sql"
	"errors"
)

type telegramUsersSQLiteRepo struct {
	db *sql.DB
}

// NewTelegramUsersSQLiteRepo принимает готовое соединение с SQLite и возвращает
// реализацию Repository. Жизненный цикл соединения управляется вызывающим
// (main.go) — репозиторий его не закрывает.
func NewTelegramUsersSQLiteRepo(db *sql.DB) Repository {
	return &telegramUsersSQLiteRepo{db: db}
}

func (r *telegramUsersSQLiteRepo) Create(ctx context.Context, userID int64, telegramUserID int64) (int64, error) {
	result, err := r.db.ExecContext(
		ctx,
		`INSERT INTO telegram_users (user_id, telegram_user_id) VALUES (?, ?)`,
		userID, telegramUserID,
	)
	if err != nil {
		return 0, errors.Join(ErrCreate, err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, errors.Join(ErrCreate, err)
	}

	return id, nil
}
