ALTER TABLE service_proposals
    ADD COLUMN currency VARCHAR(3) NOT NULL,
    ADD COLUMN deposit_cents BIGINT NOT NULL,
    ADD COLUMN platform_fee_total_cents BIGINT NOT NULL,
    ADD COLUMN platform_fee_due_now_cents BIGINT NOT NULL,
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
