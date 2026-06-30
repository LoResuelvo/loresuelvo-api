ALTER TABLE message_images
    ADD COLUMN description TEXT NOT NULL DEFAULT '';

ALTER TABLE message_images
    ALTER COLUMN description DROP DEFAULT;

CREATE TABLE problem_assessment_images (
    problem_assessment_id INTEGER NOT NULL REFERENCES problem_assessments(id) ON DELETE CASCADE,
    file_id UUID NOT NULL REFERENCES files(id),
    position SMALLINT NOT NULL CHECK (position >= 0 AND position < 3),
    PRIMARY KEY (problem_assessment_id, file_id),
    CONSTRAINT problem_assessment_images_position_unique UNIQUE (problem_assessment_id, position)
);

CREATE INDEX problem_assessment_images_assessment_id_idx
    ON problem_assessment_images (problem_assessment_id);

ALTER TABLE job_request_images
    DROP CONSTRAINT job_request_images_file_id_key;
