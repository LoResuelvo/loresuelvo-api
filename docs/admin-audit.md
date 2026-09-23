# Administrative audit component (US-62)

This is an internal, append-only **application contract** for administrative
audit events. It does not add an HTTP endpoint, authorize requests, or
automatically instrument `/admin`. Producing use cases choose when to create
an event and own the corresponding acceptance tests. The separate audit-log
query API belongs to US-63 (#214).

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
| `StateChange` | Optional `NewStateChange(field, from, to)`: each component is snake_case, 1–64 bytes; `from` and `to` must differ. Only use it for a bounded, non-sensitive state transition. |

Safe identifiers match `[A-Za-z0-9][A-Za-z0-9._:-]*`. The only structured metadata admitted by this contract is the typed `Reason`
and `StateChange`. There is no generic metadata map or arbitrary HTTP-body
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
  `Save` and `FindByID` through `audit.Writer`/`audit.Reader`. Scenario steps
  should use those ports, not raw SQL or `GET /admin/audit-logs`.

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

This policy does not make excluded reads public; their authentication,
authorization, privacy controls, and sanitized technical/security logs remain
in force. No global audit middleware should be introduced. Do not expand the
policy to new exports, private data, or future administrative mutations
without a separately defined increment.

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
