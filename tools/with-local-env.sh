#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
mkdir -p "$root/.cache/go-build" "$root/.cache/gopath" "$root/.cache/tmp" "$root/.cache/npm" "$root/.cache/npm-global" "$root/.cache/pip" "$root/.cache/xdg" "$root/.config" "$root/.home"
export GOROOT="$root/go" GOPATH="$root/.cache/gopath" GOMODCACHE="$root/.cache/gopath/pkg/mod" GOCACHE="$root/.cache/go-build"
export GOTMPDIR="$root/.cache/tmp" TMPDIR="$root/.cache/tmp" HOME="$root/.home" GOENV="$root/.config/go/env"
export GOTOOLCHAIN=local GOPROXY="file://$root/.cache/go-mod/cache/download" GOSUMDB=off
export XDG_CACHE_HOME="$root/.cache/xdg" npm_config_cache="$root/.cache/npm" PIP_CACHE_DIR="$root/.cache/pip"
export npm_config_prefix="$root/.cache/npm-global" npm_config_userconfig="$root/.config/npmrc" NODE_REPL_HISTORY="$root/.cache/node-repl-history"
export PATH="$root/tools/node/bin:$GOROOT/bin:$PATH"
cd "$root/blog-reconstruction"
exec "$@"
