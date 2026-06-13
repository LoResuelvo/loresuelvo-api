---
name: architecture-best-practices
description: Use when designing new features, reviewing code structure, or extending the domain layer. Ensures proper hexagonal architecture boundaries: domain is pure, adapters implement interfaces, infrastructure connects external systems.
---

# Architecture Best Practices

Project follows Clean Architecture / Hexagonal Architecture.

## Layer Rules

```
cmd/api/main.go
  -> bootstrap/dependencies.go   (DI container)
    -> adapters/http/router.go   (HTTP adapter)
      -> handlers                (depend on domain services)
        -> domain/*/service.go   (pure business logic)
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
- Repository methods must describe persistence operations, not business workflow:
  - Good: `SaveWithMessage`, `FindBetween`, `DeleteBetween`, `ExistsBetween`
  - Bad: `CreatePending`, `StartWorkRequest`
- SQL should live in the repository that owns the table. For multi-table atomic writes, let the coordinator repository own the transaction and call unexported helpers on the table owner, e.g. `userRepository.saveWithTx(tx, user)` or `messageRepository.saveWithTx(tx, ...)`.
- Keep transaction helpers unexported unless an external adapter truly needs them; do not expose `*sql.Tx` in domain interfaces.

## Adding a New Feature

1. Create `domain/<entity>/entity.go` — struct with exported fields.
2. Create `domain/<entity>/errors.go` — sentinel errors.
3. Create `domain/<entity>/repository.go` — repository/capability interface definitions.
4. Create `domain/<entity>/service.go` — business logic implementing service interface.
5. Create `adapters/repositories/<entity>_repository.go` — implements repository interface.
6. Create `adapters/http/handler/<entity>.go` — HTTP handlers using service interface.
7. Register routes in `adapters/http/router.go`.
8. Add BDD feature and steps in `features/`.

## Dependency Injection

- All wiring in `bootstrap/dependencies.go`.
- Pass concrete implementations (adapters/repositories) to domain services via interfaces.
