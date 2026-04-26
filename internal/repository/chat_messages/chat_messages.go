// Package chatMessagesRepo содержит контракт и реализации репозитория
// таблицы chat_messages — истории общения пользователя с ИИ-ассистентом
// (очищенный диалог "вопрос пользователя -> ответ ассистента", который
// подаётся в LLM как контекст).
//
// Не путать с telegram_events: там сырой журнал всех типов событий
// (команды, callback'и, контакты), он остаётся для аудита и отладки.
//
// Завязка на users.id (а не на telegram_users.id): ассистент работает
// только для пользователей с привязанным номером телефона; до привязки
// записи в users нет, и история не пишется.
//
// Доменная структура (ChatMessage) живёт в пакете internal/models.
package chatMessagesRepo

import (
	"context"
	"errors"

	"github.com/bklv-kirill/asker/internal/models"
)

// ErrCreate — ошибка вставки строки в chat_messages. Оборачивается
// причиной через errors.Join (правило проекта: fmt.Errorf в return запрещён).
var ErrCreate = errors.New("chat_messages: create")

// ErrGetLast — ошибка чтения последних N сообщений пользователя
// (сбой I/O или сканирования). «Не найдено» отдельным sentinel'ом не
// различается: пустая история — это пустой срез + nil-ошибка.
var ErrGetLast = errors.New("chat_messages: get last")

// Repository — интерфейс доступа к таблице chat_messages. Потребители
// (Telegram-хендлер ассистента, будущие сценарии экспорта истории и т.п.)
// зависят от этого интерфейса, а не от конкретной реализации.
type Repository interface {
	// Create сохраняет одно сообщение диалога и возвращает id созданной
	// строки. Принимает DTO models.ChatMessageCreate по значению — без
	// скрытой мутации структуры вызывающего; id и created_at проставляет
	// БД (AUTOINCREMENT / DEFAULT CURRENT_TIMESTAMP). При сбое I/O —
	// ошибка, обёрнутая ErrCreate.
	Create(ctx context.Context, m models.ChatMessageCreate) (int64, error)

	// GetLast возвращает последние limit сообщений пользователя в
	// хронологическом порядке (oldest-first) — удобно для подачи в LLM
	// как `messages` без дополнительной сортировки на стороне вызывающего.
	// Если истории нет — пустой срез + nil-ошибка. При сбое I/O или
	// сканирования — ошибка, обёрнутая ErrGetLast.
	GetLast(ctx context.Context, userID int64, limit int) ([]models.ChatMessage, error)
}
