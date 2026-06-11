# EPIC 05: Tracking Worker Consumer & Raw Location Persistence

## Status

Done

---

## Goal

Реализовать `tracking-worker`, который читает события `location.events.v1` из NATS JetStream durable consumer, валидирует event envelope/payload, идемпотентно сохраняет сырые GPS-точки в Postgres/PostGIS и подтверждает NATS message только после успешной записи.

---

## Context

После EPIC 4 `tracking-gateway` принимает location events через WebSocket, валидирует JWT/device/session/batch и публикует accepted events в NATS JetStream subject `location.events.v1`. ACK клиенту отправляется только после successful JetStream PubAck.

EPIC 4 намеренно не реализовывал:
- tracking-worker NATS consumer
- raw GPS persistence
- TripDetectionEngine, route matching, analytics API, Android client

EPIC 5 реализует первый backend worker pipeline:
```
NATS JetStream location.events.v1
  ↓ durable consumer (tracking-worker-location-consumer)
tracking-worker
  ↓ validate / deserialize / idempotent insert
Postgres/PostGIS raw_location_points
```

Зависимости:
- EPIC 01: Go workspace, Docker Compose, Postgres/PostGIS, NATS
- EPIC 02: users/devices tables, JWT — tracking-worker читает user_id/device_id из trusted events (без FK)
- EPIC 03: `libs/events` contracts, `libs/eventbus` Publisher abstraction, NATS stream WRANY_EVENTS
- EPIC 04: `location.events.v1` publishing from gateway — producer стороны

---

## Problem Analysis

### 1. Consumer abstraction gap
`libs/eventbus` (EPIC 3) реализовал только `Publisher`. Consumer API был отложен. EPIC 5 должен добавить минимальный Consumer interface в `libs/eventbus` без утечки NATS типов в usecase/domain.

### 2. At-least-once delivery
NATS JetStream гарантирует at-least-once. Worker может получить одно сообщение несколько раз (network hiccup, crash before ACK, redelivery after AckWait). Решение: PK `(user_id, device_id, event_id)` + `INSERT ... ON CONFLICT DO NOTHING`.

### 3. ACK semantics
ACK должен происходить только после:
- успешного commit DB insert, ИЛИ
- подтверждённого дубликата (ON CONFLICT DO NOTHING вернул 0 rows, но это ожидаемо).

NACK при transient DB ошибках → redelivery. Это предотвращает потерю данных.

### 4. Dead-letter для невалидных сообщений
Невалидные сообщения (плохой JSON, неизвестный event_type, невалидный payload) не должны блокировать consumer. Стратегия: publish `dead-letter.v1` → ACK original. Если dead-letter publish падает → NACK original.

### 5. Ordering
NATS не даёт Kafka-like partition ordering по ключу. Gateway публикует batch в порядке поступления. Worker хранит каждую точку с `recorded_at`. Future TripDetectionEngine должен делать `ORDER BY (user_id, device_id, recorded_at)` и обрабатывать late/out-of-order points с buffer window. В EPIC 5 ordering не реализуется.

### 6. PostGIS geometry
`geom` — `geometry(Point, 4326)`, создаётся как `ST_SetSRID(ST_MakePoint(lon, lat), 4326)`. Порядок: lon, lat (X, Y) — не lat, lon. Это критично: ошибка порядка даёт точку-зеркало.

### 7. Migration ownership
Текущие миграции живут в `services/tracking-gateway/infra/migrations/`. Для `tracking-worker` нужен отдельный migration path: `services/tracking-worker/infra/migrations/`. Makefile нужно расширить для worker migrations.

---

## Best Practice Research

### NATS JetStream durable consumer
- Durable name = consumer survives restarts, сохраняет позицию.
- `DeliverAll` policy: читать с начала при первом старте.
- `AckExplicit`: ACK только вручную.
- `AckWait`: 30s — время до redelivery если ACK не пришёл.
- `MaxDeliver`: 5 — ограничивает количество redelivery. После исчерпания NATS прекращает доставку этого сообщения. Это НЕ связано с нашим `dead-letter.v1` — тот публикуется явно нашим кодом только для invalid/unprocessable сообщений.
- `Fetch`/`FetchBatch` с timeout вместо push consumer: нет busy loop, проще graceful shutdown.

### Idempotency pattern
```sql
INSERT INTO raw_location_points (...) VALUES (...)
ON CONFLICT (user_id, device_id, event_id) DO NOTHING
```
Возвращаемое число affected rows: 1 = новая строка, 0 = дубликат. Оба случая = ACK.

### Consumer abstraction
```go
type Consumer interface {
    Consume(ctx context.Context, handler MessageHandler) error
    Close() error
}
type MessageHandler func(ctx context.Context, msg Message) error
type Message interface {
    Subject() string
    Data() []byte
    Headers() map[string][]string
    Ack() error
    Nak() error
}
```
NATS-specific реализация живёт в `libs/eventbus/nats/`. `eventbus.Message` используется только в `transport/nats` — usecase его не видит и не вызывает Ack/Nak напрямую.

### ProcessingInput / ProcessingResult
Transport/nats преобразует входящее `eventbus.Message` в `ProcessingInput` (raw payload bytes) и передаёт usecase. Usecase возвращает `ProcessingResult` с действием — Ack или Nak. Transport/nats исполняет Ack/Nak на оригинальном сообщении:

```go
// usecase boundary — никаких NATS imports
type ProcessingInput struct {
    Data []byte
}

type Action int
const (
    ActionAck Action = iota
    ActionNak
)

type ProcessingResult struct {
    Action Action
}
```

---

## Solution Design

### libs/eventbus: добавить Consumer abstraction
Файлы:
- `libs/eventbus/consumer.go` — `Consumer`, `MessageHandler`, `Message` interfaces + `ErrConsume`
- `libs/eventbus/nats/consumer.go` — `JetStreamConsumer` реализация
- `libs/eventbus/nats/consumer_config.go` — конфиг durable consumer

### tracking-worker: новые файлы
```
services/tracking-worker/
  cmd/tracking-worker/main.go              (обновить: добавить DB + consumer wire)
  infra/migrations/
    0001_create_raw_location_points.up.sql
    0001_create_raw_location_points.down.sql
  internal/
    config/config.go                       (расширить: DATABASE_URL, NATS vars)
    domain/
      raw_location_point.go                (новый)
      processing_errors.go                 (новый)
    usecase/
      location_event_processor.go          (новый)
      location_event_processor_test.go     (новый)
    transport/
      nats/
        location_consumer.go               (новый — NATS adapter, вызывает usecase)
        location_consumer_test.go          (новый)
    storage/
      postgres/
        raw_location_repo.go               (новый)
        raw_location_repo_test.go          (новый)
        testhelper_test.go                 (новый)
    app/app.go                             (обновить: wire consumer + processor + repo)
```

### Processing flow (per message)

**transport/nats** (оркестрирует):
```
1. Fetch msg from durable consumer
2. Construct ProcessingInput{Data: msg.Data()}
3. result := processor.Process(ctx, input)   // вызов usecase
4. switch result.Action:
   - ActionAck → msg.Ack()
   - ActionNak → msg.Nak()
```

**usecase** (бизнес-логика, не видит msg/NATS):
```
1. json.Unmarshal data → events.Envelope
2. Validate envelope (event_id, event_type, occurred_at, producer)
3. Check event_type == "location.events.v1"
4. json.Unmarshal envelope.Payload → location.EventV1
5. Validate location payload (lat/lon bounds, accuracy >= 0, activity_type)
6. Map → domain.RawLocationPoint
7. repo.Insert(ctx, point)  [ON CONFLICT DO NOTHING]

8a. insert OK || duplicate           → return ProcessingResult{ActionAck}
8b. transient DB error               → return ProcessingResult{ActionNak}

9.  invalid envelope/payload:
    publisher.Publish(ctx, dead-letter.v1, envelope)
    9a. dead-letter publish OK       → return ProcessingResult{ActionAck}
    9b. dead-letter publish failed   → return ProcessingResult{ActionNak}
```

### DB schema
```sql
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE TABLE raw_location_points (
    user_id         UUID NOT NULL,
    device_id       UUID NOT NULL,
    event_id        TEXT NOT NULL,
    recorded_at     TIMESTAMPTZ NOT NULL,
    received_at     TIMESTAMPTZ NOT NULL,
    stored_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    lat             DOUBLE PRECISION NOT NULL CHECK (lat >= -90 AND lat <= 90),
    lon             DOUBLE PRECISION NOT NULL CHECK (lon >= -180 AND lon <= 180),
    geom            geometry(Point, 4326) NOT NULL,
    accuracy_m      DOUBLE PRECISION NOT NULL CHECK (accuracy_m >= 0),
    speed_mps       DOUBLE PRECISION NULL CHECK (speed_mps IS NULL OR speed_mps >= 0),
    bearing_deg     DOUBLE PRECISION NULL CHECK (bearing_deg IS NULL OR (bearing_deg >= 0 AND bearing_deg <= 360)),
    activity_type   TEXT NOT NULL,
    activity_confidence DOUBLE PRECISION NULL CHECK (activity_confidence IS NULL OR (activity_confidence >= 0 AND activity_confidence <= 1)),
    battery_level   DOUBLE PRECISION NULL CHECK (battery_level IS NULL OR (battery_level >= 0 AND battery_level <= 1)),
    source          TEXT NOT NULL,
    PRIMARY KEY (user_id, device_id, event_id)
);
CREATE INDEX idx_raw_location_points_user_device_recorded_at ON raw_location_points(user_id, device_id, recorded_at);
CREATE INDEX idx_raw_location_points_recorded_at ON raw_location_points(recorded_at);
CREATE INDEX idx_raw_location_points_geom ON raw_location_points USING GIST (geom);
```

---

## Architecture Notes

- `tracking-worker` не имеет FK на `users`/`devices` — user_id/device_id берутся из trusted NATS events.
- **Usecase boundary**: `location_event_processor.go` принимает `ProcessingInput{Data []byte}`, возвращает `ProcessingResult{Action}`. Не импортирует `eventbus.Message`. Не вызывает Ack/Nak/Term. Зависит только от `libs/events`, `libs/eventbus.Publisher`, domain types.
- **Transport boundary**: `transport/nats/location_consumer.go` читает `eventbus.Message`, строит `ProcessingInput`, вызывает usecase, получает `ProcessingResult`, вызывает `msg.Ack()` или `msg.Nak()`. NATS типы не выходят за пределы этого пакета.
- **Storage**: импортирует только `pgx/v5` и domain types. Не знает о NATS.
- **Dead-letter**: usecase получает `eventbus.Publisher` как dependency. При invalid message публикует `dead-letter.v1`. Если publish успешен → возвращает ActionAck. Если publish failed → возвращает ActionNak. Transport/nats исполняет Ack/Nak.
- **Transient DB errors**: usecase возвращает ActionNak. Transport вызывает `msg.Nak()`. Redelivery. Dead-letter.v1 для transient ошибок в EPIC 5 не публикуется.
- **MaxDeliver semantics**: `MaxDeliver = 5` ограничивает redelivery на стороне NATS. После исчерпания NATS перестаёт доставлять сообщение. Это не связано с нашим `dead-letter.v1` subject.
- **Consumer loop**: `Fetch(batch, timeout)` в цикле, context cancellation останавливает loop. No busy loop: Fetch блокирует до timeout или batch full.
- **Graceful shutdown**: context cancel → loop exit → `consumer.Close()` → `db.Close()`.
- **Migrations**: `services/tracking-worker/infra/migrations/` — отдельный путь, не пересекается с gateway migrations. Makefile targets `migrate-worker-up/down` используют именно этот путь.

---

## Tasks

- [ ] T01: Проверить git state, создать ветку, создать EPIC.md *(выполняется сейчас)*
- [ ] T02: Обновить `.claude/EPICS.md` с scope EPIC 5
- [ ] T03: Добавить `Consumer`/`Message`/`MessageHandler` interfaces в `libs/eventbus/consumer.go`
- [ ] T04: Реализовать `JetStreamConsumer` в `libs/eventbus/nats/consumer.go`
- [ ] T05: Создать `services/tracking-worker/infra/migrations/0001_create_raw_location_points.{up,down}.sql`
- [ ] T06: Расширить `services/tracking-worker/internal/config/config.go` (DATABASE_URL, NATS vars)
- [ ] T07: Реализовать `domain.RawLocationPoint` и `domain.ProcessingError` types
- [ ] T08: Реализовать `storage/postgres/raw_location_repo.go` с idempotent insert
- [ ] T09: Реализовать `usecase/location_event_processor.go`
- [ ] T10: Реализовать `transport/nats/location_consumer.go` (NATS adapter)
- [ ] T11: Обновить `internal/app/app.go` — wire DB + consumer + processor + repo
- [ ] T12: Обновить `cmd/tracking-worker/main.go` — graceful shutdown с DB и consumer
- [ ] T13: Обновить `docker-compose.yml` и `.env.example` для worker
- [ ] T14: Обновить `Makefile` — добавить `migrate-worker-up`, `migrate-worker-down`
- [ ] T15: Написать unit tests (processor, repo mock, dead-letter paths)
- [ ] T16: Написать integration tests (Postgres + NATS)
- [ ] T17: Обновить docs (`tracking-worker.md`, `events.md`, `README.md`)
- [ ] T18: `make test` pass, `make up` + smoke test
- [ ] T19: Заполнить Implementation Log и Final Report

---

## Acceptance Criteria

- [ ] `tracking-worker` потребляет `location.events.v1` из NATS JetStream
- [ ] Consumer durable name: `tracking-worker-location-consumer`
- [ ] Explicit ACK: ACK только после DB commit или подтверждённого дубликата
- [ ] Валидный event → строка в `raw_location_points`
- [ ] Дубликат `(user_id, device_id, event_id)` → 0 новых строк, ACK
- [ ] Невалидный event → `dead-letter.v1` опубликован, оригинал ACKed
- [ ] Если dead-letter publish падает → оригинал NACKed
- [ ] Transient DB ошибка → NACK, redelivery
- [ ] `raw_location_points.geom` = `ST_SetSRID(ST_MakePoint(lon, lat), 4326)`
- [ ] Индекс `(user_id, device_id, recorded_at)` существует
- [ ] GIST индекс на `geom` существует
- [ ] `make test` passes
- [ ] `make up` стартует tracking-worker с активным consumer
- [ ] Manual smoke test: event → DB row
- [ ] EPIC 6 не начат

---

## Test Plan

### Unit tests

**Processor (usecase/location_event_processor_test.go)**
- [ ] Валидный envelope + payload → RawLocationPoint → repo.Insert вызван
- [ ] Невалидный JSON envelope → dead-letter published, ACK
- [ ] Неизвестный event_type → dead-letter published, ACK
- [ ] Неизвестный event_version → dead-letter published, ACK
- [ ] Невалидный location payload (lat > 90) → dead-letter published, ACK
- [ ] repo.Insert возвращает transient error → NACK
- [ ] dead-letter publish failure → NACK original
- [ ] Дубликат (ON CONFLICT → 0 rows) → ACK
- [ ] Usecase не импортирует NATS packages (compile-time rule / import check)
- [ ] lon/lat порядок в geom корректен

**Repository (storage/postgres/raw_location_repo_test.go)**
- [ ] Insert valid point → row exists
- [ ] Insert duplicate → 0 affected rows, no error
- [ ] Insert invalid lat → constraint violation

### Integration tests
- [ ] Postgres migration применяется без ошибок
- [ ] PostGIS extension создаётся
- [ ] Insert RawLocationPoint → row exists с корректным geom
- [ ] ST_X(geom) = lon, ST_Y(geom) = lat
- [ ] ST_SRID(geom) = 4326
- [ ] Дублирующий insert → 1 строка в таблице
- [ ] NATS: опубликовать `location.events.v1` → worker сохраняет строку и ACKs
- [ ] NATS: невалидный message → появляется в `dead-letter.v1`

### Manual smoke test
```
1. make up
2. make migrate-worker-up
3. make nats-init
4. publish valid location.events.v1 (nats-box или dev tool)
5. SELECT * FROM raw_location_points → 1 строка
6. publish тот же event_id → COUNT(*) не изменился
7. publish invalid payload → событие в dead-letter.v1
```

---

## Documentation Plan

- [ ] `docs/architecture/tracking-worker.md` — consumer architecture, ACK semantics, dead-letter, ordering limitations
- [ ] `docs/contracts/events.md` — добавить примеры использования `location.events.v1` consumer
- [ ] `services/tracking-worker/README.md` — env vars, migration, running
- [ ] `.env.example` — новые WORKER_* и NATS consumer vars
- [ ] `.claude/EPICS.md` — добавить EPIC 05 scope

---

## Implementation Log

### 2026-06-11

**libs/eventbus/consumer.go** — добавлены `Message`, `MessageHandler`, `Consumer` интерфейсы. Нет утечки NATS-типов наружу.

**libs/eventbus/nats/consumer.go** — `JetStreamConsumer` с pull-loop, `ConsumerConfig`, `natsMessage` wrapper. `CreateOrUpdateConsumer` с `DeliverAllPolicy` + `AckExplicitPolicy`.

**services/tracking-worker/infra/migrations/0001_create_raw_location_points.up.sql** — таблица с PostGIS geometry, PK `(user_id, device_id, event_id)`, CHECK constraints, GIST index на geom.

**internal/domain/** — `RawLocationPoint`, `ErrTransient`, `ErrInvalidMessage`.

**internal/migrations/runner.go** — `Run` + `RunWithTable` (изолированная таблица мигрций на сервис через `x-migrations-table` query param).

**internal/storage/postgres/raw_location_repo.go** — `INSERT ... ON CONFLICT DO NOTHING`, `ST_SetSRID(ST_MakePoint($lon, $lat), 4326)`.

**internal/usecase/location_event_processor.go** — `ProcessingInput`/`ProcessingResult`/`Action` boundary. Dead-letter flow: invalid → publish `dead-letter.v1` → ActionAck; DB error → ActionNak без dead-letter. Фикс: `json.RawMessage` из невалидного JSON → encode как JSON string.

**internal/transport/nats/location_consumer.go** — единственное место вызова `msg.Ack()`/`msg.Nak()`. Usecase не знает о NATS.

**internal/app/app.go** — `bus.EnsureStream(ctx)` добавлен для идемпотентного создания stream при старте worker.

**docker-compose.yml** — `restart: always` на gateway и worker; worker получил все NATS env vars.

**entrypoint.sh** — `x-migrations-table=tracking_worker_schema_migrations` через query param в DATABASE_URL (изоляция от gateway's `schema_migrations`).

**Тесты** — unit (usecase + transport/nats), integration (testcontainers Postgres для storage), все прошли.

**Smoke test** — publish `location.events.v1` → строка в `raw_location_points`; дубликат → 1 строка; невалидный JSON → `dead-letter.v1` получен.

---

## Final Report

### Что реализовано

EPIC 5 полностью реализован:

1. **Consumer abstraction** — `eventbus.Consumer`/`Message` интерфейсы в `libs/eventbus`; NATS реализация в `libs/eventbus/nats/consumer.go`.

2. **tracking-worker** сервис:
   - Clean Architecture: domain → usecase → transport/nats, storage/postgres
   - NATS durable pull consumer `tracking-worker-location-consumer` на stream `WRANY_EVENTS` subject `location.events.v1`
   - Идемпотентный INSERT с `ON CONFLICT DO NOTHING` (PK: user_id, device_id, event_id)
   - PostGIS geometry: `ST_SetSRID(ST_MakePoint(lon, lat), 4326)` — lon=X, lat=Y
   - Dead-letter.v1 для невалидных/неразбираемых сообщений
   - Явная граница ACK/NAK: только `transport/nats` вызывает Ack/Nak

3. **Migrations** — изолированы в `services/tracking-worker/infra/migrations/` с отдельной таблицей `tracking_worker_schema_migrations`.

4. **Тесты** — 12 unit-тестов (usecase: 8, transport/nats: 4) + 4 интеграционных (testcontainers Postgres), все зелёные.

### Решённые нетривиальные проблемы

- `json.RawMessage` требует валидный JSON → невалидный originalData кодируется как JSON-строка перед вставкой в dead-letter payload
- Shared `schema_migrations` между сервисами → `x-migrations-table` query param
- `EnsureStream` нужно вызывать явно в worker (gateway создаёт stream, но worker может стартовать первым)
- Race condition при параллельном старте → `restart: always` + idempotent `EnsureStream`

### Acceptance Criteria

- [x] Worker читает `location.events.v1` из NATS JetStream durable consumer
- [x] Valid event → строка в `raw_location_points`, ACK
- [x] Duplicate event → ON CONFLICT DO NOTHING, ACK (без error)
- [x] Invalid JSON / невалидный envelope → dead-letter.v1, ACK
- [x] DB transient error → NAK (retry)
- [x] Dead-letter publish failure → NAK
- [x] PostGIS geom: ST_X=lon, ST_Y=lat
- [x] Usecase не импортирует NATS-пакеты
- [x] Worker migrations изолированы от gateway migrations
- [x] `make test` зелёный по всем модулям
- [x] `make up` → оба сервиса healthy
- [x] Smoke test пройден вручную
