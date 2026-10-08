ALTER TABLE work_order_reviews ADD COLUMN moderation_version INTEGER NOT NULL DEFAULT 1 CHECK(moderation_version>0);
ALTER TABLE work_order_reviews ADD COLUMN hiding_decision_id INTEGER;
CREATE TABLE review_decisions (
 id SERIAL PRIMARY KEY,
 work_order_id INTEGER NOT NULL REFERENCES work_order_reviews(work_order_id) ON DELETE CASCADE,
 operator_id INTEGER NOT NULL REFERENCES users(id),
 action TEXT NOT NULL CHECK(action IN ('hide','unhide','dismiss_reports')),
 category TEXT,
 reason TEXT NOT NULL,
 report_id INTEGER REFERENCES review_reports(id),
 previous_hide_id INTEGER REFERENCES review_decisions(id),
 created_on TIMESTAMPTZ NOT NULL
);
ALTER TABLE work_order_reviews ADD CONSTRAINT review_hiding_decision_fkey FOREIGN KEY(hiding_decision_id) REFERENCES review_decisions(id);
CREATE INDEX review_decisions_order_id_idx ON review_decisions(work_order_id,id DESC);
