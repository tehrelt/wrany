# TODO / Follow-up: Operator-triggered location reprocess + affected trips rebuild

Status: Not started (intentionally out of scope for EPIC 14)

## Why separate

EPIC 14 added the `algorithm_version` infrastructure and in-place reprocess
repo methods (`FetchPointsForReprocessing`, `UpsertProcessedPoints`) but did NOT
wire automatic reprocessing or historical trip rebuild. Automatically deleting
and rebuilding every trip on each `algorithm_version` bump is risky and would
silently mutate historical data. This must be a deliberate, operator-triggered
operation.

## Requirements

- MUST be triggered explicitly by an operator (CLI/admin endpoint/one-off job),
  never automatically on worker start or on an `algorithm_version` bump.
- MUST accept a selector: `user_id` / `device_id` / time range (`from`, `to`).
- MUST recompute `processed_location_points` for the selection through the
  current noise pipeline (`CurrentAlgorithmVersion`).
- MUST rebuild the trips affected by the selection (delete + recompute
  `trips` / `trip_points` and reset `trip_detection_state` for the affected
  pair/range), so trips stay consistent with the recomputed processed points.
- MUST support a dry-run mode that reports what would change (counts, affected
  trips, distance deltas) without writing.
- MUST NOT rebuild all trips automatically on every `algorithm_version` bump.

## Notes

- The manual backfill used during EPIC 14 (delete derived rows for a pair, let
  the worker reprocess within `maxLookback`) is the throwaway equivalent of this
  feature; it is not safe as an automatic behavior.
- Consider bounding/locking per pair to avoid concurrent rebuilds, and emitting
  trip lifecycle events for downstream consumers when trips are rebuilt.
