package main

import (
	"fmt"
	"time"

	"github.com/bklv-kirill/asker/internal/config"
)

func main() {
	cfg := config.Load()

	fmt.Printf("starting %s (%s)\n", cfg.AppName, cfg.BotName)

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		fmt.Println("working...")
	}
}
