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
assert "head: instructs by capability (subagent-launch mechanism)" \
  'grep -qiE "subagent-launch mechanism|this mode.s subagent" "$HEAD"'
assert "head: still forbids running inline" 'grep -qiE "not? run the skill inline|never run.*inline" "$HEAD"'
assert "head: still requires foreground" 'grep -qi "foreground" "$HEAD"'

# Population derived by glob, with a floor. Nine built-in agents ship fragments today; the floor
# is >= 9 so adding a tenth agent does not redden, while a vanished/renamed directory does.
frags=""; n=0
for f in "$REPO"/cursor-rules/dispatch/docket-*.md; do
  [ -e "$f" ] || continue
  frags="$frags $f"; n=$((n+1))
done
assert "fragments: population floor reached (>= 9 found)" '[ "$n" -ge 9 ]'

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
