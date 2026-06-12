CREATE TABLE trips (
    id          UUID             PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID             NOT NULL,
    device_id   UUID             NOT NULL,

    status      TEXT             NOT NULL
                                 CHECK (status IN ('TRIP_ACTIVE', 'TRIP_COMPLETED')),

    started_at  TIMESTAMPTZ      NOT NULL,
    ended_at    TIMESTAMPTZ      NULL,

    start_lat   DOUBLE PRECISION NOT NULL CHECK (start_lat >= -90  AND start_lat <= 90),
    start_lon   DOUBLE PRECISION NOT NULL CHECK (start_lon >= -180 AND start_lon <= 180),
    end_lat     DOUBLE PRECISION NULL     CHECK (end_lat IS NULL OR (end_lat >= -90  AND end_lat <= 90)),
    end_lon     DOUBLE PRECISION NULL     CHECK (end_lon IS NULL OR (end_lon >= -180 AND end_lon <= 180)),

    distance_m    DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (distance_m >= 0),
    duration_sec  BIGINT           NOT NULL DEFAULT 0 CHECK (duration_sec >= 0),
    points_count  INTEGER          NOT NULL DEFAULT 0 CHECK (points_count >= 0),

    created_at  TIMESTAMPTZ      NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ      NOT NULL DEFAULT now()
);

-- Primary query path: list trips for a user, ordered by time.
CREATE INDEX idx_trips_user_device_started_at
    ON trips (user_id, device_id, started_at DESC);

-- Filter by status (e.g. find all TRIP_ACTIVE for recovery on restart).
CREATE INDEX idx_trips_user_status
    ON trips (user_id, status)
    WHERE status = 'TRIP_ACTIVE';
