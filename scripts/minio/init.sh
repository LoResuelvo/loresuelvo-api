#!/bin/sh
set -eu

MINIO_ALIAS="local"
MINIO_API_PORT="${MINIO_API_PORT:-9000}"
MINIO_ENDPOINT="http://minio.localhost:${MINIO_API_PORT}"
PUBLIC_BUCKET="${STORAGE_PUBLIC_BUCKET:-loresuelvo-public-local}"
PRIVATE_BUCKET="${STORAGE_PRIVATE_BUCKET:-loresuelvo-private-local}"

mc alias set "$MINIO_ALIAS" "$MINIO_ENDPOINT" "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD"

mc mb --ignore-existing "$MINIO_ALIAS/$PUBLIC_BUCKET"
mc mb --ignore-existing "$MINIO_ALIAS/$PRIVATE_BUCKET"

# Public profile photos are served as plain public URLs after the API validates
# ownership, purpose, status, mime type and size during upload confirmation.
mc anonymous set download "$MINIO_ALIAS/$PUBLIC_BUCKET"
mc anonymous set none "$MINIO_ALIAS/$PRIVATE_BUCKET"

# Browser clients upload directly through presigned PUT URLs. CORS is configured
# globally on the MinIO server through MINIO_API_CORS_ALLOW_ORIGIN so local dev,
# Swagger and acceptance tests share one predictable policy.

mc ls "$MINIO_ALIAS"
