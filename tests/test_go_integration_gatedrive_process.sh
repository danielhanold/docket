#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_gatedrive_process.sh — Go integration shard (change 0466, extending
# change 0333's partition): the gate driver's real-process and real-git tests (driving the REAL
# native supervisor internal/process.Service across slices, fresh-process resume, deadline and
# death handling, real-git sequences, and the worktree fingerprint/handoff proofs over real
# repositories) — moved out of the default internal/gatedrive corpus behind the `integration`
# build tag, prefix ^TestIntegrationGatedrive. The default internal/gatedrive corpus must never
# start real git (testsupport.InstallNoGitGuard, installed from the package's TestMain, change
# 0470). Declarations only — execution and inspection live in tests/lib/go-integration-shard.sh;
# the completeness contract is tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/gatedrive"
SHARD_PREFIX="TestIntegrationGatedrive"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
