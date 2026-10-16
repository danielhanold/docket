#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_app_reposetup_private.sh — Go integration shard
# (change 0527): the private-visibility repository setup tests (private init
# layout, store adoption and refusals, metadata-remote flags, global private
# default, private check and migrate refusals, reclone and moved-clone recovery,
# private harness choice), behind the `integration` build tag, prefix
# ^TestIntegrationPrivateRepoSetup; split out of
# tests/test_go_integration_app_reposetup.sh. Declarations only — execution and
# inspection live in tests/lib/go-integration-shard.sh; the completeness contract
# is tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/app"
SHARD_PREFIX="TestIntegrationPrivateRepoSetup"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
