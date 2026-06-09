# EPIC 01: Project Foundation

## Status

Done

## Goal

Создать локальную инфраструктуру разработки: Docker Compose со всеми сервисами, структуру репозитория, базовые конфигурации и скрипты запуска. После выполнения эпика вся команда может поднять окружение одной командой.

## Context

Проект WR any% — система автоматического трекинга маршрутов. Состоит из:
- Android-клиента (React Native) → `apps/android-tracker`
- Web-клиента аналитики (React) → `apps/web`
- Backend-сервисов (Go) → `services/tracking-gateway`, `services/tracking-worker`
- Kafka как внутренней шины событий (только внутри Docker-сети)
- PostgreSQL + PostGIS как хранилища

На этапе EPIC 1 production-код сервисов не пишется. Цель — рабочий скелет репозитория и локальное окружение.

## Problem Analysis

### Проблема 1: Отсутствие единого окружения

Без Docker Compose каждый разработчик настраивает окружение вручную. Это приводит к расхождениям версий, "works on my machine" и потере времени.

### Проблема 2: Монорепо vs полирепо

Проект включает несколько независимых сервисов на разных стеках (Go, React Native, React). Нужно решить структуру репозитория с учётом будущих эпиков.

### Проблема 3: PostGIS требует специфического образа

Стандартный `postgres` образ не включает PostGIS. Нужен `postgis/postgis` или отдельная инициализация.

### Проблема 4: Kafka KRaft mode

Использовать `apache/kafka:3.7.0` в KRaft mode — без Zookeeper. Kafka не пробрасывается на host: порт 9092 доступен только внутри Docker-сети. Проверка топиков выполняется через `docker compose exec` внутри контейнера.

### Проблема 5: Порядок запуска сервисов

Сервисы Go зависят от Kafka и Postgres. Docker Compose `depends_on` не гарантирует готовность — нужны healthcheck'и.

## Best Practice Research

### Монорепо структура

Подход: монорепо с workspace-разделением по директориям.

```
wrany/
├── services/                      # Go backend сервисы
│   ├── tracking-gateway/
│   └── tracking-worker/
├── apps/                          # клиентские приложения
│   ├── android-tracker/
│   └── web/
├── infra/
│   └── docker/
│       └── postgres/
├── go.work
├── docker-compose.yml
├── .env.example
├── Makefile
└── README.md
```

### Docker Compose best practices

- Healthcheck для каждого stateful сервиса
- `depends_on` с `condition: service_healthy`
- `.env.example` с документированными переменными
- Отдельные volumes для данных
- Именованные сети для изоляции
- `.dockerignore` для каждого Go-сервиса

### Kafka

Образ: `apache/kafka:3.7.0` в KRaft mode — без Zookeeper, официальный Apache образ.

Kafka **не пробрасывается на host**. Порт 9092 доступен только внутри Docker-сети `wrany-net`. Проверка выполняется через:

```bash
docker compose exec kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server localhost:9092 --list
```

### PostgreSQL + PostGIS

Образ: `postgis/postgis:16-3.4` — официальный, содержит PostGIS из коробки.

### Go workspace

Использовать Go workspaces (`go.work`) для монорепо с несколькими Go-модулями.

## Solution Design

### Структура репозитория

```
wrany/
├── .claude/
│   └── epics/
│       └── 01-project-foundation/
│           └── EPIC.md
├── services/
│   ├── tracking-gateway/
│   │   ├── cmd/gateway/main.go    # stub: HTTP /healthz на порту 8080
│   │   ├── go.mod
│   │   ├── Dockerfile
│   │   └── .dockerignore
│   └── tracking-worker/
│       ├── cmd/worker/main.go     # stub: HTTP /healthz на порту 8081
│       ├── go.mod
│       ├── Dockerfile
│       └── .dockerignore
├── apps/
│   ├── android-tracker/           # пустая директория + README
│   └── web/                       # пустая директория + README
├── infra/
│   └── docker/
│       └── postgres/
│           └── init.sql           # CREATE EXTENSION IF NOT EXISTS postgis
├── go.work
├── docker-compose.yml
├── .env.example
├── Makefile
└── README.md
```

### Docker Compose сервисы

| Сервис           | Образ                    | Порт (внутренний) | Host-порт | Healthcheck                        |
|------------------|--------------------------|-------------------|-----------|------------------------------------|
| postgres         | postgis/postgis:16-3.4   | 5432              | 5432      | pg_isready                         |
| kafka            | apache/kafka:3.7.0       | 9092              | —         | kafka-topics.sh (внутри контейнера)|
| tracking-gateway | (local build)            | 8080              | 8080      | GET /healthz → 200                 |
| tracking-worker  | (local build)            | 8081              | 8081      | GET /healthz → 200                 |

Kafka: порт 9092 **не пробрасывается на host**.

### Go стаб-сервисы

`tracking-gateway` — HTTP-сервер на `:8080`:
- `GET /healthz` → `{"status":"ok"}` + HTTP 200
- Graceful shutdown по SIGTERM/SIGINT

`tracking-worker` — HTTP-сервер на `:8081`:
- `GET /healthz` → `{"status":"ok"}` + HTTP 200
- Graceful shutdown по SIGTERM/SIGINT

**Dockerfile Go-сервисов:** при использовании минимального runtime-образа (например, `gcr.io/distroless/static` или `alpine`) необходимо явно включить `curl` или `wget` — они нужны Docker Compose для выполнения healthcheck. Рекомендуется `alpine` с установкой `curl` в финальном образе.

### Makefile команды

```makefile
make up           # docker compose up -d --build
make down         # docker compose down
make logs         # docker compose logs -f
make reset        # down + volumes rm + up
make check-postgis  # проверить SELECT PostGIS_Version() в postgres
make db-shell     # psql shell в postgres контейнере
make test         # тесты по каждому Go-сервису явно:
                  #   cd services/tracking-gateway && go test ./...
                  #   cd services/tracking-worker  && go test ./...
```

`make migrate` исключён из EPIC 1. Миграции — отдельная задача в будущем эпике.

## Architecture Notes

- Kafka не экспонируется наружу. Порт 9092 доступен только внутри `wrany-net`.
- Проверка Kafka выполняется через `docker compose exec kafka ...`.
- PostGIS инициализируется через `infra/docker/postgres/init.sql` при первом запуске.
- Go workspaces упрощают локальную разработку без публикации модулей.
- `.env.example` — единственный источник правды по переменным окружения. `.env` в `.gitignore`.
- `.dockerignore` добавляется в каждый Go-сервис для исключения лишних файлов из build context.

### Go Service Structure (Clean Architecture)

Все Go-сервисы следуют Clean Architecture / Hexagonal Architecture:

```
services/<service-name>/
  cmd/<service-name>/main.go        # composition root only
  internal/
    app/app.go                      # lifecycle management
    config/config.go                # env var loading
    domain/                         # entities, errors (no external deps)
    usecase/                        # business logic (no transport/driver deps)
    transport/
      http/                         # HTTP handlers + router
      kafka/                        # Kafka adapters
      grpc/                         # gRPC handlers
    storage/
      postgres/
      memory/
```

Dependency direction: `transport → usecase → domain`, `storage → usecase interfaces`.
`main.go` только: load config → init logger → build app → run → graceful shutdown.
HTTP `/healthz` handler реализован в `internal/transport/http/`, не в `main.go`.

## Tasks

- [ ] T01: Создать структуру директорий (`services/tracking-gateway`, `services/tracking-worker`, `apps/android-tracker`, `apps/web`, `infra/docker/postgres`)
- [ ] T02: Написать `docker-compose.yml` с postgres, kafka (без host-порта), tracking-gateway, tracking-worker
- [ ] T03: Написать `.env.example` с документированными переменными
- [ ] T04: Написать `infra/docker/postgres/init.sql` (`CREATE EXTENSION IF NOT EXISTS postgis`)
- [ ] T05: Создать Go-стаб `services/tracking-gateway` (main.go + go.mod + Dockerfile + .dockerignore)
- [ ] T06: Создать Go-стаб `services/tracking-worker` (main.go + go.mod + Dockerfile + .dockerignore; HTTP /healthz на :8081)
- [ ] T07: Создать `go.work` для Go workspaces
- [ ] T08: Написать `Makefile` с командами up/down/logs/reset/check-postgis/db-shell/test
- [ ] T09: Написать корневой `README.md` с инструкцией запуска
- [ ] T10: Добавить `apps/android-tracker/README.md` и `apps/web/README.md`
- [ ] T11: Проверить `make up` — все сервисы healthy
- [ ] T12: Проверить `curl localhost:8080/healthz` → 200
- [ ] T13: Проверить `curl localhost:8081/healthz` → 200
- [ ] T14: Проверить `make check-postgis` выводит версию PostGIS
- [ ] T15: Проверить `make db-shell` открывает psql
- [ ] T16: Проверить Kafka через `docker compose exec kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server localhost:9092 --list`
- [ ] T17: Проверить `make test` — unit-тесты стабов проходят
- [ ] T18: Commit с EPIC.md и всей структурой

## Acceptance Criteria

- [ ] `make up` поднимает все сервисы без ошибок
- [ ] `docker compose ps` показывает все сервисы в состоянии `healthy`
- [ ] `curl localhost:8080/healthz` → HTTP 200 `{"status":"ok"}`
- [ ] `curl localhost:8081/healthz` → HTTP 200 `{"status":"ok"}`
- [ ] `make check-postgis` выводит версию PostGIS
- [ ] `make db-shell` открывает psql без ошибок
- [ ] Kafka проверяется только через `docker compose exec`, не через host-порт
- [ ] `.env` отсутствует в git (только `.env.example`)
- [ ] `go work sync` выполняется без ошибок
- [ ] `make test` проходит без ошибок
- [ ] `.dockerignore` присутствует в обоих Go-сервисах
- [ ] README содержит инструкцию `make up`

## Test Plan

### Интеграционный тест окружения (ручной)

1. Клонировать репо в чистую директорию
2. Скопировать `.env.example` → `.env`
3. `make up`
4. Дождаться healthy всех контейнеров
5. `curl localhost:8080/healthz` → 200
6. `curl localhost:8081/healthz` → 200
7. `make check-postgis` → вывод версии PostGIS
8. `docker compose exec kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server localhost:9092 --list` → успешно
9. `make down` — всё останавливается чисто

### Go unit-тесты стабов

- `tracking-gateway`: тест HTTP handler `/healthz` (200, `{"status":"ok"}`)
- `tracking-worker`: тест HTTP handler `/healthz` (200, `{"status":"ok"}`)
- Оба сервиса: тест graceful shutdown по SIGTERM

## Documentation Plan

- `README.md` в корне: быстрый старт, `make up`, требования (Docker, Go 1.22+)
- `services/tracking-gateway/README.md`: описание, переменные окружения, порт 8080
- `services/tracking-worker/README.md`: описание, переменные окружения, порт 8081
- `apps/android-tracker/README.md`: заглушка для будущего Android-клиента
- `apps/web/README.md`: заглушка для будущего web-клиента
- `EPIC.md` актуален на протяжении всего эпика

## Implementation Log

**2026-06-10 (revision 2)** — Добавлены README.md в все placeholder-директории обоих сервисов (`domain/`, `usecase/`, `transport/kafka/`, `transport/grpc/`, `storage/postgres/`, `storage/memory/`) — 12 файлов. Архитектурный скелет теперь зафиксирован в репозитории. Проверены: `go work sync`, `make test`, `make up`, оба `/healthz`, `check-postgis`, Kafka exec, `make down` — все OK. main.go проверен: handlers и бизнес-логика отсутствуют.

**2026-06-10 (revision 1)** — Введено правило Clean Architecture для всех Go-сервисов. Оба сервиса рефакторены:
- `cmd/gateway/` → `cmd/tracking-gateway/`; `cmd/worker/` → `cmd/tracking-worker/`
- Добавлены `internal/app/`, `internal/config/`, `internal/transport/http/`, `internal/domain/`, `internal/usecase/`, `internal/storage/` (placeholder dirs)
- `main.go` — только composition root (config → app → run → shutdown)
- HTTP `/healthz` перенесён в `internal/transport/http/health_handler.go` + `router.go`
- Тесты перенесены в `internal/transport/http/health_handler_test.go`
- Dockerfiles обновлены под новые build-пути
- `.claude/CLAUDE.md` обновлён: добавлено обязательное правило Go Clean Architecture
- Все тесты зелёные, все сервисы `healthy` после пересборки

**2026-06-10** — Создана структура директорий: `services/tracking-gateway`, `services/tracking-worker`, `apps/android-tracker`, `apps/web`, `infra/docker/postgres`.

**2026-06-10** — Написан `docker-compose.yml`: postgres (postgis/postgis:16-3.4), kafka (apache/kafka:3.7.0 KRaft), tracking-gateway, tracking-worker. Kafka не пробрасывается на host. Все сервисы с healthcheck и `depends_on: condition: service_healthy`.

**2026-06-10** — Написаны Go-стабы `tracking-gateway` (`:8080`) и `tracking-worker` (`:8081`): HTTP `/healthz`, graceful shutdown. Добавлены unit-тесты для обоих. Dockerfile на `alpine:3.19` с `curl` для healthcheck.

**2026-06-10** — Создан `go.work` для Go workspaces. Написан `Makefile` (up/down/logs/reset/check-postgis/db-shell/test). Написаны README для корня и всех сервисов/приложений.

**2026-06-10** — Проверка acceptance criteria:
- `docker compose ps` → все 4 сервиса `healthy`
- `curl localhost:8080/healthz` → `{"status":"ok"}`
- `curl localhost:8081/healthz` → `{"status":"ok"}`
- `SELECT PostGIS_Version()` → `3.4 USE_GEOS=1 USE_PROJ=1 USE_STATS=1`
- Kafka проверен через `docker compose exec` (без host-порта)
- `go work sync` → OK
- `go test ./...` → OK оба сервиса

## Final Report

**Дата завершения:** 2026-06-10

**Результат:** все acceptance criteria выполнены.

**Что сделано:**
- Монорепо структура: `services/`, `apps/`, `infra/`
- Docker Compose с 4 сервисами, все `healthy` при старте
- Kafka 3.7.0 в KRaft mode, только внутри `wrany-net`
- PostgreSQL 16 + PostGIS 3.4, инициализирован через `init.sql`
- Go-сервисы `tracking-gateway` (:8080) и `tracking-worker` (:8081) по **Clean Architecture**:
  - `cmd/<service>/main.go` — composition root only
  - `internal/app/` — lifecycle management
  - `internal/config/` — env var loading
  - `internal/transport/http/` — HTTP handler + router
  - `internal/domain/`, `internal/usecase/`, `internal/storage/` — placeholder dirs для будущих эпиков
- Unit-тесты в `internal/transport/http/` — зелёные
- `go.work` для Go workspaces
- Makefile с 7 командами
- `.dockerignore` для обоих Go-сервисов
- README для корня, сервисов и приложений
- `CLAUDE.md` обновлён: обязательное правило Go Clean Architecture для всего проекта

**Решения и компромиссы:**
- `apache/kafka:3.7.0` — официальный образ, KRaft без Zookeeper
- `alpine:3.19` + `curl` в финальном образе — необходим для Docker Compose healthcheck
- `make test` вызывает `go test` явно по каждому сервису, не через `go work`
- Go service foundation сделан под Clean Architecture: transport → usecase → domain, storage → usecase interfaces

**Changed Files (final):**
- `services/tracking-gateway/internal/domain/README.md` (new)
- `services/tracking-gateway/internal/usecase/README.md` (new)
- `services/tracking-gateway/internal/transport/kafka/README.md` (new)
- `services/tracking-gateway/internal/transport/grpc/README.md` (new)
- `services/tracking-gateway/internal/storage/postgres/README.md` (new)
- `services/tracking-gateway/internal/storage/memory/README.md` (new)
- `services/tracking-worker/internal/domain/README.md` (new)
- `services/tracking-worker/internal/usecase/README.md` (new)
- `services/tracking-worker/internal/transport/kafka/README.md` (new)
- `services/tracking-worker/internal/transport/grpc/README.md` (new)
- `services/tracking-worker/internal/storage/postgres/README.md` (new)
- `services/tracking-worker/internal/storage/memory/README.md` (new)

**Test Results:**
- `go work sync` — OK
- `go test ./...` tracking-gateway — OK (`internal/transport/http` pass)
- `go test ./...` tracking-worker — OK (`internal/transport/http` pass)
- `curl localhost:8080/healthz` — `{"status":"ok"}`
- `curl localhost:8081/healthz` — `{"status":"ok"}`
- `make check-postgis` — PostGIS 3.4 USE_GEOS=1 USE_PROJ=1 USE_STATS=1
- `docker compose exec kafka /opt/kafka/bin/kafka-topics.sh --list` — OK (no host port)
- `make down` — all containers stopped cleanly

**Remaining Risks:**
- `transport/grpc/` placeholder — gRPC не будет использоваться в ближайших эпиках; директория зарезервирована
- Makefile `make check-postgis` использует `$${}` для env vars — корректно работает только при наличии `.env`; нужно документировать при онбординге

**Следующий шаг:** EPIC 2 — Auth & Device Registration.
