---
name: postgresql-best-practices
description: Use when writing repository implementations, migrations, or database queries. Ensures proper use of pgx, migrations, connection pooling, and transaction patterns with this project's PostgreSQL 16 setup.
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
- For atomic multi-table writes, the coordinating repository starts the transaction and delegates table-specific SQL to unexported helpers, e.g. `userRepository.saveWithTx(tx, user)` or `messageRepository.saveWithTx(tx, ...)`.

```go
row := db.QueryRowContext(ctx, "SELECT id FROM users WHERE auth_id = $1", authID)
```

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
