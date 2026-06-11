# WR any% Web Client

React/Vite/TypeScript dashboard for viewing raw GPS tracking data.

## Requirements

- Node.js 20+
- Backend services running (see root README)

## Setup

```bash
cp .env.example .env
npm install
```

## Run dev server

```bash
npm run dev
# → http://localhost:3000
```

## Build

```bash
npm run build
```

## Test

```bash
npm test
```

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `VITE_API_BASE_URL` | `http://localhost:8080` | tracking-gateway base URL |

## Regenerate API types

After changing backend endpoints:

```bash
# from repo root:
make swagger-gen    # regenerate OpenAPI spec
make ts-client      # generate apps/web/src/api/generated/schema.d.ts
```

Generated types are gitignored. Run before building in CI.

## Via Docker Compose

```bash
# from repo root:
docker compose --profile web up web
# → http://localhost:3000
```

## Manual E2E flow

1. `make up && make migrate-up && make migrate-worker-up`
2. Start Android app or send test location events
3. Open http://localhost:3000
4. Register / login as the same user
5. Select device + date range
6. Map shows GPS points, table lists rows, summary cards show stats

## Token refresh

Access tokens refresh automatically on 401. On refresh failure, user is
redirected to login. Tokens are stored in `localStorage` (HttpOnly cookie
hardening is future scope).
