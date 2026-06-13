# EPIC 14: GPS Noise Filtering & Trip Detection Correctness

## Status

Implemented | Tested | Deployed (local)

## Goal

Fix the GPS noise/location-processing pipeline in `tracking-worker` so that real
movement produces trips. A full day of real points produced 0 trips despite a
clear ~3-minute walk; diagnose and fix the silent loss of movement.

## Context

Diagnostic was run against the live database (today's 214 raw points, one
device pair). Findings: 194/207 accepted points flagged `jitter` (distance 0),
total distance 489 m, 0 trips. Raw immutability, the processed/raw split, and
config-driven thresholds were already correct (EPIC 05/08 + the
`algorithm_version` reprocessing groundwork).

## Problem Analysis

1. Distance/jitter were computed on the **smoothed** track. The weighted
   moving-average lag collapsed 16–35 m walking steps below the jitter radius,
   so ~94% of real walking was marked `jitter` with `distance_delta_m = 0`.
2. A single `jitter` point reset the motion candidate, so a walk peppered with
   GPS noise never confirmed a trip (and stale candidates kept an old start).
3. No time-gap segmentation: points minutes/hours apart were treated as one
   continuous track (smoothing, distance, windows). 31 gaps > 5 min today,
   max 107 min.
4. An active trip absorbed points arriving after a long silence instead of
   finalizing — a 3-minute walk showed up as a 33-minute `TRIP_ACTIVE`.
5. (Out of scope here) Android tracker sends empty `activity_type`.

## Best Practice Research

Standard GPS track processing measures displacement on the raw fix and uses
smoothing only for display; noise is rejected by an accuracy-derived radius,
not by collapsing the track. Sessions/segments are split on time gaps. These
informed the fixes below.

## Solution Design

- Measure distance/jitter on the raw track; smoothing only feeds `filtered_*`
  (display) and is bounded to one segment.
- Introduce `SegmentMaxGapSec` (`GPS_SEGMENT_MAX_GAP_SEC`, default 120s):
  points after a long gap are `segment_break` with no distance across the gap.
- Motion candidate tolerates isolated jitter; resets only on a real stop
  (`IsStationary`) or a gap > `StopMinDurationSec` (also fixes stale candidates).
- Active trip completes at the last known point when a gap > `StopMinDurationSec`
  occurs; the late point starts a fresh evaluation.
- Reprocessing groundwork: `algorithm_version` column (migration 0009),
  stamped by the pipeline, `CurrentAlgorithmVersion = 2`;
  `FetchPointsForReprocessing` + `UpsertProcessedPoints` select/upsert stale
  rows. Full automatic trip rebuild remains a TODO.

## Architecture Notes

All changes stay within `tracking-worker` (domain/usecase/storage/config/app).
No change to raw ingestion, transport, or other services. Distance is a usecase
concern; smoothing is an injectable `LocationSmoother`.

## Tasks

- [x] Diagnose against live DB.
- [x] Fix distance/jitter on raw track + segment break.
- [x] Fix motion candidate jitter tolerance + gap reset.
- [x] Complete active trip on long gap.
- [x] `algorithm_version` column, stamping, reprocess repo methods.
- [x] Unit + storage tests; deploy + backfill validation.
- [ ] Operator-triggered reprocess + affected trips rebuild — see
  [TODO-operator-reprocess.md](./TODO-operator-reprocess.md) (NOT automatic).
- [ ] Tracker-side `activity_type` (separate, Android repo).

## Acceptance Criteria

- The real walk produces a single completed trip with realistic distance/duration.
- No false distance across long time gaps.
- `go test ./...` and `go vet ./...` green.

## Test Plan

Unit (`noise`, `usecase`): walk-after-rest counts distance, long gap adds no
distance, isolated jitter keeps candidate, long gap resets stale candidate,
outlier adds no distance, long gap completes active trip, algorithm_version
stamped. Storage (Postgres/testcontainers): reprocess selection + version-gated
upsert. Live validation: backfill today's points through the new worker.

## Documentation Plan

This EPIC.md; inline code comments on each fix and the reprocess TODO.

## Implementation Log

- Diagnosed live DB: 194 jitter, 489 m, 0 trips; isolated the 10:45–10:47 walk.
- Implemented fixes #1/#2 + segment break; 4 TDD tests RED→GREEN.
- Added `algorithm_version` (migration 0009) + reprocess repo methods + tests.
- Deployed worker v2, backfilled: 489→1446 m, 1 trip created.
- Added active-trip gap completion; redeployed + re-backfilled.
- Diagnosed second live case (user 0ec65079, 2026-06-13 17:34–20:34 local):
  6496 pts, 2.6 km real spread, only 82.8 m accumulated, 5002 jitter, 0 trips,
  state stuck IDLE. Root cause: jitter gated on **adjacent-sample** distance —
  at ~1.3 s / 1.35 m cadence every step is below the 8 m floor, so v2's raw-track
  fix still dropped the walk.
- Fix #3 (algo v3): measure distance/jitter against the last position-establishing
  anchor (`lastAnchor`, skips jitter/stationary) instead of the previous sample;
  small steps now accumulate displacement, in-place dither still suppressed.
  Added 2 regression tests (high-frequency walk recovers >200 m; in-place dither
  stays ~0). All noise + worker tests green, build clean.
- Diagnosed third live case (user 0ec65079, 2026-06-13): after a rest at a bus
  stop (correctly filtered — standing, not a bug), the **bus ride was dropped**.
  Window 14:45–14:55: 303 `teleport`, ~0 accepted for ~10 min, trip stuck. Root
  cause: client always sends `activity_type="unknown"`, so the speed gate uses the
  walking ceiling (3.5 m/s) and rejects all 5–9 m/s vehicle samples as teleports.
  The "movement onset poorly detected" the user reported = the speed jump on
  boarding looks like a teleport while speed stays above walking.
- Fix #4 (algo v4): speed-outlier detection now confirms a faster-than-walking
  sample against the **next** sample before rejecting it. Above the vehicle
  ceiling (60 m/s) → always teleport; in between → accept if the next sample keeps
  progressing away from the anchor (sustained travel), reject if it snaps back
  (out-and-back spike). Activity-independent, so the broken `activity_type` no
  longer drops vehicle trips. 2 TDD tests (sustained fast travel accepted +
  accumulates distance; teleport-and-return rejected). All tests green, vet clean.

## Final Report

Live result after all fixes (same 214 raw points): pipeline distance
489 m → 1446 m; the walk became **TRIP_COMPLETED 10:45:03 → 10:47:43,
249.5 m, 160 s, 12 points** (was 0 trips). All tests green, `go vet` clean.

v4 live result (user 0ec65079, 2026-06-13, manual delete+reset reprocess of the
14:00→ window): bus-ride window `teleport` 303 → **0**; the hour-long void
(trip ended 14:44 at the stop, next trip only 15:44) became a continuous
**TRIP_COMPLETED 14:44:49 → 15:22:18, 1701 m, 1465 points** covering the ride.

Remaining: automatic reprocess+trip-rebuild wiring (TODO in code); tracker-side
`activity_type` is broken (always `unknown`) — Android Activity Recognition is a
separate task. Once real activity is sent, per-mode speed ceilings become exact
and the v4 confirmation becomes a safety net rather than the primary gate.
