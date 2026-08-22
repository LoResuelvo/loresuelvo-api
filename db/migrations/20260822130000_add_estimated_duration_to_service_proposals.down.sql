ALTER TABLE service_proposals
    DROP CONSTRAINT IF EXISTS service_proposals_estimated_duration_minutes_check;

ALTER TABLE service_proposals
    DROP COLUMN IF EXISTS estimated_duration_minutes;
