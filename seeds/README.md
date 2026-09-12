# Provider seed data

This folder contains optional seed data for fake provider profiles used to hydrate
local, staging, or production environments.

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
