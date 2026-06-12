CREATE TABLE trip_detection_state (
    user_id     UUID NOT NULL,
    device_id   UUID NOT NULL,

    -- Current state machine state for this (user, device) pair.
    state       TEXT NOT NULL DEFAULT 'IDLE'
                CHECK (state IN ('IDLE', 'MOTION_CANDIDATE', 'TRIP_ACTIVE', 'STOP_CANDIDATE')),

    -- Set while state = TRIP_ACTIVE or STOP_CANDIDATE.
    active_trip_id              UUID        NULL REFERENCES trips(id),

    -- Set while state = MOTION_CANDIDATE: when candidate window opened.
    candidate_started_at        TIMESTAMPTZ NULL,
    -- event_id of the first point that triggered MOTION_CANDIDATE.
    candidate_start_point_id    TEXT        NULL,

    -- Set while state = STOP_CANDIDATE: when the stop began.
    stop_started_at             TIMESTAMPTZ NULL,
    -- Geographic center of the stop zone (first point of the stop).
    stop_center_lat             DOUBLE PRECISION NULL CHECK (stop_center_lat IS NULL OR (stop_center_lat >= -90  AND stop_center_lat <= 90)),
    stop_center_lon             DOUBLE PRECISION NULL CHECK (stop_center_lon IS NULL OR (stop_center_lon >= -180 AND stop_center_lon <= 180)),

    -- recorded_at of the last point fed into the state machine.
    -- Used on restart to avoid re-processing already-seen points before the watermark.
    last_processed_recorded_at  TIMESTAMPTZ NULL,

    -- Watermark-based checkpoint.
    -- Detection query: recorded_at >= last_watermark_at AND recorded_at < now() - late_arrival_window_sec.
    -- After a successful run: last_watermark_at = now() - late_arrival_window_sec.
    -- Never advance past now() - late_arrival_window_sec to preserve the late-arrival window.
    last_watermark_at           TIMESTAMPTZ NULL,
    late_arrival_window_sec     INTEGER     NOT NULL DEFAULT 300 CHECK (late_arrival_window_sec >= 0),

    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (user_id, device_id)
);
