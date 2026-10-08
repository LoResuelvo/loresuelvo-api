ALTER TABLE work_order_reviews ADD COLUMN visible BOOLEAN NOT NULL DEFAULT TRUE;
CREATE TABLE review_reports (
 id SERIAL PRIMARY KEY,
 work_order_id INTEGER NOT NULL REFERENCES work_order_reviews(work_order_id) ON DELETE CASCADE,
 reporter_id INTEGER NOT NULL REFERENCES providers(user_id),
 category TEXT NOT NULL CHECK (category IN ('abusive_language','personal_data','spam_advertising','unrelated_content')),
 explanation TEXT NOT NULL DEFAULT '' CHECK (char_length(explanation) <= 500),
 status TEXT NOT NULL CHECK (status IN ('pending','upheld','dismissed')),
 created_on TIMESTAMPTZ NOT NULL,
 CONSTRAINT review_reports_work_order_id_key UNIQUE (work_order_id)
);
