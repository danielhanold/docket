#!/usr/bin/env bash
# tests/test_harness_defaults_validator.sh — the malformed-shape validator rejections (shard of
# test_harness_defaults.sh, change 0227). Run: bash tests/test_harness_defaults_validator.sh
# Boundary chosen by measured cost: every assertion here costs one full `hd_validate` sweep
# (~3.3s), and this shard holds 13 of the original file's 24 calls.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

. "$REPO/scripts/lib/harness-defaults.sh"
HD="$REPO/agents/harness-defaults.yml"
SRC="$REPO/agents"

T="$(mktemp -d)"
mut(){ cp "$HD" "$T/hd.yml"; }

# ---- validator rejects each malformed shape ---------------------------------
mut; printf '    phantom:               { model: x, effort: low }\n' >> "$T/hd.yml"
assert "reject: phantom agent key" '! hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'

mut; printf '\n  default:\n    adr:                   { model: x, effort: low }\n' >> "$T/hd.yml"
assert "reject: harness-neutral default block" '! hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'

mut; printf '\n  bogus:\n    adr:                   { model: x, effort: low }\n' >> "$T/hd.yml"
assert "reject: unknown harness key" '! hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'

mut; sed -i.bak 's|^    adr:.*|    adr:                   { model: claude-opus-5 }|' "$T/hd.yml"
assert "reject: entry missing effort" '! hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'

mut; sed -i.bak 's|^    adr:.*|    adr:                   { model: claude-opus-5, effort: low, runner: codex }|' "$T/hd.yml"
assert "reject: runner is forbidden" '! hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'

mut; sed -i.bak 's|^    build-standard:.*cursor-grok-4.5-medium.*|    build-standard:          { model: , effort: auto }|' "$T/hd.yml"
assert "reject: empty field value" '! hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'

# 0168 whole-branch review, IMPORTANT 5: a SECOND block for a harness that already has one. This
# was dead code — the guard counted against hd_harnesses(), which ends in `sort -u`, so its count
# could never exceed 1 and a duplicate block validated clean.
# The two blocks below share NO agent key, which is what makes this fixture load-bearing: a
# duplicate that DOES repeat a key is already caught by the duplicate-entry guard further down,
# so a repeating fixture would go green against the dead guard and prove nothing.
mut; printf '\n  codex:\n    adr:                   { model: x, effort: low }\n\n  codex:\n    status:                { model: y, effort: low }\n' >> "$T/hd.yml"
assert "reject: duplicate harness block" '! hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'
dup_diag="$(hd_validate "$T/hd.yml" "$SRC" 2>&1 || true)"
assert "reject: duplicate harness block names the harness" \
  'grep -q "duplicate harness block .codex." <<<"$dup_diag"'

# 0168 whole-branch review, IMPORTANT 6: a provider-prefixed model ID. The value class used to be
# [A-Za-z0-9._-]+, which stops at the '/' — hd_field returned "anthropic", hd_validate saw a
# non-empty value and passed, and a WRONG pin got generated. ADR-0015 makes model IDs opaque
# passthrough with no allowlist, so the reader must consume the whole token.
mut; sed -i.bak 's|^    adr:.*|    adr:                   { model: anthropic/claude-opus-5, effort: low }|' "$T/hd.yml"
assert "reader: a '/'-bearing model ID is read WHOLE" \
  '[ "$(hd_field "$T/hd.yml" claude adr model)" = "anthropic/claude-opus-5" ]'
assert "accept: a '/'-bearing model ID validates" 'hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'

mut; sed -i.bak 's|^    adr:.*|    adr:                   { model: openai:gpt-5.6-sol, effort: low }|' "$T/hd.yml"
assert "reader: a ':'-bearing model ID is read WHOLE" \
  '[ "$(hd_field "$T/hd.yml" claude adr model)" = "openai:gpt-5.6-sol" ]'
assert "accept: a ':'-bearing model ID validates" 'hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'

# 0168 whole-branch review, MINOR 2: a QUOTED scalar. The reader consumes bare tokens only, so a
# quoted value is truncated. Rejecting it with a diagnostic that names the real problem beats the
# misleading "missing a non-empty 'model'" the completeness check would otherwise emit.
mut; sed -i.bak 's|^    adr:.*|    adr:                   { model: "claude opus 5", effort: low }|' "$T/hd.yml"
assert "reject: quoted (non-bare) scalar" '! hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'
q_diag="$(hd_validate "$T/hd.yml" "$SRC" 2>&1 || true)"
assert "reject: quoted scalar diagnostic names bareness, not absence" \
  'grep -q "bare" <<<"$q_diag" && ! grep -q "missing a non-empty .model." <<<"$q_diag"'

assert "reject: missing file" '! hd_validate "$T/nope.yml" "$SRC" 2>/dev/null'
rm -rf "$T"

[ "$fail" = 0 ] && echo "PASS" || echo "FAIL"
exit "$fail"
