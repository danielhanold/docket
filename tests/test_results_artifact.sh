#!/usr/bin/env bash
# tests/test_results_artifact.sh — verifies the change-results-artifact convention.
# Run: bash tests/test_results_artifact.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

SKILLS=(docket-new-change docket-status docket-implement-next docket-finalize-change docket-adr)

# 1. The real convention blocks are byte-identical across all skills.
assert "convention blocks in sync (sync-convention.sh --check)" \
  'bash sync-convention.sh --check >/dev/null 2>&1'

# 2. The convention carries the results: manifest field in EVERY skill.
for s in "${SKILLS[@]}"; do
  assert "results: field present in $s" \
    'grep -q "^results:" "skills/'"$s"'/SKILL.md"'
done

# 3. The convention carries the results_dir knob + the docs/results layout entry in every skill.
for s in "${SKILLS[@]}"; do
  assert "results_dir knob present in $s" 'grep -q "results_dir" "skills/'"$s"'/SKILL.md"'
  assert "results_dir layout entry present in $s" 'grep -q "<results_dir>/" "skills/'"$s"'/SKILL.md"'
done

# 4. Branch-model line includes results.
assert "branch-model line mentions results" \
  'grep -q "plan + results + code" "skills/docket-new-change/SKILL.md"'

# 5. Templates.
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

# 6. Flow prose wired into the three skills.
assert "implement-next has a results close-out step" \
  'grep -qi "results close-out" skills/docket-implement-next/SKILL.md'
assert "status health check covers results: link" \
  'grep -q "results:" skills/docket-status/SKILL.md'
assert "finalize mentions appending to the results file" \
  'grep -qi "results" skills/docket-finalize-change/SKILL.md'

# 7. Design spec + README reconciled.
assert "design spec has results-artifact decision" \
  'grep -qi "results artifact" docs/superpowers/specs/2026-05-30-docket-design.md'
assert "README documents results_dir" 'grep -q "results_dir" README.md'

exit $fail
