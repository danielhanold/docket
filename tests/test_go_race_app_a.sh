#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_race_app_a.sh — first derived half of internal/app's default
# corpus under the race detector. The parent test_go_race.sh validates the full
# sibling partition before excluding internal/app from its module package set.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/app"
SHARD_INDEX=0
SHARD_COUNT=2
. "$REPO/tests/lib/go-race-app-shard.sh"
race_app_inspect_maybe
run_race_app_shard
exit "$fail"
