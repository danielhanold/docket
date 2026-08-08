#!/usr/bin/env bash
# tests/test_sync_agents_harness_gaps.sh — the two gaps change 0245 closes in sync-agents.sh:
#   (1) an accepted-but-unmapped harness token silently got a Claude-shaped wrapper (#0142);
#   (2) a global agent_harnesses with no repo opt-in generated nothing and said nothing (#0082).
# Both are DIAGNOSTIC contracts, so every assertion here reads stderr, not a generated file.
# run: bash tests/test_sync_agents_harness_gaps.sh
set -u
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

REPO="$(cd "$(dirname "$0")/.." && pwd)"
WORK="$(mktemp -d "${TMPDIR:-/tmp}/docket-gaps-XXXXXX")"
trap 'rm -rf "$WORK"' EXIT

mk_repo(){  # $1=dest  $2=.docket.yml body
  mkdir -p "$1"; git -C "$1" init -q 2>/dev/null || true
  printf '%s\n' "$2" > "$1/.docket.yml"
}

# --- (1) unmapped token WARNs, once per harness per run ---------------------
RK="$WORK/kiro"; mk_repo "$RK" 'agent_harnesses: [claude, kiro]'
( cd "$RK" && DOCKET_HARNESS_ROOT="$WORK/home-kiro" bash "$REPO/sync-agents.sh" ) >"$WORK/kiro.out" 2>"$WORK/kiro.err" || true

assert "unmapped token 'kiro' produces a WARN" \
  'grep -q "WARN" "$WORK/kiro.err" && grep -q "kiro" "$WORK/kiro.err"'
assert "the WARN names the unverified Claude-shaped wrapper" \
  'grep -qi "claude-shaped" "$WORK/kiro.err"'
# 16 agents are generated per harness; a per-wrapper warn would print 16 lines.
assert "the WARN fires exactly once for the run, not once per wrapper" \
  '[ "$(grep -c "unverified for .kiro" "$WORK/kiro.err")" = "1" ]'
assert "the unmapped run still generates kiro wrappers (WARN, not refusal)" \
  '[ "$(ls "$RK"/.kiro/agents/docket-*.md 2>/dev/null | wc -l | tr -d " ")" = "16" ]'

# A named harness must stay silent — this is the discriminating half. A WARN that fired for every
# token would pass every assert above and still be wrong.
RC="$WORK/claudeonly"; mk_repo "$RC" 'agent_harnesses: [claude, codex, cursor, opencode]'
( cd "$RC" && DOCKET_HARNESS_ROOT="$WORK/home-claude" bash "$REPO/sync-agents.sh" ) >/dev/null 2>"$WORK/claude.err" || true
assert "no unmapped WARN for the four named harnesses" \
  '! grep -q "unverified for" "$WORK/claude.err"'

# --- (2) --check surfaces it as a NON-failing advisory ----------------------
( cd "$RK" && DOCKET_HARNESS_ROOT="$WORK/home-kiro" bash "$REPO/sync-agents.sh" --check ) >"$WORK/chk.out" 2>"$WORK/chk.err"; chk_rc=$?
assert "--check prints an advisory for the unmapped token" \
  'grep -q "advisory:" "$WORK/chk.err" && grep -q "kiro" "$WORK/chk.err"'
assert "--check advisory does NOT fail the run (rc unchanged)" '[ "'"$chk_rc"'" = "0" ]'

( cd "$RC" && DOCKET_HARNESS_ROOT="$WORK/home-claude" bash "$REPO/sync-agents.sh" --check ) >/dev/null 2>"$WORK/chk2.err" || true
assert "--check prints no unmapped advisory for named harnesses" \
  '! grep -q "advisory: harness" "$WORK/chk2.err"'

echo "---"; [ "$fail" = "0" ] && echo "ALL PASS" || echo "FAILURES"; exit $fail
