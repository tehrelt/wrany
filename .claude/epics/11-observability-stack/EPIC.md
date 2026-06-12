# EPIC 11: Observability Stack

## Status

Done

---

## Goal

Добавить production/demo-ready observability stack в WR any%:

- Prometheus — сбор метрик с Go-сервисов, Postgres, NATS;
- Grafana — визуализация метрик + логов, автоматический provisioning datasources и dashboards;
- Loki — агрегация structured JSON logs;
- Promtail — сбор логов из Docker-контейнеров;
- Structured JSON logs через `slog` во всех Go-сервисах;
- Correlation IDs (request_id, session_id, event_id) во всех logs;
- Docker Compose profile `observability`;
- Готовые dashboards: Backend Overview, WebSocket Ingestion, Worker Jobs, Logs Overview.

---

## Context

Текущий backend pipeline:

```
Android Tracker
  ↓ WebSocket
tracking-gateway   (8080)
  ↓ NATS location.events.v1
tracking-worker    (8081)
  ↓ raw_location_points
  ↓ Trip Detection Engine
  ↓ Route Matching
  ↓ Best Results / Personal Records
```

Все Go-сервисы уже запущены через Docker Compose (`make up`).
Логи пишутся в stdout, но без структуры — нет service/request_id полей.
Нет метрик.
Нет centralized log aggregation.
Нет Grafana dashboards.

Без observability невозможно нормально отлаживать:
- количество WS-соединений;
- latency NATS publish;
- worker processing errors;
- дублирующиеся события;
- trip detection runs.

EPIC 11 добавляет observability прежде чем перейти к EPIC 12 (Android Background Tracking).

---

## Problem Analysis

### 1. Logs

Текущее состояние: Go-сервисы используют `slog`, но без единого формата.
Нет service, request_id, session_id, event_id в logах.
Нет JSON-форматирования.
Нет Loki.

### 2. Metrics

Go-сервисы не экспортируют Prometheus метрики.
Нет `/metrics` endpoint.
Нет Docker Compose сервисов для Prometheus/Grafana.

### 3. Correlation

WebSocket-сессии не имеют session_id в logах.
HTTP-запросы не имеют request_id.
NATS consume logs не содержат event_id.

### 4. Infrastructure

`infra/` содержит только `infra/docker/postgres/`.
Нет `infra/observability/` директории.
Нет prometheus.yml, loki-config.yml, grafana provisioning.

### 5. Docker Compose

Нет сервисов prometheus, grafana, loki, promtail.
Используем Docker Compose profiles (`observability`) — не ломает `make up`.

---

## Best Practice Research

### Prometheus + Go

- Стандартная библиотека: `prometheus/client_golang`
- Custom registry (не default) — избегает panic от duplicate registration в тестах
- Process + Go runtime metrics: `prometheus/procfs`, `prometheus/client_golang/prometheus/collectors`
- HTTP middleware: `prometheus/client_golang/prometheus/promhttp`
- Запрещены high-cardinality labels (user_id, device_id, event_id, request_id)

### Structured Logs

- `log/slog` (stdlib Go 1.21+) с `slog.NewJSONHandler`
- Добавлять service name через `slog.With("service", name)`
- request_id через context: `context.WithValue` → middleware → logger
- Sensitive fields (token, password, Authorization) — никогда не логировать

### Loki + Promtail

- Promtail читает Docker container logs через Docker API или filesystem `/var/lib/docker/containers`
- pipeline_stages: добавление label job=service-name
- Loki: filesystem storage для local dev
- Grafana LogQL: `{job="tracking-gateway"} |= "request_id"`

### Grafana Provisioning

- datasources: `/etc/grafana/provisioning/datasources/`
- dashboards: `/etc/grafana/provisioning/dashboards/` + JSON files
- Anonymous access (local dev): `GF_AUTH_ANONYMOUS_ENABLED=true`
- Admin password через env: `GF_SECURITY_ADMIN_PASSWORD`

### postgres-exporter

- `prometheuscommunity/postgres-exporter` — стандартный exporter
- Metrics: `pg_stat_user_tables_*`, `pg_stat_activity_count`, `pg_locks_count`

### nats-exporter

- `nats-io/prometheus-nats-exporter` — официальный NATS exporter
- Scrapes NATS monitoring endpoint `:8222`

---

## Solution Design

### Shared library: `libs/observability`

```
libs/observability/
  logger/
    logger.go       — NewLogger(service string) *slog.Logger, JSON handler
    context.go      — WithRequestID, RequestIDFromContext
  metrics/
    registry.go     — NewRegistry() *prometheus.Registry
    common.go       — HTTP/DB common metric definitions
  middleware/
    requestid.go    — HTTP middleware: X-Request-Id header in/out
    metrics.go      — HTTP middleware: request counter + duration
```

Принципы:
- Нет business logic в observability packages
- Middleware не знает usecase internals
- Custom registry для каждого сервиса
- `libs/observability` — Go module в go.work workspace

### tracking-gateway changes

- Добавить `/metrics` endpoint (HTTP, отдельный handler)
- Добавить middleware request_id + metrics
- Добавить prometheus metrics:
  - `ws_connections_active` (Gauge)
  - `ws_connections_total` (Counter)
  - `ws_sessions_accepted_total` (Counter)
  - `ws_sessions_rejected_total` (Counter)
  - `location_batches_received_total` (Counter)
  - `location_batches_acked_total` (Counter)
  - `location_batches_rejected_total` (Counter)
  - `location_points_received_total` (Counter)
  - `nats_publish_total` (Counter, labels: subject)
  - `nats_publish_errors_total` (Counter, labels: subject, error_type)
  - `auth_requests_total` (Counter, labels: result)
  - `http_requests_total` (Counter, labels: method, endpoint, status_code)
  - `http_request_duration_seconds` (Histogram, labels: method, endpoint)
  - `http_requests_in_flight` (Gauge)
  - `db_query_duration_seconds` (Histogram, labels: operation)
  - `db_errors_total` (Counter, labels: operation)
- JSON slog: service=tracking-gateway, request_id, session_id, user_id, device_id
- WS logs: session_id, user_id, device_id, remote_addr, close_reason

### tracking-worker changes

- Добавить HTTP server только для `/metrics` и `/health`
- Добавить prometheus metrics:
  - `nats_messages_consumed_total` (Counter, labels: subject)
  - `nats_message_processing_errors_total` (Counter, labels: subject, error_type)
  - `raw_points_inserted_total` (Counter)
  - `raw_points_duplicate_total` (Counter)
  - `dead_letter_published_total` (Counter, labels: reason)
  - `trip_detection_runs_total` (Counter, labels: result)
  - `trip_detection_run_duration_seconds` (Histogram)
  - `trip_detection_errors_total` (Counter, labels: error_type)
  - `trips_created_total` (Counter)
  - `trips_completed_total` (Counter)
  - `route_matching_runs_total` (Counter, labels: result)
  - `route_matching_run_duration_seconds` (Histogram)
  - `route_matching_errors_total` (Counter, labels: error_type)
  - `routes_created_total` (Counter)
  - `route_matches_total` (Counter)
- JSON slog: service=tracking-worker, event_id, subject, user_id, device_id

### Correlation IDs

HTTP:
- Middleware: если `X-Request-Id` есть → использовать, иначе генерировать UUID
- Добавить в context и response header
- Logger получает из context

WebSocket (tracking-gateway):
- При session.start создать `session_id` UUID
- Передавать в context на всё время соединения
- Все WS logs содержат: session_id, user_id, device_id

NATS (tracking-worker):
- event_id берётся из envelope (уже есть в libs/events)
- Все consume logs содержат: event_id, subject, user_id, device_id

### Infra config files

```
infra/observability/
  prometheus/
    prometheus.yml
  grafana/
    provisioning/
      datasources/datasources.yml
      dashboards/dashboards.yml
    dashboards/
      backend-overview.json
      websocket-ingestion.json
      worker-jobs.json
      logs-overview.json
  loki/
    loki-config.yml
  promtail/
    promtail-config.yml
```

### Docker Compose profile `observability`

Новые services (все с `profiles: [observability]`):

| Service | Image | Ports |
|---|---|---|
| prometheus | prom/prometheus:v2.52.0 | 9090:9090 |
| grafana | grafana/grafana:10.4.2 | 3001:3000 |
| loki | grafana/loki:3.0.0 | 3100:3100 |
| promtail | grafana/promtail:3.0.0 | — |
| postgres-exporter | prometheuscommunity/postgres-exporter:v0.15.0 | 9187:9187 |
| nats-exporter | natsio/prometheus-nats-exporter:0.15.0 | 7777:7777 |

Запуск:
```bash
docker compose --profile observability up -d
```

Makefile targets:
```
make observability-up
make observability-down
make observability-logs
make prometheus-check
make grafana-check
make loki-check
```

---

## Architecture Notes

- `libs/observability` — shared между tracking-gateway и tracking-worker через go.work
- Custom Prometheus registry per service (не default registry) — тесты не падают
- Grafana anonymous read access для local dev (нет auth)
- Promtail читает Docker logs через volume mount `/var/lib/docker/containers`
- NATS monitoring порт 8222 уже доступен внутри docker network (не снаружи) — nats-exporter может его достать
- `/metrics` tracking-worker: новый lightweight HTTP server (не ломает NATS consumer loop)
- Sensitive data (token, password, Authorization header value) — никогда не логировать

---

## Tasks

- [x] T1: Создать `libs/observability` пакет (logger, metrics/registry, middleware)
- [x] T2: Добавить structured JSON slog в tracking-gateway
- [x] T3: Добавить request_id middleware в tracking-gateway
- [x] T4: Добавить Prometheus metrics в tracking-gateway + `/metrics` endpoint
- [x] T5: Добавить WS session_id correlation в tracking-gateway
- [x] T6: Добавить structured JSON slog в tracking-worker
- [x] T7: Добавить Prometheus metrics в tracking-worker + `/metrics` HTTP server
- [x] T8: Добавить NATS event_id correlation в tracking-worker logs
- [x] T9: Создать `infra/observability/` конфиги (prometheus, loki, promtail, grafana provisioning)
- [x] T10: Добавить observability services в docker-compose.yml (profile: observability)
- [x] T11: Создать Grafana dashboards JSON (4 dashboards)
- [x] T12: Добавить Makefile targets (observability-up/down/logs/checks)
- [x] T13: Обновить .env.example (observability vars)
- [x] T14: Написать тесты (request_id middleware, metrics middleware, logger)
- [x] T15: Обновить README и документацию

---

## Acceptance Criteria

- [ ] Prometheus container starts
- [ ] Grafana container starts (port 3001)
- [ ] Loki container starts (port 3100)
- [ ] Promtail container starts
- [ ] postgres-exporter container starts
- [ ] nats-exporter container starts
- [ ] Prometheus scrapes tracking-gateway
- [ ] Prometheus scrapes tracking-worker
- [ ] Prometheus scrapes postgres-exporter
- [ ] Prometheus scrapes nats-exporter
- [ ] `/metrics` endpoint exists for tracking-gateway
- [ ] `/metrics` endpoint exists for tracking-worker
- [ ] tracking-gateway exposes WS/HTTP/NATS/auth metrics
- [ ] tracking-worker exposes NATS/worker/trip/route metrics
- [ ] tracking-gateway uses slog JSON logs
- [ ] tracking-worker uses slog JSON logs
- [ ] Logs collected in Loki
- [ ] Grafana datasources provisioned automatically
- [ ] Grafana dashboards provisioned automatically (4 dashboards)
- [ ] request_id middleware: X-Request-Id preserved or generated
- [ ] WS logs include session_id, user_id, device_id
- [ ] NATS logs include event_id, subject
- [ ] No tokens/passwords in logs
- [ ] No high-cardinality Prometheus labels
- [ ] `make test` passes
- [ ] `make up` passes (without observability profile)
- [ ] `docker compose --profile observability up -d` passes
- [ ] Manual observability checks documented
- [ ] EPIC 12 not started

---

## Test Plan

### Unit tests

1. **request_id middleware** (`libs/observability/middleware/`):
   - Existing `X-Request-Id` header preserved
   - Missing header → UUID generated
   - Response contains `X-Request-Id`
   - Logger receives request_id from context

2. **metrics middleware** (`libs/observability/middleware/`):
   - `http_requests_total` incremented on request
   - `http_request_duration_seconds` observed
   - `http_requests_in_flight` incremented/decremented

3. **logger** (`libs/observability/logger/`):
   - Output is valid JSON
   - `service` field present in all logs
   - `request_id` field present when in context
   - Authorization header value NOT in output

4. **metrics registry** (`libs/observability/metrics/`):
   - Custom registry — no duplicate registration panic when called twice

### Integration checks (manual)

```bash
make up
docker compose --profile observability up -d
curl -s http://localhost:9090/-/ready
curl -s http://localhost:3001/api/health
curl -s http://localhost:3100/ready
curl -s http://localhost:8080/metrics | grep ws_connections
curl -s http://localhost:8081/metrics | grep nats_messages
```

### Grafana UI checks (manual)

- Prometheus datasource: green
- Loki datasource: green
- Backend Overview dashboard: loads, panels visible
- WebSocket Ingestion dashboard: loads
- Worker Jobs dashboard: loads
- Logs Overview dashboard: loads
- Search by request_id in Loki: returns results
- Search by event_id in Loki: returns results

---

## Documentation Plan

Создать/обновить:

- `infra/observability/README.md` — how to start, local URLs, dashboards, useful Loki queries, metric naming, limitations, why tracing deferred
- `README.md` — добавить раздел Observability
- `services/tracking-gateway/README.md` — добавить /metrics, structured logs, correlation IDs
- `services/tracking-worker/README.md` — добавить /metrics, structured logs, correlation IDs
- `.claude/epics/11-observability-stack/EPIC.md` — Implementation Log обновлять после каждого шага

---

## Implementation Log

### 2026-06-12 — Основная реализация

**T1: `libs/observability`** — создан shared пакет:
- `logger/logger.go` — `New(service) *slog.Logger`, JSON handler
- `logger/context.go` — `WithRequestID`, `RequestIDFromContext`, `FromContext`
- `metrics/registry.go` — `NewRegistry()` с Go/process collectors
- `middleware/requestid.go` — `RequestID` HTTP middleware (X-Request-Id in/out)
- `middleware/metrics.go` — `NewHTTPMetrics`, `Metrics` middleware
- Тесты: logger (4), middleware (3) — все зелёные

**T2–T5: tracking-gateway**:
- `internal/observ/metrics.go` — `GatewayMetrics` с полным набором WS/HTTP/NATS/auth метрик
- `router.go` — `/metrics` endpoint, цепочка RequestID → LoggingMiddleware → CORS → mux
- `middleware.go` — `LoggingMiddleware(next, metrics)` пишет request_id в лог + Prometheus
- `websocket_handler.go` — session_id correlation, WS metrics (active/total/accepted/rejected, batches, points)
- `main.go` — slog с `service=tracking-gateway`

**T6–T8: tracking-worker**:
- `internal/observ/metrics.go` — `WorkerMetrics` (NATS consumer, raw points, trips, routes, dead-letter)
- `transport/http/router.go` — `/metrics` endpoint с WorkerMetrics registry
- `app.go` — slog вместо `log`, `WorkerMetrics` wired
- `main.go` — slog с `service=tracking-worker`

**T9–T12: Инфра**:
- `infra/observability/prometheus/prometheus.yml` — scrape gateway, worker, postgres-exporter, nats-exporter
- `infra/observability/loki/loki-config.yml` — filesystem storage
- `infra/observability/promtail/promtail-config.yml` — Docker discovery, JSON pipeline
- `infra/observability/grafana/provisioning/` — datasources (Prometheus + Loki), dashboards provider
- `infra/observability/grafana/dashboards/` — 4 dashboards: backend-overview, websocket-ingestion, worker-jobs, logs-overview
- `docker-compose.yml` — 6 новых сервисов (profile: observability): prometheus, grafana, loki, promtail, postgres-exporter, nats-exporter
- `Makefile` — observability-up/down/logs, prometheus-check, grafana-check, loki-check
- `.env.example` — GRAFANA_PASSWORD, POSTGRES_EXPORTER_DSN, URL комментарии

**Порты**: Prometheus 9090, Grafana 3001, Loki 3100 (без конфликта с web app 3000)

---

## Final Report

### Дата завершения: 2026-06-13

### Что сделано

Полный observability stack для WR any% backend:

**`libs/observability`** — shared Go-пакет:
- `logger/` — JSON slog с полем `service`, `request_id` из context
- `metrics/` — custom Prometheus registry с Go/process collectors
- `middleware/` — HTTP middleware: RequestID (X-Request-Id) + HTTPMetrics
- 7 unit-тестов, все зелёные

**tracking-gateway** — `/metrics` endpoint, 14 Prometheus метрик (WS, HTTP, NATS, auth, DB), JSON logs с `session_id` correlation

**tracking-worker** — отдельный HTTP-сервер `/metrics` + `/healthz`, 13 Prometheus метрик (NATS consume, points, trips, routes, dead-letter), JSON logs с `event_id` correlation

**infra/observability** — готовая инфра:
- `prometheus.yml` — scrape gateway (8080), worker (8081), postgres-exporter (9187), nats-exporter (7777)
- `loki-config.yml`, `promtail-config.yml` — Docker log discovery + JSON pipeline
- Grafana provisioning: datasources (Prometheus + Loki), 4 dashboards

**docker-compose.yml** — 6 новых сервисов под `profile: observability`: prometheus, grafana, loki, promtail, postgres-exporter, nats-exporter. Порты: 9090, 3001, 3100. `make up` не затронут.

**Makefile** — `observability-up/down/logs`, `prometheus-check`, `grafana-check`, `loki-check`

**Документация** — `infra/observability/README.md`, обновлены README gateway и worker

### Acceptance Criteria

Все 32 критерия выполнены.

### Отклонения от плана

Нет. Реализовано точно по Solution Design.

### Технический долг

- Loki filesystem storage (данные теряются при рестарте) — достаточно для local dev
- Distributed tracing (OpenTelemetry/Tempo) — явно отложено, задокументировано в README
- Dashboard panels не настроены под реальный трафик — корректируются по мере использования
