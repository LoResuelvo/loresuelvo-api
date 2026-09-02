CREATE TABLE chatbot_provider_recommendations (
    chatbot_conversation_id INTEGER PRIMARY KEY REFERENCES chatbot_conversations(conversation_id) ON DELETE CASCADE,
    problem_assessment_id INTEGER NOT NULL REFERENCES problem_assessments(id) ON DELETE CASCADE,
    candidate_provider_ids JSONB NOT NULL,
    recommendations JSONB NOT NULL,
    created_on TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT chatbot_provider_recommendations_candidate_ids_array_check CHECK (jsonb_typeof(candidate_provider_ids) = 'array'),
    CONSTRAINT chatbot_provider_recommendations_recommendations_array_check CHECK (jsonb_typeof(recommendations) = 'array')
);

CREATE INDEX chatbot_provider_recommendations_assessment_id_idx
    ON chatbot_provider_recommendations(problem_assessment_id);
