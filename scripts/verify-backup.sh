#!/bin/sh
set -eu
backup="${1:?usage: $0 BACKUP.tar.gz [IMAGE]}"
image="${2:-ghcr.io/charlesfield19/iblog:latest}"
case "$backup" in /*) ;; *) backup="$(pwd)/$backup" ;; esac
test -s "$backup"
docker image inspect "$image" >/dev/null 2>&1 || docker pull "$image" >/dev/null
volume="${BACKUP_CHECK_VOLUME:-iblog-backup-check-$$}"
container="${BACKUP_CHECK_CONTAINER:-iblog-backup-check-$$}"
cleanup() {
  docker rm -f "$container" >/dev/null 2>&1 || true
  docker volume rm "$volume" >/dev/null 2>&1 || true
}
trap cleanup EXIT
docker volume create "$volume" >/dev/null
docker run --rm -v "$volume:/data" -v "$(dirname "$backup"):/backup:ro" alpine:3.20 sh -eu -c '
  archive="/backup/$1"
  tar -tzf "$archive" | awk '\''$0 !~ /^data(\/|$)/ || $0 ~ /(^|\/)\.\.(\/|$)/ { bad=1 } END { exit bad }'\''
  tar -tvzf "$archive" | awk '\''substr($0,1,1) != "-" && substr($0,1,1) != "d" { bad=1 } END { exit bad }'\''
  tar -xzf "$archive" -C /
  test -s /data/blog.db
' sh "$(basename "$backup")"
docker run -d --name "$container" -e BLOG_ADDR=:11190 -v "$volume:/data" "$image" >/dev/null
for i in $(seq 1 20); do
  if docker exec "$container" wget -qO- http://127.0.0.1:11190/api/v1/health | grep -q '"status":"ok"'; then
    printf 'Backup is readable and the application starts: %s\n' "$backup"
    exit 0
  fi
  sleep 1
done
docker logs "$container" >&2 || true
echo "Backup verification failed: application did not become healthy" >&2
exit 1
