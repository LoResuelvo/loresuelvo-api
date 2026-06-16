DROP INDEX job_requests_open_consumer_provider_unique_idx;
DROP INDEX job_requests_open_conversation_unique_idx;

ALTER TABLE job_requests
    ADD CONSTRAINT job_requests_conversation_unique UNIQUE (conversation_id),
    ADD CONSTRAINT job_requests_consumer_provider_unique UNIQUE (consumer_id, provider_id);

ALTER TABLE job_requests
    DROP CONSTRAINT job_requests_status_check,
    DROP COLUMN status;
