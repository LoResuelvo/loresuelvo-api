CREATE TABLE file_videos (
    file_id UUID PRIMARY KEY REFERENCES files(id) ON DELETE CASCADE,
    video_codec VARCHAR(50) NOT NULL CHECK (length(btrim(video_codec)) > 0),
    audio_codec VARCHAR(50),
    duration_seconds INTEGER NOT NULL CHECK (duration_seconds > 0),
    width INTEGER NOT NULL CHECK (width > 0),
    height INTEGER NOT NULL CHECK (height > 0)
);

CREATE TABLE message_videos (
    message_id INTEGER PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE,
    file_id UUID NOT NULL UNIQUE REFERENCES file_videos(file_id)
);
