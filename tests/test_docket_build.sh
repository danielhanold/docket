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

# An escalated worker inherits the worktree — it must account for uncommitted changes.
assert "worker: escalated worker must not blindly discard existing uncommitted work" \
  'grep -qiE "uncommitted" <<<"$worker_body"'

# Repository instructions outrank this generic contract.
assert "worker: repository instructions override the generic contract" \
  'grep -qF -- "AGENTS.md" <<<"$worker_body"'

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
assert "controller: reads BUILD_CHECKPOINT" \
  'grep -qF -- "from the Step-0 config export" <<<"$ctrl_body"'
assert "controller: names the ledger path" \
  'grep -qF -- ".superpowers/docket-build/<change-id>/progress.md" <<<"$ctrl_body"'
assert "controller: skips a resumed task only on COMPLETE + plan hash + ancestor commit" \
  'grep -qiE "ancestor" <<<"$ctrl_body"'

# Tier C: an un-dispatchable build halts unless the human explicitly configured auto.
assert "controller: un-dispatchable profile routing halts (Tier C)" \
  'grep -qiE "Tier C" <<<"$ctrl_body"'
assert "controller: cites the convention's dispatch-capability resolution" \
  'grep -qiF -- "Dispatch-capability resolution" <<<"$ctrl_body"'
assert "controller: forbids concluding unavailability from a tool name" \
  'grep -qF -- "never from a tool name" <<<"$ctrl_body"'

# A malformed worker return is never read as success.
assert "controller: a missing or malformed outcome halts" \
  'grep -qiE "(missing or malformed|malformed)[^.]{0,60}halt" <<<"$ctrl_body"'
assert "controller: never infers success from a child reporting it finished" \
  'grep -qiE "never infer" <<<"$ctrl_body"'

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

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
