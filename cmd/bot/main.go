package main

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/bklv-kirill/asker/internal/config"
	telegramUsersRepo "github.com/bklv-kirill/asker/internal/repository/telegram_users"
	"github.com/bklv-kirill/asker/internal/storage/sqlite"
	"github.com/bklv-kirill/asker/internal/telegram"
)

func main() {
	var cfg *config.Config = config.Load()

	var logger *slog.Logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logger.Info("starting", "app", cfg.AppName, "bot", cfg.BotName)

	var db *sql.DB = sqlite.New(cfg, logger)
	defer func() {
		var err error = db.Close()
		if err != nil {
			logger.Error("sqlite close", "err", err)
		}
	}()

	var telegramUsers telegramUsersRepo.Repository = telegramUsersRepo.NewTelegramUsersSQLiteRepo(db)

	var tg *telegram.TelegramBot = telegram.NewTelegramBot(
		cfg.TokenBotToken,
		cfg.BotName,
		logger,
		telegramUsers,
	)
	var err error = tg.Start(ctx)
	if err != nil {
		logger.Error("telegram start", "err", err)
		os.Exit(1)
	}

	logger.Info("shutdown complete")
}
