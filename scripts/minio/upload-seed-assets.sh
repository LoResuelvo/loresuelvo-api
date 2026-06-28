#!/bin/sh
set -eu

MINIO_ALIAS="local"
MINIO_API_PORT="${MINIO_API_PORT:-9000}"
MINIO_ENDPOINT="http://minio.localhost:${MINIO_API_PORT}"
PUBLIC_BUCKET="${STORAGE_PUBLIC_BUCKET:-loresuelvo-public-local}"
ASSETS_DIR="${SEED_ASSETS_DIR:-/seed-assets}"
MANIFEST_FILE="${SEED_ASSET_MANIFEST:-/seed-manifest/providers-100-assets.tsv}"

mc alias set "$MINIO_ALIAS" "$MINIO_ENDPOINT" "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD"
mc mb --ignore-existing "$MINIO_ALIAS/$PUBLIC_BUCKET"
mc anonymous set download "$MINIO_ALIAS/$PUBLIC_BUCKET"

if [ ! -f "$MANIFEST_FILE" ]; then
	echo "seed asset manifest not found: $MANIFEST_FILE" >&2
	exit 1
fi

while IFS= read -r line || [ -n "$line" ]; do
	case "$line" in
		""|\#*) continue ;;
	esac

	source_asset=$(printf '%s' "$line" | cut -f1)
	target_key=$(printf '%s' "$line" | cut -f2)
	source_path="$ASSETS_DIR/$source_asset"

	if [ ! -f "$source_path" ]; then
		echo "seed asset not found: $source_path" >&2
		exit 1
	fi

	mc cp --attr "Content-Type=image/webp" "$source_path" "$MINIO_ALIAS/$PUBLIC_BUCKET/$target_key"
done < "$MANIFEST_FILE"

echo "Seed assets uploaded to MinIO bucket $PUBLIC_BUCKET"
