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

Зависимости (БД, Redis, внешние API) **пока отсутствуют**. Появятся после решения о функционале бота.

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
├── cmd/
│   └── bot/
│       └── main.go      — точка входа: загружает Config, создаёт TelegramBot, запускает long-polling до SIGINT/SIGTERM
└── internal/
    ├── config/
    │   └── config.go    — структура Config и функция Load() (viper: .env + env vars, panic при ошибке или пустом required-поле)
    └── telegram/
        └── telegram.go  — структура TelegramBot (NewTelegramBot + Start), обработчик /start
```

## Как запустить локально

```bash
cp .env.example .env
# заполнить TOKEN_BOT_TOKEN (получить у @BotFather) — без валидного токена приложение упадёт
docker compose up --build
```

После старта в stdout идёт лог вида `YYYY/MM/DD HH:MM:SS starting Asker (Герман)`, затем бот ждёт апдейты в long-polling. На команду `/start` от пользователя отвечает `Привет, <FirstName>! Я <BotName>.`. При изменении `.go` файлов `air` автоматически пересобирает и перезапускает бинарник.

## Переменные окружения

Загружаются через `internal/config` (viper): сначала читается `.env` в корне проекта (если есть), затем поверх накладываются переменные окружения процесса — env vars имеют приоритет. Публичный API: `config.Load() *Config` (при ошибке — `panic`, дальнейшее выполнение без конфига не имеет смысла).

Все поля — **обязательные**: при пустом значении `config.Load` паникует с сообщением `config: <KEY> is required but empty`.

| Имя | Поле `Config` | Назначение | Обязательная |
|---|---|---|---|
| `APP_NAME` | `AppName` | Префикс имени контейнера и идентификатор приложения | да |
| `BOT_NAME` | `BotName` | Отображаемое имя бота (используется в приветствии `/start`) | да |
| `TOKEN_BOT_TOKEN` | `TokenBotToken` | Токен Telegram Bot API | да |

## Фазы реализации

- [x] **Фаза 0 — Скаффолд инфраструктуры.** docker-compose + Dockerfile + air + базовый Go-луп `working...`.
- [x] **Фаза 1 — Подключение к Telegram (БАЗА).** Клиент `github.com/go-telegram/bot` в long-polling, структура `TelegramBot` с `NewTelegramBot(token, botName)` и `Start(ctx)`, обработка `/start` с персонализированным приветствием, graceful shutdown по SIGINT/SIGTERM. Расширение набора команд — в следующих фазах.
- [ ] **Фаза 2 — Функционал бота «Герман».** Определяется отдельно.
- [ ] **Фаза 3 — Хранилище.** Выбор БД (если нужна) и интеграция.
- [ ] **Фаза 4 — Prod-деплой.** Multi-stage Dockerfile, systemd / compose-стек на сервере, nginx (если нужен webhook).

## Changelog

- **2026-04-24** — создан SPEC, скаффолд инфраструктуры (Фаза 0).
- **2026-04-24** — добавлен `internal/config` на базе `github.com/spf13/viper`: единая структура `Config` и функция `Load()`; `main.go` читает конфиг при старте.
- **2026-04-24** — `config.Load` больше не возвращает `error`: при любой ошибке загрузки — `panic`; `main.go` упрощён (без проверки ошибки).
- **2026-04-24** — `config.Load` дополнительно валидирует непустоту всех полей (`APP_NAME`, `BOT_NAME`, `TOKEN_BOT_TOKEN`) — panic при пустом значении.
- **2026-04-24** — Фаза 1: добавлен `internal/telegram` на базе `github.com/go-telegram/bot`: структура `TelegramBot` (`NewTelegramBot(token, botName)` + `Start(ctx)`), приватный обработчик `/start` с приветствием `Привет, {FirstName}! Я {BotName}.`; `main.go` запускает бота с `signal.NotifyContext` (SIGINT/SIGTERM) для чистого shutdown.
