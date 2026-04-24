package main

import (
	"log"
	"time"

	"github.com/bklv-kirill/asker/internal/config"
)

func main() {
	cfg := config.Load()

	log.Printf("starting %s (%s)", cfg.AppName, cfg.BotName)

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		log.Println("working...")
	}
}
