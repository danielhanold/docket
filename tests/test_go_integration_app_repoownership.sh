#!/usr/bin/env bash
# tests/test_go_integration_app_repoownership.sh — Go integration shard (change 0378):
# the shared metadata-root ownership verifier scenarios (topology and native
# receipt proofs, plus the unknown mapping — the receiptless-nonempty legacy
# equivalence path lands here too once Task 4 fills the stub), behind the
# `integration` build tag, prefix ^TestIntegrationRepoOwnership. Split out as its
# own shard because it is tagged (the default corpus cannot see it) and no
# existing app prefix is a name-prefix of TestIntegrationRepoOwnership.
# Declarations only — execution and inspection live in
# tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/app"
SHARD_PREFIX="TestIntegrationRepoOwnership"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
