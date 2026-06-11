# EPIC 07: Web Client MVP — Tracking Data Viewer

## Status

In Progress

---

## Goal

Create a web client that lets an authenticated user view their raw GPS tracking data:
select a device, pick a time range, see points on a map and in a table, and read basic
stats. This is an MVP debug/viewer dashboard — no trip detection, no route analytics.

---

## Context

Backend pipeline after EPIC 6:

```
Android Tracker
  ↓ WebSocket
tracking-gateway  (EPIC 4)
  ↓ NATS location.events.v1
tracking-worker   (EPIC 5)
  ↓
Postgres/PostGIS raw_location_points
```

EPIC 6 established the OpenAPI generation pipeline:
- swaggo/swag annotations on tracking-gateway REST handlers
- `make openapi-generate` → `docs/openapi/generated/tracking-gateway.json`
- `make openapi-merge` → `docs/openapi/generated/combined.json`
- Swagger UI at http://localhost:8088

EPIC 7 adds the web client and the read API it needs.

Dependencies: EPIC 1, 2, 3, 4, 5, 6.

---

## Problem Analysis

### No read API exists

`raw_location_points` is written by tracking-worker but there are no REST endpoints
to query them. The web client needs two: paginated point list and period summary.

### Where to add the read API

`tracking-gateway` already handles JWT auth and is the HTTP entry point. Adding
read endpoints here avoids creating a new service for MVP. Both `tracking-gateway`
and `tracking-worker` connect to the same Postgres instance, so the gateway can
query `raw_location_points` read-only without migrations.

### User isolation

`raw_location_points` has a `user_id` column. Every query must filter by `user_id`
from the JWT — the API must not accept a `user_id` query param.

### Pagination

GPS sessions can produce thousands of points. Cursor-based pagination
(encoded `recorded_at` + `event_id`) avoids the offset-scan problem on large tables.

### TypeScript client

EPIC 6 established codegen but only for the Android app. EPIC 7 must wire the same
pipeline for `apps/web` — the web app must never hand-write REST DTOs.

---

## Best Practice Research

### React/Vite stack

Vite is the standard fast-build tool for React + TypeScript. No need for CRA or Next.js
for a pure client-side dashboard.

### UI kit — shadcn/ui + Tailwind CSS

`shadcn/ui` generates unstyled, fully-owned components into the project rather than
depending on a versioned library. Combined with Tailwind CSS it produces a clean,
accessible dashboard with no bundle overhead from components not used. Chosen over
heavyweight enterprise kits (MUI, Ant Design) because:
- no runtime style injection;
- components live in repo, fully customisable;
- Tailwind utility classes scale well for dashboards.

Complement with `lucide-react` for icons (tree-shakeable, used by shadcn/ui internally)
and `sonner` for toast/error notifications (shadcn/ui's recommended toaster).

### Map library — MapLibre GL + react-map-gl

`MapLibre GL JS` is the open-source fork of Mapbox GL JS, with no API key requirement.
`react-map-gl` wraps MapLibre for React.

Advantages over react-leaflet for this project:
- GPU-accelerated WebGL rendering — handles thousands of GeoJSON points without DOM overhead;
- native GeoJSON source/layer model: LineString for track, circle layer for raw points,
  separate highlight layer for selected point;
- extensible: clustering, heatmap, animated replay can be added later without replacing
  the map library;
- better visual output (vector/raster styles, smooth zoom).

Basemap tile provider is configurable via `VITE_MAP_STYLE_URL` env var.
Dev fallback: OSM raster tiles via MapLibre's `raster-tiles` source.
Attribution `© OpenStreetMap contributors` always rendered.

### Data fetching

`@tanstack/react-query` provides built-in loading/error/cache states, avoiding
manual `useEffect` + `useState` boilerplate. Best practice for REST-heavy dashboards.

### TypeScript codegen

`openapi-typescript` generates schema types (`.d.ts`) from OpenAPI spec — lighter
than `openapi-typescript-codegen` (which generates full client classes). For MVP
either works; we use the same approach as Android (`openapi-typescript-codegen`)
for consistency.

### Token storage

`localStorage` for MVP. Secure cookie / HttpOnly approach deferred to hardening epic.

---

## Solution Design

### Backend: two new read endpoints

```
GET /v1/tracking/points
  Query: device_id?, from (ISO8601, required), to (ISO8601, required),
         limit? (default 1000, max 5000), cursor?
  Auth: Bearer JWT (existing middleware)
  Response: { items: TrackingPoint[], next_cursor: string | null }

GET /v1/tracking/summary
  Query: device_id?, from (required), to (required)
  Auth: Bearer JWT
  Response: TrackingSummary
```

### Backend Clean Architecture additions

```
services/tracking-gateway/
  internal/
    domain/
      tracking_point.go      — TrackingPoint, TrackingPointFilter, TrackingSummary
    usecase/
      tracking_query.go      — TrackingQueryRepo interface + TrackingQueryUsecase
    storage/
      postgres/
        tracking_query_repo.go — queries raw_location_points
    transport/
      http/
        tracking_query_handler.go — handlers with swaggo annotations
```

Router adds:
```
GET /v1/tracking/points   → authMiddleware → trackingQueryHandler.GetPoints
GET /v1/tracking/summary  → authMiddleware → trackingQueryHandler.GetSummary
```

### Web app structure

```
apps/web/
  src/
    app/
      router.tsx          — createBrowserRouter (react-router-dom v6)
      providers.tsx       — QueryClientProvider + Toaster
    pages/
      LoginPage.tsx
      DashboardPage.tsx
    features/
      auth/
        authApi.ts        — calls generated client
        useAuth.ts        — token state, login/logout, auth:logout event
        AuthGuard.tsx     — redirect if not authenticated
      devices/
        devicesApi.ts     — calls generated client
        DeviceSelector.tsx
      tracking/
        trackingApi.ts    — calls generated client
        TrackingFilters.tsx  — device select + datetime inputs + refresh button
    components/
      ui/                 — shadcn/ui generated components (Button, Card, …)
      layout/
        AppLayout.tsx     — topbar (app name, user, logout) + main area
      map/
        TrackingMap.tsx   — react-map-gl + MapLibre, GeoJSON source/layers
        trackingGeoJson.ts — helpers: pointsToGeoJSON (LineString + Points)
      tracking/
        PointsTable.tsx   — shadcn/ui Table, columns: time/lat/lon/acc/speed/activity
        SummaryCards.tsx  — 6 shadcn/ui Cards (count/duration/avg_speed/max_speed/first_at/last_at)
    api/
      client.ts           — axios base instance, JWT request interceptor,
                            401 → single-flight refresh → retry, auth:logout on failure
      generated/          — codegen output (gitignored)
    config/
      env.ts              — VITE_API_BASE_URL, VITE_MAP_STYLE_URL
  .env.example
  vite.config.ts
  tsconfig.json
  index.html
  package.json
  README.md
  Dockerfile.dev
```

### Dashboard layout

```
┌─────────────────────────────────────────────────┐
│ TopBar: "WR any%" | user email | Logout button  │
├───────────────┬─────────────────────────────────┤
│ Filter panel  │ Summary cards (6 x Card)        │
│ ─ Device      ├─────────────────────────────────┤
│ ─ From date   │ Map (MapLibre GL, full width)   │
│ ─ To date     │                                 │
│ ─ Refresh btn ├─────────────────────────────────┤
│               │ Points Table (shadcn/ui Table)  │
└───────────────┴─────────────────────────────────┘
```

### Map rendering

```
GeoJSON source "tracking":
  - LineString feature — track polyline
  - Point features — individual GPS points (properties: recorded_at, speed_mps, …)

Layers:
  - "track-line"   — line layer, color #3b82f6, width 2
  - "points-circle" — circle layer, radius 4, color #2563eb
  - "point-selected" — circle layer, radius 8, color #dc2626, filtered by selected_id

Behavior:
  - fitBounds to all points on load/data change
  - click on circle → set selectedPointId → Popup or Sheet with point details
  - empty state: centered message if no points
  - loading: Skeleton over map area
  - error: Alert below filters
```

---

## Architecture Notes

- `user_id` always comes from JWT context in handlers — never from query params.
- Cursor encodes `(recorded_at::text, event_id)` joined as `<recorded_at>|<event_id>`,
  base64-encoded. Decoded in storage layer, used as `WHERE (recorded_at, event_id) > ($1, $2)`.
- Limit is enforced in usecase layer: default 1000, max 5000.
- `from`/`to` validation: both required, `from < to`, max range 31 days (guard for MVP).
- tracking-gateway does not write to `raw_location_points` — read-only queries only.
- No NATS interaction in the new read path.
- Web app depends on generated client → must regenerate after backend spec changes.
- Map tile provider is NOT hardcoded. `VITE_MAP_STYLE_URL` controls the basemap.
  Dev fallback uses OSM raster tiles; `© OpenStreetMap contributors` attribution always visible.
  Production should use a dedicated tile provider or self-hosted tiles.
- Map renders GeoJSON layers (not DOM markers) — scalable to thousands of points.
- shadcn/ui components are generated into `src/components/ui/` and owned by the repo.
  Do not update them via `shadcn` CLI without reviewing the diff.
- `sonner` Toaster is mounted in `providers.tsx`; use `toast.error()` for API errors.

---

## Tasks

### T01 — Update `.claude/EPICS.md` with EPIC 7 scope
Status: Done (committed with EPIC.md)

### T02 — Backend: domain types
- `services/tracking-gateway/internal/domain/tracking_point.go`
- `TrackingPoint`, `TrackingPointFilter`, `TrackingSummary`

### T03 — Backend: usecase
- `services/tracking-gateway/internal/usecase/tracking_query.go`
- `TrackingQueryRepo` interface
- `TrackingQueryUsecase` with `GetPoints` and `GetSummary`
- Limit cap + from/to validation

### T04 — Backend: storage
- `services/tracking-gateway/internal/storage/postgres/tracking_query_repo.go`
- Query `raw_location_points` with mandatory `user_id` filter
- Cursor-based pagination
- Optional `device_id`, `from`, `to` filters

### T05 — Backend: HTTP handler
- `services/tracking-gateway/internal/transport/http/tracking_query_handler.go`
- `GET /v1/tracking/points` handler
- `GET /v1/tracking/summary` handler
- Full swaggo annotations

### T06 — Wire into router and app
- `internal/transport/http/router.go` — add two routes
- `internal/app/app.go` — init repo, usecase, handler

### T07 — Backend tests
- Unauthenticated → 401
- User isolation (own data only)
- device_id filter
- from/to filter
- limit cap
- cursor pagination
- summary correctness
- empty range → empty result

### T08 — Regenerate OpenAPI spec
- `make openapi-generate` → updated tracking-gateway.json
- `make openapi-merge` → updated combined.json
- Verify new endpoints appear in Swagger UI

### T09 — Add `make ts-client` and regenerate TS client
- Check if target exists in Makefile; add if missing
- Output: `apps/web/src/api/generated/`
- Add to `.gitignore`

### T10 — Rebuild `apps/web` with new stack
- Remove old Leaflet dependencies
- Install new: `maplibre-gl`, `react-map-gl`, `shadcn/ui`, `tailwindcss`,
  `lucide-react`, `sonner`
- Init Tailwind CSS (`tailwind.config.ts`, `postcss.config.js`)
- Run `npx shadcn@latest init` — configure components.json
- Add shadcn/ui components: Button, Card, Input, Label, Select, Table, Badge,
  Alert, Skeleton, Sheet, Separator, Toaster (sonner)
- Update `tsconfig.json` path aliases (`@/`)
- Update `vite.config.ts` for `@/` alias

### T11 — Configure API client + env
- `apps/web/src/api/client.ts` — axios instance, JWT interceptor, refresh cycle
- `apps/web/src/config/env.ts` — `VITE_API_BASE_URL`, `VITE_MAP_STYLE_URL`
- `.env.example` — both vars documented

### T12 — Auth feature
- `features/auth/authApi.ts` — register/login/refresh via generated client
- `features/auth/useAuth.ts` — token state, `auth:logout` event listener
- `features/auth/AuthGuard.tsx`
- `pages/LoginPage.tsx` — shadcn/ui Card + Input + Button form
- `app/providers.tsx` — QueryClientProvider + Toaster
- `app/router.tsx` — createBrowserRouter, /login → LoginPage, / → AuthGuard → DashboardPage

### T13 — Devices feature
- `features/devices/devicesApi.ts`
- `features/devices/DeviceSelector.tsx` — shadcn/ui Select

### T14 — Tracking filters
- `features/tracking/TrackingFilters.tsx`
- shadcn/ui Card with: DeviceSelector, datetime inputs (Input type="datetime-local"),
  Refresh button
- Defaults: from = now−24h, to = now

### T15 — Map component (MapLibre GL)
- `components/map/trackingGeoJson.ts` — convert `TrackingPoint[]` to GeoJSON
  FeatureCollection: LineString + Point features
- `components/map/TrackingMap.tsx` — react-map-gl Map, GeoJSON source,
  track-line layer, points-circle layer, point-selected layer
- fitBounds on data change
- click → selectedPointId state → Sheet with point details
- Skeleton over map while loading
- Empty state overlay when no points

### T16 — Points table
- `components/tracking/PointsTable.tsx`
- shadcn/ui Table
- Columns: recorded_at, lat, lon, accuracy_m, speed_mps, activity_type
- Highlight row on selectedPointId sync with map

### T17 — Summary cards
- `components/tracking/SummaryCards.tsx`
- 6 shadcn/ui Cards: points_count, duration, avg_speed, max_speed, first_at, last_at
- Skeleton while loading

### T18 — Dashboard page + layout
- `components/layout/AppLayout.tsx` — topbar (app name, user email, Logout Button)
- `pages/DashboardPage.tsx` — TrackingFilters sidebar + SummaryCards + TrackingMap + PointsTable
- TanStack Query: `useQuery` for devices, points, summary
- Loading/error/empty states wired

### T19 — Docker Compose web service
- Add `web` service to `docker-compose.yml`
- `apps/web/Dockerfile.dev`
- Port 3000

### T20 — Documentation
- `apps/web/README.md`
- `docs/architecture/web-client.md`
- `docs/manual-testing/web-dashboard.md`
- Root `README.md` web section

### T21 — Run `make test` and verify
- All Go tests pass
- `cd apps/web && npm run build` succeeds with no TS errors

### T22 — Manual E2E flow
- `make up` + `make migrate-up` + `make migrate-worker-up`
- Android sends point → worker stores → web shows it

---

## Acceptance Criteria

- [ ] `apps/web` exists and `npm run build` succeeds with no errors
- [ ] Web app uses generated TypeScript client for all REST calls
- [ ] `GET /v1/tracking/points` — authenticated, user-isolated, paginated
- [ ] `GET /v1/tracking/summary` — authenticated, user-isolated
- [ ] OpenAPI spec includes new endpoints; Swagger UI shows them
- [ ] TypeScript client regenerated from updated spec
- [ ] User can login from web app (register page optional for MVP)
- [ ] User can select device and date range
- [ ] `apps/web` uses shadcn/ui + Tailwind CSS (no bare HTML forms/tables)
- [ ] Dashboard uses shadcn/ui Card, Button, Input, Select, Table, Badge, Skeleton, Alert
- [ ] Map implemented with MapLibre GL + react-map-gl (not Leaflet)
- [ ] Track points rendered as GeoJSON circle layer (not DOM markers)
- [ ] Track rendered as GeoJSON line layer
- [ ] Selected point highlighted (separate GeoJSON layer, red circle)
- [ ] Sheet/popup shows selected point details
- [ ] fitBounds fires when points data changes
- [ ] Map attribution `© OpenStreetMap contributors` visible
- [ ] Map tile provider configurable via `VITE_MAP_STYLE_URL` env var
- [ ] Raw points appear in shadcn/ui Table
- [ ] Summary cards show correct stats (6 cards)
- [ ] Unauthenticated request to `/v1/tracking/points` → 401
- [ ] User A cannot see user B's points
- [ ] Empty state renders correctly (no points in range)
- [ ] Loading skeleton renders on map and cards while data fetches
- [ ] Error alert renders on fetch failure
- [ ] `make test` passes
- [ ] Manual E2E: Android → backend → web flow verified

---

## Test Plan

### Backend (Go tests)

| Test | Expected |
|------|----------|
| GET /v1/tracking/points — no auth | 401 |
| GET /v1/tracking/points — user sees own points | 200, own data |
| GET /v1/tracking/points — user cannot see other user | 200, empty |
| device_id filter applied | only matching device |
| from/to filter applied | only points in range |
| limit=10 applied | max 10 items |
| limit=9999 → capped to 5000 | max 5000 items |
| cursor pagination — second page | correct next page |
| GET /v1/tracking/summary — count/speed correct | correct values |
| empty range → empty result | not error |

### Web (Vitest)

| Test | Expected |
|------|----------|
| AuthGuard redirects unauthenticated | redirect to /login |
| DeviceSelector renders device list | devices shown |
| PointsTable renders 3 points | 3 rows |
| SummaryCards renders zero state | "0 points" shown |
| MapView renders with empty points | no crash |

### Manual E2E

1. `make up && make migrate-up && make migrate-worker-up`
2. Run Android app (or use test HTTP client to send a location event)
3. Open http://localhost:3000
4. Register / login as same user
5. Select device, set date range containing the sent point
6. Map shows the point as a marker
7. Table shows the point row
8. Summary cards: points_count = 1

---

## Documentation Plan

| File | Content |
|------|---------|
| `apps/web/README.md` | Setup, env vars, codegen steps, dev server |
| `docs/architecture/web-client.md` | Architecture overview, component diagram |
| `docs/manual-testing/web-dashboard.md` | Step-by-step E2E guide |
| Root `README.md` | Add web client section with port and run instructions |
| `.claude/EPICS.md` | EPIC 07 entry (done) |

---

## Implementation Log

### 2026-06-11 — T10–T19 implemented: shadcn/ui + MapLibre + new layout

- Tailwind v4 + @tailwindcss/vite plugin configured
- shadcn/ui components written: Button, Card, Input, Label, Select, Table, Badge, Alert, Skeleton, Sheet, Separator, Sonner
- `@/` path alias in tsconfig + vite
- MapLibre GL + react-map-gl v8: TrackingMap with GeoJSON source, track-line/points-circle/point-selected layers
- fitBounds on data change, click → Sheet with point details
- OSM raster fallback style, VITE_MAP_STYLE_URL env var
- AppLayout: topbar + filter sidebar + main content area
- SummaryCards, PointsTable — shadcn/ui Card/Table
- TrackingFilters replaces DateRangePicker
- LoginPage/DashboardPage rewritten with shadcn/ui
- Login + Register merged into single LoginPage (mode toggle)
- 6 tests pass, `npm run build` succeeds

### 2026-06-11 — Stack update: shadcn/ui + MapLibre (plan only)

EPIC.md updated to reflect new frontend stack:
- UI kit: shadcn/ui + Tailwind CSS + lucide-react + sonner (replaces bare HTML)
- Map: MapLibre GL + react-map-gl (replaces react-leaflet)
- Map rendering: GeoJSON source/layers instead of DOM markers
- Dashboard layout: topbar + filter sidebar + summary + map + table
- `VITE_MAP_STYLE_URL` env var for tile provider
- Tasks T10–T18 rewritten for new stack
- Acceptance Criteria expanded

### 2026-06-11 — JWT refresh cycle (Android + web)

- `tokenStorage.ts` — добавлен `getRefreshToken`
- `authApi.ts` (Android) — добавлен `refreshTokens`
- `httpClient.ts` (Android) — auto-read token из storage, 401→refresh→retry, mutex, `AuthExpiredError`
- `App.tsx` — `onSessionExpired` callback → `clearTokens` + reset state
- `TrackerScreen.tsx` — `onSessionExpired` prop, catch `AuthExpiredError`, reconnect с fresh token из storage
- `authApi.ts` (web) — добавлен `refreshTokens`
- `client.ts` (web) — axios response interceptor, single-flight refresh, `auth:logout` event на failure
- `useAuth.ts` (web) — слушает `auth:logout`, сбрасывает token state

### 2026-06-11 — Backend read API (T02–T07)

- `internal/domain/tracking_point.go` — `TrackingPoint`, `TrackingPointFilter`, `TrackingSummary`
- `internal/usecase/tracking_query.go` — `TrackingQueryRepo` interface, `TrackingQueryUsecase` с лимитом и валидацией диапазона (max 31 day)
- `internal/storage/postgres/tracking_query_repo.go` — cursor pagination, mandatory `user_id` filter
- `internal/transport/http/tracking_query_handler.go` — `GET /v1/tracking/points`, `GET /v1/tracking/summary`, swaggo annotations
- `router.go`, `app.go` обновлены
- Unit tests (usecase): 8 тестов — pass
- Integration tests (repo): 5 тестов — pass
- `make swagger-gen` — новые эндпоинты в swagger.json

### 2026-06-11 — Web app scaffold (T08–T19)

- `Makefile` — `ts-client`, `web-up`, `web-build`
- `apps/web/` — React 18 + Vite 6 + TypeScript 5, все зависимости установлены
- Auth feature, devices feature, tracking feature
- MapView (react-leaflet + OSM), PointsTable, SummaryCards, состояния loading/error/empty
- LoginPage, RegisterPage, DashboardPage
- 5 unit тестов — pass; `npm run build` — success
- `docker-compose.yml` — сервис `web` (profile: web, port 3000)

---

## Final Report

_Filled when EPIC is Done._
