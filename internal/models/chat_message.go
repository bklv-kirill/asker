package models

import "time"

// ChatMessageRole — типизированное значение колонки chat_messages.role.
// Named-type над string: исключает случайную передачу произвольной строки
// в сигнатурах, в логах/маршалинге работает как обычная строка.
// Допустимые значения — константы ChatMessageRoleUser /
// ChatMessageRoleAssistant, они же перечислены в CHECK схемы (см.
// migrations/0004_chat_messages.up.sql). Роль system сюда не пишется:
// системный промпт ассистента живёт в файле, не в БД.
type ChatMessageRole string

const (
	ChatMessageRoleUser      ChatMessageRole = "user"
	ChatMessageRoleAssistant ChatMessageRole = "assistant"
)

// ChatMessage — доменное представление строки таблицы chat_messages
// (см. migrations/0004_chat_messages.up.sql). Завязано на users.id, потому
// что ассистент работает только для пользователей с привязанным номером
// телефона; до привязки доменного users-юзера нет, и история не пишется.
type ChatMessage struct {
	ID        int64
	UserID    int64
	Role      ChatMessageRole
	Content   string
	CreatedAt time.Time
}
