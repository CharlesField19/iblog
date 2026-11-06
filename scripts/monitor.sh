#!/bin/sh
set -eu

URL="${BLOG_HEALTH_URL:-http://127.0.0.1:11190/api/v1/health}"
BACKUP_DIR="${BLOG_BACKUP_DIR:-$HOME/blog-backups}"
MAX_BACKUP_AGE_HOURS="${BLOG_MAX_BACKUP_AGE_HOURS:-192}"
CONTAINER="${CONTAINER_NAME:-iblog}"
VOLUME="${BLOG_DATA_VOLUME:-iblog_data}"

curl -fsS "$URL" | grep -q '"status":"ok"' || { echo "blog health check failed" >&2; exit 1; }
running="$(docker inspect -f '{{.State.Running}}' "$CONTAINER" 2>/dev/null || echo false)"
[ "$running" = true ] || { echo "container is not running: $CONTAINER" >&2; exit 1; }
backup="$(find "$BACKUP_DIR" -maxdepth 1 -type f -name 'iblog-data-*.tar.gz' -printf '%T@ %p\n' 2>/dev/null | sort -nr | head -n 1 | cut -d' ' -f2- || true)"
[ -n "$backup" ] && [ -s "$backup" ] || { echo "no usable backup found in $BACKUP_DIR" >&2; exit 1; }
age="$(awk -v now="$(date +%s)" -v mtime="$(stat -c %Y "$backup")" 'BEGIN { print int((now-mtime)/3600) }')"
[ "$age" -le "$MAX_BACKUP_AGE_HOURS" ] || { echo "latest backup is too old: ${age}h" >&2; exit 1; }
df -P "$BACKUP_DIR" | awk 'NR==2 { gsub(/%/,"",$5); if ($5 >= 90) exit 1 }' || { echo "backup disk usage is at least 90%" >&2; exit 1; }
docker exec "$CONTAINER" df -Pk /data | awk 'NR==2 { if ($4 < 1048576) exit 1 }' || { echo "less than 1 GiB free on database volume storage" >&2; exit 1; }
printf 'monitor ok: container=%s backup=%s age=%sh\n' "$CONTAINER" "$backup" "$age"
