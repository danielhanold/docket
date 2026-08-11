#!/usr/bin/env bash
# tests/test_sync_agents_cursor.sh — Cursor harness wrapper generation against Cursor's OWN
# documented subagent contract (change 0135). Cursor documents exactly five frontmatter fields
# (name, description, model, readonly, is_background) and encodes reasoning effort INSIDE the
# model value; it has no `effort:` field and no `skills:` preload. Before 0135 docket emitted a
# Claude-shaped wrapper here, so the model pin, the effort pin, and the skills were all silently
# ignored while docket reported them as honored.
# run: bash tests/test_sync_agents_cursor.sh
set -uo pipefail
unset XDG_CONFIG_HOME
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SYNC="$REPO/sync-agents.sh"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

# Single-line frontmatter scalar. Anchored to the FIRST --- block would be stricter, but these
# generated wrappers are emitter output with a known shape and no body prose using these keys.
fm(){ sed -n "s/^$2:[[:space:]]*//p" "$1" | sed -n 1p | sed 's/[[:space:]]*$//'; }
# Frontmatter = lines between the first two --- fences. Key-absence asserts MUST scope to it:
# a bare `grep -q '^effort:'` over the whole file would also match wrapper body prose.
front(){ awk '/^---[[:space:]]*$/{d++; next} d==1{print} d>=2{exit}' "$1"; }
has_fm_key(){ local f; f="$(front "$1")"; grep -qE "^$2[[:space:]]*:" <<<"$f"; }
# Body = everything after the frontmatter's closing fence. Change 0168 made emit() strip-and-insert
# the resolved model/effort, so a generated wrapper is no longer byte-identical to its source.
body_of(){ awk '/^---[[:space:]]*$/ && d<2 {d++; next} d>=2 {print}' "$1"; }

mk_cursor_repo(){  # $1 = optional .docket.yml body (default: bare [claude, cursor] opt-in)
  SBX="$(mktemp -d)"
  git -C "$SBX" init --quiet
  git -C "$SBX" config user.email t@t.test
  git -C "$SBX" config user.name Test
  printf '%s' "${1:-$(printf 'agent_harnesses: [claude, cursor]\n')}" > "$SBX/.docket.yml"
  ( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
  C="$SBX/.cursor/agents/docket-status.md"
}

# --- the two fields Cursor does not have are GONE ------------------------------------------------
mk_cursor_repo
assert "cursor: wrapper generated"            '[ -f "$C" ]'
assert "cursor: full built-in set (16 files)"  '[ "$(find "$SBX/.cursor/agents" -name "docket-*.md" | wc -l | tr -d " ")" = "16" ]'
assert "cursor: emits NO standalone effort: key" '! has_fm_key "$C" effort'
assert "cursor: emits NO skills: preload key"    '! has_fm_key "$C" skills'
assert "cursor: emits name"                      '[ "$(fm "$C" name)" = "docket-status" ]'
assert "cursor: emits description from source"   '[ "$(fm "$C" description)" = "$(sed -n "/^description:/{s/^description:[[:space:]]*//;p;q;}" "$REPO/agents/docket-status.md")" ]'
assert "cursor: does NOT emit readonly"          '! has_fm_key "$C" readonly'
assert "cursor: does NOT emit is_background"     '! has_fm_key "$C" is_background'

# --- a non-build cursor agent resolves its OWN cursor ID, not a claude one -----------------------
# Before change 0168 this asserted `claude-haiku-4-5-20251001[effort=medium]` — docket-status's
# CLAUDE pin, read off the wrapper source and leaked onto a harness that cannot run it. 0168 first
# made it honestly UNPINNED (no model line); the amendment completes the cursor block, so the
# correct output is now docket-status's own CURSOR ID.
#
# Read from the sidecar rather than hard-coded: this asserts "the wrapper says what docket ships
# for cursor/status", which is the no-leak property itself. A literal would restate the sidecar and
# would still pass if BOTH moved to a claude ID together.
# The honest-unpinned path is not lost, but it moved: since change 0169 all three shipped harnesses
# carry complete blocks, so no shipped harness can reach it. It is now covered in
# tests/test_sync_agents.sh by the repo-copy fixture whose asserts read
# "0169 fixture: the copy has no codex block" — it strips codex from the copy's
# HD_SHIPPED_HARNESSES and deletes its block, reconstructing "known but unshipped".
# Bracket encoding is exercised below by the explicit
# model+effort override ("unknown model+effort pass through verbatim").
. "$REPO/scripts/lib/harness-defaults.sh"
cursor_status_id="$(hd_field "$REPO/agents/harness-defaults.yml" cursor status model)"
assert "cursor: non-build agent is pinned to its own cursor ID (no claude ID leak)" \
  '[ -n "$cursor_status_id" ] && [ "$(fm "$C" model)" = "$cursor_status_id" ]'
assert "cursor: a shipped auto-effort ID carries no bracket-encoded effort" \
  '! grep -q "\[effort=" "$C"'

# --- the body preamble replaces the inert skills: preload ----------------------------------------
assert "cursor: body preamble names the agent's own skill" 'grep -qF "docket-status" "$C"'
assert "cursor: body preamble names docket-convention"     'grep -qF "docket-convention" "$C"'
assert "cursor: preamble tells the child to LOAD them"     'grep -qiF "load these docket skills" "$C"'
assert "cursor: wrapper body survives verbatim"            'grep -qi "refresh docket state" "$C"'

# --- the four shipped Cursor build-profile pins (change 0184) -------------------------------------
# Each sidecar ID is a COMPLETE Cursor built-in whose variant is already encoded, so its effort is
# `auto` and the emitter must write the ID verbatim rather than appending a second, conflicting
# [effort=…] suffix on top of the one already baked into the name.
assert "cursor: build-economy carries its shipped Cursor ID, no effort suffix" \
  '[ "$(fm "$SBX/.cursor/agents/docket-build-economy.md" model)" = "cursor-grok-4.5-low" ]'
assert "cursor: build-standard carries its shipped Cursor ID, no effort suffix" \
  '[ "$(fm "$SBX/.cursor/agents/docket-build-standard.md" model)" = "cursor-grok-4.5-medium" ]'
assert "cursor: build-premium carries its shipped Cursor ID, no effort suffix" \
  '[ "$(fm "$SBX/.cursor/agents/docket-build-premium.md" model)" = "cursor-grok-4.5-high" ]'
assert "cursor: build-max carries its shipped Cursor ID, no effort suffix" \
  '[ "$(fm "$SBX/.cursor/agents/docket-build-max.md" model)" = "claude-opus-5-high" ]'
for p in economy standard premium max; do
  assert "cursor: build-$p emits no standalone effort: key" \
    '! has_fm_key "$SBX/.cursor/agents/docket-build-'"$p"'.md" effort'
  assert "cursor: build-$p model has no appended [effort=…] suffix" \
    '! grep -q "\[effort=" "$SBX/.cursor/agents/docket-build-'"$p"'.md"'
done
# The four profiles must be four DISTINCT Cursor models — the Cursor ladder varies by model where
# the Claude ladder varies by effort, so a copy-paste that collapses them is the failure to catch.
cursor_profile_models="$(for p in economy standard premium max; do fm "$SBX/.cursor/agents/docket-build-$p.md" model; done)"
assert "cursor: the four build profiles carry four DISTINCT models" \
  '[ "$(grep -c . <<<"$cursor_profile_models")" = "4" ] &&
   [ "$(sort -u <<<"$cursor_profile_models" | grep -c .)" = "4" ]'

# --- the claude and codex sides are untouched (emitter-split regression guard) --------------------
# Byte identity with the source is now structurally impossible: change 0168 made the source a
# behavior-only template and the generator INJECTS the pin from agents/harness-defaults.yml. The
# mechanism this guarded — the emitter split changes NOTHING on the Claude side except that
# injection — is asserted directly instead.
assert "cursor split: claude wrapper body is verbatim from its source" \
  'diff -q <(body_of "$REPO/agents/docket-status.md") <(body_of "$SBX/.claude/agents/docket-status.md") >/dev/null'
assert "cursor split: claude wrapper name/description/skills come from the source" \
  '[ "$(fm "$SBX/.claude/agents/docket-status.md" name)" = "docket-status" ] &&
   [ "$(fm "$SBX/.claude/agents/docket-status.md" description)" = "$(fm "$REPO/agents/docket-status.md" description)" ] &&
   [ "$(fm "$SBX/.claude/agents/docket-status.md" skills)" = "$(fm "$REPO/agents/docket-status.md" skills)" ]'
# The load-bearing one: it pins BOTH halves — the generated file HAS the shipped pin and the source
# does NOT — so restoring a default to the source frontmatter reddens it just as dropping the
# injection does.
assert "cursor split: claude wrapper carries the SHIPPED pin, injected not copied" \
  '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "claude-haiku-4-5-20251001" ] &&
   [ "$(fm "$SBX/.claude/agents/docket-status.md" effort)" = "medium" ] &&
   ! grep -qE "^(model|effort):" "$REPO/agents/docket-status.md"'
assert "cursor split: claude wrapper still HAS effort:" 'has_fm_key "$SBX/.claude/agents/docket-status.md" effort'
assert "cursor split: claude wrapper still HAS skills:" 'has_fm_key "$SBX/.claude/agents/docket-status.md" skills'
rm -rf "$SBX"

# --- bare model when no effort resolves (effort: auto drops the effort) --------------------------
mk_cursor_repo "$(printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: gpt-5.5-medium-fast, effort: auto }\n')"
assert "cursor: effort auto => BARE model, no bracket" '[ "$(fm "$C" model)" = "gpt-5.5-medium-fast" ]'
assert "cursor: effort auto => still no effort: key"   '! has_fm_key "$C" effort'
rm -rf "$SBX"

# --- an arbitrary non-Claude id and an arbitrary effort token pass through VERBATIM --------------
# ADR-0015 + ADR-0059: docket holds no allowlist of Cursor model IDs or effort tokens. A committed
# table of a vendor's internals goes stale silently, and a stale entry produces a FALSE NEGATIVE
# that reads as a successful degrade. `zzz-not-a-real-effort` is the discriminating input: any
# validation layer would reject or rewrite it, and this assert would redden.
mk_cursor_repo "$(printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: gpt-5.5-medium-fast, effort: zzz-not-a-real-effort }\n')"
assert "cursor: unknown model+effort pass through verbatim (no allowlist)" \
  '[ "$(fm "$C" model)" = "gpt-5.5-medium-fast[effort=zzz-not-a-real-effort]" ]'
rm -rf "$SBX"

# --- inherit + no effort => NO model line at all --------------------------------------------------
mk_cursor_repo "$(printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: inherit, effort: auto }\n')"
assert "cursor: model inherit => no model: line" '! has_fm_key "$C" model'
rm -rf "$SBX"

# --- inherit + an effort => no model line, and a LOUD warn ----------------------------------------
# Effort has nowhere to attach without a model, so the pin is dropped — and a dropped pin must
# never be silent, since silently-dropped pins are the defect this whole change exists to fix.
SBX="$(mktemp -d)"
git -C "$SBX" init --quiet; git -C "$SBX" config user.email t@t.test; git -C "$SBX" config user.name Test
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: inherit, effort: xhigh }\n' > "$SBX/.docket.yml"
warn_out="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null )"
C="$SBX/.cursor/agents/docket-status.md"
assert "cursor: inherit+effort => no model: line"      '! has_fm_key "$C" model'
assert "cursor: inherit+effort => WARN emitted"        'grep -qi "effort" <<<"$warn_out" && grep -qi "dropped" <<<"$warn_out"'
assert "cursor: WARN names the cursor harness + agent"  'grep -qF "cursor/docket-status" <<<"$warn_out"'
# The WARN goes to STDERR and must never leak into the generated document.
assert "cursor: WARN never leaks into the wrapper file" '! grep -qi "dropped" "$C"'
rm -rf "$SBX"

echo "---"; [ "$fail" = "0" ] && echo "ALL PASS" || echo "FAILURES"; exit $fail
