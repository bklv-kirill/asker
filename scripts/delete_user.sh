#!/usr/bin/env bash
#
# delete_user.sh — полное удаление пользователя по users.id.
#
# Удаляет одной транзакцией:
#   - запись в telegram_users, привязанную к этому users.id (если есть) —
#     каскад на telegram_events
#   - запись в users — каскад на chat_messages
#
# FK telegram_users.user_id → users(id) объявлен как ON DELETE SET NULL
# (схема намеренно не каскадит — журнал orphan-TG не должен теряться при
# удалении доменного пользователя). Поэтому полное удаление делаем двумя
# явными DELETE в транзакции.
#
# FK-каскады включаются явно через PRAGMA foreign_keys=ON: sqlite3-клиент
# по умолчанию открывает соединение с PRAGMA=OFF (в отличие от приложения,
# которое включает FK через DSN-параметр), и без этого DELETE сработает,
# но дочерние строки останутся висеть orphan'ами.
#
# Использование:
#   ./scripts/delete_user.sh <users.id>
#
# Пример:
#   ./scripts/delete_user.sh 1

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
    echo "delete_user: users.id должен быть числом, получено: $USER_ID" >&2
    exit 1
fi

if ! command -v sqlite3 >/dev/null 2>&1; then
    echo "delete_user: sqlite3 не найден в PATH — установи 'apt install sqlite3'" >&2
    exit 1
fi

if [[ ! -f "$DB_FILE" ]]; then
    echo "delete_user: база $DB_FILE не найдена" >&2
    exit 1
fi

echo "delete_user: ищу запись users.id=$USER_ID"

USER_ROW="$(sqlite3 -separator '|' "$DB_FILE" \
    "SELECT COALESCE(name, ''), phone FROM users WHERE id = $USER_ID;")"

if [[ -z "$USER_ROW" ]]; then
    echo "delete_user: пользователя с id=$USER_ID нет — нечего удалять"
    exit 0
fi

IFS='|' read -r USER_NAME USER_PHONE <<< "$USER_ROW"

echo "  users.name  = ${USER_NAME:-<пусто>}"
echo "  users.phone = $USER_PHONE"

MSG_COUNT="$(sqlite3 "$DB_FILE" \
    "SELECT COUNT(*) FROM chat_messages WHERE user_id = $USER_ID;")"
echo "  chat_messages = $MSG_COUNT"

TG_ROW="$(sqlite3 -separator '|' "$DB_FILE" \
    "SELECT id, telegram_user_id, first_name, COALESCE(username, '')
     FROM telegram_users WHERE user_id = $USER_ID;")"

TG_ROW_ID=""
if [[ -n "$TG_ROW" ]]; then
    IFS='|' read -r TG_ROW_ID TG_USER_ID FIRST_NAME USERNAME <<< "$TG_ROW"
    echo "  telegram_users.id   = $TG_ROW_ID"
    echo "  telegram_user_id    = $TG_USER_ID"
    echo "  first_name/username = $FIRST_NAME / @${USERNAME:-—}"

    EVENT_COUNT="$(sqlite3 "$DB_FILE" \
        "SELECT COUNT(*) FROM telegram_events WHERE telegram_user_id = $TG_ROW_ID;")"
    echo "  telegram_events     = $EVENT_COUNT"
else
    echo "  telegram_users      = <не привязан>"
fi

read -rp "delete_user: подтверди удаление (yes/no): " CONFIRM
if [[ "$CONFIRM" != "yes" ]]; then
    echo "delete_user: отменено"
    exit 0
fi

if [[ -n "$TG_ROW_ID" ]]; then
    sqlite3 "$DB_FILE" <<SQL
PRAGMA foreign_keys = ON;
BEGIN;
DELETE FROM telegram_users WHERE id = $TG_ROW_ID;
DELETE FROM users          WHERE id = $USER_ID;
COMMIT;
SQL
else
    sqlite3 "$DB_FILE" <<SQL
PRAGMA foreign_keys = ON;
BEGIN;
DELETE FROM users WHERE id = $USER_ID;
COMMIT;
SQL
fi

echo "delete_user: готово"
