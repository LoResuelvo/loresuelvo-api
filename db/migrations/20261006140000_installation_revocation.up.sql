ALTER TABLE installations ADD COLUMN revoked BOOLEAN NOT NULL DEFAULT FALSE;
UPDATE installations SET revoked = TRUE WHERE NOT enabled;
