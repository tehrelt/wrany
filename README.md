# WR any%

Automatic route tracking system. Detects movement start, active trip, finish, loops, repeated routes, and personal records — without manual user input.

## Requirements

- Docker 24+
- Docker Compose v2
- Go 1.22+

## Quick Start

```bash
cp .env.example .env
make up
```

Wait for all services to report `healthy`:

```bash
docker compose ps
```

## Services

| Service          | Host Port        | Description                                  |
|------------------|------------------|----------------------------------------------|
| postgres         | 5432             | PostgreSQL 16 + PostGIS 3.4                  |
| nats             | 127.0.0.1:4222   | NATS JetStream (internal bus, localhost-only bind for dev) |
| tracking-gateway | 8080             | WebSocket gateway stub                       |
| tracking-worker  | 8081             | Trip detection worker stub                   |

## Make Commands

```bash
make up            # start all services
make down          # stop all services
make logs          # follow logs
make reset         # full reset (destroys volumes)
make check-postgis # verify PostGIS is active
make db-shell      # open psql shell
make test          # run Go unit tests
make nats-check    # verify JetStream is enabled
make nats-init     # create WRANY_EVENTS stream (idempotent)
make nats-streams  # list JetStream streams
```

## Verify

```bash
curl localhost:8080/healthz   # {"status":"ok"}
curl localhost:8081/healthz   # {"status":"ok"}
make check-postgis            # PostGIS version output
```

## Architecture

- Android tracker → WebSocket → tracking-gateway → NATS JetStream → tracking-worker
- NATS JetStream is the internal event bus, never exposed to external clients
  (port 4222 is bound to 127.0.0.1 for local dev only)
- Event contracts: `libs/events`; broker abstraction: `libs/eventbus`
- Storage: PostgreSQL + PostGIS

Details: `docs/architecture/event-bus.md`, `docs/contracts/events.md`.
