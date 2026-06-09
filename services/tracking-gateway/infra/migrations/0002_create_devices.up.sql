CREATE TABLE devices (
    id           UUID        PRIMARY KEY,
    user_id      UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id    UUID        NOT NULL,
    name         TEXT,
    platform     TEXT,
    last_seen_at TIMESTAMPTZ NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL,
    CONSTRAINT devices_user_device_unique UNIQUE (user_id, device_id)
);
