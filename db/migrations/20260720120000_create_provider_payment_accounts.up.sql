CREATE TABLE payment_account_authorization_attempts (
    id SERIAL PRIMARY KEY,
    provider_id INTEGER NOT NULL REFERENCES providers(user_id) ON DELETE CASCADE,
    payment_provider VARCHAR(50) NOT NULL,
    state_digest BYTEA NOT NULL UNIQUE,
    code_verifier_ciphertext BYTEA NOT NULL,
    expires_on TIMESTAMPTZ NOT NULL,
    consumed_on TIMESTAMPTZ,
    created_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT payment_account_authorization_attempts_provider_check
        CHECK (length(btrim(payment_provider)) > 0),
    CONSTRAINT payment_account_authorization_attempts_state_digest_check
        CHECK (octet_length(state_digest) = 32)
);

CREATE INDEX payment_account_authorization_attempts_provider_id_idx
    ON payment_account_authorization_attempts(provider_id);

CREATE TABLE provider_payment_accounts (
    id SERIAL PRIMARY KEY,
    provider_id INTEGER NOT NULL REFERENCES providers(user_id) ON DELETE CASCADE,
    payment_provider VARCHAR(50) NOT NULL,
    external_account_id VARCHAR(255) NOT NULL,
    access_token_ciphertext BYTEA NOT NULL,
    refresh_token_ciphertext BYTEA,
    token_expires_on TIMESTAMPTZ NOT NULL,
    can_receive_marketplace_payments BOOLEAN NOT NULL DEFAULT FALSE,
    connected_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT provider_payment_accounts_provider_check
        CHECK (length(btrim(payment_provider)) > 0),
    CONSTRAINT provider_payment_accounts_provider_unique
        UNIQUE (provider_id, payment_provider),
    CONSTRAINT provider_payment_accounts_external_unique
        UNIQUE (payment_provider, external_account_id)
);
