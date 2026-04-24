-- 0001_users.down.sql
-- Откат 0001_users.up.sql. Триггер дропается явно до таблицы (SQLite удаляет
-- связанные триггеры и при DROP TABLE, но явный порядок снимает неоднозначность).
-- IF EXISTS — чтобы down был идемпотентным и не падал на чистой БД.

DROP TRIGGER IF EXISTS users_set_updated_at;
DROP TABLE   IF EXISTS users;
