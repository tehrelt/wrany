# tracking-worker

Go background worker service. Consumes location events from NATS JetStream and runs trip detection, route matching, and personal record logic.

## Port

`8081` — HTTP server for health checks and Prometheus metrics.

## Environment Variables

| Variable    | Default | Description        |
|-------------|---------|--------------------|
| WORKER_PORT | 8081    | HTTP listener port |

## Health Check & Metrics

```bash
curl localhost:8081/healthz
# {"status":"ok"}

curl localhost:8081/metrics
# Prometheus metrics
```

## Observability

### Prometheus Metrics

`GET /metrics`

Key metrics:

| Metric | Type | Description |
|--------|------|-------------|
| `nats_messages_consumed_total` | Counter | NATS messages consumed |
| `nats_message_processing_errors_total` | Counter | Processing errors by type |
| `raw_points_inserted_total` | Counter | Raw location points inserted |
| `raw_points_duplicate_total` | Counter | Duplicate points skipped |
| `dead_letter_published_total` | Counter | Events sent to dead-letter |
| `trip_detection_runs_total` | Counter | Trip detection runs by result |
| `trip_detection_run_duration_seconds` | Histogram | Trip detection latency |
| `trips_created_total` | Counter | Trips opened |
| `trips_completed_total` | Counter | Trips closed |
| `route_matching_runs_total` | Counter | Route matching runs |
| `route_matching_run_duration_seconds` | Histogram | Route matching latency |
| `routes_created_total` | Counter | New routes created |
| `route_matches_total` | Counter | Route matches found |

### Structured Logs

All logs are JSON (`slog`). Common fields:

```json
{"time":"...","level":"INFO","service":"tracking-worker","event_id":"uuid","subject":"location.events.v1","msg":"..."}
```

NATS consumer logs include `event_id`, `subject`, `user_id`, `device_id`.
