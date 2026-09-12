#!/bin/sh
set -eu

mkdir -p /etc/nginx/assetlinks

envsubst \
  '${ANDROID_APP_LINK_PACKAGE_NAME} ${ANDROID_APP_LINK_SHA256_CERT_FINGERPRINT}' \
  < /etc/nginx/assetlinks-template/assetlinks-dev.json.template \
  > /etc/nginx/assetlinks/assetlinks.json

exec /docker-entrypoint.sh nginx -g 'daemon off;'