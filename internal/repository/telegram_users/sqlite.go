package telegramUsersRepo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/bklv-kirill/asker/internal/models"
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

func (r *telegramUsersSQLiteRepo) Create(ctx context.Context, m models.TelegramUserCreate) (int64, error) {
	// user_id передаём явным NULL, чтобы порядок колонок в INSERT соответствовал
	// порядку в схеме (user_id перед telegram_user_id). Запись users появится
	// позже — через SetUserIDByTelegramUserID, когда юзер привяжет номер.
	result, err := r.db.ExecContext(
		ctx,
		`INSERT INTO telegram_users (user_id, telegram_user_id, first_name, last_name, username) VALUES (?, ?, ?, ?, ?)`,
		sql.NullInt64{}, m.TelegramUserID, m.FirstName, ptrToNullString(m.LastName), ptrToNullString(m.Username),
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

func (r *telegramUsersSQLiteRepo) GetByTelegramUserID(ctx context.Context, telegramUserID int64) (models.TelegramUser, error) {
	var (
		id        int64
		userID    sql.NullInt64
		tgUserID  int64
		firstName string
		lastName  sql.NullString
		username  sql.NullString
		createdAt time.Time
		updatedAt time.Time
	)
	var err error = r.db.QueryRowContext(
		ctx,
		`SELECT id, user_id, telegram_user_id, first_name, last_name, username, created_at, updated_at
		   FROM telegram_users
		  WHERE telegram_user_id = ?
		  LIMIT 1`,
		telegramUserID,
	).Scan(&id, &userID, &tgUserID, &firstName, &lastName, &username, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.TelegramUser{}, ErrNotFound
	}
	if err != nil {
		return models.TelegramUser{}, errors.Join(ErrGetByTelegramUserID, err)
	}

	return models.TelegramUser{
		ID:             id,
		UserID:         nullInt64ToPtr(userID),
		TelegramUserID: tgUserID,
		FirstName:      firstName,
		LastName:       nullStringToPtr(lastName),
		Username:       nullStringToPtr(username),
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}, nil
}

func (r *telegramUsersSQLiteRepo) SetUserIDByTelegramUserID(ctx context.Context, telegramUserID int64, userID int64) error {
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE telegram_users SET user_id = ? WHERE telegram_user_id = ?`,
		userID, telegramUserID,
	)
	if err != nil {
		return errors.Join(ErrSetUserID, err)
	}

	return nil
}

// ptrToNullString мапит опциональное поле контракта (*string) в sql.NullString —
// драйвер-специфичный тип, который знает, как писать NULL в столбец.
func ptrToNullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}

	return sql.NullString{String: *s, Valid: true}
}

// nullStringToPtr — обратное отображение: из sql.NullString в *string для
// доменной структуры. NULL (Valid == false) даёт nil, непустое значение —
// указатель на копию строки.
func nullStringToPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}

	return &ns.String
}

// nullInt64ToPtr — для nullable INTEGER колонок (например, user_id).
// NULL даёт nil, заполненное значение — указатель на копию int64.
func nullInt64ToPtr(ni sql.NullInt64) *int64 {
	if !ni.Valid {
		return nil
	}

	return &ni.Int64
}
