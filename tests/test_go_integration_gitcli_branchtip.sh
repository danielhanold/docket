#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_gitcli_branchtip.sh — Go integration shard (change
# 0333): the gitcli branch-tip movement tests (the in-place compare-and-swap
# fast-forward, the worktree fast-forward, the checked delete, and the
# replace-ref-blind ancestry probe that judges them), behind the `integration` build tag, prefix
# ^TestIntegrationBranchTip. Split out of the gitcli repo shard so neither grows
# past its runtime budget. Declarations only — execution and inspection live in
# tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/gitcli"
SHARD_PREFIX="TestIntegrationBranchTip"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
