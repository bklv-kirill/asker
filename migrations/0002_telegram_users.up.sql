-- 0002_telegram_users.up.sql
-- Привязка доменного пользователя (users) к его Telegram-аккаунту.
-- Один users.id может иметь максимум одну запись здесь (один TG-аккаунт на юзера);
-- один Telegram-аккаунт принадлежит строго одному users.id (UNIQUE telegram_user_id).

CREATE TABLE IF NOT EXISTS telegram_users (
    id                INTEGER  PRIMARY KEY AUTOINCREMENT,
    -- FK на доменного пользователя. UNIQUE жёстко фиксирует отношение 1-к-1:
    -- у одного users.id не может быть больше одной TG-привязки. CASCADE —
    -- удаление users чистит привязку. UNIQUE автоматически создаёт индекс,
    -- отдельный CREATE INDEX на user_id не нужен.
    user_id           INTEGER  NOT NULL UNIQUE,
    -- ID аккаунта в Telegram (update.Message.From.ID). Может быть большим — int64
    -- вмещается в SQLite INTEGER. UNIQUE: один TG-аккаунт = одна привязка.
    telegram_user_id  INTEGER  NOT NULL UNIQUE,
    created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
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
