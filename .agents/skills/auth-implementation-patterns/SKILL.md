---
name: auth-implementation-patterns
description: Use when implementing or reviewing authentication/authorization logic. Covers Auth0 JWT validation, JWT middleware setup, custom claims, permission-based access control, and test authentication patterns.
---

# Auth Implementation Patterns

## Auth0 JWT Validation

Validator implemented in `internal/adapters/auth0/validator.go`.

- Uses JWKS (JSON Web Key Set) for signature verification.
- Caches JWKS with 5-minute TTL.
- Validates RS256 algorithm.
- Domain: `AUTH0_DOMAIN`, Audience: `AUTH0_AUDIENCE` from env.

## JWT Middleware

HTTP middleware in `internal/adapters/http/middleware/auth.go`:

```go
func AuthMiddleware(validator *auth0.Validator) gin.HandlerFunc
```

Applied per-route in `router.go`. Extracts token from `Authorization: Bearer <token>` header.

## Custom Claims

JWT includes standard claims + custom claims (permissions, scopes). Project validates:

- Permissions via `RequirePermissionLayer` middleware.
- Scope whitespace must be clean (no double spaces, trimmed).

## Middleware Chain

```
gin.New() -> RecoveryMiddleware() -> CORSLayer() -> [route-specific auth middleware] -> handlers
```

## Handler Patterns

Auth info extracted per-request:

```go
claims, ok := GetClaimsFromContext(c.Request().Context())
if !ok {
    c.AbortWithStatus(401)
    return
}
authID := claims.Sub // internal user identifier
```

Never expose Auth0 IDs directly. Use internal `auth_id` from `users` table.

## Testing Auth

Use `testhelper/fakevalidator.go` instead of real Auth0 in tests. Provides fake token generation for test scenarios.

## User Roles

- `consumer` — registers via `/consumers`
- `provider` — registers via `/providers` with category

Role stored in `users.role` column.
