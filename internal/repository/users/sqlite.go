package usersRepo

import (
	"context"
	"database/sql"
	"errors"
	"time"

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

func (r *usersSQLiteRepo) GetByID(ctx context.Context, id int64) (*models.User, error) {
	var (
		userID    int64
		name      sql.NullString
		gender    sql.NullString
		age       sql.NullInt64
		info      sql.NullString
		phone     string
		createdAt time.Time
		updatedAt time.Time
	)
	var err error = r.db.QueryRowContext(
		ctx,
		`SELECT id, name, gender, age, info, phone, created_at, updated_at
		   FROM users
		  WHERE id = ?
		  LIMIT 1`,
		id,
	).Scan(&userID, &name, &gender, &age, &info, &phone, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, errors.Join(ErrGetByID, err)
	}

	return &models.User{
		ID:        userID,
		Name:      nullStringToPtr(name),
		Gender:    nullStringToGenderPtr(gender),
		Age:       nullInt64ToIntPtr(age),
		Info:      nullStringToPtr(info),
		Phone:     phone,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}

// nullStringToPtr — NULL → nil, заполненная строка → указатель на копию.
// Локальная копия (для package-level state-share не годится — телефонная
// модель использует разные мапперы по типам колонок).
func nullStringToPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}

	return &ns.String
}

// nullStringToGenderPtr — для users.gender (TEXT с CHECK enum'ом). NULL даёт
// nil, заполненное значение — указатель на models.Gender (named-type над
// string), что сохраняет типизацию на уровне модели.
func nullStringToGenderPtr(ns sql.NullString) *models.Gender {
	if !ns.Valid {
		return nil
	}

	var g models.Gender = models.Gender(ns.String)

	return &g
}

// nullInt64ToIntPtr — для users.age (INTEGER 1..120). NULL даёт nil,
// заполненное значение — указатель на int (домен `models.User.Age` — int,
// не int64).
func nullInt64ToIntPtr(ni sql.NullInt64) *int {
	if !ni.Valid {
		return nil
	}

	var v int = int(ni.Int64)

	return &v
}
