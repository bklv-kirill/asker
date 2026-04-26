// Package claudeCodeCLI — реализация ai.LLM поверх локального Claude Code CLI
// (`claude` в PATH). Провайдер вызывает CLI как подпроцесс и парсит ответ;
// API-ключ не требуется, авторизация наследуется из окружения CLI.
//
// Сейчас — заглушка: GetInfo возвращает фиксированный Info, Prompt возвращает
// строку "claude code cli" независимо от входа. Реальный exec.Command + парсинг
// stdout появятся отдельным коммитом.
package claudeCodeCLI

import (
	"context"

	"github.com/bklv-kirill/asker/internal/services/ai"
)

const (
	providerName = "claude-code-cli"
	modelName    = "claude-code-cli"
)

type claudeCodeCLI struct{}

// NewClaudeCodeCLI конструирует реализацию ai.LLM поверх Claude Code CLI.
func NewClaudeCodeCLI() ai.LLM {
	return &claudeCodeCLI{}
}

func (c *claudeCodeCLI) GetInfo() ai.Info {
	return ai.Info{
		Provider: providerName,
		Model:    modelName,
	}
}

func (c *claudeCodeCLI) Prompt(ctx context.Context, prompt ai.Prompt) (string, error) {
	return "claude code cli", nil
}
