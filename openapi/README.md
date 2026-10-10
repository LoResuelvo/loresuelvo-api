# OpenAPI source

The API contract is maintained as modular YAML under this directory and bundled
into the root `openapi.json` artifact.

## Structure

- `openapi.yaml` — root document with global metadata, tags, servers, security,
  and references to domain path files/components.
- `paths/` — path items grouped by domain/context (`categories`, `consumers`,
  `providers`, `users`) plus `health`.
- `components/security-schemes.yaml` — shared security schemes.
- `components/schemas/` — reusable request/response schemas.

## Updating the public artifact

After editing any file in `openapi/`, run:

```sh
make openapi
```

This executes `scripts/bundle_openapi.py`, resolving file-based `$ref` values and
rewriting the root `openapi.json`. Internal schema refs such as
`#/components/schemas/ErrorResponse` are preserved intentionally so the bundled
spec remains readable.

## Swagger UI

To view and try the API contract locally, run:

```sh
make swagger
```

This regenerates `openapi.json` and starts the `swagger-ui` compose service on:

```text
http://localhost:8081
```

The OpenAPI `servers` section defaults to `http://localhost:8080`, so local
Swagger UI **Try it out** requests target the development API without selecting a
server per operation. Hosted environments can still select `/ - Current API host`
from Swagger UI's server dropdown. The API development container exposes
`CORS_ALLOWED_ORIGINS=http://localhost:8081`, so local **Try it out** requests
from Swagger UI are allowed. Stop the Swagger UI service with:

```sh
make swagger-down
```

## Phone notices (US-20)

`PUT /installations/{installation_id}` registers or renews a phone with a JWT.
`DELETE` on the same path unlinks its login using the required JSON body.
The path and request schemas document installation possession, role checks,
token renewal, account changes, idempotency, and stale-binding conflicts.
Clients privately retain the installation ID and secret across logins, generate
a new binding UUID for each login, and preserve the latest binding for the next
registration. Credentials are never returned or included in push data.

[`PushNotice`](components/schemas/push-notice.yaml) describes FCM `message.data`:
all 13 fields are strings. The API uses constant Spanish/English title and body
templates, with Spanish as default. Android uses that text, validates the current
recipient/app/installation/binding, discards expired or superseded notices, and
deduplicates using the stable event ID. It suppresses the system popup for an
active chat only on that installation. WebSocket continues updating screens.

| Confirmed trigger / push type | Recipient | FCM destination |
| --- | --- | --- |
| New text/image/audio/video message / `conversation.message.created` | Counterpart, excluding author | Conversation |
| New job request / `job_request_received` | Assigned provider only | Job request |
| Proposal received / `service_proposal_received` | Consumer | Proposal |
| Verified deposit and accepted proposal / `service_proposal_accepted` | Provider | Generated work order |
| Existing appointment reminder rule / `work_order_close_to_scheduled_time` | Both participants | Work order |
| Completion reported / `work_order_completion_reported` | Consumer | Work order |
| Verified balance approval and paid order / `work_order_final_payment_approved` | Provider only | Work order |

One message produces one notice regardless of attachment count. Reminders keep
their existing deduplication and expire at the scheduled time; other notices
expire 24 hours after creation. The final-payment notice is saved atomically
with the payment and paid order, then also emitted as `notification.created`.
Repeated confirmations create no new notice; pending/rejected payments and
browser returns do not trigger it. Approval does not imply withdrawable funds.
Calendar and rejected-proposal notices remain outside the FCM contract.

FCM sends originate only on the operation's producer, after successful
persistence, using the existing `MessagePublisher` and `Notificator` ports
composed with WebSocket/RabbitMQ. Broadcast consumers never send FCM. Channel
attempts are independent; FCM failure cannot reverse a saved operation. Missing
installations are harmless. Confirmed invalid tokens disable only the captured
installation revision; transient, authentication, and generic payload errors
retain registrations. A refreshed token can restore a live binding, while a
logged-out binding requires a new login. Delivery is best effort: no outbox or
persistent retries, no immediate or exactly-once delivery guarantee. Opening
the app retrieves the saved business data through authenticated API calls.

Sending defaults to disabled (`FCM_ENABLED=false`), preserving WebSocket.
Enable with `FCM_ENABLED=true`, `FCM_PROJECT_ID`, and Google Application Default
Credentials with the `firebase.messaging` scope and sending permission for that
environment. `GOOGLE_APPLICATION_CREDENTIALS` may point to a privately mounted
credential file; do not put keys in images or version control. `FCM_TIMEOUT`
defaults to `5s` and accepts `1s` through `30s`, bounding each whole delivery batch
including lookups and authenticated sends. See [`.env.example`](../.env.example).
Deterministic tests use a fake FCM HTTP boundary; real device delivery and
environment configuration are coordinated with the linked Android/infra issues
in [API issue 97](https://github.com/LoResuelvo/loresuelvo-api/issues/97).
