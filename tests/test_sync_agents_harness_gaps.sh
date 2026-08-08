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
  'grep -q "WARN harness .kiro. has no named emitter" "$WORK/kiro.err"'
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
  'grep -q "advisory: harness .kiro." "$WORK/chk.err"'
assert "--check advisory does NOT fail the run (rc unchanged)" '[ "'"$chk_rc"'" = "0" ]'

( cd "$RC" && DOCKET_HARNESS_ROOT="$WORK/home-claude" bash "$REPO/sync-agents.sh" --check ) >/dev/null 2>"$WORK/chk2.err" || true
assert "--check prints no unmapped advisory for named harnesses" \
  '! grep -q "advisory: harness" "$WORK/chk2.err"'

# --- (3) #0082: global agent_harnesses + no repo opt-in is no longer silent ---
# Four cells, because the hint must fire in exactly ONE of them. A test of only the firing cell
# cannot separate "fires correctly" from "fires always".
GH="$WORK/gcfg"; mkdir -p "$GH/docket"
printf 'agent_harnesses: [claude, cursor]\n' > "$GH/docket/config.yml"

run_cell(){  # $1=label  $2=global-set(1|0)  $3=repo .docket.yml body or empty
  local d="$WORK/cell-$1"; mkdir -p "$d"; git -C "$d" init -q 2>/dev/null || true
  [ -n "$3" ] && printf '%s\n' "$3" > "$d/.docket.yml"
  if [ "$2" = 1 ]; then
    ( cd "$d" && XDG_CONFIG_HOME="$GH" DOCKET_HARNESS_ROOT="$WORK/h-$1" bash "$REPO/sync-agents.sh" ) >/dev/null 2>"$WORK/cell-$1.err" || true
  else
    ( cd "$d" && XDG_CONFIG_HOME="$WORK/empty-xdg" DOCKET_HARNESS_ROOT="$WORK/h-$1" bash "$REPO/sync-agents.sh" ) >/dev/null 2>"$WORK/cell-$1.err" || true
  fi
}
mkdir -p "$WORK/empty-xdg"
HINT="has not opted in"

run_cell global-noopt 1 ''
assert "#0082: global set + no repo opt-in PRINTS the hint" \
  'grep -qF "'"$HINT"'" "$WORK/cell-global-noopt.err"'
assert "#0082: the hint names .docket.local.yml" \
  'grep -qF ".docket.local.yml" "$WORK/cell-global-noopt.err"'
assert "#0082: the hint names the global config path it read" \
  'grep -qF "docket/config.yml" "$WORK/cell-global-noopt.err"'

run_cell global-opted 1 'agent_harnesses: [claude]'
assert "#0082: global set + repo OPTED IN stays silent" \
  '! grep -qF "'"$HINT"'" "$WORK/cell-global-opted.err"'

run_cell noglobal-noopt 0 ''
assert "#0082: no global + no opt-in stays silent" \
  '! grep -qF "'"$HINT"'" "$WORK/cell-noglobal-noopt.err"'

run_cell noglobal-opted 0 'agent_harnesses: [claude]'
assert "#0082: no global + repo opted in stays silent" \
  '! grep -qF "'"$HINT"'" "$WORK/cell-noglobal-opted.err"'

# The hint must name the path the run ACTUALLY read, which is derived from GLOBAL_CFG:
#   ${XDG_CONFIG_HOME:-$DOCKET_HARNESS_ROOT-or-$HOME/.config}/docket/config.yml
# Every cell above sets XDG_CONFIG_HOME, so a hint that re-derived the fallback as $HOME/.config
# would still match them. This cell leaves XDG_CONFIG_HOME UNSET so the harness-root fallback is
# the live branch, and asserts on the computed path rather than a literal.
XH="$WORK/h-xdgless"; mkdir -p "$XH/.config/docket"
printf 'agent_harnesses: [claude, cursor]\n' > "$XH/.config/docket/config.yml"
XD="$WORK/cell-xdgless"; mkdir -p "$XD"; git -C "$XD" init -q 2>/dev/null || true
( cd "$XD" && unset XDG_CONFIG_HOME; DOCKET_HARNESS_ROOT="$XH" bash "$REPO/sync-agents.sh" ) \
  >/dev/null 2>"$WORK/cell-xdgless.err" || true
assert "#0082: XDG unset — the hint still fires" \
  'grep -qF "'"$HINT"'" "$WORK/cell-xdgless.err"'
assert "#0082: XDG unset — the hint names the global config the run actually read" \
  'grep -qF "$XH/.config/docket/config.yml" "$WORK/cell-xdgless.err"'

echo "---"; [ "$fail" = "0" ] && echo "ALL PASS" || echo "FAILURES"; exit $fail
