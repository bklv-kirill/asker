// Package models содержит доменные структуры приложения, которые
// переиспользуются несколькими слоями (репозитории, хендлеры, сервисы).
// Тут живут только дата-классы — без зависимостей от БД-драйверов и логики.
package models

import "time"

// TelegramUser — доменное представление строки таблицы telegram_users.
// Опциональные поля — указатели, чтобы различать «нет значения» (nil) и
// «пустое значение» (например, *s == ""). Структура не завязана на типы
// БД-драйвера (sql.NullString / sql.NullInt64 сюда не течёт).
//
// UserID — привязка к доменному users(id). NULL до момента, когда
// пользователь привязал номер телефона; после привязки — внутренний id из
// users.
type TelegramUser struct {
	ID             int64
	UserID         *int64
	TelegramUserID int64
	FirstName      string
	LastName       *string
	Username       *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// TelegramUserCreate — DTO для метода Create репозитория. Несёт ровно
// те поля, которые задаёт вызывающий при появлении нового TG-аккаунта:
// ID, CreatedAt, UpdatedAt проставляет БД и здесь их нет; UserID
// (привязка к доменному users) тоже не задаётся при создании — он
// проставляется отдельным методом SetUserIDByTelegramUserID после
// того, как пользователь привязал номер телефона. Опциональные поля —
// указатели (nil = NULL в БД).
type TelegramUserCreate struct {
	TelegramUserID int64
	FirstName      string
	LastName       *string
	Username       *string
}
