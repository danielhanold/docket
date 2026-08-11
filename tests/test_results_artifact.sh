#!/usr/bin/env bash
# tests/test_results_artifact.sh — verifies the change-results-artifact convention.
# Run: bash tests/test_results_artifact.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

# 1. The convention carries the results: manifest field (single-sourced in docket-convention).
assert "results: field present in the convention" \
  'grep -q "^results:" skills/docket-convention/SKILL.md'

# 2. The convention carries the results_dir knob + the docs/results layout entry (single-sourced in docket-convention).
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
# Change 0145 removed skills/docket-status/SKILL.md's restatement of the check-id vocabulary
# (including this explanation) in favor of a pointer to scripts/board-checks.md, the authoritative
# source. Assert the reasoning lives there instead of pinning prose SKILL.md no longer carries.
assert "status health check covers results: link (owned by scripts/board-checks.md after change 0145)" \
  'grep -q "still live on the unmerged feature" scripts/board-checks.md'
assert "finalize mentions appending to the results file" \
  'grep -q "append interactive-verification" skills/docket-finalize-change/SKILL.md'

# 5b. The merged-artifact freeze clause (change 0200). It is normative prose on the always-loaded
# Step-0 surface and nothing else pins it, so a slim or a size-budget squeeze could delete it
# silently. Owned here because this file owns the results-artifact convention end-to-end — its
# field, its template, and its close-out/finalize lifecycle; the freeze rule IS that lifecycle's
# terminal state. (test_artifact_backlink_coverage.sh is a producer-coverage sentinel over which
# skills invoke the renderer, not a statement about what merged artifacts may become.)
# The clause is wrapped prose, so collapse whitespace to one line first — otherwise a pure re-flow
# reddens the assert. Each slice binds the phrase to the claim it makes, not to a bare noun.
CONV_FLAT="$(tr '\n' ' ' < skills/docket-convention/SKILL.md | tr -s ' ')"
assert "convention freezes merged plans + results against hand-editing" \
  'grep -qF -- "**Merged plans and results are frozen build records.** Once a change'"'"'s PR merges, its \`plan:\` and \`results:\` files are never hand-edited again" <<<"$CONV_FLAT"'
assert "freeze clause carves out only the generated backlink re-stamp" \
  'grep -qF -- "The one writer allowed to touch them afterward is \`render-artifact-backlink.sh\`, re-stamping the generated \`docket:backlink\` block at terminal publish; authored content never changes." <<<"$CONV_FLAT"'
assert "freeze clause routes corrections into a new change" \
  'grep -qF -- "Corrections go in a new change, never in the merged artifact." <<<"$CONV_FLAT"'

# 6. Design spec + README reconciled.
assert "design spec has results-artifact decision" \
  'grep -qi "results artifact" docs/superpowers/specs/2026-05-30-docket-design.md'
assert "README documents results_dir" 'grep -q "results_dir" README.md'

exit $fail
