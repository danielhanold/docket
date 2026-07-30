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

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
