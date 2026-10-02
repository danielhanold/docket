#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_gatedrive_race.sh — Go integration shard (change 0466, extending
# change 0333's partition): the gate driver's real-concurrency integration tests (takeover of a
# live supervised run, same-worktree generations against a live incumbent, concurrent Starts
# over a legacy-seeded store) — moved out of the default
# internal/gatedrive corpus behind the `integration` build tag, prefix
# ^TestRaceIntegrationGatedrive, run in RACE mode. Declarations only — execution and inspection
# live in tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/gatedrive"
SHARD_PREFIX="TestRaceIntegrationGatedrive"
SHARD_MODE="race"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
