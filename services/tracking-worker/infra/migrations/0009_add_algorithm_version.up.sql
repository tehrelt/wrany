-- Stamp every processing result with the algorithm version that produced it.
-- Enables reprocessing of rows produced by an older algorithm version without
-- touching raw_location_points (the immutable source of truth).
ALTER TABLE processed_location_points
    ADD COLUMN algorithm_version SMALLINT NOT NULL DEFAULT 1;

-- Supports the reprocess scan: find accepted-or-not rows whose algorithm_version
-- is behind the current one for a given pair, ordered by recorded_at.
CREATE INDEX idx_processed_location_points_algo_version
    ON processed_location_points (user_id, device_id, algorithm_version, recorded_at);
