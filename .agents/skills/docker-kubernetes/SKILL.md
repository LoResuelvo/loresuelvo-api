---
name: docker-kubernetes
description: Use when managing Docker setup, docker-compose configurations, or preparing deployment. Ensures proper multi-stage builds, development vs production separation, and nginx reverse proxy configuration.
---

# Docker & Kubernetes

## Current Setup

- `Dockerfile` — multi-stage build (builder + alpine runtime).
- `Dockerfile.dev` — development with live reloading.
- `docker-compose.yml` — local dev: api-dev, swagger-ui, dev-db, test-db.
- `compose.prod.yml` — production: api, nginx, certbot.

## Key Services

### Development (`docker-compose.yml`)
- `api-dev` : Go API on port 8080
- `swagger-ui` : OpenAPI docs
- `dev-db` : PostgreSQL 16 for development
- `test-db` : PostgreSQL 16 for tests

### Production (`compose.prod.yml`)
- `api` : Go API (no exposed port — internal)
- `nginx` : Reverse proxy with SSL (Let's Encrypt)
- `certbot` : TLS certificate management

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

- Location: `nginx/default.conf`
- SSL/TLS 1.2+ only.
- Proxies: api (8080), web (3000), gestion (3000).
- Server names: api.loresuelvo.com.ar, loresuelvo.com.ar, gestion.loresuelvo.com.ar.

## Adding a New Service

1. Add service definition to `docker-compose.yml` / `compose.prod.yml`.
2. If new env vars needed, add to `.env.example` and document.
3. Update `Makefile` if new targets required.
