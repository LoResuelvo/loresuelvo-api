# Provider seed data

This folder contains optional seed data for fake provider profiles used to
hydrate local, staging, or production environments. The administrator profile
is deliberately **not** stored here: its Auth0 identity and e-mail are
per-environment secrets and are provisioned by the API's optional startup
seed.

## Optional admin profile

To provision an administrator during startup, provide all four variables:

```bash
# Sensitive values: inject from the environment's secret manager.
ADMIN_SEED_AUTH_ID=auth0|<user-id-from-the-environment-tenant>
ADMIN_SEED_EMAIL=<admin-email-from-the-environment-tenant>
# Non-secret profile metadata.
ADMIN_SEED_NAME=<admin-name>
ADMIN_SEED_SURNAME=<admin-surname>
```

`ADMIN_SEED_AUTH_ID` and `ADMIN_SEED_EMAIL` must not be committed, printed in
logs, or copied between tenants. Auth0 creates the identity; the API creates
the corresponding local `users` row with `role = 'admin'`. This seed runs
before the API accepts requests, independently of `SEEDS_ENABLED`, and is
idempotent for the same values. When all four variables are absent, the API
skips the admin seed. A partial configuration, invalid values, or conflicting
values fail startup rather than promoting or overwriting an existing user. Do
not add the admin to a YAML seed file or provision it with a manual SQL query.

## Generate provider seeds

```bash
python3 scripts/generate_provider_seed.py \
  --count 100 \
  --output seeds/providers-100.yaml \
  --manifest seeds/providers-100-assets.tsv \
  --assets-dir seeds/assets/provider_profile_photo
```

The generated YAML uses the existing seed format consumed by `SEEDS_FILE`.
The TSV manifest maps a reusable local WebP asset to each provider-specific
public object key.

## Local development

`make up` runs the development initialization in dependency order: it waits
for PostgreSQL, applies all migrations, creates the MinIO buckets, uploads the
seed assets when seeds are enabled, and only then starts the API. The API
applies the YAML seed synchronously before opening port 8080.

The default local configuration uses:

```bash
SEEDS_ENABLED=true
SEEDS_FILE=seeds/providers-100.yaml
```

To upload only the assets manually, use `make seed-assets-local`.

Production and staging deployment configuration is owned by `infra-devops`.
Environments that use the admin seed must map the two sensitive values to its
secret store and the name/surname values to ordinary deployment configuration.
