CREATE TABLE work_order_calendar_events (
    work_order_id INTEGER NOT NULL REFERENCES work_orders(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    calendar_id VARCHAR(255) NOT NULL,
    google_event_id VARCHAR(1024),
    attempt_count INTEGER NOT NULL DEFAULT 0,
    retry_after TIMESTAMPTZ,
    last_attempted_on TIMESTAMPTZ,
    last_error_code VARCHAR(100),
    synced_on TIMESTAMPTZ,
    failed_on TIMESTAMPTZ,
    created_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (work_order_id, user_id),
    CONSTRAINT work_order_calendar_events_calendar_id_check
        CHECK (length(btrim(calendar_id)) > 0),
    CONSTRAINT work_order_calendar_events_google_event_id_check
        CHECK (google_event_id IS NULL OR length(btrim(google_event_id)) > 0),
    CONSTRAINT work_order_calendar_events_attempt_count_check
        CHECK (attempt_count >= 0),
    CONSTRAINT work_order_calendar_events_last_error_code_check
        CHECK (last_error_code IS NULL OR length(btrim(last_error_code)) > 0)
);
