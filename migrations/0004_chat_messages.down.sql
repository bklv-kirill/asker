-- 0004_chat_messages.down.sql
-- Откат 0004_chat_messages.up.sql. Порядок: индекс -> таблица.
-- IF EXISTS — для идемпотентности (refresh_db.sh может прогонять down
-- по чистой БД).

DROP INDEX IF EXISTS idx_chat_messages_user_created;
DROP TABLE IF EXISTS chat_messages;
