# Tracker Ingestion Architecture

## Overview

```
Android Tracker
  ↓ WebSocket (JWT Bearer)
GET /v1/ws/tracker
  ↓ AuthMiddleware (JWT → user_id in context)
  ↓ TrackerHandler (upgrade, read loop, origin check)
  ↓ TrackerIngestionUseCase
      ├── DeviceLookup        → Postgres devices table
      ├── IngestionDedupRepo  → Postgres ingested_location_events table
      └── Publisher           → libs/eventbus → NATS JetStream
                                  ↓ location.events.v1
                              future tracking-worker
```

## Component Responsibilities

### TrackerHandler (`internal/transport/http/websocket_handler.go`)

- WebSocket upgrade with origin policy (CheckOrigin).
- Transport-level limits: `SetReadLimit(WS_MAX_MESSAGE_SIZE_BYTES)`.
- Ping/pong keepalive, read/write deadlines.
- Parses wire-format messages → dispatches to usecase.
- Does NOT contain business logic.

### TrackerIngestionUseCase (`internal/usecase/tracker_ingestion.go`)

- `StartSession`: validates device ownership via `DeviceLookup`.
- `IngestBatch`: validates events (business rules), dedup check, NATS publish, ledger insert.
- Returns `BatchResult{Accepted, Duplicated, Rejected}`.
- Does NOT import NATS packages; uses `eventbus.Publisher` interface.

### IngestionDedupRepo (`internal/storage/postgres/ingestion_dedup_repo.go`)

- `IsDuplicate(userID, deviceID, eventID)` — SELECT from ledger.
- `MarkPublished(userID, deviceID, eventID)` — INSERT ON CONFLICT DO NOTHING.
- PK: `(user_id, device_id, event_id)` — dedup is per-device, not global.

## Dedup Strategy (variant B: publish-first)

```
for each event in batch:
  1. IsDuplicate(user_id, device_id, event_id)?
     YES → add to Duplicated, skip
     NO  → continue

  2. Publish to NATS (Nats-Msg-Id = event_id)
     FAIL → add to Rejected (EVENT_BUS_UNAVAILABLE)
     OK   → MarkPublished (INSERT ON CONFLICT DO NOTHING)
            Conflict = non-fatal (publish confirmed)
            → add to Accepted

3. Send location.batch.ack ONLY after all publishes complete
```

**Why publish-first (not ledger-first):**

Variant A (ledger first → publish): if NATS fails after ledger insert, the client gets no ACK but the event is marked as published, creating a silent loss.

Variant B (publish first → ledger after PubAck): if ledger insert fails after PubAck, the client may see `accepted` again on retry → JetStream dedup window absorbs the repeat publish. Safer for client sync semantics.

## Database Schema

```sql
CREATE TABLE ingested_location_events (
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id   UUID        NOT NULL,
    event_id    TEXT        NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, device_id, event_id)
);
```

This table is NOT raw GPS storage. It is the ingestion dedup ledger only.

## NATS Headers

Each published message carries:

| Header | Value |
|--------|-------|
| `Nats-Msg-Id` | `event_id` (JetStream dedup) |
| `Wrany-Event-Type` | `location.events.v1` |
| `Wrany-Correlation-Id` | `session_id` |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `WS_MAX_MESSAGE_SIZE_BYTES` | `262144` | Transport-level frame size limit (256 KB) |
| `WS_READ_DEADLINE_SEC` | `60` | Read deadline reset on each pong |
| `WS_WRITE_DEADLINE_SEC` | `10` | Write deadline per message |
| `WS_PING_INTERVAL_SEC` | `30` | Server-initiated ping interval |
| `WS_MAX_BATCH_SIZE` | `100` | Max events per `location.batch` |
| `WS_ALLOWED_ORIGINS` | `` | Comma-separated browser origins; empty = mobile only |

## Clean Architecture Layer Map

```
cmd/tracking-gateway/main.go   — composition root
internal/app/app.go            — wiring (NATS, repos, usecases, router)
internal/config/config.go      — env vars
internal/domain/               — TrackerSession, LocationEvent, error codes
internal/usecase/              — TrackerIngestionUseCase (business rules)
internal/transport/http/       — WebSocket handler, protocol structs, router
internal/storage/postgres/     — IngestionDedupRepo, DeviceRepo
libs/eventbus/                 — Publisher interface, NopPublisher
libs/eventbus/nats/            — NATS JetStream adapter
libs/events/location/          — location.events.v1 contract
```
