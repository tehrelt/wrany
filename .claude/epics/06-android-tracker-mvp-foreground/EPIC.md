# EPIC 06: Android Tracker MVP — Foreground Manual Tracking

## Status

Planned

---

## Goal

Build a minimal React Native Android application for manual foreground testing of the complete backend pipeline with real GPS data from a phone.

Not a production background tracker. Foreground only.

Additionally: establish automatic OpenAPI generation pipeline from backend source annotations, Swagger UI dev container, and TypeScript client codegen — so Android REST layer uses generated types, not hand-written DTOs.

---

## Context

Backend pipeline after EPIC 5:

```
Android Tracker
  ↓ WebSocket
tracking-gateway  (EPIC 4)
  ↓ NATS location.events.v1
tracking-worker   (EPIC 5)
  ↓
Postgres/PostGIS raw_location_points
```

Android client does not exist yet. EPIC 6 creates the missing first hop.

Dependencies:
- EPIC 1: project foundation
- EPIC 2: auth & device registration endpoints
- EPIC 3: NATS JetStream
- EPIC 4: WebSocket tracker protocol (`/v1/ws/tracker`)
- EPIC 5: tracking-worker consumer + raw_location_points

---

## Problem Analysis

### Backend is complete but untested end-to-end

The full pipeline (gateway → NATS → worker → Postgres) was implemented and unit-tested in epics 4–5, but never exercised with a real GPS device. Manual E2E testing requires a real client.

### OpenAPI gap

Backend HTTP endpoints exist but have no machine-readable contract. Hand-writing OpenAPI specs is error-prone and diverges from code. The correct approach: generate specs from source annotations and treat generated output as the single source of truth for client codegen.

### Testability requirements

To verify the pipeline we need a client that can:
1. Authenticate (EPIC 2 JWT)
2. Register a device (EPIC 2 device registration)
3. Open a WebSocket with `Authorization: Bearer <token>` header
4. Send `session.start` and receive `session.accepted`
5. Collect real GPS coordinates (or send synthetic ones)
6. Batch and send `location.batch` messages
7. Handle `location.batch.ack` and update counters

### Protocol constraints (must not break)

The WebSocket protocol is fully specified in EPIC 4. The client must conform exactly:
- `session.start` before any `location.batch`
- Batch size ≤ 100 events
- Message size ≤ 256 KB
- Required fields: `event_id`, `recorded_at`, `lat`, `lon`, `accuracy_m`, `activity_type`

### Future compatibility

EPIC (TBD) will add background tracking. Architecture choices in EPIC 6 must not block that.

---

## Best Practice Research

### React Native CLI vs Expo

**Chosen: React Native CLI**

Expo managed workflow restricts background location, foreground services, and custom native modules. Since EPIC (TBD) needs background location + foreground service, starting with React Native CLI avoids a painful Expo eject later.

### WebSocket auth on Android

React Native's built-in `WebSocket` does not support custom headers on the HTTP upgrade request on Android (known platform limitation). The gateway currently expects `Authorization: Bearer <token>` header.

**Chosen: `?token=<jwt>` query parameter.** Requires a 2-line backward-compatible change to `tracking-gateway` ws_handler (still accepts header for future server-to-server use). Simpler than a custom WS library, no protocol changes needed.

**STOP POINT:** Requires user confirmation before backend change is implemented.

### OpenAPI generation tool for Go (standard net/http)

Tracking-gateway uses **standard `net/http` + `http.ServeMux`** — no gin/echo/chi framework.

Evaluated options:

| Tool | Approach | Works with net/http | Output |
|------|---------|-------------------|--------|
| `swaggo/swag` | Comment annotations | Yes | OpenAPI 2.0 / 3.0 |
| `go-swagger` | Comment annotations | Yes | Swagger 2.0 |
| `huma` | Type-driven framework | No (rewrite needed) | OpenAPI 3.1 |
| `oapi-codegen` | Spec-first (generates Go) | N/A — wrong direction | — |

**Chosen: `swaggo/swag`** (`github.com/swaggo/swag`)

Reasons:
- Works with standard `net/http` via comment annotations on handler methods
- No framework migration required
- Generates OpenAPI 3.0 (with `--v3` flag) or Swagger 2.0
- CLI: `swag init -g cmd/tracking-gateway/main.go --output docs/swagger`
- Actively maintained, widely used in Go ecosystem
- Example annotation fits current handler style naturally:

```go
// Register godoc
// @Summary      Register a new user
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body  RegisterRequest  true  "Registration data"
// @Success      201   {object}  TokenPair
// @Failure      409   {object}  ErrorResponse
// @Router       /v1/auth/register [post]
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
```

**Output format:** Swagger 2.0 JSON (default `swag init`). Mergeable, well-supported by Swagger UI.

If OpenAPI 3.0 is needed later, use `swag fmt --v3` or a conversion step.

### OpenAPI merge tool

Evaluated options:

| Tool | Language | Path conflict handling |
|------|---------|----------------------|
| `openapi-merge-cli` | Node.js | Configurable per-operationId prefix |
| `swagger-cli bundle` | Node.js | Bundles $refs, no path dedup |
| `redocly merge` | Node.js | Explicit namespace required |

**Chosen: `openapi-merge-cli`**

Reasons:
- Explicitly designed for merging multiple specs into one
- Supports `operationIdPrefix` per service — prevents collisions as more services are added
- Config-driven (`openapi-merge.json`) — easy to add new services later
- Path conflict → fails fast with clear error (does not silently overwrite)
- Widely used, actively maintained

### TypeScript client codegen

**Chosen: `openapi-typescript-codegen`** (`npm install -D openapi-typescript-codegen`)

Reasons:
- Generates TypeScript types + fetch-based client from Swagger 2.0 / OpenAPI 3.0
- Zero runtime dependencies in generated code
- Works offline (no cloud service)
- Simple CLI: `openapi --input combined.json --output src/api/generated --client fetch`

Alternative `@openapitools/openapi-generator-cli` is heavier (Java dependency) — avoid for React Native.

### Swagger UI Docker container

**Image: `swaggerapi/swagger-ui`**

Supports `URLS` env var for multi-spec display. Serves specs from mounted volume.

---

## Solution Design

### OpenAPI pipeline

```
Go backend source (annotations)
  ↓ swag init
docs/openapi/generated/<service>.json        (per-service Swagger 2.0)
  ↓ openapi-merge-cli
docs/openapi/generated/combined.json         (merged)
  ↓ swagger-ui container (port 8088)
  ↓ openapi-typescript-codegen
apps/android-tracker/src/api/generated/      (TypeScript client)
```

**Rules:**
- `docs/openapi/generated/` is gitignored — never committed, always regenerated
- Manual edits to generated files are forbidden
- All API contract changes go through backend annotations → regenerate
- WebSocket protocol stays out of OpenAPI (separate `docs/contracts/websocket-tracker-protocol.md`)

### Endpoints annotated in tracking-gateway

All current real routes (no invented endpoints):

```
GET  /healthz
POST /v1/auth/register
POST /v1/auth/login
POST /v1/auth/refresh
GET  /v1/me
POST /v1/devices/register
GET  /v1/devices
```

`GET /v1/ws/tracker` — NOT described in OpenAPI (WebSocket, see docs/contracts/).

### Makefile commands

```makefile
openapi-generate    # swag init for each service → docs/openapi/generated/
openapi-merge       # openapi-merge-cli → docs/openapi/generated/combined.json
openapi-check       # regenerate + diff — fails CI if stale
swagger-up          # docker compose up swagger-ui (detached)
```

### Docker Compose service

```yaml
swagger-ui:
  image: swaggerapi/swagger-ui
  ports:
    - "8088:8080"
  volumes:
    - ./docs/openapi/generated:/usr/share/nginx/html/openapi:ro
  environment:
    URLS: >
      [{"name":"combined","url":"/openapi/combined.json"},
       {"name":"tracking-gateway","url":"/openapi/tracking-gateway.json"}]
  profiles:
    - tools
```

Using `profiles: [tools]` — not started by `make up`, only by `make swagger-up`.

### Android app flow

```
Launch
  → check stored token
  → if valid: go to TrackerScreen
  → if not: go to AuthScreen

AuthScreen
  → Register / Login  (uses generated API client)
  → on success: save token, navigate to TrackerScreen

TrackerScreen (debug UI)
  → Register Device   (uses generated API client)
  → Connect WS
  → Start Tracking
  → Stop Tracking
  → Send Test Point
  → Live counters: pending / accepted / duplicated / rejected
```

**Android REST layer uses generated TypeScript client from `combined.json`. No hand-written DTOs.**

WebSocket layer (`trackerSocket.ts`) is hand-written (protocol not in OpenAPI).

### Batch queue (in-memory)

```
pendingEvents: LocationEvent[]

flush when:
  - pendingEvents.length >= 10
  - OR 10 seconds elapsed since last flush
  - OR Stop Tracking pressed

on ack:
  - accepted   → remove from pending
  - duplicated → remove from pending
  - rejected   → remove from pending (log)
  - no ack / disconnect → keep pending (retry on reconnect — future EPIC)
```

Max batch size: 50 (well under backend limit of 100).

---

## Architecture Notes

### OpenAPI file layout

```
docs/
  openapi/
    generated/           ← gitignored, always regenerated
      tracking-gateway.json
      combined.json
    merge/
      openapi-merge.json ← merge config (committed)
  contracts/
    websocket-tracker-protocol.md
```

### Android app layout

```
apps/android-tracker/
  android/                  ← RN native Android project
  src/
    app/
      App.tsx
      navigation.ts
    screens/
      AuthScreen.tsx
      TrackerScreen.tsx
    api/
      generated/            ← gitignored, from openapi-typescript-codegen
      httpClient.ts         ← base URL config passed to generated client
    tracker/
      deviceId.ts
      locationService.ts
      trackerSocket.ts
      batchQueue.ts
      types.ts              ← WS protocol types only (not REST DTOs)
    storage/
      tokenStorage.ts
    config/
      env.ts
  package.json
  tsconfig.json
  README.md
```

### Dependency direction

```
screens → api/generated, tracker, storage, config
tracker/trackerSocket → tracker/batchQueue, tracker/types
api/generated → config (base URL)
```

---

## Tasks

### T01 — Update EPICS.md with EPIC 6 scope

### T02 — Backend: add `?token=` query param to WebSocket endpoint
- Small backward-compatible change to `internal/transport/http/websocket_handler.go`
- Still accepts `Authorization` header (existing behavior)
- Add test
- **Requires user confirmation before implementation**

### T03 — Add swaggo/swag annotations to tracking-gateway
- Install `swag` CLI
- Add `@General` info block in `cmd/tracking-gateway/main.go`
- Add per-endpoint annotations to all REST handlers
- Define request/response structs for `swag` to pick up types

### T04 — Add `make openapi-generate`
- Run `swag init` for tracking-gateway
- Output: `docs/openapi/generated/tracking-gateway.json`
- Add `docs/openapi/generated/` to `.gitignore`

### T05 — Add `openapi-merge.json` config and `make openapi-merge`
- Install `openapi-merge-cli`
- Config: `docs/openapi/merge/openapi-merge.json`
- Output: `docs/openapi/generated/combined.json`
- Path conflict detection: fail on collision

### T06 — Add `make openapi-check`
- Regenerate + diff against committed baseline (or validate format)
- Usable in CI to catch stale specs

### T07 — Add `swagger-ui` service to `docker-compose.yml`
- Profile: `tools`
- Port: `8088`
- Mount `docs/openapi/generated`
- `URLS` env with combined + tracking-gateway

### T08 — Add `make swagger-up`
- `docker compose --profile tools up swagger-ui -d`

### T09 — TypeScript client codegen setup
- Install `openapi-typescript-codegen` as devDependency in `apps/android-tracker`
- Add `generate:api` script in `package.json`
- Output: `src/api/generated/`
- Add `src/api/generated/` to `.gitignore`

### T10 — Init React Native CLI project in `apps/android-tracker`
- TypeScript template
- Verify builds for Android

### T11 — Configure env / base URLs
- `src/config/env.ts`
- Constants for emulator (`10.0.2.2:8080`) and LAN IP
- Document in README

### T12 — Implement token storage
- `src/storage/tokenStorage.ts`
- `saveToken / getToken / clearToken`

### T13 — Implement device UUID
- `src/tracker/deviceId.ts`
- Generate v4 UUID on first call, persist in AsyncStorage

### T14 — Implement WS protocol types
- `src/tracker/types.ts`
- All WS message interfaces: `SessionStart`, `SessionAccepted`, `LocationBatch`, `LocationBatchAck`, `ErrorMessage`, `Ping`, `Pong`
- `LocationEvent` with required/optional fields

### T15 — Implement batch queue
- `src/tracker/batchQueue.ts`
- In-memory queue + flush logic
- Unit-testable pure functions

### T16 — Implement WebSocket tracker client
- `src/tracker/trackerSocket.ts`
- Connect with `?token=<jwt>`
- `session.start` → wait `session.accepted` → enable batch sends
- Handle `location.batch.ack`

### T17 — Implement foreground location service
- `src/tracker/locationService.ts`
- Request `ACCESS_FINE_LOCATION` permission
- Start/stop watcher, push to batchQueue

### T18 — Wire httpClient to generated API client
- `src/api/httpClient.ts`
- Configure base URL from `env.ts` into generated client

### T19 — Implement AuthScreen
- Uses generated auth API types/client
- email/password, Register/Login buttons, error display

### T20 — Implement TrackerScreen (debug UI)
- All status fields, live counters, all buttons
- `Send Test Point` — synthetic event without GPS movement

### T21 — Add unit tests
- batch queue: flush, ack handling
- protocol message builders
- deviceId persistence

### T22 — Write `apps/android-tracker/README.md`
- Backend setup, Android run steps, emulator URL, LAN IP, permissions, test flow, known limits

### T23 — Update root README

### T24 — Fill Implementation Log and Final Report

---

## Acceptance Criteria

### OpenAPI pipeline
- [ ] `swaggo/swag` annotations on all REST handlers in tracking-gateway
- [ ] `docs/openapi/generated/tracking-gateway.json` generated automatically via `make openapi-generate`
- [ ] Generated spec is never hand-edited
- [ ] `docs/openapi/generated/combined.json` created via `make openapi-merge`
- [ ] Path conflicts in merge fail explicitly
- [ ] `make openapi-check` detects stale specs
- [ ] `swagger-ui` container starts via `make swagger-up`, accessible at `http://localhost:8088`
- [ ] Swagger UI shows tracking-gateway and combined specs
- [ ] TypeScript client generated from `combined.json` via `npm run generate:api`
- [ ] `src/api/generated/` is gitignored

### Android app
- [ ] `apps/android-tracker` exists and builds for Android
- [ ] App REST layer uses generated TypeScript types — no hand-written DTOs
- [ ] User can register/login against backend
- [ ] App generates and persists app-level device UUID
- [ ] App registers device with backend
- [ ] App connects to `/v1/ws/tracker` with `?token=<jwt>`
- [ ] App sends `session.start`
- [ ] App receives `session.accepted`
- [ ] App sends `location.batch`
- [ ] App receives `location.batch.ack`
- [ ] Accepted/duplicated/rejected counters update in UI
- [ ] App requests and uses foreground location permission
- [ ] App collects foreground GPS points and sends them in batches
- [ ] Backend stores points in `raw_location_points`
- [ ] `Send Test Point` works without real GPS movement
- [ ] No background tracking implemented
- [ ] No TripDetectionEngine implemented
- [ ] EPIC 7 not started

---

## Test Plan

### Manual E2E

1. `make up`
2. `make migrate-up && make migrate-worker-up`
3. `make nats-init`
4. `make openapi-generate && make openapi-merge`
5. `make swagger-up` → open `http://localhost:8088`, verify specs load
6. `npm run generate:api` in `apps/android-tracker`
7. Start Android app on emulator
8. Register user → confirm 201 response
9. Login → confirm token received
10. Register device → confirm 200
11. Connect WS → confirm `session.accepted` in UI
12. Tap `Send Test Point` → confirm `accepted` counter increments
13. Query DB: `SELECT count(*) FROM raw_location_points` → expect ≥ 1
14. Tap `Send Test Point` again with same event_id → confirm `duplicated`
15. Tap `Start Tracking` → grant permission → observe GPS points flowing
16. Query DB again → confirm rows accumulating
17. Tap `Stop Tracking` → WS closes cleanly

### Unit tests

- batch queue removes accepted/duplicated events on ack
- batch queue keeps pending events when no ack
- protocol builders produce correct JSON shape
- deviceId is stable across multiple `getOrCreateDeviceId()` calls

---

## Documentation Plan

- `apps/android-tracker/README.md` — full setup + testing guide
- `docs/openapi/merge/openapi-merge.json` — merge config
- Root `README.md` — add Android + OpenAPI sections
- `.claude/EPICS.md` — add EPIC 06 entry
- `docs/contracts/websocket-tracker-protocol.md` — WS protocol reference (can reference existing EPIC 4 docs)

---

## Implementation Log

_(empty — not started)_

---

## Final Report

_(empty — not started)_
