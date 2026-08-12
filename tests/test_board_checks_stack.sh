#!/usr/bin/env bash
# tests/test_board_checks_stack.sh — the two STACKED health checks in scripts/board-checks.sh
# (change 0298): `stack-invalid` and `stack-parent-killed`.
#
# SIBLING SHARD, not an extension: tests/test_board_checks.sh owns the other checks but sits at 55s
# in tests/runtime-budgets.tsv, and that table's header says the remedy for a spent row is a shard,
# never a raise. The correspondence guards that pin the four registration surfaces stay in that
# file — they are derived from board-checks.sh itself and belong with the rest of the registry.
#
# What this file pins is that the two checks are SEPARATE and that each keys on the resolver's own
# exit code: `stack_effective_base` exits 3 for a killed parent and 4 for an unresolvable chain, and
# the remedies differ (a scoping decision versus a data repair), so collapsing them into one id
# must redden here. The `! has_finding stack-invalid 74` assert is what carries that: a collapse
# emits stack-invalid for the killed case too.
#
# Hermetic: a bare origin plus one clone, so `refs/remotes/origin/<branch>` is a real ref. That
# matters — rule 1's remote-ref conjunct is exactly what separates fixture 75 (a parent whose branch
# is pushed) from fixture 73 (a parent whose branch was never pushed), and a stubbed git could not
# be used here because board-checks.sh needs a real one for its other checks. No gh, no network.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO/scripts/board-checks.sh"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }
git_quiet(){ git "$@" >/dev/null 2>&1; }

# has_finding CHECK-ID CHANGE-ID — exit 0 iff $out has a line beginning with the LITERAL
# "<check-id><TAB><change-id><TAB>" prefix. Same shape as tests/test_board_checks.sh's helper,
# closing over the single $out this file produces: matched with `case` against a QUOTED pattern, so
# no argument value is ever reinterpreted as a glob or an ERE metacharacter, and consumed from a
# here-string rather than a pipe, because this file runs under `set -uo pipefail` where feeding a
# producer into an early-exiting consumer is a real hazard.
has_finding(){
  local prefix line
  prefix="$1"$'\t'"$2"$'\t'
  while IFS= read -r line; do
    case "$line" in
      "$prefix"*) return 0 ;;
    esac
  done <<<"$out"
  return 1
}

assert "script exists and is executable" '[ -x "$SCRIPT" ]'

tmp="$(mktemp -d "${TMPDIR:-/tmp}/board-checks-stack.XXXXXX")"; trap 'rm -rf "$tmp"' EXIT

# --- fixture: a bare origin and one clone carrying main, docket, and one PUSHED feature branch ---
git_quiet init --bare "$tmp/origin.git"
git_quiet clone "$tmp/origin.git" "$tmp/work"
W="$tmp/work"
git -C "$W" config user.email t@t; git -C "$W" config user.name t
git_quiet -C "$W" checkout -b main
mkdir -p "$W/docs/superpowers/plans"
echo "# plan" > "$W/docs/superpowers/plans/2026-08-12-present.md"
git -C "$W" add -A; git_quiet -C "$W" commit -m "main artifacts"
git_quiet -C "$W" push -u origin main
# feat/parent is PUSHED; feat/unpushed is deliberately never created anywhere. The pair is the
# whole discrimination: a `branch:` value alone never satisfies rule 1.
git_quiet -C "$W" checkout -b feat/parent
git_quiet -C "$W" commit --allow-empty -m "parent work"
git_quiet -C "$W" push -u origin feat/parent
git_quiet -C "$W" checkout --orphan docket
git_quiet -C "$W" rm -rf .
mkdir -p "$W/docs/changes/active" "$W/docs/changes/archive"
git -C "$W" add -A 2>/dev/null; git_quiet -C "$W" commit --allow-empty -m "docket baseline"
git_quiet -C "$W" push -u origin docket

CD="$W/docs/changes"
mkchange(){ # mkchange <id> <slug> <status> [stacked_on] [branch]
  cat > "$CD/active/$(printf '%04d' "$1")-$2.md" <<EOF
---
id: $1
slug: $2
title: "Change $1"
status: $3
priority: medium
created: 2026-08-12
updated: 2026-08-12
depends_on: []
stacked_on: ${4:-}
branch: ${5:-}
---

## Why

Fixture. Body prose may legitimately mention stacked_on: 9999 without being frontmatter.
EOF
}

# The parents.
mkchange 70 parent implemented "" feat/parent          # live parent, branch pushed  -> rule 1
mkchange 77 killed-parent killed "" feat/killed        # spec §9: a human decision, never a fallback
mkchange 79 unpushed in-progress "" feat/unpushed      # claimed but never pushed     -> rule 4

# The children under test.
mkchange 71 missing-parent proposed 99                 # names a parent that does not exist
mkchange 72 cycle-a proposed 78                        # 72 -> 78 -> 72
mkchange 78 cycle-b proposed 72
mkchange 73 unpushed-base proposed 79                  # parent's branch has no remote ref
mkchange 74 killed-base proposed 77                    # parent killed
mkchange 75 well-formed proposed 70                    # resolves to feat/parent
mkchange 76 unstacked proposed ""                      # carries no stacked_on at all

# The killed change reached across a RECURSIVE hop. 82 is stacked-merged with a branch that was
# never pushed, so the resolver falls back to its parent's base and lands on killed 77 — one hop up
# from 83's own `stacked_on:`. 82 is non-terminal, so it reports too, giving both message shapes in
# a single run.
mkchange 82 sm-hop stacked-merged 77 feat/gone
mkchange 83 killed-grandparent proposed 82

out="$(bash "$SCRIPT" --changes-dir "$CD" --metadata-branch docket --integration-branch main 2>/dev/null)"

assert "a missing stacked_on parent is flagged" 'has_finding stack-invalid 71'
assert "a cycle is flagged" 'has_finding stack-invalid 72'
assert "a populated branch with no remote ref is flagged" 'has_finding stack-invalid 73'
assert "a killed parent flags its descendant separately" 'has_finding stack-parent-killed 74'
assert "a well-formed stack produces no finding" '! has_finding stack-invalid 75 && ! has_finding stack-parent-killed 75'
assert "an unstacked change produces no finding" '! has_finding stack-invalid 76'

# The separateness pin, stated in the other direction too. Collapsing the two checks into one that
# emits stack-invalid for exit 3 as well leaves the assert above red AND this one red; a single
# direction would let a future "simplification" that renames rather than merges slip through.
assert "the killed-parent case is NOT also reported as stack-invalid (two checks, two remedies)" \
  '! has_finding stack-invalid 74'
assert "the invalid cases are NOT reported as stack-parent-killed" \
  '! has_finding stack-parent-killed 71 && ! has_finding stack-parent-killed 73'

# A terminal change is out of scope: its chain is history, and re-parenting a `done` change is not a
# remedy anyone can act on. Seeded WITH a stacked_on so the gate under test is the STATUS gate — a
# fixture without one would short-circuit before reaching it and pass no matter what.
mkchange 80 done-child done 99
mkchange 81 killed-child killed 99
tout="$(bash "$SCRIPT" --changes-dir "$CD" --metadata-branch docket --integration-branch main 2>/dev/null)"
out="$tout"
assert "a done change with a broken chain is not flagged (terminal is out of scope)" \
  '! has_finding stack-invalid 80'
assert "a killed change with a broken chain is not flagged" \
  '! has_finding stack-invalid 81'
assert "the terminal-scope asserts are not vacuous — the live cases still fire in the same run" \
  'has_finding stack-invalid 71 && has_finding stack-parent-killed 74'

# The messages carry the remedy, not just the fact. Both name the parent by padded id, because
# "your chain is broken" without the id is a finding a human has to re-derive by hand.
killed_line="$(grep -F "stack-parent-killed"$'\t'"74" <<<"$tout")"
invalid_71="$(grep -F "stack-invalid"$'\t'"71" <<<"$tout")"
assert "the killed-parent message names the parent by padded id" \
  '[ -n "$killed_line" ] && grep -qF -- "#0077" <<<"$killed_line"'
assert "the killed-parent message names the human remedy, never a fallback" \
  'grep -qF -- "rescope" <<<"$killed_line"'
assert "the invalid message names the parent by padded id" \
  '[ -n "$invalid_71" ] && grep -qF -- "#0099" <<<"$invalid_71"'
assert "the invalid message names the data repairs" \
  'grep -qF -- "push" <<<"$invalid_71"'

# …and it must not ASSERT that the named parent is the killed one when the kill sits further up.
# 83's own `stacked_on:` is 82 (stacked-merged, alive); the killed change is 77. A message that
# says "stacked on #0082, which is killed" points the human at a change that is not killed.
killed_83="$(grep -F "stack-parent-killed"$'\t'"83" <<<"$tout")"
assert "the killed-ancestor case is reported at all" '[ -n "$killed_83" ]'
assert "the killed-ancestor message names the KILLED change, not just the immediate parent" \
  'grep -qF -- "#0077" <<<"$killed_83"'
assert "the killed-ancestor message still names the immediate parent as the edge to start from" \
  'grep -qF -- "#0082" <<<"$killed_83"'
assert "the killed-ancestor message never asserts the immediate parent is the killed one" \
  '! grep -qF -- "#0082, which is killed" <<<"$killed_83"'
# The direct case keeps the plain assertion — it is true there, and generalizing every message to
# the chain phrasing would make the common finding read as a puzzle.
assert "the immediate-parent case still states the parent itself is killed" \
  'grep -qF -- "#0077, which is killed" <<<"$killed_line"'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
