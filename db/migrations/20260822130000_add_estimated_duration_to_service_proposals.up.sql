ALTER TABLE service_proposals
    ADD COLUMN estimated_duration_minutes INTEGER;

UPDATE service_proposals
SET estimated_duration_minutes = 60
WHERE estimated_duration_minutes IS NULL;

ALTER TABLE service_proposals
    ADD CONSTRAINT service_proposals_estimated_duration_minutes_check
        CHECK (estimated_duration_minutes BETWEEN 15 AND 1440);

ALTER TABLE service_proposals
    ALTER COLUMN estimated_duration_minutes SET NOT NULL;
