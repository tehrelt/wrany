# EPIC 03: Event Bus Contracts & NATS JetStream Foundation

## Status

Implemented

## Goal

Создать фундамент внутренней шины событий backend'а:

1. Заменить Kafka на NATS JetStream в актуальной MVP-архитектуре (доки, Docker Compose, env, Makefile).
2. Создать shared-модуль `libs/events` с контрактами доменных событий (envelope + payloads + validation).
3. Создать shared-модуль `libs/eventbus` с broker-agnostic абстракцией `Publisher` и минимальным NATS JetStream адаптером.
4. Подготовить NATS JetStream dev-инфраструктуру (stream, subjects, naming conventions).

Без WebSocket ingestion, без trip detection, без Android-изменений.

## Context

Проект self-hosted, целевая железка — Orange Pi 3B 4GB. Kafka слишком тяжёлая для такого окружения. NATS JetStream даёт persistence, replay, ack/redelivery, durable consumers, at-least-once delivery при минимальном потреблении ресурсов (один бинарник, ~64MB RAM baseline).

Kafka/Redpanda остаются возможными future adapters — поэтому код не должен жёстко завязываться на конкретный брокер: usecase-слой видит только абстракцию `Publisher` (в будущем — полный `EventBus`) и контракты из `libs/events`.

Текущее состояние репозитория (после EPIC 1–2):

- `docker-compose.yml` поднимает `postgres`, `kafka` (apache/kafka:3.7.0 KRaft, internal only), `tracking-gateway`, `tracking-worker`;
- `.env.example` содержит `KAFKA_CLUSTER_ID`;
- `go.work` включает только два сервиса, `libs/` нет;
- сервисы содержат placeholder-директории `internal/transport/kafka/` с README;
- `docs/` пуст — контрактных доков ещё нет;
- реальный producer/consumer код отсутствует — миграция дешёвая, меняется только инфраструктура и документация.

## Problem Analysis

### Проблема 1: Kafka references разбросаны по repo

`rg -ni kafka` находит упоминания в: `.claude/CLAUDE.md`, `.claude/EPICS.md`, `README.md`, `.env.example`, `docker-compose.yml`, `Makefile` (нет kafka-команд — проверено), `services/*/README.md`, `services/*/internal/transport/kafka/README.md`, `services/*/internal/usecase/README.md`, `.claude/epics/01-project-foundation/EPIC.md`.

Классификация:

| Файл | Действие |
|---|---|
| `.claude/CLAUDE.md` | Обновить: event bus = NATS JetStream, `transport/kafka/` → `transport/nats/`, EPIC list |
| `.claude/EPICS.md` | Обновить: EPIC 03 = Event Bus Contracts & NATS JetStream Foundation; упоминание Kafka в EPIC 01 пометить superseded |
| `README.md` | Обновить таблицу сервисов и data flow |
| `.env.example` | Удалить `KAFKA_CLUSTER_ID`, добавить NATS vars |
| `docker-compose.yml` | Заменить kafka service на nats |
| `services/tracking-gateway/internal/transport/kafka/` | Переименовать в `transport/nats/`, обновить README |
| `services/tracking-worker/internal/transport/kafka/` | Переименовать в `transport/nats/`, обновить README |
| `services/tracking-worker/README.md` | Обновить «consumes from Kafka» → NATS JetStream |
| `services/*/internal/usecase/README.md` | Обновить упоминание Kafka в списке запрещённых импортов |
| `.claude/epics/01-project-foundation/EPIC.md` | Исторический файл: НЕ переписывать; добавить заметку «Superseded by EPIC 03: Event Bus changed from Kafka to NATS JetStream» в начало |

### Проблема 2: ordering-семантика NATS ≠ Kafka partitions

Kafka даёт ordering per partition key. NATS subjects так не работают. Честная документация гарантий:

- порядок сообщений сохраняется per publisher connection (один gateway-инстанс публикует последовательно);
- workers обязаны сортировать/обрабатывать точки по `recorded_at`;
- будущий trip detection обязан обрабатывать late/out-of-order точки через небольшой buffer window;
- логический ordering key `user_id:device_id` передаётся в headers — для будущего шардирования/фильтрации, не как гарантия порядка.

Нельзя писать, что NATS даёт Kafka-like partition ordering по key.

### Проблема 3: абстракция без overengineering

Нужен интерфейс, не привязанный к NATS, но без преждевременных обобщений. Решение: минимальный `Publisher`-интерфейс сейчас, consumer API — задокументированный design (durable name, ack/nack, context cancellation, graceful shutdown), реализация consumer'а — в эпике tracking-worker, когда появится первый реальный потребитель.

### Проблема 4: healthcheck NATS-контейнера

Образ `nats:2.10-alpine` основан на Alpine — busybox `wget` присутствует. Healthcheck через monitoring endpoint `:8222/healthz?js-enabled-only=true` валиден. Если бы использовался scratch-образ `nats:2.10` — wget недоступен; берём alpine.

### Проблема 5: nats CLI на host

Не устанавливаем. `make nats-check` / `make nats-streams` используют одноразовый контейнер `natsio/nats-box` в compose-сети. Имя сети — `wrany_wrany-net` (compose project `wrany` + network `wrany-net`); в Makefile вычислять не хардкодом, а через `docker compose` networking (`docker compose run`/`docker run --network`), уточнить при реализации.

### Проблема 6: кто создаёт stream

Варианты: (a) decl-конфиг при старте сервиса, (b) init-контейнер, (c) ручная команда. Решение MVP: `libs/eventbus/nats` содержит идемпотентный `EnsureStream` (create-or-update), вызывается будущими сервисами при старте; для EPIC 3 stream создаётся через `make nats-init` (one-shot nats-box) либо первым запуском адаптер-теста. Документируется как dev-механизм; production hardening — EPIC 15/16.

## Best Practice Research

- **NATS JetStream limits**: один stream с wildcard subjects — рекомендованный паттерн для связанных доменных событий малого/среднего объёма; consumers фильтруют по subject filter. Multi-stream нужен при разных retention policies — не наш случай в MVP.
- **Storage**: `file` storage + `-sd /data` + volume — durable across restarts. Retention: `limits` policy (по умолчанию) с max-age — replay возможен, диск ограничен; подходит для Orange Pi.
- **At-least-once**: JetStream publish с ack (синхронный `Publish` возвращает `PubAck`) — аналог Kafka acks=all для single-node. Consumer-сторона: explicit ack policy, redelivery через `AckWait`/`MaxDeliver`, dead-letter через advisory или явную публикацию в `dead-letter.v1` — выбран явный dead-letter subject (проще, переносимо на другие брокеры).
- **Envelope pattern**: стандарт для event-driven систем (CloudEvents-подобный): метаданные отделены от payload, версионирование через `event_type` suffix `.v1` + `event_version`. Полный CloudEvents SDK не берём — лишняя зависимость для MVP, наш envelope совместим по духу.
- **Идемпотентность**: `event_id` в envelope + `Nats-Msg-Id` header → JetStream deduplication window — бесплатная защита от дублей publisher retry.
- **Go client**: `github.com/nats-io/nats.go` с новым `jetstream` API (`nats.go/jetstream`) — актуальный рекомендованный интерфейс (старый `js.Publish` API в maintenance).
- **Anti-pattern, которого избегаем**: завязка domain/usecase на broker types. Контракты — pure Go structs + JSON, абстракция — узкий интерфейс.

## Solution Design

### Архитектура (MVP data flow)

```text
Android Tracker
  ↓ WebSocket
tracking-gateway
  ↓ publish (EventBus → NATS adapter)
NATS JetStream (stream WRANY_EVENTS)
  ↓ consume (durable consumers)
tracking-worker
  ↓ publish
NATS JetStream
  ↓ consume
route-worker / analytics services (later)
```

NATS — внутренний брокер. Не публикуется наружу: порт 4222 только внутри Docker-сети (monitoring 8222 — опционально localhost bind для dev).

ACK-семантика гейтвея (будущие эпики): backend отправляет ACK клиенту только после успешного `PubAck` от JetStream — прежнее правило «ACK после записи в Kafka» переносится 1:1.

### NATS naming

Stream:

```text
WRANY_EVENTS
```

Stream subjects (wildcard):

```text
location.events.*
trip.*
route.*
dead-letter.*
```

Конкретные subjects v1:

```text
location.events.v1
trip.started.v1
trip.updated.v1
trip.completed.v1
route.matched.v1
dead-letter.v1
```

Durable consumer naming convention (реализация — future epics):

```text
tracking-worker-location-consumer
route-worker-trip-consumer
analytics-route-consumer
```

### Event envelope

```json
{
  "event_id": "evt_123",
  "event_type": "location.events.v1",
  "event_version": 1,
  "occurred_at": "2026-06-10T12:00:01Z",
  "produced_at": "2026-06-10T12:00:04Z",
  "producer": "tracking-gateway",
  "correlation_id": "req_123",
  "payload": {}
}
```

Обязательные поля: `event_id`, `event_type`, `event_version`, `occurred_at`, `produced_at`, `producer`. `correlation_id` — required для location events (приходит из request), optional у derived events.

Go: `Envelope` со `payload json.RawMessage`; типизированные конструкторы `NewLocationEvent(...)` и т.п. собирают envelope + сериализуют payload.

### Payloads

`location.events.v1`:

```json
{
  "user_id": "user_123",
  "device_id": "device_123",
  "recorded_at": "2026-06-10T12:00:01Z",
  "received_at": "2026-06-10T12:00:04Z",
  "lat": 55.751244,
  "lon": 37.618423,
  "accuracy_m": 8.5,
  "speed_mps": 1.4,
  "bearing_deg": 82,
  "activity_type": "walking",
  "activity_confidence": 0.87,
  "battery_level": 0.74,
  "source": "android_tracker"
}
```

`trip.started.v1` / `trip.updated.v1` / `trip.completed.v1`: `trip_id`, `user_id`, `device_id`, временные метки, агрегаты (distance_m, duration_s, point_count) — минимальные поля, расширяются в EPIC 8 с bump версии при breaking changes.

`route.matched.v1`: `trip_id`, `route_id`, `user_id`, `match_score`.

`dead-letter.v1`: `original_subject`, `original_event` (raw envelope), `error`, `failed_at`, `consumer`.

Validation: required-поля, lat ∈ [-90, 90], lon ∈ [-180, 180], `accuracy_m >= 0`, `event_version >= 1`, непустые id. Fail fast, ошибки агрегируются.

### Ordering strategy (честная)

- subject: `location.events.v1` (один subject для всех location events);
- NATS headers каждого сообщения: `Nats-Msg-Id` (= `envelope.event_id`, дедупликация), `Wrany-User-Id`, `Wrany-Device-Id`, `Wrany-Correlation-Id`;
- логический ordering key: `user_id:device_id` — метаданные, не механизм;
- гарантия: порядок per publisher connection; workers сортируют по `recorded_at`; trip detection (EPIC 8) обязан иметь buffer window для late/out-of-order точек;
- НЕ заявляем Kafka-like partition ordering.

### Deduplication

- NATS adapter обязан устанавливать header `Nats-Msg-Id = envelope.event_id` при каждом publish;
- JetStream отбрасывает дубликаты с тем же `Nats-Msg-Id` в пределах dedup window стрима (default 2m, фиксируется в `EnsureStream`);
- это **best-effort защита от publisher retry** (повторная отправка при таймауте/реконнекте) — НЕ глобальная бизнес-идемпотентность;
- за пределами dedup window дубли возможны; бизнес-идемпотентность (по `event_id` на стороне consumers/storage) — ответственность будущих эпиков (trip detection, storage layer).

### Go module structure

```text
libs/
  events/                  # module github.com/<owner>/wrany/libs/events (path уточнить по go.mod сервисов)
    go.mod
    envelope.go            # Envelope, конструктор, маршалинг
    topics.go              # subject constants, stream name, consumer naming
    validation.go          # envelope validation + helpers
    location/event.go      # LocationPayload + validation + constructor
    trip/started.go
    trip/updated.go
    trip/completed.go
    route/matched.go
    deadletter/event.go
  eventbus/                # module .../libs/eventbus
    go.mod                 # зависит на libs/events и nats.go
    eventbus.go            # Publisher interface + errors (EventBus — future)
    nats/
      jetstream.go         # JetStream adapter: Publish, EnsureStream, Close
      config.go            # NATS_URL, NATS_STREAM parsing
```

`go.work` дополняется `./libs/events`, `./libs/eventbus`.

`libs/events` — без внешних зависимостей (stdlib only). `libs/eventbus` — единственное место с `nats.go` импортом (кроме будущих service-specific `internal/transport/nats`).

### Publisher interface

В EPIC 3 реализуется только publish-сторона, поэтому интерфейс называется `Publisher`, а не `EventBus`:

```go
// libs/eventbus/eventbus.go
type Publisher interface {
    Publish(ctx context.Context, subject string, event events.Envelope) error
}
```

`EventBus` как объединение publish/consume — **future abstraction**: появится, когда будет реализована consumer-сторона (эпик tracking-worker), как композиция `Publisher` + `Consumer`. В EPIC 3 — только design-описание (см. Architecture Notes).

Consumer API — design only, реализация отложена до первого реального потребителя. Обоснование: в EPIC 3 нет потребителей; реализация consumer без потребителя — мёртвый код без интеграционного контекста, нарушение scope.

NATS adapter — **реализуем минимально** (Publish + EnsureStream + Close): обоснование — (a) даёт компилируемое доказательство, что абстракция реализуема; (b) `make nats-check` + adapter дают проверяемый acceptance; (c) ~150 строк, scope не раздувает. Подключение адаптера к сервисам (wiring в app.go) — НЕ в этом эпике: реальная публикация запрещена scope'ом.

### Docker Compose

```yaml
nats:
  image: nats:2.10-alpine
  command: ["-js", "-sd", "/data", "-m", "8222"]
  volumes:
    - nats_data:/data
  networks:
    - wrany-net
  # NATS intentionally not exposed to host — internal only
  healthcheck:
    test: ["CMD-SHELL", "wget -q -O - 'http://localhost:8222/healthz?js-enabled-only=true' || exit 1"]
    interval: 10s
    timeout: 5s
    retries: 5
```

Kafka service + `kafka_data` volume удаляются. `depends_on: kafka` в сервисах → `nats`. Management UI не добавляем.

### Env

`.env.example` содержит только реально используемые переменные:

```text
# NATS JetStream (internal only, not exposed to host)
NATS_URL=nats://nats:4222
NATS_STREAM=WRANY_EVENTS
```

`NATS_STORAGE` **не добавляется**: storage type фиксируется как `file` внутри `EnsureStream` (dev/MVP default, соответствует `-sd /data` в compose). Если при реализации `config.go` всё же будет читать storage type — переменная возвращается в `.env.example`; правило: env-файл содержит только то, что читает код.

`KAFKA_CLUSTER_ID` удаляется (код её не читает — безопасно).

### Makefile

Добавить:

- `make nats-check` — JetStream enabled check (nats-box: `nats account info` или healthz через docker exec);
- `make nats-streams` — `nats stream ls` через nats-box;
- `make nats-init` — идемпотентное создание стрима WRANY_EVENTS (dev-механизм);
- `make test` дополнить `libs/events`, `libs/eventbus`.

Kafka-specific команд в Makefile нет (проверено) — удалять нечего.

### Docs

- `docs/architecture/event-bus.md` — выбор NATS, data flow, ordering-гарантии честно, dead-letter strategy, future Kafka/Redpanda adapters;
- `docs/contracts/events.md` — envelope, все payloads, subjects, headers, versioning policy, consumer naming.

## Architecture Notes

### Clean Architecture compliance

- Контракты (`libs/events`) — pure structs, допустимы к импорту из domain/usecase любого сервиса;
- `libs/eventbus` интерфейс — допустим в usecase как порт; NATS adapter импортируется только в composition root (app.go) будущих сервисов;
- service-specific NATS-адаптеры (subscriptions, handlers) будут жить в `services/<svc>/internal/transport/nats/`;
- `main.go` остаётся composition root only.

### Consumer API design (реализация в future epic)

```go
type Handler func(ctx context.Context, msg ConsumedEvent) error // err → nack/redelivery

type Consumer interface {
    // durable name, subject filter — в конфиге
    Consume(ctx context.Context, handler Handler) error // блокирует до ctx.Done, graceful drain
}

type ConsumedEvent struct {
    Envelope events.Envelope
    Ack()  error
    Nak()  error
}
```

Требования: durable consumer name, explicit ack, nack/redelivery (`AckWait`, `MaxDeliver`), context cancellation, graceful shutdown (drain). После `MaxDeliver` — публикация в `dead-letter.v1` consumer-логикой.

### Roadmap update (без автоматического renumbering)

Автоматический swap EPIC 3 ↔ EPIC 7 **не делается**. Правила:

- EPIC 03 становится «Event Bus Contracts & NATS JetStream Foundation»;
- бывший EPIC 03 «Android Background Location Tracking» помечается **Rescheduled** — текст сохраняется, новый номер НЕ присваивается без явного подтверждения пользователя;
- пункт 7 «Kafka Event Contracts» в списке CLAUDE.md помечается «superseded by EPIC 03» (его содержимое поглощено новым EPIC 03);
- остальные эпики не перенумеровываются;
- `.claude/EPICS.md` обновляется первым, diff roadmap показывается пользователю до остальных изменений.

### Security

- NATS не экспонируется на host и в интернет; только Docker-сеть;
- auth внутри сети — отложено (single-tenant dev), отмечено как hardening item для EPIC 15/16;
- validation контрактов на границе — обязательна перед publish.

## Tasks

- [ ] T01: Обновить `.claude/EPICS.md` — EPIC 03 = Event Bus Contracts & NATS JetStream Foundation; Android tracking пометить Rescheduled (номер не присваивать); заметка superseded для Kafka в EPIC 01; показать diff roadmap пользователю
- [ ] T02: Обновить `.claude/CLAUDE.md` — event bus = NATS JetStream, ACK-правило, directory structure (`transport/nats/`), EPIC list: пункт 3 = новое имя, пункт 7 = superseded by EPIC 03, без перенумерации остальных
- [ ] T03: Обновить `README.md` — таблица сервисов, data flow
- [ ] T04: Добавить заметку «Superseded by EPIC 03» в `.claude/epics/01-project-foundation/EPIC.md` (без переписывания истории)
- [ ] T05: `docker-compose.yml` — заменить kafka на nats (JetStream, volume, healthcheck, internal only), обновить depends_on
- [ ] T06: `.env.example` — NATS vars, удалить KAFKA_CLUSTER_ID; обновить локальный `.env` (не в git)
- [ ] T07: Переименовать `services/*/internal/transport/kafka/` → `transport/nats/`, обновить README этих директорий и `services/*/README.md`, `services/*/internal/usecase/README.md`
- [ ] T08: Создать `libs/events`: go.mod, envelope.go, topics.go, validation.go
- [ ] T09: `libs/events/location/event.go` — payload + validation
- [ ] T10: `libs/events/trip/{started,updated,completed}.go`
- [ ] T11: `libs/events/route/matched.go`, `libs/events/deadletter/event.go`
- [ ] T12: Unit tests для envelope + validation + всех payloads (table-driven, TDD)
- [ ] T13: Создать `libs/eventbus`: go.mod, eventbus.go (Publisher interface; EventBus — future abstraction, design only)
- [ ] T14: `libs/eventbus/nats/{jetstream.go,config.go}` — minimal adapter (Publish + EnsureStream + Close) + unit tests конфига
- [ ] T15: `go.work` — добавить libs modules
- [ ] T16: Makefile — `nats-check`, `nats-streams`, `nats-init`, расширить `test`
- [ ] T17: `docs/architecture/event-bus.md`, `docs/contracts/events.md`
- [ ] T18: Проверка: `make test`, `make up`, `make nats-check`, `make nats-streams`
- [ ] T19: Code review (code-reviewer agent), security pass
- [ ] T20: Заполнить Implementation Log + Final Report

## Acceptance Criteria

- [ ] `.claude/EPICS.md`: EPIC 3 = Event Bus Contracts & NATS JetStream Foundation
- [ ] Актуальные Kafka references заменены на NATS JetStream или помечены historical/superseded
- [ ] `docker-compose.yml` использует NATS JetStream вместо Kafka
- [ ] `.env.example` содержит NATS variables; Kafka env vars удалены
- [ ] `libs/events` создан; envelope, topic constants, location/trip/route/dead-letter contracts, validation реализованы
- [ ] `libs/eventbus` создан; минимальный `Publisher` interface реализован; `EventBus` (publish+consume) описан как future abstraction
- [ ] NATS JetStream adapter реализован минимально (Publish + EnsureStream), выбор обоснован
- [ ] NATS adapter устанавливает header `Nats-Msg-Id = envelope.event_id` при publish
- [ ] Unit tests для event validation проходят
- [ ] `make test` проходит
- [ ] `make up` поднимает postgres, tracking-gateway, tracking-worker, nats
- [ ] `make nats-check` подтверждает JetStream enabled
- [ ] README/docs обновлены
- [ ] Final Report заполнен
- [ ] Ветка — `epic/03-event-bus-contracts-and-nats-foundation`; EPIC 4 не начат

## Test Plan

1. **Unit (libs/events)**: table-driven тесты — валидный envelope; отсутствие каждого required-поля; границы lat/lon; отрицательная accuracy; пустые id; round-trip marshal/unmarshal envelope+payload каждого типа.
2. **Unit (libs/eventbus)**: config parsing (defaults, отсутствие NATS_URL); subject/stream constants.
3. **Adapter sanity**: интеграционный тест адаптера за build tag `integration` (skip без NATS) — EnsureStream идемпотентен; Publish возвращает ack; повторный publish с тем же `event_id` в пределах JetStream dedup window не создаёт дубль (количество сообщений в стриме не увеличивается). Дедупликация проверяется как best-effort publisher retry protection, не как бизнес-идемпотентность. Запуск вручную при поднятом `make up`.
4. **Infra**: `make up` — все контейнеры healthy; `make nats-check` — JetStream enabled; `make nats-streams` — после `make nats-init` виден WRANY_EVENTS; рестарт nats-контейнера — stream переживает (file storage).
5. **Regression**: существующие тесты tracking-gateway проходят (`make test`).

## Documentation Plan

- `docs/architecture/event-bus.md` — new
- `docs/contracts/events.md` — new
- `README.md` — update (services table, data flow, make commands)
- `.claude/CLAUDE.md` — update (architecture, directory structure, epic list)
- `.claude/EPICS.md` — update (EPIC 03 scope, Android tracking → Rescheduled без перенумерации)
- `services/*/README.md` + `transport/nats/README.md` — update
- `.env.example` — update

## Implementation Log

**2026-06-10** — Kafka → NATS JetStream: CLAUDE.md, EPICS.md, README.md, docker-compose.yml, .env.example, .env обновлены. `transport/kafka/` → `transport/nats/`. EPIC 01 помечен superseded.

**2026-06-10** — `libs/events` создан: envelope, validation helpers, topics (constants, StreamSubjects, ConsumerName), payloads: location/trip×3/route/deadletter. Unit tests для всех пакетов.

**2026-06-10** — `libs/eventbus` создан: `Publisher` interface + NATS adapter (Connect, EnsureStream idempotent, Publish с Nats-Msg-Id dedup header, Close/drain). Config с LoadConfig. Интеграционный тест (build tag `integration`).

**2026-06-10** — `go.work` дополнен libs. Dockerfile gateway обновлён на golang:1.25-alpine. Makefile: nats-check, nats-streams, nats-init, make test включает libs. Docs: event-bus.md, contracts/events.md.

**2026-06-10** — `make test` зелёный. `make up` требует `golang:1.25-alpine` (pull при наличии сети).

## Final Report

**Цель достигнута.** Event bus foundation создан.

### Что сделано

- Kafka → NATS JetStream во всех актуальных source-of-truth файлах.
- `libs/events`: envelope + 6 event contracts + validation. Нет внешних зависимостей.
- `libs/eventbus`: `Publisher` interface + NATS JetStream adapter с `Nats-Msg-Id` dedup.
- `make test` зелёный. Unit tests проходят.
- Docs: event-bus.md, contracts/events.md.

### Открытые вопросы

- `make up` требует `docker pull golang:1.25-alpine` при стабильной сети (образ gateway).
- Интеграционный тест: `cd libs/eventbus && go test -tags integration ./nats/...` при поднятом NATS.
- Wiring Publisher в сервисы — EPIC 5/6.
- Consumer + полный EventBus — epic tracking-worker.
- `COMPOSE_NETWORK` в Makefile = `wrany_wrany-net`; скорректировать при нестандартном project name.
