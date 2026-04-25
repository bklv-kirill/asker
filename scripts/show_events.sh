#!/usr/bin/env bash
# show_events.sh — выводит последние N записей таблицы telegram_events в
# человеко-читаемом виде. JOIN с telegram_users подставляет first_name и
# @username вместо внутреннего id, ключевые поля payload разворачиваются
# через json_extract в отдельные колонки (event/text).
#
# Использование:
#   ./scripts/show_events.sh         # последние 50 записей (новые сверху)
#   ./scripts/show_events.sh 200     # последние 200 записей
#
# Время выводится в UTC — так оно и хранится в SQLite (CURRENT_TIMESTAMP).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

DB_FILE="./data/asker.db"
LIMIT="${1:-50}"

if ! command -v sqlite3 >/dev/null 2>&1; then
    echo "show_events: sqlite3 не найден в PATH — установи 'apt install sqlite3'" >&2
    exit 1
fi

if [[ ! -f "$DB_FILE" ]]; then
    echo "show_events: файл БД $DB_FILE не найден — запусти ./scripts/refresh_db.sh" >&2
    exit 1
fi

if ! [[ "$LIMIT" =~ ^[0-9]+$ ]]; then
    echo "show_events: limit должен быть положительным числом, получено: $LIMIT" >&2
    exit 1
fi

# В heredoc (<<SQL без кавычек) bash раскрывает $LIMIT, а \$ оставляет
# литеральный $ — он нужен внутри JSON-path аргументов json_extract ('$.event').
sqlite3 "$DB_FILE" <<SQL
.headers on
.mode table
SELECT
    e.id                                                                                  AS id,
    datetime(e.created_at)                                                                AS at_utc,
    u.first_name || coalesce(' @' || u.username, '')                                      AS user,
    json_extract(e.payload, '\$.event')                                                   AS event,
    substr(replace(coalesce(json_extract(e.payload, '\$.text'), ''), char(10), ' '), 1, 80) AS text
FROM telegram_events e
JOIN telegram_users u ON u.id = e.telegram_user_id
ORDER BY e.id DESC
LIMIT $LIMIT;
SQL
