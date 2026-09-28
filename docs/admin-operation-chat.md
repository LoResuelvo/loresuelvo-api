# Audited administrative operation chat (US-64.2, #217)

`GET /admin/operations/{id}/conversation` is a separate, private read capability.
It is not the ordinary operation detail or a participant conversation endpoint.
The exact response schema and errors are in
[`AdminOperationChat`](../openapi/components/schemas/admin-operation-chat.yaml)
and the [endpoint contract](../openapi/paths/admin-operation-chat.yaml).

## Authorization boundaries

| Capability | Authorization |
| --- | --- |
| Administrative inbox, operation detail and operation image evidence | `read:admin_operations` |
| Administrative operation conversation | `read:admin_chat_audit` |
| Administrative audit-log query | `read:admin_audit` |
| Participant conversation and WebSocket connection | Their own authenticated participant/membership rules |

These permissions are independent. Reading the inbox/detail does not enable
chat access, and chat permission does not grant participant membership or a
WebSocket subscription. The operator must resolve to a provisioned local
administrator for the audit; an identity lookup or audit failure does not
release the chat.

All responses on the administrative chat route, including errors, have
`Cache-Control: private, no-store`. Request logging omits its body, response
body and query values; reason, messages and signed media access URLs must not
be copied into technical logs. Participant `/conversations` reads and message
writes also omit sensitive body/query logging, while retaining method, route,
status and request correlation diagnostics.

## Stable operation and shared conversation

- IDs are `jr-<id>` or `sp-<id>`, with a canonical positive numeric suffix up
  to `2147483647`; leading zeroes and other resource prefixes are rejected.
- The first proposal belonging to a request remains part of its `jr` identity.
  Later proposals have their own `sp` identity, but can share the same work
  conversation and its historical messages.
- The backend resolves the persisted operation/conversation relation and checks
  the work conversation's consumer/provider against the operation. Callers
  cannot choose another `conversation_id` query parameter.
- `service_proposal_id` identifies the selected proposal; ordered
  `related_service_proposal_ids` identifies all proposals sharing the
  conversation. `shared_conversation` is true when there is more than one.
  Messages are conversation-owned: no per-message proposal attribution is
  synthesized.
- `job_request_id` and `service_proposal_id` are explicitly nullable. Arrays
  are never null. A conversation with no messages returns `messages: []` and
  `next_cursor: null`.

## Reason entered once, transmitted on every page

The frontend asks for a reason when the operator enters the conversation,
then resends the same `X-Audit-Reason` on every page automatically. It does not
need to prompt the operator again just because the messages are paginated.
The API has no access-session state: every request independently authorizes
and audits the read and requires a valid header.

After Unicode surrounding whitespace is trimmed, the reason must be valid
UTF-8, nonempty, free of Unicode control characters, and **1–500 UTF-8 bytes**.
This is not a 500-character limit: multibyte characters count by encoded byte
length. JSON Schema/OpenAPI `maxLength` counts characters rather than trimmed
UTF-8 bytes, so the header's runtime byte rule is described explicitly instead
of imposing a misleading raw-header length constraint. The reason must not
contain credentials, chat content or signed URLs. A GET body is not a source
for this reason.

## Bounded chronological pagination

- `limit` defaults to `20`, accepts decimal `1–100`, and may occur once.
- Messages are ordered by `(created_on, id)` ascending. The first page starts
  at the oldest message; a continuation selects strictly greater ordering
  keys. IDs disambiguate messages with identical timestamps.
- A bounded `limit + 1` read detects another page; the lookahead message is
  neither returned nor used for attachment access preparation.
- `next_cursor` is a signed opaque token or null at exhaustion. Send it as
  `cursor`; omit `limit` to inherit the token's limit, or specify the same
  limit. A different limit, duplicate parameter, unknown parameter, empty
  cursor or invalid cursor returns `400`.
- The versioned cursor binds the stable operation, persisted conversation,
  limit and final delivered message's timestamp/ID. It contains no reason,
  message text or attachment URLs, and is limited to `4096` characters.
- Cursor integrity is protected using the existing `AUDIT_CURSOR_HMAC_KEY`
  (at least 32 bytes), with the `operation_chat:v1` signing purpose separated
  from audit-log cursors. It is not an authorization credential. The backend
  rechecks authorization and resolves the persisted association on every page.
- This is keyset navigation over persisted messages, **not a frozen snapshot**.
  There is no ingest watermark or guarantee that later pages describe a
  single historical database snapshot.

Example first request (replace the sample Bearer token):

```http
GET /admin/operations/sp-42/conversation?limit=2
Authorization: Bearer <access-token>
X-Audit-Reason: Investigate support incident 123
X-Request-ID: support-chat-page-1
```

If the response provides a non-null `next_cursor`, send its exact value in
`cursor` with the same reason and a new request correlation ID. Do not decode
or construct tokens, or reuse one for another operation.

## Audit and private media preparation

The use case reads the bounded message page and saves **one shared audit
event** before delivering messages or resolving private media access:

- `ActionAccess` / `ResultPrepared`;
- local operator ID, primary `job_request` or `service_proposal` resource with
  its integer ID as a string, and optional typed positive `ConversationID`;
- trimmed reason, occurrence time and effective request correlation ID.

`prepared` does not prove client receipt, display or reading. Each page has
its own audit event; pagination does not reuse a previous event. Audit failure
returns `500` without a partial chat response or media access URL generation.
No chat text, attachment data or URLs enter the event, and no second chat-
specific audit mechanism is introduced. See the [audit component policy](admin-audit.md).

After a successful audit, attachment references are batch-read only for
**delivered page messages**, the associated work conversation and matching
participants. The file uploader must match the message's persisted sender.
Existing file policy admits only available confirmed, private message-image,
message-audio or message-video files before resolving signed download URLs.
Another conversation's attachments, messages outside the delivered page and
foreign sender files are not signed. Unavailable media is omitted; image,
audio and video fields are optional, so text-only messages retain only their
ID, sender role, content and timestamp. No public visibility change, raw internal
storage-key/bucket fields, credentials or provider payload is returned.

The administrative read does not change conversation/request state, message
history or participant membership, create notifications, or register realtime
connections. Ordinary operation detail continues to return no chat content.

## HTTP outcomes

| Status | Meaning |
| --- | --- |
| `200` | Audited page, including a valid empty conversation. |
| `400` | Malformed operation ID, missing/invalid reason or invalid query/cursor binding. |
| `401` | Missing or invalid Bearer token. |
| `403` | Missing `read:admin_chat_audit`, independently of other admin permissions. |
| `404` | Stable operation or its matching persisted work conversation/participants not found. |
| `500` | Identity, persistence, audit or media preparation failure; no partial chat response. |
