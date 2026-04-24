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
}

// Load читает .env (если существует) и переменные окружения, собирает Config.
// Переменные окружения имеют приоритет над значениями из .env.
func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigFile(".env")
	v.SetConfigType("env")
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read .env: %w", err)
	}

	// BindEnv нужен, чтобы Unmarshal подхватывал переменные окружения,
	// которых нет в .env — без него AutomaticEnv срабатывает только для Get*.
	for _, key := range []string{"APP_NAME", "BOT_NAME", "TOKEN_BOT_TOKEN"} {
		if err := v.BindEnv(key); err != nil {
			return nil, fmt.Errorf("bind env %s: %w", key, err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return &cfg, nil
}
