#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_app_reposynccheck.sh — Go integration shard: the local
# .docket copy relationship scenarios for `repository check` (a clean behind-only copy
# is healthy with no finding and configure-tests accepts it; ahead, diverged, and
# dirty-behind each report exactly their own finding), behind the `integration` build
# tag, prefix ^TestIntegrationRepoSyncCheck. A new shard because no existing app
# prefix is a name-prefix of TestIntegrationRepoSyncCheck. Declarations only —
# execution and inspection live in tests/lib/go-integration-shard.sh; the
# completeness contract is tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/app"
SHARD_PREFIX="TestIntegrationRepoSyncCheck"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
