DROP TABLE IF EXISTS work_order_completion_images;
DROP TABLE IF EXISTS work_order_completion_reports;

ALTER TABLE work_orders
    DROP CONSTRAINT work_orders_status_check,
    DROP COLUMN paid_on;

ALTER TABLE work_orders
    ADD CONSTRAINT work_orders_status_check
        CHECK (status IN ('scheduled', 'paid'));
