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

| Service          | Host Port | Description                             |
|------------------|-----------|-----------------------------------------|
| postgres         | 5432      | PostgreSQL 16 + PostGIS 3.4             |
| kafka            | —         | Apache Kafka 3.7 (KRaft, internal only) |
| tracking-gateway | 8080      | WebSocket gateway stub                  |
| tracking-worker  | 8081      | Trip detection worker stub              |

## Make Commands

```bash
make up            # start all services
make down          # stop all services
make logs          # follow logs
make reset         # full reset (destroys volumes)
make check-postgis # verify PostGIS is active
make db-shell      # open psql shell
make test          # run Go unit tests
```

## Verify

```bash
curl localhost:8080/healthz   # {"status":"ok"}
curl localhost:8081/healthz   # {"status":"ok"}
make check-postgis            # PostGIS version output
```

## Architecture

- Android tracker → WebSocket → tracking-gateway → Kafka → tracking-worker
- Kafka is internal only, never exposed to host
- Storage: PostgreSQL + PostGIS
