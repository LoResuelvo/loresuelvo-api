CREATE TABLE payment_intents (
    id UUID PRIMARY KEY,
    service_proposal_id INTEGER NOT NULL REFERENCES service_proposals(id) ON DELETE CASCADE,
    purpose VARCHAR(50) NOT NULL,
    currency VARCHAR(3) NOT NULL,
    seller_amount_cents BIGINT NOT NULL,
    platform_fee_cents BIGINT NOT NULL,
    total_amount_cents BIGINT NOT NULL,
    status VARCHAR(50) NOT NULL,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    CONSTRAINT payment_intents_purpose_check
        CHECK (purpose IN ('booking_deposit')),
    CONSTRAINT payment_intents_currency_check
        CHECK (currency = 'ARS'),
    CONSTRAINT payment_intents_amounts_check
        CHECK (
            seller_amount_cents > 0
            AND platform_fee_cents >= 0
            AND total_amount_cents = seller_amount_cents + platform_fee_cents
        ),
    CONSTRAINT payment_intents_status_check
        CHECK (
            status IN (
                'requires_checkout',
                'checkout_ready',
                'processing',
                'paid',
                'expired',
                'cancelled',
                'refunded',
                'disputed',
                'payment_mismatch',
                'rejected'
            )
        )
);

CREATE UNIQUE INDEX payment_intents_active_booking_deposit_unique
    ON payment_intents (service_proposal_id, purpose)
    WHERE status IN ('requires_checkout', 'checkout_ready', 'processing', 'paid');

CREATE TABLE payment_checkout_sessions (
    id BIGSERIAL PRIMARY KEY,
    payment_intent_id UUID NOT NULL REFERENCES payment_intents(id) ON DELETE CASCADE,
    processor VARCHAR(50) NOT NULL,
    external_preference_id VARCHAR(255) NOT NULL,
    checkout_url TEXT NOT NULL,
    created_on TIMESTAMPTZ NOT NULL,
    CONSTRAINT payment_checkout_sessions_processor_check
        CHECK (processor = 'mercado_pago'),
    CONSTRAINT payment_checkout_sessions_external_preference_unique
        UNIQUE (processor, external_preference_id),
    CONSTRAINT payment_checkout_sessions_url_not_empty_check
        CHECK (length(btrim(checkout_url)) > 0)
);

CREATE INDEX payment_checkout_sessions_intent_id_idx
    ON payment_checkout_sessions (payment_intent_id);
