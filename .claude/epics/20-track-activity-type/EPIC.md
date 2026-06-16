# EPIC 20: Activity type on the /track route

## Status

Done

## Goal

Expose `activity_type` on the `/v1/tracking/track` route segments so the web
map's activity hover layer shows the real activity (walking / running /
bicycle / vehicle / stationary) instead of always "unknown".

## Context

The web route map (`DashboardPage` → `RouteMap` → `OpenFreeMapProvider`)
builds its hover "activity runs" from `buildActivityRuns(state.points)`. Those
points come from `/v1/tracking/track`, mapped in `DashboardPage.mapPoints`.

`TrackSegment` (web) and `domain.TrackSegment` (gateway) have no
`activity_type`, and the gateway `GetTrack` query reads from
`processed_location_points`, which does not store activity at all. So
`routeSegments.activityOf` falls back to `'unknown'` for every point — the
hover always says UNKNOWN even though `/points` and `raw_location_points`
hold the correct value (e.g. all `walking`).

## Problem Analysis

Chrome-devtools inspection of `?from=2026-06-16T12:00Z&to=2026-06-16T12:50Z`:

- `/v1/tracking/points` → 776 points, all `activity_type=walking`.
- `/v1/tracking/track` → 308 segments, **no `activity_type` field**.
- `/v1/tracking/summary` → no activity_type.

Root cause chain:

1. `processed_location_points` has no `activity_type` column (verified via
   `\d processed_location_points`). The worker domain
   `ProcessedLocationPoint` already carries `ActivityType` /
   `ActivityConfidence`, but `insertProcessedPoints` /
   `UpsertProcessedPoints` never persist them.
2. Gateway `GetTrack` therefore cannot select activity.
3. `domain.TrackSegment` + web `TrackSegment` have no field for it.
4. Web `mapPoints` maps segments without `activityType` → `'unknown'`.

## Best Practice Research

- A "move" segment is a single accepted point → use its own `activity_type`.
- A "stay" segment is an aggregate of many points → use the statistical mode
  via Postgres `mode() WITHIN GROUP (ORDER BY activity_type)` as the
  representative activity.
- Backfill existing `processed_location_points` from `raw_location_points`
  (FK guarantees a matching raw row) so historical routes get activity too,
  without a full reprocess.

## Solution Design

1. **Migration (tracking-worker owns the table)** `0010_add_activity_type_to_processed`:
   - `ADD COLUMN activity_type text NOT NULL DEFAULT 'unknown'`
   - `ADD COLUMN activity_confidence double precision`
   - Backfill both from `raw_location_points`.
   - Down: drop the two columns.
2. **Worker repo** (`trip_repo.go`): persist `activity_type` /
   `activity_confidence` in `insertProcessedPoints` and `UpsertProcessedPoints`
   (incl. the `DO UPDATE SET` for reprocessing).
3. **Gateway `GetTrack`**: select `activity_type` in the `accepted` CTE; emit
   `g.activity_type` for `move` rows and `mode() WITHIN GROUP` for `stay` rows;
   scan into `domain.TrackSegment.ActivityType`.
4. **Gateway DTO / swagger**: add `activity_type` to the track segment response.
5. **Web**: add `activity_type` to `TrackSegment`; set
   `mapPoints[].activityType = segment.activity_type` in `DashboardPage`.

## Architecture Notes

Gateway only reads `processed_location_points`; no gateway migration needed.
The column is added by the worker migration. Dependency direction unchanged.

## Tasks

- [x] Worker migration 0010 (+backfill) up/down
- [x] Persist activity in worker repo (insert + upsert)
- [x] Gateway GetTrack query + domain.TrackSegment field + scan
- [x] Gateway track DTO / swagger annotation
- [x] Web TrackSegment + mapPoints wiring
- [x] Tests: worker repo, gateway GetTrack, web mapping

## Acceptance Criteria

- `/v1/tracking/track` returns `activity_type` per segment.
- Hover on a route segment in the web map shows the real activity, not
  "unknown", for both new and historical (backfilled) data.
- Existing tests pass; new coverage for the query and mapping.

## Test Plan

- Worker: `insertProcessedPoints` persists activity (repo test).
- Gateway: `GetTrack` returns activity for move (point value) and stay (mode).
- Web: `mapPoints` carries `activityType`; `buildActivityRuns` groups by it.

## Documentation Plan

- Regenerate swagger (`make swagger-gen`).
- Update tracking-gateway README track section if it lists fields.

## Implementation Log

- 2026-06-16: Added migration 0010 (activity_type NOT NULL DEFAULT 'unknown' +
  activity_confidence, backfilled from raw_location_points). Worker repo now
  persists both in insertProcessedPoints and UpsertProcessedPoints (+ guard
  `activityTypeOrUnknown`). Gateway GetTrack selects activity from the accepted
  CTE — move emits the point's value, stay emits `mode() WITHIN GROUP`; scanned
  into the new `domain.TrackSegment.ActivityType`. Added `activity_type` to
  `TrackSegmentItem` (swagger) and the handler mapping; regenerated swagger.
  Web: `TrackSegment.activity_type` + `mapPoints.activityType`.
- Deployed worker+gateway (migration auto-applied). Backfill result on
  processed_location_points: walking 1486, stationary 93, unknown 14957.

## Final Report

Fixed: the web route hover layer no longer shows "unknown" — `/v1/tracking/track`
now returns `activity_type` per segment. Verified live via chrome-devtools on
`?from=2026-06-16T12:00Z&to=2026-06-16T12:50Z`: all 308 segments report
`walking` (was: field absent → 'unknown'). Backfill covers historical data.

Tests: gateway GetTrack asserts activity flows through (stay→modal, move→point);
worker repo tests green; web trackingGeoJson + routeSegments green; full gateway
suite exit 0.

Note: representative activity for a stay segment is the statistical mode of its
points. Per-point activity granularity inside a stay is intentionally collapsed.
