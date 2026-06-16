-- Persist activity on processed points so the /track route can expose it.
-- The worker already computes ActivityType/ActivityConfidence; until now they
-- were dropped on write, leaving the track route's activity always "unknown".

ALTER TABLE processed_location_points
    ADD COLUMN IF NOT EXISTS activity_type text NOT NULL DEFAULT 'unknown',
    ADD COLUMN IF NOT EXISTS activity_confidence double precision;

-- Backfill from the immutable raw rows (FK guarantees a match) so historical
-- routes get activity without a full reprocess.
UPDATE processed_location_points p
SET activity_type = COALESCE(NULLIF(r.activity_type, ''), 'unknown'),
    activity_confidence = r.activity_confidence
FROM raw_location_points r
WHERE p.user_id = r.user_id
  AND p.device_id = r.device_id
  AND p.event_id = r.event_id;
