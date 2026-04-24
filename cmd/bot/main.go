package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/bklv-kirill/asker/internal/config"
	telegramUsersRepo "github.com/bklv-kirill/asker/internal/repository/telegram_users"
	usersRepo "github.com/bklv-kirill/asker/internal/repository/users"
	"github.com/bklv-kirill/asker/internal/storage/sqlite"
	"github.com/bklv-kirill/asker/internal/telegram"
)

func main() {
	cfg := config.Load()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logger.Info("starting", "app", cfg.AppName, "bot", cfg.BotName)

	db := sqlite.New(cfg, logger)
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("sqlite close", "err", err)
		}
	}()

	users := usersRepo.NewUsersSQLiteRepo(db)
	telegramUsers := telegramUsersRepo.NewTelegramUsersSQLiteRepo(db)

	tg := telegram.NewTelegramBot(cfg.TokenBotToken, cfg.BotName, logger, users, telegramUsers)
	if err := tg.Start(ctx); err != nil {
		logger.Error("telegram start", "err", err)
		os.Exit(1)
	}

	logger.Info("shutdown complete")
}
