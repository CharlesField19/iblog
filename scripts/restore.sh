#!/bin/sh
set -eu
backup="${1:?usage: $0 BACKUP.tar.gz}"
if [ "${CONFIRM_RESTORE:-}" != "yes" ]; then
  echo "This replaces all data in the iblog_data volume. Set CONFIRM_RESTORE=yes to continue." >&2
  exit 2
fi
volume="${BLOG_DATA_VOLUME:-iblog_data}"
container="${CONTAINER_NAME:-iblog}"
case "$backup" in /*) ;; *) backup="$(pwd)/$backup" ;; esac
test -s "$backup"
# Validate paths, entry types and the database before touching the live volume.
docker run --rm -v "$(dirname "$backup"):/backup:ro" alpine:3.20 sh -eu -c '
  archive="/backup/$1"
  tar -tzf "$archive" > /tmp/entries
  awk '\''$0 !~ /^data(\/|$)/ || $0 ~ /(^|\/)\.\.(\/|$)/ { bad=1 } END { exit bad }'\'' /tmp/entries
  tar -tvzf "$archive" > /tmp/types
  awk '\''substr($0,1,1) != "-" && substr($0,1,1) != "d" { bad=1 } END { exit bad }'\'' /tmp/types
  mkdir /restore
  tar -xzf "$archive" -C /restore
  test -s /restore/data/blog.db
' sh "$(basename "$backup")"
was_running="$(docker inspect -f '{{.State.Running}}' "$container" 2>/dev/null || echo false)"
cleanup() { if [ "$was_running" = true ]; then docker start "$container" >/dev/null 2>&1 || true; fi; }
trap cleanup EXIT
trap 'exit 1' HUP INT TERM
if [ "$was_running" = true ]; then docker stop "$container" >/dev/null; fi
docker run --rm -v "$volume:/data" -v "$(dirname "$backup"):/backup:ro" alpine:3.20 sh -eu -c '
  find /data -mindepth 1 -maxdepth 1 -exec rm -rf {} +
  tar -xzf "/backup/$1" -C /
' sh "$(basename "$backup")"
echo "Restore completed. Verify the application and login before deleting the old backup."
