CREATE TABLE trip_points (
    trip_id     UUID        NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    user_id     UUID        NOT NULL,
    device_id   UUID        NOT NULL,
    -- event_id matches raw_location_points.event_id for the same (user_id, device_id).
    event_id    TEXT        NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL,

    -- A trip owns each (trip_id, event_id) pair exactly once.
    PRIMARY KEY (trip_id, event_id),

    -- A raw point can belong to at most one trip per user/device.
    -- This is the idempotency guard: re-running detection cannot insert the same
    -- raw point into a second trip row.
    CONSTRAINT uq_trip_points_user_device_event UNIQUE (user_id, device_id, event_id)
);

-- Fetch ordered points for a trip (polyline rendering, stats).
CREATE INDEX idx_trip_points_trip_recorded_at
    ON trip_points (trip_id, recorded_at);
