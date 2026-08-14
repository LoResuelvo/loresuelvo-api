ALTER TABLE work_orders
    DROP CONSTRAINT work_orders_status_check;

ALTER TABLE work_orders
    ADD CONSTRAINT work_orders_status_check
        CHECK (status IN ('scheduled', 'paid'));
