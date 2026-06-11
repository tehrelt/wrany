CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE raw_location_points (
    user_id     UUID             NOT NULL,
    device_id   UUID             NOT NULL,
    event_id    TEXT             NOT NULL,

    recorded_at TIMESTAMPTZ      NOT NULL,
    received_at TIMESTAMPTZ      NOT NULL,
    stored_at   TIMESTAMPTZ      NOT NULL DEFAULT now(),

    lat         DOUBLE PRECISION NOT NULL CHECK (lat >= -90  AND lat <= 90),
    lon         DOUBLE PRECISION NOT NULL CHECK (lon >= -180 AND lon <= 180),
    -- geom stores (lon, lat) as (X, Y): ST_MakePoint(lon, lat), SRID 4326.
    -- ST_X(geom) = lon, ST_Y(geom) = lat. Never swap the order.
    geom        geometry(Point, 4326) NOT NULL,

    accuracy_m  DOUBLE PRECISION NOT NULL CHECK (accuracy_m >= 0),
    speed_mps   DOUBLE PRECISION NULL     CHECK (speed_mps  IS NULL OR speed_mps  >= 0),
    bearing_deg DOUBLE PRECISION NULL     CHECK (bearing_deg IS NULL OR (bearing_deg >= 0 AND bearing_deg <= 360)),

    activity_type        TEXT             NOT NULL,
    activity_confidence  DOUBLE PRECISION NULL     CHECK (activity_confidence IS NULL OR (activity_confidence >= 0 AND activity_confidence <= 1)),
    battery_level        DOUBLE PRECISION NULL     CHECK (battery_level       IS NULL OR (battery_level       >= 0 AND battery_level       <= 1)),

    source TEXT NOT NULL,

    PRIMARY KEY (user_id, device_id, event_id)
);

-- Query path: fetch points for a user/device session ordered by time.
CREATE INDEX idx_raw_location_points_user_device_recorded_at
    ON raw_location_points (user_id, device_id, recorded_at);

-- Query path: time-range scans across all users.
CREATE INDEX idx_raw_location_points_recorded_at
    ON raw_location_points (recorded_at);

-- Spatial queries: nearest-neighbour, bounding-box, etc.
CREATE INDEX idx_raw_location_points_geom
    ON raw_location_points
    USING GIST (geom);
