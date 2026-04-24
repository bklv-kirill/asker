// Package telegram инкапсулирует жизненный цикл Telegram-бота: инициализацию клиента,
// регистрацию обработчиков команд и запуск long-polling.
//
// Соглашение: каждый обработчик команды живёт в отдельном файле handler_<name>.go
// (например, handler_start.go) как приватный метод *TelegramBot. Регистрация всех
// обработчиков выполняется в Start.
package telegram

import (
	"context"
	"errors"
	"log/slog"

	"github.com/go-telegram/bot"

	telegramUsersRepo "github.com/bklv-kirill/asker/internal/repository/telegram_users"
	usersRepo "github.com/bklv-kirill/asker/internal/repository/users"
)

var ErrInitBot = errors.New("telegram: init bot")

type TelegramBot struct {
	token   string
	botName string

	logger *slog.Logger

	users         usersRepo.Repository
	telegramUsers telegramUsersRepo.Repository
}

func NewTelegramBot(
	token, botName string,
	logger *slog.Logger,
	users usersRepo.Repository,
	telegramUsers telegramUsersRepo.Repository,
) *TelegramBot {
	return &TelegramBot{
		token:         token,
		botName:       botName,
		logger:        logger,
		users:         users,
		telegramUsers: telegramUsers,
	}
}

// Start инициализирует клиента Bot API, регистрирует обработчики и запускает
// long-polling. Блокирует вызывающего, пока ctx не будет отменён.
func (t *TelegramBot) Start(ctx context.Context) error {
	b, err := bot.New(t.token, bot.WithDefaultHandler(t.handleEcho))
	if err != nil {
		return errors.Join(ErrInitBot, err)
	}

	b.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeExact, t.handleStart)

	b.Start(ctx)

	return nil
}
