// Package telegramUsersRepo содержит контракт и реализации репозитория
// таблицы telegram_users — записи о Telegram-аккаунте. Привязки к доменному
// users у таблицы сейчас нет (добавится отдельной миграцией при появлении
// сценария). Интерфейс Repository описывает доступные операции; конкретные
// реализации (SQLite, файловое хранилище и т.п.) живут в отдельных файлах
// этого пакета.
package telegramUsersRepo

import (
	"context"
	"errors"
)

// ErrCreate — ошибка вставки записи в хранилище. Оборачивается причиной
// через errors.Join (правило проекта: fmt.Errorf в return запрещён).
var ErrCreate = errors.New("telegram_users: create")

// ErrExistsByTelegramUserID — ошибка проверки существования записи по
// telegram_user_id. «Не найдено» ошибкой не считается — это валидный false.
var ErrExistsByTelegramUserID = errors.New("telegram_users: exists by telegram_user_id")

// Repository — интерфейс доступа к хранилищу telegram_users. Контракт
// намеренно свободен от деталей конкретной реализации (SQL/файлы/память):
// опциональные поля представлены через *string, где nil = «значения нет».
type Repository interface {
	// Create сохраняет запись о Telegram-аккаунте и возвращает id созданной
	// записи. telegramUserID и firstName — обязательные (в TG API оба всегда
	// присутствуют и непустые). lastName и username опциональны: nil означает,
	// что значение отсутствует (в SQLite-реализации маппится в NULL).
	// Уникальность telegram_user_id гарантирует реализация — повторная вставка
	// того же аккаунта должна приводить к ошибке.
	Create(
		ctx context.Context,
		telegramUserID int64,
		firstName string,
		lastName *string,
		username *string,
	) (int64, error)

	// ExistsByTelegramUserID возвращает true, если запись с указанным
	// telegram_user_id уже есть в хранилище. Отсутствие записи — это false,
	// не ошибка. Ошибка возвращается только при реальном сбое (I/O, SQL и т.п.).
	ExistsByTelegramUserID(ctx context.Context, telegramUserID int64) (bool, error)
}
