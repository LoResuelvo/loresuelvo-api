ALTER TABLE service_proposals
    DROP CONSTRAINT IF EXISTS service_proposals_booking_payment_deadline_check,
    DROP COLUMN IF EXISTS booking_payment_deadline;
