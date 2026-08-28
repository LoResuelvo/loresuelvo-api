DROP TABLE identity_verification_events;

ALTER TABLE identity_verification_sessions
    DROP COLUMN risk_codes,
    DROP COLUMN last_result_on,
    DROP COLUMN verified_on,
    DROP CONSTRAINT identity_verification_sessions_status_check,
    ADD CONSTRAINT identity_verification_sessions_status_check CHECK (status IN ('not_started'));
