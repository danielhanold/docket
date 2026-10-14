#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_app_metadatafollow.sh — Go integration shard: a docket
# write moves the clone's metadata checkout (shared .docket, private
# checkouts/<clone-id>) forward in place to the commit it just pushed when that is
# safe — clean, attached on the metadata branch, strictly behind — and leaves every
# other checkout untouched, behind the `integration` build tag, prefix
# ^TestIntegrationMetadataFollow. A new shard because no existing app prefix is a
# name-prefix of TestIntegrationMetadataFollow. Declarations only — execution and
# inspection live in tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/app"
SHARD_PREFIX="TestIntegrationMetadataFollow"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
