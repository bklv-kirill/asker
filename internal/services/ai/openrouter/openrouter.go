// Package openrouter — реализация ai.LLM поверх OpenRouter
// (https://openrouter.ai), OpenAI-совместимый chat-completions API.
//
// Провайдер делает один синхронный POST-запрос на /chat/completions,
// без streaming, retry и tool-use (см. SPEC: явно отвергнуто как YAGNI).
// Системный промпт фиксируется на момент конструирования — main.go читает
// файл по AI_SYSTEM_PROMPT_PATH и передаёт строку в NewOpenRouter; во время
// вызова Prompt(ctx, ...) системный промпт идёт первым {role:"system"}-сообщением,
// дальше один {role:"user"} с готовым ai.Prompt.
package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bklv-kirill/asker/internal/services/ai"
)

const (
	providerName    = "openrouter"
	apiURL          = "https://openrouter.ai/api/v1/chat/completions"
	roleSystem      = "system"
	roleUser        = "user"
	contentTypeJSON = "application/json"
)

var (
	ErrRequest = errors.New("openrouter: request")
	ErrStatus  = errors.New("openrouter: status")
	ErrParse   = errors.New("openrouter: parse")
	ErrEmpty   = errors.New("openrouter: empty response")
)

type openrouter struct {
	apiKey       string
	model        string
	systemPrompt ai.SystemPrompt
	timeout      time.Duration
	httpClient   *http.Client
}

// NewOpenRouter конструирует реализацию ai.LLM для OpenRouter.
//
// apiKey — ключ из AI_API_KEY; model — идентификатор модели OpenRouter
// (напр. "anthropic/claude-sonnet-4-5", "qwen/qwen-plus"); timeout —
// дедлайн одного запроса (передаётся именно time.Duration, а не секунды,
// чтобы провайдер не знал про юнит); systemPrompt — готовая строка
// системного промпта, main.go читает её из файла по AI_SYSTEM_PROMPT_PATH.
// Пустой systemPrompt валиден — system-сообщение просто не отправляется
// (полезно для отладки).
//
// http.Client без своего Timeout: дедлайн уважается через context.WithTimeout
// внутри Prompt(ctx, ...), что позволяет аккуратно прервать и парсинг тела ответа.
func NewOpenRouter(apiKey, model string, systemPrompt ai.SystemPrompt, timeout time.Duration) ai.LLM {
	return &openrouter{
		apiKey:       apiKey,
		model:        model,
		systemPrompt: systemPrompt,
		timeout:      timeout,
		httpClient:   &http.Client{},
	}
}

func (o *openrouter) GetInfo() ai.Info {
	return ai.Info{
		Provider: providerName,
		Model:    o.model,
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatResponse struct {
	Choices []chatResponseChoice `json:"choices"`
}

type chatResponseChoice struct {
	Message chatMessage `json:"message"`
}

func (o *openrouter) Prompt(ctx context.Context, prompt ai.Prompt) (string, error) {
	var reqCtx context.Context
	var cancel context.CancelFunc
	reqCtx, cancel = context.WithTimeout(ctx, o.timeout)
	defer cancel()

	var messages []chatMessage = make([]chatMessage, 0, 2)
	if o.systemPrompt != "" {
		messages = append(messages, chatMessage{
			Role:    roleSystem,
			Content: string(o.systemPrompt),
		})
	}
	messages = append(messages, chatMessage{
		Role:    roleUser,
		Content: string(prompt),
	})

	var payload chatRequest = chatRequest{
		Model:    o.model,
		Messages: messages,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", errors.Join(ErrRequest, err)
	}

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return "", errors.Join(ErrRequest, err)
	}
	req.Header.Set("Authorization", "Bearer "+o.apiKey)
	req.Header.Set("Content-Type", contentTypeJSON)
	req.Header.Set("Accept", contentTypeJSON)

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return "", errors.Join(ErrRequest, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Join(ErrRequest, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", errors.Join(ErrStatus, fmt.Errorf("status=%d body=%s", resp.StatusCode, string(respBody)))
	}

	var parsed chatResponse
	err = json.Unmarshal(respBody, &parsed)
	if err != nil {
		return "", errors.Join(ErrParse, err)
	}

	if len(parsed.Choices) == 0 || parsed.Choices[0].Message.Content == "" {
		return "", ErrEmpty
	}

	return parsed.Choices[0].Message.Content, nil
}
