#!/usr/bin/env bash
# tests/test_docket_review.sh — guards change 0170's review role: the docket-review skill
# contract, the three rung wrappers, the build-evidence chain, and finalize's conditional skip.
# Run: bash tests/test_docket_review.sh
set -u

REPO="$(cd "$(dirname "$0")/.." && pwd)"
# The shipped-harness population and the sidecar readers. Every harness loop below derives its
# population from $HD_SHIPPED_HARNESSES rather than naming harnesses, so a newly shipped harness
# arms these guards for free (repo AGENTS.md: never hand-list the sites of a literal).
# shellcheck source=/dev/null
. "$REPO/scripts/lib/harness-defaults.sh"
fails=0
assert(){ if eval "$2"; then echo "ok   - $1"; else echo "FAIL - $1"; fails=$((fails+1)); fi; }

REV="$REPO/skills/docket-review/SKILL.md"

# --- the skill exists and declares itself -------------------------------------------------
assert "docket-review skill exists" '[ -f "$REV" ]'
assert "docket-review frontmatter name is docket-review" \
  'awk "/^---$/{n++; next} n==1" "$REV" | grep -qE "^name: docket-review$"'

# --- read-only conduct: the properties that make the verdict trustworthy -------------------
# Each is a distinct promise; a single "read-only" mention would not prove any of them.
assert "conduct: forbids running the test suite" \
  'grep -qiE "never runs? the (full )?(test )?suite" "$REV"'
assert "conduct: forbids writing, committing, or checking out" \
  'grep -qiE "never (writes|commits|checks out)" "$REV"'
assert "conduct: forbids dispatching subagents" \
  'grep -qiE "never dispatches" "$REV"'
assert "conduct: no reviewer escalation ladder" \
  'grep -qiE "never re-dispatches itself|no .{0,20}escalation" "$REV"'

# --- the finding schema: every field a triaging controller must be able to read ------------
for f in severity location summary rationale suggested_fix; do
  assert "finding schema names the '$f' field" 'grep -qF -- "$f" "$REV"'
done
for s in blocker important minor; do
  assert "finding schema names the '$s' severity" 'grep -qE "\`$s\`|\*\*$s\*\*" "$REV"'
done

# --- the evidence backstop finding --------------------------------------------------------
# The reviewer's ONLY answer to bad evidence is a finding; it must never run the suite itself.
assert "reviewer reports unverified-build-state rather than running the suite" \
  'grep -qF -- "unverified-build-state" "$REV"'
assert "reviewer verifies the evidence head_sha against the branch HEAD" \
  'grep -qF -- "head_sha" "$REV"'

# --- the three rung wrappers ---------------------------------------------------------------
HD="$REPO/agents/harness-defaults.yml"
REV_DESC="$(awk "/^---$/{n++; next} n==1" "$REV" | sed -n 's/^description: //p')"
assert "the skill's description is non-empty (anchor for the wrapper compare)" '[ -n "$REV_DESC" ]'

for rung in lean standard deep; do
  W="$REPO/agents/docket-review-$rung.md"
  assert "wrapper exists: docket-review-$rung" '[ -f "$W" ]'
  [ -f "$W" ] || continue
  assert "docket-review-$rung: name matches its filename" \
    'grep -qE "^name: docket-review-'"$rung"'$" "$W"'
  # Byte-equality with the skill's own description is the house rule for wrappers.
  assert "docket-review-$rung: description matches the skill's" \
    'wd="$(sed -n "s/^description: //p" "$W")"; [ "$wd" = "$REV_DESC" ]'
  # The wrapper wraps the review skill ONLY — no docket-convention, mirroring the build workers.
  assert "docket-review-$rung: injects docket-review" 'grep -qF -- "skills: [docket-review]" "$W"'
  assert "docket-review-$rung: does NOT inject docket-convention" \
    '! grep -qF -- "docket-convention" "$W"'
  assert "docket-review-$rung: carries the abort-and-report posture" \
    'grep -qF -- "abort-and-report" "$W"'
  # No pins live in wrapper files since change 0168 — they live in the sidecar.
  assert "docket-review-$rung: carries no model/effort pin" \
    '! grep -qE "^(model|effort):" "$W"'
  # Every shipped harness must supply a pair, or generation fails outright. Read per harness
  # through hd_field, so the assert really is about THIS harness's block and not about the file
  # containing the row somewhere.
  n_pair=0
  for h in $HD_SHIPPED_HARNESSES; do
    assert "harness-defaults: $h supplies a pair for review-$rung" \
      '[ -n "$(hd_field "$HD" '"$h"' review-'"$rung"' model)" ] &&
       [ -n "$(hd_field "$HD" '"$h"' review-'"$rung"' effort)" ]'
    n_pair=$((n_pair+1))
  done
  # Floor: a failed source would leave $HD_SHIPPED_HARNESSES empty and the loop above vacuous.
  assert "the review-$rung pair was checked on every shipped harness" '[ "$n_pair" -ge 4 ]'
  F="$REPO/cursor-rules/dispatch/docket-review-$rung.md"
  assert "cursor dispatch fragment exists: docket-review-$rung" '[ -f "$F" ]'
done

# The cap-rung invariant, asserted per harness rather than asserted in prose: review-deep is
# pinned exactly where build-max is, so the cap rung never reviews below the strength the
# riskiest build work was built with.
# Read through hd_field rather than slicing the block with an awk regex: hd_field is the sidecar's
# own reader, so the pair is compared exactly as sync-agents.sh would resolve it, and no harness
# name whose spelling the slicer's boundary pattern fails to match can slip past.
n_cap=0
for h in $HD_SHIPPED_HARNESSES; do
  assert "$h: the review-deep pin equals the build-max pin" \
    'd="$(hd_field "$HD" '"$h"' review-deep model)/$(hd_field "$HD" '"$h"' review-deep effort)";
     m="$(hd_field "$HD" '"$h"' build-max model)/$(hd_field "$HD" '"$h"' build-max effort)";
     [ "$d" != "/" ] && [ "$d" = "$m" ]'
  n_cap=$((n_cap+1))
done
assert "the cap-rung invariant was checked on every shipped harness" '[ "$n_cap" -ge 4 ]'

# The rung wrappers must NOT introduce a new dispatch site into test_dispatch_capability.sh's
# reverse-correspondence population. The four docket-build-* workers set the precedent: they are
# referred to as profile agents, never in the `name`-near-"subagent" shape that guard derives on.
assert "rung dispatch prose avoids the derived-dispatch-site shape" \
  '! grep -rohE --include="*.md" "\`docket-review-[a-z]+\`[^\`]{0,20}subagent" "$REPO/skills/" | grep -q .'

# --- the build-evidence chain: producer ----------------------------------------------------
# Per the learnings finding `specified-but-unreachable`: a contract with a producer and a consumer
# needs at least one assert anchored on the paragraph that PERFORMS the write, not only on the
# section that defines what the write means. The producer is docket-build's gate.
BUILD="$REPO/skills/docket-build/SKILL.md"
assert "producer: docket-build names the build-evidence record" \
  'grep -qF -- "build-evidence" "$BUILD"'
assert "producer: the evidence markers are defined where the gate emits them" \
  'grep -qF -- "docket:build-evidence:start" "$BUILD"'
for f in command result head_sha ran_at; do
  assert "producer: the evidence record carries the '$f' field" 'grep -qF -- "$f" "$BUILD"'
done
# The emission must be attached to the GREEN path of the gate, never to the section that merely
# describes the record: scope the search to the gate section's own text.
gate_sec="$(awk "/^## The build gate/{f=1;next} /^## /{f=0} f" "$BUILD")"
assert "producer: the gate section itself emits the evidence (not just a definition elsewhere)" \
  'grep -qF -- "build-evidence" <<<"$gate_sec"'
assert "producer: a red suite mints no evidence record" \
  'grep -qiE "red .{0,60}(never|no) .{0,30}evidence|evidence .{0,40}only .{0,20}green" "$BUILD"'

# --- the build-evidence chain: controller (consumer #1) ------------------------------------
IMPL="$REPO/skills/docket-implement-next/SKILL.md"
step6="$(awk "/^### Step 6 — Review/{f=1;next} /^### Step 6.5/{f=0} f" "$IMPL")"
assert "controller: Step 6 was located (non-vacuity anchor)" '[ -n "$step6" ]'
assert "controller: Step 6 validates the evidence before dispatching review" \
  'grep -qF -- "build-evidence" <<<"$step6"'
assert "controller: uncertified evidence re-runs the gate rather than reviewing blind" \
  'grep -qiE "re-run.{0,40}(gate|suite)" <<<"$step6"'
# Subshell, deliberately: `assert` runs its expression through `eval` in the CURRENT shell, so a
# bare `exit 1` inside the loop would terminate the whole test run at the first missing rung
# instead of recording one FAIL and continuing — every later assert would go unreported (observed
# while watching this section fail). The `( … )` keeps the loop's semantics and confines the exit.
assert "controller: names all three reviewer rungs" \
  '( for r in lean standard deep; do grep -qF -- "docket-review-$r" <<<"$step6" || exit 1; done )'
assert "controller: rung selection is deterministic, from the build's highest profile" \
  'grep -qiE "highest .{0,40}profile" <<<"$step6"'
assert "controller: blockers route through the docket-build-task contract" \
  'grep -qF -- "docket-build-task" <<<"$step6"'
assert "controller: important/minor findings go to the PR body, never auto-fixed" \
  'grep -qE "important" <<<"$step6" && grep -qiE "PR body" <<<"$step6"'
assert "controller: no re-review round after fixes" \
  'grep -qiE "no re-review|never re-review" <<<"$step6"'
# The bare `|halt` alternation this assert once carried made it vacuous: Step 6 already said
# "authorized-or-halt" before the triage prose existed, so it passed on prose it did not guard.
# Keyed on the proximity shape instead — the committed "A red re-run **halts**" satisfies it.
assert "controller: a red re-run halts" 'grep -qiE "red .{0,40}halt" <<<"$step6"'

step7="$(awk "/^### Step 7 — PR/{f=1;next} /^### Terminal disposition/{f=0} f" "$IMPL")"
assert "controller: Step 7 was located (non-vacuity anchor)" '[ -n "$step7" ]'
assert "controller: Step 7 writes the evidence block into the PR body" \
  'grep -qF -- "docket:build-evidence:start" <<<"$step7"'

# The Tier C review site must survive untouched — this change adds rungs, not a new posture.
assert "controller: the review role keeps its Tier C dispatch paragraph" \
  'grep -qF -- "resolved review skill" "$IMPL"'

# --- the build-evidence chain: finalize (consumer #2) --------------------------------------
FIN="$REPO/skills/docket-finalize-change/SKILL.md"
assert "finalize: reads the PR body's build-evidence block" \
  'grep -qF -- "build-evidence" "$FIN"'
# All three skip conditions must be stated; any one missing turns "fails toward running" into
# "fails toward merging an untested branch".
assert "finalize: skip requires a no-op rebase" \
  'grep -qiE "no-op rebase|rebase was a no-op" "$FIN"'
assert "finalize: skip requires result green" 'grep -qF -- "result: green" "$FIN"'
assert "finalize: skip requires the head_sha to match the branch HEAD" \
  'grep -qF -- "head_sha" "$FIN"'
# The posture is the safety property, not a nicety.
assert "finalize: any doubt runs the suite (fails toward running)" \
  'grep -qiE "fails? toward running|any doubt .{0,40}runs" "$FIN"'
assert "finalize: a skip is logged so the decision is auditable" \
  'grep -qiE "log.{0,60}skip|skip .{0,40}logged" "$FIN"'
assert "finalize: only the local gate path is affected" \
  'grep -qE "\`ci\`.{0,60}untouched|untouched.{0,60}\`ci\`" "$FIN"'
# The skip must NOT live inside the executable fragment, which the suite runs verbatim.
frag="$(awk "/configured-bash-finalize:start/{f=1;next} /configured-bash-finalize:end/{f=0} f" "$FIN")"
# Non-vacuity anchor, deliberately paired with the purity assert below: an awk range over a renamed
# or deleted marker yields an EMPTY haystack, and a negated grep over nothing is permanently green.
# Anchoring on the fragment's own control variable proves the extraction found the real fragment.
assert "finalize: the executable fragment was located (non-vacuity anchor)" \
  '[ -n "$frag" ] && grep -qF -- "FINALIZE_TEST_COMMAND" <<<"$frag"'
assert "finalize: the executable bash fragment is untouched by the skip logic" \
  '! grep -qiE "evidence|skip|head_sha" <<<"$frag"'

# --- documentation + the dogfood binding ---------------------------------------------------
RM="$REPO/README.md"
assert "README documents the docket-review role" 'grep -qF -- "docket-review" "$RM"'
assert "README explains why the suite lives in the build gate, not the reviewer" \
  'grep -qiE "build gate" "$RM" && grep -qF -- "build-evidence" "$RM"'
# The plan's assert here was `grep -qiE "once|one run" "$RM"` over the WHOLE README — vacuous
# against a 900-line file that says "once" for a dozen unrelated reasons. Scope it to the new
# section instead, with a non-vacuity anchor so a renamed heading reddens rather than passing on
# an empty haystack.
rvsec="$(awk "/^### docket-review/{f=1;next} /^### /{f=0} f" "$RM")"
assert "README: the docket-review section was located (non-vacuity anchor)" \
  '[ -n "$rvsec" ] && grep -qF -- "build-evidence" <<<"$rvsec"'
assert "README states the suite-run count the change delivers" \
  'grep -qiE "one full-suite run when" <<<"$rvsec" && grep -qiE "three only when both" <<<"$rvsec"'
# The shipped cross-harness default is now docket-review (change 0193). Anchored on the resolver,
# both directions, mirroring the build guard in tests/test_docket_build.sh.
review_default="$(grep -E 'SKILL_REVIEW=|skill_role review' "$REPO/scripts/docket-config.sh")"
assert "resolver's review default line was located (non-vacuity anchor)" '[ -n "$review_default" ]'
assert "shipped skills.review default is docket-review" \
  'grep -qF -- "docket-review" <<<"$review_default"'
assert "shipped skills.review default is no longer superpowers review" \
  '! grep -qF -- "superpowers:requesting-code-review" <<<"$review_default"'
DY="$REPO/.docket.yml"
assert "this repo dogfoods docket-review via .docket.yml" \
  'awk "/^skills:/{f=1;next} /^[a-z_]+:/{f=0} f" "$DY" | grep -qE "^ +review: +docket-review$"'
# The block guards a real property — the example config is the cross-harness default surface and
# must agree with the resolver — so invert it, both directions: a revert of the example alone
# would leave it disagreeing with the resolver, which is exactly the drift this pair exists to
# catch.
assert "the example config states the shipped docket-review default" \
  'grep -qE "^ +review: +docket-review$" "$REPO/.docket.example.yml"'
assert "the example config no longer ships the superpowers review default" \
  '! grep -qE "^ +review: +superpowers:requesting-code-review$" "$REPO/.docket.example.yml"'

echo "---"; [ "$fails" -eq 0 ] && echo "PASS" || { echo "FAIL ($fails)"; exit 1; }
