CREATE INDEX payment_transactions_approved_verified_cursor_idx
    ON payment_transactions (verified_on DESC, id DESC)
    WHERE status = 'approved';
