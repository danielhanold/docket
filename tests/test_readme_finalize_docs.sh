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
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

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

# (e) configured-Bash boundary — RETIRED (0316, category (a)). Change 0132 gave the finalize skill a
# published configured-Bash shell-test invocation (`"$DOCKET_BASH_PATH" "$test"`, an explicit
# `eval "$FINALIZE_TEST_COMMAND"`, DOCKET_BASH_PATH kept in the environment). Change 0316 removed it:
# the local gate is now COMPOSED into `docket finalize rebase`, which launches and observes a
# supervised run through `docket gate` — there is no shell-test fragment in the skill. This mirrors
# the whole-file inversion already landed in tests/test_configured_bash_finalize.sh. Authority #3
# (the skill states "The gate is composed into `finalize rebase`") + *Out of scope* (Bash fallback
# behavior). Inverted guards proving the configured-Bash shell-test prose stayed out, with a
# non-vacuity anchor. (Bash removal is change 0318's; the guards are inverted here, deleted there.)
# The literals are held in single-quoted variables so their `$DOCKET_BASH_PATH`/`$test` reach grep
# verbatim rather than expanding under `assert`'s eval.
BASH_TEST_LIT='"$DOCKET_BASH_PATH" "$test"'
EVAL_LIT='eval "$FINALIZE_TEST_COMMAND"'
assert "finalize SKILL is the Go sequencer (non-vacuity anchor)" 'grep -qF "docket finalize" "$FIN"'
assert "finalize carries no configured-Bash shell-test invocation" \
  '! grep -qF -- "$BASH_TEST_LIT" "$FIN"'
assert "finalize carries no explicit FINALIZE_TEST_COMMAND eval" \
  '! grep -qF -- "$EVAL_LIT" "$FIN"'
assert "finalize composes the local gate into the Go rebase verb instead" \
  'grep -Eqi "gate is composed into .finalize rebase|composed into .{0,3}docket .?finalize rebase" "$FIN"'

# (f) the runtime installer is shipped, not future work. Search current user-facing surfaces as a
# set so moving stale copy between files cannot evade the guard.
assert "runtime docs contain no pre-install future/manual-setup claims" \
  '! rg -qi --glob "*.md" --glob "!docs/superpowers/**" --glob "!docs/changes/**" --glob "!docs/results/**" "forthcoming installer|next installer slice|set (it )?manually until|currently contains no active keys|exec bash scripts/runners" "$ROOT/README.md" "$ROOT/scripts" "$ROOT/skills" "$ROOT/.docket.example.yml"'

exit $fail
