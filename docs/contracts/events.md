# Event Contracts

Source of truth in code: `libs/events`. This document describes the wire format.

## Envelope

Every message on the bus is a JSON envelope:

```json
{
  "event_id": "evt_123",
  "event_type": "location.events.v1",
  "event_version": 1,
  "occurred_at": "2026-06-10T12:00:01Z",
  "produced_at": "2026-06-10T12:00:04Z",
  "producer": "tracking-gateway",
  "correlation_id": "req_123",
  "payload": {}
}
```

| Field | Required | Notes |
|-------|----------|-------|
| event_id | yes | unique per event; used as `Nats-Msg-Id` for dedup |
| event_type | yes | equals the subject, version suffix included |
| event_version | yes | >= 1; bumped on breaking payload changes |
| occurred_at | yes | when the domain fact happened |
| produced_at | yes | when the event was published |
| producer | yes | producing service name |
| correlation_id | location: yes; derived events: optional | request tracing |
| payload | yes | event-specific body, see below |

## Subjects

| Subject | Producer | Description |
|---------|----------|-------------|
| `location.events.v1` | tracking-gateway | raw location points |
| `trip.started.v1` | tracking-worker | trip detected as started |
| `trip.updated.v1` | tracking-worker | active trip progress |
| `trip.completed.v1` | tracking-worker | trip finished |
| `route.matched.v1` | route-worker | trip matched to a known route |
| `dead-letter.v1` | any consumer | message processing failed permanently |

Stream `WRANY_EVENTS` is bound to wildcard filters: `location.events.*`,
`trip.*`, `route.*`, `dead-letter.*`.

## Message headers

| Header | Value | Purpose |
|--------|-------|---------|
| `Nats-Msg-Id` | envelope `event_id` | JetStream dedup (best-effort publisher retry protection, **not** business idempotency) |
| `Wrany-Event-Type` | envelope `event_type` | filtering/observability without parsing the body |
| `Wrany-Correlation-Id` | envelope `correlation_id` | request tracing |
| `Wrany-User-Id` | payload `user_id` | set by the location publishing path (future epic) |
| `Wrany-Device-Id` | payload `device_id` | set by the location publishing path (future epic) |

The logical ordering key is `user_id:device_id` (see
`location.Payload.OrderingKey`). It is metadata only — NATS preserves order per
publisher connection, consumers must sort by `recorded_at`.

## Payloads

### location.events.v1

```json
{
  "user_id": "user_123",
  "device_id": "device_123",
  "recorded_at": "2026-06-10T12:00:01Z",
  "received_at": "2026-06-10T12:00:04Z",
  "lat": 55.751244,
  "lon": 37.618423,
  "accuracy_m": 8.5,
  "speed_mps": 1.4,
  "bearing_deg": 82,
  "activity_type": "walking",
  "activity_confidence": 0.87,
  "battery_level": 0.74,
  "source": "android_tracker"
}
```

Validation: `user_id`, `device_id`, `recorded_at`, `received_at`, `source`
required; `lat` ∈ [-90, 90]; `lon` ∈ [-180, 180]; `accuracy_m` >= 0;
`activity_confidence`, `battery_level` ∈ [0, 1]. `correlation_id` is required
on the envelope.

### trip.started.v1

```json
{
  "trip_id": "trip_123",
  "user_id": "user_123",
  "device_id": "device_123",
  "started_at": "2026-06-10T12:00:00Z"
}
```

### trip.updated.v1

```json
{
  "trip_id": "trip_123",
  "user_id": "user_123",
  "device_id": "device_123",
  "updated_at": "2026-06-10T12:05:00Z",
  "distance_m": 1200,
  "duration_s": 300,
  "point_count": 60
}
```

### trip.completed.v1

```json
{
  "trip_id": "trip_123",
  "user_id": "user_123",
  "device_id": "device_123",
  "started_at": "2026-06-10T12:00:00Z",
  "completed_at": "2026-06-10T12:30:00Z",
  "distance_m": 5000,
  "duration_s": 1800,
  "point_count": 360
}
```

Trip payloads are minimal for the MVP; fields are extended in the trip
detection epic with a version bump on breaking changes.

### route.matched.v1

```json
{
  "trip_id": "trip_123",
  "route_id": "route_45",
  "user_id": "user_123",
  "matched_at": "2026-06-10T12:30:05Z",
  "match_score": 0.93
}
```

`match_score` ∈ [0, 1].

### dead-letter.v1

```json
{
  "original_subject": "location.events.v1",
  "original_event": { "event_id": "evt_123" },
  "error": "handler failed after max deliveries",
  "failed_at": "2026-06-10T12:00:10Z",
  "consumer": "tracking-worker-location-consumer"
}
```

`original_event` carries the raw original envelope.

## Versioning policy

- The version suffix in the subject (`.v1`) and `event_version` change together.
- Additive, backward-compatible fields do not require a version bump.
- Breaking changes (removed/renamed fields, semantic changes) require a new
  subject version; producers may dual-publish during migration.

## Durable consumer naming

`<service>-<domain>-consumer`:

```text
tracking-worker-location-consumer
route-worker-trip-consumer
analytics-route-consumer
```
