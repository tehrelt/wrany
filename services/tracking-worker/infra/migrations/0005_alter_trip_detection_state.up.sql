-- Add columns required for correct cross-batch state machine behaviour.
-- candidate_start_lat/lon: start position of a trip (set when entering MOTION_CANDIDATE).
-- candidate_distance_m: accumulated distance during MOTION_CANDIDATE, needed across batches.
-- last_point_lat/lon: position of the last accepted point, needed for distance and GPS-jump detection.
ALTER TABLE trip_detection_state
    ADD COLUMN candidate_distance_m  DOUBLE PRECISION NOT NULL DEFAULT 0,
    ADD COLUMN candidate_start_lat   DOUBLE PRECISION NULL
                                     CHECK (candidate_start_lat IS NULL OR (candidate_start_lat >= -90  AND candidate_start_lat <= 90)),
    ADD COLUMN candidate_start_lon   DOUBLE PRECISION NULL
                                     CHECK (candidate_start_lon IS NULL OR (candidate_start_lon >= -180 AND candidate_start_lon <= 180)),
    ADD COLUMN last_point_lat        DOUBLE PRECISION NULL
                                     CHECK (last_point_lat IS NULL OR (last_point_lat >= -90  AND last_point_lat <= 90)),
    ADD COLUMN last_point_lon        DOUBLE PRECISION NULL
                                     CHECK (last_point_lon IS NULL OR (last_point_lon >= -180 AND last_point_lon <= 180));
