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
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

flat(){ tr '\n' ' ' < "$1" | tr -s '[:space:]' ' '; }

# A literal backtick, held in a SINGLE-quoted literal. The token patterns below need it next to a
# `$tok` expansion, and no backtick may sit inside double quotes in test source: bare, the shell
# runs it when it reads the line; backslash-escaped, the escape is consumed there and a bare
# backtick travels on to the next evaluation (change 0221,
# scripts/check-test-source-hygiene.sh). Concatenating this expansion is inert either way — an
# expansion's result is never re-scanned for command substitution.
BT='`'

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
# The membership test has ONE home. Every skill body conditions on `DUMMY_MODE_ENABLED` alone, so
# if the shared definition does not state "only when the surface is in DUMMY_MODE_SURFACES" as an
# instruction, the knob resolves and exports and is then honored by nobody. Bound to one sentence so
# a description of the map elsewhere in the section cannot satisfy it. Backticks reach grep from a
# variable, per the idiom the reference asserts use below.
dm_pat='[Aa]pply[^.]{0,200}in `DUMMY_MODE_SURFACES`'
assert "convention: applying a surface is gated on DUMMY_MODE_SURFACES membership" \
  'grep -qE "$dm_pat" <<<"$conv_section"'
dm_pat='`DUMMY_MODE_SURFACES`[^.]{0,200}`all` matches every'
assert "convention: the literal all is stated to match every token" \
  'grep -qE "$dm_pat" <<<"$conv_section"'

assert "convention: the three exports are named" \
  'grep -qF "DUMMY_MODE_ENABLED" "$CONV" && grep -qF "DUMMY_MODE_PERSONA" "$CONV" && grep -qF "DUMMY_MODE_SURFACES" "$CONV"'

assert "reference: the file exists" '[ -f "$REF" ]'
ref_flat=""
[ -f "$REF" ] && ref_flat="$(flat "$REF")"

# All five tokens, each bound to its own mode — adjacency to the word "replace" somewhere in a
# table is satisfied by a table that lists both modes for everything. The pattern is built in a
# variable so the backticks reach grep as literals: an expansion's result is not re-scanned for
# command substitution, while a backtick written inside the eval'd string would be. The backtick
# itself comes from `$BT` for the same reason it is defined above.
for tok in dialogue reports; do
  dm_pat="$BT$tok$BT.{0,250}replace"
  assert "reference: $tok is classified replace" 'grep -qE "$dm_pat" <<<"$ref_flat"'
done
for tok in results change-sections pr; do
  dm_pat="$BT$tok$BT.{0,250}additive"
  assert "reference: $tok is classified additive" 'grep -qE "$dm_pat" <<<"$ref_flat"'
done

assert "reference: additive blocks are authored with their parent, not retro-added" \
  'grep -qE "In plain terms[^.]{0,250}same (commit|moment)" <<<"$ref_flat"'
assert "reference: ad-hoc enablement is session-scoped and writes nothing" \
  'grep -qE "session[^.]{0,250}(writes nothing|no writes)" <<<"$ref_flat"'
assert "reference: agent-facing artifacts are named as never eligible" \
  'grep -qE "[Nn]ever eligible[^.]{0,200}plans" <<<"$ref_flat"'

# --- skill-body pointers (change 0276) -----------------------------------------------------------
# Each eligible skill body carries ONE pointer naming the surfaces IT owns and CITING the shared
# definition rather than restating it. A bare "the file mentions dummy mode" would survive a
# pointer copy-pasted into the wrong skill, so each token is bound to the phrase "dummy mode".
#
# BOUNDS. Every gap is a WITHIN-SENTENCE `[^.]{0,200}` — under the 255-char repetition ceiling BSD
# grep enforces (tests/test_grep_portability.sh), and scoped tightly enough that two unrelated
# sentences cannot satisfy a binding between them. The two directions of the token binding are two
# SEPARATE greps rather than one alternation: a single ERE carrying two bounded repeats is a
# catastrophic-backtracking shape, and nothing is gained by fusing them.
check_pointer(){ # check_pointer <skill-relpath> <token>...
  local rel="$1"; shift
  local f="$REPO/$rel" body tok dm_def dm_fwd dm_rev
  assert "pointer: $rel exists" '[ -f "$f" ]'
  [ -f "$f" ] || return 0
  body="$(flat "$f")"
  dm_def="[Dd]ummy mode[^.]{0,200}shared definition"
  assert "pointer: $rel names the shared definition" 'grep -qE "$dm_def" <<<"$body"'
  for tok in "$@"; do
    # Same expansion idiom as the reference asserts above: the backticks must reach grep from a
    # variable, since a backtick written inside the eval'd string would be a command substitution.
    dm_fwd="[Dd]ummy mode[^.]{0,200}$BT$tok$BT"
    dm_rev="$BT$tok$BT[^.]{0,200}[Dd]ummy mode"
    assert "pointer: $rel binds the $tok surface to dummy mode" \
      'grep -qE "$dm_fwd" <<<"$body" || grep -qE "$dm_rev" <<<"$body"'
  done
}
check_pointer skills/docket-new-change/SKILL.md      dialogue
check_pointer skills/docket-groom-next/SKILL.md      dialogue
check_pointer skills/docket-implement-next/SKILL.md  pr reports change-sections results
# RETIRED (0316, category (a)): the finalize Go sequencer no longer carries a dummy-mode
# surface-binding pointer (dialogue / reports / change-sections). `dummy_mode.enabled` is
# `dispDeferred` in internal/config/schema.go — enabling it BLOCKS all mutation, so this skill never
# runs with it on and cannot exercise any surface. Commit 8c74c1c8 deflated the paragraph to state
# exactly that deferred status. Authority #2 (dispDeferred: dummy_mode.enabled blocks mutation) +
# Authority #3 (the skill's positive "Dummy mode is a deferred capability … rejected at the config
# gate"). Guard re-pointed at the deflation: finalize states dummy mode is deferred and binds no
# surface. (The token-cap and DUMMY_MODE_SURFACES no-restatement asserts below still cover finalize.)
FIN_DM="$REPO/skills/docket-finalize-change/SKILL.md"
fin_dm_flat="$(flat "$FIN_DM")"
assert "finalize states dummy mode is a deferred capability (deflated, not a surface pointer)" \
  'grep -qiE "[Dd]ummy mode.{0,6}is a.{0,12}deferred capability" <<<"$fin_dm_flat"'
assert "finalize states dummy_mode.enabled is rejected at the config gate" \
  'grep -qiE "rejected at the config gate" <<<"$fin_dm_flat"'
assert "finalize binds no dummy-mode surface (deferred, cannot be exercised)" \
  '! grep -qiE "[Dd]ummy mode[^.]{0,200}.(dialogue|reports|change-sections). surface" <<<"$fin_dm_flat"'
check_pointer skills/docket-status/SKILL.md          reports
check_pointer skills/docket-auto-groom/SKILL.md      reports change-sections

# The reverse direction: no skill body RESTATES the token table, which is the restatement class
# change 0154 exists to stop. A body that lists more tokens than it OWNS has copied the table.
#
# The cap is per-skill because ownership is: docket-implement-next authors four of the five
# surfaces itself (`reports`, `pr`, `change-sections` — `## Run halted` — and the Step-6.5
# `results` artifact), so a 3-cap would forbid it from naming a surface it actually writes rather
# than forbidding a copied table. Its cap is 4; every other body keeps 3. Either way a body that
# copied the table names all FIVE and still reddens.
for rel in skills/docket-new-change/SKILL.md skills/docket-groom-next/SKILL.md \
           skills/docket-implement-next/SKILL.md skills/docket-finalize-change/SKILL.md \
           skills/docket-status/SKILL.md skills/docket-auto-groom/SKILL.md; do
  n=0
  for tok in dialogue reports results change-sections pr; do
    grep -qF -- "$BT$tok$BT" "$REPO/$rel" && n=$((n+1))
  done
  cap=3
  [ "$rel" = "skills/docket-implement-next/SKILL.md" ] && cap=4
  assert "no restatement: $rel names at most $cap surface tokens (got $n)" '[ "$n" -le "$cap" ]'
  # ...and none of them restates the membership test either. A pointer that spells
  # `DUMMY_MODE_SURFACES` has taken a second copy of a rule the shared definition owns — the copy
  # that five of the six bodies would then be missing, which is how the knob came to be honored by
  # exactly one reader.
  assert "no restatement: $rel leaves the DUMMY_MODE_SURFACES test to the shared definition" \
    '! grep -qF -- "DUMMY_MODE_SURFACES" "$REPO/$rel"'
done

exit $fail
