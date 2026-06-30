DROP TABLE IF EXISTS problem_assessment_images;

ALTER TABLE job_request_images
    DROP CONSTRAINT IF EXISTS job_request_images_file_id_key;

ALTER TABLE job_request_images
    ADD CONSTRAINT job_request_images_file_id_key UNIQUE (file_id);

ALTER TABLE message_images
    DROP COLUMN IF EXISTS description;
