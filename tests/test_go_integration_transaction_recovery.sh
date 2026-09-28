#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_transaction_recovery.sh — Go integration shard (change 0466, extending
# change 0333's partition): the transaction engine's replay, materialization, interruption,
# cleanup, and abandoned-candidate recovery real-git tests — moved out of the default
# internal/repository/transaction corpus, which must never start real git
# (testsupport.InstallNoGitGuard, installed from the package's TestMain) — behind the
# `integration` build tag, prefix ^TestIntegrationTxnRecovery. Declarations only — execution and
# inspection live in tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/repository/transaction"
SHARD_PREFIX="TestIntegrationTxnRecovery"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
