CREATE TABLE google_calendar_authorization_attempts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    state_digest BYTEA NOT NULL UNIQUE,
    code_verifier_ciphertext BYTEA NOT NULL,
    expires_on TIMESTAMPTZ NOT NULL,
    consumed_on TIMESTAMPTZ,
    created_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT google_calendar_authorization_attempts_state_digest_check
        CHECK (octet_length(state_digest) = 32)
);

CREATE INDEX google_calendar_authorization_attempts_user_id_idx
    ON google_calendar_authorization_attempts(user_id);

CREATE TABLE google_calendar_connections (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token_ciphertext BYTEA NOT NULL,
    calendar_id VARCHAR(255) NOT NULL,
    status VARCHAR(30) NOT NULL,
    connected_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT google_calendar_connections_user_unique
        UNIQUE (user_id),
    CONSTRAINT google_calendar_connections_calendar_check
        CHECK (length(btrim(calendar_id)) > 0),
    CONSTRAINT google_calendar_connections_status_check
        CHECK (status IN ('connected', 'action_required'))
);
