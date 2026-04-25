package models

import "time"

// Gender — типизированное значение колонки users.gender. Named-type над
// string: для логирования/маршалинга работает как обычная строка, но в
// сигнатурах исключает случайную передачу произвольного string'а.
// Допустимые значения — константы GenderMale / GenderFemale, они же
// перечислены в CHECK схемы (см. migrations/0001_users.up.sql).
type Gender string

const (
	GenderMale   Gender = "мужчина"
	GenderFemale Gender = "женщина"
)

// User — доменное представление строки таблицы users (см.
// migrations/0001_users.up.sql). Опциональные поля — указатели, чтобы
// различать «нет значения» (nil) и «пустое значение». Phone хранится
// нормализованным (только цифры) — это требование CHECK на уровне БД.
type User struct {
	ID        int64
	Name      *string
	Gender    *Gender
	Age       *int
	Info      *string
	Phone     string
	CreatedAt time.Time
	UpdatedAt time.Time
}
