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
}

// Load читает .env (если существует) и переменные окружения, собирает Config.
// Переменные окружения имеют приоритет над значениями из .env.
// При любой ошибке загрузки — panic: дальнейшее выполнение без конфига не имеет смысла.
func Load() *Config {
	v := viper.New()
	v.SetConfigFile(".env")
	v.SetConfigType("env")
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil && !errors.Is(err, os.ErrNotExist) {
		panic(fmt.Errorf("config: read .env: %w", err))
	}

	// BindEnv нужен, чтобы Unmarshal подхватывал переменные окружения,
	// которых нет в .env — без него AutomaticEnv срабатывает только для Get*.
	for _, key := range []string{"APP_NAME", "BOT_NAME", "TOKEN_BOT_TOKEN", "DB_PATH"} {
		if err := v.BindEnv(key); err != nil {
			panic(fmt.Errorf("config: bind env %s: %w", key, err))
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		panic(fmt.Errorf("config: unmarshal: %w", err))
	}

	requireNonEmpty("APP_NAME", cfg.AppName)
	requireNonEmpty("BOT_NAME", cfg.BotName)
	requireNonEmpty("TOKEN_BOT_TOKEN", cfg.TokenBotToken)
	requireNonEmpty("DB_PATH", cfg.DBPath)

	return &cfg
}

func requireNonEmpty(key, value string) {
	if value == "" {
		panic(fmt.Errorf("config: %s is required but empty", key))
	}
}
