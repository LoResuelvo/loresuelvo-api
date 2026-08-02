ALTER TABLE payment_intents
    DROP CONSTRAINT payment_intents_purpose_check;

ALTER TABLE payment_intents
    ADD CONSTRAINT payment_intents_purpose_check
        CHECK (purpose IN ('booking_deposit', 'service_balance'));

ALTER INDEX payment_intents_active_booking_deposit_unique
    RENAME TO payment_intents_active_per_proposal_and_purpose_unique;
