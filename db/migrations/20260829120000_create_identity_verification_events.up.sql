CREATE TABLE identity_verification_events (
    external_event_id UUID PRIMARY KEY,
    external_session_id UUID NOT NULL REFERENCES identity_verification_sessions(external_session_id) ON DELETE CASCADE,
    occurred_on TIMESTAMPTZ NOT NULL,
    received_on TIMESTAMPTZ NOT NULL
);

CREATE INDEX identity_verification_events_session_occurred_idx
    ON identity_verification_events (external_session_id, occurred_on DESC);
