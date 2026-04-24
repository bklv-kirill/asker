-- 0002_telegram_users.up.sql
-- Запись о Telegram-аккаунте. Пока живёт сама по себе: привязки к доменному
-- users нет (будет добавлена отдельной миграцией, когда появится сценарий,
-- где такая связь реально нужна).

CREATE TABLE IF NOT EXISTS telegram_users (
    id                INTEGER  PRIMARY KEY AUTOINCREMENT,
    -- ID аккаунта в Telegram (update.Message.From.ID). Может быть большим — int64
    -- вмещается в SQLite INTEGER. UNIQUE: один TG-аккаунт = одна запись.
    telegram_user_id  INTEGER  NOT NULL UNIQUE,
    -- Имя из профиля TG (tgmodels.User.FirstName). В TG это единственное
    -- обязательное имя — всегда непустое, поэтому NOT NULL.
    first_name        TEXT     NOT NULL,
    -- Фамилия (tgmodels.User.LastName) — у многих не задана, TG отдаёт пусто,
    -- поэтому nullable: хранить именно NULL, а не пустую строку.
    last_name         TEXT,
    -- @username (tgmodels.User.Username) — опциональный и меняется во времени,
    -- identity-полем не является. Identity — только telegram_user_id.
    username          TEXT,
    created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Автоматическое обновление updated_at по триггеру (SQLite не умеет
-- ON UPDATE на уровне колонки). См. 0001_users.up.sql — логика идентична.
CREATE TRIGGER IF NOT EXISTS telegram_users_set_updated_at
AFTER UPDATE ON telegram_users
FOR EACH ROW
WHEN NEW.updated_at = OLD.updated_at
BEGIN
    UPDATE telegram_users SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
END;
