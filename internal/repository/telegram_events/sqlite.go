package telegramEventsRepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

type telegramEventsSQLiteRepo struct {
	db *sql.DB
}

// NewTelegramEventsSQLiteRepo принимает готовое соединение с SQLite и возвращает
// реализацию Repository. Жизненный цикл соединения управляется вызывающим
// (main.go) — репозиторий его не закрывает.
func NewTelegramEventsSQLiteRepo(db *sql.DB) Repository {
	return &telegramEventsSQLiteRepo{db: db}
}

func (r *telegramEventsSQLiteRepo) Create(
	ctx context.Context,
	telegramUserID int64,
	payload json.RawMessage,
) (int64, error) {
	// payload передаём как []byte — драйвер пишет его в TEXT-колонку как есть,
	// CHECK (json_valid(payload)) на уровне схемы отбракует невалидный JSON.
	result, err := r.db.ExecContext(
		ctx,
		`INSERT INTO telegram_events (telegram_user_id, payload) VALUES (?, ?)`,
		telegramUserID, []byte(payload),
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
