package events

import (
	"context"
	"log/slog"
	"sync"
)

// dispatcher — реализация Dispatcher (паттерн exported interface +
// unexported struct, как в репозиториях проекта). Жизненный цикл
// управляется вызывающим: инстанс создаётся один на приложение и шарится
// между сервисами через DI; подписки регистрируются на старте.
type dispatcher struct {
	mu        sync.RWMutex
	logger    *slog.Logger
	listeners map[string][]Listener
}

// NewDispatcher возвращает готовый Dispatcher с пустой картой подписок.
// Logger используется для записи ошибок и panic'ов listener'ов — Dispatch
// fire-and-forget, и наружу эти ошибки не пробрасываются.
func NewDispatcher(logger *slog.Logger) Dispatcher {
	return &dispatcher{
		logger:    logger,
		listeners: make(map[string][]Listener),
	}
}

func (d *dispatcher) Subscribe(eventName string, listener Listener) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.listeners[eventName] = append(d.listeners[eventName], listener)
}

// Dispatch запускает зарегистрированных listener'ов в отдельных горутинах
// и сразу возвращается. Срез listener'ов копируется под RLock'ом, чтобы
// итерация и горутины не видели изменения мапы из параллельного Subscribe.
func (d *dispatcher) Dispatch(ctx context.Context, event Event) {
	var name string = event.GetName()

	d.mu.RLock()
	var snapshot []Listener = append([]Listener(nil), d.listeners[name]...)
	d.mu.RUnlock()

	for _, listener := range snapshot {
		var l Listener = listener
		go d.run(ctx, event, l)
	}
}

// run — worker для одной горутины listener'а. Defer-recover нужен, чтобы
// panic в listener'е не ронял процесс бота: ошибочный listener изолирован
// в своей горутине.
func (d *dispatcher) run(ctx context.Context, event Event, l Listener) {
	defer func() {
		if r := recover(); r != nil {
			d.logger.Error("event listener panic", "panic", r, "event", event.GetName())
		}
	}()

	var err error = l.Handle(ctx, event)
	if err != nil {
		d.logger.Error("event listener", "err", err, "event", event.GetName())
	}
}
