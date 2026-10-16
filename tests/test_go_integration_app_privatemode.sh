#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_app_privatemode.sh — Go integration shard (change 0536):
# the private-visibility tests — the init-to-implemented acceptance run and the
# leak-check tests (seeded leaks, match_word off, an unverified scan, the branch
# name, the PR write, and the shared-mode negative) — behind the `integration`
# build tag, prefix ^TestIntegrationPrivateMode. Split out of
# tests/test_go_integration_app_workflowlifecycle.sh. Declarations only — execution and
# inspection live in tests/lib/go-integration-shard.sh; the completeness contract
# is tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/app"
SHARD_PREFIX="TestIntegrationPrivateMode"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
