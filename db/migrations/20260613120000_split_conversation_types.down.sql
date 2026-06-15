DELETE FROM conversations c
USING chatbot_conversations cc
WHERE cc.conversation_id = c.id;

ALTER TABLE messages
    DROP CONSTRAINT messages_sender_role_check,
    ADD CONSTRAINT messages_sender_role_check CHECK (sender_role IN ('consumer', 'provider'));

ALTER TABLE conversations
    ADD COLUMN consumer_id INTEGER REFERENCES consumers(id) ON DELETE CASCADE,
    ADD COLUMN provider_id INTEGER REFERENCES providers(id) ON DELETE CASCADE;

UPDATE conversations c
SET consumer_id = wc.consumer_id,
    provider_id = wc.provider_id
FROM work_conversations wc
WHERE wc.conversation_id = c.id;

DELETE FROM conversations
WHERE consumer_id IS NULL OR provider_id IS NULL;

ALTER TABLE conversations
    ALTER COLUMN consumer_id SET NOT NULL,
    ALTER COLUMN provider_id SET NOT NULL,
    ADD CONSTRAINT conversations_consumer_provider_unique UNIQUE (consumer_id, provider_id);

DROP TABLE chatbot_conversations;
DROP TABLE work_conversations;

ALTER TABLE conversations
    DROP CONSTRAINT conversations_type_check,
    DROP COLUMN type;
