// Package telegram инкапсулирует жизненный цикл Telegram-бота: инициализацию клиента,
// регистрацию обработчиков команд и запуск long-polling.
package telegram

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

var ErrInitBot = errors.New("telegram: init bot")

type TelegramBot struct {
	token   string
	botName string
}

func NewTelegramBot(token, botName string) *TelegramBot {
	return &TelegramBot{token: token, botName: botName}
}

// Start инициализирует клиента Bot API, регистрирует обработчики и запускает
// long-polling. Блокирует вызывающего, пока ctx не будет отменён.
func (t *TelegramBot) Start(ctx context.Context) error {
	b, err := bot.New(t.token)
	if err != nil {
		return errors.Join(ErrInitBot, err)
	}

	b.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeExact, t.handleStart)

	b.Start(ctx)
	return nil
}

func (t *TelegramBot) handleStart(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	text := fmt.Sprintf("Привет, %s! Я %s.", update.Message.From.FirstName, t.botName)
	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   text,
	}); err != nil {
		log.Printf("telegram: send /start reply: %v", err)
	}
}
