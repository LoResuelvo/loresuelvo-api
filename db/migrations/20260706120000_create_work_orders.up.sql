CREATE TABLE work_orders (
    id SERIAL PRIMARY KEY,
    service_proposal_id INTEGER NOT NULL UNIQUE REFERENCES service_proposals(id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL,
    accepted_on TIMESTAMP NOT NULL,
    updated_on TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT work_orders_status_check CHECK (status IN ('scheduled'))
);

