-- WARNING: This will delete all detection state and watermark checkpoints.
-- After dropping this table the worker will reprocess all raw points from scratch on next start,
-- which may create duplicate trips if trips and trip_points tables still exist.
-- Only run in development. Never run against production without a backup.
DROP TABLE IF EXISTS trip_detection_state CASCADE;
