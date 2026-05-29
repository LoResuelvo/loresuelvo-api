CREATE TABLE conversations (
    id SERIAL PRIMARY KEY,
    consumer_id INTEGER NOT NULL REFERENCES consumers(id) ON DELETE CASCADE,
    provider_id INTEGER NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL,
    created_on TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_on TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT conversations_status_check CHECK (status IN ('pending', 'active', 'rejected')),
    CONSTRAINT conversations_consumer_provider_unique UNIQUE (consumer_id, provider_id)
);

CREATE INDEX conversations_consumer_id_idx
    ON conversations (consumer_id);

CREATE INDEX conversations_provider_id_idx
    ON conversations (provider_id);
