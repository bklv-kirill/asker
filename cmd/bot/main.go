package main

import (
	"fmt"
	"log"
	"time"

	"github.com/bklv-kirill/asker/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	fmt.Printf("starting %s (%s)\n", cfg.AppName, cfg.BotName)

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		fmt.Println("working...")
	}
}
