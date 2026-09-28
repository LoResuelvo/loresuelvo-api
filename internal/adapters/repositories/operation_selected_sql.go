package repositories

// operationSelectedSQL preserves the stable inbox identity across administrative readers.
const operationSelectedSQL = `WITH selected AS (
 SELECT 'jr'::text AS kind, jr.id AS resource_id, jr.created_on AS started_on,
   jr.id AS job_request_id,
   (SELECT sp.id FROM service_proposals sp WHERE sp.conversation_id = jr.conversation_id ORDER BY sp.id LIMIT 1) AS proposal_id,
   jr.consumer_id, jr.provider_id, jr.conversation_id
 FROM job_requests jr WHERE $1::text = 'jr' AND jr.id = $2
 UNION ALL
 SELECT 'sp'::text, sp.id, sp.created_on, jr.id, sp.id, sp.consumer_id, sp.provider_id, sp.conversation_id
 FROM service_proposals sp
 LEFT JOIN job_requests jr ON jr.conversation_id = sp.conversation_id
 WHERE $1::text = 'sp' AND sp.id = $2
   AND (jr.id IS NULL OR sp.id <> (SELECT first.id FROM service_proposals first WHERE first.conversation_id = sp.conversation_id ORDER BY first.id LIMIT 1))
)`
