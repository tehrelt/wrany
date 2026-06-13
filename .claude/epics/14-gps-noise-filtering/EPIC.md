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
- [ ] Wire automatic reprocess + trip rebuild into the job (TODO).
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

## Final Report

Live result after all fixes (same 214 raw points): pipeline distance
489 m → 1446 m; the walk became **TRIP_COMPLETED 10:45:03 → 10:47:43,
249.5 m, 160 s, 12 points** (was 0 trips). All tests green, `go vet` clean.
Remaining: automatic reprocess+trip-rebuild wiring (TODO in code) and
tracker-side `activity_type`.
