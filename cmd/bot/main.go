package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/bklv-kirill/asker/internal/config"
	"github.com/bklv-kirill/asker/internal/telegram"
)

func main() {
	cfg := config.Load()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logger.Info("starting", "app", cfg.AppName, "bot", cfg.BotName)

	tg := telegram.NewTelegramBot(cfg.TokenBotToken, cfg.BotName, logger)
	if err := tg.Start(ctx); err != nil {
		logger.Error("telegram start", "err", err)
		os.Exit(1)
	}

	logger.Info("shutdown complete")
}
