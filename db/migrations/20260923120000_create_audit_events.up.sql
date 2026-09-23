-- Audit events are append-only by application contract. The current database role
-- retains write privileges; PostgreSQL privilege hardening is a separate task.
CREATE TABLE audit_events (
    id UUID PRIMARY KEY,
    operator_id INTEGER NOT NULL CHECK (operator_id > 0),
    action VARCHAR(16) NOT NULL CHECK (action IN ('create', 'access', 'execute')),
    resource_type VARCHAR(64) NOT NULL CHECK (resource_type ~ '^[a-z][a-z0-9_]*$'),
    resource_id VARCHAR(128) NOT NULL DEFAULT '' CHECK (resource_id = '' OR resource_id ~ '^[A-Za-z0-9][A-Za-z0-9._:-]*$'),
    occurred_on TIMESTAMPTZ NOT NULL,
    result VARCHAR(16) NOT NULL CHECK (result IN ('succeeded', 'prepared', 'failed')),
    correlation_id VARCHAR(128) NOT NULL CHECK (correlation_id ~ '^[A-Za-z0-9][A-Za-z0-9._:-]*$'),
    reason TEXT CHECK (reason IS NULL OR (octet_length(reason) BETWEEN 1 AND 500 AND reason = btrim(reason))),
    changed_field VARCHAR(64),
    state_from VARCHAR(64),
    state_to VARCHAR(64),
    CONSTRAINT audit_events_result_action_check CHECK (
        (result <> 'prepared' OR action = 'access') AND
        (result <> 'succeeded' OR action <> 'access')
    ),
    CONSTRAINT audit_events_state_change_check CHECK (
        num_nonnulls(changed_field, state_from, state_to) = 0 OR
        (num_nonnulls(changed_field, state_from, state_to) = 3 AND
         changed_field ~ '^[a-z][a-z0-9_]*$' AND
         state_from ~ '^[a-z][a-z0-9_]*$' AND
         state_to ~ '^[a-z][a-z0-9_]*$' AND
         state_from <> state_to)
    )
);

CREATE INDEX audit_events_resource_occurred_idx
    ON audit_events (resource_type, resource_id, occurred_on DESC);

CREATE INDEX audit_events_operator_occurred_idx
    ON audit_events (operator_id, occurred_on DESC);
