---
name: security-best-practices
description: Use when building, modifying, or reviewing API endpoints, auth logic, or data handling. Ensures JWT/auth0 validation, CORS, SQL injection prevention, and input validation are implemented correctly.
---

# Security Best Practices

This project exposes a REST API secured with Auth0 JWT. All endpoints except `/` require Bearer tokens.

## JWT / Auth0

- Validate JWT on every protected route using `auth0.Validatore` from `internal/adapters/auth0`.
- Extract `sub` claim as internal `auth_id` — never expose Auth0 IDs directly.
- Validate audience (`AUD0_AUDIENCE`) and domain (`AUTH0_DOMAIN`) from env.
- Reject tokens with invalid signature, expired, or missing required claims.
- Do NOT log token contents or error details that could leak auth secrets.

## CORS

- Allowed origins configured via `CORS_ALLOWED_ORIGINS` env var.
- Never accept wildcard `*` in production for `Access-Control-Allow-Origin`.
- Only allow `Authorization` and `Content-Type` headers in CORS exposed headers.

## Input Validation

- Email: use `validator.ValidateEmail()` — regex-based, covers standard formats.
- Category name: must not be empty or whitespace-only after trim.
- IDs (category_id, user_id): must be positive integers.
- Request bodies: always use `ShouldBindJSON` with struct tags.
- Never trust user input — validate before DB operations.

## SQL Injection Prevention

- All DB queries use `pgx` parameterized queries — no string concatenation.
- Never interpolate user input directly into SQL strings.

## Error Responses

- Return generic errors to clients: `"authorization required"` not `"token expired"`.
- Log detailed errors server-side for debugging, never expose to clients.
- Use proper HTTP status codes: 401 for auth failures, 403 for forbidden, 400 for bad input.

## Secrets Management

- All secrets via env vars, never hardcoded.
- `.env` is gitignored — never commit real credentials.
