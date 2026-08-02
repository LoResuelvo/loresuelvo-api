ALTER TABLE work_orders
    ADD COLUMN completion_code_ciphertext BYTEA,
    ADD COLUMN fully_paid_on TIMESTAMPTZ;

ALTER TABLE work_orders
    DROP CONSTRAINT work_orders_status_check;

ALTER TABLE work_orders
    ADD CONSTRAINT work_orders_status_check
        CHECK (status IN ('scheduled', 'paid')),
    ADD CONSTRAINT work_orders_payment_completion_check
        CHECK (
            (status = 'scheduled' AND completion_code_ciphertext IS NULL AND fully_paid_on IS NULL)
            OR
            (status = 'paid' AND completion_code_ciphertext IS NOT NULL AND octet_length(completion_code_ciphertext) > 0 AND fully_paid_on IS NOT NULL)
        );
