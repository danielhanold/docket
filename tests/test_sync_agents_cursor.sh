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
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# Single-line frontmatter scalar. Anchored to the FIRST --- block would be stricter, but these
# generated wrappers are emitter output with a known shape and no body prose using these keys.
fm(){ sed -n "s/^$2:[[:space:]]*//p" "$1" | head -n1 | sed 's/[[:space:]]*$//'; }
# Frontmatter = lines between the first two --- fences. Key-absence asserts MUST scope to it:
# a bare `grep -q '^effort:'` over the whole file would also match wrapper body prose.
front(){ awk '/^---[[:space:]]*$/{d++; next} d==1{print} d>=2{exit}' "$1"; }
has_fm_key(){ local f; f="$(front "$1")"; grep -qE "^$2[[:space:]]*:" <<<"$f"; }

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
assert "cursor: full built-in set (9 files)"  '[ "$(find "$SBX/.cursor/agents" -name "docket-*.md" | wc -l | tr -d " ")" = "9" ]'
assert "cursor: emits NO standalone effort: key" '! has_fm_key "$C" effort'
assert "cursor: emits NO skills: preload key"    '! has_fm_key "$C" skills'
assert "cursor: emits name"                      '[ "$(fm "$C" name)" = "docket-status" ]'
assert "cursor: emits description from source"   '[ "$(fm "$C" description)" = "$(sed -n "/^description:/{s/^description:[[:space:]]*//;p;q;}" "$REPO/agents/docket-status.md")" ]'
assert "cursor: does NOT emit readonly"          '! has_fm_key "$C" readonly'
assert "cursor: does NOT emit is_background"     '! has_fm_key "$C" is_background'

# --- effort rides INSIDE the model value ---------------------------------------------------------
assert "cursor: model carries bracket-encoded effort" \
  '[ "$(fm "$C" model)" = "claude-haiku-4-5-20251001[effort=medium]" ]'

# --- the body preamble replaces the inert skills: preload ----------------------------------------
assert "cursor: body preamble names the agent's own skill" 'grep -qF "docket-status" "$C"'
assert "cursor: body preamble names docket-convention"     'grep -qF "docket-convention" "$C"'
assert "cursor: preamble tells the child to LOAD them"     'grep -qiF "load these docket skills" "$C"'
assert "cursor: wrapper body survives verbatim"            'grep -qi "refresh docket state" "$C"'

# --- the claude and codex sides are untouched (emitter-split regression guard) --------------------
assert "cursor split: claude wrapper still byte-identical to its source" \
  'diff -q "$REPO/agents/docket-status.md" "$SBX/.claude/agents/docket-status.md" >/dev/null'
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
