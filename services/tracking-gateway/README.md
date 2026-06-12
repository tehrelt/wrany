# tracking-gateway

HTTP API gateway for the WR any% tracking system.

## Endpoints

### Auth

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/v1/auth/register` | — | Register a new user |
| `POST` | `/v1/auth/login` | — | Login and receive tokens |
| `POST` | `/v1/auth/refresh` | — | Rotate refresh token |
| `GET`  | `/v1/me` | Bearer JWT | Current user info |

### Devices

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/v1/devices/register` | Bearer JWT | Register or update a device |
| `GET`  | `/v1/devices` | Bearer JWT | List current user's devices |

### Health & Metrics

```
GET /healthz
GET /metrics    Prometheus metrics
```

## Auth Flow

```sh
# Register
curl -X POST http://localhost:8080/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"securepass"}'

# Login
curl -X POST http://localhost:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"securepass"}'

# Use access token
curl http://localhost:8080/v1/me \
  -H "Authorization: Bearer <access_token>"

# Refresh tokens
curl -X POST http://localhost:8080/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{"refresh_token":"<refresh_token>"}'

# Register device
curl -X POST http://localhost:8080/v1/devices/register \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{"device_id":"550e8400-e29b-41d4-a716-446655440000","name":"Pixel 7","platform":"android"}'
```

## Observability

### Prometheus Metrics

`GET /metrics` — exposed on the same port as the API.

Key metrics:

| Metric | Type | Description |
|--------|------|-------------|
| `ws_connections_active` | Gauge | Active WebSocket connections |
| `ws_connections_total` | Counter | Total WS connections accepted |
| `location_batches_received_total` | Counter | Location batches received |
| `location_points_received_total` | Counter | Location points received |
| `nats_publish_total` | Counter | NATS publish attempts |
| `nats_publish_errors_total` | Counter | NATS publish errors |
| `http_requests_total` | Counter | HTTP requests by method/endpoint/status |
| `http_request_duration_seconds` | Histogram | HTTP request latency |
| `http_requests_in_flight` | Gauge | Concurrent HTTP requests |
| `db_query_duration_seconds` | Histogram | DB query latency |
| `auth_requests_total` | Counter | Auth attempts by result |

### Structured Logs

All logs are JSON (`slog`). Common fields:

```json
{"time":"...","level":"INFO","service":"tracking-gateway","request_id":"uuid","msg":"..."}
```

WebSocket logs also include `session_id`, `user_id`, `device_id`, `remote_addr`.

### Correlation IDs

- HTTP: `X-Request-Id` header — preserved if present, generated if absent. Echoed in response.
- WebSocket: `session_id` (UUID) generated per connection, present in all WS log lines.

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `GATEWAY_PORT` | no | `8080` | HTTP listen port |
| `DATABASE_URL` | **yes** | — | PostgreSQL DSN |
| `JWT_SECRET` | **yes** | — | JWT signing secret (min 32 chars) |
| `JWT_ACCESS_TTL` | no | `15m` | Access token TTL |
| `JWT_REFRESH_TTL` | no | `168h` | Refresh token TTL (7 days) |
| `MIGRATIONS_PATH` | no | `./infra/migrations` | Path to migration files |

Copy `.env.example` to `.env` and fill in required values.

## Migrations

Migration tool: [`golang-migrate/migrate/v4`](https://github.com/golang-migrate/migrate).

Migration files: `infra/migrations/`.

### Run locally (requires `migrate` CLI)

```sh
# Install migrate CLI once
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Apply all pending migrations
make migrate-up

# Roll back last migration
make migrate-down

# Show current schema version
make migrate-version
```

### Docker / Production

Migrations run automatically on container startup **before** the server starts.
If migrations fail, the container exits with a non-zero code — the service does not start.

No manual migration step is required in production; the entrypoint handles it.

## Architecture

Clean Architecture / Hexagonal:

```
cmd/tracking-gateway/     composition root
internal/
  config/                 env vars
  domain/                 entities, errors (no external deps)
  usecase/                business logic + repository interfaces
  storage/postgres/       pgx/v5 repository implementations
  transport/http/         handlers, middleware, router
  migrations/             golang-migrate runner
  app/                    wires everything, manages lifecycle
infra/migrations/         SQL migration files
```

Dependency direction: `transport → usecase → domain`, `storage → usecase interfaces`.
