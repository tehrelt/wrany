# EPIC 09: Route Matching & Repeated Routes

## Status

Implemented

---

## Goal

Реализовать backend Route Matching Engine: trip → route.

Система должна:
1. брать completed trips без привязки к маршруту;
2. сравнивать новый trip с уже известными route templates;
3. если найден похожий route — привязать trip к нему;
4. если похожего route нет — создать новый route;
5. сохранять связь route ↔ trips;
6. отдавать routes через REST API;
7. показывать routes в web client.

---

## Context

### Текущее состояние pipeline

```
Android Tracker
  ↓ WebSocket
tracking-gateway
  ↓ NATS location.events.v1
tracking-worker
  ↓ raw_location_points
  ↓ Trip Detection Engine (EPIC 08)
trips + trip_points
  ↓ web client Trips page
```

EPIC 08 научил систему превращать сырые GPS-точки в `trips`.
Каждый `trip` существует отдельно. EPIC 09 добавляет слой группировки:
несколько trips → один route.

### Пример

```
Route: "Дом → Магазин"
  Trip 1: 2026-06-10, 13:20, 950m, 12m34s
  Trip 2: 2026-06-12, 18:10, 970m, 13m05s
  Trip 3: 2026-06-15, 09:00, 940m, 11m58s
```

### Что уже есть

- `libs/events/route/matched.go` — контракт `route.matched.v1` готов
- `services/tracking-worker/internal/usecase/trip_detection_job.go` — паттерн periodic job
- migrations 0001–0005 в `services/tracking-worker/infra/migrations/`
- `services/tracking-gateway/internal/transport/http/trip_handler.go` — паттерн handler

---

## Problem Analysis

### Проблема

После EPIC 08 trips существуют изолированно. Нет способа узнать:
- Сколько раз пользователь прошёл этот маршрут?
- Это новый маршрут или уже знакомый?
- Как менялось время/дистанция от попытки к попытке?

### Ключевые вопросы

1. **Как определить "похожесть" двух routes?**
   → По близости start/end точек + совпадению расстояния + схожести пути

2. **Когда создавать новый route?**
   → Когда ни один существующий route не подходит по threshold'ам

3. **Directionality?**
   → Direction-sensitive: A→B ≠ B→A (bidirectional — future epic)

4. **Template update?**
   → MVP: template = polyline первого trip. Averaging — future improvement.

5. **Idempotency?**
   → Job должна быть идемпотентной: повторный запуск не создаёт дубликатов

---

## Best Practice Research

### MVP алгоритм (без ML и heavy map matching)

Trip считается похожим на route, если выполнены все условия:

```
1. start points close:     ST_Distance(trip.start, route.start) < start_radius_m
2. end points close:       ST_Distance(trip.end, route.end) < end_radius_m
3. distance similar:       |trip.distance - route.distance| / route.distance < tolerance_ratio
4. path similar:           avg_point_distance(normalize(trip), normalize(route)) < threshold_m
```

Нормализация polyline до N=50 точек (линейная интерполяция).
Average point-to-point distance как метрика схожести пути.

### Thresholds (configurable)

```
START_RADIUS_M         = 75
END_RADIUS_M           = 75
DISTANCE_TOLERANCE_RATIO = 0.25
PATH_SIMILARITY_THRESHOLD_M = 50   (avg distance в метрах)
MIN_TRIP_POINTS        = 5
NORMALIZE_POINTS_N     = 50
```

### Аналоги в production системах

- Strava route detection использует Fréchet distance / DTW
- Google Maps uses map-matched polylines + spatial clustering
- MVP подход (start/end + distance + normalized path) достаточен для городского трекинга

---

## Solution Design

### Компоненты

#### tracking-worker (matching engine)

```
domain/route.go               Route, RouteTrip, RouteMatchConfig, RouteMatchResult
usecase/route_matching.go     RouteMatchingUseCase — pure matching logic
usecase/route_matching_job.go RouteMatchingJob — periodic runner (pattern from TripDetectionJob)
storage/postgres/route_repo.go RouteRepository — DB operations
```

#### tracking-gateway (read API)

```
domain/route_query.go                  Route, RouteTrip, RouteFilter read models
usecase/route_query.go                 RouteQueryUseCase
storage/postgres/route_query_repo.go   RouteQueryRepository
transport/http/route_handler.go        GET /v1/routes endpoints
```

#### web client

```
apps/web/src/pages/RoutesPage.tsx
apps/web/src/features/routes/          routesApi, RouteMap, types
```

### Processing flow

```
RouteMatchingJob.RunOnce(ctx):
  1. Load completed trips WITHOUT route_trips (unmatched)
  2. For each unmatched trip:
     a. Load candidate routes: same user_id, ST_Distance(start) < start_radius_m
     b. Filter by end distance
     c. Filter by distance tolerance
     d. Load trip_points + route template_geom
     e. Normalize both polylines to 50 points
     f. Compute avg point-to-point distance (Haversine)
     g. Pick best candidate (lowest avg_distance if < threshold)
     h. If match found:
        - INSERT route_trips ON CONFLICT DO NOTHING
        - UPDATE routes SET trips_count, last_trip_id, updated_at
     i. If no match:
        - INSERT route (template_geom = trip polyline)
        - INSERT route_trips
     j. Publish route.matched.v1
```

### Directionality

A→B и B→A — разные routes (direction-sensitive).
Bidirectional merging — out of scope, future epic.

---

## Architecture Notes

### Data model

#### routes
```sql
id UUID PK
user_id UUID NOT NULL
device_id UUID                        -- nullable (multi-device support)
name TEXT                             -- nullable (future: user naming)
status TEXT NOT NULL DEFAULT 'active'
start_lat, start_lon DOUBLE PRECISION
end_lat, end_lon DOUBLE PRECISION
distance_m DOUBLE PRECISION
trips_count INTEGER DEFAULT 0
template_geom geometry(LineString, 4326)
start_geom geometry(Point, 4326) GENERATED  -- for spatial index
end_geom geometry(Point, 4326) GENERATED    -- for spatial index
first_trip_id UUID REFERENCES trips(id)
last_trip_id UUID REFERENCES trips(id)
created_at, updated_at TIMESTAMPTZ
```

Spatial indexes: GIST на start_geom, end_geom, template_geom.
Generated columns безопаснее expression index (нет риска неверного типа).

#### route_trips
```sql
(route_id, trip_id) PK
trip_id UNIQUE       -- один trip принадлежит ровно одному route
user_id, device_id   UUID
match_score DOUBLE   -- 0..1, 1.0 для первого trip (создавшего route)
matched_at TIMESTAMPTZ
duration_sec BIGINT
distance_m DOUBLE
```

### Dependency direction

```
tracking-worker:
  transport/nats → usecase/route_matching_job → usecase/route_matching → domain/route
  storage/postgres/route_repo → domain/route

tracking-gateway:
  transport/http/route_handler → usecase/route_query → domain/route_query
  storage/postgres/route_query_repo → domain/route_query
```

### Makefile targets (actual names in this project)

- `make swagger-gen` (не openapi-generate)
- `make ts-client`

---

## Tasks

### Phase 1: Data model
- [x] Migration 0006: CREATE TABLE routes
- [x] Migration 0007: CREATE TABLE route_trips
- [ ] Verify migrations apply cleanly (pending `make up`)

### Phase 2: tracking-worker
- [x] domain/route.go — types
- [x] usecase/route_matching.go — matching algorithm
- [x] usecase/route_matching_job.go — periodic runner
- [x] usecase/route_matching_test.go — unit tests (10 pass)
- [x] storage/postgres/route_repo.go — repository
- [ ] storage/postgres/route_repo_test.go — integration tests (pending)
- [x] config/config.go — add RouteMatchingIntervalSec
- [x] app/app.go — wire RouteMatchingJob

### Phase 3: tracking-gateway
- [x] domain/route_query.go
- [x] usecase/route_query.go
- [x] storage/postgres/route_query_repo.go
- [x] transport/http/route_handler.go
- [x] transport/http/router.go — register /v1/routes

### Phase 4: OpenAPI + codegen
- [x] swaggo annotations on route_handler.go
- [x] make swagger-gen
- [x] make ts-client (orval, local config)

### Phase 5: Web client
- [x] features/routes/routesApi.ts
- [x] pages/RoutesPage.tsx
- [x] AppLayout — add Routes nav link

### Phase 6: Tests & verification
- [x] make test — all pass (exit 0)
- [ ] Manual E2E flow (pending live stack)

---

## Acceptance Criteria

- `routes` table exists with spatial indexes
- `route_trips` table exists, UNIQUE (trip_id) enforced
- Completed trip без route → matched или создаёт новый route
- Похожие trips привязываются к одному route
- Trips с разными start/end → разные routes
- Обратный маршрут создаёт отдельный route
- Повторный запуск job не дублирует route_trips
- `route.matched.v1` публикуется
- API возвращает только routes текущего пользователя
- OpenAPI регенерирован
- TypeScript client регенерирован
- Web Routes page отображает routes
- Выбранный route показывает template polyline на карте
- Выбранный route показывает список trips
- Best lap / personal records НЕ реализованы
- `make test` проходит
- Manual E2E: Android → repeated trips → worker groups them → web shows route

---

## Test Plan

### Unit tests (route_matching_test.go)

- `TestMatchSameRoute` — одинаковые start/end/distance/path → match
- `TestNoMatchDifferentStart` — далёкие start points → no match
- `TestNoMatchDifferentEnd` — далёкие end points → no match
- `TestNoMatchDistanceTooLarge` — distance ratio > threshold → no match
- `TestNoMatchPathTooLarge` — avg path distance > threshold → no match
- `TestNoMatchReverseDirection` — B→A не совпадает с A→B
- `TestFirstTripCreatesRoute` — первый trip создаёт route
- `TestDuplicateTripIdempotent` — повторный вызов не дублирует
- `TestMatchScoreDeterministic` — одинаковый input → одинаковый score

### Integration tests (route_repo_test.go)

- Seed completed trip + trip_points → run job → route created
- Seed similar second trip → run job → same route (trips_count=2)
- Run job again → no duplicates (idempotency)
- Different user → separate route (user isolation)
- `GET /v1/routes` → only current user's routes

### Manual E2E

1. Seed / send first trip через WebSocket
2. TripDetectionJob создаёт trip
3. RouteMatchingJob создаёт route
4. Seed / send похожий второй trip
5. RouteMatchingJob привязывает к тому же route
6. `GET /v1/routes` → trips_count=2
7. Web Routes page показывает 1 route с 2 trips

---

## Documentation Plan

- Обновить EPIC.md Implementation Log после каждой фазы
- Заполнить Final Report после завершения
- swaggo аннотации = документация API

---

## Implementation Log

### Phase 1 — Data model (2026-06-12)

Migrations 0006 и 0007 созданы:
- `routes` — с generated columns `start_geom`/`end_geom` для spatial index
- `route_trips` — с UNIQUE (trip_id) для idempotency

### Phase 2 — tracking-worker (2026-06-12)

- `domain/route.go` — Route, RouteTrip, GeoPoint, RouteMatchConfig, DefaultRouteMatchConfig()
- `usecase/route_matching.go` — pure algorithm: normalizePolyline, avgPointDistance, haversine, FindBestMatch
- `usecase/route_matching_job.go` — periodic job, pattern from TripDetectionJob
- `usecase/route_matching_test.go` — 10 unit tests, все прошли
- `storage/postgres/route_repo.go` — RouteRepository: FindUnmatchedTrips (JOIN с raw_location_points для lat/lon), FindCandidateRoutes (ST_DWithin), FindRouteTemplate (ST_DumpPoints), InsertRoute (ST_GeomFromText WKT), InsertRouteTrip (ON CONFLICT DO NOTHING), IncrRouteStats
- `config/config.go` — добавлен RouteMatchingIntervalSec (default 60s)
- `app/app.go` — RouteMatchingJob wired рядом с TripDetectionJob

Key insight: `trip_points` не содержит lat/lon — нужен JOIN с `raw_location_points`.

### Phase 3 — tracking-gateway (2026-06-12)

- `domain/route_query.go` — Route, RouteTrip, RoutePoint read models
- `usecase/route_query.go` — RouteQueryUsecase с user isolation
- `storage/postgres/route_query_repo.go` — cursor-based pagination, ST_DumpPoints для template polyline
- `transport/http/route_handler.go` — 4 endpoints с swaggo аннотациями
- `transport/http/swagger_types.go` — RouteItem, RouteTripItem, RoutePointItem, envelopes
- `transport/http/router.go` — GET /v1/routes зарегистрированы
- `app/app.go` — RouteQueryUsecase wired

### Phase 4 — OpenAPI + codegen (2026-06-12)

- `make swagger-gen` — успешно, RouteItem/RouteTripItem/RoutePointItem сгенерированы
- Исправлен `make ts-client`: сломанный openapi-typescript → orval + orval.config.local.ts
- `make ts-client` — успешно, `apps/web/src/api/generated/routes/` создан

### Phase 5 — Web client (2026-06-12)

- `features/routes/routesApi.ts` — listRoutes, getRoute, getRouteTrips, getRoutePoints
- `pages/RoutesPage.tsx` — sidebar со списком routes, карта с template polyline, таблица trips
- `AppLayout.tsx` — добавлен Routes nav link (Navigation icon)
- `App.tsx` — добавлен /routes route
- `make web-build` — успешно

---

## Final Report

_Пусто — заполнять при завершении EPIC._
