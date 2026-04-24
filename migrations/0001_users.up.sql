-- 0001_users.up.sql
-- Базовая, платформо-независимая таблица пользователей. Хранит доменные данные
-- (имя, пол, возраст), без какой-либо привязки к конкретной платформе (TG/VK/Max).
-- Привязки к платформам лежат в отдельных таблицах-мостах (например, telegram_users).

CREATE TABLE IF NOT EXISTS users (
    id          INTEGER  PRIMARY KEY AUTOINCREMENT,
    -- Имя пользователя (как он себя назвал в боте). Опциональное: на момент
    -- создания записи имя может быть ещё не собрано.
    name        TEXT,
    -- Пол как enum: 'мужчина' / 'женщина'. SQLite не имеет настоящего ENUM,
    -- поэтому используется TEXT + CHECK. Nullable: до заполнения профиля пусто.
    gender      TEXT     CHECK (gender IN ('мужчина', 'женщина')),
    -- Возраст в полных годах. Диапазон 1..120 — отсекает явно невалидные значения
    -- на уровне БД, точную валидацию (возраст согласия, возрастные сегменты и т.п.)
    -- делает приложение. Nullable: до заполнения профиля пусто.
    age         INTEGER  CHECK (age BETWEEN 1 AND 120),
    -- Произвольная дополнительная информация о пользователе (заметки, интересы,
    -- пометки от админа и т.п.). Свободный текст без структуры. Если позже
    -- потребуются типизированные поля — выносим в отдельные колонки/таблицы.
    info        TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    -- Проставляется автоматически триггером users_set_updated_at ниже.
    -- Приложение может явно задать своё значение (импорт, миграции данных) —
    -- триггер в этом случае его не перезатрёт.
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- SQLite не умеет ON UPDATE CURRENT_TIMESTAMP на уровне колонки — имитируем
-- триггером. Условие WHEN NEW.updated_at = OLD.updated_at пропускает явно
-- заданные приложением значения и исключает рекурсивный повторный запуск
-- триггера после его собственного UPDATE.
CREATE TRIGGER IF NOT EXISTS users_set_updated_at
AFTER UPDATE ON users
FOR EACH ROW
WHEN NEW.updated_at = OLD.updated_at
BEGIN
    UPDATE users SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
END;
