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

# 0255: a `#` INSIDE the flow map. _hd_block strips comments before either reader runs, so
# `{ model: c#5 }` truncates to `c` with v == raw == c and slides past every leg above — the same
# silent-truncation class the quote leg exists to close, in a corner the value-comparison legs
# structurally cannot see. The strip ORDER is deliberately unchanged (reordering it would break
# legitimate trailing and full-line comments everywhere); the corner is caught on a PRE-STRIP view.
mut; sed -i.bak 's|^    adr:.*|    adr:                   { model: c#5, effort: low }|' "$T/hd.yml"
assert "reject: '#' inside the flow map" '! hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'
h_diag="$(hd_validate "$T/hd.yml" "$SRC" 2>&1 || true)"
assert "reject: '#' diagnostic names the flow map, not bareness" \
  'grep -q "inside the flow map" <<<"$h_diag"'
# The remedy for a truncating `#` is NOT "write it unquoted" — a diagnostic that blames the wrong
# cause is the defect the split diagnostics in this file exist to prevent.
assert "reject: '#' diagnostic does not blame quoting" \
  '! grep -q "unquoted and space-free" <<<"$h_diag"'

# Ignore probes — the carve-outs. Over-rejection here would hard-abort generation on config styles
# used throughout .docket.example.yml and this very sidecar.
mut; sed -i.bak 's|^    adr:.*|    adr:                   { model: claude-opus-5, effort: low }   # a trailing note|' "$T/hd.yml"
assert "accept: a trailing comment AFTER the closing brace is legal" \
  'hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'

# A commented-out map is field-absent post-strip. In the SIDECAR that is already an error — but the
# CORRECT one (missing field), not a truncation complaint. Pinning which diagnostic fires is the
# point: the `#` leg must stay silent here.
mut; sed -i.bak 's|^    adr:.*|    adr:                   # { model: c#5, effort: low }|' "$T/hd.yml"
c_diag="$(hd_validate "$T/hd.yml" "$SRC" 2>&1 || true)"
assert "accept: a commented-out map does not fire the '#' leg" \
  '! grep -q "inside the flow map" <<<"$c_diag"'
assert "reject: a commented-out map is a MISSING-field error instead" \
  'grep -q "missing a non-empty" <<<"$c_diag"'

# The raw view must select the SAME line the stripped view does, or the two disagree about WHICH
# entry is being judged — the failure mode that makes a duplicated gate diverge on exactly the
# inputs it was written to catch.
mut; sed -i.bak 's|^    adr:.*|    adr:                   { model: claude-opus-5, effort: low }   # note|' "$T/hd.yml"
assert "reader: keep_comments view returns the same entry, comments intact" \
  '[ "$(_hd_entry_line "$T/hd.yml" claude adr 1)" != "$(_hd_entry_line "$T/hd.yml" claude adr)" ] &&
   grep -q "^    adr:" <<<"$(_hd_entry_line "$T/hd.yml" claude adr 1)" &&
   grep -q "note" <<<"$(_hd_entry_line "$T/hd.yml" claude adr 1)"'

rm -rf "$T"

[ "$fail" = 0 ] && echo "PASS" || echo "FAIL"
exit "$fail"
