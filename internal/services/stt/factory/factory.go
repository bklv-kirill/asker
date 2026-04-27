// Package factory собирает конкретную реализацию stt.STT по идентификатору
// провайдера. Живёт в отдельном подпакете по той же причине, что и
// internal/services/ai/factory: пакет stt содержит контракт STT, и реализации
// импортируют его — родитель не может тянуть реализации обратно без import
// cycle. Зависимость направлена правильно: factory знает и про stt, и про
// реализации; stt не знает ни про кого.
package factory

import (
	"fmt"
	"time"

	"github.com/bklv-kirill/asker/internal/services/stt"
	"github.com/bklv-kirill/asker/internal/services/stt/groq"
)

const (
	ProviderGroq = "groq"
)

// NewSTT собирает реализацию stt.STT по идентификатору provider.
//
// При неизвестном provider — panic: фабрика вызывается на старте, без
// валидной STT-реализации приложение не должно подниматься (консистентно
// с config.Load, sqlite.New и aiFactory.NewLLM).
func NewSTT(provider, apiKey, model string, timeout time.Duration) stt.STT {
	switch provider {
	case ProviderGroq:
		return groq.NewGroq(apiKey, model, timeout)

	default:
		panic(fmt.Errorf("stt: unknown provider %q", provider))
	}
}
