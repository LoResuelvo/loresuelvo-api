ALTER TABLE identity_verification_sessions
    ADD COLUMN risk_codes TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN last_result_on TIMESTAMPTZ,
    ADD COLUMN verified_on TIMESTAMPTZ;

ALTER TABLE identity_verification_sessions
    DROP CONSTRAINT identity_verification_sessions_status_check,
    ADD CONSTRAINT identity_verification_sessions_status_check CHECK (status IN (
        'not_started', 'in_progress', 'awaiting_user', 'in_review', 'approved',
        'declined', 'resubmitted', 'abandoned', 'expired', 'kyc_expired'
    ));
