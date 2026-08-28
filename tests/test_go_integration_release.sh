#!/usr/bin/env bash
# tests/test_go_integration_release.sh — Go integration shard (change 0362): the
# release packaging/archive real cross-build, subprocess, tar/gzip and filesystem
# corpus, behind the `integration` build tag, prefix ^TestIntegrationRelease,
# sequential and non-race (no concurrent protocol; see the 0362 design's
# "No release race shard" non-goal). Declarations only — execution and inspection
# live in tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/release"
SHARD_PREFIX="TestIntegrationRelease"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
