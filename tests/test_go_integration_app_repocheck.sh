#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_app_repocheck.sh — Go integration shard (change 0352):
# the read-only repository check service scenarios (fresh, needs-review→healthy,
# every healthy postcondition, read-only invariance, unknown authority), behind
# the `integration` build tag, prefix ^TestIntegrationRepoCheck. Split out of the
# reposetup shard when the combined check+init readings exceeded that shard's row
# (per the budget rule: split, never raise). Declarations only — execution and
# inspection live in tests/lib/go-integration-shard.sh; the completeness contract
# is tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/app"
SHARD_PREFIX="TestIntegrationRepoCheck"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
