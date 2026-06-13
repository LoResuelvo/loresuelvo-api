---
name: api-design-principles
description: Use when creating new endpoints, reviewing API design, or extending the REST API. Ensures RESTful conventions, proper HTTP methods/status codes, and consistent OpenAPI documentation.
---

# API Design Principles

REST API with OpenAPI spec. Base URL: `http://localhost:8080`.

## Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/` | No | Health check |
| GET | `/categories` | Bearer JWT | List all categories |
| POST | `/categories` | Bearer JWT | Create category |
| POST | `/consumers` | Bearer JWT | Register consumer |
| GET | `/providers` | Bearer JWT | Filter providers by category |
| POST | `/providers` | Bearer JWT | Register provider |
| GET | `/me` | Bearer JWT | Current user info |

## REST Conventions

- **GET** — read/list resources, no request body.
- **POST** — create resources, return 201 with Location header.
- Use query params for filtering (`?category_id=1`).
- Use path params for resource identification (`/resources/:id`).

## HTTP Status Codes

| Code | Use |
|------|-----|
| 200 | Successful read/list |
| 201 | Resource created |
| 400 | Bad request (validation error) |
| 401 | Unauthorized (missing/invalid token) |
| 403 | Forbidden (valid token but no permission) |
| 404 | Resource not found |
| 409 | Conflict (duplicate, e.g., email exists) |
| 500 | Internal server error |

## Request/Response

- Use JSON for request/response bodies.
- Define structs with `json:"fieldName"` tags.
- Return consistent error structure: `{"error": "message"}`.
- Include `Location` header on 201 pointing to created resource.

## OpenAPI

- Modular YAML sources in `openapi/`.
- Bundle with `python3 scripts/bundle_openapi.py` → `openapi.json`.
- Update OpenAPI when adding/modifying endpoints.
- Document all error responses.

## Authentication

- All protected endpoints require `Authorization: Bearer <token>`.
- JWT `sub` claim = user's internal `auth_id`.
