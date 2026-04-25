// Package usersRepo содержит контракт и реализации репозитория доменной
// таблицы users. Интерфейс Repository описывает доступные операции;
// конкретная реализация поверх SQLite лежит в sqlite.go.
package usersRepo

import (
	"context"
	"errors"

	"github.com/bklv-kirill/asker/internal/models"
)

// ErrCreate — ошибка вставки строки в users. Оборачивается причиной через
// errors.Join (правило проекта: fmt.Errorf в return запрещён).
var ErrCreate = errors.New("users: create")

// ErrPhoneTaken — попытка вставить запись с уже занятым phone (UNIQUE-конфликт
// в БД). Доменно отличается от ErrCreate: вызывающий хендлер показывает
// пользователю «номер занят», а не «попробуй позже». Реализация определяет
// этот случай по типизированной ошибке драйвера (sqlite3.ErrConstraintUnique),
// без string-match.
var ErrPhoneTaken = errors.New("users: phone taken")

// ErrSetGender — ошибка обновления users.gender (сбой I/O или нарушение
// CHECK на уровне БД, если значение не из enum).
var ErrSetGender = errors.New("users: set gender")

// ErrSetAge — ошибка обновления users.age (сбой I/O или нарушение
// CHECK на уровне БД, если значение вне диапазона 1..120).
var ErrSetAge = errors.New("users: set age")

// ErrSetInfo — ошибка обновления users.info (сбой I/O). CHECK'а на колонке
// нет (свободный текст), поэтому валидация пустоты/длины — на стороне
// вызывающего хендлера.
var ErrSetInfo = errors.New("users: set info")

// ErrGetByID — ошибка чтения строки users по id (сбой I/O или сканирования).
// «Не найдено» — отдельный sentinel ErrNotFound, чтобы вызывающий мог
// различать «нет записи» и «БД упала».
var ErrGetByID = errors.New("users: get by id")

// ErrNotFound — запись с указанным id отсутствует. Возвращается в чистом
// виде (без errors.Join), потребитель сравнивает через errors.Is.
var ErrNotFound = errors.New("users: not found")

// Repository — интерфейс доступа к таблице users. Потребители (хендлеры,
// сервисы) зависят от этого интерфейса, а не от конкретной реализации.
type Repository interface {
	// Create вставляет запись с заданным именем и номером телефона и возвращает
	// id созданной строки. phone — обязательный (запись users появляется только
	// после того, как пользователь привязал номер); реализация ожидает уже
	// нормализованную строку из одних цифр (CHECK в схеме отбракует мусор).
	// Остальные поля профиля (gender, age, info) опциональны — на момент создания
	// они NULL, заполняются позже отдельными методами.
	Create(ctx context.Context, name, phone string) (int64, error)

	// SetGender обновляет колонку gender для записи с указанным id. Допустимые
	// значения — константы models.GenderMale / models.GenderFemale
	// (соответствуют CHECK в схеме). Если запись с таким id отсутствует —
	// UPDATE затронет 0 строк, это не считается ошибкой (метод возвращает nil);
	// вызывающий должен проверять существование сам, если важно. При реальном
	// сбое или нарушении CHECK — ошибка, обёрнутая ErrSetGender.
	SetGender(ctx context.Context, id int64, gender models.Gender) error

	// SetAge обновляет колонку age для записи с указанным id. Допустимый
	// диапазон — 1..120 (CHECK в схеме). Валидация — на стороне вызывающего
	// (хендлер парсит ввод пользователя); БД на нарушение CHECK вернёт
	// ошибку, обёрнутую ErrSetAge. Если записи с таким id нет — UPDATE
	// затронет 0 строк, это не считается ошибкой.
	SetAge(ctx context.Context, id int64, age int) error

	// SetInfo обновляет колонку info для записи с указанным id. CHECK'а
	// на колонке нет — info свободный текст; trim/непустоту проверяет
	// вызывающий. Если записи с таким id нет — UPDATE затронет 0 строк,
	// это не считается ошибкой. При сбое I/O — ошибка, обёрнутая ErrSetInfo.
	SetInfo(ctx context.Context, id int64, info string) error

	// GetByID возвращает запись users по внутреннему id. «Не найдено» —
	// ErrNotFound (без обёртки), сбой I/O/сканирования — обёрнутая
	// ErrGetByID. Опциональные колонки (name, gender, age, info) маппятся
	// в указатели — nil означает NULL.
	GetByID(ctx context.Context, id int64) (*models.User, error)
}
