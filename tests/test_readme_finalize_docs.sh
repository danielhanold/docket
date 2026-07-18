#!/usr/bin/env bash
# tests/test_readme_finalize_docs.sh — doc-sentinel for the finalize/merge documentation
# (change 0095). Guards that README documents (a) the Claude Code auto-mode classifier
# behavior as the reason the bot-approval approach failed, (b) the single-maintainer
# branch-protection recipe, and (c) the preserved human-approval path for repos that
# require reviews. Run: bash tests/test_readme_finalize_docs.sh
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
RM="$ROOT/README.md"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

assert "README exists" '[ -f "$RM" ]'

# (a) the classifier behavior — named, and tied to what it blocks
assert "documents the auto-mode classifier" \
  'grep -qi "classifier" "$RM"'
assert "names the soft-deny the classifier applies" \
  'grep -qi "soft.deny\|soft deny" "$RM"'
assert "states an allow-rule cannot clear a soft-deny" \
  'grep -Eqi "permissions.allow|allow.rule" "$RM"'
assert "scopes the observation to mode and version" \
  'grep -Eqi "version.*(scoped|specific)|mode.*(scoped|specific)|headless" "$RM"'

# (b) the single-maintainer branch-protection recipe
assert "documents the branch-protection recipe" \
  'grep -qi "branch protection" "$RM"'
assert "names the zero-approvals recipe" \
  'grep -Eq "required_approving_review_count: 0|zero[^a-zA-Z]*approvals|0 approvals" "$RM"'
assert "states the merge needs no --admin" \
  'grep -Eqi "without .*--admin|no .*--admin" "$RM"'

# (c) the preserved human-approval path
assert "documents the human-approval path for approval-required repos" \
  'grep -q "require_pr_approval" "$RM" && grep -q "APPROVED" "$RM"'

# negative: the retired subsystem must not come back as live documentation
assert "no live auto-approve subsystem reference" \
  '! grep -Eqi "auto_approve|setup-auto-approve|auto-approve-setup.md|docket-approve.yml" "$RM"'
assert "the deleted setup guide is not linked" \
  '[ ! -f "$ROOT/docs/auto-approve-setup.md" ]'

exit $fail
