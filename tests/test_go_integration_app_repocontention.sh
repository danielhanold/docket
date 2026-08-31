#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_app_repocontention.sh — Go integration shard (change 0352):
# the sequential migration-contention tests (metadata-lease loss under a second-writer
# docket advance, integration-lease loss rebuilt atop the fresh re-read, and a concurrent
# `repository check` observing only the resumable partial), behind the `integration` build
# tag, prefix ^TestIntegrationRepoContention, run in NORMAL mode. Declarations only —
# execution and inspection live in tests/lib/go-integration-shard.sh; the completeness
# contract is tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/app"
SHARD_PREFIX="TestIntegrationRepoContention"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
