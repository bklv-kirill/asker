// Package telegram инкапсулирует жизненный цикл Telegram-бота: инициализацию клиента,
// регистрацию обработчиков команд и запуск long-polling.
//
// Соглашение: каждый обработчик команды живёт в отдельном файле handler_<name>.go
// (например, handler_start.go) как приватный метод *TelegramBot. Регистрация всех
// обработчиков выполняется в Start. В telegram.go, помимо типа/конструктора/Start,
// живут доменные методы *TelegramBot, переиспользуемые несколькими хендлерами
// (например, CreateNewTelegramUserIfNotExists).
package telegram

import (
	"context"
	"errors"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	telegramUsersRepo "github.com/bklv-kirill/asker/internal/repository/telegram_users"
)

var ErrInitBot = errors.New("telegram: init bot")

type TelegramBot struct {
	token   string
	botName string

	logger *slog.Logger

	telegramUsers telegramUsersRepo.Repository
}

func NewTelegramBot(
	token, botName string,

	logger *slog.Logger,

	telegramUsers telegramUsersRepo.Repository,
) *TelegramBot {
	return &TelegramBot{
		token:   token,
		botName: botName,

		logger: logger,

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

// CreateNewTelegramUserIfNotExists сохраняет запись о TG-аккаунте в telegram_users,
// если её там ещё нет. Идемпотентен: повторный вызов для того же from.ID — no-op.
// Ошибки хранилища логируются и наружу не пробрасываются: вызывающий сценарий
// (хендлер) не должен блокироваться, если БД временно недоступна.
func (t *TelegramBot) CreateNewTelegramUserIfNotExists(ctx context.Context, from *models.User) {
	exists, err := t.telegramUsers.ExistsByTelegramUserID(ctx, from.ID)
	if err != nil {
		t.logger.Error("telegram_users exists check", "err", err, "telegram_user_id", from.ID)

		return
	}
	if exists {
		return
	}

	var lastName *string = optionalString(from.LastName)
	var username *string = optionalString(from.Username)

	_, err = t.telegramUsers.Create(ctx, from.ID, from.FirstName, lastName, username)
	if err != nil {
		t.logger.Error("telegram_users create", "err", err, "telegram_user_id", from.ID)
	}
}

// optionalString отличает «TG не прислал поле» от «прислал, но пусто»: в
// tgmodels.User опциональные поля приходят как пустая строка — маппим её в nil,
// чтобы в БД легла NULL, а не "".
func optionalString(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}
