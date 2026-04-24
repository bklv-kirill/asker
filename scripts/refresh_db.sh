#!/usr/bin/env bash
# refresh_db.sh — пересоздаёт SQLite-базу бота на хосте.
# Останавливает контейнер, чтобы он не держал открытым ./data/asker.db;
# прогоняет все migrations/*.down.sql в обратном порядке;
# потом все migrations/*.up.sql в прямом порядке; поднимает контейнер заново.
# Down-миграции написаны идемпотентно (DROP ... IF EXISTS) — первый прогон
# на чистой БД не падает.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

DB_FILE="./data/asker.db"
MIGRATIONS_DIR="./migrations"

if ! command -v sqlite3 >/dev/null 2>&1; then
    echo "refresh_db: sqlite3 не найден в PATH — установи 'apt install sqlite3'" >&2
    exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
    echo "refresh_db: docker не найден в PATH" >&2
    exit 1
fi

mkdir -p "$(dirname "$DB_FILE")"

echo "refresh_db: stopping bot container (no-op если не запущен)"
docker compose stop app >/dev/null

shopt -s nullglob

down_files=( "$MIGRATIONS_DIR"/*.down.sql )
up_files=( "$MIGRATIONS_DIR"/*.up.sql )

if [[ ${#up_files[@]} -eq 0 ]]; then
    echo "refresh_db: в $MIGRATIONS_DIR нет *.up.sql — нечего применять" >&2
    exit 1
fi

echo "refresh_db: applying down migrations (reverse order)"
# Разворачиваем массив вручную — `sort -r` мог бы сломаться на путях с пробелами.
for (( i=${#down_files[@]}-1; i>=0; i-- )); do
    f="${down_files[$i]}"
    echo "  ← $(basename "$f")"
    sqlite3 "$DB_FILE" < "$f"
done

echo "refresh_db: applying up migrations (forward order)"
for f in "${up_files[@]}"; do
    echo "  → $(basename "$f")"
    sqlite3 "$DB_FILE" < "$f"
done

echo "refresh_db: starting bot container"
docker compose start app >/dev/null

echo "refresh_db: done"
