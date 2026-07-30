#!/usr/bin/env bash
# tests/test_docket_build.sh — change 0167. Contract guards for docket's own build role:
# the docket-build controller skill and the docket-build-task worker skill.
# Guards are keyed on the load-bearing CLAUSES of each contract, so a rewrite that keeps the
# rule stays green while a rewrite that drops the rule reddens. Run: bash tests/test_docket_build.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
WORKER="$REPO/skills/docket-build-task/SKILL.md"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# ---------------------------------------------------------------------------
# docket-build-task — the worker contract
# ---------------------------------------------------------------------------
assert "worker: SKILL.md exists" '[ -f "$WORKER" ]'
worker_body="$(cat "$WORKER" 2>/dev/null)"

# Non-vacuity floor: every negative/shape assert below reads $worker_body, so an empty or
# unreadable file must redden HERE rather than passing every grep by default.
assert "worker: contract is non-vacuous (>= 40 lines)" \
  '[ "$(printf "%s\n" "$worker_body" | grep -c .)" -ge 40 ]'

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

# METADATA BOUNDARY (whole-branch review, finding 3). The three profile wrappers deliberately do
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
# docket-build — the controller contract
# ---------------------------------------------------------------------------
CTRL="$REPO/skills/docket-build/SKILL.md"
assert "controller: SKILL.md exists" '[ -f "$CTRL" ]'
ctrl_body="$(cat "$CTRL" 2>/dev/null)"
assert "controller: contract is non-vacuous (>= 50 lines)" \
  '[ "$(printf "%s\n" "$ctrl_body" | grep -c .)" -ge 50 ]'

# It must dispatch by AGENT NAME — the whole point of the change is that model and effort are
# properties of a named agent rather than an ad-hoc per-dispatch argument.
for a in docket-build-economy docket-build-standard docket-build-premium; do
  assert "controller: names the $a agent" 'grep -qF -- "'"$a"'" <<<"$ctrl_body"'
done

# The routing rubric, with its deliberate asymmetry. The economy/standard rubric bullets are
# anchored on the same "^- **`token`**" structural idiom this file already uses for the worker's
# outcome bullets, since the prose disjunctions they replace were absorbed by the `## Routing`
# summary sentence — deleting the operative bullet reddened nothing (fix round 2, finding 2).
assert "controller: economy must be POSITIVELY established" \
  'grep -qE "^- \*\*\`economy\`\*\* \*only when\*" <<<"$ctrl_body"'
assert "controller: named risk selects premium" \
  'grep -qiE "premium[^.]{0,200}(authentication|security boundar)" <<<"$ctrl_body"'
assert "controller: uncertainty defaults to standard" \
  'grep -qE "^- \*\*\`standard\`\*\* for everything remaining" <<<"$ctrl_body"'

# The plan override and its fail-loud contract.
assert "controller: honors an explicit plan Build profile override" \
  'grep -qF -- "Build profile:" <<<"$ctrl_body"'
assert "controller: an invalid explicit profile HALTS rather than falling back" \
  'grep -qiE "invalid[^.]{0,120}halt" <<<"$ctrl_body"'

# The escalation ladder — all three edges, including the terminal one. Each is anchored on its
# "initial <profile>" prefix (the ladder fence's defining shape), not bare "<profile>", since the
# build gate's repair-ladder literal "standard -> premium -> halt" decoy-matches any assert keyed
# on bare profile-name-then-arrow text — proven for all three edges by mutation (deleting the
# whole ladder fence still left bare-anchored asserts green; fix round 1 caught it for the premium
# edge only, fix round 2 applied the same anchor to economy and standard).
assert "controller: economy escalates to standard" \
  'grep -qiE "initial economy[^.]{0,40}(->|→|to)[^.]{0,20}standard" <<<"$ctrl_body"'
assert "controller: standard escalates to premium" \
  'grep -qiE "initial standard[^.]{0,40}(->|→|to)[^.]{0,20}premium" <<<"$ctrl_body"'
assert "controller: premium escalation halts" \
  'grep -qiE "initial premium[^.]{0,20}(->|→|to)?[^.]{0,20}halt" <<<"$ctrl_body"'
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
assert "controller: repair ladder is standard -> premium -> halt" \
  'grep -qiE "standard[^.]{0,60}premium[^.]{0,60}halt" <<<"$ctrl_body"'

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
still red after the premium repair|still red after the premium repair
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
# which the two-branch gate read as RED — manufacturing a repair task and burning standard ->
# premium -> halt on a configuration problem. Keyed on the classification itself, not on the word
# "halt", since a rewrite that keeps the halt but drops the classification re-opens the mis-routing.
assert "controller: an undetectable suite is a configuration gap, not a red suite" \
  'grep -qiF -- "configuration gap, not a red suite" <<<"$ctrl_body" && grep -qF -- "finalize.test_command" <<<"$ctrl_body"'

# Finding 5's in-place rule: a failed attempt that left a COMMIT (not merely a dirty tree) cancels
# the escalation, because the escalated worker is separately forbidden to rewrite earlier task
# commits and the exactly-one-commit accounting is already contaminated.
assert "controller: does not escalate onto a commit left by a failed attempt" \
  'grep -qiE "\b(do not|does not|never)\b escalate onto a stray commit" <<<"$ctrl_body"'

# ---------------------------------------------------------------------------
# The three Claude build-profile wrappers (change 0167)
# ---------------------------------------------------------------------------
fmv(){ awk 'NR==1 && $0=="---"{f=1;next} f && $0=="---"{exit} f{print}' "$1" \
        | sed -n "s/^$2:[[:space:]]*//p" | head -n1 | sed 's/[[:space:]]*$//'; }

# The ladder is a triple, and effort is the ONLY thing that differs. Asserting the efforts
# pairwise-distinct is what stops a copy-paste that silently makes all three the same agent.
efforts=""
for p in economy:low standard:medium premium:high; do
  name="${p%%:*}"; want="${p##*:}"
  w="$REPO/agents/docket-build-$name.md"
  assert "profile $name: wrapper exists" '[ -f "$w" ]'
  [ -f "$w" ] || continue
  assert "profile $name: name field matches its filename" '[ "$(fmv "$w" name)" = "docket-build-'"$name"'" ]'
  assert "profile $name: effort is $want" '[ "$(fmv "$w" effort)" = "'"$want"'" ]'
  assert "profile $name: model is set" '[ -n "$(fmv "$w" model)" ]'
  assert "profile $name: preloads the shared worker skill" \
    'grep -qF -- "docket-build-task" <<<"$(fmv "$w" skills)"'
  assert "profile $name: emits no maxTurns" '! grep -qiE "^maxTurns[[:space:]]*:" "$w"'
  efforts="$efforts $(fmv "$w" effort)"
done
assert "the three profiles carry three DISTINCT efforts" \
  '[ "$(tr " " "\n" <<<"$efforts" | grep -c .)" = 3 ] && [ "$(tr " " "\n" <<<"$efforts" | grep -c . )" = "$(tr " " "\n" <<<"$efforts" | grep . | sort -u | wc -l | tr -d " ")" ]'

# All three share one model — the profile axis is effort, not model. If a future change
# deliberately splits models, this assert is the place that must be updated consciously.
# Collected as three raw values (not sort -u'd away first): a deleted model: line collapses to a
# blank that a bare "one distinct value" check would silently ignore, so the non-vacuity half
# (exactly 3 non-empty values) is asserted alongside the one-value half, same shape as the
# efforts DISTINCT-count assert above.
models=""
for n in economy standard premium; do models="$models $(fmv "$REPO/agents/docket-build-$n.md" model)"; done
assert "the three profiles share one model" \
  '[ "$(tr " " "\n" <<<"$models" | grep -c .)" = 3 ] && [ "$(tr " " "\n" <<<"$models" | grep . | sort -u | wc -l | tr -d " ")" = 1 ]'

# The IDs must NOT appear under agents.default in the example — Claude model IDs there would
# falsely present themselves as harness-portable (spec: "never the harness-neutral fallback").
EX="$REPO/.docket.example.yml"
default_blk="$(awk '/^#[[:space:]]*default:[[:space:]]*$/{inblk=1;next} inblk && /^#[[:space:]]{0,3}[a-z]/{inblk=0} inblk{print}' "$EX")"
assert "no build profile is documented under agents.default" \
  '! grep -qE "build-(economy|standard|premium)" <<<"$default_blk"'

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
assert "repo's skills: block extraction is non-vacuous" '[ -n "$skills_blk" ]'
assert "repo opts skills.build in to docket-build" \
  'grep -qE "^[[:space:]]+build:[[:space:]]+docket-build[[:space:]]*$" <<<"$skills_blk"'

assert "repo's build: block extraction is non-vacuous" '[ -n "$build_blk" ]'
assert "repo pins build.checkpoint explicitly" \
  'grep -qE "^[[:space:]]+checkpoint:[[:space:]]+(true|false)[[:space:]]*$" <<<"$build_blk"'

# The SHIPPED cross-harness default must stay SDD — the opt-in is this repo's, not everyone's.
# Anchored on the resolver, which is what actually decides the default.
sdd_default="$(grep -E 'SKILL_BUILD=|skill_role build' "$REPO/scripts/docket-config.sh")"
assert "shipped skills.build default is still superpowers SDD" \
  'grep -qF -- "superpowers:subagent-driven-development" <<<"$sdd_default"'

# The knob is documented for users, not only implemented (config-knob-ship-end-to-end).
RM="$REPO/README.md"
rm_body="$(cat "$RM")"
assert "README documents the docket-build role" 'grep -qF -- "docket-build" <<<"$rm_body"'
assert "README documents the three profiles" \
  'grep -qF -- "economy" <<<"$rm_body" && grep -qF -- "premium" <<<"$rm_body"'
assert "README documents build.checkpoint" 'grep -qF -- "build.checkpoint" <<<"$rm_body"'
assert "README says how to opt back into SDD" \
  'grep -qF -- "superpowers:subagent-driven-development" <<<"$rm_body"'
assert "README states the Claude-only support boundary for the profiles" \
  'grep -qiE "docket-build[^.]{0,200}(claude-only|Claude Code only|only.{0,20}Claude)" <<<"$rm_body"'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
