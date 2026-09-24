DROP INDEX audit_events_operator_occurred_id_idx;
CREATE INDEX audit_events_operator_occurred_idx
    ON audit_events (operator_id, occurred_on DESC);
DROP INDEX audit_events_occurred_id_idx;
DROP TRIGGER audit_events_assign_ingest_seq ON audit_events;
DROP FUNCTION assign_audit_ingest_seq();
ALTER TABLE audit_events DROP COLUMN ingest_seq;
DROP TABLE audit_ingest_counter;
