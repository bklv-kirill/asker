# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Current state

Проект — **Фаза 1 (БАЗА Telegram-бота)**. Source of truth — `SPEC.md` в корне; читай его перед любыми решениями о фичах. Стек зафиксирован:

- **Язык/рантайм:** Go 1.25.
- **Модуль:** `github.com/bklv-kirill/asker`.
- **Упаковка:** docker-compose + собственный dev-Dockerfile на базе `golang:1.25-alpine` с установленным `air` (`github.com/air-verse/air`) для hot reload.
- **Структура:** `cmd/bot/main.go` — точка входа; `internal/config` — загрузка конфига (viper: `.env` + env vars → `Config`); `internal/storage/sqlite` — `New(path string, logger *slog.Logger) *sql.DB` открывает SQLite-файл и возвращает соединение; `internal/repository/<model>/` — репозитории (пакет `<model>Repo`); `internal/telegram` — `TelegramBot` (клиент `github.com/go-telegram/bot` в long-polling) + отдельные файлы-хендлеры. `main.go` на старте: `config.Load()` → `signal.NotifyContext(SIGINT, SIGTERM)` → `sqlite.New(cfg.DBPath, logger)` + `defer db.Close()` → собирает репозитории (`telegramUsersRepo.NewTelegramUsersSQLiteRepo(db)`, ...) → `telegram.NewTelegramBot(token, botName, logger, telegramUsers, ...).Start(ctx)` (блокирует до отмены контекста). **Внутренние пакеты не импортируют `internal/config`** — `main.go` распаковывает поля `*config.Config` и передаёт конструкторам конкретные значения (`cfg.DBPath`, `cfg.AIAPIKey`, ...); это исключает связку между слоями и упрощает тесты.
- **Конфиг:** `github.com/spf13/viper`. Единая структура `config.Config` + `config.Load() *Config`. `.env` опционален (читается если лежит в CWD), env vars имеют приоритет. **При любой ошибке загрузки или пустом required-поле — `panic`**: без валидного конфига приложение не должно стартовать. Все текущие поля (`APP_NAME`, `BOT_NAME`, `TELEGRAM_BOT_TOKEN`, `DB_PATH`) обязательные. При добавлении новой переменной — править: структуру `Config`, список `BindEnv` и (если требуется) `requireNonEmpty` в `internal/config/config.go`, `.env.example`, таблицу переменных в `SPEC.md`, блок `environment:` в `docker-compose.yaml`.
- **Хранилище:** SQLite через `github.com/mattn/go-sqlite3` (cgo, поэтому в Dockerfile `build-base`) + голый `database/sql` (без ORM/sqlx). Пакет `internal/storage/sqlite`, `New(path string, logger *slog.Logger) *sql.DB` — open + ping, возвращает готовый `*sql.DB`. Принимает строку, не `*config.Config` (внутренние пакеты не зависят от пакета config). К DSN добавляется `?_foreign_keys=on` (параметр `mattn/go-sqlite3`) — это включает PRAGMA `foreign_keys` на КАЖДОМ новом соединении пула. `db.Exec("PRAGMA foreign_keys = ON")` после `Open` НЕ использовать: PRAGMA scoped к соединению, а `database/sql` пулит — Exec затронет одно случайное. Без этой настройки FK в схеме (`telegram_events.telegram_user_id → telegram_users(id)`, `telegram_users.user_id → users(id)`) висят декларативно и не enforce'ятся в рантайме. **При любой ошибке — `panic` через `fmt.Errorf("sqlite: ...: %w", err)`** (консистентно с `config.Load`: без валидного хранилища приложение не должно стартовать). При неудаче ping соединение закрывается перед panic, чтобы не текли дескрипторы. `panic(fmt.Errorf(...))` не нарушает правила «ошибки только через переменные» — правило касается только `return`. Файл БД — bind-mount `./data:/data`, путь задаёт `DB_PATH` (dev: `/data/asker.db`); каталог `./data/` в `.gitignore`.
- **Репозитории:** один пакет на модель в `internal/repository/<model>/`. Имя пакета — `<model>Repo` (пример: папка `users/` → `package usersRepo`). Раскладка по файлам внутри пакета: `<model>.go` — контракт (интерфейс `Repository` + sentinel-ошибки), `<driver>.go` — реализация под конкретное хранилище (пример: `sqlite.go` — `usersSQLiteRepo` + `NewUsersSQLiteRepo(db *sql.DB) Repository`). Паттерн — **exported interface + unexported struct**: конструктор возвращает интерфейс, потребители (хендлеры, сервисы) зависят от интерфейса, а не от конкретной реализации. Соединение с БД живёт в `main.go` — репозиторий его **не закрывает**. Методы принимают `ctx context.Context` и используют `db.ExecContext` / `QueryContext` — это обязательно, не опция. Ошибки возвращаются через sentinel уровня пакета (`var ErrCreate = errors.New("users: create")`) + `errors.Join(sentinel, cause)`, без `fmt.Errorf` в `return` (правило проекта).
- **Доменные модели:** общие дата-структуры (`TelegramUser` и т.п.) живут в пакете `internal/models` (файл на модель: `internal/models/<model>.go`). Пакет `models` **без зависимостей** от БД-драйверов (`database/sql`, `sql.NullString`) и бизнес-логики — только поля, опциональные как `*T`. Репозитории и хендлеры импортируют `models` и возвращают/принимают `*models.X`; маппинг в БД-специфичные типы (`sql.NullString` и т.п.) делается локальными хелперами в `internal/repository/<model>/<driver>.go`.
- **Миграции:** лежат в `migrations/` как раздельные `NNNN_<snake_case>.up.sql` и `NNNN_<snake_case>.down.sql`. Нумерация монотонная (`0001`, `0002`, ...). Новая фича = новая миграция (не редактировать уже применённые). Down-файлы обязаны быть идемпотентны — `DROP TRIGGER IF EXISTS ...; DROP TABLE IF EXISTS ...;`. Порядок внутри одного down: сначала триггер, потом таблица (для наглядности). Порядок между миграциями в down: обратный от up (зависимые таблицы дропаются первыми). **Автоматический runner при инициализации БД намеренно не делаем** — настройка схемы лежит на человеке. Перед первым `docker compose up` (и каждый раз, когда нужно обнулить БД) вручную запускается `./scripts/refresh_db.sh`. `sqlite.New` только открывает файл и делает ping; если таблиц нет — бот упадёт в рантайме на первом запросе (`no such table: ...`), это осознанная цена за простоту и явный жизненный цикл БД. Предложения «давай при старте бота автоматически применим миграции» — отклоняй, ссылаясь на этот пункт.
- **`scripts/`:** утилитные shell-скрипты, запускаются с хоста (не из контейнера). Сейчас там один скрипт — `refresh_db.sh`, делает полный refresh SQLite: `docker compose stop app` → все `*.down.sql` в обратном порядке → все `*.up.sql` в прямом → `docker compose start app`. Требует `sqlite3` в PATH хоста. Использует `./data/asker.db` напрямую (bind-mount'нутый в контейнер).
- **Прокси:** стандартные `HTTP_PROXY`/`HTTPS_PROXY`/`NO_PROXY` пробрасываются в контейнер через `environment:` compose, берутся из `.env`. В `config.Config` они **не входят** — Go-шный `http.DefaultTransport` подхватывает их автоматически. Обязательны на этом хосте (российский прод) — без прокси `bot.New` падает на `getMe: context deadline exceeded`, т.к. `api.telegram.org` заблокирован. Значения для локальной разработки берутся из `~/.bashrc` (Beget EU); в `.env.example` шаблон с пустыми строками — для деплоя вне РФ прокси не нужен.
- **Telegram:** `github.com/go-telegram/bot` (v1.20.0), long-polling. Обработчики регистрируются в `TelegramBot.Start` через `b.RegisterHandler(bot.HandlerTypeMessageText, "<cmd>", bot.MatchTypeExact, <method>)`. Default-хендлер (fallback на всё, что не поймали зарегистрированные) подключается через `bot.WithDefaultHandler(t.handleXxx)` в `bot.New(...)` — сейчас так подключён echo (`handler_echo.go`). **Соглашение: один хендлер — один файл.** Каждая команда — приватный метод `*TelegramBot` в отдельном файле `internal/telegram/handler_<name>.go` (пример: `handler_start.go` → `handleStart`, `handler_echo.go` → `handleEcho`). В `telegram.go` лежит тип `TelegramBot`, конструктор, `Start`, регистрация и **доменные методы `*TelegramBot`, переиспользуемые несколькими хендлерами** (например, `CreateNewTelegramUserIfNotExists` — идемпотентно сохраняет TG-аккаунт в `telegram_users`; ошибки логирует, наружу не возвращает). Когда количество хендлеров начнёт делиться на явные домены — переедем на группировку `handler_<domain>_<name>.go` одним рефакторингом. Webhook откладывается до Фазы 4 (prod). **Зависимости (репозитории и т.п.) инжектируются через конструктор `NewTelegramBot` и хранятся приватными полями `*TelegramBot`** — хендлеры обращаются к ним через `t.<field>` (сейчас так подключён `t.telegramUsers telegramUsersRepo.Repository`). Новая доменная зависимость добавляется в три места: поле структуры, параметр конструктора, присваивание в `NewTelegramBot`. Внутри пакета `telegram` зависимости создавать нельзя — только принимать готовыми.
- **Логирование:** stdlib `log/slog`. Root-логгер (`slog.NewTextHandler(os.Stdout, nil)`) создаётся в `main`, передаётся в компоненты третьим аргументом конструктора (`NewTelegramBot(token, botName, logger)`), хранится в поле `logger`. Пакет `log` в проекте не использовать — только `slog`. В `main.go` вместо `log.Fatalf` — `logger.Error(...)` + `os.Exit(1)`. При добавлении нового компонента (бот-handler, HTTP-клиент, хранилище) — принимай `*slog.Logger` в конструкторе, логгер **не** создавай внутри пакета. Соглашение: **в конструкторах сервисов `*slog.Logger` идёт первым параметром**, дальше доменные зависимости (репозитории, клиенты).
- **Claude Code CLI внутри контейнера:** в Dockerfile глобально ставится `@anthropic-ai/claude-code` (нужен провайдеру `claude_code_cli` в `internal/services/ai/`, который вызывает `claude` как подпроцесс). OAuth-токены подписки шарятся между хостом и контейнером через bind-mount двух путей: `./data/claude-bot:/root/.claude` и `./data/claude-bot.json:/root/.claude.json` (контейнер бежит под root, поэтому пути в `/root/`, а не в `/home/<user>/`). Хостовый watcher `/usr/local/bin/claude-creds-watcher.sh` (systemd unit `claude-creds-watcher.service`) через inotify держит `.credentials.json` в синхроне между четырьмя точками: `/opt/claude-credentials/credentials.json`, `/root/.claude/.credentials.json`, ingarden-volume и asker-volume `./data/claude-bot/.credentials.json`. Ownership/perms watcher проставляет сам (для asker — `0:0`, `chmod 600`). Перед первым `docker compose up` директория `./data/claude-bot/` и пустой файл `./data/claude-bot.json` должны существовать на хосте — иначе bind-mount файла превратит его в директорию. Оба попадают под `/data/` в `.gitignore`. После `claude /login` на любом из трёх инстансов watcher разнесёт новые токены по остальным; ручной sync-скрипт здесь не нужен.
- **AI-функционал (Фаза 2):** одни абстракции — пакет `internal/services/ai` (файл `ai.go`): тип `Prompt` (named-type над string), `Info{Provider, Model}`, единственный интерфейс `LLM` (`Prompt(ctx, prompt) (string, error)` + `GetInfo()`). Реализации провайдеров лежат в подпапках `internal/services/ai/<name>/` (openrouter, claude-code-cli, в будущем — anthropic, openai, fake) — каждая в своём пакете. `Prompt` принципиально живёт **в одном пакете** с `LLM`, чтобы реализации провайдеров не зависели друг от друга и не делали лишних конвертаций между чужими named-типами. Сборка `Prompt` (system + история + текущий вопрос) — задача вызывающего Telegram-хендлера, отдельного интерфейса-генератора нет (был `PromptGenerator`, удалён 2026-04-26 как абстракция без потребителей). Отдельного `Assistant`-сервиса тоже намеренно нет: `LLM.Prompt` достаточно, лишний слой не несёт ценности. История общения с ботом для подачи в LLM хранится в `chat_messages` (привязка к `users.id`, ассистент работает только для пользователей с привязанным номером). Системный промпт — в `prompts/system_prompt.md`, bind-mount `./prompts:/prompts:ro` в compose, грузится один раз при старте `main.go` и передаётся в конструктор реализации `LLM`. AI-конфиг (`AI_*`) — все поля безусловно обязательные; ассистент включён всегда (мастер-выключатель `AI_ENABLED` удалён 2026-04-26).
- **Без prod-Dockerfile.** Multi-stage под prod появится на Фазе 4.

## Как запустить

```bash
cp .env.example .env     # обязательно — compose интерполирует ${APP_NAME}
# Проставить валидный TELEGRAM_BOT_TOKEN (получить у @BotFather), иначе config.Load упадёт в panic
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

## Объявление переменных — только через `var` с явным типом

**Правило проекта (обязательное).** Все локальные переменные объявляются через `var <name> <Type> = <value>` с **явным указанием типа**. Короткая форма `:=` — **запрещена**.

Единственное исключение — **приём нескольких значений из функции** (`a, b := foo()`): там `:=` допустим, потому что типы всех возвращаемых значений выводятся однозначно из сигнатуры функции. Сюда попадают и `for _, x := range ...`, и `_, err := foo()` (blank-идентификатор считается за «другую» переменную).

Примеры:

```go
// ❌ Нельзя
cfg := config.Load()
chatID := update.Message.Chat.ID
text := fmt.Sprintf("...", x)
if err := db.Ping(); err != nil { ... }

// ✅ Надо
var cfg *config.Config = config.Load()
var chatID int64 = update.Message.Chat.ID
var text string = fmt.Sprintf("...", x)
var err error = db.Ping()
if err != nil { ... }

// ✅ Исключение — несколько возвращаемых значений
result, err := r.db.ExecContext(ctx, ...)
for _, key := range keys { ... }
if _, err := b.SendMessage(ctx, ...); err != nil { ... }
```

Зачем:
- Тип виден на месте объявления — не нужно прыгать к сигнатуре функции, чтобы понять, что за значение.
- Диффы читаемее: смена типа — один символ в одном месте, а не каскад по месту использования.
- Code review быстрее — нет «а это точно `int64`, а не `int32`?» на каждом shadowed `:=`.

Применимость:
- Распространяется на локальные переменные (внутри функций/методов) и на inline-формы `if`/`for`/`switch`: `if err := foo(); ...` → вынести на строку выше как `var err error = foo(); if err != nil { ... }`.
- На параметры функций, поля структур, пакетные `var`-блоки и константы (`const`) правило не распространяется — там тип и так обязан быть указан по синтаксису Go.
- `var exists int` (без инициализации) — валидно: тип явно указан, значение — zero.

## Пустая строка перед `return`

**Правило проекта (обязательное).** Перед каждым `return` ставится пустая строка, отделяющая выход от предыдущего кода.

Единственное исключение — **`return` является единственной инструкцией в своём блоке** (`if`, `else`, `case`, `func` с одним выражением и т.п.). В этом случае пустая строка избыточна.

Примеры:

```go
// ❌ Нельзя
func foo() (int, error) {
    x, err := bar()
    if err != nil {
        return 0, err
    }
    y := x * 2
    return y, nil
}

// ✅ Надо
func foo() (int, error) {
    x, err := bar()
    if err != nil {
        return 0, err
    }

    y := x * 2

    return y, nil
}

// ✅ Исключение — единственная инструкция в блоке
func id(x int) int {
    return x
}

if exists {
    return
}
```

Зачем:
- Визуально отделяет точку выхода от подготовки — легче читать, где именно функция возвращается.
- Диффы чище: добавление строки между подготовкой и `return` не ломает отступ у return.

## Host context

This repo lives on the shared production server (see `/root/CLAUDE.md`). Nothing here is deployed yet — there is no systemd unit, no nginx vhost, no domain. When deployment time comes, follow `/root/.claude/rules/deploy-checklist.md`.
