CREATE TABLE service_proposals (
    id SERIAL PRIMARY KEY,
    consumer_id INTEGER NOT NULL REFERENCES consumers(id) ON DELETE CASCADE,
    provider_id INTEGER NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    conversation_id INTEGER NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    amount_cents BIGINT NOT NULL,
    scheduled_on TIMESTAMP NOT NULL,
    description TEXT NOT NULL,
    status VARCHAR(50) NOT NULL,
    created_on TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_on TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT service_proposals_amount_cents_positive_check CHECK (amount_cents > 0),
    CONSTRAINT service_proposals_description_not_empty_check CHECK (length(btrim(description)) > 0),
    CONSTRAINT service_proposals_status_check CHECK (status IN ('pending', 'accepted', 'rejected'))
);

CREATE INDEX service_proposals_consumer_id_idx
    ON service_proposals (consumer_id);

CREATE INDEX service_proposals_provider_id_idx
    ON service_proposals (provider_id);

CREATE INDEX service_proposals_conversation_id_idx
    ON service_proposals (conversation_id);
