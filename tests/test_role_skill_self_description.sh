#!/usr/bin/env bash
# tests/test_role_skill_self_description.sh — change 0194. A docket-owned role skill body names its
# role and its `skills.<role>` binding key; it never states whether that binding is the shipped
# default, and never positions itself as an "alternative" to another role skill. Defaults are owned
# by the docket-convention *Skill layer* role table and by README.md.
#
# NEGATIVE guard, deliberately limited — named here so a later reader does not over-trust it:
#   * LINE-SCOPED. A default claim split across two lines escapes it.
#   * VOCABULARY-SCOPED. A claim phrased without `alternative|default|instead of|opt-in` escapes it.
# The job is to catch recurrence of the exact construct change 0194 removed, not to prove the
# absence of every paraphrase. The non-vacuity block below is what keeps it from going quietly
# green if the pattern is ever broken.
# Run: bash tests/test_role_skill_self_description.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

# The forbidden shape: a `superpowers:` reference co-occurring on one line with a
# default/alternative word. Kept in one place so the guard and its own non-vacuity probe below
# cannot drift apart.
CLAIM='superpowers:'
WORDS='alternative|default|instead of|opt-in'

# grep -c under `set -o pipefail` with no early-exiting consumer: capture, never pipe into head/-q.
claim_hits(){ grep -inE "$CLAIM" "$1" 2>/dev/null | grep -icE "$WORDS" || true; }

ROLE_SKILLS="docket-build docket-review docket-brainstorm"

for s in $ROLE_SKILLS; do
  f="$REPO/skills/$s/SKILL.md"
  # Non-vacuity anchor #1: the file the guard reads must exist and be non-empty, or every
  # absence assert below passes for reasons that have nothing to do with the property.
  assert "role skill exists and is non-empty: skills/$s/SKILL.md" '[ -s "$f" ]'
  [ -s "$f" ] || continue
  # Non-vacuity anchor #2: a live PRESENCE assert through the same file read. If the path is
  # wrong or the file is renamed, this reddens instead of the absence asserts going green.
  assert "skills/$s/SKILL.md names its own skill" 'grep -qF -- "$s" "$f"'
  n="$(claim_hits "$f")"
  assert "skills/$s/SKILL.md asserts no default status (found $n such lines)" '[ "$n" -eq 0 ]'
done

if [ "$fail" != 0 ]; then
  echo "REMEDY: a docket-owned role skill body names its role and its skills.<role> binding key,"
  echo "        never whether that binding is the shipped default. Delete the claim. Which skill a"
  echo "        role resolves to by default is owned by the docket-convention *Skill layer* role"
  echo "        table and by README.md — state it there, not here."
fi

# Non-vacuity anchor #3 (mutation-in-fixture): the matcher must actually FIRE on the shape it
# claims to reject. A typo in CLAIM or WORDS would otherwise make every assert above permanently
# green. This is the inversion mirrored-guard-enforces-its-own-property warns about.
probe="$(mktemp)"
printf '%s\n' 'The lean alternative to `superpowers:subagent-driven-development`.' > "$probe"
pn="$(claim_hits "$probe")"
assert "the matcher fires on a synthetic forbidden line (got $pn)" '[ "$pn" -eq 1 ]'
# And it must NOT fire on a conforming line that merely mentions another skill operationally.
printf '%s\n' 'Do NOT continue to `superpowers:writing-plans`; stop at the executed plan.' > "$probe"
cn="$(claim_hits "$probe")"
assert "the matcher ignores a bare operational reference (got $cn)" '[ "$cn" -eq 0 ]'
rm -f "$probe"

# The rule has a single home. The remedy above sends readers to the convention's *Skill layer*;
# assert it is really stated there, so the remedy can never become a pointer to nothing.
CONV="$REPO/skills/docket-convention/SKILL.md"
assert "convention exists and is non-empty" '[ -s "$CONV" ]'
assert "convention *Skill layer* owns the role-self-description rule" \
  'grep -qiE "role skill (body )?(self-)?description" "$CONV" && grep -qF -- "skills.<role>" "$CONV"'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
