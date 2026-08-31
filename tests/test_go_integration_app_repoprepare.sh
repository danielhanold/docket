#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_app_repoprepare.sh — Go integration shard (change 0377):
# the shared Step-0 `repository prepare` service scenarios (a real multi-commit docket
# chain classifies owned, worktree attachment to a healthy remote, the clean-behind
# fast-forward to the pinned revision, dirty/ahead/diverged refusals with byte-
# untouched worktree evidence, and re-run idempotence), behind the `integration`
# build tag, prefix ^TestIntegrationRepoPrepare. A new shard because no existing app
# prefix is a name-prefix of TestIntegrationRepoPrepare and the real-git attach/
# fast-forward effects need their own end-to-end coverage. Declarations only —
# execution and inspection live in tests/lib/go-integration-shard.sh; the
# completeness contract is tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/app"
SHARD_PREFIX="TestIntegrationRepoPrepare"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
