UPDATE identity_verification_sessions
SET status = 'in_review'
WHERE status IN ('declined', 'resubmitted', 'abandoned', 'expired', 'kyc_expired');

ALTER TABLE identity_verification_sessions
    DROP CONSTRAINT identity_verification_sessions_status_check,
    ADD CONSTRAINT identity_verification_sessions_status_check CHECK (status IN (
        'not_started', 'in_progress', 'awaiting_user', 'in_review', 'approved'
    ));

ALTER TABLE identity_verification_sessions
    DROP COLUMN risk_codes,
    DROP COLUMN last_result_on,
    DROP COLUMN verified_on;
