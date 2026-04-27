#!/usr/bin/env bash
#
# clear_history.sh — удаляет всю историю общения с ассистентом для одного
# пользователя (записи chat_messages по users.id). Сам пользователь и его
# TG-аккаунт не трогаются.
#
# Использование:
#   ./scripts/clear_history.sh <users.id>
#
# Пример:
#   ./scripts/clear_history.sh 1

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
    echo "clear_history: users.id должен быть числом, получено: $USER_ID" >&2
    exit 1
fi

if ! command -v sqlite3 >/dev/null 2>&1; then
    echo "clear_history: sqlite3 не найден в PATH — установи 'apt install sqlite3'" >&2
    exit 1
fi

if [[ ! -f "$DB_FILE" ]]; then
    echo "clear_history: база $DB_FILE не найдена" >&2
    exit 1
fi

echo "clear_history: проверяю users.id=$USER_ID"

USER_ROW="$(sqlite3 -separator '|' "$DB_FILE" \
    "SELECT COALESCE(name, ''), phone FROM users WHERE id = $USER_ID;")"

if [[ -z "$USER_ROW" ]]; then
    echo "clear_history: пользователя с id=$USER_ID нет"
    exit 0
fi

IFS='|' read -r USER_NAME USER_PHONE <<< "$USER_ROW"

echo "  users.name  = ${USER_NAME:-<пусто>}"
echo "  users.phone = $USER_PHONE"

MSG_COUNT="$(sqlite3 "$DB_FILE" \
    "SELECT COUNT(*) FROM chat_messages WHERE user_id = $USER_ID;")"
echo "  chat_messages = $MSG_COUNT"

if [[ "$MSG_COUNT" == "0" ]]; then
    echo "clear_history: истории нет — нечего удалять"
    exit 0
fi

read -rp "clear_history: подтверди удаление истории (yes/no): " CONFIRM
if [[ "$CONFIRM" != "yes" ]]; then
    echo "clear_history: отменено"
    exit 0
fi

sqlite3 "$DB_FILE" <<SQL
PRAGMA foreign_keys = ON;
DELETE FROM chat_messages WHERE user_id = $USER_ID;
SQL

echo "clear_history: готово, удалено $MSG_COUNT записей"
