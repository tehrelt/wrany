# WR any% Android Tracker — Foreground MVP

Minimal React Native Android app for manual testing of the full backend pipeline.
Foreground tracking only — no background service.

---

## Prerequisites

- Node.js 18+
- JDK 17
- Android Studio + Android SDK (API 33+)
- Android emulator or real device
- Backend running (`make up`)

---

## Backend setup

```bash
make up
make migrate-up
make migrate-worker-up
make nats-init
```

---

## Running on Android emulator

```bash
cd apps/android-tracker
npm install
npm run generate:api   # requires: make openapi-generate && make openapi-merge first
npx react-native run-android
```

### Emulator URL

The app defaults to `10.0.2.2:8080` which routes to the host machine from an Android emulator.
No config change needed for emulator.

### Real device on LAN

Edit `src/config/env.ts`:
```ts
const HOST = '192.168.1.42';  // your machine's LAN IP
```

---

## TypeScript API client codegen

The REST API layer uses a generated TypeScript client:

```bash
# From repo root
make openapi-generate    # generate tracking-gateway.json from Go annotations
make openapi-merge       # merge into combined.json

# From apps/android-tracker
npm run generate:api     # generates src/api/generated/ from combined.json
```

`src/api/generated/` is gitignored — always regenerate after backend API changes.

---

## App flow

```
Launch → check stored token
  → AuthScreen (if no token): register or login
  → TrackerScreen: register device, connect WS, start tracking
```

### Debug UI buttons

| Button | Action |
|--------|--------|
| Register Device | Registers this device UUID with backend |
| Connect WS | Opens WebSocket, sends session.start |
| Start Tracking | Requests GPS permission, starts location watcher |
| Stop Tracking | Stops watcher, flushes pending batch |
| Send Test Point | Sends a synthetic GPS event (lat 55.75, lon 37.62) without moving |

### Counters

- **Pending** — events in memory awaiting ACK
- **Accepted** — events confirmed by backend
- **Duplicated** — events already seen (same event_id)
- **Rejected** — events rejected by backend validation

---

## Manual E2E test checklist

1. `make up && make migrate-up && make migrate-worker-up && make nats-init`
2. Start app on emulator
3. Register user → expect no error
4. Login → expect token stored
5. Tap **Register Device** → expect success
6. Tap **Connect WS** → status changes to `session_accepted`
7. Tap **Send Test Point** → accepted counter increments
8. Verify DB: `SELECT count(*) FROM raw_location_points` → expect >= 1
9. Tap **Send Test Point** again (same synthetic event_id) → duplicated counter increments
10. Tap **Start Tracking** → grant permission → observe GPS points flowing
11. Verify DB again → rows accumulating
12. Tap **Stop Tracking** → WS closes cleanly

---

## Required Android permissions

```xml
<uses-permission android:name="android.permission.INTERNET" />
<uses-permission android:name="android.permission.ACCESS_FINE_LOCATION" />
<uses-permission android:name="android.permission.ACCESS_COARSE_LOCATION" />
```

Background location is NOT requested. Tracking stops when the app goes to the background.

---

## Known limitations (EPIC 6 scope)

- Foreground only — no background tracking
- In-memory queue only — events lost on app restart
- No offline queue — requires active WS connection to send
- Token not stored securely (AsyncStorage MVP — no keychain)
- No auto-reconnect after disconnect
- No activity recognition (activity_type always `unknown`)
- WebSocket auth uses `?access_token=` query param (React Native limitation with custom headers)
