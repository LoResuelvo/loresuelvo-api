CREATE TABLE websocket_tickets (
    ticket TEXT PRIMARY KEY,
    auth_id TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX websocket_tickets_expires_at_idx
    ON websocket_tickets (expires_at);
