#!/usr/bin/env bash
# tests/test_change_links_coverage.sh — every field-writing skill body must invoke the
# per-change link renderer (change 0035). Sentinel scan, mirroring test_render_board.sh's
# wiring sentinels. A sentinel is sampling, not parsing — pair with whole-branch review.
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
ok(){ printf 'ok - %s\n' "$1"; }
no(){ printf 'NOT OK - %s\n' "$1"; fail=1; }

# docket-finalize-change is handled separately below (migrated to the Go-v1 sequencer, 0316).
SKILLS=(
  docket-new-change docket-groom-next docket-auto-groom
)
for s in "${SKILLS[@]}"; do
  f="$ROOT/skills/$s/SKILL.md"
  if grep -qF 'docket.sh render-change-links' "$f"; then ok "$s invokes render-change-links (via the docket.sh facade)"; else no "$s invokes render-change-links (via the docket.sh facade)"; fi
done

# docket-status migrated to the Go-v1 sweep (change 0377 Task 9): the merge sweep archives through the
# typed close-out, which regenerates the per-change `## Artifacts` block atomically in the same metadata
# commit, so the sweep no longer invokes the legacy facade renderer. Same treatment as
# docket-implement-next / docket-finalize-change below (guarantee relocated, not dropped): assert the
# retired facade call is GONE, and floor it on the exceptional-drift repair path so it cannot go
# vacuously green (restatement-accumulates-its-own-guards; assert-detects-removal-not-replacement).
STATUS="$ROOT/skills/docket-status/SKILL.md"
if grep -qF 'docket.sh render-change-links' "$STATUS"; then no "docket-status no longer invokes the legacy render-change-links facade"; else ok "docket-status no longer invokes the legacy render-change-links facade"; fi
if grep -qF 'docket repository migrate' "$STATUS"; then ok "docket-status names the exceptional-drift artifact-links repair path"; else no "docket-status names the exceptional-drift artifact-links repair path"; fi

# docket-implement-next migrated to the Go-v1 transaction path (change 0315): its field writes go
# through `docket change` / attach / mark-implemented transactions that regenerate the per-change
# `## Artifacts` block ATOMICALLY in the same metadata commit, so the skill no longer invokes the
# legacy facade renderer. The guarantee is not dropped, only relocated (learnings:
# restatement-accumulates-its-own-guards): assert the transaction-owned artifact-block render, and
# — the mutation twin — that the retired facade call is GONE, so its reintroduction reddens.
IMPL="$ROOT/skills/docket-implement-next/SKILL.md"
if grep -qiE 'Artifacts. block[^.]{0,40}(same commit|atomic)|(regenerat|render)[^.]{0,40}Artifacts. block' "$IMPL"; then ok "docket-implement-next regenerates the Artifacts block inside its change transactions"; else no "docket-implement-next regenerates the Artifacts block inside its change transactions"; fi
if grep -qF 'docket.sh render-change-links' "$IMPL"; then no "docket-implement-next no longer invokes the legacy render-change-links facade"; else ok "docket-implement-next no longer invokes the legacy render-change-links facade"; fi

# docket-finalize-change migrated to the Go-v1 sequencer (change 0316): its close-out backlink leg
# is owned by `docket finalize closeout` (the `docket`-mode integration-ref leg patches the existing
# `docket:backlink` blocks), so the skill no longer invokes the legacy facade renderer. Same
# treatment as docket-implement-next above (Authority #2: finalize closeout owns the backlink leg):
# assert the closeout-owned backlink render, and — the mutation twin — that the retired facade call
# is GONE, so its reintroduction reddens.
FINCL="$ROOT/skills/docket-finalize-change/SKILL.md"
if grep -qF 'docket finalize closeout' "$FINCL" && grep -qiE 'backlink' "$FINCL"; then ok "docket-finalize-change renders its backlinks inside the Go closeout transaction"; else no "docket-finalize-change renders its backlinks inside the Go closeout transaction"; fi
if grep -qF 'docket.sh render-change-links' "$FINCL"; then no "docket-finalize-change no longer invokes the legacy render-change-links facade"; else ok "docket-finalize-change no longer invokes the legacy render-change-links facade"; fi

# The renderer script exists and is executable.
[ -x "$ROOT/scripts/render-change-links.sh" ] && ok "renderer script present + executable" || no "renderer script present + executable"

# The convention documents the generated block (sole-writer language anchored to the marker).
if grep -qF 'render-change-links.sh' "$ROOT/skills/docket-convention/SKILL.md"; then ok "convention names the renderer"; else no "convention names the renderer"; fi

exit $fail
