ALTER TABLE installations ADD COLUMN secret_hash BYTEA NOT NULL DEFAULT '\x';
ALTER TABLE installations ADD COLUMN revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0);
