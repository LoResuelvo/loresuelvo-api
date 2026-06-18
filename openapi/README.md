# OpenAPI source

The API contract is maintained as modular YAML under this directory and bundled
into the root `openapi.json` artifact.

## Structure

- `openapi.yaml` — root document with global metadata, tags, servers, security,
  and references to domain path files/components.
- `paths/` — path items grouped by domain/context (`categories`, `consumers`,
  `providers`, `users`) plus `health`.
- `components/security-schemes.yaml` — shared security schemes.
- `components/schemas/` — reusable request/response schemas.

## Updating the public artifact

After editing any file in `openapi/`, run:

```sh
make openapi
```

This executes `scripts/bundle_openapi.py`, resolving file-based `$ref` values and
rewriting the root `openapi.json`. Internal schema refs such as
`#/components/schemas/ErrorResponse` are preserved intentionally so the bundled
spec remains readable.

## Swagger UI

To view and try the API contract locally, run:

```sh
make swagger
```

This regenerates `openapi.json` and starts the `swagger-ui` compose service on:

```text
http://localhost:8081
```

The OpenAPI `servers` section defaults to `http://localhost:8080`, so local
Swagger UI **Try it out** requests target the development API without selecting a
server per operation. Hosted environments can still select `/ - Current API host`
from Swagger UI's server dropdown. The API development container exposes
`CORS_ALLOWED_ORIGINS=http://localhost:8081`, so local **Try it out** requests
from Swagger UI are allowed. Stop the Swagger UI service with:

```sh
make swagger-down
```
