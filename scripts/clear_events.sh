#!/usr/bin/env bash
#
# clear_events.sh — удаляет все telegram_events для одного пользователя
# по users.id. Записи users, telegram_users и chat_messages не трогаются.
#
# events привязаны к telegram_users.id, а не к users.id напрямую: скрипт
# сначала резолвит telegram_users.id через telegram_users.user_id = $USER_ID,
# а потом удаляет события по telegram_user_id.
#
# Использование:
#   ./scripts/clear_events.sh <users.id>
#
# Пример:
#   ./scripts/clear_events.sh 1

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

DB_FILE="./data/asker.db"

if [[ $# -ne 1 ]]; then
    echo "usage: $0 <users.id>" >&2
    exit 1
fi

USER_ID="$1"

if ! [[ "$USER_ID" =~ ^[0-9]+$ ]]; then
    echo "clear_events: users.id должен быть числом, получено: $USER_ID" >&2
    exit 1
fi

if ! command -v sqlite3 >/dev/null 2>&1; then
    echo "clear_events: sqlite3 не найден в PATH — установи 'apt install sqlite3'" >&2
    exit 1
fi

if [[ ! -f "$DB_FILE" ]]; then
    echo "clear_events: база $DB_FILE не найдена" >&2
    exit 1
fi

echo "clear_events: проверяю users.id=$USER_ID"

USER_ROW="$(sqlite3 -separator '|' "$DB_FILE" \
    "SELECT COALESCE(name, ''), phone FROM users WHERE id = $USER_ID;")"

if [[ -z "$USER_ROW" ]]; then
    echo "clear_events: пользователя с id=$USER_ID нет"
    exit 0
fi

IFS='|' read -r USER_NAME USER_PHONE <<< "$USER_ROW"

echo "  users.name  = ${USER_NAME:-<пусто>}"
echo "  users.phone = $USER_PHONE"

TG_ROW="$(sqlite3 -separator '|' "$DB_FILE" \
    "SELECT id, telegram_user_id, first_name, COALESCE(username, '')
     FROM telegram_users WHERE user_id = $USER_ID;")"

if [[ -z "$TG_ROW" ]]; then
    echo "clear_events: к users.id=$USER_ID не привязан ни один TG-аккаунт — событий нет"
    exit 0
fi

IFS='|' read -r TG_ROW_ID TG_USER_ID FIRST_NAME USERNAME <<< "$TG_ROW"

echo "  telegram_users.id   = $TG_ROW_ID"
echo "  telegram_user_id    = $TG_USER_ID"
echo "  first_name/username = $FIRST_NAME / @${USERNAME:-—}"

EVENT_COUNT="$(sqlite3 "$DB_FILE" \
    "SELECT COUNT(*) FROM telegram_events WHERE telegram_user_id = $TG_ROW_ID;")"
echo "  telegram_events     = $EVENT_COUNT"

if [[ "$EVENT_COUNT" == "0" ]]; then
    echo "clear_events: событий нет — нечего удалять"
    exit 0
fi

read -rp "clear_events: подтверди удаление событий (yes/no): " CONFIRM
if [[ "$CONFIRM" != "yes" ]]; then
    echo "clear_events: отменено"
    exit 0
fi

sqlite3 "$DB_FILE" <<SQL
PRAGMA foreign_keys = ON;
DELETE FROM telegram_events WHERE telegram_user_id = $TG_ROW_ID;
SQL

echo "clear_events: готово, удалено $EVENT_COUNT записей"
