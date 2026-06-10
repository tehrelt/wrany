# WebSocket Tracker Protocol

## Endpoint

```
GET /v1/ws/tracker
Authorization: Bearer <access_token>
```

HTTP upgrade to WebSocket. JWT is required before upgrade. Unauthenticated connections receive HTTP 401 and are not upgraded.

## Origin Policy

- **Empty origin**: always allowed (Android / non-browser clients).
- **Browser origin**: must be in `WS_ALLOWED_ORIGINS` comma-separated allowlist.
- Unknown browser origin → upgrade rejected (403 Forbidden).

## Message Format

All messages are JSON text frames:

```json
{
  "type": "<message_type>",
  "request_id": "<optional>",
  "payload": { ... }
}
```

## Message Types

### Client → Server

#### `session.start`

Must be sent before any `location.batch`. Opens a tracker session and validates device ownership.

```json
{
  "type": "session.start",
  "request_id": "req_001",
  "payload": {
    "device_id": "b0d34c19-ef5e-4e35-bd30-1d6680245c10",
    "app_version": "1.0.0",
    "platform": "android"
  }
}
```

- `device_id`: app-generated UUID (not hardware ID). Must be registered via `POST /v1/devices/register`.
- Backend does not accept hardware identifiers (IMEI, serial, Android ID).

#### `location.batch`

Sends a batch of location events. Requires an accepted session.

```json
{
  "type": "location.batch",
  "request_id": "req_002",
  "payload": {
    "device_id": "b0d34c19-ef5e-4e35-bd30-1d6680245c10",
    "events": [
      {
        "event_id": "evt_001",
        "recorded_at": "2026-06-10T12:00:01Z",
        "lat": 55.751244,
        "lon": 37.618423,
        "accuracy_m": 8.5,
        "speed_mps": 1.4,
        "bearing_deg": 82,
        "activity_type": "walking",
        "activity_confidence": 0.87,
        "battery_level": 0.74
      }
    ]
  }
}
```

**Event field rules:**

| Field | Required | Constraints |
|-------|----------|-------------|
| `event_id` | yes | non-empty string |
| `recorded_at` | yes | RFC3339 |
| `lat` | yes | `[-90, 90]` |
| `lon` | yes | `[-180, 180]` |
| `accuracy_m` | yes | `>= 0` |
| `speed_mps` | no | `>= 0` if present |
| `bearing_deg` | no | `[0, 360]` if present |
| `activity_type` | no | enum: `walking`, `running`, `bicycle`, `vehicle`, `stationary`, `unknown` |
| `activity_confidence` | no | `[0, 1]` if present |
| `battery_level` | no | `[0, 1]` if present |

**Batch limits:**
- Max events per batch: 100 (usecase layer validation → `BATCH_TOO_LARGE`)
- Max message size: 256 KB (transport layer, `SetReadLimit` → connection closed)

#### `ping`

Application-level ping.

```json
{ "type": "ping", "request_id": "req_003" }
```

### Server → Client

#### `session.accepted`

```json
{
  "type": "session.accepted",
  "request_id": "req_001",
  "payload": {
    "session_id": "a4e8c1f2-...",
    "server_time": "2026-06-10T12:00:00Z",
    "config": {
      "max_batch_size": 100,
      "recommended_flush_interval_sec": 10
    }
  }
}
```

#### `location.batch.ack`

Sent **only after** successful JetStream PubAck for all accepted events.

```json
{
  "type": "location.batch.ack",
  "request_id": "req_002",
  "payload": {
    "accepted": ["evt_001"],
    "duplicated": ["evt_002"],
    "rejected": [
      { "event_id": "evt_003", "reason": "invalid_latitude" }
    ]
  }
}
```

#### `error`

```json
{
  "type": "error",
  "request_id": "req_001",
  "payload": {
    "code": "DEVICE_NOT_REGISTERED",
    "message": "device not registered for this user"
  }
}
```

#### `pong`

Response to application-level ping:

```json
{ "type": "pong", "request_id": "req_003" }
```

## Error Codes

| Code | Meaning |
|------|---------|
| `UNAUTHORIZED` | JWT missing or invalid (pre-upgrade) |
| `DEVICE_NOT_REGISTERED` | device_id not found for this user |
| `SESSION_NOT_ACCEPTED` | location.batch before session.start |
| `VALIDATION_ERROR` | invalid JSON or field validation failure |
| `BATCH_TOO_LARGE` | batch > 100 events |
| `MESSAGE_TOO_LARGE` | WebSocket frame > 256 KB (connection closed) |
| `EVENT_BUS_UNAVAILABLE` | NATS JetStream unavailable |
| `INTERNAL_ERROR` | unexpected server error |

## ACK Semantics

- **accepted**: event published to NATS, PubAck received, ledger updated. Client may mark as synced.
- **duplicated**: `(user_id, device_id, event_id)` already in dedup ledger. Not published again. Client may mark as synced.
- **rejected**: validation failure or per-event publish error. Client must not mark as synced.
- **No ACK / connection lost**: client keeps event as pending and retries.

## Dedup Semantics

Dedup scope: `(user_id, device_id, event_id)`. The same `event_id` from two different devices is not a duplicate.

Flow (variant B — publish-first):

1. Check ledger by `(user_id, device_id, event_id)`.
2. If found → `duplicated` response, no publish.
3. If not found → publish to NATS with `Nats-Msg-Id = event_id`.
4. On PubAck → `INSERT INTO ingested_location_events ON CONFLICT DO NOTHING`.
5. Conflict on insert is non-fatal: publish already confirmed, ACK stays successful.

JetStream `Nats-Msg-Id` provides additional best-effort dedup within the 2-minute dedup window for concurrent retries.

## Ordering Limitations

- Client should send events sorted by `recorded_at`.
- Gateway publishes in received batch order.
- NATS preserves order per publisher connection only.
- Future workers must sort by `recorded_at`.
- Future trip detection must use a late-arrival buffer window.
- EPIC 4 does not implement trip ordering logic.

## Connection Lifecycle

- Read deadline: reset on each pong (default 60s).
- Write deadline: 10s per write.
- Server ping interval: 30s.
- Graceful close: server sends `websocket.CloseGoingAway` on shutdown.
- `session.start` state is in-memory per connection; lost on disconnect.
