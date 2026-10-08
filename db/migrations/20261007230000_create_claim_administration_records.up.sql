CREATE TABLE claim_administration_records (
 operator_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 key UUID NOT NULL,
 fingerprint CHAR(64) NOT NULL,
 claim_id INTEGER NOT NULL REFERENCES claims(id) ON DELETE CASCADE,
 action_id INTEGER NOT NULL REFERENCES claim_actions(id) ON DELETE CASCADE,
 PRIMARY KEY(operator_id,key)
);
CREATE INDEX claims_admin_created_idx ON claims(created_on DESC,id DESC);
CREATE INDEX claims_admin_status_created_idx ON claims(status,created_on DESC,id DESC);
