ALTER TABLE processed_location_points
    DROP COLUMN IF EXISTS activity_type,
    DROP COLUMN IF EXISTS activity_confidence;
