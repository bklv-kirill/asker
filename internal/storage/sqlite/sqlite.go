// Package sqlite открывает соединение с SQLite-файлом по пути из конфига и
// возвращает готовый *sql.DB. Конкретные репозитории (под модели) строятся
// отдельно и принимают *sql.DB в своих конструкторах.
package sqlite

import (
	"database/sql"
	"errors"
	"log/slog"

	_ "github.com/mattn/go-sqlite3"

	"github.com/bklv-kirill/asker/internal/config"
)

var (
	ErrOpen = errors.New("sqlite: open database")
	ErrPing = errors.New("sqlite: ping database")
)

// New открывает SQLite-файл по пути из cfg.DBPath и проверяет соединение.
// При неудаче ping соединение закрывается до возврата ошибки, чтобы не
// оставлять висящих дескрипторов.
func New(cfg *config.Config, logger *slog.Logger) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", cfg.DBPath)
	if err != nil {
		return nil, errors.Join(ErrOpen, err)
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, errors.Join(ErrPing, err)
	}

	logger.Info("sqlite opened", "path", cfg.DBPath)

	return db, nil
}
