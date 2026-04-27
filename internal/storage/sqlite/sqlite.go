// Package sqlite открывает соединение с SQLite-файлом и возвращает готовый
// *sql.DB. Конкретные репозитории (под модели) строятся отдельно и принимают
// *sql.DB в своих конструкторах.
package sqlite

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

// New открывает SQLite-файл по пути path и проверяет соединение.
// При любой ошибке — panic: без валидного хранилища приложение не должно
// стартовать (консистентно с контрактом config.Load). При неудаче ping
// соединение закрывается перед паникой, чтобы не оставлять висящих дескрипторов.
//
// _foreign_keys=on (DSN-параметр mattn/go-sqlite3) включает PRAGMA
// foreign_keys на каждом новом соединении пула. Без этого FK-ограничения
// (`telegram_events.telegram_user_id → telegram_users(id)`,
// `telegram_users.user_id → users(id)`) висят в схеме как декларация и не
// enforce'ятся в рантайме. Параметр должен ставиться через DSN, а не через
// `db.Exec("PRAGMA foreign_keys = ON")` после Open: PRAGMA в SQLite живёт
// на уровне соединения, а database/sql пулит соединения — Exec затронет
// только одно случайное.
func New(path string) *sql.DB {
	var dsn string = path + "?_foreign_keys=on"

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		panic(fmt.Errorf("sqlite: open %s: %w", path, err))
	}

	err = db.Ping()
	if err != nil {
		_ = db.Close()
		panic(fmt.Errorf("sqlite: ping %s: %w", path, err))
	}

	return db
}
