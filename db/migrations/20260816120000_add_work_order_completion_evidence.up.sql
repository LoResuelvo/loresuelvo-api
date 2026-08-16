ALTER TABLE work_orders
    DROP CONSTRAINT work_orders_status_check;

ALTER TABLE work_orders
    ADD COLUMN paid_on TIMESTAMP,
    ADD CONSTRAINT work_orders_status_check
        CHECK (status IN ('scheduled', 'awaiting_payment', 'paid'));

CREATE TABLE work_order_completion_reports (
    id SERIAL PRIMARY KEY,
    work_order_id INTEGER NOT NULL UNIQUE REFERENCES work_orders(id) ON DELETE CASCADE,
    description TEXT NOT NULL,
    reported_on TIMESTAMP NOT NULL,
    CONSTRAINT work_order_completion_reports_description_not_empty_check
        CHECK (length(btrim(description)) > 0)
);

CREATE TABLE work_order_completion_images (
    completion_report_id INTEGER NOT NULL REFERENCES work_order_completion_reports(id) ON DELETE CASCADE,
    file_id UUID NOT NULL UNIQUE REFERENCES files(id) ON DELETE CASCADE,
    position SMALLINT NOT NULL CHECK (position >= 0 AND position < 3),
    PRIMARY KEY (completion_report_id, file_id),
    CONSTRAINT work_order_completion_images_report_position_unique
        UNIQUE (completion_report_id, position)
);

CREATE INDEX work_order_completion_reports_work_order_id_idx
    ON work_order_completion_reports (work_order_id);

CREATE INDEX work_order_completion_images_completion_report_id_idx
    ON work_order_completion_images (completion_report_id);
