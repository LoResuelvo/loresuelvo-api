CREATE TABLE realtime_events (
    id TEXT PRIMARY KEY,
    target_auth_id TEXT NOT NULL,
    target_role TEXT NOT NULL,
    target_profile_id INTEGER NOT NULL CHECK (target_profile_id > 0),
    payload BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX realtime_events_created_at_idx
    ON realtime_events (created_at);
