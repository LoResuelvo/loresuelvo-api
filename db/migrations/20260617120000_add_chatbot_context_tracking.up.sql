ALTER TABLE chatbot_conversations
    ADD COLUMN context_summary TEXT NOT NULL,
    ADD COLUMN last_summarized_message_id INTEGER NOT NULL,
    ADD COLUMN processing_started_on TIMESTAMP NULL,
    ADD COLUMN last_response_status TEXT NOT NULL,
    ADD COLUMN diagnosis_completed BOOLEAN NOT NULL,
    ADD COLUMN recommended_category_id INTEGER NULL REFERENCES categories(id);

ALTER TABLE chatbot_conversations
    ADD CONSTRAINT chatbot_conversations_last_summarized_message_id_non_negative_check CHECK (last_summarized_message_id >= 0),
    ADD CONSTRAINT chatbot_conversations_last_response_status_check CHECK (last_response_status IN ('answered', 'out_of_scope'));

CREATE INDEX messages_conversation_id_id_idx
    ON messages (conversation_id, id);
