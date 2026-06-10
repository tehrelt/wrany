# Event Bus Architecture

## Why NATS JetStream

WR any% is self-hosted; the target hardware can be as small as an Orange Pi 3B
with 4 GB RAM. Kafka is too heavy for that environment. NATS JetStream provides
everything the MVP needs at a fraction of the footprint:

- persistence (file storage)
- replay
- ack / redelivery
- durable consumers
- at-least-once delivery

Kafka and Redpanda remain possible **future adapters**. Code must never depend
on a concrete broker: services use the `Publisher` abstraction from
`libs/eventbus` and the contracts from `libs/events`. The only package that
imports NATS types is `libs/eventbus/nats` (plus future service-specific
`internal/transport/nats` adapters).

## Data flow

```text
Android Tracker
  ↓ WebSocket
tracking-gateway
  ↓ publish (Publisher → NATS adapter)
NATS JetStream (stream WRANY_EVENTS)
  ↓ consume (durable consumers)
tracking-worker
  ↓ publish
NATS JetStream
  ↓ consume
route-worker / analytics services (later)
```

The gateway ACKs a client message only after JetStream confirms the publish
(`PubAck`) — the event is durably stored before the client hears "accepted".

## Exposure

NATS is the internal event bus and must never be exposed to external clients.

- In Docker Compose, NATS lives on the internal `wrany-net` network.
- Port 4222 is bound to `127.0.0.1` **for local development only** (integration
  tests, `nats` CLI). Production-style deployments must not publish the port.
- The monitoring port 8222 is used by the container healthcheck and is not
  published to the host.

## Stream layout

One stream holds all domain events:

| Setting   | Value |
|-----------|-------|
| Name      | `WRANY_EVENTS` |
| Subjects  | `location.events.*`, `trip.*`, `route.*`, `dead-letter.*` |
| Storage   | file |
| Retention | limits |
| Dedup window | 2m |

Concrete subjects are versioned: `location.events.v1`, `trip.started.v1`,
`trip.updated.v1`, `trip.completed.v1`, `route.matched.v1`, `dead-letter.v1`.
See `docs/contracts/events.md`.

The stream is provisioned idempotently:

- `make nats-init` (one-shot nats-box container), or
- `EnsureStream` in `libs/eventbus/nats`, called by services on startup
  (dev/MVP mechanism; production provisioning is hardened in a later epic).

## Ordering — honest guarantees

NATS subjects are **not** Kafka partitions. There is no per-key ordering:

- message order is preserved **per publisher connection** only;
- the logical ordering key `user_id:device_id` is carried in headers as
  metadata for future sharding/filtering — it is *not* an ordering mechanism;
- consumers must sort/process location points by `recorded_at`;
- trip detection (future epic) must tolerate late and out-of-order points with
  a small buffer window.

## Deduplication

Publishers set the `Nats-Msg-Id` header to the envelope `event_id`. JetStream
drops duplicates within the stream's dedup window (2 minutes).

This is **best-effort publisher retry protection** — it covers re-publishing
after a timeout or reconnect. It is **not** global business idempotency:
duplicates outside the window are possible, and consumers/storage must handle
business-level idempotency by `event_id` (future epics).

## Dead letters

When a consumer exhausts redelivery attempts (`MaxDeliver`), it publishes a
`dead-letter.v1` event describing the failure (original subject, raw envelope,
error, consumer name). An explicit dead-letter subject was chosen over NATS
advisories because it is simpler and portable to other brokers.

## Consumer model (design, implemented in a later epic)

Durable consumer naming convention: `<service>-<domain>-consumer`, e.g.

```text
tracking-worker-location-consumer
route-worker-trip-consumer
analytics-route-consumer
```

The consumer abstraction must support: durable name, explicit ack,
nack/redelivery (`AckWait`, `MaxDeliver`), context cancellation, and graceful
shutdown (drain). The combined `EventBus` (Publisher + Consumer) abstraction
will be introduced together with the first real consumer.
