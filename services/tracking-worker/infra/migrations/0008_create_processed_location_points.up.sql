CREATE TABLE processed_location_points (
    user_id     UUID NOT NULL,
    device_id   UUID NOT NULL,
    event_id    TEXT NOT NULL,

    raw_lat     DOUBLE PRECISION NOT NULL,
    raw_lon     DOUBLE PRECISION NOT NULL,
    filtered_lat DOUBLE PRECISION NULL,
    filtered_lon DOUBLE PRECISION NULL,
    filtered_geom geometry(Point, 4326) NULL,

    accuracy_m        DOUBLE PRECISION NOT NULL,
    speed_mps         DOUBLE PRECISION NULL,
    implied_speed_mps DOUBLE PRECISION NOT NULL DEFAULT 0,
    distance_delta_m  DOUBLE PRECISION NOT NULL DEFAULT 0,

    is_accepted   BOOLEAN NOT NULL,
    is_outlier    BOOLEAN NOT NULL,
    is_stationary BOOLEAN NOT NULL,
    noise_reason  TEXT NOT NULL DEFAULT '',
    stationary_since TIMESTAMPTZ NULL,

    recorded_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (user_id, device_id, event_id),
    FOREIGN KEY (user_id, device_id, event_id)
        REFERENCES raw_location_points(user_id, device_id, event_id)
        ON DELETE CASCADE
);

CREATE INDEX idx_processed_location_points_pending
    ON processed_location_points (user_id, device_id, recorded_at, event_id);

CREATE INDEX idx_processed_location_points_accepted
    ON processed_location_points (user_id, device_id, recorded_at)
    WHERE is_accepted;

CREATE INDEX idx_processed_location_points_geom
    ON processed_location_points USING GIST (filtered_geom)
    WHERE filtered_geom IS NOT NULL;

ALTER TABLE trip_detection_state
    ADD COLUMN candidate_good_points INTEGER NOT NULL DEFAULT 0;

ALTER TABLE trip_detection_state
    ALTER COLUMN late_arrival_window_sec SET DEFAULT 45;
