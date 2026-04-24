-- 0002_telegram_users.down.sql
-- Откат 0002_telegram_users.up.sql. Эта миграция ссылается на users(id) через FK,
-- поэтому в рамках refresh_db.sh она применяется ПЕРВОЙ (до 0001.down) — иначе
-- с включённым PRAGMA foreign_keys зависимости не дадут корректно дропнуть users.
-- IF EXISTS — для идемпотентности первого прогона на чистой БД.

DROP TRIGGER IF EXISTS telegram_users_set_updated_at;
DROP TABLE   IF EXISTS telegram_users;
