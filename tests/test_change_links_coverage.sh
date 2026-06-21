#!/usr/bin/env bash
# tests/test_change_links_coverage.sh — every field-writing skill body must invoke the
# per-change link renderer (change 0035). Sentinel scan, mirroring test_render_board.sh's
# wiring sentinels. A sentinel is sampling, not parsing — pair with whole-branch review.
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
ok(){ printf 'ok   - %s\n' "$1"; }
no(){ printf 'NOT OK - %s\n' "$1"; fail=1; }

SKILLS=(
  docket-new-change docket-groom-next docket-auto-groom
  docket-implement-next docket-finalize-change docket-status
)
for s in "${SKILLS[@]}"; do
  f="$ROOT/skills/$s/SKILL.md"
  if grep -qF 'scripts/render-change-links.sh' "$f"; then ok "$s invokes render-change-links.sh"; else no "$s invokes render-change-links.sh"; fi
done

# The renderer script exists and is executable.
[ -x "$ROOT/scripts/render-change-links.sh" ] && ok "renderer script present + executable" || no "renderer script present + executable"

# The convention documents the generated block (sole-writer language anchored to the marker).
if grep -qF 'render-change-links.sh' "$ROOT/skills/docket-convention/SKILL.md"; then ok "convention names the renderer"; else no "convention names the renderer"; fi

exit $fail
