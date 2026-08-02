ALTER TABLE work_orders
    DROP CONSTRAINT work_orders_payment_completion_check;

UPDATE work_orders
SET status = 'scheduled',
    completion_code_ciphertext = NULL,
    fully_paid_on = NULL
WHERE status = 'paid';

ALTER TABLE work_orders
    DROP CONSTRAINT work_orders_status_check;

ALTER TABLE work_orders
    ADD CONSTRAINT work_orders_status_check
        CHECK (status IN ('scheduled'));

ALTER TABLE work_orders
    DROP COLUMN completion_code_ciphertext,
    DROP COLUMN fully_paid_on;
