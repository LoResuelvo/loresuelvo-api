CREATE TABLE work_conversations (
    conversation_id INTEGER PRIMARY KEY REFERENCES conversations(id) ON DELETE CASCADE,
    consumer_id INTEGER NOT NULL REFERENCES consumers(id) ON DELETE CASCADE,
    provider_id INTEGER NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    CONSTRAINT work_conversations_consumer_provider_unique UNIQUE (consumer_id, provider_id)
);

INSERT INTO work_conversations (conversation_id, consumer_id, provider_id)
SELECT id, consumer_id, provider_id
FROM conversations;

ALTER TABLE conversations
    DROP CONSTRAINT conversations_consumer_provider_unique,
    DROP COLUMN consumer_id,
    DROP COLUMN provider_id;

CREATE INDEX work_conversations_consumer_id_idx
    ON work_conversations (consumer_id);

CREATE INDEX work_conversations_provider_id_idx
    ON work_conversations (provider_id);

CREATE TABLE chatbot_conversations (
    conversation_id INTEGER PRIMARY KEY REFERENCES conversations(id) ON DELETE CASCADE,
    consumer_id INTEGER NOT NULL REFERENCES consumers(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    CONSTRAINT chatbot_conversations_title_not_empty_check CHECK (length(btrim(title)) > 0)
);

CREATE INDEX chatbot_conversations_consumer_id_idx
    ON chatbot_conversations (consumer_id);

ALTER TABLE messages
    DROP CONSTRAINT messages_sender_role_check,
    ADD CONSTRAINT messages_sender_role_check CHECK (sender_role IN ('consumer', 'provider', 'chatbot'));
