ALTER TABLE service_proposals
    DROP CONSTRAINT IF EXISTS service_proposals_platform_fee_due_now_check,
    DROP CONSTRAINT IF EXISTS service_proposals_platform_fee_total_check,
    DROP CONSTRAINT IF EXISTS service_proposals_deposit_check,
    DROP CONSTRAINT IF EXISTS service_proposals_currency_check,
    DROP COLUMN IF EXISTS platform_fee_due_now_cents,
    DROP COLUMN IF EXISTS platform_fee_total_cents,
    DROP COLUMN IF EXISTS deposit_cents,
    DROP COLUMN IF EXISTS currency;
