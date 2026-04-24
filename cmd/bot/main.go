package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/bklv-kirill/asker/internal/config"
	"github.com/bklv-kirill/asker/internal/telegram"
)

func main() {
	cfg := config.Load()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	log.Printf("starting %s (%s)", cfg.AppName, cfg.BotName)

	tg := telegram.NewTelegramBot(cfg.TokenBotToken, cfg.BotName)
	if err := tg.Start(ctx); err != nil {
		log.Fatalf("telegram: %v", err)
	}

	log.Printf("shutdown complete")
}
