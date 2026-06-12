# EPIC 12: Android Background Tracking + Offline Queue

## Status

In Progress

---

## Goal

Реализовать Android background tracking MVP:

- Android foreground service для сбора геолокации в фоне;
- background location permission flow с обучающим UI;
- persistent tracking notification;
- durable SQLite offline queue (Room) с ACK-based удалением;
- stable event_id для очереди — retry не дублирует точки на backend;
- WebSocket reconnect/retry через OkHttp (в native layer);
- сохранение GPS-точек при отсутствии сети;
- debug/status UI в Android app.

Backend не меняется. Pipeline неизменен:

```text
Android Tracker
  ↓ WebSocket (tracking-gateway)
NATS JetStream (location.events.v1)
  ↓
tracking-worker → trips → routes → personal records
```

---

## Context

EPIC 10 (Best Results & Personal Records) завершён, смержен в main, статус Done.

Текущее состояние Android tracker:
- React Native 0.86.0, `targetSdkVersion: 36`, `minSdkVersion: 24`
- Kotlin уже настроен (`MainActivity.kt`, `MainApplication.kt`)
- `react-native-geolocation-service` установлен
- Location watcher живёт в JS-слое — умирает вместе с приложением
- `BatchQueue` in-memory — данные теряются при kill
- Нет foreground service, нет background permission
- Нет нативных модулей (только autolinked)

WebSocket protocol:
- `event_id` per-point (не per-batch) — стабильный идентификатор для SQLite queue
- ACK: `accepted[]`, `duplicated[]`, `rejected[]` по `event_id`
- Токен через query param `?access_token=...`
- Dedup: `(user_id, device_id, event_id)` — backend защищён от дубликатов

---

## Problem Analysis

### Проблема 1: Location теряется в background

JS-слой React Native получает kill или suspend при переходе в background на Android.
`Geolocation.watchPosition()` в JS не гарантирует работу в фоне.
**Решение:** Android Foreground Service с `foregroundServiceType="location"`.

### Проблема 2: Данные теряются при отсутствии сети

`BatchQueue` in-memory — любой краш или отсутствие сети означает потерю точек.
**Решение:** SQLite (Room) с persist-before-send — точка попадает в БД до попытки отправки.

### Проблема 3: Дублирование при retry

Если отправить точку дважды — backend должен это обнаружить.
Backend dedup работает по `(user_id, device_id, event_id)`.
**Решение:** `event_id` генерируется при записи в SQLite, никогда не меняется при retry.

### Проблема 4: WebSocket отрывается при потере сети

JS WebSocket не reconnects в background.
**Решение:** OkHttp WebSocket в native layer (ForegroundService), reconnect с exponential backoff.

### Проблема 5: Background location permission (Android 10+)

На Android 10+ `ACCESS_BACKGROUND_LOCATION` выдаётся отдельно через Settings.
Нельзя запросить одновременно с foreground location.
**Решение:** Staged permission flow: foreground → объяснение → Settings redirect.

---

## Best Practice Research

### Android Foreground Service (Location type)

- Требует `android:foregroundServiceType="location"` в Manifest (API 29+)
- `FOREGROUND_SERVICE_LOCATION` permission требуется с API 34 (targetSdkVersion 34+)
- `startForeground()` должен вызываться в течение 10 секунд после `startService()`
- Service должен показывать notification — пользователь должен знать о tracking

### Background Location Permission Flow

- Нельзя запрашивать `ACCESS_BACKGROUND_LOCATION` одновременно с foreground
- Сначала: foreground permission (`ACCESS_FINE_LOCATION`)
- Затем: объяснение пользователю (educational UI)
- Затем: `Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS)` → пользователь сам выбирает "Allow all the time"
- Graceful degradation: без background permission — foreground-only tracking

### FusedLocationProviderClient

- Google Play Services API, наиболее надёжный на Android
- `LocationRequest` с `PRIORITY_HIGH_ACCURACY` или `PRIORITY_BALANCED_POWER_ACCURACY`
- Interval: 10-15 секунд для MVP
- `LocationCallback` живёт в Service, не зависит от JS

### Room SQLite

- Стандартная Android persistence library
- Поддержка сложных запросов, migration, LiveData/Flow
- Подходит для offline queue: insert before send, delete on ACK
- Работает в том же процессе что и ForegroundService

### OkHttp WebSocket

- Стандарт для Android native HTTP/WS
- `WebSocketListener` для callback-driven модели
- Reconnect: `newWebSocket()` после `onFailure()` с backoff
- Thread-safe send через `webSocket.send()`

### event_id idempotency

- Генерировать при INSERT в SQLite: `UUID.randomUUID().toString()`
- Никогда не регенерировать при retry
- Backend dedup по `(user_id, device_id, event_id)` защищает от дубликатов

---

## Solution Design

### Архитектура

```text
React Native UI (TypeScript)
  ↓ NativeModule bridge (TrackingModule)
TrackingForegroundService (Kotlin)
  ↓ FusedLocationProviderClient
LocationQueue (Room/SQLite)
  ↓ BatchSender
OkHttp WebSocket → tracking-gateway
```

### Native/RN boundary

**RN контролирует:**
- `enableTracking()` — запустить сервис
- `disableTracking()` — остановить сервис
- `flushNow()` — немедленная отправка
- `getTrackingStatus()` → `{ serviceRunning, wsStatus, pendingCount, failedCount, lastLocationTime, lastSyncTime, permissions }`

**Native layer владеет:**
- lifecycle foreground service
- location updates (FusedLocation)
- SQLite writes
- WebSocket connection и reconnect
- notification updates

### Permission flow (staged)

```
1. Request ACCESS_FINE_LOCATION
   ↓ granted
2. Educational UI: "Need background access for tracking when app is closed"
   ↓ user agrees
3. Open Settings → user selects "Allow all the time"
   ↓ or user skips
4. If background denied → foreground-only mode (честный UI)
5. Request POST_NOTIFICATIONS (Android 13+)
```

### Location settings (configurable)

```kotlin
interval = 12_000          // 12 sec
fastestInterval = 5_000    // 5 sec
minDistance = 15f          // 15 meters
priority = PRIORITY_HIGH_ACCURACY
```

### Queue schema

```sql
CREATE TABLE location_queue (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  device_id TEXT NOT NULL,
  recorded_at TEXT NOT NULL,
  lat REAL NOT NULL,
  lon REAL NOT NULL,
  accuracy_m REAL NOT NULL,
  speed_mps REAL,
  bearing_deg REAL,
  activity_type TEXT,
  activity_confidence REAL,
  battery_level REAL,
  source TEXT NOT NULL DEFAULT 'android_tracker',
  status TEXT NOT NULL DEFAULT 'pending',
  attempts INTEGER NOT NULL DEFAULT 0,
  last_error TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE INDEX idx_location_queue_status ON location_queue(status);
```

### Batch sending

- Batch size: 10–50 points (MVP: 20)
- Flush interval: 15 sec
- Max inflight batches: 1 (MVP — упрощает ACK tracking)
- ACK → `UPDATE status='acked' WHERE id IN (accepted+duplicated)`
- Rejected (permanent) → `UPDATE status='failed', last_error=reason`
- Network error → `UPDATE status='pending'` (retry)
- Cleanup: acked points старше 7 дней → DELETE

### Reconnect backoff

```
attempt 1: 1 sec
attempt 2: 2 sec
attempt 3: 4 sec
attempt 4: 8 sec
attempt 5+: 30 sec (max)
```

### Token access из native layer

AsyncStorage использует SharedPreferences под капотом.
Ключ: `@wrany/access_token` → `com.androidtracker.storage` SharedPreferences.
Прямой доступ из Kotlin возможен без bridge.

---

## Architecture Notes

- Kotlin (уже настроен, `build.gradle` содержит `kotlin("android")`)
- Новые gradle dependencies: `room-runtime`, `room-ktx`, `play-services-location`, `okhttp`
- Native module регистрируется через `TrackingPackage` → добавить в `MainApplication.getPackages()`
- ForegroundService запускается через `Context.startForegroundService()`
- `startForeground()` вызывается в `onStartCommand()` немедленно (< 10 sec)
- Notification channel создаётся при инициализации Application
- Для Android 13+: `POST_NOTIFICATIONS` permission требуется для notification

---

## Tasks

### Фаза 1: Native foundation

- [ ] T1.1: AndroidManifest — добавить permissions, foreground service declaration
- [ ] T1.2: `TrackingNotification.kt` — notification channel, builder, update
- [ ] T1.3: `TrackingForegroundService.kt` — skeleton: start/stop/notification
- [ ] T1.4: `TrackingModule.kt` — RN bridge: enableTracking/disableTracking/getStatus
- [ ] T1.5: `TrackingPackage.kt` — регистрация, добавить в `MainApplication`

### Фаза 2: Location + SQLite Queue

- [ ] T2.1: `build.gradle` — добавить Room, FusedLocation, OkHttp dependencies
- [ ] T2.2: `LocationQueue.kt` — Room entity, DAO, database class
- [ ] T2.3: `LocationProvider.kt` — FusedLocationProviderClient wrapper
- [ ] T2.4: Подключить LocationProvider → LocationQueue в ForegroundService

### Фаза 3: WebSocket BatchSender

- [ ] T3.1: `BatchSender.kt` — OkHttp WS, session.start → location.batch
- [ ] T3.2: Reconnect с exponential backoff
- [ ] T3.3: ACK handler → обновить статусы в LocationQueue
- [ ] T3.4: Token reader из SharedPreferences

### Фаза 4: RN UI

- [ ] T4.1: `types.ts` — TrackingStatus, PermissionStatus типы
- [ ] T4.2: `trackingStore.ts` — состояние: serviceRunning, wsStatus, counters, permissions
- [ ] T4.3: `TrackingStatusScreen.tsx` — toggle, permissions, counters, warnings, buttons
- [ ] T4.4: `App.tsx` — добавить TrackingStatusScreen после TrackerScreen

### Фаза 5: Tests + Docs

- [ ] T5.1: Unit tests — LocationQueue (insert/read/ack/retry/stable event_id)
- [ ] T5.2: Unit tests — BatchSender (no send without session.accepted)
- [ ] T5.3: EPIC.md — Manual E2E checklist
- [ ] T5.4: `apps/android-tracker/README.md` — permissions, emulator/device testing

---

## Acceptance Criteria

- [ ] Android foreground service существует и стартует через RN toggle
- [ ] `android:foregroundServiceType="location"` указан в Manifest
- [ ] Persistent notification видна во время tracking
- [ ] Пользователь может включить/выключить tracking service
- [ ] Нет кнопок "Start Trip" / "Finish Trip"
- [ ] Background permission flow реализован
- [ ] Denied background permission → foreground-only mode с честным UI
- [ ] Location points персистируются в SQLite до отправки
- [ ] SQLite queue переживает restart приложения
- [ ] Потеря сети не теряет points
- [ ] WebSocket reconnect/retry работает
- [ ] ACK убирает/помечает точки в очереди
- [ ] Повторная отправка не создаёт дубликаты на backend
- [ ] Debug/status UI показывает: service, WS, queue, permissions
- [ ] Android build проходит (`./gradlew assembleDebug`)
- [ ] `make test` (backend) остаётся зелёным
- [ ] Manual E2E: Android background points → trips → routes → records

---

## Test Plan

### Automated

- Room: insert pending point → verify DB row
- Room: ACK accepted → verify status='acked'
- Room: network error → verify status='pending', attempts++
- Room: permanent reject → verify status='failed', last_error set
- Room: stable event_id — same row при retry (не новая запись)
- Room: cleanup — acked points > 7 дней удаляются
- BatchSender: location.batch не отправляется без session.accepted

### Manual (emulator / real device)

1. Установить app
2. Войти / зарегистрироваться
3. Зарегистрировать device
4. Разрешить foreground location
5. Включить tracking → проверить foreground service notification
6. Перевести app в background → точки продолжают собираться
7. Отключить сеть → pending queue растёт
8. Включить сеть → очередь отправляется, статус acked
9. Kill + reopen app → pending queue сохранился
10. Backend получил точки → web показывает trips/routes/records

---

## Documentation Plan

- `apps/android-tracker/README.md`:
  - Permissions и зачем они нужны
  - Как тестировать на эмуляторе (Extended Controls → Location)
  - Как тестировать на реальном устройстве
  - Известные ограничения (OEM battery optimization)
  - Battery optimization notes (Doze mode, vendor-specific)
  - Manual E2E checklist

- `.claude/epics/11-android-background-tracking-offline-queue/EPIC.md`:
  - Implementation Log (обновлять после каждой значимой задачи)
  - Final Report (после Done)

---

## Implementation Log

### 2026-06-12 — Фазы 1-4: Native foundation + RN UI

**Изменения:**
- `android/build.gradle`: добавлен KSP plugin (`com.google.devtools.ksp:2.1.20-1.0.32`)
- `android/app/build.gradle`: KSP + Room 2.6.1 + FusedLocation 21.3.0 + OkHttp 4.12.0
- `AndroidManifest.xml`: добавлены permissions (BACKGROUND_LOCATION, FOREGROUND_SERVICE, FOREGROUND_SERVICE_LOCATION, POST_NOTIFICATIONS, NETWORK_STATE) + service declaration с `foregroundServiceType="location"`
- `tracking/TrackingNotification.kt`: notification channel (IMPORTANCE_LOW) + builder с pending count и stop action
- `tracking/LocationQueue.kt`: Room entity `LocationPoint` + DAO + DB singleton
- `tracking/LocationProvider.kt`: FusedLocationProviderClient wrapper (12s interval, 5s fastest, 15m distance)
- `tracking/BatchSender.kt`: OkHttp WS client, session.start → location.batch → ACK handler, exponential backoff reconnect (1s→30s max), batch size 20
- `tracking/TrackingForegroundService.kt`: Foreground service с START_STICKY, notification updater каждые 10s
- `tracking/TrackingModule.kt`: RN bridge — enableTracking/disableTracking/getTrackingStatus/flushNow/clearFailed/updateToken
- `tracking/TrackingPackage.kt`: ReactPackage регистрация
- `MainApplication.kt`: TrackingPackage + создание notification channel при onCreate
- `src/features/tracking/types.ts`, `trackingNativeModule.ts`, `TrackingStatusScreen.tsx`: RN UI
- `src/app/App.tsx`: tab-переключение Background/Legacy WS

**Решения:**
- kapt → KSP (kapt несовместим с Kotlin 2.1.x; metadata version mismatch)
- Credentials хранятся в native SharedPreferences `wrany_tracking` (не AsyncStorage)
- event_id = UUID при INSERT в SQLite, не меняется при retry — idempotency гарантирована
- BatchSender: max 1 inflight batch для MVP

**Результат:**
- `./gradlew assembleDebug assembleDebugAndroidTest` — BUILD SUCCESSFUL (оба APK)
- `make test` — все backend тесты зелёные
- 9 инструментальных тестов: LocationQueueTest (insert, ACK, retry, stable id, cleanup, ordering)

---

## Final Report

_Будет заполнен после завершения эпика._
