CREATE TABLE files (
    id UUID PRIMARY KEY,
    key TEXT NOT NULL UNIQUE,
    bucket VARCHAR(255) NOT NULL,
    original_name VARCHAR(255) NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    size_bytes INTEGER NOT NULL,
    status VARCHAR(50) NOT NULL,
    visibility VARCHAR(50) NOT NULL,
    purpose VARCHAR(100) NOT NULL,
    uploaded_by_auth_id VARCHAR(255) NOT NULL,
    created_on TIMESTAMP NOT NULL,
    updated_on TIMESTAMP NOT NULL
);

CREATE INDEX files_uploaded_by_auth_id_idx ON files (uploaded_by_auth_id);
CREATE INDEX files_status_purpose_idx ON files (status, purpose);
