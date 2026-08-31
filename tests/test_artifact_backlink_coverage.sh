#!/usr/bin/env bash
# tests/test_artifact_backlink_coverage.sh — the skills/close-out that WRITE an artifact must invoke
# the back-link renderer (change 0136). Sentinel scan, anchored on the producer paragraphs, mirroring
# test_change_links_coverage.sh. A sentinel is sampling, not parsing — pair with whole-branch review.
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
ok(){ printf 'ok - %s\n' "$1"; }
no(){ printf 'NOT OK - %s\n' "$1"; fail=1; }

# (1) The renderer script exists + is executable.
[ -x "$ROOT/scripts/render-artifact-backlink.sh" ] && ok "renderer script present + executable" || no "renderer script present + executable"

# (2) Every skill that WRITES a spec artifact stamps the back-link. Change 0369 migrated these three
# from the legacy `docket.sh render-artifact-backlink` facade to the Go-v1 `docket artifact backlink`
# command; change 0377 Task 11 then ABSORBED that stamp into the atomic `docket change create` /
# `docket change groom` transactions — the spec exit submits authored Markdown through
# `docket change groom`, which writes the spec file AND stamps its reciprocal `docket:backlink` block
# in the SAME metadata commit, so the standalone `docket artifact backlink` call is retired here too.
# Coverage is RELOCATED onto the atomic groom transaction, never dropped (learnings:
# restatement-accumulates-its-own-guards). The absence asserts detect removal-not-replacement: BOTH
# retired spellings (the facade renderer AND the standalone Go backlink call) must be GONE.
SPEC_SKILLS=( docket-new-change docket-groom-next docket-auto-groom )
for s in "${SPEC_SKILLS[@]}"; do
  f="$ROOT/skills/$s/SKILL.md"
  if grep -qF 'docket change groom' "$f"; then ok "$s stamps the spec back-link inside the atomic docket change groom transaction"; else no "$s stamps the spec back-link inside the atomic docket change groom transaction"; fi
  if grep -E -e 'docket\.sh[[:space:]]+render-artifact-backlink' "$f" >/dev/null; then no "$s retired the render-artifact-backlink facade spelling"; else ok "$s retired the render-artifact-backlink facade spelling"; fi
done

# (3) docket-implement-next stamps plan (§4, via the plan-writer child) and results (§6.5) on disk,
# and adds a PR-body back-link (§7). Change 0315 migrated the on-disk stamp from the legacy
# `docket.sh render-artifact-backlink` facade to the Go-v1 `docket artifact backlink` command; the
# coverage is relocated onto that spelling, never dropped (learnings:
# restatement-accumulates-its-own-guards).
impl="$ROOT/skills/docket-implement-next/SKILL.md"
if grep -qF 'docket artifact backlink' "$impl"; then ok "docket-implement-next stamps plan/results back-links (via docket artifact backlink)"; else no "docket-implement-next stamps plan/results back-links (via docket artifact backlink)"; fi
if grep -qiE 'PR[ -]body back-link|back-link line' "$impl"; then ok "docket-implement-next adds a PR-body back-link"; else no "docket-implement-next adds a PR-body back-link"; fi

# (4) The terminal close-out re-renders the spec back-link at close-out (producer paragraph).
# Change 0369 + 0377 Task 10: BOTH terminal paths now restamp the spec back-link atomically inside
# their step-1 transaction — the DONE path via `docket finalize closeout` (proven by
# internal/app/finalize_closeout_test.go's TestCloseoutBacklinkLegDocketMode), and the KILL path via
# `docket change kill` (which retargets the linked spec's backlink in its one transaction; Task 10
# retired the separate `docket artifact backlink` kill step). Coverage is RELOCATED onto the atomic
# owners, never dropped; the retired facade spelling must be GONE (learnings:
# assert-detects-removal-not-replacement).
tco="$ROOT/skills/docket-convention/references/terminal-close-out.md"
if grep -qF 'docket change kill' "$tco"; then ok "close-out restamps the kill-path spec back-link atomically (via docket change kill)"; else no "close-out restamps the kill-path spec back-link atomically (via docket change kill)"; fi
if grep -qF 'docket finalize closeout' "$tco"; then ok "close-out restamps the done-path spec back-link atomically (via docket finalize closeout)"; else no "close-out restamps the done-path spec back-link atomically (via docket finalize closeout)"; fi
if grep -E -e 'docket\.sh[[:space:]]+render-artifact-backlink' "$tco" >/dev/null; then no "close-out retired the render-artifact-backlink facade spelling"; else ok "close-out retired the render-artifact-backlink facade spelling"; fi

# (5) The convention names the renderer in the derived-view script family.
if grep -qF 'render-artifact-backlink.sh' "$ROOT/skills/docket-convention/SKILL.md"; then ok "convention names the back-link renderer"; else no "convention names the back-link renderer"; fi

exit $fail
