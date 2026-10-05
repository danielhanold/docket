#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_app_repoinplaceff.sh — Go integration shard: the in-place
# fast-forward and attach-freshness scenarios for `repository prepare` (a clean, behind
# .docket advances inside the existing worktree with nothing removed — lock, directory
# identity, ignored files, and the per-worktree hooks-off setting survive — by a
# compare-and-swap on the router's observed tip that refuses a stale tip, with no
# repository hook run even when hooks-off is missing; a re-run is a no-op; an
# interrupted fast-forward reads dirty and prepare refuses), behind the `integration`
# build tag, prefix ^TestIntegrationRepoInPlaceFF. A new shard because no existing app
# prefix is a name-prefix of TestIntegrationRepoInPlaceFF. Declarations only —
# execution and inspection live in tests/lib/go-integration-shard.sh; the
# completeness contract is tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/app"
SHARD_PREFIX="TestIntegrationRepoInPlaceFF"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
