#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_app_reporepair.sh — Go integration shard (change 0496):
# the `repository repair` service on an already-migrated repository — preview
# writes nothing, one lease-guarded descendant commit carrying exactly the listed
# files, frontmatter + derived-view composition, moved-tip contention, legacy and
# fresh refusals, malformed-marker manual review, and the empty-claimed_at no-op —
# behind the `integration` build tag, prefix ^TestIntegrationRepoRepair.
# Declarations only — execution and inspection live in
# tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/app"
SHARD_PREFIX="TestIntegrationRepoRepair"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
