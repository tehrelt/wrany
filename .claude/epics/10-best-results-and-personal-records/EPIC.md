# EPIC 10: Best Results & Personal Records

## Status

Done

## Goal

Реализовать Personal Records для routes:
- лучший trip по времени (min duration_sec);
- последний trip (max started_at);
- сравнение latest vs best;
- статистику attempts;
- REST API для route results;
- web UI для просмотра лучших результатов.

## Context

Pipeline после EPIC 09:

```text
Android Tracker
  ↓ WebSocket
tracking-gateway
  ↓ NATS location.events.v1
tracking-worker
  ↓ raw_location_points → Trip Detection Engine
trips + trip_points
  ↓ Route Matching Engine
routes + route_trips
  ↓ (EPIC 10)
route results / personal records
```

Данные для расчётов уже есть в `route_trips` (duration_sec, distance_m, matched_at)
и `trips` (started_at, status). Новые таблицы не нужны — calculated-on-read.

## Problem Analysis

Нужно ответить на вопросы:
1. Какой trip лучший? → min(duration_sec) среди completed trips
2. Какой trip последний? → max(started_at) среди completed trips
3. Насколько последний хуже/лучше лучшего? → разница в секундах и процентах
4. Сколько всего попыток? → COUNT(*) completed trips
5. Как безопасно обработать маршрут без attempts? → nullable best/latest

Тай-брейкер для best:
1. duration_sec ASC
2. started_at ASC (при равных duration)
3. trip_id ASC (стабильная сортировка)

## Best Practice Research

**Option A (calculated-on-read)** выбран для MVP:
- Нет отдельной таблицы → нет риска рассинхрона
- Простые SQL запросы на существующих данных
- Достаточная производительность для MVP объёмов
- При необходимости легко добавить materialized view позже

**Option B (materialized route_results table)** — отложено:
- Нужен NATS consumer для обновлений
- Добавляет сложность без реальной необходимости на MVP

**Keyset pagination** для attempts:
- `ORDER BY rt.matched_at DESC, rt.trip_id DESC`
- Курсор кодирует оба поля → нет дублей/пропусков при одинаковых matched_at

## Solution Design

### Новые endpoints

```
GET /v1/routes/{route_id}/results
GET /v1/routes/{route_id}/attempts
```

### GET /v1/routes/{route_id}/results response

```json
{
  "route_id": "uuid",
  "attempts_count": 7,
  "best": {
    "trip_id": "uuid",
    "started_at": "RFC3339",
    "duration_sec": 754,
    "distance_m": 950.0,
    "avg_speed_mps": 1.26
  },
  "latest": {
    "trip_id": "uuid",
    "started_at": "RFC3339",
    "duration_sec": 805,
    "distance_m": 960.0,
    "avg_speed_mps": 1.19
  },
  "comparison": {
    "latest_vs_best_sec": 51,
    "latest_vs_best_percent": 6.76
  }
}
```

Если 0 attempts: `best=null`, `latest=null`, `comparison=null`, `attempts_count=0`.
Если latest == best: comparison нули, web показывает "personal record".

### GET /v1/routes/{route_id}/attempts response

```json
{
  "items": [
    {
      "trip_id": "uuid",
      "started_at": "RFC3339",
      "ended_at": "RFC3339",
      "duration_sec": 754,
      "distance_m": 950.0,
      "avg_speed_mps": 1.26,
      "match_score": 0.91,
      "is_best": true
    }
  ],
  "next_cursor": null
}
```

Pagination: keyset cursor (matched_at + trip_id), default limit 50, max 200.

### SQL стратегия

`GetRouteResult` — последовательные запросы (no goroutines):
1. Ownership check через существующий GetRoute
2. COUNT(*) completed trips
3. Best trip (ORDER BY duration_sec ASC, started_at ASC, trip_id ASC LIMIT 1)
4. Latest trip (ORDER BY started_at DESC, trip_id ASC LIMIT 1)

`ListRouteAttempts` — один запрос с keyset cursor + best_trip_id для аннотации is_best.

## Architecture Notes

Clean Architecture в `tracking-gateway`:

```text
domain/route_result.go           ← TripResult, TripAttempt, RouteResult
usecase/route_result_query.go    ← RouteResultQueryUsecase + RouteResultRepo interface
storage/postgres/route_result_query_repo.go  ← SQL реализация
transport/http/route_result_handler.go       ← HTTP handlers + swagger types
transport/http/router.go         ← регистрация новых routes
app/app.go                       ← wiring
```

Переиспользование существующего кода:
- `RouteQueryRepo.GetRoute(ctx, routeID, userID)` — ownership check
- `auth()` middleware wrapper — JWT защита
- Cursor/envelope паттерны из `route_query_repo.go` и `swagger_types.go`

Swagger annotations → `make openapi-generate` → TypeScript codegen.
Web использует только generated client, не ручные DTO.

## Tasks

- [x] Создать ветку epic/10-best-results-and-personal-records
- [x] Создать EPIC.md
- [x] domain/route_result.go
- [x] usecase/route_result_query.go
- [x] storage/postgres/route_result_query_repo.go
- [x] transport/http/route_result_handler.go + swagger_types additions
- [x] transport/http/router.go — регистрация endpoints
- [x] app/app.go — wiring
- [x] usecase/route_result_query_test.go
- [x] storage/postgres/route_result_query_repo_test.go
- [x] make openapi-generate && make openapi-merge
- [x] make api-client-generate
- [x] web: RoutesPage.tsx — Personal Records section
- [x] make test passes
- [x] Final Report

## Acceptance Criteria

- `GET /v1/routes/{route_id}/results` существует
- `GET /v1/routes/{route_id}/attempts` существует
- User видит только собственные route results
- Best trip = минимальный duration_sec (с тай-брейкером)
- Latest trip = максимальный started_at
- Comparison latest vs best корректен
- Attempts endpoint пагинирован (keyset cursor)
- Attempts содержат is_best
- Zero-attempt route обработан безопасно (nulls)
- OpenAPI регенерирован
- TypeScript client регенерирован
- Web Routes page показывает best/latest/comparison
- Best attempt выделен в таблице
- `make test` проходит
- EPIC 11 не начат

## Test Plan

### Unit tests (usecase)

- comparison calc
- div-by-zero guard (duration_sec=0)
- nil best/latest handling

### Integration tests (storage/postgres)

- best trip = minimal duration_sec
- latest trip = latest started_at
- tie-breaker (equal duration → earlier started_at → stable trip_id)
- attempts_count correct
- attempts pagination (cursor advances correctly, no skip/dup)
- is_best flag correct
- user isolation (чужой route → not found)
- zero-attempt route → null best/latest, count=0
- invalid route_id → not found

### Manual E2E

1. Seed route с 2+ completed trips
2. GET /v1/routes/{id}/results → verify best/latest/comparison
3. GET /v1/routes/{id}/attempts → verify is_best, pagination
4. Open web Routes page → verify PR section

## Documentation Plan

- EPIC.md Implementation Log обновляется после каждого значимого изменения
- Final Report заполняется перед merge
- OpenAPI spec обновлён
- TypeScript client обновлён

## Implementation Log

### 2026-06-12

- Создана ветка `epic/10-best-results-and-personal-records`
- Создан EPIC.md (planning sections filled)
- Реализован backend:
  - `domain/route_result.go` — TripResult, RouteResult, TripAttempt, TripAttemptFilter
  - `usecase/route_result_query.go` — RouteResultQueryUsecase, comparison logic
  - `storage/postgres/route_result_query_repo.go` — CTE query, keyset pagination
  - `transport/http/route_result_handler.go` — GetRouteResult, ListRouteAttempts
  - `transport/http/swagger_types.go` — добавлены DTO для новых endpoints
  - `transport/http/router.go` — зарегистрированы /results и /attempts
  - `app/app.go` — wired RouteResultQueryRepo + Usecase
- Тесты:
  - `usecase/route_result_query_test.go` — 9 unit-тестов (все PASS)
  - `storage/postgres/route_result_query_repo_test.go` — 10 integration-тестов (все PASS)
- OpenAPI регенерирован (`swag init`)
- TypeScript client регенерирован (`npx orval`)
- Web:
  - `routesApi.ts` — добавлены getRouteResults, getRouteAttempts
  - `RoutesPage.tsx` — Personal Records секция, AttemptsTable с is_best

## Final Report

Реализован EPIC 10 — Best Results & Personal Records.

**Backend (tracking-gateway):**
- `domain/route_result.go` — TripResult, RouteResult, TripAttempt, TripAttemptFilter
- `usecase/route_result_query.go` — RouteResultQueryUsecase с comparison logic
- `storage/postgres/route_result_query_repo.go` — calculated-on-read через SQL, keyset pagination
- `transport/http/route_result_handler.go` — GET /v1/routes/{id}/results + /attempts
- OpenAPI регенерирован, TypeScript client обновлён

**Тесты:**
- 9 unit-тестов usecase (comparison, div-by-zero, nil handling) — PASS
- 10 integration-тестов storage (best/latest/tie-breaker/pagination/isolation/zero-attempts) — PASS

**Web:**
- Personal Records секция на RoutesPage (best/latest/comparison)
- AttemptsTable с подсветкой best attempt

**Acceptance Criteria:** все выполнены.
