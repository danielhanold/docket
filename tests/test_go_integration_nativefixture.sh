#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_nativefixture.sh — the candidate-source-bound fixture
# generator acceptance, behind the `integration` build tag and prefix
# ^TestIntegrationNativeFixture. Declarations only; the shared integration runner
# owns execution and the integration contract proves complete, unique membership.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./cmd/nativefixture"
SHARD_PREFIX="TestIntegrationNativeFixture"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
