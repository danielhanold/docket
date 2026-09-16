#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_toolchain_app_b.sh — second derived half of internal/app's
# default host-test corpus. The parent test_go_toolchain_test.sh validates the
# full sibling partition before excluding internal/app from its package set.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/app"
SHARD_INDEX=1
SHARD_COUNT=2
SHARD_FAMILY=toolchain
SHARD_TEST_FLAG=''
SHARD_LABEL="host-test"
. "$REPO/tests/lib/go-app-shard.sh"
app_shard_inspect_maybe
run_app_shard
exit "$fail"
