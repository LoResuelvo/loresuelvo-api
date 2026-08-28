CREATE TABLE identity_verification_sessions (
    external_session_id UUID PRIMARY KEY,
    provider_id INTEGER NOT NULL REFERENCES providers(user_id) ON DELETE CASCADE,
    verifier VARCHAR(50) NOT NULL,
    workflow_id UUID NOT NULL,
    workflow_version INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL,
    risk_codes TEXT[] NOT NULL DEFAULT '{}',
    last_result_on TIMESTAMPTZ,
    verified_on TIMESTAMPTZ,
    created_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT identity_verification_sessions_status_check CHECK (status IN (
        'not_started', 'in_progress', 'awaiting_user', 'in_review', 'approved',
        'declined', 'resubmitted', 'abandoned', 'expired', 'kyc_expired'
    )),
    CONSTRAINT identity_verification_sessions_workflow_version_check CHECK (workflow_version >= 0)
);

CREATE INDEX identity_verification_sessions_provider_created_idx
    ON identity_verification_sessions (provider_id, created_on DESC);

CREATE TABLE identity_verification_events (
    external_event_id UUID PRIMARY KEY,
    external_session_id UUID NOT NULL REFERENCES identity_verification_sessions(external_session_id) ON DELETE CASCADE,
    event_type VARCHAR(50) NOT NULL,
    occurred_on TIMESTAMPTZ NOT NULL,
    received_on TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX identity_verification_events_session_idx
    ON identity_verification_events (external_session_id, occurred_on DESC);
