CREATE TABLE problem_assessments (
    id SERIAL PRIMARY KEY,
    chatbot_conversation_id INTEGER NOT NULL REFERENCES chatbot_conversations(conversation_id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    outcome TEXT NOT NULL,
    problem_category_id INTEGER NULL REFERENCES categories(id),
    problem_title TEXT NOT NULL DEFAULT '',
    problem_description TEXT NOT NULL DEFAULT '',
    based_on_message_id INTEGER NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    created_on TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT problem_assessments_version_positive_check CHECK (version > 0),
    CONSTRAINT problem_assessments_outcome_check CHECK (outcome IN ('collecting_information', 'self_service', 'professional_required')),
    CONSTRAINT problem_assessments_shape_check CHECK (
        (outcome = 'collecting_information' AND problem_category_id IS NULL AND btrim(problem_title) = '' AND btrim(problem_description) = '')
        OR
        (outcome = 'self_service' AND btrim(problem_title) <> '' AND btrim(problem_description) <> '')
        OR
        (outcome = 'professional_required' AND problem_category_id IS NOT NULL AND btrim(problem_title) <> '' AND btrim(problem_description) <> '')
    ),
    CONSTRAINT problem_assessments_conversation_version_unique UNIQUE (chatbot_conversation_id, version)
);

CREATE INDEX problem_assessments_chatbot_conversation_id_idx
    ON problem_assessments (chatbot_conversation_id);

ALTER TABLE chatbot_conversations
    ADD COLUMN current_assessment_id INTEGER NULL REFERENCES problem_assessments(id) ON DELETE SET NULL;

ALTER TABLE job_requests
    ADD COLUMN source_assessment_id INTEGER NULL REFERENCES problem_assessments(id);

CREATE INDEX job_requests_source_assessment_id_idx
    ON job_requests (source_assessment_id);

ALTER TABLE chatbot_conversations
    DROP COLUMN diagnosis_completed,
    DROP COLUMN recommended_category_id;
