ALTER TABLE service_proposals
    ADD COLUMN currency VARCHAR(3),
    ADD COLUMN deposit_cents BIGINT,
    ADD COLUMN platform_fee_total_cents BIGINT,
    ADD COLUMN platform_fee_due_now_cents BIGINT;

UPDATE service_proposals
SET
    currency = 'ARS',
    deposit_cents = ROUND(amount_cents * 0.20)::BIGINT,
    platform_fee_total_cents = 500000,
    platform_fee_due_now_cents = 100000;

ALTER TABLE service_proposals
    ALTER COLUMN currency SET NOT NULL,
    ALTER COLUMN deposit_cents SET NOT NULL,
    ALTER COLUMN platform_fee_total_cents SET NOT NULL,
    ALTER COLUMN platform_fee_due_now_cents SET NOT NULL,
    ADD CONSTRAINT service_proposals_currency_check
        CHECK (currency = 'ARS'),
    ADD CONSTRAINT service_proposals_deposit_check
        CHECK (deposit_cents > 0 AND deposit_cents <= amount_cents),
    ADD CONSTRAINT service_proposals_platform_fee_total_check
        CHECK (platform_fee_total_cents >= 0),
    ADD CONSTRAINT service_proposals_platform_fee_due_now_check
        CHECK (
            platform_fee_due_now_cents >= 0
            AND platform_fee_due_now_cents <= platform_fee_total_cents
        );
