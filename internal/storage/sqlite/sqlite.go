// Package sqlite открывает соединение с SQLite-файлом по пути из конфига и
// возвращает готовый *sql.DB. Конкретные репозитории (под модели) строятся
// отдельно и принимают *sql.DB в своих конструкторах.
package sqlite

import (
	"database/sql"
	"fmt"
	"log/slog"

	_ "github.com/mattn/go-sqlite3"

	"github.com/bklv-kirill/asker/internal/config"
)

// New открывает SQLite-файл по пути из cfg.DBPath и проверяет соединение.
// При любой ошибке — panic: без валидного хранилища приложение не должно
// стартовать (консистентно с контрактом config.Load). При неудаче ping
// соединение закрывается перед паникой, чтобы не оставлять висящих дескрипторов.
func New(cfg *config.Config, logger *slog.Logger) *sql.DB {
	db, err := sql.Open("sqlite3", cfg.DBPath)
	if err != nil {
		panic(fmt.Errorf("sqlite: open %s: %w", cfg.DBPath, err))
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		panic(fmt.Errorf("sqlite: ping %s: %w", cfg.DBPath, err))
	}

	logger.Info("sqlite opened", "path", cfg.DBPath)

	return db
}
