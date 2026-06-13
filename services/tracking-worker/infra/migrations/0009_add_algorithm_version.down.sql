DROP INDEX IF EXISTS idx_processed_location_points_algo_version;

ALTER TABLE processed_location_points
    DROP COLUMN IF EXISTS algorithm_version;
