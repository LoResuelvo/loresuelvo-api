#!/bin/sh
set -e

# Start nginx in the background
nginx -g 'daemon off;' &
NGINX_PID=$!

# Watch for changes to the config and reload nginx
while inotifywait -e close_write /etc/nginx/conf.d/default.conf; do
  echo "Reloading nginx config..."
  nginx -s reload
  if ! kill -0 $NGINX_PID 2>/dev/null; then
    echo "nginx process exited, exiting watcher."
    exit 1
  fi
done
