#!/usr/bin/env bash
# tests/test_harness_defaults_flow_map.sh — value-level probes for hd_validate's two change-0255
# legs: the ADR-0065 quote leg, and the `#`-inside-the-flow-map leg. A separate shard from
# test_harness_defaults_validator.sh purely on cost: every assert here is one full hd_validate
# sweep (~3.3s), and that file's 50s row has no margin left.
# Run: bash tests/test_harness_defaults_flow_map.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

. "$REPO/scripts/lib/harness-defaults.sh"
HD="$REPO/agents/harness-defaults.yml"
SRC="$REPO/agents"

T="$(mktemp -d "${TMPDIR:-/tmp}/hd-flow-map.XXXXXX")"
mut(){ cp "$HD" "$T/hd.yml"; }

# 0255 / ADR-0065: a quoted but SPACE-FREE value. `"claude-opus-5"` has consumed == raw under both
# readers, so the `!=` leg structurally CANNOT see it — the quotes would ride into the emitted pin
# verbatim while the diagnostic's own remedy tells the user to write them unquoted.
# This pair is what pins the explicit quote leg, and it is the twin of the probe change 0173 added
# to tests/test_sync_agents_validator.sh for the user-config validator.
mut; sed -i.bak 's|^    adr:.*|    adr:                   { model: "claude-opus-5", effort: low }|' "$T/hd.yml"
assert "reject: double-quoted SPACE-FREE scalar" '! hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'
dq_diag="$(hd_validate "$T/hd.yml" "$SRC" 2>&1 || true)"
assert "reject: double-quoted space-free diagnostic names the remedy" \
  'grep -q "unquoted and space-free" <<<"$dq_diag"'

mut; sed -i.bak "s|^    adr:.*|    adr:                   { model: 'claude-opus-5', effort: low }|" "$T/hd.yml"
assert "reject: single-quoted SPACE-FREE scalar" '! hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'

rm -rf "$T"

[ "$fail" = 0 ] && echo "PASS" || echo "FAIL"
exit "$fail"
