#!/bin/sh
set -eu
umask 077

volume="${BLOG_DATA_VOLUME:-iblog_data}"
container="${CONTAINER_NAME:-iblog}"
backup_dir="${BLOG_BACKUP_DIR:-$HOME/blog-backups}"
timestamp="$(date -u +%Y%m%dT%H%M%SZ)-$$"
backup="$backup_dir/iblog-data-$timestamp.tar.gz"
temporary="$backup.tmp"

mkdir -p "$backup_dir"
docker volume inspect "$volume" >/dev/null
was_running="$(docker inspect -f '{{.State.Running}}' "$container" 2>/dev/null || echo false)"
cleanup() {
  rm -f "$temporary"
  if [ "$was_running" = true ]; then docker start "$container" >/dev/null 2>&1 || true; fi
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM
if [ "$was_running" = true ]; then docker stop "$container" >/dev/null; fi
docker run --rm -v "$volume:/data:ro" alpine:3.20 \
  tar -czf - -C / data > "$temporary"
test -s "$temporary"
mv "$temporary" "$backup"
printf 'Backup created: %s\n' "$backup"
