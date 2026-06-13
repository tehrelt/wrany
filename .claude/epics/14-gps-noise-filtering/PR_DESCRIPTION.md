## Summary
- Added GPS noise filtering v2.
- Added algorithm_version infrastructure.
- Added reprocess-ready processed location schema.
- Fixed distance calculation using raw segment distance.
- Added trip segmentation by location gaps.
- Fixed active trip completion on segment gap.
- Added EPIC.md documentation.

## Validation
- go test ./...
- go vet ./...
- storage tests passed against real Postgres
- worker v2 built and healthy
- migrations applied successfully

## Real data check
Dataset: 214 location points.
Before:
- jitter: 194
- pipeline distance: 489 m
- trips: 0

After:
- jitter: 104
- pipeline distance: 1446 m
- trips: 1
- completed trip: 10:45:03→10:47:43, 249.5 m, 160 s

### Distance reconciliation (1446 m vs 249.5 m)
These measure different scopes and are both correct:
- **Pipeline distance (1446 m)** is the sum of `distance_delta_m` over **all**
  accepted processed points for the day, split across **62 segments** (segment
  boundaries created by long time gaps). It is the total filtered movement of
  the whole processed set, not a single trip.
- **Trip distance (249.5 m)** is the distance of the **one** completed trip
  (10:45:03→10:47:43). The sum of accepted `distance_delta_m` inside that trip
  window is 243.3 m over 11 points; the trip row reports 249.5 m / 12 points.
  The ~6 m difference is the trip state machine's boundary/start-point handling
  vs a naive SQL window sum — not a discrepancy. The remaining ~1197 m lives in
  the other 61 segments that never accumulated enough continuous movement to
  form a trip.

## Notes
- Auto-reprocess and historical trip rebuild are intentionally not included.
- Full reprocess should be operator-triggered, not automatic on every
  algorithm_version bump. See
  `.claude/epics/14-gps-noise-filtering/TODO-operator-reprocess.md`.
- Android activity_type is still empty and should be handled in tracker client repo.
