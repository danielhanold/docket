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
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fails=$((fails+1)); fi; }

# Collapse runs of whitespace so a phrase assert survives a pure re-flow of hard-wrapped markdown
# (learnings: phrase-grep-over-wrapped-prose). Runs, not only newlines: an indented list
# continuation leaves several spaces behind, and `tr '\n' ' '` alone would not close them up.
flat(){ tr -s '[:space:]' ' ' <<<"$1"; }

REV="$REPO/skills/docket-review/SKILL.md"

# --- the skill exists and declares itself -------------------------------------------------
assert "docket-review skill exists" '[ -f "$REV" ]'
assert "docket-review frontmatter name is docket-review" \
  'awk "/^---$/{n++; next} n==1" "$REV" | grep >/dev/null -E "^name: docket-review$"'

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
  '! grep -rohE --include="*.md" "\`docket-review-[a-z]+\`[^\`]{0,20}subagent" "$REPO/skills/" | grep >/dev/null .'

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
# --- change 0342: the evidence re-mint DRIVES the native gate, never a raw launch/observe recipe ---
# Task 15 migrated Step 6's evidence re-mint (and every post-review re-gate) onto the typed gate
# DRIVER — the same `docket gate drive` contract Task 14 moved the build workers onto. The raw
# `docket gate launch`/`observe` verbs are primitives the driver composes (gate-caller-loop.md § the
# raw verbs are primitive/operator APIs); a Step-6 recipe that composes them by hand IS the
# full-budget observe loop change 0342 retired. Keyed on shape, one clause at a time: the driver
# command group must be named, and neither raw verb may appear as Step 6's workflow recipe.
# Mutation: revert the recipe to `docket gate launch`/`observe` -> the negative reddens; drop the
# `docket gate drive` naming -> the positive reddens.
assert "controller: Step 6 evidence re-mint drives the native gate (docket gate drive)" \
  'grep -qF -- "docket gate drive" <<<"$step6"'
assert "controller: Step 6 composes no raw launch/observe verb as a workflow recipe" \
  '! grep -qF -- "docket gate launch" <<<"$step6" && ! grep -qF -- "docket gate observe" <<<"$step6"'
# Subshell, deliberately: `assert` runs its expression through `eval` in the CURRENT shell, so a
# bare `exit 1` inside the loop would terminate the whole test run at the first missing rung
# instead of recording one FAIL and continuing — every later assert would go unreported (observed
# while watching this section fail). The `( … )` keeps the loop's semantics and confines the exit.
assert "controller: names all three reviewer rungs" \
  '( for r in lean standard deep; do grep -qF -- "docket-review-$r" <<<"$step6" || exit 1; done )'
assert "controller: rung selection is deterministic, from the build's highest profile" \
  'grep -qiE "highest .{0,40}profile" <<<"$step6"'
# --- change 0218: findings are FIXED in-branch, not recorded and re-minted ----
# The removed rule was "An `important` or `minor` finding is recorded in the PR body for the
# human's merge-time judgment, never auto-fixed". The assert that used to sit here confirmed the
# words "important" and "PR body" were present — both of which survive the rewrite, so it would
# have stayed green across the exact change it was meant to notice. Assert the NEGATIVE instead.
assert "controller: Step 6 no longer forbids auto-fixing non-blockers" \
  '! grep -qiE "never auto-fixed" <<<"$step6"'
assert "controller: Step 6 sends findings through a bounded in-branch fix loop" \
  'grep -qiF -- "fix loop" <<<"$step6"'
assert "controller: Step 6 points at the fix-loop reference (blocking read)" \
  'grep -qF -- "references/fix-loop.md" <<<"$step6"'
assert "controller: Step 6 names the severity threshold knob" \
  'grep -qF -- "REVIEW_MIN_FIX_SEVERITY" <<<"$step6"'
assert "controller: blockers still route through the docket-build-task contract" \
  'grep -qF -- "docket-build-task" <<<"$step6"'
assert "controller: no re-review round after fixes" \
  'grep -qiE "no re-review|never re-review" <<<"$step6"'

# --- change 0355: rung dispatch is docket-review's topology, not a universal post-step -------
# A resolved skills.review value is invoked as a skill; ONLY the built-in docket-review binding
# gets the deterministic Docket rung dispatch. A custom binding returns its own findings and
# receives no additional Docket review. Guards bind the dispatch sentence to its condition so
# deleting the condition (re-unconditionalizing the dispatch) reddens.
assert "controller: the named Step-6 slice terminator exists" 'grep -q "^### Step 6.5" "$IMPL"'
step6_flat="$(flat "$step6")"
assert "controller: \$SKILL_REVIEW remains a directed skill invocation" \
  'grep -qE "SKILL_REVIEW[^.]{0,160}DIRECTED to:" <<<"$step6_flat"'
assert "controller: rung dispatch is conditional on the docket-review binding" \
  'grep -qiE "is \`docket-review\`\*\*[^.]{0,120}rung wrapper" <<<"$step6_flat"'
assert "controller: the rung fan-out is the binding's topology, not a universal post-step" \
  'grep -qiE "\`docket-review\`.s (own )?topology" <<<"$step6_flat"'
assert "controller: a custom review binding receives no additional Docket rung" \
  'grep -qiE "dispatch \*\*no\*\* docket reviewer rung in addition" <<<"$step6_flat"'
assert "controller: the auto fallback dispatches no reviewer" \
  'grep -qiE "warning prominently.[^.]{0,60}dispatch no reviewer" <<<"$step6_flat"'

# --- the fix-loop reference itself --------------------------------------------
FIX="$REPO/skills/docket-implement-next/references/fix-loop.md"
assert "fix-loop: the reference exists" '[ -f "$FIX" ]'
fix_body="$(cat "$FIX" 2>/dev/null)"
assert "fix-loop: reference is non-vacuous (>= 30 lines)" \
  '[ "$(grep <<<"$fix_body" -c .)" -ge 30 ]'

# Every assert below whose pattern can SPAN A LINE BREAK reads a newline-FLATTENED haystack. grep
# matches within a line, so a phrase-spanning assert over hard-wrapped markdown silently doubles as
# a line-wrap guard: a pure re-flow of a paragraph reddens it with a message about a policy that
# did not change, sending the next author hunting for a rule nobody touched. Flattening keys the
# assert on the prose alone, which is what it is about.
#
# Four shapes deliberately stay LINE-based, because the line IS the thing they guard: the
# `^`-anchored restated-rubric bullet just below, the line-count floor above, the pipe-anchored
# disposition-table rows, and every awk section extractor (an extractor's input must keep its
# newlines or the slice is the whole file). A handful of asserts keep reading $fix_body simply
# because a single unbroken literal — `task-routing.md`, `docket-build-task`, `batch` — is not
# splittable by a re-wrap and gains nothing from flattening.
#
# `[^.]` bounds a window to one sentence; `[^.|]` additionally holds the table-cell boundary. Use
# `[^.|]` wherever the routing TABLE could otherwise bridge two flattened rows and satisfy a guard
# whose prose rule was deleted — mutation-checked: with a bare `[^.]`, the blocker-floor assert
# passes off `| \`economy\` | … the blocker floor … | | \`standard\` |` alone.
#
# `-s` (squeeze) is load-bearing, not tidiness: a wrapped LIST ITEM or numbered step indents its
# continuation lines, so a plain `tr '\n' ' '` leaves "build-evidence" and "record" separated by
# four spaces and a single-space pattern misses. Found by the re-wrap positive control — a re-flow
# at width 64 reddened the post-revert-re-run guard until the squeeze went in.
flatten(){ tr -s '[:space:]' ' '; }
fix_flat="$(flatten <<<"$fix_body")"

# The routing axis DELEGATES — it must never restate the rubric it shares with docket-build.
assert "fix-loop: routes by character via the shared rubric" \
  'grep -qF -- "task-routing.md" <<<"$fix_body"'
# LINE-anchored on purpose: `^- **\`economy\`** — *only when*` detects the rubric's own bullet
# reappearing as a bullet. Flattening would erase the line start and with it the whole signal.
assert "fix-loop: does not restate the rubric's economy bullet" \
  '! grep -qE "^- \*\*\`economy\`\*\* — \*only when\*" <<<"$fix_body"'

# The CEILING is the whole safety argument: no fix task may ever reach max, at any severity.
assert "fix-loop: never dispatches the max profile" \
  'grep -qiE "never[^.|]{0,80}\`?max\`?|no fix task[^.|]{0,60}max" <<<"$fix_flat"'
# `[^.]`, not `[^.|]`, and deliberately: the routing table's own `| \`max\` | **halt** |` row states
# this rule as legitimately as the prose sentence does, and it satisfied this assert before the
# flattening too. Narrowing it to prose-only would be a semantics change, not a haystack change.
assert "fix-loop: a max-character blocker halts" \
  'grep -qiE "max[^.]{0,120}halt" <<<"$fix_flat"'

# The FLOOR is the ceiling's mirror: a blocker's fix may never START below standard, so a blocker
# misrouted as mechanical still reaches premium before halting (the pre-0218 guarantee).
assert "fix-loop: blocker fixes start no lower than standard (the floor)" \
  'grep -qiE "blocker[^.|]{0,120}no lower than \`standard\`|floor[^.|]{0,120}\`standard\`" <<<"$fix_flat"'
assert "fix-loop: the floor is named as the one exception to orthogonality" \
  'grep -qiE "exception[^.|]{0,120}orthogonalit" <<<"$fix_flat"'

# Severity sets POSTURE only — the orthogonality claim, which is what keeps a minor finding from
# being handed to a cheap model just for being minor.
assert "fix-loop: severity selects the failure posture, not the profile" \
  'grep -qiE "severity[^.|]{0,100}posture" <<<"$fix_flat"'

# Task shape and commits.
# Task ORDER is load-bearing, not cosmetic: blockers first is what keeps the non-blocker fix
# commits at the branch's tail, so the suite gate's revert never has to unstack a blocker fix that
# landed on top of the same region. Scoped to the section where task order BELONGS — a whole-file
# grep would stay green if the rule existed only as an aside inside the revert step, and a rule
# restated at its point of use is the drift class this repo already documents
# (restatement-accumulates-its-own-guards).
# The extractor keeps its newline-bearing input ($fix_body) — an awk range over a flattened file
# would match the first `## ` heading and slice nothing. The FLATTENED slice is derived from it and
# is what the phrase-spanning asserts read; both are covered by the one non-vacuity anchor, since
# flattening a non-empty slice cannot produce an empty one.
tasks_section="$(awk '/^## Tasks, batching, commits/{f=1; next} /^## /{f=0} f' <<<"$fix_body")"
tasks_flat="$(flatten <<<"$tasks_section")"
assert "fix-loop: the tasks section is extractable (anchor for the ordering guard)" \
  '[ -n "$tasks_section" ]'
assert "fix-loop: fix tasks are ordered blockers first" \
  'grep -qiE "blockers?[^.|]{0,40}(first|before)" <<<"$tasks_flat"'
assert "fix-loop: the ordering is justified by leaving non-blockers at the branch tail" \
  'grep -qiE "tail" <<<"$tasks_section"'
assert "fix-loop: blockers and importants get one task per finding" \
  'grep -qiE "(one task per finding|per-finding task)" <<<"$fix_flat"'
assert "fix-loop: minors batch by shared routed profile" \
  'grep -qiE "batch" <<<"$fix_body"'
assert "fix-loop: fixes run the docket-build-task contract" \
  'grep -qF -- "docket-build-task" <<<"$fix_body"'

# The Tier C dispatch clause — what happens when profile dispatch is unavailable. Scoped to the
# paragraph that carries it (blank-line-delimited, the same slicing shape
# tests/test_dispatch_capability.sh uses), with its own non-vacuity anchor.
#
# That file already wires THIS paragraph as a `check_site` row, and those asserts are not restated
# here: the site exists, it cites the convention's *Dispatch-capability resolution*, it forbids
# concluding unavailability "from a tool name", and `Tier C` sits in the same clause as the site's
# own noun. Its convention-side companions pin the Tier C table ROW. What no assert anywhere
# covered is this clause's own operative half — the equivalence that makes a MISSING wrapper the
# same condition as a rejection, the posture, the borrowed authorizer, and the non-fallback — i.e.
# everything a reader who has established unavailability actually acts on.
dispatch_para="$(awk 'BEGIN{RS=""} /If profile dispatch is unavailable/{print; exit}' "$FIX")"
dispatch_flat="$(flatten <<<"$dispatch_para")"
assert "fix-loop: the Tier C dispatch paragraph is extractable (non-vacuity anchor)" \
  '[ -n "$dispatch_para" ] && grep -qF -- "Tier C" <<<"$dispatch_para"'
assert "fix-loop: an unregistered profile wrapper is the same condition as a concrete rejection" \
  'grep -qiE "unregistered profile wrapper[^.|]{0,60}same condition[^.|]{0,60}rejection" <<<"$dispatch_flat"'
assert "fix-loop: the fix dispatch carries the authorized-or-halt posture" \
  'grep -qF -- "authorized-or-halt" <<<"$dispatch_flat"'
assert "fix-loop: an explicitly configured skills.build: auto authorizes fixing inline" \
  'grep -qiE "explicitly configured .?skills\.build: auto.?[^.|]{0,80}inline" <<<"$dispatch_flat"'
assert "fix-loop: any other resolved value is abort-and-report" \
  'grep -qiE "other resolved value[^.|]{0,40}abort-and-report" <<<"$dispatch_flat"'
assert "fix-loop: recording every finding instead is NOT the fallback" \
  'grep -qiE "recording every finding[^.|]{0,60}not[^.|]{0,20}the fallback" <<<"$dispatch_flat"'

# The count cap. Without it, only escalations and suite runs are bounded — a ten-plus-finding
# review expands Step 6 without limit. Blockers must stay outside the count, or the cap disarms
# the same gate the blocker floor exists to protect. Scoped to the tasks section, where the cap
# belongs; overflow must land in the disposition table, not silently vanish.
# Re-keyed off the literal "five" when the cap became configurable: the prose must now name the
# EXPORT, because an agent reading this file has to know which value to look up rather than a
# number to apply. The default is asserted separately below so a doc that names the knob but drops
# the number — leaving the reader with no value at all when no layer sets it — still reddens.
#
# These four read the NEWLINE-FLATTENED haystacks defined above ($tasks_flat / $fix_flat), for the
# reason given there. `[^.|]` still bounds the window to one sentence and still keeps the
# disposition TABLE row (pipe-delimited) from satisfying the overflow assert on its own.
assert "fix-loop: non-blocker fix tasks are capped per run" \
  'grep -qE "at most .?REVIEW_MAX_FIX_TASKS.? non-blocker fix tasks" <<<"$tasks_flat"'
assert "fix-loop: the cap's default is stated where the knob is named" \
  'grep -qiE "REVIEW_MAX_FIX_TASKS[^.]{0,120}\`?10\`? by default" <<<"$tasks_flat"'
assert "fix-loop: blockers are never counted against the cap" \
  'grep -qiE "blockers are never counted against" <<<"$tasks_flat"'
assert "fix-loop: cap overflow takes the deferred disposition" \
  'grep -qiE "overflow[^.|]{0,120}deferred|deferred[^.|]{0,120}overflow" <<<"$fix_flat"'

# The suite gate: revert-and-record, bounded at two runs. Flattened haystack — even "full suite"
# and "at most two" are wrap-splittable two-word phrases.
assert "fix-loop: re-runs the full suite after fixes land" \
  'grep -qiE "full[- ]suite" <<<"$fix_flat"'
assert "fix-loop: a red re-run reverts the NON-BLOCKER fix commits" \
  'grep -qiE "revert[^.|]{0,120}non-blocker|non-blocker[^.|]{0,120}revert" <<<"$fix_flat"'
assert "fix-loop: the gate is bounded at two suite runs" \
  'grep -qiE "two suite runs|at most two" <<<"$fix_flat"'
assert "fix-loop: still-red after the revert halts" \
  'grep -qiE "still[- ]red[^.|]{0,80}halt" <<<"$fix_flat"'

# The revert path's re-run must REFRESH the evidence record too. Without it the first (red) run's
# result survives into the PR body's `docket:build-evidence` block for a branch that is actually
# green — misreporting to the human and to docket-finalize-change, which reads that block to decide
# whether to skip its own post-rebase suite run. Scoped to the suite-gate SECTION and to the
# numbered step that re-runs, not the whole file: the first branch already says "refresh", so a
# whole-file grep would stay green with the revert path silent — exactly the defect.
gate_section="$(awk '/^## The suite gate/{f=1; next} /^## /{f=0} f' <<<"$fix_body")"
gate_flat="$(flatten <<<"$gate_section")"
assert "fix-loop: the suite-gate section is extractable (anchor for the re-run guard)" \
  '[ -n "$gate_section" ]'
# The numbered-step extractor is line-based by construction (it keys on `^[0-9]+. `), so it reads
# the un-flattened slice; its OWN output is then flattened for the phrase-spanning assert below.
rerun_step="$(awk '/^[0-9]+\. /{keep = tolower($0) ~ /re-?run/} keep' <<<"$gate_section")"
rerun_flat="$(flatten <<<"$rerun_step")"
assert "fix-loop: the post-revert re-run step is extractable (non-vacuity anchor)" \
  '[ -n "$rerun_step" ]'
assert "fix-loop: the post-revert re-run refreshes the build-evidence record" \
  'grep -qiE "refresh[^.]{0,80}(build-)?evidence record" <<<"$rerun_flat"'

# A conflicted revert is the one way the "never worse than the green build that entered" guarantee
# can fail open: an autonomous run has no human to finish a half-applied revert, and the worktree is
# shared. The posture must be stated, and it must be a halt that first puts the worktree back.
# Scoped to the suite-gate section for the same reason as the re-run guard above.
assert "fix-loop: a conflicted revert halts" \
  'grep -qiE "conflict[^.]{0,160}halt|halt[^.]{0,160}conflict" <<<"$gate_flat"'
assert "fix-loop: a conflicted revert restores the worktree to its pre-revert state" \
  'grep -qiE "worktree[^.]{0,120}pre-revert|pre-revert[^.]{0,120}worktree" <<<"$gate_flat"'

# The knob, and the always-fix-blockers carve-out that makes it safe.
assert "fix-loop: reads the severity threshold from the resolved knob" \
  'grep -qF -- "REVIEW_MIN_FIX_SEVERITY" <<<"$fix_body"'
assert "fix-loop: blockers are fixed regardless of the threshold" \
  'grep -qiE "blocker[^.|]{0,120}regardless" <<<"$fix_flat"'

# `unverified-build-state` — the one blocker the controller answers itself, and the only suite run
# in Step 6 that is NOT charged to the gate's two-run bound. Both halves shipped unguarded.
#
# The bound answer is the load-bearing half: without it this file holds two sentences that can be
# read as contradicting each other ("at most three suite runs across Step 6" here, "at most two
# suite runs" in the gate section), and the reader who resolves that contradiction the other way —
# charging the self re-run to the gate — leaves a run with ONE gate slot: a red post-fix suite
# could revert but never re-verify, and revert-and-record fails exactly where it matters. So the
# REASON is guarded alongside the answer; an unreasoned "does not count" is one edit away from
# being tidied back into the bound by someone who cannot see why it sits outside.
#
# Scoped to the threshold SECTION — where the carve-out belongs — with the knob as the slice's
# non-vacuity anchor, and flattened for the same reason as everything above.
thr_section="$(awk '/^## The severity threshold/{f=1; next} /^## /{f=0} f' <<<"$fix_body")"
thr_flat="$(flatten <<<"$thr_section")"
assert "fix-loop: the severity-threshold section is extractable (non-vacuity anchor)" \
  '[ -n "$thr_section" ] && grep -qF -- "REVIEW_MIN_FIX_SEVERITY" <<<"$thr_section"'
assert "fix-loop: unverified-build-state is never handed to a fix worker" \
  'grep -qiE "unverified-build-state[^.|]{0,80}never hand" <<<"$thr_flat"'
assert "fix-loop: the controller resolves it by re-running the suite itself" \
  'grep -qiE "resolve[^.|]{0,40}re-run(ning)? the suite yourself" <<<"$thr_flat"'
assert "fix-loop: that self re-run sits OUTSIDE the gate's two-run bound" \
  'grep -qiE "re-run does[^.|]{0,20}not[^.|]{0,40}count against[^.|]{0,60}bound" <<<"$thr_flat"'
assert "fix-loop: the carve-out is REASONED — charging it would spend the revert path's re-run" \
  'grep -qiE "charg[^.|]{0,60}gate[^.|]{0,60}revert[^.|]{0,40}re-run" <<<"$thr_flat"'
assert "fix-loop: the resulting Step 6 total is stated (at most three suite runs)" \
  'grep -qiE "at most[^.|]{0,20}three[^.|]{0,20}suite runs" <<<"$thr_flat"'
assert "fix-loop: the gate's own bound is stated as scoped to the gate, unchanged" \
  'grep -qiE "bound[^.|]{0,40}scoped to the gate" <<<"$thr_flat"'

# The recording surface. Flattened: "disposition table" is a two-word phrase a re-wrap can split.
assert "fix-loop: the PR body carries a disposition table" \
  'grep -qiF -- "disposition table" <<<"$fix_flat"'
# Anchored on the table ROWS' shape inside the extracted section, not on the words anywhere in the
# file. A whole-file `\bfixed\b`-style loop was near-vacuous: all of these words occur in the
# surrounding prose independently of the table ("recorded unfixed in the PR body", "the reverted
# findings"), so deleting every row left all four green — it confirmed vocabulary, not the table.
# The section extractor gets its own non-vacuity anchor for the same reason the auto-capture and
# suite-gate slices below/above do: a renamed heading would otherwise empty the slice and make the
# row asserts unfalsifiable. Verified by mutation: deleting a single row reddens exactly that row's
# assert. `reported` is in the loop because beyond-the-branch work is reported for deliberate
# capture (automatic change capture is deferred from Go v1) — a finding that takes that path needs a
# row or the table is not the complete per-finding accounting it claims to be.
disp_section="$(awk '/^## Recording/{f=1; next} /^## /{f=0} f' <<<"$fix_body")"
assert "fix-loop: the disposition-table section is extractable (non-vacuity anchor)" \
  '[ -n "$disp_section" ]'
for d in fixed deferred reverted recorded reported; do
  assert "fix-loop: the disposition table has a '$d' ROW" \
    'grep -qE "^\| \*\*'"$d"'\*\* \|" <<<"$disp_section"'
done

# --- change 0231: the discard-and-re-dispatch prohibition in the fix loop's OWN vocabulary ---
#
# docket-implement-next Step 6 dispatches docket-build-task workers directly and never loads
# docket-build's SKILL.md, so it cannot inherit the docket-build controller rule and must not
# import docket-build's `halted` BUILD outcome. Its disposition is abort-and-report with the
# change left in-progress and claimed_at refreshed. These asserts pin that it says so in its own
# terms. The docket-build side of the same rule is guarded in tests/test_docket_build.sh, on
# $ctrl_malformed_flat and $ctrl_dispatch_flat.
#
# No separate non-vacuity floor: these read $fix_flat, which the section's own
# "fix-loop: reference is non-vacuous (>= 30 lines)" assert above already arms against an
# unreadable or moved $FIX turning every negative grep below into a permanent green.
assert "fix-loop: forbids discarding the worktree and dispatching a fresh worker" \
  'grep -qiF -- "never discard the worktree and dispatch a fresh worker" <<<"$fix_flat"'

# The disposition must be the fix loop's own, stated with the prohibition rather than imported.
assert "fix-loop: gives that prohibition the abort-and-report disposition" \
  'grep -qiE "never discard the worktree and dispatch a fresh worker.{0,240}abort-and-report" <<<"$fix_flat"'

assert "fix-loop: that disposition refreshes the claim lease" \
  'grep -qiE "never discard the worktree and dispatch a fresh worker.{0,240}claimed_at" <<<"$fix_flat"'

# It must NOT import docket-build's build-outcome vocabulary, which is the mis-import this
# separate sentence exists to avoid. Keyed on the VOCABULARY, not on phrasings: any occurrence of
# the outcome/section word (`halted`, `Halting conditions`) or of the build role itself (`build`,
# which `\b` also matches inside `docket-build`) within the disposition window is the import.
# Enumerating spellings — the pre-review form `(return .halted.|halted build outcome)` — stayed
# green on the realistic mis-imports "halt per `docket-build`'s *Halting conditions*" and "halts
# the build". Bare "Halt instead", the fix loop's own word for stopping, is deliberately still
# allowed: only the inflected `halted`/`halting` and the role noun are banned.
assert "fix-loop: does not import the build role halted outcome for this rule" \
  '! grep -qiE "never discard the worktree and dispatch a fresh worker.{0,240}(\bhalt(ed|ing)\b|\bbuild\b)" <<<"$fix_flat"'

# A5: the prohibition must not claim to reach finalize, which has no discard-and-re-dispatch path.
assert "fix-loop: the prohibition does not claim to cover docket-finalize-change" \
  '! grep -qiE "never discard the worktree and dispatch a fresh worker.{0,200}finalize" <<<"$fix_flat"'

# --- the PR body records a disposition, not a wishlist ------------------------
EP="$REPO/skills/docket-implement-next/references/edge-paths.md"
ep_body="$(cat "$EP" 2>/dev/null)"
assert "edge-paths: reference is non-vacuous (>= 15 lines)" \
  '[ "$(grep <<<"$ep_body" -c .)" -ge 15 ]'
assert "edge-paths: the PR body carries the findings disposition table" \
  'grep -qiF -- "disposition table" <<<"$ep_body"'
assert "edge-paths: no longer parks importants/minors for merge-time judgment alone" \
  '! grep -qiF -- "left for merge-time judgment" <<<"$ep_body"'

# --- Verify (human) is manual checks only -------------------------------------
RT="$REPO/skills/docket-implement-next/results-template.md"
rt_body="$(cat "$RT" 2>/dev/null)"
assert "results-template: is non-vacuous (>= 15 lines)" \
  '[ "$(grep <<<"$rt_body" -c .)" -ge 15 ]'
assert "results-template: Verify (human) excludes fixed findings" \
  'grep -qiE "fixed finding[^.]{0,80}(never|not)|(never|not)[^.]{0,80}fixed finding" <<<"$rt_body"'

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
#
# RETIRED BY CHANGE 0316 (Go migration). The prose assertions that used to live here required the
# finalize SKILL to restate the whole conditional-skip predicate: the three skip conditions, the
# fails-toward-running posture, the audit log, and change 0190's second limb (strict-ancestor
# head_sha + a results_dir allowlist derived with --name-only -z --no-renames over tracked paths).
#
# Two independent reasons none of that belongs in the skill any more:
#
#   1. The DECISION moved into Go. internal/app/finalize_rebase.go owns `gateDecision`, the pure
#      local-gate skip policy; the gate is composed into `docket finalize rebase`/`rebase-continue`,
#      which report the compose state ("skipped"/"ran") and the Permit naming the evidence head a
#      skip rests on. A skill restating a predicate the binary evaluates is duplication that drifts.
#   2. The SECOND LIMB is deferred, not merely relocated. Change 0316's *Out of scope* defers
#      "results-only skips", and the skill says so positively rather than staying silent:
#      "There is no strict-ancestor or results-only skip."
#
# What survives is the property worth guarding: finalize must still consume the build-evidence
# chain, and must still state the deferral positively so a reader cannot mistake silence for an
# undocumented skip. Both asserts below are anchored for non-vacuity.
FIN="$REPO/skills/docket-finalize-change/SKILL.md"
assert "finalize SKILL.md exists and is non-empty (non-vacuity anchor)" '[ -s "$FIN" ]'
assert "finalize: reads the PR body's build-evidence block" \
  'grep -qF -- "build-evidence" "$FIN"'
assert "finalize: the gate is composed into the Go rebase verb (not restated as skill prose)" \
  'grep -qiE "composed into .?.?docket .?finalize rebase|gate is composed into" "$FIN"'
assert "finalize: the deferred results-only skip is stated positively, never left silent" \
  'grep -qiE "no strict-ancestor or results-only skip" "$FIN"'
# The exact-head permit is the one skip that DOES survive, and it must stay conditioned on a no-op
# rebase — otherwise "skip" would mean "merge an untested branch".
assert "finalize: the surviving skip still requires a no-op rebase and exact-head green evidence" \
  'grep -qiE "no-op" "$FIN" && grep -qiE "exact" "$FIN" && grep -qiE "green build-evidence|build-evidence for the" "$FIN"'

# ...and the key the prose names must be a REAL resolved key, not a plausible-looking literal. The
# resolver has to BOTH assign it from its own leaf name and fence it to the repo-committed layer;
# an arming switch settable from a machine-scoped layer would re-open the finding it closes (a
# machine asserting a suite property for repos where nobody established it).
assert "0190: the arming key is resolved from its own leaf by docket-config.sh" \
  'grep -qE "^FINALIZE_SKIP_RESULTS_ONLY_DELTA=.*skip_results_only_delta" "$REPO/scripts/docket-config.sh"'
assert "0190: the arming key is coordination-fenced (repo-committed layer only)" \
  'grep -q "skip_results_only_delta" <<<"$(sed -n "/^for _fkey in /p" "$REPO/scripts/docket-config.sh")"'

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
# Re-keyed by change 0218: the pre-0218 arithmetic (one / two / three) was written when only a
# BLOCKER fix could land after the build gate. The in-branch fix loop lands importants and minors
# too and can itself run the suite twice (fix run, then the post-revert run), so the ceiling is
# FOUR — build gate, fix run, post-revert run, finalize rebase. The floor key is "on the clean
# path" rather than a bare "one full-suite run", because the fix-loop paragraph in this same
# section already says "One full-suite run after the fixes land" and would satisfy the looser key
# with the arithmetic sentence deleted entirely.
assert "README states the suite-run count the change delivers" \
  'grep -qiE "one full-suite run on the clean path" <<<"$rvsec" && grep -qiE "at most \*{0,2}four" <<<"$rvsec"'

# --- change 0218: README documents the in-branch fix loop and its knob -------
# Scoped to the docket-review section via $rvsec — the section-scoped haystack this file already
# extracts, whose non-vacuity is anchored by "README: the docket-review section was located" above.
# (The plan named a fresh `rm_body`; $rvsec IS that extractor, so it is reused rather than
# duplicated — a second identical awk range would be one more thing to keep in step.) A whole-README
# grep for "min_fix_severity" would be satisfied by any passing mention anywhere in a 900-line file.
assert "README: the fix loop replaced the record-everything rule" \
  '! grep -qiF -- "go into the PR body for merge-time judgment" <<<"$rvsec"'
assert "README: documents the in-branch fix loop" \
  'grep -qiF -- "fix loop" <<<"$rvsec"'
assert "README: documents the min_fix_severity knob" \
  'grep -qF -- "min_fix_severity" <<<"$rvsec"'
assert "README: documents the max_fix_tasks cap knob" \
  'grep -qF -- "max_fix_tasks" <<<"$rvsec"'
assert "README: states that fix routing never reaches max" \
  'grep -qiE "never[^.]{0,80}\`?max\`?" <<<"$rvsec"'
assert "README: states blockers are fixed regardless of the knob" \
  'grep -qiE "blocker[^.]{0,120}regardless" <<<"$rvsec"'

# The shipped cross-harness default is now docket-review (change 0193). Anchored on the resolver,
# both directions, mirroring the build guard in tests/test_docket_build.sh.
review_default="$(grep -E 'SKILL_REVIEW=|skill_role review' "$REPO/scripts/docket-config.sh")"
assert "resolver's review default line was located (non-vacuity anchor)" '[ -n "$review_default" ]'
assert "shipped skills.review default is docket-review" \
  'grep -qF -- "docket-review" <<<"$review_default"'
assert "shipped skills.review default is no longer superpowers review" \
  '! grep -qF -- "superpowers:requesting-code-review" <<<"$review_default"'
DY="$REPO/.docket.yml"
dy_skills="$(awk "/^skills:/{f=1;next} /^[a-z_]+:/{f=0} f" "$DY")"
# Change 0193: docket-review is the shipped default, so this repo stops pinning it. The dogfood
# is now "we run what we ship", and the assert that proves it is the pin's ABSENCE.
assert "this repo no longer pins skills.review (it runs the shipped default)" '[ -z "$dy_skills" ]'
# The block guards a real property — the example config is the cross-harness default surface and
# must agree with the resolver — so invert it, both directions: a revert of the example alone
# would leave it disagreeing with the resolver, which is exactly the drift this pair exists to
# catch.
assert "the example config states the shipped docket-review default" \
  'grep -qE "^ +review: +docket-review$" "$REPO/.docket.example.yml"'
assert "the example config no longer ships the superpowers review default" \
  '! grep -qE "^ +review: +superpowers:requesting-code-review$" "$REPO/.docket.example.yml"'

# --- change 0331 (re-keyed onto the driver by change 0342): Step 6's re-mint path names its producer ---
# The recovery path for missing/malformed/stale evidence must be an executable chain, and change 0342
# migrated it off the raw `docket gate launch`/`observe` verbs onto the native gate DRIVER:
# `docket gate drive start` produces the drive that `docket gate drive advance` carries to a terminal
# disposition, and only a `PASSED` disposition exposes the raw run dir that `docket evidence record`
# consumes at the exact feature head, with `docket evidence verify` re-checking the same head. Guarded
# on command SHAPE and ORDERING, whitespace-collapsed so a Markdown re-flow is not a semantic failure,
# one bounded gap per ERE (two stacked gaps backtrack catastrophically on exactly the mutated input).
# The checker takes the file as an argument so the same code runs against the authored skill and the
# mutated copy.

# Position of a fixed literal in a haystack (fails when absent) — pure bash, no regex.
remint_pos(){ local pre="${1%%"$2"*}"; [ "$pre" != "$1" ] || return 1; printf '%s\n' "${#pre}"; }

check_remint_chain(){
  local file="$1" sec flat p_start p_advance p_record p_verify
  sec="$(awk '/^### Step 6 — Review/{f=1;next} /^### Step 6\.5 — Results close-out/{f=0} f' "$file")"
  [ -n "$sec" ] || { echo "step6-slice-empty"; return 1; }
  flat="$(tr -s '[:space:]' ' ' <<<"$sec")"
  # (a) the DRIVER producer exists on the re-mint path — never the retired raw launch/observe verbs
  grep -qF -- "docket gate drive start" <<<"$flat" || { echo "start-missing"; return 1; }
  grep -qF -- "docket gate drive advance" <<<"$flat" || { echo "advance-missing"; return 1; }
  # (b) start shape: --repo-dir, --run-root, and the child-command `--` boundary (one gap per pattern)
  grep -qE -e "docket gate drive start [^.]{0,120}--repo-dir" <<<"$flat" || { echo "repo-dir-missing"; return 1; }
  grep -qE -e "--repo-dir [^.]{0,120}--run-root" <<<"$flat" || { echo "run-root-missing"; return 1; }
  grep -qE -e "--run-root [^.]{0,160} -- " <<<"$flat" || { echo "separator-missing"; return 1; }
  # (c) ordering: start precedes advance precedes record; record precedes verify
  p_start="$(remint_pos "$flat" "docket gate drive start")" || { echo "start-missing"; return 1; }
  p_advance="$(remint_pos "$flat" "docket gate drive advance")" || { echo "advance-missing"; return 1; }
  p_record="$(remint_pos "$flat" "docket evidence record")" || { echo "record-missing"; return 1; }
  p_verify="$(remint_pos "$flat" "docket evidence verify")" || { echo "verify-missing"; return 1; }
  [ "$p_start" -lt "$p_advance" ] || { echo "start-after-advance"; return 1; }
  [ "$p_advance" -lt "$p_record" ] || { echo "advance-after-record"; return 1; }
  [ "$p_record" -lt "$p_verify" ] || { echo "record-after-verify"; return 1; }
  # (d) only a PASSED disposition exposes the raw run dir that record consumes at the exact head
  grep -qE -e "PASSED[^.]{0,140}raw run dir" <<<"$flat" || { echo "passed-no-rundir"; return 1; }
  grep -qE -e "docket evidence record [^.]{0,80}--run" <<<"$flat" || { echo "record-no-rundir"; return 1; }
  grep -qE -e "--run [^.]{0,80}--head <feature head>" <<<"$flat" || { echo "record-no-head"; return 1; }
  # (e) verify follows record and checks the same head
  grep -qE -e "docket evidence verify [^.]{0,100}--head <feature head>" <<<"$flat" || { echo "verify-no-head"; return 1; }
  return 0
}

# SEPARATE non-vacuity anchors: an empty or renamed section must fail HERE, positively, so the
# negative conditions inside the checker can never pass against text that was simply not searched.
remint_sec="$(awk '/^### Step 6 — Review/{f=1;next} /^### Step 6\.5 — Results close-out/{f=0} f' "$IMPL")"
assert "remint: Step 6 section slice is non-empty (existence anchor)" '[ -n "$remint_sec" ]'
assert "remint: the named terminator heading still exists (slice cannot widen to EOF)" \
  'grep -qF -- "### Step 6.5 — Results close-out" "$IMPL"'

assert "remint: start -> advance -> record -> verify driver chain holds in the authored skill" \
  'check_remint_chain "$IMPL"'

# Mutation proof: the guard is load-bearing. Copy, confirm the occurrence, remove it, confirm the
# removal landed, and require the SAME checker to reject the copy. Temp copy only — the real
# worktree is never edited, so no restoration step exists to get wrong.
remint_mut="$(mktemp "${TMPDIR:-/tmp}/remint-mutation.XXXXXX")"
assert "remint mutation: the gate-drive-start occurrence exists before removal" \
  'grep -qF -- "docket gate drive start" "$IMPL"'
grep -vF -- "docket gate drive start" "$IMPL" >"$remint_mut"
assert "remint mutation: the removal landed in the copy" \
  '! grep -qF -- "docket gate drive start" "$remint_mut"'
assert "remint mutation: the checker rejects the start-less copy" \
  '! check_remint_chain "$remint_mut" >/dev/null'
rm -f "$remint_mut"

# Mutation (i): break the start SHAPE only. Strip the child-command ` -- ` boundary from the
# start line (keeping `docket gate drive start`, `--repo-dir`, `--run-root` present) and require
# clause (b)'s separator check — and ONLY it — to redden. Keying on the checker's exact first-failing
# error string proves the start line's separator is what's guarded, not some other clause tripping.
remint_shape_mut="$(mktemp "${TMPDIR:-/tmp}/remint-shape-mutation.XXXXXX")"
sed '/docket gate drive start/ s/ -- / /g' "$IMPL" >"$remint_shape_mut"
assert "remint mutation (shape): all four command literals survive the start-line edit" \
  'grep -qF -- "docket gate drive start" "$remint_shape_mut" && grep -qF -- "docket gate drive advance" "$remint_shape_mut" && grep -qF -- "docket evidence record" "$remint_shape_mut" && grep -qF -- "docket evidence verify" "$remint_shape_mut"'
assert "remint mutation (shape): the checker rejects on the separator clause specifically" \
  '[ "$(check_remint_chain "$remint_shape_mut")" = "separator-missing" ]'
rm -f "$remint_shape_mut"

# Mutation (ii): break the ORDERING only. Swap the `docket gate drive advance` and `docket evidence
# record` occurrences (all four command tokens stay present, the start line's shape is untouched) so
# advance no longer precedes record, and require clause (c) — and ONLY it — to redden.
remint_ord_mut="$(mktemp "${TMPDIR:-/tmp}/remint-ordering-mutation.XXXXXX")"
sed -e 's/docket gate drive advance/@@REMINT_SWAP@@/g' \
    -e 's/docket evidence record/docket gate drive advance/g' \
    -e 's/@@REMINT_SWAP@@/docket evidence record/g' "$IMPL" >"$remint_ord_mut"
assert "remint mutation (ordering): all four command literals survive the swap" \
  'grep -qF -- "docket gate drive start" "$remint_ord_mut" && grep -qF -- "docket gate drive advance" "$remint_ord_mut" && grep -qF -- "docket evidence record" "$remint_ord_mut" && grep -qF -- "docket evidence verify" "$remint_ord_mut"'
assert "remint mutation (ordering): the checker rejects on the advance-before-record clause specifically" \
  '[ "$(check_remint_chain "$remint_ord_mut")" = "advance-after-record" ]'
rm -f "$remint_ord_mut"

echo "---"; [ "$fails" -eq 0 ] && echo "PASS" || { echo "FAIL ($fails)"; exit 1; }
