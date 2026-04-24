-- 0002_telegram_users.down.sql
-- Откат 0002_telegram_users.up.sql. Таблица сейчас ни с чем не связана (FK на
-- users убран), поэтому порядок относительно 0001.down неважен — следуем общему
-- правилу refresh_db.sh: обратный относительно up.
-- IF EXISTS — для идемпотентности первого прогона на чистой БД.

DROP TRIGGER IF EXISTS telegram_users_set_updated_at;
DROP TABLE   IF EXISTS telegram_users;
