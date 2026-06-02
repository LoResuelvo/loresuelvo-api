CREATE TABLE job_requests (
    id SERIAL PRIMARY KEY,
    consumer_id INTEGER NOT NULL REFERENCES consumers(id) ON DELETE CASCADE,
    provider_id INTEGER NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    conversation_id INTEGER NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_on TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_on TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT job_requests_title_not_empty_check CHECK (length(btrim(title)) > 0),
    CONSTRAINT job_requests_conversation_unique UNIQUE (conversation_id),
    CONSTRAINT job_requests_consumer_provider_unique UNIQUE (consumer_id, provider_id)
);

CREATE INDEX job_requests_consumer_id_idx
    ON job_requests (consumer_id);

CREATE INDEX job_requests_provider_id_idx
    ON job_requests (provider_id);
