// Package models содержит доменные структуры приложения, которые
// переиспользуются несколькими слоями (репозитории, хендлеры, сервисы).
// Тут живут только дата-классы — без зависимостей от БД-драйверов и логики.
package models

import "time"

// TelegramUser — доменное представление строки таблицы telegram_users.
// Опциональные поля — *string, чтобы различать «нет значения» (nil) и
// «пустая строка» (*s == ""). Структура не завязана на тип БД-драйвера
// (sql.NullString сюда не течёт).
type TelegramUser struct {
	ID             int64
	TelegramUserID int64
	FirstName      string
	LastName       *string
	Username       *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
