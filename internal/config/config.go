// Package config загружает конфигурацию приложения из .env и переменных окружения.
package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/viper"
)

type Config struct {
	AppName       string `mapstructure:"APP_NAME"`
	BotName       string `mapstructure:"BOT_NAME"`
	TokenBotToken string `mapstructure:"TOKEN_BOT_TOKEN"`
	DBPath        string `mapstructure:"DB_PATH"`

	// AIEnabled — мастер-выключатель ИИ-ассистента. При false ассистент
	// не инициализируется и default-handler Telegram-бота остаётся как есть
	// (это позволяет локальной разработке работать без API-ключа). При true
	// все остальные AI_* поля становятся required и валидируются на непустоту.
	AIEnabled bool `mapstructure:"AI_ENABLED"`

	// AIProvider — идентификатор реализации llm.Client ("anthropic", "openai",
	// "fake" и т.п.). Сборка конкретного клиента по этой строке — в main.go.
	AIProvider string `mapstructure:"AI_PROVIDER"`

	// AIModel — имя модели у выбранного провайдера (напр. "claude-opus-4-7").
	AIModel string `mapstructure:"AI_MODEL"`

	// AIAPIKey — ключ доступа к LLM-провайдеру. Не логировать.
	AIAPIKey string `mapstructure:"AI_API_KEY"`

	// AISystemPromptPath — путь к файлу с системным промптом ассистента
	// (читается один раз при старте). Bind-mount /prompts/ в контейнер
	// смотрит на ./prompts/ в репозитории.
	AISystemPromptPath string `mapstructure:"AI_SYSTEM_PROMPT_PATH"`

	// AITimeoutSec — таймаут одного запроса к LLM в секундах. Используется
	// для context.WithTimeout в Telegram-хендлере перед вызовом Answer.
	AITimeoutSec int `mapstructure:"AI_TIMEOUT_SEC"`

	// AIHistoryLimit — сколько последних сообщений из chat_messages подавать
	// в LLM как контекст диалога.
	AIHistoryLimit int `mapstructure:"AI_HISTORY_LIMIT"`
}

// Load читает .env (если существует) и переменные окружения, собирает Config.
// Переменные окружения имеют приоритет над значениями из .env.
// При любой ошибке загрузки — panic: дальнейшее выполнение без конфига не имеет смысла.
func Load() *Config {
	var v *viper.Viper = viper.New()
	v.SetConfigFile(".env")
	v.SetConfigType("env")
	v.AutomaticEnv()

	var err error = v.ReadInConfig()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		panic(fmt.Errorf("config: read .env: %w", err))
	}

	// BindEnv нужен, чтобы Unmarshal подхватывал переменные окружения,
	// которых нет в .env — без него AutomaticEnv срабатывает только для Get*.
	for _, key := range []string{
		"APP_NAME", "BOT_NAME", "TOKEN_BOT_TOKEN", "DB_PATH",
		"AI_ENABLED", "AI_PROVIDER", "AI_MODEL", "AI_API_KEY",
		"AI_SYSTEM_PROMPT_PATH", "AI_TIMEOUT_SEC", "AI_HISTORY_LIMIT",
	} {
		err = v.BindEnv(key)
		if err != nil {
			panic(fmt.Errorf("config: bind env %s: %w", key, err))
		}
	}

	var cfg Config
	err = v.Unmarshal(&cfg)
	if err != nil {
		panic(fmt.Errorf("config: unmarshal: %w", err))
	}

	requireNonEmpty("APP_NAME", cfg.AppName)
	requireNonEmpty("BOT_NAME", cfg.BotName)
	requireNonEmpty("TOKEN_BOT_TOKEN", cfg.TokenBotToken)
	requireNonEmpty("DB_PATH", cfg.DBPath)

	// AI_* поля required только при AI_ENABLED=true: иначе локальная
	// разработка без API-ключа невозможна. При выключенном ассистенте
	// поля могут быть пустыми/нулями — main.go их не трогает.
	if cfg.AIEnabled {
		requireNonEmpty("AI_PROVIDER", cfg.AIProvider)
		requireNonEmpty("AI_MODEL", cfg.AIModel)
		requireNonEmpty("AI_API_KEY", cfg.AIAPIKey)
		requireNonEmpty("AI_SYSTEM_PROMPT_PATH", cfg.AISystemPromptPath)
		requirePositive("AI_TIMEOUT_SEC", cfg.AITimeoutSec)
		requirePositive("AI_HISTORY_LIMIT", cfg.AIHistoryLimit)
	}

	return &cfg
}

func requireNonEmpty(key, value string) {
	if value == "" {
		panic(fmt.Errorf("config: %s is required but empty", key))
	}
}

func requirePositive(key string, value int) {
	if value <= 0 {
		panic(fmt.Errorf("config: %s must be > 0, got %d", key, value))
	}
}
