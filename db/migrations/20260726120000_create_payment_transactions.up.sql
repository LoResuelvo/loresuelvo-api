CREATE TABLE payment_transactions (
    id BIGSERIAL PRIMARY KEY,
    payment_intent_id UUID NOT NULL REFERENCES payment_intents(id) ON DELETE CASCADE,
    processor VARCHAR(50) NOT NULL,
    external_payment_id VARCHAR(255) NOT NULL,
    seller_account_id VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL,
    currency VARCHAR(3) NOT NULL,
    amount_cents BIGINT NOT NULL,
    verified_on TIMESTAMPTZ NOT NULL,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    CONSTRAINT payment_transactions_processor_check
        CHECK (processor = 'mercado_pago'),
    CONSTRAINT payment_transactions_external_payment_unique
        UNIQUE (processor, external_payment_id),
    CONSTRAINT payment_transactions_status_check
        CHECK (status IN ('approved', 'processing', 'rejected', 'refunded', 'disputed')),
    CONSTRAINT payment_transactions_currency_not_empty_check
        CHECK (length(btrim(currency)) > 0),
    CONSTRAINT payment_transactions_amount_positive_check
        CHECK (amount_cents > 0)
);

CREATE INDEX payment_transactions_intent_id_idx
    ON payment_transactions (payment_intent_id);
