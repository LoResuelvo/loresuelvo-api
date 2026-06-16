ALTER TABLE job_requests
    ADD COLUMN status VARCHAR(50) NOT NULL DEFAULT 'pending',
    ADD CONSTRAINT job_requests_status_check CHECK (status IN ('pending', 'accepted'));

ALTER TABLE job_requests
    ALTER COLUMN status DROP DEFAULT;

ALTER TABLE job_requests
    DROP CONSTRAINT job_requests_conversation_unique,
    DROP CONSTRAINT job_requests_consumer_provider_unique;

CREATE UNIQUE INDEX job_requests_open_conversation_unique_idx
    ON job_requests (conversation_id)
    WHERE status IN ('pending', 'accepted');

CREATE UNIQUE INDEX job_requests_open_consumer_provider_unique_idx
    ON job_requests (consumer_id, provider_id)
    WHERE status IN ('pending', 'accepted');
