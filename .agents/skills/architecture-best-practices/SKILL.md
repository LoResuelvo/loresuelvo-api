---
name: architecture-best-practices
description: Use when designing features, reviewing code structure, extending the domain layer, refactoring large services, or deciding where business behavior belongs. Enforces hexagonal boundaries, rich domain models, Tell Don't Ask, object collaboration, polymorphic dispatch, and repository ports without business workflows.
---

# Architecture Best Practices

Project follows Clean Architecture / Hexagonal Architecture.

## Layer Rules

```
cmd/api/main.go
  -> bootstrap/dependencies.go   (DI container)
    -> adapters/http/router.go   (HTTP adapter)
      -> handlers                (depend on domain services)
        -> domain entities/value objects/policies (business decisions)
          -> domain/*/service.go (use-case orchestration)
            -> repository/capability interfaces (defined here, NOT in adapters)
        -> adapters/repositories (implement domain interfaces using database/sql)
        -> adapters/auth0        (JWT validation adapter)
internal/
  domain/        # Entities, service interfaces, service implementations, errors
  adapters/      # HTTP handlers, repositories (implement domain interfaces)
  infrastructure/# DB connection (postgres.go), external services
```

## Critical Rules

1. **Domain must never import adapters or infrastructure.**
2. **Repository interfaces live in domain, not in adapters.**
3. **Handlers depend on domain services (via interfaces), never on repositories directly.**
4. **Infrastructure knows nothing about domain — only provides raw connections.**
5. **Cross-domain dependencies should use small capability ports owned by the consuming package.**
6. **Services/ports return domain entities or domain value objects, not HTTP/read DTOs.**
7. **DTOs and response-shaping structs live in adapters (HTTP/OpenAPI), then map from domain entities at the boundary.**
8. **Put business decisions in domain objects before adding conditions to a service.**
9. **Keep services as use-case coordinators; do not turn them into procedural domain models.**

## Rich Domain Model

- Ask first which entity, aggregate, value object, policy, or domain strategy owns a rule.
- Put valid state transitions behind intention-revealing methods such as `Expire`, `Accept`, `Apply`, or `PrepareCheckout`.
- Keep invariants next to the state they protect. Reject invalid transitions inside the owning domain object.
- Let services load collaborators, invoke domain behavior, coordinate ports, and define transaction boundaries.
- Do not let services inspect fields and reproduce a transition with assignments.

```go
// Good: tell the aggregate what happened.
outcome, err := verifiedPayment.ApplyTo(intent, processor, now)

// Bad: ask for state and reproduce its rules procedurally.
if intent.Status == StatusReady && payment.Status == StatusApproved {
	intent.Status = StatusPaid
}
```

Use a domain service or policy only when a rule genuinely spans objects and has no natural entity or value-object owner. Keep such objects pure and named after the domain rule, not after infrastructure.

## Object Collaboration

- Prefer Tell Don't Ask: send commands to objects instead of extracting state and making their decisions elsewhere.
- Prefer small collaborators with one behavioral role over long `if`/`switch` chains in application services.
- Represent meaningful variants with polymorphism when behavior changes by type or state.
- Apply double dispatch when an input variant and an outcome/target variant jointly determine behavior, especially for payment states, commands, events, and workflow outcomes.
- Keep one exhaustive variant selection at the boundary or factory, then dispatch through behavior.
- Use a visitor/outcome collaborator when each result requires different orchestration after the domain transition.
- Do not add double dispatch for a binary guard or a stable one-line condition; use it when it removes repeated branching and makes variants independently extensible.

Before accepting a branching service, check:

1. Does the condition protect an invariant owned by an entity?
2. Can the input become a strategy or command with `ApplyTo`?
3. Can the result dispatch to a visitor instead of making the service inspect a status?
4. Will a new variant require editing several branches? If yes, introduce polymorphic collaboration.

## Domain Entities vs DTOs

- Model the domain entity/aggregate needed by the use case instead of creating parallel DTOs in `internal/domain`.
  - Good: `Conversation` owns `Messages []Message` when the use case is "get a conversation with its messages".
  - Bad: `ConversationDetail` / `MessageDetail` domain structs that exist only to satisfy an HTTP response.
- Do not add fields to domain entities solely for BDD/API convenience (for example `SenderEmail` when the domain message already has IDs/roles).
- Resolve human-readable identifiers used in scenarios (emails, names) in the test/adapter layer by mapping them to domain IDs or roles.
- Keep response names like `ConversationDetailResponse` in the HTTP adapter, where they describe an API representation rather than a domain concept.
- Passing `context.Context` from `c.Request.Context()` into services/repositories is idiomatic for cancellation/timeouts. Pass the standard-library context only; never pass `*gin.Context` into domain or repositories.
- When adding new service/repository methods that touch request-scoped work, prefer `ctx context.Context` as the first parameter. Avoid broad legacy rewrites unless the change is already in scope.

## Ports and Repository Boundaries

- Define interfaces where they are consumed, with the smallest capability needed:
  - `ConsumerIDFinder` for `FindIDByAuthID`
  - `ProviderExistenceChecker` for `ExistsByID`
  - `CategoryFinder` for `FindByID`
- You can still inject the full concrete repository in `bootstrap`; Go satisfies the narrow interface implicitly.
- Prefer capability names (`Finder`, `Checker`, `Store`) when the interface is not a full repository.
- Model repositories like collections of aggregates:
  - Good: `Save`, `FindByID`, `FindByProviderID`, `FindBetween`, `Delete`, `Exists`
  - Bad: `SaveRejected`, `MarkPaid`, `AcceptProposal`, `ConfirmBooking`, `StartWorkRequest`
- Pass a domain object already transitioned by domain behavior to `Save`; never ask a repository to perform that transition.
- SQL should live in the repository that owns the table. Let a repository coordinate multiple tables only when they belong to the same aggregate.
- Keep transaction helpers unexported unless an external adapter truly needs them; do not expose `*sql.Tx` in domain interfaces.
- Use a generic `UnitOfWork` when one use case must persist multiple aggregates or repositories atomically. Its transactional store may expose only generic persistence operations and must not encode the use-case decision sequence.

## Adding a New Feature

1. Create `domain/<entity>/entity.go` — entity/aggregate with state and invariant-preserving behavior.
2. Create `domain/<entity>/errors.go` — sentinel errors.
3. Create `domain/<entity>/repository.go` — repository/capability interface definitions.
4. Create `domain/<entity>/service.go` — thin use-case orchestration around domain behavior and ports.
5. Create `adapters/repositories/<entity>_repository.go` — implements repository interface.
6. Create `adapters/http/handler/<entity>.go` — HTTP handlers using service interface.
7. Register routes in `adapters/http/router.go`.
8. Add BDD feature and steps in `features/`.

## Dependency Injection

- All wiring in `bootstrap/dependencies.go`.
- Pass concrete implementations (adapters/repositories) to domain services via interfaces.
