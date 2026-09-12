---
name: docker-kubernetes
description: Use when managing Docker setup, docker-compose configurations, or preparing deployment. Ensures proper multi-stage builds, development vs production separation, and nginx reverse proxy configuration.
---

# Docker & Kubernetes

## Current Setup

- `Dockerfile` — multi-stage build (builder + alpine runtime).
- `Dockerfile.dev` — development with live reloading.
- `docker-compose.yml` — local dev: api-dev, swagger-ui, dev-db, test-db.
- Production deployment is owned by the `infra-devops` repository.

## Key Services

### Development (`docker-compose.yml`)
- `api-dev` : Go API on port 8080
- `swagger-ui` : OpenAPI docs
- `dev-db` : PostgreSQL 16 for development
- `test-db` : PostgreSQL 16 for tests

## Common Commands

```bash
make up           # Start dev environment
make down         # Stop dev environment
make build        # Build production image
make clean        # Remove containers and volumes
make bash         # Shell into api container
make swagger      # Start Swagger UI standalone
```

## Nginx

- Local development only: `nginx/dev.conf.template`.
- Serves local Android App Links configuration and proxies the API and web app.
- Production gateway configuration lives in `infra-devops`.

## Adding a New Service

1. Add local services to `docker-compose.yml`; add deployed services to `infra-devops`.
2. If new env vars needed, add to `.env.example` and document.
3. Update `Makefile` if new targets required.
