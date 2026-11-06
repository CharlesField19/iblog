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
was_running="$(docker inspect -f '{{.State.Running}}' "$container" 2>/dev/null || echo false)"
if [ "$was_running" = true ]; then docker stop "$container" >/dev/null; fi
cleanup() { if [ "$was_running" = true ]; then docker start "$container" >/dev/null 2>&1 || true; fi; }
trap cleanup EXIT
docker run --rm -v "$volume:/data" -v "$(dirname "$backup"):/backup:ro" alpine:3.20 sh -c "rm -rf /data/* /data/.[!.]* /data/..?*; tar -xzf /backup/$(basename "$backup") -C /"
echo "Restore completed. Verify the application and login before deleting the old backup."
