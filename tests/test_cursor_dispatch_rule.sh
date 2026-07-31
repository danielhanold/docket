#!/usr/bin/env bash
# tests/test_cursor_dispatch_rule.sh — the Cursor dispatch rule instructs by CAPABILITY, never by
# a literal tool name (change 0135; ADR-0059 §2). A concrete call snippet may still appear, but
# only as a clearly-labelled ILLUSTRATION — naming a tool in an instruction primes the next agent
# to probe for that literal and conclude absence when the mechanism ships under a different name
# (learnings: capability-absence-needs-a-failed-attempt).
# run: bash tests/test_cursor_dispatch_rule.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

HEAD="$REPO/cursor-rules/dispatch.head.md"
assert "head: exists" '[ -f "$HEAD" ]'
# The head's `description:` frontmatter PARAPHRASES the same rules the instruction body states, so
# a positive assert run over the whole file is satisfied by the frontmatter alone while the actual
# instruction to the parent chat is deleted — mutation-proven: deleting `1. Do **NOT** run the skill
# inline in this chat.` outright left every head assert green, because `never run inline` in the
# description matched instead. Positive asserts therefore run over the BODY only: everything after
# the closing frontmatter delimiter. `*` is stripped first so bold markup cannot break a phrase
# match — that is the second half of how the frontmatter-anchoring hid, since `Do **NOT** run` does
# not match `not run`.
head_body="$(awk 'NR>1 && /^---$/ {on=1; next} on' "$HEAD")"
head_plain="$(tr -d '*' <<<"$head_body")"
# Floor: if the frontmatter delimiters ever change shape the extraction yields nothing, and every
# body-scoped assert below would pass or fail for the wrong reason. Pin that the body was reached.
assert "head: instruction body extracted (floor — not frontmatter-only)" \
  'grep -q "^## Required dispatch pattern" <<<"$head_body"'
assert "head: instructs by capability (subagent-launch mechanism)" \
  'grep -qiE "subagent-launch mechanism|this mode.s subagent" <<<"$head_body"'
assert "head: still forbids running inline" 'grep -qi "do not run the skill inline" <<<"$head_plain"'
assert "head: still requires foreground" 'grep -qi "foreground" <<<"$head_plain"'
# The NEGATIVE assert runs over the WHOLE file, frontmatter included — the `description:` line was
# one of only two genuine pre-existing violations in the tree, so scoping this to the body would
# leave the very place the defect lived uncovered. Same shape as the per-fragment assert below:
# indented lines are the labelled illustration and are exempt.
assert "head: instruction prose names no dispatch tool literal" \
  '! grep -qE "\b(Task|Agent)\b" <<<"$(grep -v "^    " "$HEAD")"'

# 0168 whole-branch review, IMPORTANT 3: the head's RATIONALE must stay true against the shipped
# sidecar. It used to open "Docket ships model/effort-pinned subagent wrappers in
# `.cursor/agents/docket-*.md`" and rest the whole dispatch requirement on "which defeats the pin"
# — true when every wrapper source carried a pin, false since change 0168 made the harness-indexed
# sidecar the default store and shipped cursor IDs for the three build profiles only. The head is
# catted verbatim into the generated `.cursor/rules/docket-dispatch.mdc`, so the false claim
# shipped into every cursor repo.
# The premise is DERIVED, not hard-coded: a future change that pins every cursor wrapper turns the
# `if` false and retires the guard, rather than leaving a stale assert behind.
# shellcheck source=/dev/null
. "$REPO/scripts/lib/harness-defaults.sh"
HD="$REPO/agents/harness-defaults.yml"
n_src=0
for f in "$REPO"/agents/docket-*.md; do [ -e "$f" ] || continue; n_src=$((n_src+1)); done
n_cursor_pinned="$(hd_agents "$HD" cursor | grep -c . || true)"
assert "head: sidecar population floor (>=12 sources, >=1 shipped cursor pin) — the premise below is not vacuous" \
  '[ "$n_src" -ge 12 ] && [ "$n_cursor_pinned" -ge 1 ]'
if [ "$n_cursor_pinned" -lt "$n_src" ]; then
  assert "head: makes no blanket 'ships model/effort-pinned wrappers' claim ($n_cursor_pinned of $n_src cursor wrappers carry a shipped pin)" \
    '! grep -qiE "ships model/effort-pinned" "$HEAD"'
  assert "head: says the unpinned wrappers exist" 'grep -qi "unpinned" <<<"$head_plain"'
  assert "head: requires the dispatch for a pinned and an unpinned wrapper alike" \
    'grep -qi "either way" <<<"$head_plain"'
fi

# Population derived by glob, with a floor. Twelve built-in agents ship fragments today, and the
# floor is that same 12 (raised from 9 by change 0167, which added the three docket-build profile
# agents — a floor of 9 would have tolerated deleting all three fragments): adding a thirteenth
# agent does not redden, while a vanished/renamed directory or a dropped fragment does.
frags=""; n=0
for f in "$REPO"/cursor-rules/dispatch/docket-*.md; do
  [ -e "$f" ] || continue
  frags="$frags $f"; n=$((n+1))
done
assert "fragments: population floor reached (>= 12 found)" '[ "$n" -ge 12 ]'

for f in $frags; do
  b="$(basename "$f")"
  # A fragment may SHOW a call snippet, but its INSTRUCTION line must not name a tool. The
  # instruction lines are the prose ones; the illustration is indented as a code block.
  instr="$(grep -v '^    ' "$f")"
  assert "$b: instruction prose names no dispatch tool literal" \
    '! grep -qE "\b(Task|Agent)\b" <<<"$instr"'
  assert "$b: instruction says dispatch to the subagent" \
    'grep -qiE "dispatch (to|the)" <<<"$instr"'
  # If a call snippet is present at all, it must be LABELLED as an illustration, so a reader
  # cannot mistake the name for the contract.
  if grep -qE '^    [A-Za-z]+\(' "$f"; then
    assert "$b: call snippet is labelled an illustration" 'grep -qi "illustration" "$f"'
  fi
done

echo "---"; [ "$fail" = "0" ] && echo "ALL PASS" || echo "FAILURES"; exit $fail
