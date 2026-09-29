#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_app_runfence.sh — Go integration shard (change 0465, extending change
# 0333's partition): the run-tracker fencing and ownership tests — real-git tests moved out of the
# default internal/app corpus, which must never start real git (the no-real-git guard
# in internal/app/nogit_guard_test.go) — behind the `integration` build tag, prefix
# ^TestIntegrationRunFence. Declarations only — execution and inspection live in
# tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/app"
SHARD_PREFIX="TestIntegrationRunFence"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
