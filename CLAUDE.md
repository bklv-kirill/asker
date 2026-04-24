# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Current state

Проект — **Фаза 1 (БАЗА Telegram-бота)**. Source of truth — `SPEC.md` в корне; читай его перед любыми решениями о фичах. Стек зафиксирован:

- **Язык/рантайм:** Go 1.25.
- **Модуль:** `github.com/bklv-kirill/asker`.
- **Упаковка:** docker-compose + собственный dev-Dockerfile на базе `golang:1.25-alpine` с установленным `air` (`github.com/air-verse/air`) для hot reload.
- **Структура:** `cmd/bot/main.go` — точка входа; `internal/config` — загрузка конфига (viper: `.env` + env vars → `Config`); `internal/storage/sqlite` — `New(cfg, logger) (*sql.DB, error)` открывает SQLite-файл и возвращает соединение; `internal/telegram` — `TelegramBot` (клиент `github.com/go-telegram/bot` в long-polling) + отдельные файлы-хендлеры. `main.go` на старте: `config.Load()` → `signal.NotifyContext(SIGINT, SIGTERM)` → `sqlite.New(cfg, logger)` + `defer db.Close()` → `telegram.NewTelegramBot(token, botName, logger).Start(ctx)` (блокирует до отмены контекста).
- **Конфиг:** `github.com/spf13/viper`. Единая структура `config.Config` + `config.Load() *Config`. `.env` опционален (читается если лежит в CWD), env vars имеют приоритет. **При любой ошибке загрузки или пустом required-поле — `panic`**: без валидного конфига приложение не должно стартовать. Все текущие поля (`APP_NAME`, `BOT_NAME`, `TOKEN_BOT_TOKEN`, `DB_PATH`) обязательные. При добавлении новой переменной — править: структуру `Config`, список `BindEnv` и (если требуется) `requireNonEmpty` в `internal/config/config.go`, `.env.example`, таблицу переменных в `SPEC.md`, блок `environment:` в `docker-compose.yaml`.
- **Хранилище:** SQLite через `github.com/mattn/go-sqlite3` (cgo, поэтому в Dockerfile `build-base`) + голый `database/sql` (без ORM/sqlx). Пакет `internal/storage/sqlite`, `New(cfg *config.Config, logger *slog.Logger) *sql.DB` — open + ping, возвращает готовый `*sql.DB`. **При любой ошибке — `panic` через `fmt.Errorf("sqlite: ...: %w", err)`** (консистентно с `config.Load`: без валидного хранилища приложение не должно стартовать). При неудаче ping соединение закрывается перед panic, чтобы не текли дескрипторы. `panic(fmt.Errorf(...))` не нарушает правила «ошибки только через переменные» — правило касается только `return`. Сейчас схему не поднимаем и записи не ведём — база открыта, но холостая. Репозитории под конкретные модели будут отдельными пакетами и будут принимать `*sql.DB` в своих конструкторах (а не `*Storage`-обёртку). Файл БД — bind-mount `./data:/data`, путь задаёт `DB_PATH` (dev: `/data/asker.db`); каталог `./data/` в `.gitignore`.
- **Прокси:** стандартные `HTTP_PROXY`/`HTTPS_PROXY`/`NO_PROXY` пробрасываются в контейнер через `environment:` compose, берутся из `.env`. В `config.Config` они **не входят** — Go-шный `http.DefaultTransport` подхватывает их автоматически. Обязательны на этом хосте (российский прод) — без прокси `bot.New` падает на `getMe: context deadline exceeded`, т.к. `api.telegram.org` заблокирован. Значения для локальной разработки берутся из `~/.bashrc` (Beget EU); в `.env.example` шаблон с пустыми строками — для деплоя вне РФ прокси не нужен.
- **Telegram:** `github.com/go-telegram/bot` (v1.20.0), long-polling. Обработчики регистрируются в `TelegramBot.Start` через `b.RegisterHandler(bot.HandlerTypeMessageText, "<cmd>", bot.MatchTypeExact, <method>)`. Default-хендлер (fallback на всё, что не поймали зарегистрированные) подключается через `bot.WithDefaultHandler(t.handleXxx)` в `bot.New(...)` — сейчас так подключён echo (`handler_echo.go`). **Соглашение: один хендлер — один файл.** Каждая команда — приватный метод `*TelegramBot` в отдельном файле `internal/telegram/handler_<name>.go` (пример: `handler_start.go` → `handleStart`, `handler_echo.go` → `handleEcho`). В `telegram.go` лежит только тип `TelegramBot`, конструктор, `Start` и регистрация. Когда количество хендлеров начнёт делиться на явные домены — переедем на группировку `handler_<domain>_<name>.go` одним рефакторингом. Webhook откладывается до Фазы 4 (prod).
- **Логирование:** stdlib `log/slog`. Root-логгер (`slog.NewTextHandler(os.Stdout, nil)`) создаётся в `main`, передаётся в компоненты третьим аргументом конструктора (`NewTelegramBot(token, botName, logger)`), хранится в поле `logger`. Пакет `log` в проекте не использовать — только `slog`. В `main.go` вместо `log.Fatalf` — `logger.Error(...)` + `os.Exit(1)`. При добавлении нового компонента (бот-handler, HTTP-клиент, хранилище) — принимай `*slog.Logger` в конструкторе, логгер **не** создавай внутри пакета.
- **Без prod-Dockerfile.** Multi-stage под prod появится на Фазе 4.

## Как запустить

```bash
cp .env.example .env     # обязательно — compose интерполирует ${APP_NAME}
# Проставить валидный TOKEN_BOT_TOKEN (получить у @BotFather), иначе config.Load упадёт в panic
docker compose up --build
```

В stdout появляется `time=... level=INFO msg=starting app=Asker bot=Герман` (формат `slog.TextHandler`), затем процесс висит в long-polling. На команду `/start` в чате с ботом бот отвечает `Привет, <FirstName>! Я <BotName>.`. При сохранении `.go` файла `air` сам пересобирает и перезапускает бинарник внутри контейнера — руками ничего не нужно.

Остановить: `docker compose down`. Логи: `docker compose logs -f app`.

## Соглашения скаффолда

- `APP_NAME` из `.env` интерполируется в `container_name: ${APP_NAME}_app`. Без `.env` контейнер будет называться `_app` (невалидно) — поэтому `cp .env.example .env` обязателен перед первым `up`.
- Сеть `app` в compose — **не project-scoped** (`name: app`, `driver: bridge`). Если на хосте уже есть такая сеть — стеки её разделят. Если это нежелательно — перейти на project-scoped имя.
- Volume `go_mod_cache` переживает перезапуски контейнера — кэш `go mod download` не теряется.
- `.air.toml` собирает бинарник в `./tmp/main` из `./cmd/bot`. Каталог `tmp/` в `.gitignore` и `.dockerignore`.
- `.env.example` — шаблон. При добавлении новых переменных окружения держать его в синхроне с реальным `.env` и с `SPEC.md` (таблица переменных).
- MCP-серверы: `.claude/settings.local.json` включает `context7`, `playwright`, `stitch`. Для Go-библиотек и library-docs — `context7`.

## Документация и комментарии — всегда в синхроне с кодом

**Правило проекта (обязательное).** После каждого внесённого изменения, выполненной задачи или в процессе их выполнения — обновлять **ВСЮ** документацию в проекте и комментарии в коде.

Это означает:
- После любой правки кода пройтись по всем затронутым слоям и выровнять: `SPEC.md`, `docs/*.md`, `README.md` (когда появится), `.env.example`, `docker-compose.yaml`, комментарии в изменённых файлах и рядом (если публичный контракт/поведение функции поменялось).
- Если изменение ломает факт, утверждаемый в доке или комментарии — переписать или удалить этот факт, не оставлять drift.
- Если задача состоит из нескольких шагов — обновлять документацию по ходу, а не единым блоком в конце (чтобы репозиторий в любой момент был консистентен).
- В финальном отчёте явно указывать «Статус документации: обновлена / не требуется / есть drift» (как требует `/root/.claude/rules/completion-protocol.md`).
- Это правило действует в дополнение к глобальному «default: no comments» — комментарии всё ещё пишутся только когда объясняют **почему**, но если такой комментарий есть и устарел — его надо привести в соответствие или удалить.

## Git — правила проекта

**Обязательно, переопределяет общие git-привычки.**

1. **Никогда не создавать новые ветки без прямой команды пользователя.** Работать в текущей ветке, даже если кажется, что «логичнее было бы в feature-ветке». Новая ветка — только по явному запросу.
2. **Логические коммиты по ходу задачи.** В процессе выполнения делать коммиты файлов, связанных по логике и смыслу. Не сваливать несвязанные изменения в один коммит и не откладывать всё в финальный «big bang». Один коммит = одна смысловая единица.
3. **Формат сообщения коммита:**
   - Приставка в квадратных скобках, затем `:`, затем название в **kebab-case**.
   - Допустимые приставки:
     - `[ADD]` — добавление нового функционала / файлов
     - `[REFACTOR]` — рефакторинг кода, файлов и т. п.
     - `[DELETE]` — удаление функционала, файлов и т. п.
     - `[MODIFY]` — внесение дополнительного функционала в существующий код / файлы
     - `[FIX]` — исправление чего-либо
   - Пример: `[ADD]:some-feature`
4. **Описание коммита** — краткое, **на русском языке**, только суть изменения. Без даты, автора, `Co-Authored-By`, ссылок на задачи, эмодзи и прочего служебного шума. Только что сделано.

Примеры корректных сообщений:
- `[ADD]:telegram-webhook-handler`
- `[FIX]:empty-bot-token-crash`
- `[REFACTOR]:split-config-loader`
- `[DELETE]:unused-legacy-parser`
- `[MODIFY]:extend-compose-with-redis`

## Ошибки — только через переменные, без `fmt.Errorf` в `return`

**Правило проекта (обязательное).** Все ошибки, которые **возвращаются** из функций/методов, объявляются как переменные на уровне пакета (обычно `var ErrFoo = errors.New("...")` или `var errFoo = errors.New("...")`), и возвращаются эти переменные (или результат `errors.Join(sentinel, underlying)` для сохранения причины). **`fmt.Errorf` в `return` не использовать.**

Зачем:
- Возвращаемые ошибки становятся частью контракта пакета — сравниваются через `errors.Is`, не через подстроку.
- Единый набор sentinel-значений в одном месте проще находить и переиспользовать.
- Сообщение об ошибке не разбросано по сайтам вызова.

Как применять:
- Оборачивание причины — через `errors.Join(sentinel, cause)`, а не `fmt.Errorf("%w: %w", ...)`.
- Экспортируемость (`Err*` vs `err*`) — по обычным правилам Go: если ошибка должна быть видна потребителям пакета для `errors.Is`, экспортируем, иначе — нет.
- `panic(fmt.Errorf(...))` правило **не затрагивает**: panic — это не `return`, там допускается `fmt.Errorf` для быстрого сообщения (как в `internal/config`).
- Логирование через `log.Printf("...: %v", err)` правило тоже не затрагивает — это форматирование для вывода, а не возврат.

## Host context

This repo lives on the shared production server (see `/root/CLAUDE.md`). Nothing here is deployed yet — there is no systemd unit, no nginx vhost, no domain. When deployment time comes, follow `/root/.claude/rules/deploy-checklist.md`.
