# AGENTS.md

Project knowledge base for LoResuelvo API — a service marketplace connecting consumers with providers in Argentina.

## Project Overview

- **Type**: REST API service marketplace
- **Stack**: Go 1.26.1, Gin, PostgreSQL 16, Auth0 JWT, Docker, OpenAPI/Swagger
- **Architecture**: Hexagonal (Clean Architecture)
- **Testing**: BDD (godog), unit tests, integration tests

## Skills

Referenced skills are located in `.agents/skills/<skill-name>/SKILL.md`.

### Core Development

| Skill | Description |
|-------|-------------|
| [golang-pro](.agents/skills/golang-pro/SKILL.md) | Idiomatic Go patterns, error handling, concurrency |
| [testing-best-practices](.agents/skills/testing-best-practices/SKILL.md) | BDD, unit, and integration test patterns |
| [architecture-best-practices](.agents/skills/architecture-best-practices/SKILL.md) | Hexagonal architecture, layer boundaries |

### Security & Auth

| Skill | Description |
|-------|-------------|
| [security-best-practices](.agents/skills/security-best-practices/SKILL.md) | JWT auth, CORS, input validation, SQL injection prevention |
| [auth-implementation-patterns](.agents/skills/auth-implementation-patterns/SKILL.md) | Auth0 JWT validation, middleware, permission layers |

### Infrastructure & APIs

| Skill | Description |
|-------|-------------|
| [postgresql-best-practices](.agents/skills/postgresql-best-practices/SKILL.md) | pgx queries, migrations, transactions, connection pooling |
| [docker-kubernetes](.agents/skills/docker-kubernetes/SKILL.md) | Docker Compose dev/prod, multi-stage builds, nginx |
| [api-design-principles](.agents/skills/api-design-principles/SKILL.md) | REST conventions, HTTP status codes, OpenAPI |

### Additional

| Skill | Description |
|-------|-------------|
| [commit-suggester](.agents/skills/commit-suggester/SKILL.md) | Conventional commit formatting |

## Quick Reference

### Architecture Conventions
- Domain services/ports should return domain entities or value objects, not HTTP/read DTOs.
- Response DTOs and OpenAPI schemas belong to the HTTP adapter/API boundary.
- Optimized read projections may use explicit reader/query ports (for example `ConversationSummaryReader`) returning domain read models; keep those models under the related domain folder in `read_model/` when they are use-case outputs, not HTTP DTOs.
- Aggregates should model their owned data when the use case needs it (for example, `Conversation` owns `Messages []Message` when returning a conversation with its messages).
- Do not add fields to domain entities only to simplify BDD/API assertions; resolve scenario-facing identifiers like emails to IDs/roles in steps or adapters.
- Pass standard-library `context.Context` for request-scoped service/repository methods when useful; extract it in handlers with `c.Request.Context()` and never pass `*gin.Context` into domain or repositories.

### Tech Stack
- **Language**: Go 1.26.1
- **Framework**: Gin v1.12.0
- **Database**: PostgreSQL 16 + pgx/v5
- **Auth**: Auth0 JWT (RS256)
- **Testing**: godog (BDD), testify, sqlmock
- **Container**: Docker, Docker Compose, Nginx

### Key Paths
```
cmd/api/main.go                    # Entry point
internal/
  bootstrap/dependencies.go        # DI container
  domain/                          # Business logic (entities, services, errors, repository interfaces)
  adapters/
    http/                          # Handlers, middleware, router
    repositories/                  # PostgreSQL implementations
    auth0/                          # Auth0 validator
  infrastructure/
    db/                            # DB connection
features/
  consumer/                        # Consumer BDD features
  conversation/                    # Conversation BDD features shared by consumer/provider perspectives
  provider/                        # Provider BDD features
  steps/                           # Godog step definitions
openapi/                           # Modular OpenAPI YAML sources
db/
  migrations/                       # PostgreSQL migrations
```

### Commands
```bash
make up              # Start dev environment
make test            # Run tests with migrations
make lint            # Run linter
make migrate-up      # Run database migrations
make openapi         # Bundle OpenAPI spec
```

### Testing Conventions
- Prefer `make test` for the full suite because it runs migrations against `TEST_DATABASE_URL`.
- BDD steps must not write raw SQL; use repositories/helpers or HTTP calls.
- Spanish is acceptable in Gherkin scenario text and step regex strings only; Go identifiers, comments, errors, and internal contracts remain in English.
- When Gherkin uses emails or names, steps should map them to domain IDs/roles instead of forcing those fields into domain entities or API responses.

### Environment Variables
```bash
AUTH0_DOMAIN         # Auth0 tenant domain
AUTH0_AUDIENCE       # Auth0 API audience
DATABASE_URL         # PostgreSQL connection string (dev)
TEST_DATABASE_URL    # PostgreSQL connection string (test)
CORS_ALLOWED_ORIGINS # Allowed CORS origins
```
