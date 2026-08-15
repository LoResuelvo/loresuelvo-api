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

## Production S3/R2 upload

Upload the image objects before enabling the DB seed:

```bash
docker compose -f compose.prod.yml --profile tools run --rm seed-assets
```

Required environment variables:

```bash
STORAGE_PUBLIC_BUCKET=<public bucket>
STORAGE_ENDPOINT=<optional S3-compatible endpoint>
STORAGE_REGION=<region>
STORAGE_ACCESS_KEY_ID=<access key>
STORAGE_SECRET_ACCESS_KEY=<secret key>
```

After the upload, deploy or restart the API with:

```bash
SEEDS_ENABLED=true
SEEDS_FILE=seeds/providers-100.yaml
```

The seed is idempotent. Disable `SEEDS_ENABLED` after verification if the
environment should not keep reapplying seed data on startup.
