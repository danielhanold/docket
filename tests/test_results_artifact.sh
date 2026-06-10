#!/usr/bin/env bash
# tests/test_results_artifact.sh — verifies the change-results-artifact convention.
# Run: bash tests/test_results_artifact.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# 1. The convention carries the results: manifest field (single-sourced in docket-convention).
assert "results: field present in the convention" \
  'grep -q "^results:" skills/docket-convention/SKILL.md'

# 2. The convention carries the results_dir knob + the docs/results layout entry.
assert "results_dir knob present in the convention" 'grep -q "results_dir" skills/docket-convention/SKILL.md'
assert "results_dir layout entry present in the convention" 'grep -q "<results_dir>/" skills/docket-convention/SKILL.md'

# 3. Branch-model line includes results.
assert "branch-model line mentions results" \
  'grep -q "plan + results + code" "skills/docket-convention/SKILL.md"'

# 4. Templates.
assert "change-template has results: field" \
  'grep -q "^results:" skills/docket-new-change/change-template.md'
assert "results-template.md exists" \
  '[ -f skills/docket-implement-next/results-template.md ]'
assert "results-template has Verify (human) section" \
  'grep -q "## Verify (human)" skills/docket-implement-next/results-template.md'
assert "results-template has Findings section" \
  'grep -q "## Findings" skills/docket-implement-next/results-template.md'
assert "results-template has Follow-ups section" \
  'grep -q "## Follow-ups" skills/docket-implement-next/results-template.md'

# 5. Flow prose wired into the three skills.
assert "implement-next has a results close-out step" \
  'grep -qi "results close-out" skills/docket-implement-next/SKILL.md'
assert "status health check covers results: link" \
  'grep -q "those files legitimately still live on the unmerged" skills/docket-status/SKILL.md'
assert "finalize mentions appending to the results file" \
  'grep -q "append interactive-verification" skills/docket-finalize-change/SKILL.md'

# 6. Design spec + README reconciled.
assert "design spec has results-artifact decision" \
  'grep -qi "results artifact" docs/superpowers/specs/2026-05-30-docket-design.md'
assert "README documents results_dir" 'grep -q "results_dir" README.md'

exit $fail
