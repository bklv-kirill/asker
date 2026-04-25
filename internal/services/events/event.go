// Package events содержит инфраструктуру event-listener: общие интерфейсы
// доменного события (Event), обработчика (Listener) и диспетчера
// (Dispatcher). Подписка — name-based: каждый listener привязан к
// строковому имени события (например, "user_created"); маршрутизация в
// Dispatch — по этому же имени из event.GetName().
//
// Конкретные доменные события и их listener'ы живут рядом с доменом,
// который их порождает (например, internal/services/profile/events.go) —
// этот пакет описывает только инфраструктуру.
package events

import "context"

// Event — общий интерфейс доменных событий. GetName() возвращает строковый
// идентификатор события, по которому Dispatcher маршрутизирует подписчиков.
// Имя должно быть стабильным и уникальным в рамках приложения; рекомендуется
// объявлять его константой пакета вместе с типом события (см. примеры в
// internal/services/profile/events.go).
type Event interface {
	GetName() string
}

// Listener — обработчик события. Получает общий Event и сам делает
// type-assertion внутри Handle на конкретный тип, на который подписан.
// Возвращаемая ошибка логируется диспетчером (см. Dispatcher.Dispatch);
// вызывающего сценария Dispatch'а ошибка не достигает — он fire-and-forget.
type Listener interface {
	Handle(ctx context.Context, event Event) error
}

// Dispatcher — шина событий. Subscribe регистрирует listener на имя
// события; Dispatch асинхронно вызывает всех подписанных listener'ов.
// Multi-cast: на одно имя — произвольное число listener'ов. Async:
// каждый listener в своей горутине, Dispatch fire-and-forget.
//
// Реализация (см. dispatcher.go) потокобезопасна: Subscribe и Dispatch
// можно вызывать из разных горутин одновременно.
type Dispatcher interface {
	Subscribe(eventName string, listener Listener)
	Dispatch(ctx context.Context, event Event)
}
