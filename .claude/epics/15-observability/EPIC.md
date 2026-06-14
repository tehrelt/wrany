# EPIC 15: Observability & Reliability

## Status

In Progress

## Goal

Добавить реальную наблюдаемость: проверка зависимостей в healthcheck, структурированные доменные логи (не только HTTP-запросы), заполнение пустых Grafana-дашбордов, и распределённый трейсинг через OpenTelemetry + Tempo.

## Context

Базовая инфраструктура уже работает: Prometheus, Grafana, Loki, Promtail, /metrics и /healthz на обоих сервисах. Но /healthz возвращает {"status":"ok"} без проверки зависимостей, доменные события не логируются, два дашборда пустые, traces отсутствуют.

## Problem Analysis

1. `/healthz` не проверяет DB/NATS — оркестратор не знает о деградации зависимостей.
2. Логи покрывают только HTTP-запросы; trip created/completed, batch accepted, noise summary — не видны в Loki.
3. Невозможно проследить путь события gateway → NATS → worker → DB.
4. WebSocket Ingestion и Worker Jobs дашборды пустые (нет нужных панелей).

## Best Practice Research

- Liveness vs Readiness split (Kubernetes pattern): liveness = процесс жив, readiness = все зависимости доступны.
- Structured domain event logging на уровне usecase: user_id, device_id в каждой записи.
- OpenTelemetry W3C traceparent propagation через NATS headers для cross-service tracing.
- Loki derived fields: trace_id из JSON лога → ссылка в Tempo.

## Solution Design

- `GET /readyz` (новый) пингует DB и NATS, возвращает 503 + JSON с детальным статусом при деградации.
- `libs/observability/logger/context.go` расширяется: `WithUserID`, `WithDeviceID`, обновлённый `FromContext`.
- Usecase-слой получает `*slog.Logger` через инжекцию; логирует доменные события с контекстом.
- `libs/observability/tracing/` — новая lib: `Init(Config)` инициализирует TracerProvider, `InjectToHeaders`/`ExtractFromHeaders` для NATS.
- Tempo добавляется в docker-compose под профилем `observability`.

## Architecture Notes

- Task 1 (healthcheck) и Task 2 (логи) независимы — можно реализовывать параллельно.
- Task 4 (OTel) зависит от Task 2 (`WithTraceID` добавляется в context.go).
- Task 3 (дашборды) — последний (Tempo datasource нужен для log-trace панелей).
- `libs/eventbus/nats` получает только OTel API (без SDK) — SDK только в services.

## Tasks

- [x] EPIC.md
- [ ] Task 1: /readyz с проверкой DB + NATS (оба сервиса)
- [ ] Task 2: Доменные логи (tracker_ingestion, trip_detection, route_matching, location_processor)
- [ ] Task 4: OpenTelemetry + Tempo (libs/observability/tracing, инструментирование HTTP/pgx/NATS)
- [ ] Task 3: Дашборды (websocket-ingestion, worker-jobs, logs-overview, backend-overview)

## Acceptance Criteria

- `curl /readyz` → 503 когда NATS выключен, 200 когда всё живо
- В Loki видны логи trip_created с полями user_id, device_id, trip_id
- В Grafana Explore (Tempo) виден trace: HTTP handler → pgx query
- `trace_id` в лог-записи → кликабельная ссылка в Tempo
- `go test ./...` и `go vet ./...` зелёные

## Test Plan

- Unit: HealthHandler.Readiness с mock DB/NATS (200 vs 503)
- Integration: реальный testcontainer postgres — ping проходит
- Manual: `docker compose --profile observability up`, Grafana Explore Tempo

## Documentation Plan

EPIC.md (этот файл). Inline comments на нетривиальных местах.

## Implementation Log

### Reliability: WS auth token refresh (background tracker)

**Problem.** The native Android foreground tracking service (`BatchSender`) opened its
WebSocket with a static `access_token` read from prefs. The gateway rejects an
invalid/expired token at the HTTP upgrade with 401 (`WSAuthMiddleware`), which surfaces
in OkHttp as `onFailure(... response.code == 401)`. The service then reconnected with the
**same stale token**, looping 401 forever — background tracking silently died once the
short-lived access token expired, with no way to recover without restarting the service.

**Fix.**
- `BatchSender` now refreshes the token itself on a 401/403 upgrade failure
  (`POST /v1/auth/refresh`), persists the rotated pair to prefs, and reconnects.
  Single-flight guard + `MAX_REFRESH_ATTEMPTS` cap prevent refresh/connect storms;
  on refresh failure it marks `authExpired` and stops the loop until re-login.
- `enableTracking` now also receives `refresh_token` and `api_url` (the service needs
  them to refresh without JS).
- Refresh tokens rotate server-side (each refresh revokes the previous), so JS and native
  token stores are reconciled (`tokenSync.ts`): JS pushes its rotated pair to native after
  an HTTP-side refresh; JS pulls the service's pair on app start / screen mount. After
  re-login the screen pushes fresh creds into a still-running `authExpired` service and
  reconnects.

**Files.** `BatchSender.kt`, `TrackingModule.kt`, `TrackingForegroundService.kt`,
`tokenSync.ts`, `httpClient.ts`, `App.tsx`, `TrackingStatusScreen.tsx`,
`trackingNativeModule.ts`, `types.ts`.

**Verification.** `tsc --noEmit` clean for changed files (3 pre-existing `Platform.Version`
typing errors unrelated); `eslint` 0 errors. Kotlin not compiled here (no Android SDK in
this environment) — needs a device/emulator build before closing.

### Removed legacy debug WS tab

Deleted the JS-only "Legacy WS" tracker tab (manual foreground WebSocket via
`react-native-geolocation-service`), superseded by the native background service.
Removed: `screens/TrackerScreen.tsx`, `tracker/{trackerSocket,batchQueue,locationService,
types}.ts`, `__tests__/batchQueue.test.ts`; trimmed `App.tsx` (tab, render,
`handleSessionExpired`). `tracker/deviceId.ts` kept (used by the native path).
`tsc`/`eslint` clean, `jest` 2/2 suites pass.

## Final Report

TBD
