-- 0003_telegram_events.down.sql
-- Откат 0003_telegram_events.up.sql. Порядок: индекс → таблица.
-- IF EXISTS — для идемпотентности (refresh_db.sh может прогонять down
-- по чистой БД).

DROP INDEX IF EXISTS idx_telegram_events_user_created;
DROP TABLE IF EXISTS telegram_events;
