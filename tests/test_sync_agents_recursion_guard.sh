#!/usr/bin/env bash
# tests/test_sync_agents_recursion_guard.sh — the exact-name self-recursion guard in
# every generated wrapper (change 0334, task 8). Every renderer injects the
# byte-identical guard paragraph naming the wrapper's OWN agent, prohibiting exactly
# one edge (docket-X dispatching another docket-X for the assignment it already holds)
# while preserving required dispatches to DIFFERENT agents. This is the shell mirror's
# half of internal/harness/cross_harness_test.go's TestWrapperRecursionGuard.
#
# Population is COMPUTED from the source inventory, never hand-written
# (learnings: backstop-must-compute-not-reenumerate). The correspondence is asserted
# in BOTH directions as two one-way checks (learnings: correspondence-guard-runs-one-way):
#   A. every SOURCE agent produced a wrapper (per-harness count == source count), and
#   B. every generated wrapper's guard names THAT file's own agent.
#
# Mutation matrix — the Go probes live in cross_harness_test.go; the shell-side probe:
# strip the `recursion_guard` call from ANY one emitter in sync-agents.sh (emit,
# emit_codex_toml, emit_cursor_md, emit_opencode_md) -> that harness's per-file
# identity assert reddens here.
# run: bash tests/test_sync_agents_recursion_guard.sh
set -uo pipefail
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BT='`'   # a literal backtick, kept out of every double-quoted grep pattern below

WORK="$(mktemp -d "${TMPDIR:-/tmp}/docket-rg-XXXXXX")"
trap 'rm -rf "$WORK"' EXIT

# One fixture repo listing ALL FOUR wrapper-generating harnesses, so every renderer's
# output directory is populated in a single run.
R="$WORK/repo"
mkdir -p "$R"
git -C "$R" init -q 2>/dev/null || true
printf 'agent_harnesses: [claude, codex, cursor, opencode]\n' > "$R/.docket.yml"
( cd "$R" && DOCKET_HARNESS_ROOT="$WORK/home" bash "$REPO/sync-agents.sh" >"$WORK/gen.log" 2>&1 ) || true

# The expected wrapper count per harness, computed from the committed source inventory.
expected="$(ls "$REPO"/agents/docket-*.md 2>/dev/null | wc -l | tr -d ' ')"
assert "source inventory is non-empty (population floor)" '[ "${expected:-0}" -ge 1 ]'

total=0
for spec in "claude:.claude/agents:md" "codex:.codex/agents:toml" "cursor:.cursor/agents:md" "opencode:.opencode/agents:md"; do
  harness="${spec%%:*}"; rest="${spec#*:}"; dir="$R/${rest%%:*}"; ext="${rest##*:}"

  # Direction A: the GENERATED output directory listing matches the source inventory.
  got="$(ls "$dir"/docket-*."$ext" 2>/dev/null | wc -l | tr -d ' ')"
  assert "$harness: generated wrapper count ($got) equals source count ($expected)" \
    '[ "${got:-0}" = "$expected" ]'

  for f in "$dir"/docket-*."$ext"; do
    [ -e "$f" ] || continue
    total=$((total+1))
    base="$(basename "$f")"; name="${base%.*}"   # docket-status.md / docket-status.toml -> docket-status

    # Whitespace-collapsed body so a phrase assert binds to the words, not the
    # wrapping the renderer chose (learnings: phrase-grep-over-wrapped-prose).
    # Captured into a variable first, then matched with a here-string — never a
    # producer piped into an early-exiting grep -q under pipefail (AGENTS.md).
    collapsed="$(tr -s '[:space:]' ' ' < "$f")"

    # Direction B: the guard in THIS file names THIS file's own agent, in both the
    # identity sentence and the exact-name prohibition (each interpolates the name).
    assert "$harness/$name: guard identity names its own agent" \
      'grep -qF -- "You are already running as ${BT}${name}${BT}" <<<"$collapsed"'
    assert "$harness/$name: guard forbids dispatching another of ITSELF (exact name)" \
      'grep -qF -- "Do not dispatch another ${BT}${name}${BT}" <<<"$collapsed"'
    assert "$harness/$name: guard preserves required different-agent dispatches" \
      'grep -qF -- "Dispatches to different agents explicitly required" <<<"$collapsed"'
    assert "$harness/$name: guard does not regress to the preloaded-skill phrasing" \
      '! grep -qF -- "your preloaded skill" <<<"$collapsed"'
  done
done

# The per-file asserts are meaningful only if wrappers were actually generated.
assert "at least one generated wrapper was checked (non-vacuity)" '[ "$total" -ge 1 ]'
assert "total generated wrappers == 4 harnesses x source count" '[ "$total" = "$((expected*4))" ]'

echo "---"; [ "$fail" = "0" ] && echo "ALL PASS" || echo "FAILURES"; exit "$fail"
