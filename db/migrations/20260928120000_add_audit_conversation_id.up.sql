-- Evidence remains available after the conversation is deleted: intentionally no foreign key.
ALTER TABLE audit_events ADD COLUMN conversation_id INTEGER CHECK (conversation_id > 0);
