-- WARNING: This will delete all detected trips and cascade-delete trip_points.
-- Only run in development. Never run against production without a backup.
DROP TABLE IF EXISTS trips CASCADE;
