ALTER TABLE work_conversations DROP CONSTRAINT work_conversations_consumer_id_fkey;
ALTER TABLE work_conversations DROP CONSTRAINT work_conversations_provider_id_fkey;
ALTER TABLE chatbot_conversations DROP CONSTRAINT chatbot_conversations_consumer_id_fkey;
ALTER TABLE job_requests DROP CONSTRAINT job_requests_consumer_id_fkey;
ALTER TABLE job_requests DROP CONSTRAINT job_requests_provider_id_fkey;
ALTER TABLE service_proposals DROP CONSTRAINT service_proposals_consumer_id_fkey;
ALTER TABLE service_proposals DROP CONSTRAINT service_proposals_provider_id_fkey;

UPDATE work_conversations wc
SET consumer_id = c.user_id
FROM consumers c
WHERE wc.consumer_id = c.id;

UPDATE work_conversations wc
SET provider_id = p.user_id
FROM providers p
WHERE wc.provider_id = p.id;

UPDATE chatbot_conversations cc
SET consumer_id = c.user_id
FROM consumers c
WHERE cc.consumer_id = c.id;

UPDATE job_requests jr
SET consumer_id = c.user_id
FROM consumers c
WHERE jr.consumer_id = c.id;

UPDATE job_requests jr
SET provider_id = p.user_id
FROM providers p
WHERE jr.provider_id = p.id;

UPDATE service_proposals sp
SET consumer_id = c.user_id
FROM consumers c
WHERE sp.consumer_id = c.id;

UPDATE service_proposals sp
SET provider_id = p.user_id
FROM providers p
WHERE sp.provider_id = p.id;

ALTER TABLE consumers DROP CONSTRAINT consumers_pkey;
ALTER TABLE providers DROP CONSTRAINT providers_pkey;
ALTER TABLE consumers DROP COLUMN id;
ALTER TABLE providers DROP COLUMN id;
ALTER TABLE consumers ADD PRIMARY KEY (user_id);
ALTER TABLE providers ADD PRIMARY KEY (user_id);

ALTER TABLE work_conversations ADD FOREIGN KEY (consumer_id) REFERENCES consumers(user_id) ON DELETE CASCADE;
ALTER TABLE work_conversations ADD FOREIGN KEY (provider_id) REFERENCES providers(user_id) ON DELETE CASCADE;
ALTER TABLE chatbot_conversations ADD FOREIGN KEY (consumer_id) REFERENCES consumers(user_id) ON DELETE CASCADE;
ALTER TABLE job_requests ADD FOREIGN KEY (consumer_id) REFERENCES consumers(user_id) ON DELETE CASCADE;
ALTER TABLE job_requests ADD FOREIGN KEY (provider_id) REFERENCES providers(user_id) ON DELETE CASCADE;
ALTER TABLE service_proposals ADD FOREIGN KEY (consumer_id) REFERENCES consumers(user_id) ON DELETE CASCADE;
ALTER TABLE service_proposals ADD FOREIGN KEY (provider_id) REFERENCES providers(user_id) ON DELETE CASCADE;
