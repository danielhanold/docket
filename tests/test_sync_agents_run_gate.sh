#!/usr/bin/env bash
# tests/test_sync_agents_run_gate.sh — the caller-side run gate is single-sourced and rendered
# identically into every parent-facing surface (change 0242).
# run: bash tests/test_sync_agents_run_gate.sh
set -uo pipefail
unset XDG_CONFIG_HOME
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SYNC="$REPO/sync-agents.sh"
GATE_SRC="$REPO/cursor-rules/run-gate.md"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# Collapse runs of whitespace so an assert about a CLAIM survives a pure re-flow of the prose
# (learnings: phrase-grep-over-wrapped-prose).
flat(){ tr '\n' ' ' < "$1" | tr -s '[:space:]' ' '; }

mk_repo(){  # $1 = agent_harnesses list body, e.g. "[claude, codex]"
  SBX="$(mktemp -d "${TMPDIR:-/tmp}/rungate.XXXXXX")"
  git -C "$SBX" init --quiet
  git -C "$SBX" config user.email t@t.test
  git -C "$SBX" config user.name Test
  printf 'agent_harnesses: %s\n' "$1" > "$SBX/.docket.yml"
  ( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
}

# --- the template source exists and is the ONLY place the gate text is authored ---
assert "run-gate template exists" '[ -f "$GATE_SRC" ]'
# The gate's own commands must not be hand-copied into either assembler. Anchored on the
# distinctive flag, which appears in the template and nowhere else in the generator.
assert "sync-agents.sh does not restate the gate commands inline" \
  '[ "$(grep -c -- "--in-progress-ids" "$SYNC")" = "0" ]'

# --- brevity: the block rides always-loaded context in every harness (spec Risks) ---
GATE_LINES="$(grep -c "" "$GATE_SRC" 2>/dev/null || echo 0)"
assert "gate text is at most 14 lines" '[ "$GATE_LINES" -ge 1 ] && [ "$GATE_LINES" -le 14 ]'

# --- the four behavioral claims, each bound to what it is asserted ABOUT ---
# (learnings: prose-guard-binds-phrase-to-claim — never a bare phrase-presence grep)
G="$(flat "$GATE_SRC" 2>/dev/null)"
assert "gate: snapshots the in-progress set BEFORE dispatching" \
  '[[ "$G" == *"Before dispatching"*"verify-run --in-progress-ids"* ]]'
assert "gate: verifies the attributed id after the return" \
  '[[ "$G" == *"After the return"*"verify-run <id>"* ]]'
assert "gate: run-halted never re-dispatches" \
  '[[ "$G" == *"run-halted"*"never re-dispatch"* ]]'
assert "gate: run-incomplete re-dispatches exactly ONCE, then stops" \
  '[[ "$G" == *"run-incomplete"*"once"* ]] && [[ "$G" == *"Never a third"* ]]'

# --- rendered into BOTH surfaces, byte-identically ---
mk_repo "[cursor, codex]"
CUR="$SBX/.cursor/rules/docket-dispatch.mdc"
AGM="$SBX/AGENTS.md"
assert "cursor rule was generated"   '[ -f "$CUR" ]'
assert "AGENTS.md block was written" '[ -f "$AGM" ]'

# Slice the gate out of each rendered surface by its own heading, bounded by the TEMPLATE'S OWN
# LINE COUNT (learnings: section-slice-needs-a-named-terminator — and here the two surfaces share
# no terminator: the cursor rule ends the gate with the next `## ` fragment heading, while the
# AGENTS.md block ends it with a bullet list. A count taken from the single source is the one
# bound both surfaces share, and it also makes the slice a VERBATIM-rendering check rather than a
# same-prefix check).
slice_gate(){ awk -v n="$GATE_LINES" '/^## Run gate/{g=1} g{print; if (++c==n) exit}' "$1"; }
slice_gate "$CUR" > "$SBX/.gate-cursor"
slice_gate "$AGM" > "$SBX/.gate-agents"
assert "gate is present in the cursor rule"    '[ -s "$SBX/.gate-cursor" ]'
assert "gate is present in the AGENTS.md block" '[ -s "$SBX/.gate-agents" ]'
assert "the cursor rule renders the template verbatim" \
  'diff -q "$GATE_SRC" "$SBX/.gate-cursor" >/dev/null'
assert "the AGENTS.md block renders the template verbatim" \
  'diff -q "$GATE_SRC" "$SBX/.gate-agents" >/dev/null'
assert "the two rendered gates are byte-identical" \
  'diff -q "$SBX/.gate-cursor" "$SBX/.gate-agents" >/dev/null'
# The AGENTS.md block must still close: the gate is spliced INSIDE the managed block, and an
# unterminated block is a corrupt managed region, not a rendering detail.
assert "the AGENTS.md dispatch end marker exists" 'grep -q "docket:dispatch:end" "$AGM"'

# --- reachability: the gate arrives at a Claude parent, not merely at a template ---
mk_repo "[claude]"
assert "reachability: a claude-only repo has a Claude surface" '[ -e "$SBX/CLAUDE.md" ]'
assert "reachability: the gate is readable through that surface" \
  'grep -q -- "verify-run --in-progress-ids" "$SBX/CLAUDE.md"'

# --- the convention's pointer names the gate and binds it to the verification obligation ---
# Bound to the Composition paragraph itself, not the whole file: `verify-run` and `once` both occur
# elsewhere in the convention, so a whole-file window matches even with the pointer sentence deleted
# (mutation-proven vacuous). The paragraph is the named terminator here — it is one physical line.
CONV="$REPO/skills/docket-convention/SKILL.md"
C="$(grep -m1 '^\*\*Composition' "$CONV" | tr -s '[:space:]' ' ')"
assert "convention: the Composition paragraph exists to be asserted about" '[ -n "$C" ]'
assert "convention: Composition points at the managed-block gate" \
  '[[ "$C" == *"uncommitted working-tree files"*"verify-run"*"once"* ]]'

echo; [ "$fail" = 0 ] && echo "ALL PASS" || echo "FAILURES"; exit "$fail"
