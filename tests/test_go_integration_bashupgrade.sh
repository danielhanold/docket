#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_bashupgrade.sh — Go integration shard: the Bash docket
# v0.9.2/v0.9.3 upgrade proof. It restores the saved Bash installs under
# testdata/bash-upgrade/ and checks the binary built from this checkout against
# them (metadata ownership, then the upgrade guide's own marked steps), behind the
# `integration` build tag, prefix ^TestIntegrationBashUpgrade. Temporary: retired
# with internal/bashupgrade/ and testdata/bash-upgrade/ when stable v1.0.0 ships.
# Declarations only — execution and inspection live in
# tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/bashupgrade"
SHARD_PREFIX="TestIntegrationBashUpgrade"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
