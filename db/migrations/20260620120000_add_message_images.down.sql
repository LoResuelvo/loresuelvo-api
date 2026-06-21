DROP TABLE message_images;

DELETE FROM messages WHERE length(btrim(content)) = 0;

ALTER TABLE messages
    ADD CONSTRAINT messages_content_not_empty_check CHECK (length(btrim(content)) > 0);
