CREATE TABLE file_audios (
    file_id UUID PRIMARY KEY REFERENCES files(id) ON DELETE CASCADE,
    codec VARCHAR(50) NOT NULL CHECK (length(btrim(codec)) > 0),
    duration_seconds INTEGER NOT NULL CHECK (duration_seconds > 0)
);

CREATE TABLE message_audios (
    message_id INTEGER PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE,
    file_id UUID NOT NULL UNIQUE REFERENCES file_audios(file_id)
);
