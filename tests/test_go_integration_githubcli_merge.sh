#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_githubcli_merge.sh — Go integration shard (change 0333):
# the githubcli merge, merge-method/branch-rule probe, and retarget behavior
# tests (fake-gh `pr merge`/`pr view`/`api` subprocess runs), behind the
# `integration` build tag, prefix ^TestIntegrationMerge. Declarations only —
# execution and inspection live in tests/lib/go-integration-shard.sh; the
# completeness contract is tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/githubcli"
SHARD_PREFIX="TestIntegrationMerge"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
