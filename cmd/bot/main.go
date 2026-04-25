package main

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/bklv-kirill/asker/internal/config"
	telegramEventsRepo "github.com/bklv-kirill/asker/internal/repository/telegram_events"
	telegramUsersRepo "github.com/bklv-kirill/asker/internal/repository/telegram_users"
	usersRepo "github.com/bklv-kirill/asker/internal/repository/users"
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

	var users usersRepo.Repository = usersRepo.NewUsersSQLiteRepo(db)
	var telegramUsers telegramUsersRepo.Repository = telegramUsersRepo.NewTelegramUsersSQLiteRepo(db)
	var telegramEvents telegramEventsRepo.Repository = telegramEventsRepo.NewTelegramEventsSQLiteRepo(db)

	var tg *telegram.TelegramBot = telegram.NewTelegramBot(
		cfg.TokenBotToken,
		cfg.BotName,

		logger,

		users,
		telegramUsers,
		telegramEvents,
	)

	var err error = tg.Start(ctx)
	if err != nil {
		logger.Error("telegram start", "err", err)

		os.Exit(1)
	}

	logger.Info("shutdown complete")
}
