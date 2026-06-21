ALTER TABLE messages
    DROP CONSTRAINT messages_content_not_empty_check;

CREATE TABLE message_images (
    message_id INTEGER NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    file_id UUID NOT NULL UNIQUE REFERENCES files(id),
    position SMALLINT NOT NULL CHECK (position >= 0),
    PRIMARY KEY (message_id, file_id),
    CONSTRAINT message_images_message_position_unique UNIQUE (message_id, position)
);

CREATE INDEX message_images_message_id_idx ON message_images (message_id);
