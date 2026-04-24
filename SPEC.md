# SPEC — Asker (бот «Герман»)

**Статус:** 🟡 Planning
**Владелец:** bklv-kirill
**Создан:** 2026-04-24

## Описание

Телеграм-бот под кодовым именем **«Герман»**. Конкретный функционал бота будет определён в отдельной фазе — сейчас проект находится на стадии подготовки инфраструктуры разработки.

Имя приложения — `Asker` (используется как префикс в docker-compose и в `APP_NAME` из `.env`).

## Архитектура (текущее состояние)

Один сервис:

| Слой | Технология |
|---|---|
| Runtime | Go 1.25 |
| Упаковка | Docker (образ на базе `golang:1.25-alpine`) |
| Оркестрация | docker-compose (один bridge-network `app`) |
| Dev-режим | Hot reload через `air` (`github.com/air-verse/air`) |
| Секреты | `.env` в корне (шаблон — `.env.example`) |
| Конфиг | `github.com/spf13/viper` — загрузка `.env` + env vars в единую структуру `Config` |
| Telegram | `github.com/go-telegram/bot` — клиент Bot API в режиме long-polling |
| Хранилище | SQLite через `github.com/mattn/go-sqlite3` (cgo) + голый `database/sql`; файл в bind-mount `./data/` → `/data/` в контейнере |
| Логирование | `log/slog` (stdlib) — root-логгер создаётся в `main`, передаётся компонентам DI-стилем |

Внешние зависимости (Redis, сторонние API) **пока отсутствуют**. Появятся после решения о функционале бота.

## Структура проекта

```
asker/
├── SPEC.md              — текущий файл
├── CLAUDE.md            — инструкции для Claude Code
├── Dockerfile           — dev-образ (Go + air)
├── docker-compose.yaml  — один сервис app
├── .air.toml            — конфиг hot reload
├── .env.example         — шаблон переменных окружения
├── .gitignore
├── .dockerignore
├── go.mod
├── go.sum
├── migrations/          — SQL-миграции: файлы вида NNNN_<name>.up.sql / NNNN_<name>.down.sql
│   ├── 0001_users.up.sql / 0001_users.down.sql
│   └── 0002_telegram_users.up.sql / 0002_telegram_users.down.sql
├── scripts/             — утилитные скрипты (запускаются с хоста)
│   └── refresh_db.sh    — откатывает все down → применяет все up → перезапускает контейнер
├── cmd/
│   └── bot/
│       └── main.go      — точка входа: загружает Config, открывает SQLite, создаёт TelegramBot, запускает long-polling до SIGINT/SIGTERM
└── internal/
    ├── config/
    │   └── config.go    — структура Config и функция Load() (viper: .env + env vars, panic при ошибке или пустом required-поле)
    ├── repository/
    │   └── users/              — пакет usersRepo
    │       ├── users.go        — интерфейс Repository + ErrCreate (контракт доступа к таблице users)
    │       └── sqlite.go       — реализация поверх SQLite: usersSQLiteRepo + NewUsersSQLiteRepo(db) Repository
    ├── storage/
    │   └── sqlite/
    │       └── sqlite.go  — New(cfg, logger) *sql.DB: открывает SQLite-файл, делает ping, возвращает соединение; panic при любой ошибке
    └── telegram/
        ├── telegram.go        — структура TelegramBot (NewTelegramBot(token, botName, logger) + Start), регистрация обработчиков
        ├── handler_start.go   — обработчик /start (приватный метод *TelegramBot)
        └── handler_echo.go    — default-обработчик: повторяет произвольный текст пользователя, логирует вход и исход

Соглашение: каждый обработчик команды живёт в отдельном файле `handler_<name>.go` внутри
`internal/telegram/` как приватный метод `*TelegramBot`. Регистрация всех обработчиков —
в `TelegramBot.Start` через `b.RegisterHandler(...)`.
```

## Как запустить локально

```bash
cp .env.example .env
# заполнить TOKEN_BOT_TOKEN (получить у @BotFather) — без валидного токена приложение упадёт
docker compose up --build
```

После старта в stdout идёт лог вида `time=... level=INFO msg=starting app=Asker bot=Герман` (формат `slog.TextHandler`), затем бот ждёт апдейты в long-polling. На команду `/start` от пользователя отвечает `Привет, <FirstName>! Я <BotName>.`. На любое другое текстовое сообщение срабатывает echo-хендлер: бот повторяет текст дословно и пишет в лог два события — `incoming message` (chat_id, user_id, username, text) и `outgoing reply` (chat_id, text). При изменении `.go` файлов `air` автоматически пересобирает и перезапускает бинарник.

## Переменные окружения

Загружаются через `internal/config` (viper): сначала читается `.env` в корне проекта (если есть), затем поверх накладываются переменные окружения процесса — env vars имеют приоритет. Публичный API: `config.Load() *Config` (при ошибке — `panic`, дальнейшее выполнение без конфига не имеет смысла).

Все поля — **обязательные**: при пустом значении `config.Load` паникует с сообщением `config: <KEY> is required but empty`.

| Имя | Поле `Config` | Назначение | Обязательная |
|---|---|---|---|
| `APP_NAME` | `AppName` | Префикс имени контейнера и идентификатор приложения | да |
| `BOT_NAME` | `BotName` | Отображаемое имя бота (используется в приветствии `/start`) | да |
| `TOKEN_BOT_TOKEN` | `TokenBotToken` | Токен Telegram Bot API | да |
| `DB_PATH` | `DBPath` | Путь к SQLite-файлу внутри контейнера (dev: `/data/asker.db`) | да |

Дополнительно пробрасываются стандартные HTTP-proxy переменные — они нужны только Go-рантайму (`http.DefaultTransport` подхватывает их автоматически) и в `config.Config` не попадают. Требуются на хостах, где `api.telegram.org` заблокирован (например, российский прод-сервер): без прокси `bot.New` падает на `getMe` с `context deadline exceeded`. Если оставить пустыми — Go пойдёт напрямую.

| Имя | Назначение |
|---|---|
| `HTTP_PROXY` | HTTP-прокси для исходящих запросов |
| `HTTPS_PROXY` | HTTPS-прокси для исходящих запросов (используется для `api.telegram.org`) |
| `NO_PROXY` | Список хостов в обход прокси |

## Фазы реализации

- [x] **Фаза 0 — Скаффолд инфраструктуры.** docker-compose + Dockerfile + air + базовый Go-луп `working...`.
- [x] **Фаза 1 — Подключение к Telegram (БАЗА).** Клиент `github.com/go-telegram/bot` в long-polling, структура `TelegramBot` с `NewTelegramBot(token, botName)` и `Start(ctx)`, обработка `/start` с персонализированным приветствием, graceful shutdown по SIGINT/SIGTERM. Расширение набора команд — в следующих фазах.
- [ ] **Фаза 2 — Функционал бота «Герман».** Определяется отдельно.
- [~] **Фаза 3 — Хранилище.** SQLite подключён: пакет `internal/storage/sqlite` с `New(cfg, logger) *sql.DB` (open + ping, panic при ошибке — консистентно с `config.Load`), файл лежит в bind-mount `./data/asker.db`. Добавлены миграции `0001_users` (доменный пользователь: name/gender/age/info) и `0002_telegram_users` (привязка users ↔ telegram_user_id, 1-к-1 через `UNIQUE(user_id)` и `UNIQUE(telegram_user_id)`, FK с `ON DELETE CASCADE`, триггеры `AFTER UPDATE` на `updated_at`). Миграции лежат в `migrations/` как раздельные `.up.sql` / `.down.sql`; скрипт `scripts/refresh_db.sh` делает полный refresh (все down в обратном порядке → все up в прямом, с остановкой/стартом контейнера). **Автоматический runner миграций при старте приложения намеренно не делаем** — настройка БД лежит на человеке: перед первым `docker compose up` и каждый раз, когда нужно обнулить схему, запускается `./scripts/refresh_db.sh`. Конкретные репозитории под модели — следующий шаг.
- [ ] **Фаза 4 — Prod-деплой.** Multi-stage Dockerfile, systemd / compose-стек на сервере, nginx (если нужен webhook).

## Changelog

- **2026-04-24** — создан SPEC, скаффолд инфраструктуры (Фаза 0).
- **2026-04-24** — добавлен `internal/config` на базе `github.com/spf13/viper`: единая структура `Config` и функция `Load()`; `main.go` читает конфиг при старте.
- **2026-04-24** — `config.Load` больше не возвращает `error`: при любой ошибке загрузки — `panic`; `main.go` упрощён (без проверки ошибки).
- **2026-04-24** — `config.Load` дополнительно валидирует непустоту всех полей (`APP_NAME`, `BOT_NAME`, `TOKEN_BOT_TOKEN`) — panic при пустом значении.
- **2026-04-24** — Фаза 1: добавлен `internal/telegram` на базе `github.com/go-telegram/bot`: структура `TelegramBot` (`NewTelegramBot(token, botName)` + `Start(ctx)`), приватный обработчик `/start` с приветствием `Привет, {FirstName}! Я {BotName}.`; `main.go` запускает бота с `signal.NotifyContext` (SIGINT/SIGTERM) для чистого shutdown.
- **2026-04-24** — введён `log/slog` как единственный логгер: root создаётся в `main` (`slog.NewTextHandler(os.Stdout, nil)`), `NewTelegramBot` принимает `*slog.Logger` третьим аргументом и сохраняет в поле `logger`; `main.go` перешёл с `log` на slog, `log.Fatalf` заменён на `logger.Error` + `os.Exit(1)`.
- **2026-04-24** — принята конвенция «один хендлер — один файл»: `handleStart` вынесен из `telegram.go` в `handler_start.go`; в `telegram.go` остались только `TelegramBot`, `NewTelegramBot`, `Start` и регистрация хендлеров.
- **2026-04-24** — добавлен echo-хендлер (`handler_echo.go`): подключён в `Start` через `bot.WithDefaultHandler`, повторяет любое текстовое сообщение, которое не поймали зарегистрированные команды, логирует `incoming message` и `outgoing reply` через slog.
- **2026-04-24** — прокинут HTTP(S)-прокси в контейнер: `HTTP_PROXY`/`HTTPS_PROXY`/`NO_PROXY` добавлены в `.env.example`, `.env` и `environment:` compose. Без прокси `bot.New` падал на `getMe: context deadline exceeded` из-за блокировки `api.telegram.org` на российском хосте. Go-шный `http.DefaultTransport` подхватывает эти env автоматически — код не менялся.
- **2026-04-25** — Фаза 3 (база): добавлен пакет `internal/storage/sqlite` с `New(cfg, logger) *sql.DB` (драйвер `github.com/mattn/go-sqlite3`, голый `database/sql`). Контракт — panic при любой ошибке open/ping (консистентно с `config.Load`); при неудаче ping соединение закрывается перед panic. В Dockerfile добавлен `build-base` для cgo, в compose — bind-mount `./data:/data`, в `config.Config` — обязательное поле `DBPath` (env `DB_PATH`). `main.go` открывает БД при старте, `defer db.Close()`. Репозитории под модели будут отдельно и принимать `*sql.DB` в конструкторах.
- **2026-04-25** — Фаза 3 (миграции + refresh): добавлены миграции `migrations/0001_users` и `migrations/0002_telegram_users` в формате раздельных `.up.sql` / `.down.sql`. Схема зафиксирована: `users(id, name, gender, age, info, created_at, updated_at)` + триггер на `updated_at`; `telegram_users(id, user_id UNIQUE, telegram_user_id UNIQUE, created_at, updated_at, FK users ON DELETE CASCADE)` + триггер. `gender` = TEXT CHECK enum (`'мужчина'`/`'женщина'`), `age` CHECK 1..120. Добавлен `scripts/refresh_db.sh` — останавливает контейнер, прогоняет все down в обратном порядке, up в прямом, стартует контейнер. Down-миграции идемпотентны (`DROP ... IF EXISTS`), sqlite3-клиент ожидается на хосте. Runner внутри приложения (авто-apply при старте) ещё не подключён.
- **2026-04-25** — Фаза 3 (репозиторий users): добавлен `internal/repository/users` (пакет `usersRepo`) — `users.go` хранит контракт (интерфейс `Repository` с методом `Create(ctx, name) (int64, error)` + sentinel `ErrCreate`), `sqlite.go` — реализацию поверх `*sql.DB` (приватная `usersSQLiteRepo`, конструктор `NewUsersSQLiteRepo(db) Repository`). Соединение не закрывается репозиторием (жизненный цикл — в `main`). Ошибки через `errors.Join(ErrCreate, cause)`. Пакет существует, но пока нигде не импортируется — подключим в хендлеры по мере необходимости.
