#!/usr/bin/env bash
# tests/test_critic_return_channel.sh — change 0281. The docket-auto-groom-critic verdict travels
# on exactly ONE channel: the critic's final report, read by the groom as the dispatch's return
# while it actively blocks. This guard binds the three prose contracts that make that true:
#
#   (a) agents/docket-auto-groom-critic.md — the DELIVERY half: the verdict IS the final report,
#       that return is the ONLY channel it travels on, the dispatcher is BLOCKING on it, and the
#       critic never addresses its dispatcher by name or via an agent-listing surface.
#   (b) skills/docket-auto-groom/SKILL.md Step 3 — the RECEIVING half, plus the bounded
#       no-verdict posture that terminates in the Tier B abstain.
#   (c) skills/docket-convention/SKILL.md *Composition* — the critic dispatch sits in the
#       in-context-return family, NOT inside the git-state-contract clause.
#
# Deliberate limits, named so a later reader does not over-trust this file:
#   * PHRASE-SCOPED. A contract reworded past these anchors escapes it. The anchors are verbatim
#     quoted clauses (ADR-0054), so drift is at least mechanically visible.
#   * The (c) assert is an ORDERING fact over one paragraph, not a parse of English antecedents.
# Sentinels are sampling, not parsing (learnings: foundational-test-discipline) — pair with the
# whole-branch review.
# Run: bash tests/test_critic_return_channel.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# Whitespace-collapse: these contracts are hard-wrapped prose, so every proximity match below runs
# against a flattened copy or it would fail on a line break that means nothing.
flat(){ tr -s '[:space:]' ' ' <<<"$1"; }

CRITIC="$REPO/agents/docket-auto-groom-critic.md"
AUTOGROOM="$REPO/skills/docket-auto-groom/SKILL.md"
CONV="$REPO/skills/docket-convention/SKILL.md"

# --- (a) the critic's delivery clause -----------------------------------------------------------
# Non-vacuity anchor: the file must exist and be non-empty, or every assert below passes for
# reasons unrelated to the property.
assert "critic source exists and is non-empty" '[ -s "$CRITIC" ]'
# Non-vacuity anchor: a live PRESENCE assert through the same read, so a rename reddens here
# rather than silently greening the rest.
assert "critic source is the adversarial critic contract" 'grep -qi "adversarial critic" "$CRITIC"'

critic_flat="$(flat "$(cat "$CRITIC")")"

# Slice the UNCONDITIONAL delivery paragraph — the one led by the bolded contract — and match the
# three clauses below inside it alone. Whole-file matching does not bind this paragraph: the next
# paragraph restates the delivery clause CONDITIONALLY ("If you come to believe the return channel
# itself is unavailable … write the verdict as your final report"), so a flattened-file proximity
# match survives deleting the unconditional contract outright. The slice runs from the bolded lead
# to the first blank line; deleting the paragraph empties it and reddens the anchor.
delivery="$(awk '/\*\*Your verdict is your final report\*\*/{f=1} f&&/^[[:space:]]*$/{exit} f{print}' "$CRITIC")"
assert "critic: the unconditional delivery paragraph is present (slice anchor holds)" \
  '[ -n "$delivery" ]'
delivery_flat="$(flat "$delivery")"

# The verdict is bound to the final report. Bounded gap ([^.]{0,80}) keeps the match inside one
# sentence, so a stray "verdict" and a stray "final report" in different clauses cannot satisfy it.
assert "critic: the verdict IS the critic's final report" \
  'grep -qE "verdict[^.]{0,80}final report|final report[^.]{0,80}verdict" <<<"$delivery_flat"'
# EXCLUSIVITY — the thesis half that nothing else binds: that return is the ONLY channel. Without
# it a critic is free to read the contract as one delivery route among several and invent a second.
assert "critic: that return is the ONLY channel the verdict travels on" \
  'grep -qE "only channel[^.]{0,60}verdict|verdict[^.]{0,60}only channel" <<<"$delivery_flat"'
# The dispatcher is BLOCKING — the reason the exclusivity matters and the reason silence is not a
# neutral outcome. Keyed on the two words in proximity, not the sentence's current phrasing.
assert "critic: the dispatcher is blocking on that return" \
  'grep -qE "dispatcher[^.]{0,40}blocking|blocking[^.]{0,40}dispatcher" <<<"$delivery_flat"'

# The never-address-your-dispatcher clause, pinned on its two load-bearing halves: the prohibition
# itself, and the REASON (which is what stops a critic from inventing a workaround channel).
assert "critic: never message, address, or resolve the dispatcher" \
  'grep -qE "[Nn]ever[^.]{0,120}(message|address|resolve)[^.]{0,120}dispatcher" <<<"$critic_flat"'
assert "critic: states why no such address resolves" \
  'grep -qF -- "not registered under its skill name" <<<"$critic_flat"'
# The belief-changes-nothing clause: a critic that concludes the channel is broken must still write
# the verdict as its final report. Without this, a critic reasons its way into silence.
assert "critic: a believed-unavailable channel changes nothing about what it does" \
  'grep -qE "believe[^.]{0,160}changes nothing" <<<"$critic_flat"'

# --- (b) the groom's receiving half + bounded no-verdict posture ---------------------------------
assert "auto-groom skill exists and is non-empty" '[ -s "$AUTOGROOM" ]'
assert "auto-groom has a Step 3 critic pass" 'grep -qF -- "### Step 3 — Critic pass" "$AUTOGROOM"'

# Scope every match below to Step 3 alone: awk from the Step 3 heading to the next "### " heading,
# then whitespace-collapse. A match anywhere else in the file must NOT satisfy these.
step3="$(awk '/^### Step 3 — Critic pass/{f=1;next} f&&/^### /{exit} f{print}' "$AUTOGROOM")"
assert "Step 3 section is non-empty (slice anchor holds)" '[ -n "$step3" ]'
step3_flat="$(flat "$step3")"

# The receiving half: the verdict comes from the critic's RETURN, and out-of-band delivery is
# refused. Two separate asserts — the positive channel and the prohibition are separately
# deletable, so binding them in one match would let either deletion pass.
assert "Step 3: the verdict is read from the critic's return" \
  'grep -qE "verdict is read from[^.]{0,60}return" <<<"$step3_flat"'
assert "Step 3: the groom never waits for out-of-band delivery" \
  'grep -qE "never waits for[^.]{0,120}out-of-band" <<<"$step3_flat"'
# The prohibition stated POSITIVELY. "foreground" alone is a property of the dispatch; this is the
# thing an agent must not do, and the no-verdict posture below deliberately treats a backgrounded
# child as a recoverable input — so without this clause the file's only don't-background-it is a
# single adjective the posture then reads as tolerable.
assert "Step 3: the groom never backgrounds the critic" \
  'grep -qE "never backgrounds the critic" <<<"$step3_flat"'

# The bounded posture, pinned on each of its three bounds. All three are load-bearing: drop the
# collect and a transient plumbing fault junks a sound draft; drop the re-dispatch likewise; drop
# the third-dispatch ban and termination is no longer provable.
assert "Step 3: exactly one collect attempt" \
  'grep -qE "one collect attempt" <<<"$step3_flat"'
assert "Step 3: exactly one fresh foreground re-dispatch" \
  'grep -qE "one fresh foreground re-dispatch" <<<"$step3_flat"'
# ...and that leg must say what makes the SECOND dispatch block when the first did not, plus what to
# do when nothing does. Without both halves the retry is a verbatim repeat of the failing leg on the
# exact harness this posture exists for, and the collect attempt is the only real recovery. One
# assert, because deleting EITHER half breaks the ordered match. Bounded, period-free gaps keep it
# inside the one clause — Step 3's earlier "no dispatch mechanism resolves … **Tier B**" sentence
# would otherwise satisfy a looser mechanism→Tier B pairing (it carries no "block").
assert "Step 3: the re-dispatch names its blocking mechanism, and skips the leg without one" \
  'grep -qE "mechanism[^.]{0,40}block[^.]{0,140}Tier B" <<<"$step3_flat"'
assert "Step 3: never a third dispatch" \
  'grep -qiE "[Nn]ever a third dispatch" <<<"$step3_flat"'
assert "Step 3: never an indefinite wait" \
  'grep -qiE "never an indefinite wait" <<<"$step3_flat"'

# The mapping the whole posture exists to supply (learnings: prohibition-needs-a-return-value):
# no verdict maps to the Tier B abstain, which is a value the exit vocabulary already has.
# Bounded gap keeps antecedent and consequent inside one sentence.
assert "Step 3: no verdict maps to the Tier B abstain" \
  'grep -qE "[Ss]till no verdict[^.]{0,200}Tier B" <<<"$step3_flat"'
assert "Step 3: the Tier B outcome is the abstain exit" \
  'grep -qE "Tier B[^.]{0,60}abstain" <<<"$step3_flat"'

# WHICH abstain (change 0281 finding 2). The routed exit is the WHOLE Abstain exit, `auto_groomable`
# flip included — not a diagnostic-only variant that leaves the flag armed. The flip is what the
# skill's *Termination & concurrency* proof spends: a stub left armed is still autonomous-eligible,
# so the drain re-selects it and the no-verdict route becomes a spin. Bounded, period-free gap keeps
# the flip inside the clause that names the exit, so a stray `auto_groomable` elsewhere in Step 3
# cannot satisfy it.
assert "Step 3: the no-verdict abstain is the FULL exit, auto_groomable flip included" \
  'grep -qE "Abstain\*\* exit[^.]{0,100}auto_groomable" <<<"$step3_flat"'

# ...and the exit it routes to must legitimately COVER that route. Exit 3's precondition read "any
# needs-human-context verdict", which a no-verdict return definitionally is NOT — a route landing on
# an exit whose own precondition excludes it is an invitation to improvise a fourth exit.
# Sliced on the numbered list item alone: Step 3 also says "**Abstain**" AND says "no verdict", so a
# whole-file (or bare-marker) match would be satisfied by Step 3's own prose and never read exit 3.
abstain_exit="$(grep -E -- '^3\. \*\*Abstain\*\*' "$AUTOGROOM")"
assert "Step 4 exit 3 located (slice anchor holds)" '[ -n "$abstain_exit" ]'
assert "Step 4 exit 3 admits a no-verdict return, not verdicts alone" \
  'grep -qiE "no.verdict" <<<"$(flat "$abstain_exit")"'

# Regression anchor: the pre-existing never-yield qualifier on the SECOND critic round must
# survive this edit (it is what tests/test_composition_wiring.sh binds).
assert "Step 3: the critic re-check is still dispatched foreground" \
  'grep -qi "re-check is dispatched foreground" <<<"$step3_flat"'

# --- (c) the convention reclassifies the critic dispatch ----------------------------------------
assert "convention exists and is non-empty" '[ -s "$CONV" ]'

# Slice the Composition paragraph — it is one physical line beginning with the bolded marker.
comp="$(grep -F -- '**Composition (change 0017).**' "$CONV")"
assert "Composition paragraph located" '[ -n "$comp" ]'
comp_flat="$(flat "$comp")"
# Non-vacuity: the paragraph still names the critic at all (test_composition_wiring.sh also binds
# this). It is NOT the ordering check's anchor and must not be read as one: the paragraph names
# `docket-auto-groom-critic` TWICE — once in the dispatch sentence, once in the trailing
# no-skill-wrapper enumeration — so it survives deleting the dispatch sentence outright. The
# ordering check carries its own anchor, the introduction needle below.
assert "Composition still names docket-auto-groom-critic" \
  'grep -qF -- "docket-auto-groom-critic" <<<"$comp_flat"'
# THE ordering anchor: the clause that INTRODUCES the critic dispatch, unique to that sentence.
# `index()` returns the FIRST occurrence, so keying the ordering on the bare agent name would let a
# deleted dispatch sentence resolve to the trailing enumeration — an offset far downstream of the
# git-state clause, and the ordering assert vacuously green over the very deletion it exists to catch.
CRITIC_INTRO='`docket-auto-groom-critic` subagent for its adversarial gate'
assert "Composition introduces the critic dispatch as the adversarial gate" \
  'grep -qF -- "$CRITIC_INTRO" <<<"$comp_flat"'
# Non-vacuity: the git-state clause is still present and still says what it says.
assert "Composition still carries the git-state-contract clause" \
  'grep -qF -- "contract is **git state**" <<<"$comp_flat"'

# THE PROPERTY. The git-state-contract clause must be CLOSED before the critic is introduced, so
# the critic can no longer be an antecedent of "These dispatches … their contract is git state".
# Expressed as a byte-offset ordering over the flattened paragraph: a mechanical fact, not a parse
# of English. `awk index()` is 1-based and returns 0 when absent — both needles are asserted
# present above, so a 0 here would be a bug in the slice, not a legitimate ordering.
offset_of(){ awk -v s="$1" 'BEGIN{ }{ print index($0, s) }' <<<"$comp_flat"; }
gs_at="$(offset_of 'contract is **git state**')"
critic_at="$(offset_of "$CRITIC_INTRO")"
assert "both offsets resolved (git-state=$gs_at critic=$critic_at)" \
  '[ "$gs_at" -gt 0 ] && [ "$critic_at" -gt 0 ]'
assert "the git-state clause closes BEFORE the critic is introduced" \
  '[ "$gs_at" -lt "$critic_at" ]'

# The positive half of the reclassification: the critic's verdict is an in-context return, and
# neither git state nor agent messaging. Bounded gaps keep each inside one sentence.
assert "Composition: the critic's verdict flows back in-context as the dispatch's return" \
  'grep -qE "in-context as the dispatch.{0,3}s return" <<<"$comp_flat"'
assert "Composition: never via git state and never via agent messaging" \
  'grep -qE "never via git state[^.]{0,60}never via agent messaging" <<<"$comp_flat"'

# Regression anchor: the never-yield rule and the caller's reciprocal reading are untouched by
# this edit. If the re-order dropped either, that is a silent contract loss.
assert "Composition: the never-yield rule survives" \
  'grep -qF -- "to await a task-notification" <<<"$comp_flat"'
assert "Composition: the never-adopt-a-child's-files rule survives" \
  'grep -qF -- "never adopts or commits a child" <<<"$comp_flat"'

# Non-vacuity anchor (mutation-in-fixture): the ordering matcher must actually FIRE on the shape it
# rejects — the pre-0281 SHAPE, where the critic dispatch sits inside the git-state clause instead
# of after it. Built from the live `$CRITIC_INTRO` needle so the probe exercises the anchor the
# ordering assert actually uses; a typo in either needle would otherwise make that assert
# permanently, vacuously green.
probe_flat="$(flat "\`docket-auto-groom\` dispatches the ${CRITIC_INTRO}; their contract is **git state** on origin/docket.")"
p_gs="$(awk -v s='contract is **git state**' '{ print index($0, s) }' <<<"$probe_flat")"
p_cr="$(awk -v s="$CRITIC_INTRO" '{ print index($0, s) }' <<<"$probe_flat")"
assert "the ordering matcher rejects the pre-0281 shape (git-state=$p_gs critic=$p_cr)" \
  '[ "$p_gs" -gt 0 ] && [ "$p_cr" -gt 0 ] && [ "$p_gs" -gt "$p_cr" ]'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
