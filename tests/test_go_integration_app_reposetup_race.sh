#!/usr/bin/env bash
# tests/test_go_integration_app_reposetup_race.sh — Go integration shard (change 0352):
# the real-concurrency repository-setup tests (two goroutines racing RunRepositoryInit
# for the create-only metadata root over one upstream, and a migration racing repeated
# read-only checks), behind the `integration` build tag, prefix ^TestRaceIntegrationRepoSetup,
# run in RACE mode. Declarations only — execution and inspection live in
# tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/app"
SHARD_PREFIX="TestRaceIntegrationRepoSetup"
SHARD_MODE="race"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
