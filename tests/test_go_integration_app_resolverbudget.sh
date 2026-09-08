#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_app_resolverbudget.sh — Go integration shard (change 0349):
# the finalize resolver-budget reserve->dispatch->verified-continue tests over a real
# feature workspace (successive conflicts spend the budget, exhaustion routes to
# blocked, a reservation survives a process restart), behind the `integration` build
# tag, prefix ^TestIntegrationResolverBudget. Declarations only — execution and
# inspection live in tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/app"
SHARD_PREFIX="TestIntegrationResolverBudget"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
