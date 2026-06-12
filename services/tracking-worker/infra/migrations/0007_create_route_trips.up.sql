CREATE TABLE route_trips (
    route_id     UUID NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
    trip_id      UUID NOT NULL REFERENCES trips(id)  ON DELETE CASCADE,
    user_id      UUID NOT NULL,
    device_id    UUID NOT NULL,
    match_score  DOUBLE PRECISION NOT NULL,
    matched_at   TIMESTAMPTZ      NOT NULL DEFAULT now(),
    duration_sec BIGINT           NOT NULL,
    distance_m   DOUBLE PRECISION NOT NULL,

    PRIMARY KEY (route_id, trip_id),
    -- One trip belongs to exactly one route.
    UNIQUE (trip_id)
);

CREATE INDEX idx_route_trips_route_id ON route_trips (route_id, matched_at DESC);
