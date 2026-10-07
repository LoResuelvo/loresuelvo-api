CREATE TABLE claims (
 id SERIAL PRIMARY KEY,
 claimant_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 claimant_party TEXT NOT NULL CHECK (claimant_party IN ('consumer','provider')),
 operation_job_request_id INTEGER REFERENCES job_requests(id) ON DELETE CASCADE,
 operation_service_proposal_id INTEGER REFERENCES service_proposals(id) ON DELETE CASCADE,
 reference_job_request_id INTEGER REFERENCES job_requests(id) ON DELETE CASCADE,
 reference_service_proposal_id INTEGER REFERENCES service_proposals(id) ON DELETE CASCADE,
 reference_work_order_id INTEGER REFERENCES work_orders(id) ON DELETE CASCADE,
 reference_payment_intent_id UUID REFERENCES payment_intents(id) ON DELETE CASCADE,
 reason TEXT NOT NULL CHECK (reason IN ('non_compliance','improper_charge','damage','poor_quality')),
 description TEXT NOT NULL CHECK (char_length(description) BETWEEN 1 AND 5000),
 status TEXT NOT NULL CHECK (status IN ('open','in_review','resolved','dismissed')),
 created_on TIMESTAMPTZ NOT NULL,
 review_started_on TIMESTAMPTZ,
 closed_on TIMESTAMPTZ,
 submission_key UUID NOT NULL,
 submission_fingerprint CHAR(64) NOT NULL,
 CONSTRAINT claims_operation_check CHECK (num_nonnulls(operation_job_request_id,operation_service_proposal_id)=1),
 CONSTRAINT claims_reference_check CHECK (num_nonnulls(reference_job_request_id,reference_service_proposal_id,reference_work_order_id,reference_payment_intent_id)=1),
 CONSTRAINT claims_submission_key_unique UNIQUE (claimant_id,submission_key),
 CONSTRAINT claims_lifecycle_check CHECK (
  (status='open' AND review_started_on IS NULL AND closed_on IS NULL)
  OR (status='in_review' AND review_started_on IS NOT NULL AND closed_on IS NULL)
  OR (status IN ('resolved','dismissed') AND review_started_on IS NOT NULL AND closed_on IS NOT NULL)),
 CONSTRAINT claims_dates_check CHECK ((review_started_on IS NULL OR review_started_on>=created_on) AND (closed_on IS NULL OR closed_on>=review_started_on))
);
CREATE UNIQUE INDEX claims_unfinished_operation_unique ON claims (claimant_id,COALESCE(operation_job_request_id,0),COALESCE(operation_service_proposal_id,0)) WHERE status IN ('open','in_review');
CREATE INDEX claims_owned_created_idx ON claims(claimant_id,created_on DESC,id DESC);
CREATE INDEX claims_owned_status_created_idx ON claims(claimant_id,status,created_on DESC,id DESC);
CREATE TABLE claim_images (
 claim_id INTEGER NOT NULL REFERENCES claims(id) ON DELETE CASCADE,
 file_id UUID NOT NULL REFERENCES files(id),
 PRIMARY KEY(claim_id,file_id),
 CONSTRAINT claim_images_file_unique UNIQUE(file_id)
);
CREATE TABLE claim_actions (
 id SERIAL PRIMARY KEY,
 claim_id INTEGER NOT NULL REFERENCES claims(id) ON DELETE CASCADE,
 type TEXT NOT NULL,
 actor_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 actor_party TEXT NOT NULL CHECK(actor_party IN ('consumer','provider','operator')),
 created_on TIMESTAMPTZ NOT NULL
);
CREATE INDEX claim_actions_claim_idx ON claim_actions(claim_id,id);
CREATE TABLE claim_resolutions (
 claim_id INTEGER PRIMARY KEY REFERENCES claims(id) ON DELETE CASCADE,
 type TEXT NOT NULL CHECK(type IN ('consumer_favor','provider_favor','agreement','without_merit')),
 reasoning TEXT NOT NULL CHECK(char_length(btrim(reasoning)) BETWEEN 1 AND 5000),
 resolved_on TIMESTAMPTZ NOT NULL,
 suggested_amount_minor BIGINT CHECK(suggested_amount_minor>0),
 suggested_currency TEXT CHECK(suggested_currency ~ '^[A-Z]{3}$'),
 suggested_unit TEXT CHECK(suggested_unit='minor'),
 CONSTRAINT claim_resolutions_compensation_check CHECK(num_nonnulls(suggested_amount_minor,suggested_currency,suggested_unit) IN (0,3))
);
