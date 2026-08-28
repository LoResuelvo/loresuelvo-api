ALTER TABLE identity_verification_sessions
    DROP CONSTRAINT identity_verification_sessions_status_check,
    ADD COLUMN risk_codes TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN last_result_on TIMESTAMPTZ,
    ADD COLUMN verified_on TIMESTAMPTZ,
    ADD CONSTRAINT identity_verification_sessions_status_check CHECK (status IN (
        'not_started', 'in_progress', 'awaiting_user', 'in_review', 'approved',
        'declined', 'resubmitted', 'abandoned', 'expired', 'kyc_expired'
    ));

CREATE TABLE identity_verification_events (
    external_event_id UUID PRIMARY KEY,
    external_session_id UUID NOT NULL REFERENCES identity_verification_sessions(external_session_id) ON DELETE CASCADE,
    event_type VARCHAR(50) NOT NULL,
    occurred_on TIMESTAMPTZ NOT NULL,
    received_on TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX identity_verification_events_session_idx
    ON identity_verification_events (external_session_id, occurred_on DESC);
