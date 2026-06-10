CREATE TABLE ingested_location_events (
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id   UUID        NOT NULL,
    event_id    TEXT        NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, device_id, event_id)
);

CREATE INDEX idx_ingested_location_events_received_at
    ON ingested_location_events (received_at);
