CREATE TABLE installations (
 id VARCHAR(200) PRIMARY KEY,
 user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 app TEXT NOT NULL CHECK (app IN ('consumer', 'provider')),
 token TEXT NOT NULL UNIQUE,
 locale TEXT NOT NULL DEFAULT 'es' CHECK (locale IN ('es', 'en')),
 binding_id VARCHAR(200) NOT NULL,
 enabled BOOLEAN NOT NULL DEFAULT TRUE
);
CREATE INDEX installations_user_id_idx ON installations(user_id) WHERE enabled;
