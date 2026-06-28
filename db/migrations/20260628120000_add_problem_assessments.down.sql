ALTER TABLE chatbot_conversations
    ADD COLUMN diagnosis_completed BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN recommended_category_id INTEGER NULL REFERENCES categories(id);

UPDATE chatbot_conversations cc
SET diagnosis_completed = pa.outcome IN ('self_service', 'professional_required'),
    recommended_category_id = pa.problem_category_id
FROM problem_assessments pa
WHERE pa.id = cc.current_assessment_id;

ALTER TABLE chatbot_conversations
    ALTER COLUMN diagnosis_completed DROP DEFAULT;

DROP INDEX IF EXISTS job_requests_source_assessment_id_idx;
ALTER TABLE job_requests DROP COLUMN IF EXISTS source_assessment_id;

ALTER TABLE chatbot_conversations DROP COLUMN IF EXISTS current_assessment_id;
DROP TABLE IF EXISTS problem_assessments;
