---
name: commit-suggester
description: Use when the user asks to suggest, draft, choose, or format a git commit message for the current changes. Follow the project's Conventional Commit style: <type>[N°US]: <description>, with the allowed types and bracketed user-story number convention.
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

- For a user story, put its number in square brackets: `feat[60]: description`.
- Do not use parentheses or a `us-` prefix.
- If no user story applies, omit the bracketed number.

Examples:

```text
feat[1]: protect consumer registration with auth0 token
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
