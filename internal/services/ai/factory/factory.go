// Package factory собирает конкретную реализацию ai.LLM по идентификатору
// провайдера. Живёт в отдельном подпакете, потому что пакет ai содержит
// контракт LLM, и реализации импортируют его — родитель не может тянуть
// реализации обратно без import cycle. Зависимость направлена правильно:
// factory знает и про ai, и про реализации; ai не знает ни про кого.
package factory

import (
	"fmt"
	"os"
	"time"

	"github.com/bklv-kirill/asker/internal/services/ai"
	"github.com/bklv-kirill/asker/internal/services/ai/claude_code_cli"
	"github.com/bklv-kirill/asker/internal/services/ai/openrouter"
)

const (
	ProviderOpenRouter    = "openrouter"
	ProviderClaudeCodeCLI = "claude-code-cli"
)

// NewLLM собирает реализацию ai.LLM по идентификатору provider. Чтение
// системного промпта (systemPromptPath) — здесь же, чтобы main.go не
// знал про файлы и подкладывал в фабрику только конфиг-значения.
//
// При неизвестном provider или ошибке чтения файла — panic: фабрика
// вызывается на старте, без валидной LLM приложение не должно подниматься
// (консистентно с config.Load и sqlite.New).
func NewLLM(provider, apiKey, model, systemPromptPath string, timeout time.Duration) ai.LLM {
	systemPrompt, err := os.ReadFile(systemPromptPath)
	if err != nil {
		panic(fmt.Errorf("ai: read system prompt %q: %w", systemPromptPath, err))
	}

	switch provider {
	case ProviderOpenRouter:
		return openrouter.NewOpenRouter(apiKey, model, ai.SystemPrompt(systemPrompt), timeout)

	case ProviderClaudeCodeCLI:
		return claudeCodeCLI.NewClaudeCodeCLI(model, ai.SystemPrompt(systemPrompt), timeout)

	default:
		panic(fmt.Errorf("ai: unknown provider %q", provider))
	}
}
