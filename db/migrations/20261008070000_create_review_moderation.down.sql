ALTER TABLE work_order_reviews DROP CONSTRAINT review_hiding_decision_fkey;
DROP TABLE review_decisions;
ALTER TABLE work_order_reviews DROP COLUMN hiding_decision_id;
ALTER TABLE work_order_reviews DROP COLUMN moderation_version;
