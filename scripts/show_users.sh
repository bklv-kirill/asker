#!/usr/bin/env bash
# show_users.sh — выводит последние N записей таблицы users в человеко-читаемом
# виде. LEFT JOIN с telegram_users подставляет связанный TG-аккаунт (telegram_user_id,
# first_name, @username) — у пользователя по схеме максимум одна привязка
# (telegram_users.user_id UNIQUE). Длинные/многострочные info обрезаются до 60
# символов и переводы строк заменяются на пробелы, чтобы строки таблицы не
# разваливались.
#
# Использование:
#   ./scripts/show_users.sh         # последние 50 записей (новые сверху)
#   ./scripts/show_users.sh 200     # последние 200 записей
#
# Время выводится в UTC — так оно и хранится в SQLite (CURRENT_TIMESTAMP).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

DB_FILE="./data/asker.db"
LIMIT="${1:-50}"

if ! command -v sqlite3 >/dev/null 2>&1; then
    echo "show_users: sqlite3 не найден в PATH — установи 'apt install sqlite3'" >&2
    exit 1
fi

if [[ ! -f "$DB_FILE" ]]; then
    echo "show_users: файл БД $DB_FILE не найден — запусти ./scripts/refresh_db.sh" >&2
    exit 1
fi

if ! [[ "$LIMIT" =~ ^[0-9]+$ ]]; then
    echo "show_users: limit должен быть положительным числом, получено: $LIMIT" >&2
    exit 1
fi

sqlite3 "$DB_FILE" <<SQL
.headers on
.mode table
SELECT
    u.id                                                                                AS id,
    datetime(u.created_at)                                                              AS at_utc,
    coalesce(u.name, '')                                                                AS name,
    u.phone                                                                             AS phone,
    coalesce(u.gender, '')                                                              AS gender,
    coalesce(cast(u.age AS TEXT), '')                                                   AS age,
    substr(replace(coalesce(u.info, ''), char(10), ' '), 1, 60)                         AS info,
    coalesce(cast(tu.telegram_user_id AS TEXT), '')                                     AS tg_id,
    coalesce(tu.first_name || coalesce(' @' || tu.username, ''), '')                    AS tg_user
FROM users u
LEFT JOIN telegram_users tu ON tu.user_id = u.id
ORDER BY u.id DESC
LIMIT $LIMIT;
SQL
