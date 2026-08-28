UPDATE identity_verification_sessions SET status = 'awaiting_user' WHERE status = 'in_review';

ALTER TABLE identity_verification_sessions
    DROP CONSTRAINT identity_verification_sessions_status_check,
    ADD CONSTRAINT identity_verification_sessions_status_check CHECK (status IN ('not_started', 'in_progress', 'awaiting_user', 'approved'));
