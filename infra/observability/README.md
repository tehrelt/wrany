# Observability Stack

Prometheus + Grafana + Loki + Promtail for local development.

## Start

```bash
make up                    # core stack
make observability-up      # observability profile
```

Or together:

```bash
docker compose --profile observability up -d
```

## Local URLs

| Service    | URL                      |
|------------|--------------------------|
| Grafana    | http://localhost:3001     |
| Prometheus | http://localhost:9090     |
| Loki       | http://localhost:3100     |

Grafana login: `admin` / `admin` (or `GRAFANA_PASSWORD` from `.env`).

Anonymous read access is enabled — no login required for dashboards.

## Dashboards

All dashboards are provisioned automatically on startup.

| Dashboard            | Description                                           |
|----------------------|-------------------------------------------------------|
| Backend Overview     | HTTP request rate, latency, errors, in-flight         |
| WebSocket Ingestion  | WS connections, batches, points per second, NATS pub  |
| Worker Jobs          | NATS consume rate, raw points, trip detection, routes |
| Logs Overview        | Aggregated logs from all services via Loki            |

## Useful Loki Queries

```logql
# All logs from tracking-gateway
{job="tracking-gateway"}

# Filter by request_id
{job="tracking-gateway"} |= "request_id" | json | request_id = "abc-123"

# Filter by session_id (WebSocket)
{job="tracking-gateway"} | json | session_id = "xyz-456"

# Worker errors by event_id
{job="tracking-worker"} | json | level = "ERROR"

# Dead-letter events
{job="tracking-worker"} |= "dead_letter"
```

## Metric Naming

All custom metrics follow the pattern `<subsystem>_<name>_<unit>`.

High-cardinality values (user_id, device_id, event_id, request_id) are **never** used as Prometheus labels — they go into structured logs only.

## Health Checks

```bash
make prometheus-check   # curl Prometheus /-/ready
make grafana-check      # curl Grafana /api/health
make loki-check         # curl Loki /ready
```

## Stop

```bash
make observability-down
```

## Limitations

- Filesystem storage for Loki — data is lost on container restart.
- Promtail reads Docker container logs; only works when services run inside Docker.
- No distributed tracing (OpenTelemetry / Tempo) in this epic — deferred.
