---
name: golang-pro
description: Use when writing or reviewing Go code in this project. Ensures idiomatic Go 1.26+ patterns, proper error handling, concurrency safety, and adherence to project conventions.
---

# Go Pro

Project uses Go 1.26.1 with Gin framework, PostgreSQL via `database/sql` + pgx stdlib driver, and hexagonal architecture.

## Error Handling

- Use sentinel errors (`errors.New`) for domain errors, never bare `fmt.Errorf` without context.
- Wrap errors with `fmt.Errorf("context: %w", err)` when propagating through layers.
- Return early on errors — avoid else branches.
- Never discard errors with `_`.

```go
// BAD
if err != nil {}
user, _ := repo.FindByID(ctx, id)

// GOOD
if err != nil {
    return fmt.Errorf("finding user: %w", err)
}
user, err := repo.FindByID(ctx, id)
if err != nil {
    return err
}
```

## Context Propagation

- For request-scoped service/repository methods, pass `context.Context` as the first parameter so cancellations and timeouts propagate cleanly.
- Never pass `*gin.Context` into domain services or repositories; extract the standard-library context at the HTTP boundary with `c.Request.Context()`.
- Never use `context.Background()` in request handlers; reserve it for tests, bootstrap, CLIs, or truly process-scoped work.

## Database

- Use parameterized queries — never string concatenation.
- Prefer `*sql.Tx` for multi-step operations that need atomicity.
- Close rows explicitly when using `sql.Rows`.

## Naming

- `snake_case.go` for files, `PascalCase` for exported types/funcs, `camelCase` for unexported.
- Domain errors: `Err` prefix (e.g., `ErrAlreadyExists`).
- Code identifiers, constants, comments, errors, and internal contracts must be in English.
- Spanish is only acceptable in Gherkin scenario text and step regex strings.
- Name small interfaces by capability when they are narrower than a repository: `CategoryFinder`, `ProviderExistenceChecker`, `ConsumerIDFinder`.

## Project Structure

```
internal/
  domain/        # Pure business logic, no external dependencies
  adapters/      # HTTP, DB, Auth0 implementations
  infrastructure/# DB connection, external services
```

- Domain layer must never import adapters or infrastructure.
- Repository interfaces defined in domain, implementations in adapters.

## Concurrency

- Use `sync.Mutex` or `sync.RWMutex` for simple in-memory state.
- Use `errgroup` with `context.Context` for fan-out operations.
- Always check for context cancellation in long-running loops.

## Logging

- Uses `log/slog` for structured logging.
- Include key-value pairs: `slog.Info("msg", "operation", "find", "id", id)`.
