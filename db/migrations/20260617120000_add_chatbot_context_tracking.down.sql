DROP INDEX IF EXISTS messages_conversation_id_id_idx;

ALTER TABLE chatbot_conversations
    DROP CONSTRAINT IF EXISTS chatbot_conversations_last_response_status_check,
    DROP CONSTRAINT IF EXISTS chatbot_conversations_last_summarized_message_id_non_negative_check,
    DROP COLUMN IF EXISTS recommended_category_id,
    DROP COLUMN IF EXISTS diagnosis_completed,
    DROP COLUMN IF EXISTS last_response_status,
    DROP COLUMN IF EXISTS processing_started_on,
    DROP COLUMN IF EXISTS last_summarized_message_id,
    DROP COLUMN IF EXISTS context_summary;
