CREATE TABLE work_orders (
    id SERIAL PRIMARY KEY,
    service_proposal_id INTEGER NOT NULL UNIQUE REFERENCES service_proposals(id) ON DELETE CASCADE,
    consumer_id INTEGER NOT NULL REFERENCES consumers(user_id) ON DELETE CASCADE,
    provider_id INTEGER NOT NULL REFERENCES providers(user_id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL,
    accepted_on TIMESTAMP NOT NULL,
    updated_on TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT work_orders_status_check CHECK (status IN ('scheduled'))
);

CREATE INDEX work_orders_consumer_id_idx ON work_orders (consumer_id);
CREATE INDEX work_orders_provider_id_idx ON work_orders (provider_id);
