#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_app_repovisibility_shared.sh — Go integration shard: the
# `repository set-visibility` switch to shared over real git — restoring the
# shared layout and keys, origin-branch handling, and resume after every phase —
# behind the `integration` build tag, prefix ^TestIntegrationRepoVisibilityShared. The
# visibility tests split across three sibling shards by name prefix (none a
# prefix of another) so each stays under its runtime budget.
# Declarations only — execution and inspection live in
# tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/app"
SHARD_PREFIX="TestIntegrationRepoVisibilityShared"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
