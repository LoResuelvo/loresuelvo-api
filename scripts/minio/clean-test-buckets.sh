#!/bin/sh
set -eu

MINIO_ALIAS="local"
MINIO_API_PORT="${MINIO_API_PORT:-9000}"
MINIO_ENDPOINT="http://minio.localhost:${MINIO_API_PORT}"
TEST_PUBLIC_BUCKET="${TEST_STORAGE_PUBLIC_BUCKET:-loresuelvo-public-test}"
TEST_PRIVATE_BUCKET="${TEST_STORAGE_PRIVATE_BUCKET:-loresuelvo-private-test}"

mc alias set "$MINIO_ALIAS" "$MINIO_ENDPOINT" "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" >/dev/null 2>&1

# Keep bucket definitions and policies, but remove every object produced by tests.
# --force makes empty/missing-prefix cleanup idempotent.
mc rm --recursive --force "$MINIO_ALIAS/$TEST_PUBLIC_BUCKET" >/dev/null || true
mc rm --recursive --force "$MINIO_ALIAS/$TEST_PRIVATE_BUCKET" >/dev/null || true
