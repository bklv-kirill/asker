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

func (r *telegramUsersSQLiteRepo) Create(
	ctx context.Context,
	telegramUserID int64,
	firstName string,
	lastName *string,
	username *string,
) (int64, error) {
	result, err := r.db.ExecContext(
		ctx,
		`INSERT INTO telegram_users (telegram_user_id, first_name, last_name, username) VALUES (?, ?, ?, ?)`,
		telegramUserID, firstName, nullString(lastName), nullString(username),
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

func (r *telegramUsersSQLiteRepo) ExistsByTelegramUserID(ctx context.Context, telegramUserID int64) (bool, error) {
	// EXISTS + LIMIT 1 — запрос возвращает одну строку с 0/1, не матчит всю
	// таблицу. telegram_user_id UNIQUE, поэтому и без EXISTS скан был бы
	// коротким, но так ещё и семантика явная.
	var exists int
	var err error = r.db.QueryRowContext(
		ctx,
		`SELECT EXISTS(SELECT 1 FROM telegram_users WHERE telegram_user_id = ? LIMIT 1)`,
		telegramUserID,
	).Scan(&exists)
	if err != nil {
		return false, errors.Join(ErrExistsByTelegramUserID, err)
	}
	return exists == 1, nil
}

// nullString мапит опциональное поле контракта (*string) в sql.NullString —
// драйвер-специфичный тип, который знает, как писать NULL в столбец.
func nullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}
