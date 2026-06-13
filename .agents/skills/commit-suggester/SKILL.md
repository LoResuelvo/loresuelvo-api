---
name: commit-suggester
description: Use when the user asks to suggest, draft, choose, or format a git commit message for the current changes. Follow the project's Conventional Commit style: <type>[optional scope]: <description>, with the allowed types and optional us-x scope convention.
---

# Commit Suggester

When the user asks for a commit suggestion, inspect the current changes and propose a concise commit message using:

```text
<type>[N°US]: <description>
```

## Allowed types

- `feat` — new features or functionality.
- `fix` — production bug fixes.
- `refactor` — structural improvements without behavior change.
- `style` — cosmetic code changes only.
- `test` — adding, modifying, fixing, or improving tests.
- `docs` — documentation, comments, or API descriptions.
- `build` — build process, production dependencies, tooling/config needed for deployment or runtime.
- `ci` — CI/CD workflows or configuration.
- `chore` — administrative/supportive tasks that do not affect production code.
- `revert` — rollback of a previous commit.

Do not use `add` as a commit type; use `feat` for added user-facing behavior or `chore` for supportive additions.

## Scope

- Scope is optional.
- Prefer `us-x` only when the change clearly belongs to a user story or the user explicitly provides it.
- If no clear story scope exists, omit the scope.

Examples:

```text
feat(us-1): protect consumer registration with auth0 token
```

```text
test: update consumer registration acceptance steps
```

## Workflow

1. Run `git status --short` and inspect relevant diffs, usually with `git diff --stat` and `git diff`.
2. Determine the dominant intent of the staged/unstaged changes.
3. Suggest one primary commit message.
4. If changes mix concerns, suggest either:
   - multiple commits grouped by concern, or
   - one broader commit only if splitting is unnecessary.
5. Keep descriptions imperative, lowercase, and under ~72 characters when possible.
6. Do not commit unless the user explicitly asks.

## Response format

Keep the response short:

```text
Sugerencia:
<type>[N°US]: <description>
```

If useful, include alternatives:

```text
Alternativas:
- ...
- ...
```
