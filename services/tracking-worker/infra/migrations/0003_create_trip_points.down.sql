-- WARNING: This will delete all trip-to-point associations.
-- Only run in development. Never run against production without a backup.
DROP TABLE IF EXISTS trip_points CASCADE;
