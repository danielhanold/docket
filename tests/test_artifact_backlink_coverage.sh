#!/usr/bin/env bash
# tests/test_artifact_backlink_coverage.sh — the skills/close-out that WRITE an artifact must invoke
# the back-link renderer (change 0136). Sentinel scan, anchored on the producer paragraphs, mirroring
# test_change_links_coverage.sh. A sentinel is sampling, not parsing — pair with whole-branch review.
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
ok(){ printf 'ok   - %s\n' "$1"; }
no(){ printf 'NOT OK - %s\n' "$1"; fail=1; }

# (1) The renderer script exists + is executable.
[ -x "$ROOT/scripts/render-artifact-backlink.sh" ] && ok "renderer script present + executable" || no "renderer script present + executable"

# (2) Every skill that WRITES a spec artifact invokes the renderer through the facade.
SPEC_SKILLS=( docket-new-change docket-groom-next docket-auto-groom )
for s in "${SPEC_SKILLS[@]}"; do
  f="$ROOT/skills/$s/SKILL.md"
  if grep -qF 'docket.sh render-artifact-backlink' "$f"; then ok "$s stamps the spec back-link"; else no "$s stamps the spec back-link"; fi
done

# (3) docket-implement-next stamps plan (§4) and results (§6.5) on disk, and adds a PR-body back-link (§7).
impl="$ROOT/skills/docket-implement-next/SKILL.md"
if grep -qF 'docket.sh render-artifact-backlink' "$impl"; then ok "docket-implement-next stamps plan/results back-links"; else no "docket-implement-next stamps plan/results back-links"; fi
if grep -qiE 'PR[ -]body back-link|back-link line' "$impl"; then ok "docket-implement-next adds a PR-body back-link"; else no "docket-implement-next adds a PR-body back-link"; fi

# (4) The terminal close-out re-renders the spec back-link at close-out (producer paragraph).
tco="$ROOT/skills/docket-convention/references/terminal-close-out.md"
if grep -qF 'docket.sh render-artifact-backlink' "$tco"; then ok "close-out re-renders the spec back-link"; else no "close-out re-renders the spec back-link"; fi

# (5) The convention names the renderer in the derived-view script family.
if grep -qF 'render-artifact-backlink.sh' "$ROOT/skills/docket-convention/SKILL.md"; then ok "convention names the back-link renderer"; else no "convention names the back-link renderer"; fi

exit $fail
