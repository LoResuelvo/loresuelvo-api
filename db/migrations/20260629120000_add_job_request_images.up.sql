CREATE TABLE job_request_images (
    job_request_id INTEGER NOT NULL REFERENCES job_requests(id) ON DELETE CASCADE,
    file_id UUID NOT NULL UNIQUE REFERENCES files(id),
    position SMALLINT NOT NULL CHECK (position >= 0),
    PRIMARY KEY (job_request_id, file_id),
    CONSTRAINT job_request_images_request_position_unique UNIQUE (job_request_id, position)
);

CREATE INDEX job_request_images_job_request_id_idx
    ON job_request_images (job_request_id);
