#!/bin/sh
set -eu

MINIO_ALIAS="local"
MINIO_API_PORT="${MINIO_API_PORT:-9000}"
MINIO_ENDPOINT="http://minio.localhost:${MINIO_API_PORT}"
PUBLIC_BUCKET="${STORAGE_PUBLIC_BUCKET:-loresuelvo-public-local}"
PRIVATE_BUCKET="${STORAGE_PRIVATE_BUCKET:-loresuelvo-private-local}"
TEST_PUBLIC_BUCKET="${TEST_STORAGE_PUBLIC_BUCKET:-loresuelvo-public-test}"
TEST_PRIVATE_BUCKET="${TEST_STORAGE_PRIVATE_BUCKET:-loresuelvo-private-test}"

mc alias set "$MINIO_ALIAS" "$MINIO_ENDPOINT" "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD"

mc mb --ignore-existing "$MINIO_ALIAS/$PUBLIC_BUCKET"
mc mb --ignore-existing "$MINIO_ALIAS/$PRIVATE_BUCKET"
mc mb --ignore-existing "$MINIO_ALIAS/$TEST_PUBLIC_BUCKET"
mc mb --ignore-existing "$MINIO_ALIAS/$TEST_PRIVATE_BUCKET"

# Public profile photos are served as plain public URLs after the API validates
# ownership, purpose, status, mime type and size during upload confirmation.
mc anonymous set download "$MINIO_ALIAS/$PUBLIC_BUCKET"
mc anonymous set none "$MINIO_ALIAS/$PRIVATE_BUCKET"
mc anonymous set download "$MINIO_ALIAS/$TEST_PUBLIC_BUCKET"
mc anonymous set none "$MINIO_ALIAS/$TEST_PRIVATE_BUCKET"

# Browser clients upload directly through presigned PUT URLs. CORS is configured
# globally on the MinIO server through MINIO_API_CORS_ALLOW_ORIGIN so local dev,
# Swagger and acceptance tests share one predictable policy.

# Seed the public bucket with the provider profile photos referenced by
# `seeds/providers-100-assets.tsv` (manifest mounted at /seed-manifest, source
# assets mounted at /seed-assets). The TSV columns are
# `source_asset_path\ttarget_object_key`; the target is what the API will
# expose as `profile_photo_url`. Skipping missing source assets keeps the
# script idempotent across partial seed additions.
ASSETS_MANIFEST="${SEED_ASSETS_MANIFEST:-/seed-manifest/providers-100-assets.tsv}"
if [ -f "$ASSETS_MANIFEST" ]; then
    while IFS="$(printf '\t')" read -r source target; do
        # Skip blank lines and comment lines (start with `#`).
        [ -z "$source" ] && continue
        case "$source" in '#'*) continue ;; esac
        if [ -f "/seed-assets/$source" ]; then
            mc cp "/seed-assets/$source" "$MINIO_ALIAS/$PUBLIC_BUCKET/$target" >/dev/null
        else
            echo "init.sh: missing seed asset '$source' — skipping" >&2
        fi
    done < "$ASSETS_MANIFEST"
fi

mc ls "$MINIO_ALIAS"
