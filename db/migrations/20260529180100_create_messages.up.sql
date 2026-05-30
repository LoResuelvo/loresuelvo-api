CREATE TABLE messages (
    id SERIAL PRIMARY KEY,
    conversation_id INTEGER NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sender_role VARCHAR(50) NOT NULL,
    content TEXT NOT NULL,
    created_on TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT messages_sender_role_check CHECK (sender_role IN ('consumer', 'provider')),
    CONSTRAINT messages_content_not_empty_check CHECK (length(btrim(content)) > 0)
);

CREATE INDEX messages_conversation_id_idx
    ON messages (conversation_id);

CREATE INDEX messages_conversation_created_on_id_idx
    ON messages (conversation_id, created_on DESC, id DESC);

