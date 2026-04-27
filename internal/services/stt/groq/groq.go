// Package groq — реализация stt.STT поверх Groq Cloud
// (https://console.groq.com), OpenAI-совместимый эндпоинт
// /openai/v1/audio/transcriptions с моделями семейства Whisper
// (whisper-large-v3, whisper-large-v3-turbo, distil-whisper-large-v3-en).
//
// Провайдер делает один синхронный POST-запрос multipart/form-data,
// без streaming, retry и tool-use. Дедлайн уважается через
// context.WithTimeout внутри Transcribe, что позволяет аккуратно
// прервать и парсинг тела ответа.
package groq

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/bklv-kirill/asker/internal/services/stt"
)

const (
	providerName   = "groq"
	apiURL         = "https://api.groq.com/openai/v1/audio/transcriptions"
	languageRu     = "ru"
	responseFormat = "json"
)

var (
	ErrRequest = errors.New("groq: request")
	ErrStatus  = errors.New("groq: status")
	ErrParse   = errors.New("groq: parse")
)

type groq struct {
	apiKey     string
	model      string
	timeout    time.Duration
	httpClient *http.Client
}

// NewGroq конструирует реализацию stt.STT для Groq Cloud.
//
// apiKey — ключ из STT_API_KEY (берётся в https://console.groq.com/keys);
// model — идентификатор модели Groq (напр. "whisper-large-v3-turbo",
// "whisper-large-v3"); timeout — дедлайн одного запроса (передаётся
// именно time.Duration, а не секунды, чтобы провайдер не знал про юнит).
//
// http.Client без своего Timeout: дедлайн уважается через context.WithTimeout
// внутри Transcribe, что позволяет аккуратно прервать и парсинг тела ответа.
func NewGroq(apiKey, model string, timeout time.Duration) stt.STT {
	return &groq{
		apiKey:     apiKey,
		model:      model,
		timeout:    timeout,
		httpClient: &http.Client{},
	}
}

func (g *groq) GetInfo() stt.Info {
	return stt.Info{
		Provider: providerName,
		Model:    g.model,
	}
}

type transcriptionResponse struct {
	Text string `json:"text"`
}

func (g *groq) Transcribe(ctx context.Context, audio stt.Audio) (string, error) {
	var reqCtx context.Context
	var cancel context.CancelFunc
	reqCtx, cancel = context.WithTimeout(ctx, g.timeout)
	defer cancel()

	body, contentType, err := buildMultipart(audio, g.model)
	if err != nil {
		return "", errors.Join(ErrRequest, err)
	}

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, apiURL, body)
	if err != nil {
		return "", errors.Join(ErrRequest, err)
	}
	req.Header.Set("Authorization", "Bearer "+g.apiKey)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json")

	resp, err := g.httpClient.Do(req)
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

	var parsed transcriptionResponse
	err = json.Unmarshal(respBody, &parsed)
	if err != nil {
		return "", errors.Join(ErrParse, err)
	}

	return parsed.Text, nil
}

// buildMultipart собирает тело запроса в формате multipart/form-data.
// Поля: file (бинарь), model, language, response_format. Имя файла
// обязательно — Groq определяет формат по расширению, без него ответ 400.
func buildMultipart(audio stt.Audio, model string) (*bytes.Buffer, string, error) {
	var buf *bytes.Buffer = &bytes.Buffer{}
	var w *multipart.Writer = multipart.NewWriter(buf)

	fileWriter, err := w.CreateFormFile("file", audio.Filename)
	if err != nil {
		return nil, "", err
	}

	_, err = fileWriter.Write(audio.Bytes)
	if err != nil {
		return nil, "", err
	}

	err = w.WriteField("model", model)
	if err != nil {
		return nil, "", err
	}

	err = w.WriteField("language", languageRu)
	if err != nil {
		return nil, "", err
	}

	err = w.WriteField("response_format", responseFormat)
	if err != nil {
		return nil, "", err
	}

	err = w.Close()
	if err != nil {
		return nil, "", err
	}

	return buf, w.FormDataContentType(), nil
}
