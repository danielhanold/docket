#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_app_buildstart.sh — Go integration shard (change 0428;
# re-targeted by change 0490): build-owned starts admitted on the worktree lock,
# driven end-to-end through the application gate-drive seam with the real driver,
# process supervisor, and git worktree (a start into a worktree whose lock another
# gate holds is refused worktree-busy with no drive created, no run launched, and
# no suite attempt charged, and the next start after the holder lets go is
# admitted; a busy refusal names the running holder's drive, change, and
# run.cancel), behind the `integration` build tag, prefix ^TestIntegrationBuildStart.
# Declarations only — execution and inspection live in
# tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/app"
SHARD_PREFIX="TestIntegrationBuildStart"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
