-- 0002_telegram_users.up.sql
-- Запись о Telegram-аккаунте. Привязка к доменному users опциональна: на
-- момент первого общения с ботом TG-юзер появляется тут с user_id = NULL;
-- запись в users создаётся позже, когда пользователь предоставит номер
-- телефона — тогда сюда проставляется user_id.

CREATE TABLE IF NOT EXISTS telegram_users (
    id                INTEGER  PRIMARY KEY AUTOINCREMENT,
    -- Привязка к доменному users. NULL до момента, когда пользователь
    -- привязал номер телефона и для него создалась запись users. UNIQUE:
    -- один доменный пользователь = один TG-аккаунт; SQLite допускает
    -- множество NULL в UNIQUE-колонке, поэтому до привязки строки не
    -- конфликтуют между собой. ON DELETE SET NULL: если запись users
    -- удалена (например, по запросу на забвение) — TG-связь сбрасывается,
    -- но сам telegram_users остаётся (журнал событий не теряется).
    -- Скрипт scripts/delete_user.sh при необходимости полного удаления
    -- сносит привязанный telegram_users явным вторым DELETE — каскадом
    -- через FK схема намеренно этого не делает.
    user_id           INTEGER  UNIQUE REFERENCES users(id) ON DELETE SET NULL,
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
