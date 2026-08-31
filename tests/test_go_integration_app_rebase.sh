#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_app_rebase.sh — Go integration shard (change 0333):
# the finalize rebase real-repository tests (gate outcomes, abort/continue, recovery), behind the `integration` build tag, prefix
# ^TestIntegrationFinalizeRebase. Declarations only — execution and inspection live in
# tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/app"
SHARD_PREFIX="TestIntegrationFinalizeRebase"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
