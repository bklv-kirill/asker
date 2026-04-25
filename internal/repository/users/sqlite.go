package usersRepo

import (
	"context"
	"database/sql"
	"errors"

	"github.com/mattn/go-sqlite3"

	"github.com/bklv-kirill/asker/internal/models"
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

func (r *usersSQLiteRepo) Create(ctx context.Context, name, phone string) (int64, error) {
	result, err := r.db.ExecContext(
		ctx,
		`INSERT INTO users (name, phone) VALUES (?, ?)`,
		name, phone,
	)
	if err != nil {
		// В таблице users есть один UNIQUE — phone, поэтому extended-код
		// SQLITE_CONSTRAINT_UNIQUE однозначно указывает на конфликт по phone.
		// Если в схеме появятся другие UNIQUE-колонки, потребуется различать
		// по тексту ошибки или extended-коду + имени колонки.
		var sqliteErr sqlite3.Error
		if errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
			return 0, errors.Join(ErrPhoneTaken, err)
		}

		return 0, errors.Join(ErrCreate, err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, errors.Join(ErrCreate, err)
	}

	return id, nil
}

func (r *usersSQLiteRepo) SetGender(ctx context.Context, id int64, gender models.Gender) error {
	// Явная конвертация в string: database/sql драйверы маппят базовые
	// kinds (string/int/...), а named-type Gender напрямую могут не понять
	// без driver.Valuer.
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE users SET gender = ? WHERE id = ?`,
		string(gender), id,
	)
	if err != nil {
		return errors.Join(ErrSetGender, err)
	}

	return nil
}

func (r *usersSQLiteRepo) SetAge(ctx context.Context, id int64, age int) error {
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE users SET age = ? WHERE id = ?`,
		age, id,
	)
	if err != nil {
		return errors.Join(ErrSetAge, err)
	}

	return nil
}

func (r *usersSQLiteRepo) SetInfo(ctx context.Context, id int64, info string) error {
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE users SET info = ? WHERE id = ?`,
		info, id,
	)
	if err != nil {
		return errors.Join(ErrSetInfo, err)
	}

	return nil
}
