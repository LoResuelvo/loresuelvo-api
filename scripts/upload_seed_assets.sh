#!/bin/sh
set -eu

ASSETS_DIR="${SEED_ASSETS_DIR:-seeds/assets}"
MANIFEST_FILE="${SEED_ASSET_MANIFEST:-seeds/providers-100-assets.tsv}"
PUBLIC_BUCKET="${STORAGE_PUBLIC_BUCKET:?STORAGE_PUBLIC_BUCKET is required}"
ENDPOINT_URL="${STORAGE_ENDPOINT:-}"

if ! command -v aws >/dev/null 2>&1; then
	echo "aws CLI is required to upload seed assets" >&2
	exit 1
fi

if [ ! -f "$MANIFEST_FILE" ]; then
	echo "seed asset manifest not found: $MANIFEST_FILE" >&2
	exit 1
fi

export AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID:-${STORAGE_ACCESS_KEY_ID:-}}"
export AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY:-${STORAGE_SECRET_ACCESS_KEY:-}}"
export AWS_DEFAULT_REGION="${AWS_DEFAULT_REGION:-${STORAGE_REGION:-us-east-1}}"

if [ -z "$AWS_ACCESS_KEY_ID" ] || [ -z "$AWS_SECRET_ACCESS_KEY" ]; then
	echo "AWS_ACCESS_KEY_ID/STORAGE_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY/STORAGE_SECRET_ACCESS_KEY are required" >&2
	exit 1
fi

aws_args=""
if [ -n "$ENDPOINT_URL" ]; then
	aws_args="--endpoint-url $ENDPOINT_URL"
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

	# shellcheck disable=SC2086
	aws $aws_args s3 cp "$source_path" "s3://$PUBLIC_BUCKET/$target_key" \
		--content-type image/webp \
		--cache-control "public, max-age=31536000, immutable" \
		--only-show-errors
done < "$MANIFEST_FILE"

echo "Seed assets uploaded to bucket $PUBLIC_BUCKET"
