#!/usr/bin/env bash
# scripts/gate-verdict.sh — a thin delegator to `docket run gate-verdict` (change 0334).
#
# The run-gate facade's verdict step lives entirely in the Go binary
# (internal/app/rungate_verdict.go): attributed mode (`gate-verdict <key>`) with its
# durable-record attribution and atomic one-retry accounting, and observe-only mode
# (`gate-verdict --unattributed [<id>...]`). This wrapper exists only so the
# consumer-facing spelling `docket.sh gate-verdict …` resolves in a consuming repo,
# where the Go binary is on PATH but the helper facade is not. It owns NO behavior:
# it forwards its whole argument vector to the binary and `exec`s, so stdout and the
# exit code pass through untouched — the `gate-*` report line is the contract, and a
# report line exits 0.
#
# Mock seam: DOCKET_BIN (the Go binary), the same seam scripts/verify-run.sh uses.
# Contract: scripts/gate-verdict.md.
set -uo pipefail
DOCKET_BIN="${DOCKET_BIN:-docket}"
exec "$DOCKET_BIN" run gate-verdict "$@"
