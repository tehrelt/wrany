# WR any% Android Tracker

React Native Android app for route tracking. Sends GPS points to the backend via WebSocket.
The backend detects trips, routes, and personal records automatically — no manual Start/Stop Trip in the app.

---

## Prerequisites

- Node.js 18+
- JDK 17
- Android Studio + Android SDK (API 36)
- Android emulator or real device (API 24+)
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

## Running

```bash
cd apps/android-tracker
npm install
npm run generate:api   # requires: make openapi-generate && make openapi-merge first
npx react-native run-android
```

### Host config

**Emulator:** `10.0.2.2:8080` routes to host localhost — default, no change needed.

**Real device on LAN:** edit `src/config/env.ts`:
```ts
const HOST = '192.168.1.42';  // your machine's LAN IP
```

---

## App screens

### Background tab (default)

Native Android foreground service with SQLite offline queue.

| Section | Shows |
|---------|-------|
| Permissions | Fine location / Background location / Notifications status |
| Service | Running, pending/failed queue counts, last location & sync times |
| Controls | Enable/Disable tracking, Flush now, Clear failed points |

**Enable tracking flow:**
1. Tap **Enable tracking**
2. Grant foreground location permission
3. Grant notification permission (Android 13+)
4. For background tracking: tap **Open Settings** → "Allow all the time"
5. Foreground service starts, persistent notification appears

### Legacy WS tab

Manual WebSocket debug screen (foreground only, in-memory queue).
Use for testing the backend WS protocol directly.

---

## Permissions

| Permission | Required for |
|-----------|-------------|
| `ACCESS_FINE_LOCATION` | GPS location |
| `ACCESS_COARSE_LOCATION` | Fallback location |
| `ACCESS_BACKGROUND_LOCATION` | Tracking when app is in background |
| `FOREGROUND_SERVICE` | Running the tracking service |
| `FOREGROUND_SERVICE_LOCATION` | Location-type foreground service (API 34+) |
| `POST_NOTIFICATIONS` | Persistent tracking notification (API 33+) |
| `ACCESS_NETWORK_STATE` | Network connectivity checks |

If background location is denied, the app continues in foreground-only mode and shows a warning.

---

## Architecture

```
TrackingStatusScreen (RN)
  ↓ NativeModule bridge (TrackingModule)
TrackingForegroundService (Kotlin, Android Service)
  ├── LocationProvider  — FusedLocationProviderClient (12s / 15m)
  ├── LocationQueue     — Room/SQLite, stable event_id per point
  └── BatchSender       — OkHttp WebSocket, reconnect, ACK handling
        ↓
tracking-gateway (backend)
        ↓
NATS JetStream → tracking-worker → trips → routes → records
```

**Offline queue guarantees:**
- Points persisted to SQLite before sending
- `event_id` = UUID at INSERT time, never regenerated on retry
- Backend dedup by `(user_id, device_id, event_id)` — retries are safe
- ACK marks points as `acked`; network errors return to `pending`
- Permanent rejections mark as `failed` with reason
- Queue survives app kill and restart

**WebSocket reconnect:** exponential backoff 1s → 2s → 4s → ... → 30s max.

---

## Testing on emulator

### Simulate GPS movement

In Android Studio emulator:
1. `Extended Controls` (... button) → `Location`
2. Load a GPX route or set manual coordinates
3. Click **Play route** to simulate movement

Or use the Routes tab to replay a saved route.

### Simulate network loss

`Extended Controls` → `Cellular` → set `Network type: No network`

Verify pending count grows. Re-enable network — queue should flush automatically.

### Kill and reopen app

```bash
adb shell am force-stop com.androidtracker
# Reopen from launcher
```

Verify pending points survive in the queue.

---

## Testing on real device

1. Enable USB debugging: Settings → Developer options → USB debugging
2. Connect via USB, accept RSA key prompt
3. Verify: `adb devices` — device listed
4. `npx react-native run-android` — builds and installs

**Battery optimization:** some OEM Android (Xiaomi, Huawei, Samsung) aggressively kill background services. If tracking stops in background:
- Settings → Apps → WR any% → Battery → "No restrictions" or "Don't optimize"
- Some devices require adding the app to an autostart whitelist

---

## Running instrumented tests

Requires connected device or running emulator.

```bash
cd android
./gradlew connectedDebugAndroidTest
```

Tests cover: `LocationQueueTest` — insert, ACK, retry, stable event_id, cleanup, batch ordering.

---

## Manual E2E checklist (EPIC 11)

1. `make up && make migrate-up && make migrate-worker-up && make nats-init`
2. Install app on emulator: `npx react-native run-android`
3. Register user / login
4. Register device (Legacy WS tab)
5. Switch to **Background** tab
6. Tap **Enable tracking** → grant fine location → grant notifications
7. Verify persistent notification appears: "WR any% — Tracking active"
8. Open Settings, set location to "Allow all the time"
9. Press Home → app goes to background
10. Simulate GPS movement (Extended Controls → Location)
11. Verify notification updates queue count
12. Set network to "No network" → verify pending count grows
13. Restore network → verify queue flushes (pending → 0)
14. Force-stop app → reopen → verify pending count preserved
15. Check backend: `SELECT count(*) FROM raw_location_points` → rows exist
16. Wait for trip detection: `SELECT * FROM trips` → trip created
17. Check web analytics: route and personal records visible

---

## Known limitations

- Token not refreshed automatically when expired (restart tracking to re-auth)
- `activity_type` always null (activity recognition not implemented)
- WebSocket auth via `?access_token=` query param (RN limitation with custom WS headers)
- No iOS support
- Battery optimization on some OEM devices may interrupt background tracking

---

## TypeScript API client codegen

```bash
# From repo root
make openapi-generate    # generates tracking-gateway.json from Go annotations
make openapi-merge       # merges into combined.json

# From apps/android-tracker
npm run generate:api     # generates src/api/generated/
```

`src/api/generated/` is gitignored — always regenerate after backend API changes.
