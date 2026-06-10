# EPIC 04: WebSocket Tracker Protocol & Gateway Ingestion

## Status

Done

---

## Goal

Реализовать WebSocket endpoint в `tracking-gateway`, который принимает location events от Android tracker-клиента, валидирует пользователя/устройство/пачку событий и публикует accepted events в NATS JetStream subject `location.events.v1`.

ACK клиенту отправляется только после успешного `PubAck` от JetStream.

---

## Context

Построено поверх:

- **EPIC 1**: docker compose, сервисные заглушки, Makefile
- **EPIC 2**: auth (JWT), device registration, `users`/`devices` таблицы, auth middleware
- **EPIC 3**: `libs/events` (envelope + contracts), `libs/eventbus` (Publisher interface + NATS JetStream adapter), NATS subjects

Поток данных после EPIC 4:

```
Android Tracker
  ↓ WebSocket (JWT auth)
tracking-gateway /v1/ws/tracker
  ↓ validate / auth / device / dedup
NATS JetStream location.events.v1
  ↓
future tracking-worker (EPIC 5+)
```

Android-клиент, tracking-worker consumer, TripDetectionEngine и analytics API — вне scope этого эпика.

---

## Problem Analysis

### Проблемы, которые нужно решить

1. **WebSocket upgrade с JWT auth**: HTTP endpoint должен уметь валидировать Bearer token до upgrade.
2. **Session state**: до `session.start` батчи недопустимы. Gateway хранит состояние сессии в памяти per-connection.
3. **Device ownership**: `session.start.device_id` должен принадлежать аутентифицированному user_id.
4. **Batch validation**: каждое событие в батче валидируется независимо. Принятые/rejected/дублированные разделяются.
5. **Dedup**: одновременный client-retry возможен при обрыве соединения. `event_id` должен быть идемпотентным.
6. **Transactional consistency (dedup ledger vs NATS publish)**:

   Три варианта:
   - **A**: Insert ledger → publish NATS. Риск: NATS недоступен после insert → event не опубликован, но считается дублем при retry.
   - **B**: Publish NATS → insert ledger. Риск: crash между publish и insert → при retry event публикуется дважды (но JetStream dedup window с `Nats-Msg-Id` защищает).
   - **C**: Статусная таблица с `status: publishing|published|failed` + фоновый sweeper.

   **MVP выбор: вариант B** — publish first, then insert dedup ledger after successful PubAck.

   Обоснование:
   - JetStream `Nats-Msg-Id = event_id` обеспечивает at-most-once publish в пределах dedup window (по умолчанию 2 минуты).
   - Лиджер нужен только для protocol-level `duplicated` response клиенту.
   - Потеря лиджер-insert после успешного PubAck → клиент при retry увидит `accepted` вместо `duplicated` → event публикуется ещё раз → JetStream dedup window поглощает повтор.
   - Вариант A создаёт более опасную потерю: accepted event не попадает в NATS, но client считает его synced.
   - Вариант C избыточен для MVP.

7. **Back-pressure**: NATS unavailable → `EVENT_BUS_UNAVAILABLE`, клиент НЕ помечает события synced.
8. **Connection lifecycle**: read/write deadline, ping/pong heartbeat, max message size 256 KB, graceful close.

---

## Best Practice Research

### WebSocket в Go

- Стандартная библиотека `golang.org/x/net/websocket` — устаревшая, без поддержки control frames.
- **`github.com/gorilla/websocket`** — де-факто стандарт. Поддерживает ping/pong, deadlines, max message size. Уже используется в Go-экосистеме повсеместно.
- **`nhooyr.io/websocket`** (coder/websocket) — более новый, context-aware, но меньше battle-tested в production.
- MVP выбор: **gorilla/websocket** — стабильный, хорошо документированный.

### Session state per-connection

- Состояние сессии (session_id, device_id, user_id) хранится в памяти per-connection goroutine.
- Нет shared state между connections — горутины полностью изолированы.
- При обрыве соединения состояние уничтожается автоматически.

### Dedup ledger

- Postgres таблица `ingested_location_events` с составным `PRIMARY KEY (user_id, device_id, event_id)`.
- Dedup scope: `event_id` уникален в контексте конкретной пары `(user_id, device_id)`, а не глобально.
- Два разных устройства могут независимо сгенерировать одинаковый `event_id` — это не должно считаться дублем.
- `INSERT ... ON CONFLICT DO NOTHING` после успешного PubAck — атомарная защита от race condition при concurrent retry.
- Conflict при insert после PubAck не является fatal error: publish уже подтверждён JetStream, ACK клиенту остаётся успешным.

### NATS JetStream headers

- `Nats-Msg-Id` = `event_id` — стандартный JetStream dedup header.
- Дополнительные headers `Wrany-User-Id`, `Wrany-Device-Id`, `Wrany-Correlation-Id` — кастомные.

---

## Solution Design

### Endpoint

```
GET /v1/ws/tracker
Authorization: Bearer <access_token>
```

### Message flow

```
Client → session.start
Server → session.accepted (или error DEVICE_NOT_REGISTERED)

Client → location.batch
Server → location.batch.ack (accepted/duplicated/rejected split)
         ACK отправляется ТОЛЬКО после PubAck от JetStream
```

### Dedup flow (вариант B)

```
1. Для каждого event_id в батче:
   a. SELECT 1 FROM ingested_location_events
      WHERE user_id = $1 AND device_id = $2 AND event_id = $3
      - Найден → добавить в duplicated, пропустить
      - Не найден → добавить в accepted-candidates

2. Для каждого accepted-candidate:
   a. Publish to NATS (с Nats-Msg-Id = event_id)
   b. Дождаться PubAck
      - Ошибка → добавить в rejected (EVENT_BUS_UNAVAILABLE)
      - PubAck OK → INSERT INTO ingested_location_events (user_id, device_id, event_id, received_at)
                    ON CONFLICT DO NOTHING
        - Conflict: не fatal, publish уже подтверждён, ACK клиенту успешный

3. Собрать ack response и отправить клиенту
```

### Error handling

- NATS unavailable для всего батча → `error` message с `EVENT_BUS_UNAVAILABLE`.
- Частичный сбой (отдельные события) → отдельные записи в `rejected`.

---

## Architecture Notes

### Структура файлов

```
services/tracking-gateway/
  cmd/tracking-gateway/main.go              # composition root (не меняется)
  internal/
    domain/
      tracker_session.go                    # TrackerSession, SessionConfig
      location_event.go                     # LocationEvent, ActivityType
      ingestion_errors.go                   # error codes: UNAUTHORIZED, DEVICE_NOT_REGISTERED, ...
    usecase/
      tracker_ingestion.go                  # TrackerIngestionUseCase (interface + impl)
      tracker_ingestion_test.go
    transport/
      http/
        websocket_handler.go                # upgrade, read loop, message dispatch
        websocket_protocol.go               # message structs (session.start, location.batch, ...)
        websocket_errors.go                 # error response helpers
        websocket_handler_test.go
    storage/
      postgres/
        ingestion_dedup_repo.go             # IngestionDedupRepo: CheckAndInsert
        ingestion_dedup_repo_test.go
  infra/
    migrations/
      0004_create_ingested_location_events.up.sql
      0004_create_ingested_location_events.down.sql
```

### Dependency direction

```
websocket_handler → TrackerIngestionUseCase (interface)
TrackerIngestionUseCase → DeviceRepo (EPIC 2 interface), IngestionDedupRepo (interface), Publisher (libs/eventbus)
ingestion_dedup_repo → postgres (pgx/v5)
```

### libs/eventbus integration

- `tracking-gateway/internal/app/app.go` инициализирует NATS JetStream adapter из `libs/eventbus/nats`.
- Передаёт `Publisher` в usecase через dependency injection.
- Usecase не импортирует `nats` напрямую.

### Validation layers

Два уровня валидации с чёткими границами:

- **Transport layer** (`websocket_handler.go`): WebSocket max message size через `conn.SetReadLimit(WS_MAX_MESSAGE_SIZE_BYTES)`. Срабатывает до десериализации JSON.
- **Business/usecase layer** (`tracker_ingestion.go`): batch size limit (>100 events), required fields, lat/lon ranges, accuracy, activity_type, optional field ranges. Возвращает per-event `rejected` список с причинами.

### WebSocket Origin policy

- `WS_ALLOWED_ORIGINS` — comma-separated allowlist browser origins (например `https://app.wrany.io`).
- Empty origin разрешён всегда — необходимо для mobile/non-browser clients (Android WebSocket).
- Browser origin проверяется по allowlist.
- Не использовать `CheckOrigin: return true` — это отключает проверку полностью.
- Реализация: custom `CheckOrigin` func в upgrader.

### Config

Новые env vars (добавить в `.env.example`):

```
WS_MAX_MESSAGE_SIZE_BYTES=262144
WS_READ_DEADLINE_SEC=60
WS_WRITE_DEADLINE_SEC=10
WS_PING_INTERVAL_SEC=30
WS_MAX_BATCH_SIZE=100
WS_ALLOWED_ORIGINS=
```

`WS_ALLOWED_ORIGINS` — пустое значение означает: разрешены только empty origin (mobile). Для локальной разработки можно добавить `http://localhost:3000`.

---

## Tasks

- [x] **T01** Обновить `.claude/EPICS.md` с EPIC 4 scope
- [x] **T02** Создать migration `0004_create_ingested_location_events`
- [x] **T03** Добавить domain types: `TrackerSession`, `LocationEvent`, `ActivityType`, error codes
- [x] **T04** Добавить WebSocket protocol structs (`websocket_protocol.go`)
- [x] **T05** Добавить error response helpers (`websocket_errors.go`)
- [x] **T06** Добавить `IngestionDedupRepo` interface + Postgres реализация
- [x] **T07** Добавить `TrackerIngestionUseCase` interface + реализация
- [x] **T08** Интегрировать device validation из EPIC 2 (DeviceRepo/DeviceUseCase)
- [x] **T09** Добавить `websocket_handler.go` с upgrade, read loop, dispatch
- [x] **T10** Wiring: добавить `Publisher` в `app.go`, подключить usecase, зарегистрировать route
- [x] **T11** Добавить config vars (`WS_*`, `WS_ALLOWED_ORIGINS`) в `config.go`
- [x] **T12** Unit tests: protocol, validation, usecase mocks
- [x] **T13** Integration tests: dedup repo с testcontainers (Postgres)
- [x] **T14** Обновить документацию: `docs/contracts/websocket-tracker-protocol.md`, `docs/architecture/tracker-ingestion.md`
- [x] **T15** Обновить `.env.example`
- [x] **T16** `make test` проходит, ручная проверка WebSocket flow
- [x] **T17** Заполнить Implementation Log и Final Report

---

## Acceptance Criteria

- `GET /v1/ws/tracker` существует
- Без JWT → 401 до upgrade
- `session.start` обязателен перед `location.batch`
- `session.start` валидирует device_id (UUID формат)
- Device должен принадлежать authenticated user
- Unknown device → `DEVICE_NOT_REGISTERED`
- `session.accepted` содержит session_id, server_time, config
- `location.batch` валидирует все события
- Batch size limit (100 events) работает
- Message size limit (256 KB) работает (transport layer, SetReadLimit)
- Batch size limit (100 events) работает (usecase layer)
- Non-browser / empty Origin разрешён (Android client)
- Browser Origin проверяется по WS_ALLOWED_ORIGINS allowlist
- CheckOrigin не использует `return true` без ограничения
- Invalid events → `rejected` с причиной
- Accepted events публикуются в NATS `location.events.v1`
- ACK отправляется ТОЛЬКО после успешного JetStream PubAck
- NATS unavailable → `EVENT_BUS_UNAVAILABLE`, клиент не помечает события synced
- Дублированный `event_id` для той же пары `(user_id, device_id)` → `duplicated`, повторной публикации нет
- Conflict при insert в лиджер после PubAck не является ошибкой: ACK клиенту успешный
- `make test` проходит
- `tracking-worker` не модифицирован (кроме документации при необходимости)
- EPIC 5 не начат

---

## Test Plan

### Unit tests

**Protocol (transport layer):**
- Decode/encode всех message types
- Error response format

**Transport validation:**
- `session.start` validation: missing device_id, invalid UUID
- Message size limit: `SetReadLimit` установлен корректно (transport, не usecase)

**Business/usecase validation:**
- `location.batch`: invalid lat/lon
- `location.batch`: invalid accuracy (negative)
- `location.batch`: invalid activity_type enum
- `location.batch`: batch too large (>100 events) → `BATCH_TOO_LARGE`
- `location.batch`: optional fields valid ranges: speed, bearing, confidence, battery

**Usecase behaviour:**
- Device not registered → `DEVICE_NOT_REGISTERED`
- Accepted/rejected/duplicated split из одного батча
- Publisher failure → `EVENT_BUS_UNAVAILABLE`
- Partial batch failure: часть accepted, часть rejected
- Conflict при insert после PubAck не меняет ACK на error

**DedupRepo:**
- CheckAndInsert: first call → not found, insert succeeds
- CheckAndInsert: second call с тем же `(user_id, device_id, event_id)` → conflict, returns duplicate
- Same `event_id` разных `device_id` — не дублируются (независимые записи)
- Race/concurrent: два concurrent вызова с одним `event_id` → ровно один `accepted`, один `duplicated`

**Origin policy:**
- Empty origin → allowed (mobile)
- Known browser origin из allowlist → allowed
- Unknown browser origin не из allowlist → rejected

### Integration tests

- WebSocket connect без token → rejected (401 / close)
- WebSocket connect с valid token → upgrade OK
- `session.start` с unknown device → `DEVICE_NOT_REGISTERED`
- `session.start` с registered device → `session.accepted`
- `location.batch` перед `session.start` → `SESSION_NOT_ACCEPTED`
- Valid `location.batch` → `location.batch.ack` с accepted
- Duplicate `(user_id, device_id, event_id)` → в `duplicated`, второй публикации нет
- Тот же `event_id` от другого device → `accepted` (не дубль)
- Partial invalid batch → accepted + rejected
- NATS unavailable → `EVENT_BUS_UNAVAILABLE`, событие не в лиджере
- Published event виден в NATS stream `WRANY_EVENTS`

### Manual check

1. `make up`
2. `make migrate-up`
3. Регистрация user: `POST /v1/auth/register`
4. Login: `POST /v1/auth/login` → access_token
5. Регистрация device: `POST /v1/devices/register`
6. WebSocket connect: `GET /v1/ws/tracker` с `Authorization: Bearer <token>`
7. Отправить `session.start` с device_id
8. Получить `session.accepted`
9. Отправить `location.batch`
10. Получить `location.batch.ack`
11. `make nats-streams` → убедиться что events в stream

---

## Documentation Plan

Создать:
- `docs/contracts/websocket-tracker-protocol.md` — endpoint, auth, message types, examples, error codes, ACK semantics, dedup semantics, ordering limitations
- `docs/architecture/tracker-ingestion.md` — architecture diagram, dedup strategy, NATS publish flow

Обновить:
- `services/tracking-gateway/README.md`
- `.env.example` (новые WS_* переменные)
- `.claude/EPICS.md` (добавить EPIC 4 scope)

---

## Implementation Log

**2026-06-10 — Initial implementation (commit 7b88d63)**

- Migration `0004_create_ingested_location_events` с PK `(user_id, device_id, event_id)` и индексом по `received_at`.
- Config: добавлены `NatsURL`, `NatsStream`, `WSMaxMessageSizeBytes`, `WSReadDeadlineSec`, `WSWriteDeadlineSec`, `WSPingIntervalSec`, `WSMaxBatchSize`, `WSAllowedOrigins`.
- Domain: `TrackerSession`, `LocationEvent`, `ActivityType` enum, `RejectedEvent`, `IngestionErrorCode` константы.
- Protocol structs: `WsMessage`, `SessionStartPayload`, `SessionAcceptedPayload`, `LocationBatchPayload`, `LocationBatchAckPayload`, `ErrorPayload` и вспомогательные типы.
- WebSocket error helpers: `sendWSError`, `sendWSMessage`.
- `DeviceRepository` interface расширен методом `FindByUserAndDeviceID`; Postgres реализация добавлена.
- `IngestionDedupRepo`: `IsDuplicate` (SELECT EXISTS), `MarkPublished` (INSERT ON CONFLICT DO NOTHING). PK `(user_id, device_id, event_id)`.
- `TrackerIngestionUseCase`: `StartSession` (device ownership), `IngestBatch` (batch size check → per-event validation → dedup check → NATS publish → ledger insert). Conflict после PubAck — не ошибка.
- `TrackerHandler`: upgrade с кастомным CheckOrigin (empty origin → allowed; browser origin → allowlist), `SetReadLimit`, ping goroutine, read loop, dispatch.
- Router: `GET /v1/ws/tracker` за `AuthMiddleware`.
- App wiring: `IngestionDedupRepo`, NATS Publisher (или `NopPublisher`), `TrackerIngestionUseCase`, передача в router.
- `NopPublisher` добавлен в `libs/eventbus`.
- Unit tests: 13 usecase, 7 protocol, 11 handler — итого 31 новых тестов.
- Integration tests: 5 тестов для `IngestionDedupRepo` через testcontainers.
- Документация: `docs/contracts/websocket-tracker-protocol.md`, `docs/architecture/tracker-ingestion.md`.
- `.env.example` обновлён с WS_* переменными.

**2026-06-10 — Docker & smoke test (commit 9d50de7)**

- Dockerfile: контекст сборки расширен до корня workspace; `ENV GOWORK=off` для использования replace directives вместо go workspace.
- Dockerfile: `sed -i 's/\r//' entrypoint.sh` для устранения CRLF на Windows.
- `go mod tidy` с `GOWORK=off` для генерации go.sum с транзитивными зависимостями NATS.
- `docker-compose.yml`: добавлены `DATABASE_URL`, `JWT_SECRET`, `NATS_URL`, `NATS_STREAM`, `WS_ALLOWED_ORIGINS` в tracking-gateway.
- `cmd/wstest/main.go`: smoke test инструмент для ручной проверки WebSocket flow.

**Ручная проверка пройдена:**
- `make up` → все сервисы запустились.
- `make nats-check` → `{"status":"ok"}`.
- `make nats-streams` → stream `WRANY_EVENTS` виден.
- `/healthz` → `{"status":"ok"}`.
- WebSocket flow: register → login → device register → WS connect → `session.accepted` → `location.batch` → `location.batch.ack` → событие в NATS.

---

## Final Report

### Статус: Done

### Что реализовано

EPIC 04 реализует полный цикл приёма location событий от Android tracker через WebSocket с гарантией "ACK только после PubAck".

**Новые компоненты:**

| Компонент | Файл | Назначение |
|-----------|------|------------|
| Migration 0004 | `infra/migrations/0004_*` | Dedup ledger с PK `(user_id, device_id, event_id)` |
| Domain types | `internal/domain/` | TrackerSession, LocationEvent, IngestionErrorCode |
| Protocol structs | `transport/http/websocket_protocol.go` | Wire-формат сообщений |
| TrackerIngestionUseCase | `usecase/tracker_ingestion.go` | Бизнес-логика: validate → dedup → publish → ledger |
| IngestionDedupRepo | `storage/postgres/ingestion_dedup_repo.go` | IsDuplicate + MarkPublished |
| TrackerHandler | `transport/http/websocket_handler.go` | WebSocket upgrade, read loop, dispatch |
| NopPublisher | `libs/eventbus/eventbus.go` | Fallback без NATS |
| wstest | `cmd/wstest/main.go` | Smoke test tool |

**Протокол:**
- `session.start` → `session.accepted` (device validated)
- `location.batch` → `location.batch.ack` (only after PubAck)
- `error` + `EVENT_BUS_UNAVAILABLE` при недоступности NATS

**Dedup стратегия (Variant B: publish-first):**
- IsDuplicate → publish → MarkPublished (ON CONFLICT DO NOTHING)
- Conflict после PubAck — не ошибка

**Origin policy:**
- Empty origin → allowed (Android)
- Browser origin → проверка по `WS_ALLOWED_ORIGINS` allowlist

### Тесты

| Пакет | Тестов | Тип |
|-------|--------|-----|
| `internal/usecase` | 17 | unit |
| `internal/transport/http` | 18 | unit |
| `internal/storage/postgres` | 5 | integration (testcontainers) |

Все тесты проходят (`go test ./...`).

### Что НЕ реализовано (out of scope)

- Android client
- tracking-worker NATS consumer
- TripDetectionEngine
- Raw GPS persistence
- Route matching, analytics API

### Известные ограничения MVP

- `cmd/wstest/main.go` — dev tool, не production binary; помечен `// +build ignore`.
- Integration tests требуют Docker (testcontainers); в CI нужен Docker daemon.
- `NopPublisher` всегда возвращает `ErrPublish` — при `NATS_URL=""` все batch будут `EVENT_BUS_UNAVAILABLE`. Предназначен для юнит-тестов.

### Коммиты

- `7b88d63` epic(04): implement WebSocket tracker protocol and gateway ingestion
- `9d50de7` epic(04): fix Docker build for workspace libs and add WebSocket smoke test
