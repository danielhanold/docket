#!/usr/bin/env bash
# tests/test_docket_build.sh — change 0167. Contract guards for docket's own build role:
# the docket-build controller skill and the docket-build-task worker skill.
# Guards are keyed on the load-bearing CLAUSES of each contract, so a rewrite that keeps the
# rule stays green while a rewrite that drops the rule reddens. Run: bash tests/test_docket_build.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
WORKER="$REPO/skills/docket-build-task/SKILL.md"
ROUTING="$REPO/skills/docket-build/references/task-routing.md"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# Collapse runs of whitespace so a phrase assert survives a pure re-flow of hard-wrapped markdown
# (learnings: phrase-grep-over-wrapped-prose). Runs, not only newlines: an indented list
# continuation leaves several spaces behind, and `tr '\n' ' '` alone would not close them up.
flat(){ tr -s '[:space:]' ' ' <<<"$1"; }

# ---------------------------------------------------------------------------
# docket-build-task — the worker contract
# ---------------------------------------------------------------------------
assert "worker: SKILL.md exists" '[ -f "$WORKER" ]'
worker_body="$(cat "$WORKER" 2>/dev/null)"

# Non-vacuity floor: every negative/shape assert below reads $worker_body, so an empty or
# unreadable file must redden HERE rather than passing every grep by default.
assert "worker: contract is non-vacuous (>= 40 lines)" \
  '[ "$(grep <<<"$worker_body" -c .)" -ge 40 ]'

# The three outcome tokens are the controller's entire input vocabulary — each must be defined
# by its Outcomes-section bullet (the shape a token-presence-anywhere grep cannot observe being
# removed, since each token also appears in the frontmatter description, the commit section, and
# the return template regardless of whether its defining bullet exists).
for tok in COMPLETE NEEDS_ESCALATION BLOCKED; do
  assert "worker: defines the $tok outcome (Outcomes bullet)" \
    'grep -qE "^- \*\*\`'"$tok"'\`\*\*" <<<"$worker_body"'
done

# Exactly-one-commit rule: the deliverable of a task is one commit, and only on success.
assert "worker: requires exactly one commit on success" \
  'grep -qiE "exactly one (successful )?(task )?commit" <<<"$worker_body"'
assert "worker: forbids committing on a non-COMPLETE outcome" \
  'grep -qiE "(only on success|no commit|does not commit|never commit)" <<<"$worker_body"'

# TDD default plus the evidence-bound exception with all three required statements.
assert "worker: states the focused TDD cycle" \
  'grep -qiE "fails for the intended reason" <<<"$worker_body"'
assert "worker: bug fixes require a failing regression test" \
  'grep -qiE "regression test" <<<"$worker_body"'
assert "worker: guards require mutation evidence" \
  'grep -qiE "mutation evidence|turns red" <<<"$worker_body"'
for clause in "why RED/GREEN was unsuitable" "what verification replaced it" "what residual risk"; do
  assert "worker: TDD exception must state — $clause" 'grep -qiF -- "$clause" <<<"$worker_body"'
done
# The insufficient-reason list is the teeth of the exception: without it "hard to test" walks.
assert "worker: names the insufficient reasons for skipping RED/GREEN" \
  'grep -qiF -- "hard to test" <<<"$worker_body" && grep -qiF -- "no existing tests" <<<"$worker_body"'

# NO REVIEW: the worker self-reviews; it must never dispatch a reviewer or fix agent. The negation
# is word-anchored (\b) so it cannot match inside "Nothing", "not", "none", or "known" — an
# unanchored "no" let a body rewrite state the OPPOSITE rule and still pass (probe: finding 2).
assert "worker: forbids dispatching a reviewer or another agent" \
  'grep -qiE "\b(never|does not|do not|no)\b[^.]{0,80}(dispatch|subagent)" <<<"$worker_body"'
# Keyed on the body sentence, not the bare word, since "self-review" alone also appears in the
# frontmatter description and would satisfy a presence-only grep even if the body rule were gone.
assert "worker: self-review is part of implementation, not a second agent" \
  'grep -qiF -- "self-review is part of" <<<"$worker_body"'

# Escalation is a narrow door — an expected RED or one failed run is NOT an escalation.
assert "worker: excludes an expected RED / ordinary debugging from escalation" \
  'grep -qiE "expected RED" <<<"$worker_body"'
assert "worker: escalation needs a concrete reason" \
  'grep -qiE "concrete reason" <<<"$worker_body"'

# Scope: it owns ONE task and must not rewrite earlier task commits.
assert "worker: owns exactly one task" 'grep -qiE "exactly one task|only that task" <<<"$worker_body"'
assert "worker: must not rewrite earlier task commits" \
  'grep -qiE "not rewrite|never rewrite" <<<"$worker_body"'

# METADATA BOUNDARY (whole-branch review, finding 3). The four profile wrappers deliberately do
# NOT preload docket-convention, and docket-convention was the only document asserting they "perform
# no docket metadata operations" — i.e. the one document these workers never read. They are
# full-tool agents that write code and commit, so today the boundary holds only incidentally
# (a feature worktree happens to contain no .docket/). It must be stated in the contract they DO
# load. Extracted from the `## Scope` section rather than grepped file-wide, because the boundary is
# a SCOPE rule: a stray mention in the intro or in a NOTES example would satisfy a whole-file grep
# while the normative bullet was gone. Non-vacuity companion first, per this file's standard.
scope_blk="$(awk '/^## Scope$/{inblk=1;next} inblk && /^## /{inblk=0} inblk' <<<"$worker_body")"
assert "worker: the Scope section body is non-vacuous" \
  '[ "$(grep -c . <<<"$scope_blk")" -ge 8 ]'
assert "worker: works only inside the feature worktree, on its branch" \
  'grep -qiF -- "inside the feature worktree" <<<"$scope_blk"'
# Negations are word-anchored (\b) so a rewrite cannot state the OPPOSITE rule inside "Nothing",
# "none", or "notwithstanding" and still pass — the idiom this file already uses for the
# no-dispatch and no-concurrency rules.
assert "worker: performs NO docket metadata operations" \
  'grep -qiE "\b(no|never|not)\b[^.]{0,60}docket metadata operations" <<<"$scope_blk"'
# Each forbidden target named individually: a boundary that lists only some of them is the gap.
while IFS= read -r tgt; do
  [ -n "$tgt" ] || continue
  assert "worker: metadata boundary forbids writing — $tgt" 'grep -qF -- "$tgt" <<<"$scope_blk"'
done <<'EOF'
.docket/
metadata branch
change files
ADRs
board
learnings ledger
EOF
assert "worker: never pushes, force-pushes, resets --hard, or rebases" \
  'grep -qiE "\b(never|do not|does not)\b[^.]{0,40}push[^.]{0,80}(reset|rebase)" <<<"$scope_blk"'
# Plan checkboxes are not progress state (finding 4) — the worker half of the rule; the controller
# half is asserted in its Checkpointing block below.
assert "worker: plan checkboxes are not progress state" \
  'grep -qiE "checkboxes are \*\*not\*\* progress state|checkboxes are not progress state" <<<"$scope_blk"'

# An escalated worker inherits the worktree — it must account for uncommitted changes. The bare
# word "uncommitted" is only the SUBJECT of the rule, not the rule: a body rewrite instructing the
# escalated worker to `git checkout .` over the leftovers kept that word and stayed green (final
# fix wave, finding 2c). Anchored on the prohibition itself.
assert "worker: escalated worker must not blindly discard existing uncommitted work" \
  'grep -qiE "uncommitted" <<<"$worker_body" && grep -qiF -- "never discard them blindly" <<<"$worker_body"'

# Repository instructions outrank this generic contract. "AGENTS.md" alone names only the
# artifact — reversing the override DIRECTION (this contract overriding the repo's instructions)
# left it green (final fix wave, finding 2d) — so the operative "— **override**" construction,
# which reads correctly only in the repo-instructions-win direction, is required alongside it.
assert "worker: repository instructions override the generic contract" \
  'grep -qF -- "AGENTS.md" <<<"$worker_body" && grep -qF -- "— **override**" <<<"$worker_body"'

# The return fence is the literal WIRE FORMAT the controller keys on when reading an outcome, so
# each field name is a contract term, not formatting. Deleting the whole fenced block reddened
# nothing before this loop (final fix wave, finding 8). Each field is anchored at line start,
# where the schema declares it — the tokens also occur as prose elsewhere in the file.
for field in OUTCOME PROFILE VERIFICATION TDD COMMIT NOTES; do
  assert "worker: return schema declares the $field field" \
    'grep -qE "^'"$field"':" <<<"$worker_body"'
done

# ---------------------------------------------------------------------------
# Change 0231 — the amend ban covers the worker's OWN commit, not only earlier tasks'.
#
# The 0223 incident had the woken worker commit and then amend inside its own turn, sweeping the
# replacement's work in. A "never amend after emitting your return" clause would not have reached
# that, and "after a rival has written to the same files" is not worker-observable. Widening the
# existing Scope line to any commit is observable, absolute, and binds the woken worker.
# ---------------------------------------------------------------------------
worker_scope="$(awk '/^## Scope/{f=1;next} f&&/^## /{exit} f' <<<"$worker_body")"
worker_scope_flat="$(flat "$worker_scope")"

# Non-vacuity through the SAME extractor, so a renamed heading cannot green the negative below.
assert "worker: the Scope section is extractable" \
  '[ -n "$worker_scope" ] &&
   grep -qF -- "Implement only that task" <<<"$worker_scope_flat"'

assert "worker: the amend ban covers any commit, including one this worker just made" \
  'grep -qiE "never rewrite, amend, or revert \*\*any\*\* commit" <<<"$worker_scope_flat"'

assert "worker: directs correcting by adding a commit rather than amending" \
  'grep -qiE "adding another commit, never by amending" <<<"$worker_scope_flat"'

# Detect the REMOVED state. The pre-0231 wording scoped the ban to earlier task commits, which is
# exactly the gap the woken worker walked through; its return must redden this.
assert "worker: the narrow earlier-task-only amend ban is gone" \
  '! grep -qiE "amend, or revert earlier task commits" <<<"$worker_scope_flat"'

# The escalated-worker allowance is a deliberate carve-out and must survive the widening: an
# escalated worker still revises the weaker worker's UNCOMMITTED changes. Widening the COMMIT ban
# into a ban on touching uncommitted work would break escalation, so pin that it did not.
assert "worker: the escalated-worker allowance for uncommitted work survives" \
  'grep -qiF -- "You may revise or replace them" <<<"$worker_scope_flat"'

# ---------------------------------------------------------------------------
# docket-build — the controller contract
# ---------------------------------------------------------------------------
CTRL="$REPO/skills/docket-build/SKILL.md"
assert "controller: SKILL.md exists" '[ -f "$CTRL" ]'
ctrl_body="$(cat "$CTRL" 2>/dev/null)"
assert "controller: contract is non-vacuous (>= 50 lines)" \
  '[ "$(grep <<<"$ctrl_body" -c .)" -ge 50 ]'

# It must dispatch by AGENT NAME — the whole point of the change is that model and effort are
# properties of a named agent rather than an ad-hoc per-dispatch argument.
for a in docket-build-economy docket-build-standard docket-build-premium docket-build-max; do
  assert "controller: names the $a agent" 'grep -qF -- "'"$a"'" <<<"$ctrl_body"'
done

# --- change 0218: the routing rubric is EXTRACTED, with two owners ------------
# The rubric moved out of the controller so docket-implement-next's fix loop can classify a
# finding from the same source instead of restating it. These asserts are the multi-owner
# fixture the shared-resource-keeps-first-owner-assumptions learning demands: a guard that only
# checks docket-build still points at the file passes against a reference that never learned it
# has a second consumer. Every rubric assert below is a RELOCATION of the controller-scoped assert
# that used to sit here, repointed at the artifact that now owns the prose — never a restoration.
assert "routing: the shared reference exists" '[ -f "$ROUTING" ]'
routing_body="$(cat "$ROUTING" 2>/dev/null)"
# Non-vacuity floor, this file's standard: every negative assert below reads $routing_body, so a
# missing or unreadable reference must redden HERE rather than passing every `! grep` by default.
assert "routing: reference is non-vacuous (>= 20 lines)" \
  '[ "$(grep <<<"$routing_body" -c .)" -ge 20 ]'

# The rubric itself now lives in the reference, not the controller. The economy/standard bullets
# keep the "^- **`token`**" structural idiom this file already uses for the worker's outcome
# bullets, since the prose disjunctions they replace were absorbed by the section's summary
# sentence — deleting the operative bullet reddened nothing (0167 fix round 2, finding 2).
assert "routing: economy must be POSITIVELY established (in the reference)" \
  'grep -qE "^- \*\*\`economy\`\*\* — \*only when\*" <<<"$routing_body"'
assert "routing: named risk selects premium (in the reference)" \
  'grep -qiE "premium[^.]{0,200}(authentication|security boundar)" <<<"$routing_body"'
assert "routing: uncertainty defaults to standard (in the reference)" \
  'grep -qE "^- \*\*\`standard\`\*\* — everything remaining" <<<"$routing_body"'

# 0184's rule, repointed at its new owner: max is reachable only through narrow doors, and its
# DIRECT rubric is exactly two items. An assert that merely finds the word "max" in the rubric
# would stay green if the pre-0184 top-rung trigger list were pasted under the new name — which is
# the regression to detect. (Before 0184 there were three profiles and `premium` was the top rung,
# carrying those triggers; 0184 added a fourth above it, so the triggers belong to `premium` as the
# THIRD of four, not to `max`.)
#
# Both asserts read the max bullet FOLDED INTO ONE LOGICAL LINE, not the raw file. The rubric is
# hard-wrapped and grep is line-based, so against the raw body the negation below could only see a
# trigger sharing line 1 with the `**`max`**` token — a trigger pasted onto a CONTINUATION line of
# the same bullet left it green (proven by mutation while relocating it here, and true of the
# 0184-era controller-scoped assert it replaces). Since the bullet wraps after ~100 columns, that
# was most of the defect surface. Folding restores the intended SENTENCE scope: `[^.]{0,240}` still
# stops at the first period, so a trigger must be attached to `max` in its own opening sentence.
# The range is bounded by the next top-level bullet, never run to EOF (AGENTS.md, unbounded ranges).
routing_max="$(awk '
  /^- \*\*`max`\*\*/ { inb=1; printf "%s ", $0; next }
  inb && /^- \*\*/   { inb=0 }
  inb                { printf "%s ", $0 }
' <<<"$routing_body")"
# Non-vacuity companion for the extractor: an awk slice that silently returns empty makes the
# negative assert below pass against anything at all.
assert "routing: the max bullet extraction is non-vacuous" '[ "${#routing_max}" -ge 100 ]'
assert "routing: max's direct rubric is unresolved architecture + irreversible data only" \
  'grep -qiE "\*\*\`max\`\*\*[^.]{0,240}unresolved architecture" <<<"$routing_max" &&
   grep -qiE "\*\*\`max\`\*\*[^.]{0,240}irreversible" <<<"$routing_max"'
assert "routing: the demoted top-rung triggers name premium, not max" \
  '! grep -qiE "\*\*\`max\`\*\*[^.]{0,240}(authentication|security boundar|concurrency|release infrastructure)" <<<"$routing_max"'
# The RATIONALE moved with the rule — an extraction is behavior-neutral only if the why moved too.
assert "routing: the reference carries the max/premium organizing principle" \
  'grep -qiF -- "cannot walk back" <<<"$routing_body"'

# The controller keeps a stub + pointer, and no longer carries the rubric bullets itself.
assert "controller: Routing section points at the shared reference" \
  'grep -qF -- "references/task-routing.md" <<<"$ctrl_body"'
assert "controller: no longer restates the economy rubric bullet" \
  '! grep -qE "^- \*\*\`economy\`\*\* — \*only when\*" <<<"$ctrl_body"'
# The consumer-SPECIFIC rules stay in the controller: they are docket-build's, not the fix loop's.
# These two deliberately overlap the plan-override asserts further down the file — those pin the
# rule's CONTENT, these pin that the extraction left it behind instead of carrying it off.
assert "controller: keeps the plan-override rule" \
  'grep -qF -- "**Build profile:**" <<<"$ctrl_body"'
assert "controller: keeps the invalid-override halt" \
  'grep -qiE "invalid[^.]{0,80}halt" <<<"$ctrl_body"'

# TWO OWNERS. This is the assert a single-owner fixture cannot reach. The second consumer's path
# is named here so this file records the contract; its existence and its own guards belong to
# tests/test_docket_review.sh, which is where the fix loop is tested. Deliberately declared and
# never read: it is a greppable cross-reference, not a fixture. Any assert that would read it
# belongs in the owner file, next to its "fix-loop: the reference exists" assert.
IMPL_FIX="$REPO/skills/docket-implement-next/references/fix-loop.md"
assert "routing: the reference names both of its consumers" \
  'grep -qiF -- "docket-build" <<<"$routing_body" &&
   grep -qiF -- "docket-implement-next" <<<"$routing_body"'
assert "routing: the reference does not describe itself as docket-build's alone" \
  '! grep -qiE "only consumer|sole consumer" <<<"$routing_body"'

# Detect the removed state. The clean break means these tokens must not survive as PROFILE names in
# either contract; anchored on the two skill files rather than a whole-repo grep, because historical
# records under docs/ legitimately keep the old vocabulary.
#
# The retired vocabulary is now low/medium/high, and a BARE-word ban on those three is only
# sound because neither contract may state an effort tier in the first place — the controller's
# own "Never restate literal model IDs or effort tiers in your dispatch prose" rule, and the
# worker's silence on the subject. That makes this assert do double duty: it catches a stale
# profile token AND enforces the no-effort-tiers rule, which previously had no detector at all.
# If a future change ever needs a literal effort tier in either body, this assert is the thing
# that must be argued with first — narrowing it silently would drop both properties.
# Change 0218 added $ROUTING to the scanned set rather than leaving it behind: the rubric prose
# this ban most plausibly regrows a stale tier token in is exactly the prose that moved out of the
# controller, and a sentinel that keeps grepping the source after the copy moved guards nothing.
assert "controller + worker + routing reference carry no retired profile token (and state no effort tier)" \
  '! grep -qiE "\b(low|medium|high)\b" "$CTRL" "$WORKER" "$ROUTING"'

# The plan override and its fail-loud contract.
assert "controller: honors an explicit plan Build profile override" \
  'grep -qF -- "Build profile:" <<<"$ctrl_body"'
assert "controller: an invalid explicit profile HALTS rather than falling back" \
  'grep -qiE "invalid[^.]{0,120}halt" <<<"$ctrl_body"'

# The escalation ladder — all four edges, including the terminal one. Each is anchored on its
# "initial <profile>" prefix (the ladder fence's defining shape), not bare "<profile>", since the
# build gate's repair-ladder literal "premium -> max -> halt" decoy-matches any assert keyed
# on bare profile-name-then-arrow text — proven for all edges by mutation (deleting the
# whole ladder fence still left bare-anchored asserts green; fix round 1 caught it for the top
# edge only, fix round 2 applied the same anchor to the lower ones).
assert "controller: economy escalates to standard" \
  'grep -qiE "initial economy[^.]{0,40}(->|→|to)[^.]{0,20}standard" <<<"$ctrl_body"'
assert "controller: standard escalates to premium" \
  'grep -qiE "initial standard[^.]{0,40}(->|→|to)[^.]{0,20}premium" <<<"$ctrl_body"'
assert "controller: premium escalates to max" \
  'grep -qiE "initial premium[^.]{0,40}(->|→|to)[^.]{0,20}max" <<<"$ctrl_body"'
assert "controller: max escalation halts" \
  'grep -qiE "initial max[^.]{0,20}(->|→|to)?[^.]{0,20}halt" <<<"$ctrl_body"'
# Anchored on the ladder intro's exact literal sentence, not a disjunction that also matches the
# unrelated intro paragraph's "its single allowed escalation" — that alternative let the ladder's
# own "at most once" sentence be deleted without reddening (fix round 2, finding 2).
assert "controller: at most ONE escalation per task" \
  'grep -qiF -- "escalate automatically **at most once**" <<<"$ctrl_body"'
assert "controller: a retried task does not climb twice" \
  'grep -qiE "does not climb|never climbs|not climb again" <<<"$ctrl_body"'

# NO REVIEW inside the build — the defining property of this topology. Anchored on the
# Review-boundary section's defining SENTENCE START (^This build performs), not the bare prose
# literal — a fixed-string match on "no per-task independent review" alone would still be
# defeatable by a benign reorder ("no independent per-task review") per this file's own promise
# that "a rewrite that keeps the rule stays green" (fix round 2, finding 3). Line-start anchoring
# also keeps this distinct from the frontmatter description's unrelated "no per-task review" text
# (mutation probe 2, fix round 1).
assert "controller: performs no per-task review" \
  'grep -qE "^This build performs \*\*no per-task independent review\*\*" <<<"$ctrl_body"'
assert "controller: performs no final review of its own" \
  'grep -qiE "no final review|no whole-branch review of its own" <<<"$ctrl_body"'
assert "controller: hands the single review to docket-implement-next Step 6" \
  'grep -qiE "skills.review|Step 6" <<<"$ctrl_body"'

# The full-suite gate is DERIVED, never a second config key or a hand-copied fragment.
assert "controller: full-suite gate reads finalize.test_command" \
  'grep -qF -- "FINALIZE_TEST_COMMAND" <<<"$ctrl_body"'
assert "controller: falls back to finalize's existing auto-detection" \
  'grep -qiE "auto-detect" <<<"$ctrl_body"'
assert "controller: cites finalize's canonical suite-command block rather than copying it" \
  'grep -qF -- "configured-bash-finalize" <<<"$ctrl_body"'
# SINGLE SOURCE: the canonical fragment lives in finalize's SKILL.md and nowhere else. A second
# marker pair here would be the duplicate this change exists to avoid.
assert "controller: does not open a second configured-bash-finalize marker block" \
  '[ "$(grep -cF -- "<!-- configured-bash-finalize:start -->" "$CTRL")" = 0 ]'
assert "controller: introduces no second test-command config key" \
  '! grep -qiE "build\.test_command|BUILD_TEST_COMMAND" <<<"$ctrl_body"'

# A red suite becomes ONE synthetic repair task, not a repair/review loop.
assert "controller: a red suite does not invoke review" \
  'grep -qiE "red[^.]{0,80}(does not|never)[^.]{0,40}review" <<<"$ctrl_body"'
assert "controller: red suite becomes one integration-repair task" \
  'grep -qiE "integration.repair" <<<"$ctrl_body"'
assert "controller: repair ladder is premium -> max -> halt" \
  'grep -qiE "premium[^.]{0,60}max[^.]{0,60}halt" <<<"$ctrl_body"'

# Checkpointing: off by default, and the ledger path is exact. Both asserts below are anchored on
# their defining occurrence's full text, since the shorter shapes each replaces recur elsewhere in
# the file (BUILD_CHECKPOINT is also named in ## Output; the bare directory prefix also appears in
# the false-branch "write no .superpowers/docket-build/ files" sentence) and so survived deletion
# of the actual defining sentence (fix round 2, finding 2).
# The variable NAME is the controller<->resolver seam: it must match what docket-config.sh exports.
# Keyed on the phrase alone, renaming BUILD_CHECKPOINT to anything else throughout this SKILL.md
# stayed green, leaving the seam completely unguarded (final fix wave, finding 3) — so the name and
# its provenance are required together, as one literal.
assert "controller: reads BUILD_CHECKPOINT from the Step-0 config export" \
  'grep -qF -- "\`BUILD_CHECKPOINT\` from the Step-0 config export" <<<"$ctrl_body"'
assert "controller: names the ledger path" \
  'grep -qF -- ".superpowers/docket-build/<change-id>/progress.md" <<<"$ctrl_body"'
# Finding 4: the checkpoint-`false` resume story reads "the plan" for progress, and superpowers
# plans carry `- [ ]` checkboxes — a half-ticked plan is exactly the misread docket has been burned
# by. The controller half of the rule (the worker half is in its Scope block above).
assert "controller: plan checkboxes are NOT progress state on resume" \
  'grep -qiE "checkboxes are \*\*not\*\* progress state|checkboxes are not progress state" <<<"$ctrl_body" && grep -qiF -- "never checkbox marks" <<<"$ctrl_body"'
# The resume rule is the "only when" construction, not the word "ancestor": flipping "skip a task
# **only** when" to "whenever" — which turns a conjunction of three conditions into a licence to
# skip on any of them — left an "ancestor"-keyed assert green (final fix wave, finding 2b). All
# three conditions plus the restrictive quantifier are required.
assert "controller: skips a resumed task only on COMPLETE + plan hash + ancestor commit" \
  'grep -qF -- "skip a task **only** when" <<<"$ctrl_body" && grep -qiF -- "plan hash" <<<"$ctrl_body" && grep -qiF -- "ancestor" <<<"$ctrl_body"'

# ADR-0024 dispatch discipline — the rule docket has actually been burned by, and `## Dispatching a
# task` is the only place this change states it: a backgrounded or concurrent child returns
# `completed` on a half-done run. Unguarded before this block, rewriting the paragraph to "in the
# background, all tasks at once" plus "Always preload a review skill" passed, and deleting the
# paragraph outright passed too (final fix wave, finding 4). Each clause is anchored on its
# DEFINING occurrence — the dispatch sentence itself, keyed on "Dispatch the profile agent" —
# rather than on a bare adverb that recurs in prose; the concurrency negation is word-anchored
# (\b) so a rewrite cannot state the opposite rule inside a word like "Nothing" and still pass.
assert "controller: dispatches workers in the FOREGROUND" \
  'grep -qE "Dispatch the profile agent[^.]{0,60}foreground" <<<"$ctrl_body"'
assert "controller: dispatches one task at a time" \
  'grep -qE "Dispatch the profile agent[^.]{0,80}one task at a time" <<<"$ctrl_body"'
assert "controller: never dispatches two workers concurrently" \
  'grep -qiE "\b(never|does not|do not|no)\b[^.]{0,60}dispatch two workers concurrently" <<<"$ctrl_body"'

# Tier C: an un-dispatchable build halts unless the human explicitly configured auto. "Tier C" is
# the label, not the rule — rewriting the clause to "Tier C, run-inline-and-continue: no
# authorization is needed for inline" kept the label and passed (final fix wave, finding 2a), so
# the posture literal is required with it — as the bolded compound term at its DEFINING occurrence,
# since a bare "authorized-or-halt" also appears in the unregistered-agent clause below it and so
# survives inverting this paragraph — together with the authorization the posture turns on.
assert "controller: un-dispatchable profile routing halts (Tier C)" \
  'grep -qF -- "**Tier C, authorized-or-halt**" <<<"$ctrl_body" && grep -qF -- "skills.build: auto" <<<"$ctrl_body"'
assert "controller: cites the convention's dispatch-capability resolution" \
  'grep -qiF -- "Dispatch-capability resolution" <<<"$ctrl_body"'
assert "controller: forbids concluding unavailability from a tool name" \
  'grep -qF -- "never from a tool name" <<<"$ctrl_body"'
# The first-run failure mode after this change goes live: `.docket.yml` binds skills.build from
# origin/HEAD immediately, but the profile wrappers and build skills exist only once install.sh
# has re-run and the harness has restarted. Without this rule the controller would have to
# improvise exactly where Tier C forbids it. Two literals, reflow-proof: the condition (which
# appears nowhere else in the file) and the remedy it must name.
assert "controller: an unregistered profile agent is authorized-or-halt, remedied by install.sh" \
  'grep -qiF -- "not registered on this machine" <<<"$ctrl_body" && grep -qF -- "install.sh" <<<"$ctrl_body"'

# A malformed worker return is never read as success.
assert "controller: a missing or malformed outcome halts" \
  'grep -qiE "(missing or malformed|malformed)[^.]{0,60}halt" <<<"$ctrl_body"'
assert "controller: never infers success from a child reporting it finished" \
  'grep -qiE "never infer" <<<"$ctrl_body"'

# HALTING CONDITIONS (whole-branch review, findings 1/2/5). The review's framing: the contract
# repeatedly stated a PREDICATE ("a task without a commit is not complete") where it owed a
# DISPOSITION, leaving well-formed-but-wrong states — an unverifiable COMPLETE, an undetectable
# suite, a stray commit — with no defined action. One section now enumerates every halt and owns the
# shared disposition; the in-place rules point AT it instead of restating it. Anchored on the
# section HEADING at line start and required to be UNIQUE, since the phrase "Halting conditions"
# now recurs in every in-place back-pointer — a presence-anywhere grep would stay green with the
# section itself deleted, which is precisely the state this guard exists to catch.
assert "controller: has exactly one Halting conditions section" \
  '[ "$(grep -cE "^## Halting conditions$" <<<"$ctrl_body")" = 1 ]'
# The heading alone is not the rule: a heading whose disposition sentence is stripped enumerates
# conditions with no stated action — the exact defect being closed. All three parts of the
# disposition (halted / in-progress / worktree preserved) are required together.
assert "controller: every halt returns halted, in-progress, worktree preserved" \
  'grep -qE "^Every halt is the same disposition" <<<"$ctrl_body" && grep -qF -- "worktree is preserved" <<<"$ctrl_body" && grep -qiE "stays \`in-progress\`" <<<"$ctrl_body"'
# Non-vacuity companion for the extraction below (this file's standard for any awk slice).
halt_blk="$(awk '/^## Halting conditions$/{inblk=1;next} inblk && /^## /{inblk=0} inblk' <<<"$ctrl_body")"
assert "controller: the Halting conditions section body is non-vacuous" \
  '[ "$(grep -c . <<<"$halt_blk")" -ge 15 ]'
# Every halt the review enumerated, keyed INSIDE the section slice — a whole-file grep would be
# satisfied by the in-place rule that points here, so deleting a bullet would redden nothing.
while IFS='|' read -r label pat; do
  [ -n "$label" ] || continue
  assert "controller: Halting conditions enumerates — $label" \
    'grep -qiF -- "$pat" <<<"$halt_blk"'
done <<'EOF'
un-dispatchable profile routing|Profile routing is un-dispatchable
profile agent not registered|not registered on this machine
invalid explicit profile|value is invalid
malformed or unverifiable worker return|malformed or unverifiable
escalation allowance exhausted|escalation allowance is exhausted
stray commit from a failed attempt|failed attempt left a commit
no detectable suite|No suite is detectable
still red after the max repair|still red after the max repair
EOF

# Finding 1's in-place rule, at the surface that owns it: a COMPLETE is settled against GIT STATE.
# The prose check it replaced ("must come with ... a commit SHA") was satisfiable by the return TEXT
# containing a SHA-shaped string, against this repo's own recorded lesson that a child's completion
# report is unreliable in both directions. The ancestry COMMAND is the operative literal; the
# no-re-dispatch negation is word-anchored so the opposite rule cannot pass.
assert "controller: verifies a COMPLETE's commit is an ancestor of the branch tip" \
  'grep -qF -- "git merge-base --is-ancestor <sha> HEAD" <<<"$ctrl_body"'
assert "controller: never re-dispatches a task to repair its own return" \
  'grep -qiE "\b(never|do not|does not)\b[^.]{0,60}re-dispatch" <<<"$ctrl_body"'

# Finding 2's in-place rule: finalize's auto-detection exits non-zero in a repo with no test files,
# which the two-branch gate read as RED — manufacturing a repair task and burning premium ->
# max -> halt on a configuration problem. Keyed on the classification itself, not on the word
# "halt", since a rewrite that keeps the halt but drops the classification re-opens the mis-routing.
assert "controller: an undetectable suite is a configuration gap, not a red suite" \
  'grep -qiF -- "configuration gap, not a red suite" <<<"$ctrl_body" && grep -qF -- "finalize.test_command" <<<"$ctrl_body"'

# Finding 5's in-place rule: a failed attempt that left a COMMIT (not merely a dirty tree) cancels
# the escalation, because the escalated worker is separately forbidden to rewrite earlier task
# commits and the exactly-one-commit accounting is already contaminated.
assert "controller: does not escalate onto a commit left by a failed attempt" \
  'grep -qiE "\b(do not|does not|never)\b escalate onto a stray commit" <<<"$ctrl_body"'

# ---------------------------------------------------------------------------
# Change 0231 — never discard a dispatched worker's tree and re-dispatch.
#
# A worker that did not return with a schema-valid outcome may still be RUNNING; discarding its
# tree and dispatching a replacement puts two workers in one worktree, which is how change 0223's
# double-write happened. Both asserts below are region-scoped rather than whole-file: the phrase
# "dispatch a fresh worker" would also match a future summary line or the frontmatter description,
# and a whole-file grep cannot observe the rule being removed from the bullet that owns it.
# ---------------------------------------------------------------------------

# The malformed-return halting bullet, sliced from its own "- **" line to the next one.
ctrl_malformed="$(awk '
  /^- \*\*A worker return is malformed/ {inb=1; print; next}
  inb && /^- \*\*/ {exit}
  inb {print}
' <<<"$ctrl_body")"
ctrl_malformed_flat="$(flat "$ctrl_malformed")"

# Non-vacuity through the SAME extractor: a renamed bullet, a reflowed heading, or a broken awk
# range would empty $ctrl_malformed and turn the negative assert below into a permanent green.
# The companion reads a clause that predates this change and must still be there.
assert "controller: the malformed-return halting bullet is extractable" \
  '[ -n "$ctrl_malformed" ] &&
   grep -qF -- "Never re-dispatch a task to repair its own return" <<<"$ctrl_malformed_flat"'

assert "controller: that bullet also forbids discarding the worktree and dispatching a fresh worker" \
  'grep -qiF -- "never discard the worktree and dispatch a fresh worker" <<<"$ctrl_malformed_flat"'

assert "controller: the bullet gives the still-running worker as the reason" \
  'grep -qiE "did not observe return cleanly.{0,120}still be running" <<<"$ctrl_malformed_flat"'

# No bullet-scoped worktree-preservation assert here on purpose. Preservation is the SECTION's
# shared disposition — "the change stays `in-progress` and the worktree is preserved for
# inspection or resume" — and the preamble instructs each rule to "name their condition and point
# here rather than restating the disposition". That property is guarded above by
# "controller: every halt returns halted, in-progress, worktree preserved". Re-asserting it on
# this bullet would pin a restatement in place and redden on a legitimate consolidation.

# The trigger is the observable event, never elapsed patience. An undefined time threshold in a
# normative contract is unactionable and invites exactly the improvisation this change closes.
assert "controller: the bullet keys on the return, not on elapsed time" \
  '! grep -qiE "(minutes|elapsed|timed out|timeout|too long|patience)" <<<"$ctrl_malformed_flat"'

# A5: the rule must not claim it reaches finalize, which neither loads nor references docket-build.
# Scoped by proximity rather than banning the word file-wide — the build gate legitimately cites
# skills/docket-finalize-change/SKILL.md as the single source of the suite-command block.
assert "controller: the prohibition does not claim to cover docket-finalize-change" \
  '! grep -qiE "never discard the worktree and dispatch a fresh worker.{0,200}finalize" <<<"$(flat "$ctrl_body")"'

# The concurrency ban in the dispatch section, sliced to that section.
ctrl_dispatch="$(awk '/^## Dispatching a task/{f=1;next} f&&/^## /{exit} f' <<<"$ctrl_body")"
ctrl_dispatch_flat="$(flat "$ctrl_dispatch")"

assert "controller: the Dispatching a task section is extractable" \
  '[ -n "$ctrl_dispatch" ] &&
   grep -qF -- "Dispatch the profile agent" <<<"$ctrl_dispatch_flat"'

# Detect the REMOVED state: the bare concurrency ban that stopped at deliberate dispatch and did
# not reach a controller acting on a belief. Mutating the clause back to its bare form reddens this.
assert "controller: the concurrency ban binds a controller that believes the first worker is gone" \
  'grep -qiE "never dispatch two workers concurrently.{0,160}believes the first worker is gone" <<<"$ctrl_dispatch_flat"'

# ---------------------------------------------------------------------------
# The four build-profile wrappers (change 0167; retiered to four by change 0184)
# ---------------------------------------------------------------------------
fmv(){ awk 'NR==1 && $0=="---"{f=1;next} f && $0=="---"{exit} f{print}' "$1" \
        | sed -n "s/^$2:[[:space:]]*//p" | sed -n 1p | sed 's/[[:space:]]*$//'; }

# Change 0168 moved the shipped model/effort out of the wrapper frontmatter and into the
# harness-indexed sidecar, so the profile-ladder invariants are read from THERE. The wrapper
# sources are behavior-only templates now — fmv() over one for `model` is not merely stale, it is
# a first-match-anywhere read that would scan into the body and could return prose.
# shellcheck source=/dev/null
. "$REPO/scripts/lib/harness-defaults.sh"
HD="$REPO/agents/harness-defaults.yml"
assert "the shipped sidecar exists" '[ -f "$HD" ]'

# The ladder is a quadruple. Claude's axis is no longer effort alone: change 0184 dropped a genuinely
# cheaper MODEL onto the bottom rung, so neither "all efforts distinct" nor "all models identical"
# holds any more. The invariant that survives — and the one the codex block's header already argues —
# is that each rung is a distinct model/effort PAIR. A copy-paste that silently makes two rungs the
# same agent is exactly what this catches.
for p in economy:claude-sonnet-5:low standard:claude-opus-5:low premium:claude-opus-5:medium max:claude-opus-5:high; do
  name="${p%%:*}"; rest="${p#*:}"; want_model="${rest%%:*}"; want_effort="${rest##*:}"
  w="$REPO/agents/docket-build-$name.md"
  assert "profile $name: wrapper exists" '[ -f "$w" ]'
  [ -f "$w" ] || continue
  assert "profile $name: name field matches its filename" '[ "$(fmv "$w" name)" = "docket-build-'"$name"'" ]'
  assert "profile $name: shipped claude pin is $want_model/$want_effort" \
    '[ "$(hd_field "$HD" claude build-'"$name"' model)/$(hd_field "$HD" claude build-'"$name"' effort)" = "'"$want_model"'/'"$want_effort"'" ]'
  assert "profile $name: preloads the shared worker skill" \
    'grep -qF -- "docket-build-task" <<<"$(fmv "$w" skills)"'
  assert "profile $name: emits no maxTurns" '! grep -qiE "^maxTurns[[:space:]]*:" "$w"'
  # The source is a behavior-only template: it must carry NO pin of its own, or the sidecar is no
  # longer the single default store and the two can silently disagree.
  assert "profile $name: source carries no model:/effort: pin of its own" \
    '! grep -qE "^(model|effort):" "$w"'
done

# Pair distinctness, collected as raw values (not sort -u'd first) so a deleted entry collapses to a
# blank that a bare "all distinct" check would silently ignore: the non-vacuity half (exactly 4
# non-empty pairs) is asserted alongside the distinctness half.
pairs=""
for n in economy standard premium max; do
  pairs="$pairs $(hd_field "$HD" claude build-$n model)/$(hd_field "$HD" claude build-$n effort)"
done
assert "the four claude profiles are four DISTINCT model/effort pairs" \
  '[ "$(tr " " "\n" <<<"$pairs" | grep -c .)" = 4 ] && [ "$(tr " " "\n" <<<"$pairs" | grep . | sort -u | wc -l | tr -d " ")" = 4 ]'

# 0184's stated purpose on claude: the bottom rung is a genuinely cheaper MODEL, not merely a lower
# effort on the same one — the defect the change existed to fix ("economy never delivered a truly
# cheap floor"). Asserted as a difference, not as a literal ID, so retuning the pin does not redden.
assert "claude build-economy runs a different model from the rest of the ladder" \
  '[ -n "$(hd_field "$HD" claude build-economy model)" ] &&
   [ "$(hd_field "$HD" claude build-economy model)" != "$(hd_field "$HD" claude build-standard model)" ]'

# The compression claim: max INHERITS the pin that was the top rung before 0184, so the ladder
# gained no new headroom at the top — the savings come from the rungs below, not from spending more.
# NB `premium` is a live rung again post-rename and is now the THIRD of four, so "the pre-0184
# premium pin" would read as that rung's current pin; it means the pin of the old top rung, which
# `max` now holds.
assert "claude build-max holds the pre-0184 top-rung pin (claude-opus-5/high)" \
  '[ "$(hd_field "$HD" claude build-max model)/$(hd_field "$HD" claude build-max effort)" = "claude-opus-5/high" ]'

# The four-rung ladder must be COMPLETE and non-degenerate on every shipped harness — a rung
# missing on one harness is a build that silently falls back to that harness's own default
# mid-ladder, and two rungs sharing a pair is a copy-paste that quietly collapses the ladder.
# MODEL distinctness is the wrong assert in general: codex deliberately reuses one model at two
# efforts (sol/low for premium, sol/medium for max), so the model/effort PAIR is the role.
# Population derived from $HD_SHIPPED_HARNESSES so a newly shipped harness arms this for free.
n_lad=0
for h in $HD_SHIPPED_HARNESSES; do
  ladder=""
  for n in economy standard premium max; do
    assert "$h: build-$n carries a complete pair" \
      '[ -n "$(hd_field "$HD" '"$h"' build-'"$n"' model)" ] &&
       [ -n "$(hd_field "$HD" '"$h"' build-'"$n"' effort)" ]'
    ladder="$ladder $(hd_field "$HD" "$h" build-$n model)/$(hd_field "$HD" "$h" build-$n effort)"
  done
  assert "the four $h profiles are four DISTINCT model/effort pairs" \
    '[ "$(tr " " "\n" <<<"'"$ladder"'" | grep -c .)" = 4 ] &&
     [ "$(tr " " "\n" <<<"'"$ladder"'" | grep . | sort -u | wc -l | tr -d " ")" = 4 ]'
  n_lad=$((n_lad+1))
done
# Floor: a failed source leaves $HD_SHIPPED_HARNESSES empty and every loop above vacuous.
assert "the ladder invariant was checked on every shipped harness (got $n_lad)" '[ "$n_lad" -ge 4 ]'

# The IDs must NOT appear under agents.default in the example — Claude model IDs there would
# falsely present themselves as harness-portable (spec: "never the harness-neutral fallback").
EX="$REPO/.docket.example.yml"
default_blk="$(awk '/^#[[:space:]]*default:[[:space:]]*$/{inblk=1;next} inblk && /^#[[:space:]]{0,3}[a-z]/{inblk=0} inblk{print}' "$EX")"
assert "no build profile is documented under agents.default" \
  '! grep -qE "build-(economy|standard|premium|max)" <<<"$default_blk"'
assert "no retired profile name survives anywhere in the example" \
  '! grep -qE "build-(low|medium|high)" "$EX"'

# Non-vacuity companion: the shipped example has no commented `default:` block today, so
# $default_blk is normally the empty string and the assert above passes trivially — that must be
# an ASSERTED "nothing to check" state, not an unexamined silent pass that would stay green even
# if the awk extraction itself broke. Require either the slice is non-empty OR the file genuinely
# has no `default:` block opener at all (so a future extraction regression, with a real `default:`
# block present, has somewhere to redden). The positive half of the rule — that the three
# build-profile entries really do live under `claude:` — is covered by the mirror-equality loop in
# tests/test_docket_example_yml.sh.
default_hdr_count="$(grep -cE '^#[[:space:]]*default:[[:space:]]*$' "$EX")"
assert "agents.default guard is armed: slice is non-empty, or the file truly has no default: block" \
  '[ -n "$default_blk" ] || [ "$default_hdr_count" = 0 ]'

# ---------------------------------------------------------------------------
# Dogfood: this repo opts in, the shipped default does NOT change
# ---------------------------------------------------------------------------
DY="$REPO/.docket.yml"

# Extract the top-level `skills:`/`build:` blocks the SAME way the resolver's own
# yaml_block_body (scripts/docket-config.sh) does, so an assert here proves what the
# resolver would actually see. A same-file grep-anywhere for the indented leaf would stay
# green even if `skills:`/`build:` were renamed to `zskills:`/`zbuild:` — the resolver reads
# each leaf WITHIN its named block, so a renamed header means the opt-in silently stops
# resolving while a bare leaf-presence grep notices nothing.
dy_yaml_block_body(){  # dy_yaml_block_body <file> <top-level-key> -> child lines on stdout
  awk -v parent="$2" '
    { line=$0; sub(/[[:space:]]*#.*/, "", line) }
    line ~ ("^" parent "[[:space:]]*:[[:space:]]*$") { inblk=1; next }
    inblk && line ~ /^[^[:space:]]/ { inblk=0 }
    inblk { print }
  ' "$1"
}
skills_blk="$(dy_yaml_block_body "$DY" skills)"
build_blk="$(dy_yaml_block_body "$DY" build)"

# Non-vacuity companions: without these, a broken/renamed-header extraction silently returns
# an empty slice and the leaf assert below would never have anything to fail against.
# Change 0193 made docket-build the shipped default, so this repo no longer pins skills.build —
# it genuinely runs the default rather than an override that happens to match. Asserting the
# block is ABSENT is what detects a re-added pin silently reintroducing the duplication.
assert "repo no longer pins skills.build (it runs the shipped default)" '[ -z "$skills_blk" ]'

assert "repo's build: block extraction is non-vacuous" '[ -n "$build_blk" ]'
assert "repo pins build.checkpoint explicitly" \
  'grep -qE "^[[:space:]]+checkpoint:[[:space:]]+(true|false)[[:space:]]*$" <<<"$build_blk"'

# The SHIPPED cross-harness default is now docket-build (change 0193) — every repo gets the
# profile-routed build with no opt-in. Anchored on the resolver, which is what actually decides
# the default, and asserted in BOTH directions so a revert to SDD reddens here rather than
# silently restoring the retired opt-in posture.
build_default="$(grep -E 'SKILL_BUILD=|skill_role build' "$REPO/scripts/docket-config.sh")"
assert "resolver's build default line was located (non-vacuity anchor)" '[ -n "$build_default" ]'
assert "shipped skills.build default is docket-build" \
  'grep -qF -- "docket-build" <<<"$build_default"'
assert "shipped skills.build default is no longer superpowers SDD" \
  '! grep -qF -- "superpowers:subagent-driven-development" <<<"$build_default"'

# The knob is documented for users, not only implemented (config-knob-ship-end-to-end).
RM="$REPO/README.md"
rm_body="$(cat "$RM")"
assert "README documents the docket-build role" 'grep -qF -- "docket-build" <<<"$rm_body"'
assert "README documents the four profiles" \
  'grep -qF -- "docket-build-economy" <<<"$rm_body" && grep -qF -- "docket-build-max" <<<"$rm_body"'
assert "README documents build.checkpoint" 'grep -qF -- "build.checkpoint" <<<"$rm_body"'
assert "README says how to opt back into SDD" \
  'grep -qF -- "superpowers:subagent-driven-development" <<<"$rm_body"'
# Change 0168 moved this boundary once (the sidecar gained validated Cursor IDs, so the profiles
# stopped being Claude-only), change 0169 moved it again (the sidecar gained a complete Codex
# block, so Codex stopped being user-configured), and change 0192 moved it a third time (opencode
# joined as the fourth shipped harness). Both directions each time, because confirming only the new
# sentence would leave the falsified one undetected if both survived — and the retired claims are
# asserted ABSENT rather than deleted, so a revert of the README prose reddens here instead of
# silently restoring a false promise.
assert "README states the shipped-defaults boundary for the profiles" \
  'grep -qiE "docket-build[^.]{0,200}Claude Code, Cursor, Codex, and opencode" <<<"$rm_body"'
assert "README no longer stops the shipped-defaults boundary at Codex" \
  '! grep -qiE "docket-build[^.]{0,200}Claude Code, Cursor, and Codex\\." <<<"$rm_body"'
assert "README no longer claims the profiles are Claude-only" \
  '! grep -qiE "docket-build[^.]{0,200}(claude-only|Claude Code only|only.{0,20}Claude)" <<<"$rm_body"'
# The 0168-era pair, re-pointed again (0192): four shipped blocks, all complete. The superseded
# three-harness wording is asserted absent so a partial revert cannot leave a stale count standing.
assert "README says all four shipped harness blocks are complete" \
  'grep -qiE "all four are complete" <<<"$rm_body"'
assert "README no longer says only three shipped blocks are complete" \
  '! grep -qiE "all three are complete" <<<"$rm_body"'
assert "README no longer says Codex stays user-configured" \
  '! grep -qiE "Codex remains user-configured|until change 0169" <<<"$rm_body"'

# ---------------------------------------------------------------------------
# Change 0184: the clean break is repo-wide, and the historical record is exempt
# ---------------------------------------------------------------------------
# A per-file assert cannot see a surface nobody thought to list. This walks the LIVE tree and fails
# on any surviving profile token, with the exemption stated as a path filter rather than an
# allowlist of known files — an allowlist would be an enumerated floor that ages into the gap.
#
# Exempt by design: docs/changes/archive, docs/results, docs/superpowers, docs/adrs. Those record
# what was true when written; rewriting them would falsify the history. This plan's own file lives
# under docs/superpowers/plans and is exempt for the same reason.
live_hits="$(git -C "$REPO" grep -InE 'build-(low|medium|high)|docket-build-(low|medium|high)' -- \
  ':!docs/changes/archive' ':!docs/results' ':!docs/superpowers' ':!docs/adrs' ':!tests/test_docket_build.sh' || true)"
assert "no live surface names a retired build profile" '[ -z "$live_hits" ]'
[ -z "$live_hits" ] || printf '  live hits:\n%s\n' "$live_hits"

# The hyphenated guard above has a blind spot the rename walked straight into: a wrapper body says
# "routed to the LOW profile" and a dispatch fragment says "Profile: low" — BARE tokens, which
# `build-(low|medium|high)` cannot see. The bare-word ban earlier in this file does cover that
# shape, but only across $CTRL and $WORKER, and the wrappers are neither. Eleven stale references
# across six files survived both guards and the full suite stayed green.
#
# Sound as a BARE-word ban for the same reason it is sound on the two contracts: a wrapper source
# carries no effort field (pins live in the harness-indexed sidecar, resolved by sync-agents.sh),
# and no non-build wrapper or fragment uses these words at all — verified across the whole glob,
# not just the build four. So any occurrence here is a profile token, and a stale one. Thirteen
# agents and thirteen fragments ship today; the floor is that same 26.
# Word boundaries are POSIX character classes, NOT `\b`: git grep -E is POSIX ERE, where `\b` is
# not a word-boundary escape and the pattern silently matches nothing. Written with `\b` first,
# this guard passed against the very defect it was added for — caught only because the mutation
# probe below failed to redden. The rest of this file's `\b` asserts run through PATH grep, which
# does support it; only the git-grep-based ones must avoid it.
wrapper_hits="$(git -C "$REPO" grep -IniE '(^|[^[:alnum:]_])(low|medium|high)([^[:alnum:]_]|$)' -- \
  'agents/docket-*.md' 'cursor-rules/dispatch/docket-*.md' || true)"
assert "no wrapper or dispatch fragment names a retired profile in prose" '[ -z "$wrapper_hits" ]'
[ -z "$wrapper_hits" ] || printf '  wrapper hits:\n%s\n' "$wrapper_hits"

# Non-vacuity: the glob must actually be finding files. A renamed directory or a changed wrapper
# extension would empty $wrapper_hits and report a green guard that reads nothing — the same
# failure shape this whole section exists to prevent.
wrapper_n="$(git -C "$REPO" grep -Il '' -- 'agents/docket-*.md' 'cursor-rules/dispatch/docket-*.md' | grep -c . || true)"
assert "wrapper prose guard is armed (>= 26 wrapper/fragment files in scope)" '[ "$wrapper_n" -ge 26 ]'

# Non-vacuity: the guard above swallows failure with `|| true`, so an empty $live_hits must be
# proved to mean "nothing left" rather than "the search broke". Run the SAME pathspec list against a
# pattern the exempt history is known to contain, and require that it finds the history and nothing
# outside it. Running an unfiltered pattern instead would be the wrong companion: a later edit that
# over-broadened an exemption (say ':!docs/superpowers' widened to ':!docs') would empty $live_hits
# while an unfiltered probe stayed happily green, reporting an armed guard that checks nothing.
armed_hits="$(git -C "$REPO" grep -IlE 'build-(low|medium|high)' -- \
  'docs/changes/archive' 'docs/results' 'docs/superpowers' 'docs/adrs' || true)"
assert "retirement grep is armed (the exempt history still contains the tokens)" \
  '[ -n "$armed_hits" ]'

# ---------------------------------------------------------------------------
# Change 0224: the gate's VERDICT is the exit status, never output text
# ---------------------------------------------------------------------------
# § The build gate defined what green and red MEAN but never what DETERMINES which one a run is,
# so a gate keyed on an output-shape match (`tail -1 | grep PASS`) satisfied the contract and could
# mint a valid-looking build-evidence record for a branch nobody verified. These asserts pin the
# determinant.
#
# Terminator is the NAMED heading /^### Gate execution posture$/, not a shape like /^#+ / or
# /^## /: the level-2 form would swallow the posture subsection (which
# tests/test_gate_execution_posture.sh separately owns) and let a bounded-gap assert match across
# sections and survive its own mutation, while /^#+ / is worse still — the guarded section carries
# fenced blocks, so any heading-SHAPED line at any level (a `#` comment inside a fenced example)
# would silently truncate the slice. A name cannot be spoofed by shape
# (learnings: section-slice-needs-a-named-terminator). Verified: the named form yields byte-for-byte
# the same slice the shape form did. The learning's other rule — assert the terminator EXISTS — is
# discharged just below rather than deferred to the sibling suite, so this suite alone can tell a
# renamed heading from a real one.
gate_blk="$(awk '/^## The build gate$/{f=1;next} f && /^### Gate execution posture$/{f=0} f' <<<"$ctrl_body")"
assert "0224: the named slice terminator heading exists" \
  'grep -qE "^### Gate execution posture$" <<<"$ctrl_body"'
# Flatten before phrase matching so a pure re-flow of the hard-wrapped paragraph does not redden
# asserts about policy that never changed (learnings: phrase-grep-over-wrapped-prose).
gate_flat="$(flat "$gate_blk")"

# Non-vacuity companion through the SAME extractor. Without it, a renamed heading or a broken awk
# range empties $gate_blk and turns every assert below into a permanent green. The anchor is a
# clause that PREDATES this change and sits inside the slice, so it cannot be satisfied by the
# prose this change just added (learnings: assert-detects-removal-not-replacement, rule 5).
assert "0224: the build gate section slice is non-vacuous" \
  '[ "$(grep -c . <<<"$gate_blk")" -ge 20 ]'
assert "0224: the build gate slice extractor still resolves (pre-existing clause present)" \
  'grep -qiF -- "configuration gap, not a red suite" <<<"$gate_flat"'

# (a) The iff — an exit status decides green. Anchored on a verbatim slice of the claim, never on a
# bare common noun like "exit" or "gate" (learnings: assert-detects-removal-not-replacement, #226).
assert "0224: green is defined as the suite command exiting zero" \
  'grep -qiF -- "green if and only if the resolved suite command exits zero" <<<"$gate_flat"'

# (b) The negative — output text is not the verdict. Two separate asserts rather than one ERE with
# two bounded gaps: stacked gaps backtrack catastrophically on NON-matching input, so the mutation
# test hangs instead of reddening (learnings: stacked-gap-regex-hangs-instead-of-failing).
assert "0224: output text is classified as diagnostic, not the verdict" \
  'grep -qiF -- "diagnostic only" <<<"$gate_flat"'
assert "0224: reading the verdict out of the output is named as not a gate" \
  'grep -qiF -- "reads its verdict out of the output is not a gate" <<<"$gate_flat"'

# (c) The verdict is read from the terminal result artifact — which is also where the definition of
# "completed successfully" that references/gate-execution.md capability 5 requires finally lands —
# and the two non-verdicts stay halts. One bounded gap per ERE, never two.
assert "0224: the deciding status is the one in the terminal result artifact" \
  'grep -qiE "deciding status[^.]{0,120}terminal result artifact" <<<"$gate_flat"'
# The DEFINITION, bound end to end: a bare presence check for the two-word phrase would stay green
# on a rewrite that merely mentions "completed successfully" in a diagnostic aside while the
# definitional link — the definiendum and the zero status that defines it — was gone.
assert "0224: completed successfully is defined as the artifact recording a zero status" \
  'grep -qiE "completed successfully[^.]{0,60}records a zero status" <<<"$gate_flat"'
# The non-verdict rule is a three-part claim — the two names, their classification, and the
# halt/red consequence — so it is three asserts with ONE gap each, never one ERE with two (stacked
# gaps backtrack catastrophically on non-matching input). Each name is bound to the classification
# on its own, and the classification to its consequence: an INVERTING rewrite that made the two
# names statuses the artifact may hold and moved "never red" onto some other subject would redden,
# where a pair of asserts pinning only name-adjacency and a subjectless consequence would not.
# No anchor spans the emphasis markers the prose puts around the two names (`*result unavailable*`).
assert "0224: still running is classified as not a verdict" \
  'grep -qiE "still running[^.]{0,60}are not verdicts" <<<"$gate_flat"'
assert "0224: result unavailable is classified as not a verdict" \
  'grep -qiE "result unavailable[^.]{0,40}are not verdicts" <<<"$gate_flat"'
assert "0224: the non-verdicts stay budget halts and are never red" \
  'grep -qiE "are not verdicts[^.]{0,120}never red" <<<"$gate_flat"'
# The symmetric half: green's determinant is fixed above, so red's must be too, or the section's only
# remaining branch is "manufacture a repair task" — reachable from a run this repo's own configured
# runner (scripts/run-tests.md) documents as having zero failing tests. Three asserts, one gap each.
assert "0224: not every non-zero status is red" \
  'grep -qiF -- "nor is every non-zero status red" <<<"$gate_flat"'
assert "0224: a runner-defined non-failure status is a halt, not red" \
  'grep -qiE "non-failure[^.]{0,60}halt per" <<<"$gate_flat"'
assert "0224: red is a completed run that is neither green nor one of those halts" \
  'grep -qiF -- "neither green nor one of those halts" <<<"$gate_flat"'

# (d) The per-file-loop aggregate. Confirmed against finalize's configured-bash-finalize block,
# which accumulates suite_status=1 per failing file and exits on [ "$suite_status" -eq 0 ] — the
# aggregate IS the status, so the wording holds unchanged.
assert "0224: under a per-file loop the aggregate is the deciding status" \
  'grep -qiE "loop over per-file commands[^.]{0,220}aggregate" <<<"$gate_flat"'

# (e) The rule binds the repair worker's post-fix re-run, whose green is what ends the ladder.
assert "0224: the rule binds the repair worker's post-fix re-run" \
  'grep -qiF -- "including the repair worker'"'"'s post-fix re-run" <<<"$gate_flat"'

# ---------------------------------------------------------------------------
# Change 0249 — the worker contract carries the gate-execution pointer and a
# staging rule.
#
# Two mechanisms of the change-0223 incident that change 0231 did not close. (1) The gate execution
# posture lives in docket-build's SKILL.md and its references/gate-execution.md — files a dispatched
# worker never loads, because a worker is dispatched with its task, not with its controller's
# contract — so workers running the full suite as their honest focused verification each re-invented
# background-the-suite-and-yield. (2) "## Scope" forbade EDITING unrelated work but said nothing
# about STAGING, so `git add -A` could sweep another agent's dirty paths into the worker's one
# commit, which is how the 0223 double-write started.
# ---------------------------------------------------------------------------

# The pointer lives in "## The cycle", beside step 4's focused-not-the-whole-suite note. Slice to
# that section rather than grepping the file: the reference path and the never-yield words would
# also match a future summary line or a frontmatter description, and a whole-file grep cannot
# observe the rule being removed from the section that owns it.
worker_cycle="$(awk '/^## The cycle$/{f=1;next} f&&/^## /{exit} f' <<<"$worker_body")"
worker_cycle_flat="$(flat "$worker_cycle")"

# Non-vacuity through the SAME extractor, anchored on a clause that PREDATES this change, so a
# renamed heading or a broken awk range reddens HERE instead of greening every assert below.
assert "0249: the worker ## The cycle section is extractable" \
  '[ -n "$worker_cycle" ] &&
   grep -qF -- "fails for the intended reason" <<<"$worker_cycle_flat"'

# (1a) The pointer names the reference file by path. -F because the path carries regex
# metacharacters, and the whole point is that the worker is sent to the harness-neutral capability
# file rather than to docket-build's controller-vocabulary posture section. Also pins that the
# named path actually resolves, so a future rename/move of gate-execution.md reddens here instead
# of leaving this guard green over a dangling pointer.
assert "0249: the cycle points at docket-build/references/gate-execution.md" \
  'grep -qF -- "docket-build/references/gate-execution.md" <<<"$worker_cycle_flat" &&
   [ -f "$REPO/skills/docket-build/references/gate-execution.md" ]'

# (1b) ...and the worker-shaped consequence, not the path alone: a rewrite that keeps the pointer
# while inverting the conduct must redden. Word-anchored so the negation cannot match inside
# "whenever" or "however".
assert "0249: the cycle forbids yielding to await the run" \
  'grep -qiE "\b(never|do not)\b[^.]{0,60}yield to await" <<<"$worker_cycle_flat"'

# (1c) The observation is BOUNDED. "never yield" plus "observe by blocking" without this clause
# converts the measured yield-and-stall failure into an unbounded silent block — the reference file
# states harness capabilities, not agent conduct, so nothing else in the pointer supplies the bound.
# A rewrite that deletes "keep the observation **finite**" leaves every other 0249 assert green, so
# this one is what detects its removal. One bounded gap spans the markdown emphasis on "finite" and
# binds it to the imperative, not to a bare mention of observing.
assert "0249: the cycle bounds the blocking observation as finite" \
  'grep -qiE "keep the observation[^.]{0,40}finite" <<<"$worker_cycle_flat"'

# (1d) Fail-closed, bound to its subject with ONE gap: it is the UNFINISHED run that is not green.
# A bare presence grep for "not green" survives a rewrite that keeps the words and drops the rule.
assert "0249: an unfinished run at the observation bound is not green" \
  'grep -qiE "unfinished[^.]{0,80}not green" <<<"$worker_cycle_flat"'

# (1e) ...and fail-closed names the OUTCOME to return. "## Outcomes" enumerates exactly three, and
# the controller halts on a NEEDS_ESCALATION carrying no capacity reason — so a clause that says
# what not to claim without saying what to return leaves the malformed escalation as the likely
# worker move. One bounded gap binds the failure posture to the outcome literal.
assert "0249: failing closed on an unfinished run returns BLOCKED" \
  'grep -qiE "fail closed[^.]{0,60}BLOCKED" <<<"$worker_cycle_flat"'

# (1f) The pointer's trigger is reconciled with step 4, whose rule is focused-not-the-whole-suite:
# the long run it addresses is the FOCUSED set, not a licence to run the suite. The clause also has
# to stay repo-neutral — this contract body ships into consuming repos, whose own instructions
# override it, so a hardcoded claim about docket's suite is false there. One bounded gap binds the
# outlasting run to step 4's set; the second grep rejects the repo-specific aside it replaced.
assert "0249: the long-run pointer is reconciled with step 4's focused set" \
  'grep -qiE "outlast[^.]{0,60}step 4.{0,20}focused set" <<<"$worker_cycle_flat" &&
   ! grep -qiF -- "on this repo" <<<"$worker_cycle_flat"'

# (2) The staging prohibition, scoped through the 0231 "## Scope" extractor above. Three asserts
# with ONE bounded gap each, never one ERE with three: stacked gaps backtrack catastrophically on
# non-matching input, so the mutation test hangs instead of reddening (learnings:
# stacked-gap-regex-hangs-instead-of-failing). The gap class excludes the colon that terminates the
# prohibition clause, so no gap can bind across the sentence into unrelated Scope prose.
assert "0249: Scope forbids git add -A" \
  'grep -qiE "\bnever\b[^:]{0,80}git add -A" <<<"$worker_scope_flat"'
assert "0249: Scope forbids git add ." \
  'grep -qiE "\bnever\b[^:]{0,80}git add \." <<<"$worker_scope_flat"'
assert "0249: Scope forbids git commit -a" \
  'grep -qiE "\bnever\b[^:]{0,80}git commit -a" <<<"$worker_scope_flat"'

# (3) The positive rule the three prohibitions implement. Without it the bullet is a list of banned
# spellings, and the next sweep idiom nobody enumerated walks straight through.
assert "0249: Scope states the explicit-path staging rule" \
  'grep -qiF -- "Stage by explicit path" <<<"$worker_scope_flat"'

# (4) The observability half — the part a lazy rewrite drops first — bound to what it displaces, so
# a rewrite that redefines the rule back onto `git status` diffing reddens.
assert "0249: what the task changed is defined by the task contract, not git status" \
  'grep -qiE "task contract, not[^.]{0,60}git status" <<<"$worker_scope_flat"'

# (4b) The action half of the same rule (spec assumption A6): the posture for a path the worker
# cannot attribute is leave-and-report, never stage and never clean up — cleaning is exactly the
# sweep this change forbids. Assert (4) above pins only the DEFINITION half, so a compression pass
# that keeps "task contract, not ... git status" while dropping "leave it in place and name it in
# NOTES" would leave the worker with no instruction for the path it just decided is not its own.
assert "0249: an unattributable path is left in place and reported, never swept" \
  'grep -qiE "cannot attribute[^.]{0,80}(leave it in place|NOTES)" <<<"$worker_scope_flat"'

# (5) The escalation carve-out, pinned in BOTH directions with one gap each. A single assert on the
# permissive half alone would stay green through a rewrite that re-licensed the sweep for exactly
# the worker most likely to be in a dirty shared tree — the first-draft wording the critic gate
# rejected. These are companions to, never replacements of, the 0231 pin
# "You may revise or replace them", which must stay green above.
assert "0249: an inherited path within the task boundary is staged normally" \
  'grep -qiE "within the task[^.]{0,80}staged normally" <<<"$worker_scope_flat"'
assert "0249: an inherited path outside the task boundary is not staged" \
  'grep -qiE "outside the task boundary[^.]{0,60}not staged" <<<"$worker_scope_flat"'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
