-- Support bounded administrative keyset pages across active and historical intents.
CREATE INDEX payment_intents_created_cursor_idx
    ON payment_intents (created_on DESC, id DESC);

-- The active-intent partial uniqueness index excludes historical attempts;
-- proposal snapshots need all siblings, irrespective of their current status.
CREATE INDEX payment_intents_proposal_created_cursor_idx
    ON payment_intents (service_proposal_id, created_on DESC, id DESC);
