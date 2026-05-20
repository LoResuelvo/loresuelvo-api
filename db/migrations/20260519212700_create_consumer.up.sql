CREATE TABLE consumers (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) on delete cascade
);
