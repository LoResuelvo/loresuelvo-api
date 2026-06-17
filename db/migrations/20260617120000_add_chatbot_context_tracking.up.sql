ALTER TABLE chatbot_conversations
    ADD COLUMN context_summary TEXT NOT NULL DEFAULT '',
    ADD COLUMN last_summarized_message_id INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN processing_started_on TIMESTAMP NULL;

ALTER TABLE chatbot_conversations
    ADD CONSTRAINT chatbot_conversations_last_summarized_message_id_non_negative_check CHECK (last_summarized_message_id >= 0);

CREATE INDEX messages_conversation_id_id_idx
    ON messages (conversation_id, id);
