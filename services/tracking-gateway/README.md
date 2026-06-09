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

### Health

```
GET /healthz
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
