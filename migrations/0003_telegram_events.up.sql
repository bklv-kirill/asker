-- 0003_telegram_events.up.sql
-- Журнал событий Telegram-бота: входящие сообщения/команды/callback'и от
-- пользователя и исходящие сообщения бота. Записи иммутабельны (журнал) —
-- триггера на updated_at и самого updated_at нет.
--
-- Полиморфизм (owner_type/owner_id) сознательно не вводим: владелец сейчас
-- один — telegram_users; FK даёт целостность из коробки. При появлении
-- второго типа владельца — отдельной миграцией.
--
-- Дедуп по telegram_update_id отложен — добавим, если в реальной работе
-- увидим дубли от long-polling reconnect.

CREATE TABLE IF NOT EXISTS telegram_events (
    id               INTEGER  PRIMARY KEY AUTOINCREMENT,
    -- FK на внутренний id из telegram_users (не Telegram-овский from.ID).
    -- ON DELETE CASCADE: при удалении TG-аккаунта журнал чистится вместе с ним.
    telegram_user_id INTEGER  NOT NULL REFERENCES telegram_users(id) ON DELETE CASCADE,
    -- Произвольный JSON. Тип события (message_in / message_out / command_in /
    -- callback_in) и все детали (text, chat_id, message_id, callback data и т.п.)
    -- лежат внутри. Структура свободная, минимум — поле event с типом.
    payload          TEXT     NOT NULL CHECK (json_valid(payload)),
    -- Секундная гранулярность достаточна: порядок внутри одной секунды
    -- гарантирует AUTOINCREMENT id.
    created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Основной индекс под запрос «история событий пользователя в обратном порядке».
CREATE INDEX IF NOT EXISTS idx_telegram_events_user_created
    ON telegram_events (telegram_user_id, created_at DESC);
