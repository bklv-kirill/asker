-- 0004_chat_messages.up.sql
-- История общения пользователя с ИИ-ассистентом. Хранит очищенный диалог
-- "вопрос пользователя -> ответ ассистента", который подаётся в LLM как
-- контекст. Не путать с telegram_events: там сырой журнал всех типов
-- событий (команды, callback'и, контакты), он остаётся для аудита.
--
-- Завязка на users (а не на telegram_users): ассистент работает только
-- для пользователей с привязанным номером телефона. До привязки записи
-- в users нет, поэтому юзер физически не может попасть в этот flow.

CREATE TABLE IF NOT EXISTS chat_messages (
    id         INTEGER  PRIMARY KEY AUTOINCREMENT,
    -- FK на доменного users; ON DELETE CASCADE: удаление пользователя
    -- (например, по запросу на забвение) уносит и его диалог.
    user_id    INTEGER  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- Роль автора сообщения. system сюда не пишем: системный промпт
    -- живёт в файле, не в БД.
    role       TEXT     NOT NULL CHECK (role IN ('user', 'assistant')),
    -- Произвольный текст. Лимита длины нет: длинные ответы LLM шлются
    -- юзеру пачками на стороне Telegram-хендлера, а в БД лежат целиком.
    content    TEXT     NOT NULL,
    -- Секундная гранулярность; порядок внутри секунды гарантирует AUTOINCREMENT id.
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Индекс под основной запрос: «последние N сообщений пользователя».
CREATE INDEX IF NOT EXISTS idx_chat_messages_user_created
    ON chat_messages (user_id, created_at);
