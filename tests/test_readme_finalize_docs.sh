#!/usr/bin/env bash
# tests/test_readme_finalize_docs.sh — doc-sentinel for the finalize/merge documentation
# (change 0095). Guards that README documents (a) the Claude Code auto-mode classifier
# behavior as the reason the bot-approval approach failed, (b) the single-maintainer
# branch-protection recipe, and (c) the preserved human-approval path for repos that
# require reviews. Run: bash tests/test_readme_finalize_docs.sh
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
RM="$ROOT/README.md"
FIN="$ROOT/skills/docket-finalize-change/SKILL.md"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

assert "README exists" '[ -f "$RM" ]'

# (a) the classifier behavior — named, and tied to what it blocks
assert "documents the auto-mode classifier" \
  'grep -qi "auto-mode classifier" "$RM"'
assert "names the soft-deny the classifier applies" \
  'grep -qi "soft.deny\|soft deny" "$RM"'
assert "states an allow-rule cannot clear a soft-deny" \
  'grep -qi "permissions.allow" "$RM" && grep -Eqi "cannot.*clear" "$RM"'
assert "scopes the observation to mode and version" \
  'grep -qi "scoped to the harness" "$RM" && grep -q "\*\*version\*\*" "$RM"'

# (b) the single-maintainer branch-protection recipe
assert "documents the branch-protection recipe" \
  'grep -qi "configure branch protection" "$RM"'
assert "names the zero-approvals recipe" \
  'grep -Eq "required_approving_review_count: 0|zero[^a-zA-Z]*approvals|0 approvals" "$RM"'
assert "states the merge needs no --admin" \
  'grep -Eqi "without .*--admin|no .*--admin" "$RM"'

# (c) the preserved human-approval path
assert "documents the human-approval path for approval-required repos" \
  'grep -q "require_pr_approval" "$RM" && grep -q "APPROVED" "$RM"'

# (d) the fork-exclusion reason for docket-finalize-change (change 0087). ADR-0043 is what
# UNBLOCKED headless merge, so citing it as the reason merge is blocked inverts it. The real
# reason the skill stays unforked is that it retains prompts a fork has no channel for.
assert "ties the finalize fork-exclusion to its interactive prompts" \
  'grep -q "Fork-exclusion principle" "$RM" &&
   grep -Eqi "docket-finalize-change.{0,120}(batch confirmation.{0,80}sign-off|sign-off.{0,80}batch confirmation)" "$RM"'
assert "no stale claim that finalize's headless merge is classifier-blocked" \
  '! grep -Eqi "Merge-Without-Review|headless merge is blocked" "$RM"'

# negative: the retired subsystem must not come back as live documentation
assert "no live auto-approve subsystem reference" \
  '! grep -Eqi "auto_approve|setup-auto-approve|auto-approve-setup.md|docket-approve.yml" "$RM"'
assert "the deleted setup guide is not linked" \
  '[ ! -f "$ROOT/docs/auto-approve-setup.md" ]'

# (e) configured-Bash boundary for finalize's local suite (change 0132). The
# executable fixture in test_configured_bash_finalize.sh proves both branches;
# these sharp doc sentinels keep the user-facing contract explicit.
assert "auto-detected shell tests use the configured Bash runtime" \
  'grep -qF -- '"'"'"$DOCKET_BASH_PATH" "$test"'"'"' "$FIN"'
assert "explicit finalize command is evaluated without an interpreter prefix" \
  'grep -qF -- '"'"'eval "$FINALIZE_TEST_COMMAND"'"'"' "$FIN"'
assert "explicit finalize command retains DOCKET_BASH_PATH in its environment" \
  'grep -Eqi "FINALIZE_TEST_COMMAND.{0,180}(exported|environment).{0,80}DOCKET_BASH_PATH|DOCKET_BASH_PATH.{0,180}(environment).{0,80}FINALIZE_TEST_COMMAND" "$FIN"'

exit $fail
