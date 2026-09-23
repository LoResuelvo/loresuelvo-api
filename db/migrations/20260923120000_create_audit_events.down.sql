-- Rolling back an empty installation is safe; never discard recorded events.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM audit_events) THEN
        RAISE EXCEPTION 'cannot drop audit_events while audit events exist';
    END IF;
END
$$;

DROP TABLE audit_events;
