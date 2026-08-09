#!/usr/bin/env bash
# tests/test_dummy_mode.sh — change 0276. Guards the dummy-mode prose contract: the convention owns
# a shared definition, that definition carries the agent-safety rule, and every eligible skill body
# points at it. Asserts BIND each phrase to the claim it makes (learnings:
# prose-guard-binds-phrase-to-claim) over a whitespace-COLLAPSED haystack (learnings:
# phrase-grep-over-wrapped-prose), so a re-wrap is invisible and a reworded rule is not.
#
# WINDOWS, NOT WIDE GAPS. The convention section is far longer than the 255-char ERE repetition
# ceiling BSD grep enforces (tests/test_grep_portability.sh), so "the section points at its
# reference" cannot be written as one `.{0,N}` gap without shipping an unportable pattern that
# errors out before it examines anything. The section is sliced out by its own heading instead, and
# every remaining gap is a WITHIN-SENTENCE `[^.]{0,N}` under the ceiling.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

flat(){ tr '\n' ' ' < "$1" | tr -s '[:space:]' ' '; }

CONV="$REPO/skills/docket-convention/SKILL.md"
REF="$REPO/skills/docket-convention/references/dummy-mode.md"

# The `### Dummy mode (shared definition)` heading window, whitespace-collapsed. An empty slice is
# indistinguishable from a missing section, which is exactly what the first assert below reports.
conv_section="$(awk '/^### Dummy mode \(shared definition\)/{f=1}
                     f && /^### / && ++n>1 {exit}
                     f' "$CONV" | tr '\n' ' ' | tr -s '[:space:]' ' ')"

# Anchor existence first: a window-bound assert that silently matches nothing is worse than none.
assert "convention: the shared-definition heading exists" \
  'grep -qF "### Dummy mode (shared definition)" "$CONV"'
assert "convention: the section is non-empty" '[ -n "${conv_section// /}" ]'
assert "convention: the section points at its reference file" \
  'grep -qF "references/dummy-mode.md" <<<"$conv_section"'

# The agent-safety rule is the load-bearing sentence: bind "never a decision input" to the block it
# is about, so a rewrite that keeps the words and drops the subject reddens. `[^.]` scopes the gap
# to one sentence — two sentences that happen to mention each thing separately do not satisfy it.
assert "convention: the plain-terms block is bound to 'never a decision input'" \
  'grep -qE "In plain terms[^.]{0,200}never a decision input" <<<"$conv_section"'
assert "convention: the three exports are named" \
  'grep -qF "DUMMY_MODE_ENABLED" "$CONV" && grep -qF "DUMMY_MODE_PERSONA" "$CONV" && grep -qF "DUMMY_MODE_SURFACES" "$CONV"'

assert "reference: the file exists" '[ -f "$REF" ]'
ref_flat=""
[ -f "$REF" ] && ref_flat="$(flat "$REF")"

# All five tokens, each bound to its own mode — adjacency to the word "replace" somewhere in a
# table is satisfied by a table that lists both modes for everything. The pattern is built in a
# variable so the backticks reach grep as literals: an expansion's result is not re-scanned for
# command substitution, while a backtick written inside the eval'd string would be.
for tok in dialogue reports; do
  dm_pat="\`$tok\`.{0,250}replace"
  assert "reference: $tok is classified replace" 'grep -qE "$dm_pat" <<<"$ref_flat"'
done
for tok in results change-sections pr; do
  dm_pat="\`$tok\`.{0,250}additive"
  assert "reference: $tok is classified additive" 'grep -qE "$dm_pat" <<<"$ref_flat"'
done

assert "reference: additive blocks are authored with their parent, not retro-added" \
  'grep -qE "In plain terms[^.]{0,250}same (commit|moment)" <<<"$ref_flat"'
assert "reference: ad-hoc enablement is session-scoped and writes nothing" \
  'grep -qE "session[^.]{0,250}(writes nothing|no writes)" <<<"$ref_flat"'
assert "reference: agent-facing artifacts are named as never eligible" \
  'grep -qE "[Nn]ever eligible[^.]{0,200}plans" <<<"$ref_flat"'

exit $fail
