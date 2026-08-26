#!/usr/bin/env bash
# scripts/gate-before.sh — a thin delegator to `docket run gate-before` (change 0334).
#
# The run-gate facade's arming step (`gate-armed <key>` / `gate-unarmed <reason>`)
# lives entirely in the Go binary (internal/app/rungate_before.go). This wrapper
# exists only so the payload's consumer-facing spelling `docket.sh gate-before
# implement-next` resolves in a consuming repo, where the Go binary is on PATH but
# the helper facade is not. It owns NO behavior: it forwards its whole argument
# vector to the binary and `exec`s, so stdout and the exit code pass through
# untouched — the report line is the contract, and a report line exits 0.
#
# Mock seam: DOCKET_BIN (the Go binary), the same seam scripts/verify-run.sh uses.
# Contract: scripts/gate-before.md.
set -uo pipefail
DOCKET_BIN="${DOCKET_BIN:-docket}"
exec "$DOCKET_BIN" run gate-before "$@"
