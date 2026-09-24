-- A transactional counter, rather than a sequence, makes the watermark a
-- committed-ingest boundary. In-flight inserts cannot commit below a cut.
CREATE TABLE audit_ingest_counter (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    last_value BIGINT NOT NULL CHECK (last_value >= 0)
);
INSERT INTO audit_ingest_counter (id, last_value) VALUES (1, 0);

ALTER TABLE audit_events ADD COLUMN ingest_seq BIGINT;
WITH numbered AS (
    SELECT id, row_number() OVER (ORDER BY occurred_on, id) AS seq
    FROM audit_events
)
UPDATE audit_events AS event
SET ingest_seq = numbered.seq
FROM numbered
WHERE event.id = numbered.id;
UPDATE audit_ingest_counter SET last_value = (SELECT COUNT(*) FROM audit_events) WHERE id = 1;
ALTER TABLE audit_events ALTER COLUMN ingest_seq SET NOT NULL;
ALTER TABLE audit_events ADD CONSTRAINT audit_events_ingest_seq_unique UNIQUE (ingest_seq);

CREATE FUNCTION assign_audit_ingest_seq() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    UPDATE audit_ingest_counter SET last_value = last_value + 1
    WHERE id = 1 RETURNING last_value INTO NEW.ingest_seq;
    RETURN NEW;
END;
$$;
CREATE TRIGGER audit_events_assign_ingest_seq
    BEFORE INSERT ON audit_events
    FOR EACH ROW EXECUTE FUNCTION assign_audit_ingest_seq();

CREATE INDEX audit_events_occurred_id_idx
    ON audit_events (occurred_on DESC, id DESC);
DROP INDEX audit_events_operator_occurred_idx;
CREATE INDEX audit_events_operator_occurred_id_idx
    ON audit_events (operator_id, occurred_on DESC, id DESC);
