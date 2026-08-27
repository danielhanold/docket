#!/usr/bin/env bash
# tests/test_go_integration_githubcli_ensure.sh — Go integration shard (change 0333):
# the githubcli PR ensure/comment lifecycle tests (idempotent probe->act->verify
# through the protocol-faithful fake gh), behind the `integration` build tag,
# prefix ^TestIntegrationEnsure. Declarations only — execution and inspection live
# in tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/githubcli"
SHARD_PREFIX="TestIntegrationEnsure"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
