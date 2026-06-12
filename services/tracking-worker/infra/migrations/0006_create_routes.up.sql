CREATE TABLE routes (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL,
    device_id    UUID,
    name         TEXT,
    status       TEXT NOT NULL DEFAULT 'active',

    start_lat    DOUBLE PRECISION NOT NULL,
    start_lon    DOUBLE PRECISION NOT NULL,
    end_lat      DOUBLE PRECISION NOT NULL,
    end_lon      DOUBLE PRECISION NOT NULL,

    distance_m   DOUBLE PRECISION NOT NULL DEFAULT 0,
    trips_count  INTEGER          NOT NULL DEFAULT 0,

    template_geom geometry(LineString, 4326) NOT NULL,

    -- Generated stored columns for spatial index (safer than expression indexes).
    start_geom geometry(Point, 4326) GENERATED ALWAYS AS (ST_SetSRID(ST_MakePoint(start_lon, start_lat), 4326)) STORED,
    end_geom   geometry(Point, 4326) GENERATED ALWAYS AS (ST_SetSRID(ST_MakePoint(end_lon,   end_lat),   4326)) STORED,

    first_trip_id UUID NOT NULL REFERENCES trips(id),
    last_trip_id  UUID NOT NULL REFERENCES trips(id),

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_routes_user_updated_at ON routes (user_id, updated_at DESC);
CREATE INDEX idx_routes_start_geom      ON routes USING GIST (start_geom);
CREATE INDEX idx_routes_end_geom        ON routes USING GIST (end_geom);
CREATE INDEX idx_routes_template_geom   ON routes USING GIST (template_geom);
