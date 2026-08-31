#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_app_sweep.sh — Go integration shard (change 0333):
# the real-process `maintenance sweep` tests driven through the production entry
# point app.MaintenanceSweep — traffic accounting over real git/gh executables
# and the bounded-return-under-short-network-budgets timeout behaviour — behind
# the `integration` build tag, prefix ^TestIntegrationSweep. Declarations only —
# execution and inspection live in tests/lib/go-integration-shard.sh; the
# completeness contract is tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/app"
SHARD_PREFIX="TestIntegrationSweep"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
