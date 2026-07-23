ALTER TABLE service_proposals
    ADD COLUMN booking_payment_deadline TIMESTAMP NOT NULL,
    ADD CONSTRAINT service_proposals_booking_payment_deadline_check
        CHECK (booking_payment_deadline = scheduled_on - INTERVAL '24 hours');
