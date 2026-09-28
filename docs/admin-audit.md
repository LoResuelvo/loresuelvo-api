# Administrative audit component (US-62)

This is an internal, append-only **application contract** for administrative
audit events (US-62). It does not itself define an HTTP endpoint, authorize
requests, or automatically instrument `/admin`. Producing use cases choose
when to create an event and own the corresponding acceptance tests. The
separate audit-log query API is defined by US-63 (#214), documented in
OpenAPI, and explicitly listed in the policy below.

## Event contract

Construct events with `audit.NewEvent(audit.EventParams{...})`; do not bypass
validation or assemble arbitrary JSON. `Writer.Save(ctx, event)` creates an
event. `Reader.FindByID(ctx, id)` reads one. Neither port exposes update,
delete, or upsert behavior. To correct a record, write a **new** event and
retain the original.

| Field | Requirement and limit |
| --- | --- |
| `ID` | Required, non-nil UUID, unique per event. |
| `OperatorID` | Required, positive local user ID resolved by the producing use case from the authenticated context; never accepted as a free request field. |
| `Action` | Required: `create`, `access`, or `execute` via `ActionCreate`, `ActionAccess`, or `ActionExecute`. |
| `ResourceType` | Required snake_case resource/collection name, 1–64 bytes. |
| `ResourceID` | Optional safe ASCII identifier, 1–128 bytes when present. Omit for a collection-level access. |
| `OccurredOn` | Required nonzero instant. The constructor normalizes it to UTC; the producer supplies the event time. |
| `Result` | Required: `succeeded`, `prepared`, or `failed` via the corresponding result constants. Create/execute allow succeeded or failed; access allows prepared or failed. |
| `CorrelationID` | Required safe ASCII identifier, 1–128 bytes, linking the event to the operation/request without copying its payload. |
| `Reason` | Optional `NewReason(text)`: trimmed, nonempty valid UTF-8, no Unicode control characters, at most 500 bytes. The **producer** decides which operations require a manual reason; do not include it in technical logs. |
| `ConversationID` | Optional positive integer referencing the conversation involved in a producing use case. Defensively copied by the event. Persisted as nullable evidence without a foreign key, so deletion of the conversation does not invalidate recorded evidence. |
| `StateChange` | Optional `NewStateChange(field, from, to)`: each component is snake_case, 1–64 bytes; `from` and `to` must differ. Only use it for a bounded, non-sensitive state transition. |

Safe identifiers match `[A-Za-z0-9][A-Za-z0-9._:-]*`. The admitted metadata is explicitly typed: `Reason`, `StateChange`, and the
optional positive `ConversationID` association. There is no generic metadata map or arbitrary HTTP-body
field. Never put credentials, tokens, chat messages, documents, biometric
material, signed URLs, or response payloads in an event. A producing use case
must select and sanitize its own identifiers and state labels before building
an event. Validation failure is `audit.ErrInvalidEvent`; storage failures are
`audit.ErrPersistence` and must not be swallowed.

`ResultPrepared` on an `access` event means the requested delivery was
authorized/prepared after persistence. It does **not** prove that the client
received, displayed, or read the data. For sensitive reads, the producing
use case must save the event **before** releasing the response and fail closed
if persistence fails. It must not claim receipt or create one event per
returned row.

`ConversationID` is currently internal audit evidence: the allowlisted
`AdminAuditLogEvent` HTTP mapper does not expose it. Do not silently expand
that public response when adding a typed internal association.

## Integration boundary

- The producing use case resolves the operator from authentication, checks
  its own RBAC and privacy rules, decides whether a reason is required, builds
  the event, and calls the writer. This component knows nothing about Gin,
  Auth0, category, chat, payment, or consumer workflows.
- Standalone `Save` is available for audited reads via
  `repositories.NewAuditEventRepository(db)`. The repository's internal
  `saveWithExecutor` accepts a `*sql.Tx` as well as the DB executor; a generic
  unit-of-work adapter in the repositories package can use it alongside the
  caller's transactional store. The **caller** owns commit/rollback; the
  audit writer must not commit a caller's transaction. US-62 tests this
  generic participation only; US-62.1 (#223) owns the category-create
  integration and its business/audit atomicity test.
- Propagate audit errors. Do not pretend an event was stored when `Save`
  failed, and do not compensate by logging the private event or its reason.
- For BDD/other integration tests, `internal/testsupport.AuditEvents` offers
  `Save` and `FindByID` through `audit.Writer`/`audit.Reader`. Scenarios
  testing event production should use those ports, not raw SQL or
  `GET /admin/audit-logs`; the query endpoint is covered by its own US-63
  acceptance scenarios.

## First-iteration policy

| Operation | Persistent event? | Manual reason? | Owning issue |
| --- | --- | --- | --- |
| Consumer/provider directories | No | No | #211 |
| Shared `GET /categories` list | No | No | #212 |
| `POST /categories` create | Yes, with mutation | No | #223 |
| General `GET /admin/operations` inbox | No | No | #215 |
| Individual hiring/operation detail | Yes | No | #216 |
| Provider/consumer account detail | Yes | No | #220/#221 |
| Administrative payment detail | Yes | No | #218 |
| Contract chat read | Yes | Yes | #217 |
| Payment reconciliation | Yes, with local changes where present | Yes | #219 |
| Audit-log query | Yes, without recursive logging | No | #214 |

The query endpoint is `GET /admin/audit-logs` and requires the Auth0
`read:admin_audit` permission. It exposes only allowlisted event fields,
records one prepared access event before returning a page, fails closed if
that evidence cannot be persisted, and sends `Cache-Control: private,
no-store`. Its supported filters, signed stable-cutoff cursor, and HTTP
responses are specified in the OpenAPI contract.

This policy does not make excluded reads public; their authentication,
authorization, privacy controls, and sanitized technical/security logs remain
in force. No global audit middleware should be introduced. Do not expand the
policy to new exports, private data, or future administrative mutations
without a separately defined increment.

The operation chat endpoint is `GET /admin/operations/{id}/conversation`
(US-64.2, #217), independently guarded by `read:admin_chat_audit`. Its manual
reason is required on each page; the frontend may ask once on entry and
resend it automatically while paginating. Each page saves one access/prepared
event before delivery or private media URL preparation. The event records the
primary operation and typed conversation association, never chat text or
attachments. Cursor navigation does not represent a frozen snapshot. See the
[administrative operation chat contract](admin-operation-chat.md) for the
permission boundaries, bounded pagination, fail-closed behavior and media
ownership rules.

## Explicitly accepted risk

Append-only is enforced **by application architecture**, not PostgreSQL
privileges, in US-62. The audit port and repository expose only creation and
reading; no code may update, delete, truncate, or `UPSERT ... DO UPDATE`
existing events. `DATABASE_URL` and `TEST_DATABASE_URL` remain unchanged. With
the current PostgreSQL credentials, the database itself does **not** prevent
future code from issuing destructive SQL. Separate runtime/migrator roles,
new migration URLs, privilege tests, and Docker/CI/secrets changes are outside
this iteration and should be tracked as an independent hardening task.
The down migration refuses to drop a nonempty audit table, so a routine
rollback cannot discard recorded events.
