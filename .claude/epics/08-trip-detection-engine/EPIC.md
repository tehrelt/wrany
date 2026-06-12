# EPIC 08: Trip Detection Engine

## Status

Done

---

## Goal

Реализовать backend Trip Detection Engine, который на основе сырых GPS-точек автоматически определяет начало движения, активный trip, остановку, завершение trip и набор точек, относящихся к trip.

---

## Context

К этому моменту pipeline выглядит так:

```
Android Tracker
  ↓ WebSocket
tracking-gateway
  ↓ NATS location.events.v1
tracking-worker
  ↓
Postgres/PostGIS raw_location_points
  ↓
web client raw points viewer
```

Система умеет принимать и сохранять сырые GPS-точки, но не умеет автоматически выделять законченные перемещения. EPIC 08 превращает `raw_location_points` → `trips`.

Зависимости: EPIC 01, 02, 03, 04, 05, 06, 07 — все завершены и смержены в main.

---

## Problem Analysis

### Ключевые вызовы

1. **Шум GPS**: плохая точность, прыжки координат, задублированные точки.
2. **Порядок точек**: late/out-of-order события; нельзя полагаться на порядок NATS — сортировать по `recorded_at`.
3. **Ложные старты**: кратковременное движение (пешеход вышел и вернулся) не должно создавать trip.
4. **Ложные стопы**: остановка на светофоре — не конец trip.
5. **Идемпотентность**: повторный запуск worker не должен дублировать trips.
6. **Checkpoint**: worker должен помнить, до какой точки он обработал данные.
7. **Связь точек с trip**: нужна отдельная таблица `trip_points`, чтобы не мутировать raw ingestion table.

### Базовые пороги (MVP)

| Параметр | Значение |
|---|---|
| `motion_min_duration` | 45–60 сек |
| `motion_min_distance` | 60–100 м |
| `max_accuracy_m` | configurable (default 50 м) |
| `stop_min_duration` | 3–5 мин |
| `stop_radius_m` | 40 м |
| `min_speed_kmh` | ~2 км/ч для движения |

---

## Best Practice Research

### State Machine подход

Стандартный подход для trip detection в tracking-системах:

```
IDLE → MOTION_CANDIDATE → TRIP_ACTIVE → STOP_CANDIDATE → TRIP_COMPLETED
                ↓ (шум)
              IDLE
```

Используется в Google Maps Timeline, Strava, Moves (RIP), OsmAnd trip recording.

### Обработка GPS шума

- **Accuracy filter**: отбрасывать точки с `accuracy_m > threshold`.
- **Speed cap**: физически невозможная скорость (>300 км/ч) = GPS прыжок, игнорировать.
- **Duplicate filter**: `(device_id, recorded_at)` уже видели — пропустить.
- **Haversine distance**: расстояние между точками для фильтрации прыжков.

### Checkpoint / idempotency — watermark-based

- Хранить **watermark** per `(user_id, device_id)`: `last_watermark_at`.
- Detection обрабатывает точки: `recorded_at >= last_watermark_at AND recorded_at < now() - late_arrival_window_sec`.
- После успешного run: `last_watermark_at = now() - late_arrival_window_sec`.
- `late_arrival_window_sec` (default 300) — окно, в котором late/out-of-order points ещё могут прийти.
- Checkpoint **не двигается** вперёд выше `now() - window`, чтобы не потерять поздно пришедшие точки.
- Обработка уже-обработанных точек безопасна: `UNIQUE (user_id, device_id, event_id)` в `trip_points` предотвращает дублирование.

### State machine persistence

- Состояние `IDLE / MOTION_CANDIDATE / TRIP_ACTIVE / STOP_CANDIDATE` нельзя хранить только в памяти.
- Worker restart сбросит in-memory state → потеряет активный candidate/trip.
- Хранить state per `(user_id, device_id)` в таблице `trip_detection_state`.
- При старте job: загрузить state из БД, продолжить с того места.

### Processing model

Periodic worker (ticker) внутри `tracking-worker` — проще чем event-driven для MVP. Запускается каждые N секунд, обрабатывает новые точки с checkpoint.

### trip_points vs trip_id в raw_location_points

Отдельная таблица `trip_points` предпочтительна:
- Не мутирует raw ingestion table
- Позволяет переобработать trip без изменения raw data
- Поддерживает будущие re-detection сценарии

---

## Solution Design

### State Machine

```go
type TripState string

const (
    StateIdle            TripState = "IDLE"
    StateMotionCandidate TripState = "MOTION_CANDIDATE"
    StateTripActive      TripState = "TRIP_ACTIVE"
    StateStopCandidate   TripState = "STOP_CANDIDATE"
    StateTripCompleted   TripState = "TRIP_COMPLETED"
)
```

Переходы:

```
IDLE
  + point, движение → MOTION_CANDIDATE (запомнить candidate_start)
  + point, стоп/шум → IDLE

MOTION_CANDIDATE
  + duration >= 45s AND distance >= 60m → TRIP_ACTIVE (создать trip)
  + duration < 45s AND стоп → IDLE (сброс)
  + шум/bad accuracy → IDLE (сброс)

TRIP_ACTIVE
  + точки с движением → обновить trip (distance, duration, points_count)
  + остановка началась → STOP_CANDIDATE (запомнить stop_start)

STOP_CANDIDATE
  + остановка продолжается >= 3m AND radius <= 40m → TRIP_COMPLETED (закрыть trip)
  + движение возобновилось → TRIP_ACTIVE (сброс stop)

TRIP_COMPLETED → IDLE (начинать поиск нового trip)
```

### Data Flow

```
tracking-worker periodic job (каждые 30 сек)
  → читает raw_location_points с checkpoint
  → ORDER BY user_id, device_id, recorded_at
  → фильтрует шум (accuracy, duplicates, jumps)
  → прогоняет через state machine per (user_id, device_id)
  → пишет trips / trip_points / обновляет checkpoint
```

### NATS events

Публиковать при изменениях (контракты уже есть в `libs/events/trip`):
- `trip.started.v1` — при переходе MOTION_CANDIDATE → TRIP_ACTIVE
- `trip.updated.v1` — при обновлении активного trip
- `trip.completed.v1` — при TRIP_COMPLETED

---

## Architecture Notes

### tracking-worker (новые файлы)

```
services/tracking-worker/
  internal/
    domain/
      trip.go                    # Trip, TripPoint, TripState, TripDetectionConfig
    usecase/
      trip_detection.go          # TripDetectionUseCase, state machine logic
    storage/postgres/
      trip_repo.go               # TripRepository (create, update, addPoints, checkpoint)
    app/app.go                   # добавить периодический job
```

### tracking-gateway (новые файлы)

```
services/tracking-gateway/
  internal/
    domain/
      trip_query.go              # TripQuery domain types
    usecase/
      trip_query.go              # TripQueryUseCase (list, get, points)
    storage/postgres/
      trip_query_repo.go         # read-only queries
    transport/http/
      trip_handler.go            # GET /v1/trips, GET /v1/trips/{id}, GET /v1/trips/{id}/points
```

### Новые таблицы (миграции)

**trips** — в tracking-worker migrations (0002):
```sql
trips(id, user_id, device_id, status, started_at, ended_at,
      start_lat, start_lon, end_lat, end_lon,
      distance_m, duration_sec, points_count, created_at, updated_at)
```

**trip_points** — в tracking-worker migrations (0003):
```sql
trip_points(trip_id, user_id, device_id, event_id, recorded_at)
PK: (trip_id, event_id)
UNIQUE: (user_id, device_id, event_id)  -- одна raw point не может войти в два trip
```

**trip_detection_state** — в tracking-worker migrations (0004), объединяет checkpoint + state:
```sql
trip_detection_state(
  -- PK
  user_id, device_id,

  -- state machine
  state,                       -- IDLE | MOTION_CANDIDATE | TRIP_ACTIVE | STOP_CANDIDATE
  active_trip_id,              -- nullable, FK → trips.id
  candidate_started_at,        -- nullable, когда вошли в MOTION_CANDIDATE
  candidate_start_point_id,    -- nullable, event_id первой candidate точки
  stop_started_at,             -- nullable, когда вошли в STOP_CANDIDATE
  stop_center_lat,             -- nullable, центр зоны остановки
  stop_center_lon,             -- nullable

  -- последняя обработанная точка (для state continuity on restart)
  last_processed_recorded_at,  -- nullable

  -- watermark-based checkpoint
  last_watermark_at,           -- nullable, до этого момента всё обработано
  late_arrival_window_sec,     -- int, default 300

  updated_at
)
```

**Watermark rule**: обрабатывать `recorded_at >= last_watermark_at AND recorded_at < now() - late_arrival_window_sec`. После run: `last_watermark_at = now() - late_arrival_window_sec`.

### Dependency direction

```
trip_handler → TripQueryUseCase → TripQueryRepo (postgres)
TripDetectionJob → TripDetectionUseCase → TripRepo (postgres) + Publisher (NATS)
TripDetectionUseCase → domain.Trip (no external deps)
```

---

## Tasks

### Phase 1: Database

- [ ] Миграция 0002: таблица `trips`
- [ ] Миграция 0003: таблица `trip_points` (UNIQUE на event per user/device)
- [ ] Миграция 0004: таблица `trip_detection_state` (checkpoint + state machine)
- [ ] Down миграции с WARNING комментариями
- [ ] `make test` проходит после миграций

### Phase 2: tracking-worker — Domain & State Machine

- [ ] `domain/trip.go` — Trip, TripPoint, TripState, TripDetectionConfig, TripDetectionState
- [ ] `usecase/trip_detection.go` — state machine, noise filtering, transitions
- [ ] Unit tests для всех state machine переходов

### Phase 3: tracking-worker — Storage & Job

- [ ] `storage/postgres/trip_repo.go` — TripRepository
- [ ] Integration tests для TripRepository
- [ ] Periodic job в `app/app.go`
- [ ] NATS publish при trip events

### Phase 4: tracking-gateway — Query API

- [ ] `domain/trip_query.go`
- [ ] `usecase/trip_query.go`
- [ ] `storage/postgres/trip_query_repo.go`
- [ ] `transport/http/trip_handler.go` — GET /v1/trips, /v1/trips/{id}, /v1/trips/{id}/points
- [ ] swaggo annotations
- [ ] make openapi-generate + make ts-client

### Phase 5: Web Client

- [ ] Trips page/tab в web app
- [ ] Список trips с фильтрами (device, date range)
- [ ] Trip cards (duration, distance, points_count, start/end time)
- [ ] Trip polyline на карте (react-leaflet)

### Phase 6: Finalize

- [ ] Обновить EPIC.md Implementation Log
- [ ] Обновить Final Report
- [ ] `make test` проходит
- [ ] Manual E2E flow

---

## Acceptance Criteria

- [ ] Таблицы `trips`, `trip_points`, `trip_detection_checkpoints` созданы
- [ ] Raw points превращаются в trips
- [ ] Короткий шум (< 45 сек, < 60 м) не создаёт trip
- [ ] Устойчивое движение (>= 45 сек, >= 60 м) создаёт trip
- [ ] Длительная остановка (>= 3 мин в радиусе 40 м) завершает trip
- [ ] Late/out-of-order points обрабатываются безопасно
- [ ] Повторный запуск worker не создаёт duplicate trips
- [ ] REST API возвращает trips только текущего user
- [ ] OpenAPI обновлён
- [ ] Generated TypeScript client обновлён
- [ ] Web client показывает trips на карте
- [ ] `make test` проходит
- [ ] Manual flow: Android sends points → worker detects trip → web displays trip
- [ ] EPIC 09 не начат

---

## Test Plan

### Unit Tests (tracking-worker/internal/usecase)

- IDLE → MOTION_CANDIDATE при движении
- MOTION_CANDIDATE → IDLE при шуме (bad accuracy)
- MOTION_CANDIDATE → IDLE при остановке до threshold
- MOTION_CANDIDATE → TRIP_ACTIVE при устойчивом движении
- TRIP_ACTIVE → STOP_CANDIDATE при остановке
- STOP_CANDIDATE → TRIP_ACTIVE при возобновлении движения
- STOP_CANDIDATE → TRIP_COMPLETED при долгой остановке
- GPS jump игнорируется (скорость > 300 км/ч)
- Дублированная точка игнорируется
- Точка с плохой accuracy игнорируется
- Late point (recorded_at < последней обработанной) — безопасна
- Idempotent reprocessing

### Integration Tests

- Seed raw_location_points → run detection → verify trips created
- Run detection again → verify no duplicates
- Query GET /v1/trips → verify response
- Query GET /v1/trips/{id}/points → verify points linked

### Manual E2E

1. Запустить `make up`
2. Зарегистрироваться в web app
3. Отправить серию GPS точек с Android (или тестовым скриптом)
4. Дождаться worker цикла
5. Открыть web app → Trips → увидеть trip на карте

---

## Documentation Plan

- Обновить `services/tracking-worker/README.md` — секция Trip Detection
- Обновить `services/tracking-gateway/README.md` — секция Trips API
- Обновить `apps/web/README.md` — секция Trips UI
- Обновить OpenAPI spec (автоматически через swaggo)
- Регенерировать TypeScript client

---

## Implementation Log

### Phase 1: Database migrations (2026-06-11)

**Уточнения схемы (по ревью пользователя):**
- Checkpoint watermark-based: `last_watermark_at`, `late_arrival_window_sec` (default 300).
  Правило: обрабатывать `recorded_at >= last_watermark_at AND recorded_at < now() - late_arrival_window_sec`.
  После run: `last_watermark_at = now() - late_arrival_window_sec`.
- State machine state персистируется в БД (не in-memory) — иначе рестарт ломает detection.
- `trip_points` — отдельная таблица с `UNIQUE (user_id, device_id, event_id)` как idempotency guard.
- `trip_detection_state` объединяет checkpoint + state machine state per `(user_id, device_id)`.

**Созданы миграции:**
- `services/tracking-worker/infra/migrations/0002_create_trips.up.sql` — таблица `trips`, индексы по `(user_id, device_id, started_at)` и `status`.
- `services/tracking-worker/infra/migrations/0002_create_trips.down.sql` — DROP CASCADE с WARNING.
- `services/tracking-worker/infra/migrations/0003_create_trip_points.up.sql` — таблица `trip_points`, PK `(trip_id, event_id)`, UNIQUE `(user_id, device_id, event_id)`.
- `services/tracking-worker/infra/migrations/0003_create_trip_points.down.sql`.
- `services/tracking-worker/infra/migrations/0004_create_trip_detection_state.up.sql` — watermark + state machine state, включая stop zone fields.
- `services/tracking-worker/infra/migrations/0004_create_trip_detection_state.down.sql` — с WARNING о потере checkpoint и риске дублей trips.

**Механизм применения:**
- `make migrate-worker-up` — ручное применение (production/dev).
- Тесты: `testcontainers` + `migrations.RunWithTable` применяет все файлы автоматически.
- Существующие тесты не затронуты — новые таблицы добавляются поверх `raw_location_points`.

### Phase 2: Domain types + state machine usecase + unit tests (2026-06-11)

**Созданы файлы:**
- `internal/domain/trip.go` — TripStatus, TripState, Trip, TripPoint, TripDetectionState, TripDetectionConfig + DefaultTripDetectionConfig(), batch types (UserDevicePair, TripStatsDelta, TripCompletion, TripDetectionBatch).
- `internal/usecase/trip_detection.go` — TripDetectionUseCase.ProcessBatch: чистая state machine без I/O. CommandKind (CmdCreateTrip/CmdUpdateTrip/CmdCompleteTrip), TripCommand, ProcessBatchResult.
- `internal/usecase/trip_detection_test.go` — 16 unit-тестов, все PASS.

**Ключевые решения:**
- Sensor speed authoritative: `if pt.SpeedMps != nil { effectiveSpeed = *pt.SpeedMps }`.
- GPS jump → skip без обновления LastPointLat/Lon.
- flushActive() closure перед любым переходом из TRIP_ACTIVE.
- Watermark advance: `LastWatermarkAt = now - LateArrivalWindowSec`.
- Batch types перенесены в domain (не usecase) — чтобы storage мог импортировать без циклических зависимостей.
- Добавлена migration 0005 для cross-batch полей (CandidateDistanceM, CandidateStartLat/Lon, LastPointLat/Lon).

---

### Phase 3: Storage + job + app wiring (2026-06-11)

**Созданы файлы:**
- `internal/storage/postgres/trip_repo.go` — TripRepo implements TripDetectionRepository:
  LoadDistinctUserDevicePairs (LEFT JOIN watermark), LoadState (default IDLE), FetchPoints ([from, to) ASC), ApplyBatch (single tx: insertTrip → backfillTripPoints → insertTripPoints → updateTripStats → completeTrip → upsertDetectionState).
- `internal/usecase/trip_detection_job.go` — TripDetectionJob.RunOnce: watermark boundary, processPair (load state → fetch → ProcessBatch → buildBatch → ApplyBatch → publishEvents), события trip.started/updated/completed v1.
- `internal/storage/postgres/trip_repo_test.go` — integration tests: LoadDistinctUserDevicePairs, LoadState (default + persisted), FetchPoints (window + order), ApplyBatch (insert/update/complete trip, idempotent trip_points, upsert state).

**Обновлены файлы:**
- `internal/config/config.go` — TripDetectionIntervalSec (TRIP_DETECTION_INTERVAL_SEC, default 30).
- `internal/app/app.go` — wire TripRepo + TripDetectionJob, goroutine с ticker в Run(), импорт domain.

**Коммиты:**
- `dd71a34` epic(08): add trip detection storage, job, and app wiring (Phase 3)

### Phase 4: tracking-gateway query API (2026-06-12)

**Созданы файлы (tracking-gateway):**
- `internal/domain/trip.go` — Trip, TripPoint, TripFilter, TripPointFilter read-only types.
- `internal/usecase/trip_query.go` — TripQueryUsecase: ListTrips (keyset пагинация DESC started_at), GetTrip (ownership check), GetTripPoints (keyset ASC recorded_at + JOIN raw_location_points).
- `internal/storage/postgres/trip_query_repo.go` — запросы trips + trip_points, user_id isolation.
- `internal/transport/http/trip_handler.go` — GET /v1/trips, /v1/trips/{id}, /v1/trips/{id}/points; swaggo annotations.
- `internal/storage/postgres/trip_query_repo_test.go` — 8 integration-тестов: user isolation, status filter, пагинация, not-found, wrong user, GetTripPoints с JOIN.

**Обновлены:**
- `swagger_types.go` — TripItem, TripListResponse, TripPointItem, TripPointsResponse + envelopes.
- `router.go` — 3 новых authenticated роута + поле Trips в RouterDeps.
- `app/app.go` — wire TripQueryRepo + TripQueryUsecase.

---

### Phase 5: Web client Trips page (2026-06-12)

**Созданы файлы:**
- `apps/web/src/features/trips/tripsApi.ts` — listTrips, getTripPoints, formatDuration, formatDistance.
- `apps/web/src/components/map/TripMap.tsx` — карта с полилинией (зелёная линия, старт/финиш dots), react-map-gl/maplibre.
- `apps/web/src/pages/TripsPage.tsx` — список trips в sidebar с фильтром по статусу + TripMap + stats footer.

**Обновлены:**
- `app/App.tsx` — роут `/trips` за AuthGuard.
- `AppLayout.tsx` — навигационные вкладки Points / Trips с active state (react-router-dom Link).

---

## Final Report

### Итог EPIC 08: Trip Detection Engine

**Статус:** Done

**Что реализовано:**

| Компонент | Статус |
|-----------|--------|
| DB миграции (trips, trip_points, trip_detection_state) | ✅ |
| Watermark-based checkpoint | ✅ |
| State machine (IDLE→MOTION_CANDIDATE→TRIP_ACTIVE→STOP_CANDIDATE) | ✅ |
| Persisted state (restart-safe) | ✅ |
| tracking-worker: TripDetectionJob (periodic, 30s interval) | ✅ |
| tracking-worker: NATS events (trip.started/updated/completed v1) | ✅ |
| tracking-worker: integration tests (TripRepo) | ✅ |
| tracking-worker: unit tests (state machine, 16 тестов) | ✅ |
| tracking-gateway: REST API (GET /v1/trips, /id, /id/points) | ✅ |
| tracking-gateway: integration tests (TripQueryRepo) | ✅ |
| Web client: Trips page + TripMap polyline | ✅ |
| Web client: nav tabs Points / Trips | ✅ |

**Тесты:** `go test ./internal/usecase/...` PASS на обоих сервисах. TypeScript build чистый.

**Out of scope (следующие epics):** Loop route detection (EPIC 09), route matching (EPIC 10), best lap (EPIC 11).
