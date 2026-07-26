---
name: postgresql-best-practices
description: Use when writing or reviewing repository implementations, units of work, migrations, database transactions, or PostgreSQL queries. Enforces collection-like repositories, generic persistence operations, no business decisions in SQL, and proper database/sql transaction patterns for PostgreSQL 16.
---

# PostgreSQL Best Practices

Uses PostgreSQL 16 through Go `database/sql` with the pgx stdlib driver (`sql.Open("pgx", ...)`). Database connection lives in `internal/infrastructure/db/postgres.go`.

## Connection

- Database URL from `DATABASE_URL` / `TEST_DATABASE_URL` env vars.
- `ConnectPostgres` returns `*sql.DB`.
- Prefer `ExecContext` / `QueryContext` / `QueryRowContext` when context is available; existing repositories may still use `Exec` / `Query` / `QueryRow`.
- Do not introduce `pgxpool` unless the project intentionally migrates away from `database/sql`.

## Migrations

- Location: `db/migrations/`.
- Use `make migrate-up` / `make migrate-down` for dev.
- Use timestamped filenames consistent with the repo: `YYYYMMDDHHMMSS_name.up.sql` and `.down.sql`.
- migrations are NOT run automatically in prod.

## Schema Conventions

- Primary keys: `id SERIAL PRIMARY KEY` (auto-increment).
- Foreign keys: `REFERENCES table(id) ON DELETE CASCADE`.
- Timestamps: `created_on TIMESTAMP NOT NULL DEFAULT NOW()`.
- Soft delete NOT implemented — hard delete only.

## Queries

- Parameterized queries ONLY — no string interpolation.
- Use transactions (`*sql.Tx`) for operations spanning multiple tables.
- Close `sql.Rows` explicitly after iteration.
- SQL belongs in the repository that owns the table. Avoid duplicating table SQL across repositories.
- Let one repository write multiple tables only when those tables persist one aggregate.

```go
row := db.QueryRowContext(ctx, "SELECT id FROM users WHERE auth_id = $1", authID)
```

## Repository Semantics

Treat a repository as an in-memory collection abstraction for domain objects.

- Prefer `Save`, `FindByID`, `FindBy...`, `Delete`, `Exists`, and collection-oriented queries.
- Persist the state supplied by the domain object; do not decide its next state.
- Keep names independent of business outcomes.
- Do not expose separate methods for each domain transition.

```go
// Good
intent.MarkRejected(payment, now)
err := intentRepository.Save(ctx, intent)

// Bad
err := intentRepository.SaveRejected(ctx, intent)
err := intentRepository.RejectPayment(ctx, intent.ID)
```

Reject repository APIs such as:

- `SaveAccepted`, `SaveRejected`, `SaveProcessing`
- `ConfirmPaidBooking`, `CompleteCheckout`
- `ApproveProposal`, `CancelOrder`
- methods whose SQL changes state based on a business precondition

Named queries may describe selection criteria, such as `FindLatestByProposalIDAndPurpose`; they must not imply executing a workflow.

## Database Boundary

- Use foreign keys, uniqueness, nullability, and basic checks for structural integrity.
- Do not encode domain workflows in triggers, stored procedures, status-specific updates, or conditional persistence methods.
- Do not use SQL such as `UPDATE ... WHERE status = 'pending'` as the implementation of a domain transition. Let the domain object validate the transition, then persist its resulting state.
- Translate database constraint failures into domain/persistence errors without turning the database into the policy owner.
- Keep locks in a dedicated lock adapter when locking is a reusable technical concern; do not hide advisory-lock orchestration inside an entity repository.

## Atomic Work Across Aggregates

Use a generic Unit of Work when a use case changes multiple aggregates:

```go
err := unitOfWork.Execute(ctx, func(store TransactionalStore) error {
	if err := store.SaveIntent(ctx, intent); err != nil {
		return err
	}
	if err := store.SaveServiceProposal(ctx, proposal); err != nil {
		return err
	}
	return store.SaveWorkOrder(ctx, order)
})
```

- Keep the business sequence in the domain/application service.
- Keep `TransactionalStore` methods generic.
- Implement each store method by delegating to an unexported table-owner helper such as `saveWithTx`.
- Never expose `*sql.Tx` through a domain port.
- Do not name the Unit of Work method after a workflow such as `ConfirmPaidBooking`.

## JSON Handling

- Use `sql.NullTime` or other `database/sql` nullable types for nullable columns.
- Map JSON fields with struct tags if binding request bodies.

## Transactions

```go
tx, err := db.Begin()
if err != nil {
    return err
}

userID, err := userRepository.saveWithTx(tx, *provider.User)
if err != nil {
    return rollbackProviderTx(tx, err)
}

_, err = tx.Exec("INSERT INTO providers (user_id, category_id) VALUES ($1, $2)", userID, provider.Category.ID)
if err != nil {
    return rollbackProviderTx(tx, err)
}

return tx.Commit()
```

## Cleanup

- Prefer repository `DeleteAll` methods for test cleanup and BDD setup.
- Clean child tables before parent tables when cascade is not enough or when tests need explicit isolation: messages → conversations → users → categories.
