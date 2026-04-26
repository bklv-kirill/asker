package models

import (
	"encoding/json"
	"time"
)

// TelegramEvent — доменное представление строки таблицы telegram_events:
// журнальная запись об одном событии Telegram-бота (входящее сообщение,
// команда, callback от inline-кнопки или исходящее сообщение бота).
// Запись иммутабельна — поля updated_at нет.
//
// Payload — произвольный JSON: тип события и все детали (text, chat_id,
// message_id, callback data и т.п.) лежат внутри. Тип json.RawMessage
// несёт семантику «это уже сериализованный валидный JSON» и не требует
// повторного маршаллинга при отдаче наружу.
type TelegramEvent struct {
	ID             int64
	TelegramUserID int64
	Payload        json.RawMessage
	CreatedAt      time.Time
}

// TelegramEventCreate — DTO для метода Create репозитория. Несёт ровно
// те поля, которые задаёт вызывающий: ID и CreatedAt проставляет БД и
// в Create-структуре отсутствуют. Передаётся по значению; id созданной
// строки возвращается отдельным значением через `Create(...) (int64, error)`.
type TelegramEventCreate struct {
	TelegramUserID int64
	Payload        json.RawMessage
}
