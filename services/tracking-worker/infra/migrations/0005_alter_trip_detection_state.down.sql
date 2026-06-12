-- WARNING: Removes cross-batch state columns. After this rollback the worker will lose
-- candidate position and last-point context on restart, causing incorrect detection.
-- Only run in development. Never run against production without a backup.
ALTER TABLE trip_detection_state
    DROP COLUMN IF EXISTS candidate_distance_m,
    DROP COLUMN IF EXISTS candidate_start_lat,
    DROP COLUMN IF EXISTS candidate_start_lon,
    DROP COLUMN IF EXISTS last_point_lat,
    DROP COLUMN IF EXISTS last_point_lon;
