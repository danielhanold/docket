#!/usr/bin/env bash
# tests/test_harness_defaults_flow_map.sh — value-level probes for the two change-0255 legs on
# agents/harness-defaults.yml: the ADR-0065 quote leg, and the `#`-inside-the-flow-map leg. Both
# validators of that file are probed here — hd_validate first, then sync-agents.sh's
# validate_harness_defaults, which is the one that actually runs on Bash 4+. A separate shard from
# test_harness_defaults_validator.sh purely on cost: every hd_validate assert here is one full
# sweep (~3.3s), and that file's 50s row has no margin left. The validate_harness_defaults probes
# below are not in tests/test_sync_agents.sh for the same reason — that file measures ~44.5s
# serially against a 50s row — and they are cheap here: one awk pass each.
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

# 0255: a `#` INSIDE the flow map. _hd_block strips comments before either reader runs, so the
# truncated remainder slides past every leg above — the same silent-truncation class the quote leg
# exists to close, in a corner the value-comparison legs structurally cannot see. The strip ORDER is
# deliberately unchanged (reordering it would break legitimate trailing and full-line comments
# everywhere); the corner is caught on a PRE-STRIP view.
#
# `{ model: claude-opus-5, effort: lo#w }` is THE DISCRIMINATING PROBE and must not be "simplified"
# to a shorter-looking input. Post-strip it leaves BOTH fields readable (`claude-opus-5` / `lo`), so
# every other leg passes and pre-change hd_validate returned 0 — ACCEPTING a truncated sidecar. Only
# with the `#` leg present does rc flip 0 -> 1, so this rc assert detects the state the change
# REMOVED. A `#` in the MODEL field instead (`{ model: c#5, … }`) does not discriminate: it strips
# to an unterminated map, `effort` reads as missing, and rc was already 1 before the change — such
# an input makes the rc assert vacuous no matter how the diagnostic reads.
mut; sed -i.bak 's|^    adr:.*|    adr:                   { model: claude-opus-5, effort: lo#w }|' "$T/hd.yml"
assert "reject: '#' inside the flow map" '! hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'
h_diag="$(hd_validate "$T/hd.yml" "$SRC" 2>&1 || true)"
assert "reject: '#' diagnostic names the flow map, not bareness" \
  'grep -q "inside the flow map" <<<"$h_diag"'
# The remedy for a truncating `#` is NOT "write it unquoted" — a diagnostic that blames the wrong
# cause is the defect the split diagnostics in this file exist to prevent.
assert "reject: '#' diagnostic does not blame quoting" \
  '! grep -q "unquoted and space-free" <<<"$h_diag"'

# The non-discriminating twin, kept deliberately as a SECOND case: a `#` in the model field strips
# to an unterminated map, so the flow-map complaint and the missing-`effort` complaint must BOTH
# fire. Its rc proves nothing (see above); what it pins is that the new leg does not swallow the
# pre-existing absence diagnostic when the truncation eats a whole field.
mut; sed -i.bak 's|^    adr:.*|    adr:                   { model: c#5, effort: low }|' "$T/hd.yml"
hm_diag="$(hd_validate "$T/hd.yml" "$SRC" 2>&1 || true)"
assert "reject: a model-field '#' still names the flow map" \
  'grep -q "inside the flow map" <<<"$hm_diag"'
assert "reject: a model-field '#' also keeps the missing-field complaint" \
  'grep -q "missing a non-empty" <<<"$hm_diag"'

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

# ---- the SAME quote leg in sync-agents.sh's validate_harness_defaults -------------------------
# validate_harness_defaults is the validator that ACTUALLY gates this sidecar: it short-circuits to
# hd_validate only when ${BASH_VERSINFO[0]} -lt 4, so on Bash 4+ — both call sites, the real run and
# --check — the awk single-pass program is what runs. Its value loop computes
# `consumed=raw; sub(/[[:space:]].*$/,"",consumed)` and compares, which is exactly the
# whitespace-only test ADR-0065 says is structurally blind to `"claude-opus-5"`: a quoted but
# space-free pin has consumed == raw, passes, and the quotes ride into the emitted wrapper's pin.
# The two validators duplicate this rule BY VALUE on purpose (harness-defaults.sh's header forbids
# coupling the shipped-data reader to the user-config readers), so nothing but a test holds them in
# parity — this block is that test.
vhd(){  # $1=sidecar file -> prints the validator's diagnostics; rc is the validator's rc
  ( . "$REPO/sync-agents.sh" >/dev/null 2>&1
    set +e   # sync-agents.sh enables errexit for direct invocation; these probes expect nonzero
    validate_harness_defaults "$1" "$SRC" 2>&1 >/dev/null )
}
# The premise of every assert below. On a Bash 3 host the function delegates to hd_validate and the
# probes would silently re-test the legs already covered above instead of the awk program.
assert "parity premise: this host takes the Bash 4+ awk path" '[ "${BASH_VERSINFO[0]}" -ge 4 ]'

mut; sed -i.bak 's|^    adr:.*|    adr:                   { model: "claude-opus-5", effort: low }|' "$T/hd.yml"
vhd_dq="$(vhd "$T/hd.yml")"; vhd_dq_rc=$?
assert "awk validator rejects: double-quoted SPACE-FREE scalar" '[ "$vhd_dq_rc" != "0" ]'
assert "awk validator rejects: double-quoted diagnostic names the harness/agent" \
  '/usr/bin/grep -qF "claude/adr" <<<"$vhd_dq"'
assert "awk validator rejects: double-quoted diagnostic names the remedy" \
  '/usr/bin/grep -qF "unquoted and space-free" <<<"$vhd_dq"'

mut; sed -i.bak "s|^    adr:.*|    adr:                   { model: 'claude-opus-5', effort: low }|" "$T/hd.yml"
vhd_sq="$(vhd "$T/hd.yml")"; vhd_sq_rc=$?
assert "awk validator rejects: single-quoted SPACE-FREE scalar" '[ "$vhd_sq_rc" != "0" ]'
assert "awk validator rejects: single-quoted diagnostic names the remedy" \
  '/usr/bin/grep -qF "unquoted and space-free" <<<"$vhd_sq"'

# The `!=` leg is kept byte-for-byte; the quote leg only extends it. A space-bearing value must
# still be caught, and by the SAME sentence — the diagnostic is a specification shared with
# hd_validate and validate_user_agent_values, not this validator's private wording.
mut; sed -i.bak 's|^    adr:.*|    adr:                   { model: two words, effort: low }|' "$T/hd.yml"
vhd_sp="$(vhd "$T/hd.yml")"; vhd_sp_rc=$?
assert "awk validator rejects: the space-bearing control still fires" '[ "$vhd_sp_rc" != "0" ]'
assert "awk validator rejects: space-bearing control keeps the shared sentence" \
  '/usr/bin/grep -qF "is not a bare scalar" <<<"$vhd_sp"'

# 0255: the SAME `#`-inside-the-flow-map leg, on the awk path. The awk program judges every entry on
# `nc`, the comment-STRIPPED line, so `{ model: claude-opus-5, effort: lo#w }` arrives as an
# unterminated map and the `fields !~ /\{.*\}/` branch reports BOTH fields absent. That rejects the
# input only incidentally, and with the wrong cause — it blames bareness, names neither the `#` nor
# the flow map, and offers no remedy: exactly the failure the hd_validate probes above forbid. The
# two validators duplicate this rule by value, so only this block holds them in parity.
#
# Note what the rc assert below can and cannot do. On the awk path NO input discriminates: the
# program judges `nc`, and stripping a `#` from anywhere inside `{…}` always eats the closing brace,
# so the absence branch already returned nonzero for every such input before this change. The rc
# assert is therefore a floor, not evidence; the two DIAGNOSTIC asserts are the load-bearing ones —
# they are what redden if the flow-map leg is removed here.
mut; sed -i.bak 's|^    adr:.*|    adr:                   { model: claude-opus-5, effort: lo#w }|' "$T/hd.yml"
vhd_h="$(vhd "$T/hd.yml")"; vhd_h_rc=$?
assert "awk validator rejects: '#' inside the flow map" '[ "$vhd_h_rc" != "0" ]'
assert "awk validator rejects: '#' diagnostic names the flow map" \
  '/usr/bin/grep -qF "inside the flow map" <<<"$vhd_h"'
# The wrong-cause guard. hd_validate emits ONLY the flow-map sentence for this input (its readers
# still see both values on the truncated line); the awk path must not answer the same input with a
# contradictory ABSENCE complaint depending on which bash the operator happens to run.
assert "awk validator rejects: '#' diagnostic does not blame absence" \
  '! /usr/bin/grep -qF "missing a non-empty" <<<"$vhd_h"'

# Ignore probes — the carve-outs, on the awk path. Over-rejection here would hard-abort generation
# on comment styles used throughout .docket.example.yml and this very sidecar.
mut; sed -i.bak 's|^    adr:.*|    adr:                   { model: claude-opus-5, effort: low }   # a trailing note|' "$T/hd.yml"
vhd_tc="$(vhd "$T/hd.yml")"; vhd_tc_rc=$?
assert "awk validator accepts: a trailing comment AFTER the closing brace is legal" \
  '[ "$vhd_tc_rc" = "0" ]'

# A commented-out map is field-absent post-strip. In the SIDECAR that is already an error — but the
# CORRECT one (missing field), not a truncation complaint. Pinning WHICH diagnostic fires is the
# point, and it is the same pairing the hd_validate probes above assert.
mut; sed -i.bak 's|^    adr:.*|    adr:                   # { model: c#5, effort: low }|' "$T/hd.yml"
vhd_co="$(vhd "$T/hd.yml")"
assert "awk validator accepts: a commented-out map does not fire the '#' leg" \
  '! /usr/bin/grep -qF "inside the flow map" <<<"$vhd_co"'
assert "awk validator rejects: a commented-out map is a MISSING-field error instead" \
  '/usr/bin/grep -qF "missing a non-empty" <<<"$vhd_co"'

# Over-rejection floor: the pristine shipped sidecar must still validate clean, or the quote leg
# would hard-abort every real run instead of catching the corner it was written for.
vhd_ok="$(vhd "$HD")"; vhd_ok_rc=$?
assert "awk validator accepts: the shipped sidecar is unaffected (rc=0)" '[ "$vhd_ok_rc" = "0" ]'
assert "awk validator accepts: and emits no bare-scalar complaint" \
  '! /usr/bin/grep -qF "is not a bare scalar" <<<"$vhd_ok"'

rm -rf "$T"

[ "$fail" = 0 ] && echo "PASS" || echo "FAIL"
exit "$fail"
