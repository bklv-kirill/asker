// Package claudeCodeCLI — реализация ai.LLM поверх локального Claude Code CLI
// (бинарник `claude` в PATH контейнера). Провайдер вызывает CLI как подпроцесс
// в режиме `claude -p --output-format json` и парсит поле `result` из JSON-ответа.
// API-ключ не нужен — авторизация наследуется из OAuth-кредов CLI
// (хостовый watcher синкает `.credentials.json` в bind-mount, см. CLAUDE.md).
//
// Системный промпт фиксируется на момент конструирования: main.go читает файл
// по AI_SYSTEM_PROMPT_PATH и передаёт строку в NewClaudeCodeCLI; на каждом
// вызове Prompt он уходит в CLI через флаг --system-prompt (заменяет дефолтный).
//
// Конкуренция: каждый вызов Prompt форкает Node.js-процесс claude (~80–150 MB
// RAM, cold start ~0.5–1.5s). Чтобы случайный всплеск (юзер прислал 100
// сообщений подряд) не привёл к fork-bomb, активные подпроцессы ограничены
// семафором (maxConcurrent). Юзеры сверх лимита ждут на канале, дисциплина —
// FIFO Go runtime; ctx.Done() прерывает ожидание (таймаут хендлера срабатывает).
//
// Намеренно НЕ передаём --bare: этот флаг отключает OAuth-чтение и требует
// ANTHROPIC_API_KEY (см. `claude --help` про --bare: «Anthropic auth is strictly
// ANTHROPIC_API_KEY or apiKeyHelper via --settings (OAuth and keychain are
// never read)»). Авторизация у нас идёт через OAuth из `.credentials.json`,
// поэтому --bare применять нельзя — CLI вернёт «Not logged in».
//
// Permissions: --tools задаёт whitelist доступных инструментов (только web —
// Bash/Edit/Read/Write боту не нужны), --allowedTools auto-approve'ит вызовы
// без интерактивного диалога. --permission-mode bypassPermissions не подходит:
// контейнер бежит под root, а CLI блокирует bypass с root/sudo по соображениям
// безопасности. Точечный --allowedTools обходит ограничение, не давая полный
// bypass (сам факт того что в whitelist'е только WebSearch/WebFetch — наша
// граница безопасности).
package claudeCodeCLI

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"time"

	"github.com/bklv-kirill/asker/internal/services/ai"
)

const (
	providerName = "claude-code-cli"

	// maxConcurrent — потолок параллельных подпроцессов claude. На каждый
	// активный запрос форкается Node.js-процесс; лимит защищает от случайных
	// всплесков (50+ юзеров одновременно). N=5 — компромисс: очередь почти
	// не возникает на нормальной нагрузке, RAM-потолок ~750 MB.
	maxConcurrent = 5

	// outputFormatJSON — режим, в котором CLI печатает один JSON-объект
	// со структурой {type, subtype, is_error, result, ...}. Текст ответа —
	// поле result; при is_error=true провайдер считает ответ ошибкой.
	outputFormatJSON = "json"
)

var (
	ErrAcquire = errors.New("claude-code-cli: acquire slot")
	ErrRun     = errors.New("claude-code-cli: run process")
	ErrParse   = errors.New("claude-code-cli: parse output")
	ErrAPI     = errors.New("claude-code-cli: api error")
	ErrEmpty   = errors.New("claude-code-cli: empty response")
)

type claudeCodeCLI struct {
	model        string
	systemPrompt ai.SystemPrompt
	timeout      time.Duration
	sem          chan struct{}
}

// NewClaudeCodeCLI конструирует реализацию ai.LLM поверх Claude Code CLI.
//
// model — полное имя модели (напр. "claude-sonnet-4-6", "claude-haiku-4-5"),
// уходит в CLI через флаг --model. systemPrompt — готовая строка; пустая
// валидна (флаг --system-prompt не передаётся, CLI использует дефолтный
// промпт). timeout — дедлайн одного вызова; обязательный (>0), уважается
// через context.WithTimeout внутри Prompt и через exec.CommandContext.
func NewClaudeCodeCLI(model string, systemPrompt ai.SystemPrompt, timeout time.Duration) ai.LLM {
	return &claudeCodeCLI{
		model:        model,
		systemPrompt: systemPrompt,
		timeout:      timeout,
		sem:          make(chan struct{}, maxConcurrent),
	}
}

func (c *claudeCodeCLI) GetInfo() ai.Info {
	return ai.Info{
		Provider: providerName,
		Model:    c.model,
	}
}

type cliResponse struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
	IsError bool   `json:"is_error"`
	Result  string `json:"result"`
}

func (c *claudeCodeCLI) Prompt(ctx context.Context, prompt ai.Prompt) (string, error) {
	var reqCtx context.Context
	var cancel context.CancelFunc
	reqCtx, cancel = context.WithTimeout(ctx, c.timeout)
	defer cancel()

	select {
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
	case <-reqCtx.Done():
		return "", errors.Join(ErrAcquire, reqCtx.Err())
	}

	var args []string = []string{
		"-p",
		"--output-format", outputFormatJSON,
		"--model", c.model,
		"--tools", "WebSearch,WebFetch",
		"--allowedTools", "WebSearch,WebFetch",
		"--no-session-persistence",
	}
	if c.systemPrompt != "" {
		args = append(args, "--system-prompt", string(c.systemPrompt))
	}

	var cmd *exec.Cmd = exec.CommandContext(reqCtx, "claude", args...)
	cmd.Stdin = bytes.NewReader([]byte(prompt))

	// Setpgid: true изолирует подпроцесс claude в собственной process group.
	// Без этого signal-каскад от родительского pgroup (air шлёт SIGINT по
	// process group Go-бинарника при graceful shutdown) задевает и claude,
	// он валится с is_error=true / error_during_execution, и юзер видит
	// «не получилось получить ответ» вместо реального LLM-ответа. С Setpgid
	// сигнал от air не достаёт ребёнка; отмена по reqCtx (derived от
	// processCtx, переживающего SIGINT) продолжает работать через
	// exec.CommandContext → cmd.Process.Kill() — оно ходит по PID, не pgroup.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()

	var parsed cliResponse
	parseErr := json.Unmarshal(stdout.Bytes(), &parsed)
	if parseErr != nil {
		if runErr != nil {
			return "", errors.Join(ErrRun, fmt.Errorf("stderr=%s stdout=%s: %w", stderr.String(), stdout.String(), runErr))
		}

		return "", errors.Join(ErrParse, parseErr)
	}

	if parsed.IsError {
		return "", errors.Join(ErrAPI, fmt.Errorf("subtype=%s result=%s", parsed.Subtype, parsed.Result))
	}

	if runErr != nil {
		return "", errors.Join(ErrRun, fmt.Errorf("stderr=%s: %w", stderr.String(), runErr))
	}

	if parsed.Result == "" {
		return "", ErrEmpty
	}

	return parsed.Result, nil
}
