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
# assert. `minted` is in the loop because auto-capture still admits genuinely distinct,
# beyond-the-branch work — a finding that takes that path needs a row or the table is not the
# complete per-finding accounting it claims to be.
disp_section="$(awk '/^## Recording/{f=1; next} /^## /{f=0} f' <<<"$fix_body")"
assert "fix-loop: the disposition-table section is extractable (non-vacuity anchor)" \
  '[ -n "$disp_section" ]'
for d in fixed deferred reverted recorded minted; do
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

# --- change 0218: auto-capture no longer absorbs this branch's own findings ---
# Extended by change 0226, which reframed this reference as a capability-discovery pipeline. The
# 0218 clauses below are RE-ANCHORED to the new wording, never dropped: the reframe adds the
# positive half (what to look for, and the gates it must clear) and must not soften the negative.
AC="$REPO/skills/docket-convention/references/auto-capture.md"
ac_body="$(cat "$AC" 2>/dev/null)"
# Floor raised 20 -> 60 by 0226: the file roughly doubled, and a floor that a half-deleted file
# still clears is not a non-vacuity anchor.
assert "auto-capture: reference is non-vacuous (>= 60 lines)" \
  '[ "$(grep <<<"$ac_body" -c .)" -ge 60 ]'
ac_flat="$(flatten <<<"$ac_body")"
# Scoped to the Materiality bar SECTION, not the whole file: a whole-file grep would match the
# clause wherever it landed, including a passing mention in the mint paragraph, which is not where
# the bar is applied. The section extractor gets its own non-vacuity anchor for the same reason.
# The extractor keeps NEWLINE-BEARING input on purpose — an awk range over a flattened file has one
# line to range over, so the slice would become the whole file.
ac_bar="$(awk '/^## Materiality bar/{f=1;next} /^## /{f=0} f' "$AC")"
assert "auto-capture: the Materiality bar section was located (non-vacuity anchor)" \
  '[ -n "$ac_bar" ]'
# The three clause asserts below read a FLATTENED slice (learnings: phrase-grep-over-wrapped-prose):
# each pattern can span a line break, so against raw prose they double as line-wrap guards and a
# pure re-flow reddens asserts about policy that never changed.
ac_bar_flat="$(flatten <<<"$ac_bar")"
assert "auto-capture: the Materiality bar slice survives flattening (non-vacuity anchor)" \
  '[ -n "$ac_bar_flat" ]'
# Proximity-shaped, not a bare "in-branch" presence check. The clause this guards is "work THE
# CURRENT RUN WILL FIX in-branch FAILS THE BAR"; the sentence after it independently says the
# finding "is fixed in-branch", so a presence grep stayed GREEN when the rule-bearing clause was
# deleted (observed while mutation-testing this assert). Key it on in-branch fixability sitting next
# to failing the bar — the `[^.]` class keeps the pair inside one sentence.
#
# The scoping ("the current run will fix") is INSIDE the keyed shape, not a separate assert: this
# reference is shared by mint sites with no branch and no fix loop (the finalize/status harvest),
# and an unscoped bar tells those sites to drop precisely the follow-up nothing else will pick up.
# Dropping the scope back to the unscoped "work fixable by a small in-branch edit" wording must
# redden here, so the run-will-fix qualifier is load-bearing in the pattern.
assert "auto-capture: work the current run will fix in-branch fails the bar" \
  'grep -qiE "current run will fix in-branch[^.]{0,40}fails the bar" <<<"$ac_bar_flat"'
# The other half of the scoping: the harvest sites must be told the clause does not reach them,
# or a harvest-time reader applies the fix-loop caller's rule with no fix loop to apply it with.
assert "auto-capture: the harvest sites are exempt from the in-branch clause" \
  'grep -qiE "harvest[^.]{0,20}exempt" <<<"$ac_bar_flat"'
assert "auto-capture: the exemption says why — no branch, no fix loop at harvest" \
  'grep -qiE "no open branch[^.]{0,20}no fix loop" <<<"$ac_bar_flat"'

# --- change 0226: the reference is a DISCOVERY pipeline, gated -----------------------------
# Case one of the two the spec requires: a discovery that QUALIFIES as a new change. The file must
# actively instruct a search, not merely permit one, and must state the gates that search clears.
ac_look="$(awk '/^## What to look for/{f=1;next} /^## /{f=0} f' "$AC")"
assert "auto-capture: the 'What to look for' section was located (non-vacuity anchor)" \
  '[ -n "$ac_look" ]'
ac_look_flat="$(flatten <<<"$ac_look")"
# Shape, not spelling: an imperative to LOOK FOR work, near the property that makes it mintable.
assert "auto-capture: the reader is told to look for independently valuable work" \
  'grep -qiE "look for[^.]{0,200}(worth its own change|independently valuable)" <<<"$ac_look_flat"'
# The six discovery categories the spec enumerates. Keyed one per assert so deleting any single
# category reddens by name — a single "the categories are present" assert would not.
for cat in "reusable capabilit" "product or workflow feature" "policy or lifecycle" \
           "tooling opportunit" "architectural gap" "outlives the active change"; do
  assert "auto-capture: discovery category present: '$cat'" \
    'grep -qiF -- "'"$cat"'" <<<"$ac_look_flat"'
done
ac_gates="$(awk '/^## Admission gates/{f=1;next} /^## /{f=0} f' "$AC")"
assert "auto-capture: the 'Admission gates' section was located (non-vacuity anchor)" \
  '[ -n "$ac_gates" ]'
# All SIX gates, as an ordered list — a prose paragraph that merely mentions them would let the
# count drift silently. Six numbered items is the shape the spec pins.
n_gates="$(grep -cE '^[0-9]+\. ' <<<"$ac_gates")"
assert "auto-capture: exactly six numbered admission gates (found $n_gates)" \
  '[ "$n_gates" -eq 6 ]'
ac_gates_flat="$(flatten <<<"$ac_gates")"
for gate in "outside the scope" "independently valuable" "more than a defect" \
            "boundary" "separate change" "without expanding"; do
  assert "auto-capture: admission gate names '$gate'" \
    'grep -qiF -- "'"$gate"'" <<<"$ac_gates_flat"'
done
# The site-C carve-out must be stated HERE, in the gates section, not only in *Routing* two sections
# downstream. Gates 1, 2, 3 and 6 are written against "the active change" / the active branch, which
# do not exist at the docket-finalize-change / docket-status harvest; a harvest reader who stops at
# this section and applies them literally suppresses exactly the cheap-to-fix follow-up the
# *Materiality bar*'s change-0218 exemption exists to protect. The existing carve-out assert keys on
# `$ac_route_flat`, so without these two the gates section can stay unscoped forever with every
# assert green. Keyed to detect the UNSCOPED state (learnings: assert-detects-removal-not-
# replacement): deleting the scoping clause reddens both.
#
# Exactly ONE bounded repeat per pattern, deliberately: an ERE stacking two `{0,n}` gaps backtracks
# catastrophically on NON-matching input — the very mutation these exist to catch — and hangs for
# minutes instead of reddening (recorded on the *drill-down trigger* assert below). The `[^.]` class
# scopes each gap to one sentence, and both bounds sit under the 255 ERE repetition ceiling BSD grep
# enforces (PATH `grep` here is ugrep, which accepts more; tests/test_grep_portability.sh does not).
assert "auto-capture: the six gates are scoped to sites with a branch and a fix loop" \
  'grep -qiE "(these six|the six|all six)[^.]{0,120}(branch and a fix loop|sites A and B)" <<<"$ac_gates_flat"'
assert "auto-capture: the gates section names the harvest exemption and its own bar" \
  'grep -qiE "harvest[^.]{0,160}Materiality bar" <<<"$ac_gates_flat"'

# Case two: a current-branch finding that must NOT become a change. The never-mint list is the
# suppression half; it lives with the gates so a reader who reaches the gates cannot miss it.
#
# The gap is `[^.]*`, NOT a counted bound. The never-mint list is one semicolon-separated sentence,
# so the `[^.]` class already scopes each pattern to it — and its last entry sits 279 characters past
# "never mint", past the 255 ERE repetition ceiling BSD grep enforces (PATH `grep` here is ugrep,
# which accepts a larger bound and hides the defect; tests/test_grep_portability.sh does not).
assert "auto-capture: a review finding about the active diff is never minted" \
  'grep -qiE "never mint[^.]*review finding about the active diff" <<<"$ac_gates_flat"'
assert "auto-capture: work implement-next fixes in the current branch is never minted" \
  'grep -qiE "never mint[^.]*fix in the current branch" <<<"$ac_gates_flat"'
assert "auto-capture: cleanup with no independent value is never minted" \
  'grep -qiE "never mint[^.]*no independent value" <<<"$ac_gates_flat"'
assert "auto-capture: a vague idea with no boundary is never minted" \
  'grep -qiE "never mint[^.]*(vague idea|no clear outcome)" <<<"$ac_gates_flat"'

# --- change 0226: routing is site-dependent, and site C keeps its own bar -------------------
ac_route="$(awk '/^## Routing/{f=1;next} /^## /{f=0} f' "$AC")"
assert "auto-capture: the 'Routing' section was located (non-vacuity anchor)" \
  '[ -n "$ac_route" ]'
for route in "fix-in-branch" "record-as-learning" "report-only" "capture-as-new-change"; do
  assert "auto-capture: routing names the '$route' route" \
    'grep -qF -- "'"$route"'" <<<"$ac_route"'
done
# Row-shaped, deliberately NOT flattened: flattening the table would let one row's cells satisfy a
# pattern keyed on another row (the bridging failure the fix-loop guards above record).
assert "auto-capture: routing has a row for the implement-next reconcile site" \
  'grep -qE "^\| A [^|]*\|" <<<"$ac_route"'
assert "auto-capture: routing has a row for the implement-next review site" \
  'grep -qE "^\| B [^|]*\|" <<<"$ac_route"'
assert "auto-capture: routing has a row for the finalize/status harvest site" \
  'grep -qE "^\| C [^|]*\|" <<<"$ac_route"'
# The load-bearing asymmetry: fix-in-branch is UNAVAILABLE at site C. A guard that only checked the
# row exists would pass with the whole exemption flattened away.
#
# The two properties are asserted per CELL, not over the whole row. A single row-wide
# "unavailable|**no**" alternation stayed GREEN when the routing cell's "fix-in-branch
# **unavailable**" was rewritten to "all four routes" (observed while mutation-testing this assert):
# the row's *branch + fix loop* column independently says `**no**`, so it satisfied the alternation
# with the exemption itself deleted. `[^|]` keeps each pattern inside the cell that owns the claim —
# the same cell-bridging failure the routing table is deliberately left unflattened for.
c_row="$(grep -E "^\| C [^|]*\|" <<<"$ac_route")"
assert "auto-capture: site C has neither an open branch nor a fix loop" \
  'grep -qiE "^\| C [^|]*\| *\*\*no\*\* *\|" <<<"$c_row"'
assert "auto-capture: site C's routing cell marks fix-in-branch unavailable" \
  'grep -qiE "fix-in-branch[^|]{0,20}(unavailable|\*\*no\*\*)" <<<"$c_row"'
ac_route_flat="$(flatten <<<"$ac_route")"
assert "auto-capture: site C keeps its own admission bar, not the six gates" \
  'grep -qiE "site C[^.]{0,120}own admission bar" <<<"$ac_route_flat"'

# --- change 0226: the five capture fields live UNDER a leading `## Why` ---------------------
# mint-stub.sh hard-rejects a body that does not START with `## Why`, so the fields must be labelled
# lines under that one heading. The negative assert is the load-bearing one (learnings:
# assert-detects-removal-not-replacement): promoting any field to a top-level `##` is the exact
# defect that would ship a body mint-stub rejects, and a presence-only guard would stay green.
# This extractor terminates on the NAMED next section, not the generic `^## ` the slices above use:
# the section embeds a fenced example body whose first line is the literal `## Why` that
# `mint-stub.sh` requires, so a generic top-level-heading terminator cuts the slice off immediately
# before the very heading — and the five fields — this block exists to guard.
# The named terminator is itself asserted: a marker-keyed extractor only validates the markers it
# finds, so renaming `## Per discovery` would silently widen the slice to end-of-file with every
# positive AND negative assert below still green.
assert "auto-capture: the capture-fields terminator section exists" \
  'grep -qE "^## Per discovery$" "$AC"'
ac_fields="$(awk '/^## What a captured discovery says/{f=1;next} /^## Per discovery/{f=0} f' "$AC")"
assert "auto-capture: the capture-fields section was located (non-vacuity anchor)" \
  '[ -n "$ac_fields" ]'
assert "auto-capture: the capture body starts with a leading '## Why'" \
  'grep -qE "^## Why$" <<<"$ac_fields"'
for fld in Trigger Opportunity "Independent value" Boundary "Reason for deferral"; do
  assert "auto-capture: capture field present: '$fld'" \
    'grep -qF -- "**'"$fld"'**" <<<"$ac_fields"'
  assert "auto-capture: capture field '$fld' is NOT a top-level heading (mint-stub body contract)" \
    '! grep -qiE "^## '"$fld"'" <<<"$ac_fields"'
done
# Keyed on the CONTRACT, not the token: a bare `mint-stub` grep stays green if the "start with
# `## Why`" clause is reworded to its opposite or demoted to a passing mention. Exactly ONE bounded
# gap (ugrep backtracks catastrophically on non-matching input — the mutated file — when two are
# stacked), and the class is `[^;]` rather than `[^.]` because the very next character after the
# anchor is the `.` in `mint-stub.sh`. The bound stays under the 255 ERE repetition ceiling BSD
# grep enforces.
ac_fields_flat="$(flatten <<<"$ac_fields")"
assert "auto-capture: the mint-stub '## Why' body contract is stated where the fields are" \
  'grep -qiE "mint-stub[^;]{0,200}start with \`## Why\`" <<<"$ac_fields_flat"'
# The invariant is POSITIONAL: mint-stub.sh rejects a body that does not START with `## Why`, so an
# example rewritten with the five fields first and the heading last would satisfy every presence
# assert above while documenting a body the script exits 1 on. Assert the order directly.
ac_why_ln="$(awk '/^## Why$/{print NR; exit}' <<<"$ac_fields")"
ac_trigger_ln="$(awk '/^\*\*Trigger\*\*/{print NR; exit}' <<<"$ac_fields")"
assert "auto-capture: the '## Why' heading and the first field line were both located" \
  '[ -n "$ac_why_ln" ] && [ -n "$ac_trigger_ln" ]'
assert "auto-capture: the '## Why' heading precedes the labelled field lines" \
  '[ -n "$ac_why_ln" ] && [ -n "$ac_trigger_ln" ] && [ "$ac_why_ln" -lt "$ac_trigger_ln" ]'

# --- change 0226: the convention SUMMARY stays a summary (progressive disclosure) -----------
# The summary is what a mint site reads inline before deciding whether to drill down. It must carry
# the intent and the pointer; the categories, gates, fields, and routing table live ONLY in the
# reference. The negative asserts are the load-bearing half: a well-meaning future edit that copies
# the gates up here is exactly the restatement class this project keeps paying for, and a
# presence-only guard would never see it.
CONV="$REPO/skills/docket-convention/SKILL.md"
ac_sum="$(awk '/^### Auto-capture \(shared definition\)/{f=1;next} /^### /{f=0} f' "$CONV")"
assert "convention: the Auto-capture summary section was located (non-vacuity anchor)" \
  '[ -n "$ac_sum" ]'
ac_sum_flat="$(flatten <<<"$ac_sum")"
assert "convention: the summary leads with capability-discovery intent" \
  'grep -qiE "(capability|independently valuable)[^.]{0,160}(discover|discovery)" <<<"$ac_sum_flat"'
# Keyed on "admission gate", not the plan's looser "admission|gate": the pre-change summary already
# said "waits at the human's groom gate", so the loose alternation was satisfied by prose about a
# different gate entirely and stayed green through the very rewrite it exists to demand.
assert "convention: the summary names the strict admission gating without enumerating it" \
  'grep -qiE "admission gate" <<<"$ac_sum_flat"'
assert "convention: the summary keeps its blocking drill-down pointer" \
  'grep -qF -- "references/auto-capture.md" <<<"$ac_sum_flat"'
assert "convention: the drill-down pointer is BLOCKING" \
  'grep -qiE "blocking" <<<"$ac_sum_flat"'
# The trigger must be MINT-SITE-scoped, not discovery-scoped. Keyed to detect the removal, not to
# confirm the replacement (learnings: assert-detects-removal-not-replacement): the defect is the
# pre-fix "Discovered follow-up work mid-run -> read ... now (blocking)" form, under which the
# reference's own "What to look for" pass is loaded only by a reader who has ALREADY discovered
# something — so the discovery pass is specified but unreachable, and every presence assert above
# stays green. Requiring the mint-site scoping to sit in the SAME sentence as the read imperative is
# what reddens on that mutation; a bare "mint site" presence grep would not (the *Mint sites*
# paragraph directly above names them for a different purpose). Flattened, because the phrase wraps
# across lines. The `[^.]` class scopes the gap to one sentence; the bound is under the 255 ERE
# repetition ceiling BSD grep enforces. Exactly ONE bounded repeat, deliberately: a two-`{0,n}`
# version of this pattern ("...read[^.]{0,60}auto-capture\.md") backtracks catastrophically on
# NON-matching input — i.e. on the very mutation this assert exists to catch — and hung for minutes
# instead of reddening (observed while mutation-testing it).
assert "convention: the drill-down trigger fires at each mint site, not only after a discovery" \
  'grep -qiE "at each mint site[^.]{0,200}auto-capture" <<<"$ac_sum_flat"'
# Progressive disclosure, asserted as absence. Each of these is a thing the reference owns.
assert "convention: the summary does NOT enumerate the discovery categories" \
  '! grep -qiE "reusable capabilit|tooling opportunit" <<<"$ac_sum_flat"'
assert "convention: the summary does NOT enumerate the six admission gates" \
  '! grep -qE "^[0-9]+\. " <<<"$ac_sum"'
assert "convention: the summary does NOT carry the routing table" \
  '! grep -qE "^\| " <<<"$ac_sum"'
assert "convention: the summary does NOT carry the five capture fields" \
  '! grep -qiE "reason for deferral" <<<"$ac_sum_flat"'
# Mechanics that MUST stay inline — the summary is not a bare pointer either.
for tok in AUTO_CAPTURE_ENABLED AUTO_CAPTURE_TYPES policy-suppressed docket-auto-groom; do
  assert "convention: the summary keeps the '$tok' mechanic inline" \
    'grep -qF -- "'"$tok"'" <<<"$ac_sum_flat"'
done

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

# --- change 0190: the docs-only ancestor limb of the skip predicate -------------------------
# The sentinels above all survive 0190 as substrings, but none of them binds the NEW disjunct:
# with every one of them green the allowlist limb could widen to "any path" — or vanish — unseen.
# Keyed on syntactic SHAPE (an ancestor condition co-present with a paths-under-the-allowlisted-
# prefix condition), not on one blessed spelling, and read from a newline-flattened haystack so a
# reflow of this very long prose item cannot silently unbind it. The extraction runs to the next
# top-level numbered item, and its non-vacuity is anchored first: an awk range over a renamed or
# deleted item yields an EMPTY haystack, on which the shape grep would fail loudly rather than a
# negated grep passing on nothing.
skip_item="$(awk '/^4\. \*\*Conditional skip/{f=1} f && /^5\. /{f=0} f' "$FIN")"
skip_flat="$(flatten <<<"$skip_item")"
assert "finalize: the conditional-skip item was located (non-vacuity anchor)" \
  '[ -n "$skip_flat" ] && grep -qF -- "build-evidence" <<<"$skip_flat"'
assert "finalize: the skip's second limb needs a strict-ancestor head_sha AND an allowlisted-prefix path set" \
  'grep -qiE "strict ancestor" <<<"$skip_flat" && grep -qiE "(every|all) paths? changed[^|]{0,120}(under|within)[^|]{0,60}allowlist" <<<"$skip_flat"'
# The derivation's FLAGS are load-bearing, not incidental spelling. git's rename detection is on by
# default and `--name-only` emits only a rename pair's DESTINATION, so without rename detection
# disabled a post-gate `git mv tests/foo.sh <results_dir>/foo.sh` yields a delta that is 100%
# docs-only by the prefix test — the skip fires and a branch whose suite composition changed after
# the gate merges unvalidated. Bind the three tokens as CO-PRESENT in the skip item (the
# rename-suppressing flag by shape, either spelling, since `-M0` is equivalent), so a later reflow
# or a "simplify the command" edit cannot silently drop the guard. Same flattened haystack, whose
# non-vacuity is anchored by the "conditional-skip item was located" assert above.
assert "finalize: the delta derivation names --name-only, -z, and a rename-suppressing flag" \
  'grep -qF -- "--name-only" <<<"$skip_flat" && grep -qE -e "-z" <<<"$skip_flat" && grep -qE -e "--no-renames|-M0" <<<"$skip_flat"'

# THE REMAINING NORMATIVE CLAUSES OF THE SAME ITEM. The flags above are bound; these three are the
# rest of what step 4 actually promises, and every one of them is a sentence a reflow or a
# "tighten the prose" edit deletes without moving any assert already written. Each is keyed on
# SHAPE — a claim co-present with its counter-claim — never on one blessed spelling, and read from
# the same flattened haystack whose non-vacuity the "conditional-skip item was located" assert
# above establishes.
#
# (1) TRACKED PATHS ONLY. The prefix test is a statement about the DIFF, not about the working
# tree: a filesystem walk of the results directory would see untracked scratch files and ignore a
# tracked deletion, so "all under <results_dir>/" would stop meaning what the skip needs it to
# mean. The shape is a tracked-only claim co-present with a refusal to traverse the filesystem.
assert "finalize: the delta is tested over tracked paths only, never by filesystem traversal" \
  'grep -qiE "tracked[^|]{0,20}paths?[^|]{0,30}only" <<<"$skip_flat" && grep -qiE "never[^|]{0,40}(filesystem|traversal|traverse)" <<<"$skip_flat"'
# (2) EMPTY DIFF OVER A NON-EMPTY RANGE IS DOUBT. `head_sha..HEAD` can be non-empty on the graph
# while `git diff` reports nothing — an empty commit, a revert pair, a merge collapsed away. An
# empty path list trivially satisfies "every path is under the allowlist", so without this clause
# the widest possible uncertainty produces the strongest possible permit. The shape is the
# graph/diff disagreement co-present with the run-the-suite disposition.
assert "finalize: a range non-empty on the graph but empty in the diff is doubt and runs the suite" \
  'grep -qiE "(non-?empty|not empty)[^|]{0,40}graph[^|]{0,80}empty[^|]{0,40}diff" <<<"$skip_flat" && grep -qiE "diff[^|]{0,60}(doubt|runs? the suite)" <<<"$skip_flat"'
# (3) THE LOG NAMES WHICH PERMIT MATCHED. 0170 already required a skip to be logged (asserted over
# the whole file above), and that assert survives 0190 unchanged while the log goes on saying only
# "skipped" — which makes the two permits indistinguishable in the audit trail, exactly when a
# second, weaker permit has just been added. The shape is a matched-permit log line naming both.
assert "finalize: the skip log names which permit matched (exact-SHA vs the ancestor permit)" \
  'grep -qiE "(match|name)[^|]{0,40}permit" <<<"$skip_flat" && grep -qiE "exact-?SHA[^|]{0,120}(ancestor|docs-only)" <<<"$skip_flat"'

# ARMING (change 0190 whole-branch review, finding 2). The two shape asserts above bind the second
# limb's PREDICATE; neither binds its GATE. With both of them green the limb can still be stated as
# unconditionally ON — which is exactly how it first shipped, carrying a trailing "degrade off"
# sentence that no code read and every downstream repo would have had to self-apply. Bind the
# arming key INTO the skip item (same flattened haystack, same non-vacuity anchor above), in three
# independent directions: the exported name is present, the default reading is stated, and the
# self-applied framing it replaced is gone. Deleting any one of the three reddens on its own.
assert "finalize: the skip item names the exported key its second limb is gated on" \
  'grep -qF -- "FINALIZE_SKIP_RESULTS_ONLY_DELTA" <<<"$skip_flat"'
assert "finalize: the skip item states the unset/false reading (0170's equality-only predicate)" \
  'grep -qiE "(unset|false)[^|]{0,140}equality-only" <<<"$skip_flat"'
assert "finalize: the limb is no longer a self-applied degrade-off judgement" \
  '! grep -qiF -- "degrade off" <<<"$skip_flat"'
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

echo "---"; [ "$fails" -eq 0 ] && echo "PASS" || { echo "FAIL ($fails)"; exit 1; }
