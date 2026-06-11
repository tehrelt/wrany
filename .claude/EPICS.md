# WR any% Epic Registry

This file is the source of truth for epic scope.

Claude must read this file before starting any epic.

---

# EPIC 01: Project Foundation

## Goal

Create the local development foundation: repository structure, Docker Compose, Postgres/PostGIS, Kafka, Go service stubs, Makefile, README.

## Scope

- Docker Compose
- Postgres/PostGIS
- Kafka in KRaft mode
- Go workspace
- tracking-gateway stub
- tracking-worker stub
- apps directory placeholders
- Makefile
- README

## Out of Scope

- Real tracking logic
- Auth
- WebSocket protocol
- Android implementation
- Web implementation

---

# EPIC 02: Auth & Device Registration

## Goal

Implement the foundation for user authentication and device registration.

The backend must know which user and device are connecting before allowing tracker WebSocket sessions in future epics.

## Scope

### Backend

- Add auth-related domain structure.
- Add user model.
- Add device model.
- Add database tables for users and devices.
- Add password-based registration/login for MVP.
- Add JWT access token generation.
- Add refresh token strategy or documented MVP simplification.
- Add middleware for authenticated HTTP endpoints.
- Add device registration endpoint.
- Add endpoint to list current user devices.
- Add basic validation and error responses.
- Add tests.

### Database

Required tables:

- users
- devices
- refresh_tokens or sessions, if refresh tokens are implemented

Minimal user fields:

- id
- email
- password_hash
- created_at
- updated_at

Minimal device fields:

- id
- user_id
- device_id
- platform
- app_version
- device_name
- created_at
- last_seen_at

### API

MVP endpoints:

- POST /v1/auth/register
- POST /v1/auth/login
- POST /v1/auth/refresh, if refresh tokens are implemented
- POST /v1/devices/register
- GET /v1/devices
- GET /v1/me

### Security

- Passwords must be hashed.
- Plain passwords must never be stored.
- JWT secret must come from environment variables.
- Auth errors must not leak whether an email exists.
- Device registration must require a valid access token.

## Out of Scope

- OAuth providers
- Keycloak
- Social login
- Email verification
- Password reset
- Role-based permissions
- WebSocket auth implementation itself
- Android token storage implementation

WebSocket auth will be implemented in EPIC 5/6.

## Dependencies

Requires EPIC 1 to be completed.

## Acceptance Criteria

- User can register.
- User can login.
- Backend returns JWT access token.
- Authenticated endpoint /v1/me works.
- Unauthenticated request to protected endpoint is rejected.
- Authenticated user can register a device.
- Registered device is linked to user.
- Device list endpoint returns only current user's devices.
- Passwords are stored as hashes.
- Tests cover register/login/device registration.
- README documents auth endpoints and env variables.

---

# EPIC 03: Event Bus Contracts & NATS JetStream Foundation

> Note: this epic replaces the former "EPIC 03: Android Background Location Tracking"
> (now rescheduled, see below) and supersedes the originally planned "Kafka Event Contracts" epic.
> The MVP event bus is NATS JetStream, not Kafka. Kafka/Redpanda remain possible future adapters.

## Goal

Create the backend event bus foundation: NATS JetStream dev infrastructure,
shared event contracts, and a broker-agnostic publishing abstraction.

## Scope

### Infrastructure

- Replace Kafka with NATS JetStream in docker-compose (JetStream enabled, file storage, persistent volume, healthcheck, internal network only).
- Update `.env.example`: `NATS_URL`, `NATS_STREAM`; remove Kafka vars.
- Makefile: `nats-check`, `nats-streams`, `nats-init` (via disposable nats-box container, no host CLI install).
- Stream `WRANY_EVENTS` with wildcard subjects: `location.events.*`, `trip.*`, `route.*`, `dead-letter.*`.

### Contracts (`libs/events`)

- Common event envelope (event_id, event_type, event_version, occurred_at, produced_at, producer, correlation_id, payload).
- Subject constants and durable consumer naming convention.
- Payloads: `location.events.v1`, `trip.started.v1`, `trip.updated.v1`, `trip.completed.v1`, `route.matched.v1`, `dead-letter.v1`.
- Validation for required fields and value ranges.

### Abstraction (`libs/eventbus`)

- `Publisher` interface (publish only; `EventBus` publish+consume is a documented future abstraction).
- Minimal NATS JetStream adapter: Publish (with `Nats-Msg-Id = event_id` dedup header), EnsureStream, Close.
- Consumer API: design only (durable name, ack/nack, redelivery, graceful shutdown). Implementation deferred to the first real consumer epic.

### Documentation

- Update current source-of-truth files: `.claude/CLAUDE.md`, `README.md`, service READMEs.
- New: `docs/architecture/event-bus.md`, `docs/contracts/events.md`.
- Honest ordering semantics: per-publisher-connection order only; workers sort by `recorded_at`; no Kafka-like partition ordering claims.
- Dedup documented as best-effort publisher retry protection, not global business idempotency.
- Historical EPIC files are not rewritten; mark with "Superseded by EPIC 03" notes where needed.

## Out of Scope

- WebSocket ingestion (`/v1/ws/tracker`) and session protocol
- Android background tracking and offline queue
- Trip detection state machine
- Route matching, analytics API, web client
- Real publishing from tracking-gateway / consuming in tracking-worker
- Kafka/Redpanda adapters

## Dependencies

Requires EPIC 1 and EPIC 2 to be completed.

## Acceptance Criteria

- `docker-compose.yml` runs NATS JetStream instead of Kafka; `make up` brings up postgres, tracking-gateway, tracking-worker, nats.
- `make nats-check` confirms JetStream is enabled.
- `.env.example` contains only actually used NATS variables.
- `libs/events` implemented: envelope, subjects, all six event contracts, validation.
- `libs/eventbus` implemented: `Publisher` interface, minimal NATS adapter setting `Nats-Msg-Id = event_id`.
- Unit tests for validation pass; `make test` passes.
- Repeated publish with the same `event_id` within the JetStream dedup window does not create a duplicate (integration test).
- README/docs updated; current Kafka references replaced or marked historical.

---

# EPIC 04: WebSocket Tracker Protocol & Gateway Ingestion

## Goal

Implement WebSocket endpoint in `tracking-gateway` that accepts location events from the Android tracker client, validates user/device/batch, and publishes accepted events to NATS JetStream `location.events.v1`.

## Scope

- `GET /v1/ws/tracker` with JWT auth before upgrade
- WebSocket protocol: `session.start`, `session.accepted`, `location.batch`, `location.batch.ack`, `error`, `ping`, `pong`
- Device ownership validation (uses EPIC 2 DeviceRepo)
- Per-event batch validation (lat/lon, accuracy, activity_type, optional fields)
- Dedup ledger: `ingested_location_events` with PK `(user_id, device_id, event_id)`
- Dedup strategy: publish-first (variant B), ON CONFLICT DO NOTHING after PubAck
- NATS JetStream publish via `libs/eventbus.Publisher`
- ACK to client only after successful JetStream PubAck
- Origin policy: empty origin allowed (mobile), browser origins via allowlist
- Migration `0004_create_ingested_location_events`

## Out of Scope

- Android background tracking
- Android SQLite queue / SyncManager
- tracking-worker NATS consumer
- TripDetectionEngine
- Raw GPS points persistence
- Route matching, analytics API, web client
- Loop detection, manual correction

## Dependencies

Requires EPIC 1, EPIC 2, EPIC 3.

## Acceptance Criteria

- `GET /v1/ws/tracker` exists
- Without JWT → 401 before upgrade
- `session.start` required before `location.batch`
- Device validated against authenticated user
- Unknown device → `DEVICE_NOT_REGISTERED`
- Batch validation works (lat/lon, accuracy, activity_type, optionals)
- Batch size limit 100 events (usecase layer)
- Message size limit 256 KB (transport layer)
- Accepted events published to `location.events.v1`
- ACK only after PubAck
- NATS unavailable → `EVENT_BUS_UNAVAILABLE`
- Duplicate `(user_id, device_id, event_id)` → `duplicated`, not republished
- Conflict on ledger insert after PubAck is non-fatal
- `make test` passes

---

# RESCHEDULED: Android Background Location Tracking

> Formerly EPIC 03. Rescheduled by EPIC 03 (Event Bus Contracts & NATS JetStream Foundation).
> A new epic number will be assigned later with explicit user confirmation.
> Content preserved unchanged below.

## Goal

Implement Android background location collection foundation in React Native.

## Scope

- Background location permission flow
- Foreground service strategy
- Activity recognition placeholder
- Local GPS event creation
- Battery-aware configuration

## Out of Scope

- WebSocket sync
- Offline event queue
- Backend trip detection

Depends on EPIC 2 only if authenticated config is needed.

---

# EPIC 06: Android Tracker MVP — Foreground Manual Tracking

## Goal

Build a minimal React Native Android application for manual foreground testing of the complete backend pipeline with real GPS data from a phone.

Also establishes the OpenAPI generation pipeline: backend annotations → generated specs → Swagger UI container → TypeScript client codegen.

## Scope

### Backend
- `swaggo/swag` annotations on all tracking-gateway REST handlers
- `make openapi-generate` — generates `docs/openapi/generated/tracking-gateway.json`
- `make openapi-merge` — merges into `docs/openapi/generated/combined.json`
- `make openapi-check` — verifies spec generation succeeds
- `make swagger-up` — starts Swagger UI container at `http://localhost:8088`
- `swagger-ui` Docker service (profile: tools)
- `WSAuthMiddleware` — backward-compatible `?access_token=` fallback for WebSocket auth

### Android app (`apps/android-tracker`)
- React Native CLI + TypeScript
- Auth screen: register / login
- Device UUID generation and persistence
- Device registration
- WebSocket tracker client with `session.start` / `location.batch` / `ack` handling
- Foreground GPS location watcher
- In-memory batch queue (flush on size ≥ 10 or 10s interval)
- Debug UI with live counters: pending / accepted / duplicated / rejected
- `Send Test Point` button (synthetic event without real GPS)
- TypeScript API client from generated OpenAPI spec

## Out of Scope
- Background tracking, foreground service
- SQLite offline queue
- Activity recognition
- Route / trip detection
- Map UI, analytics, web client

## Dependencies
- EPIC 1, 2, 3, 4, 5

## Acceptance Criteria
- `make openapi-generate` succeeds; `docs/openapi/generated/tracking-gateway.json` created
- `make openapi-merge` succeeds; `combined.json` created
- Swagger UI accessible at `http://localhost:8088`
- Android app builds and connects to backend
- Full manual E2E flow works: register → login → device registration → WS connect → session.start → location.batch → ack → DB row verified
