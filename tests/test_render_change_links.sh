#!/usr/bin/env bash
# tests/test_render_change_links.sh — golden + idempotency + lifecycle + ADR + fallback tests
# for scripts/render-change-links.sh. Plain bash; hermetic fixtures; no network/gh.
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$ROOT/scripts/render-change-links.sh"
fail=0
ok(){ printf 'ok - %s\n' "$1"; }
no(){ printf 'NOT OK - %s\n' "$1"; fail=1; }

# A stub docket-config.sh: prints the export lines render-change-links.sh evals.
# METADATA_WORKTREE empty => ADR lookup falls back to ADRS_DIR (the fixture dir we pass via --adrs-dir).
make_config_stub(){ # $1 dir
  cat > "$1/docket-config.sh" <<'EOF'
#!/usr/bin/env bash
echo "METADATA_BRANCH=docket"
echo "INTEGRATION_BRANCH=main"
echo "ADRS_DIR=docs/adrs"
echo "CHANGES_DIR=docs/changes"
echo "METADATA_WORKTREE="
EOF
  chmod +x "$1/docket-config.sh"
}

# Render with hermetic config + explicit --repo (GitHub mode).
render(){ # $1 changefile ; extra args follow
  local cf="$1"; shift
  DOCKET_CONFIG="$tmp/docket-config.sh" GIT=git \
    bash "$SCRIPT" --change-file "$cf" --repo danielhanold/docket "$@"
}

tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
make_config_stub "$tmp"

# ---- Case A: spec + pr set, status in-progress (no plan/results yet) ----
cf="$tmp/0099-thing.md"
cat > "$cf" <<'EOF'
---
id: 99
slug: thing
status: in-progress
spec: docs/superpowers/specs/2026-06-21-thing-design.md
plan:
results:
branch: feat/thing
pr: https://github.com/danielhanold/docket/pull/44
adrs: []
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Motivation.
EOF

cat > "$tmp/golden_A.md" <<'EOF'
---
id: 99
slug: thing
status: in-progress
spec: docs/superpowers/specs/2026-06-21-thing-design.md
plan:
results:
branch: feat/thing
pr: https://github.com/danielhanold/docket/pull/44
adrs: []
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-06-21-thing-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-06-21-thing-design.md) |
| PR | [#44](https://github.com/danielhanold/docket/pull/44) |
<!-- docket:artifacts:end -->

## Why

Motivation.
EOF

render "$cf" >/dev/null 2>&1
if diff -u "$tmp/golden_A.md" "$cf" >/dev/null; then ok "A: spec+pr rows render between markers"; else no "A: spec+pr rows render between markers"; diff -u "$tmp/golden_A.md" "$cf" || true; fi

# ---- Case B: idempotency — second run is a byte-for-byte no-op ----
cp "$cf" "$tmp/after1.md"
render "$cf" >/dev/null 2>&1
if diff -u "$tmp/after1.md" "$cf" >/dev/null; then ok "B: second run is byte-identical"; else no "B: second run is byte-identical"; diff -u "$tmp/after1.md" "$cf" || true; fi

# ---- Case C: insertion when markers absent (first body section, after frontmatter, before ## Why) ----
cf2="$tmp/0098-nomark.md"
cat > "$cf2" <<'EOF'
---
id: 98
slug: nomark
status: proposed
spec: docs/superpowers/specs/2026-06-21-nomark-design.md
plan:
results:
branch:
pr:
adrs: []
---

## Why

Older change with no marker block.
EOF
render "$cf2" >/dev/null 2>&1
# Block must appear after the closing --- and before "## Why"; Spec row present.
if awk '/^---[[:space:]]*$/{n++} n==2 && /docket:artifacts:start/{seen_start=1} /^## Why/{ if(seen_start) print "OK"; exit }' "$cf2" | grep >/dev/null -x OK \
   && grep -qF '| Spec | [2026-06-21-nomark-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-06-21-nomark-design.md) |' "$cf2"; then
  ok "C: block inserted as first body section when markers absent"
else
  no "C: block inserted as first body section when markers absent"; sed -n '1,20p' "$cf2"
fi

# ---- Case D: empty block — no artifact fields set => heading+markers only, no table header ----
cf3="$tmp/0097-empty.md"
cat > "$cf3" <<'EOF'
---
id: 97
slug: empty
status: proposed
spec:
plan:
results:
branch:
pr:
adrs: []
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Fresh stub.
EOF
render "$cf3" >/dev/null 2>&1
if ! grep -qF '| Artifact | Link |' "$cf3" && grep -qF '<!-- docket:artifacts:start (generated — do not hand-edit) -->' "$cf3" && grep -qF '<!-- docket:artifacts:end -->' "$cf3"; then
  ok "D: empty block keeps markers, no table header"
else
  no "D: empty block keeps markers, no table header"; sed -n '1,20p' "$cf3"
fi

# ---- Case E: multiple ADRs, slug resolved from local adr files ----
mkdir -p "$tmp/adrs"
cat > "$tmp/adrs/0007-github-board-mirror-boundary.md" <<'EOF'
---
id: 7
slug: github-board-mirror-boundary
---
EOF
cat > "$tmp/adrs/0012-docket-status-script-vs-model-boundary.md" <<'EOF'
---
id: 12
slug: docket-status-script-vs-model-boundary
---
EOF
cf4="$tmp/0096-adrs.md"
cat > "$cf4" <<'EOF'
---
id: 96
slug: adrs
status: in-progress
spec:
plan:
results:
branch: feat/adrs
pr:
adrs: [7, 12]
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

x
EOF
render "$cf4" --adrs-dir "$tmp/adrs" >/dev/null 2>&1
expected_adr='| ADRs | [ADR-0007](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0007-github-board-mirror-boundary.md), [ADR-0012](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0012-docket-status-script-vs-model-boundary.md) |'
if grep -qF "$expected_adr" "$cf4"; then ok "E: multi-ADR row with resolved slugs"; else no "E: multi-ADR row with resolved slugs"; grep -F '| ADRs' "$cf4" || true; fi

# ---- Case F: missing ADR file => fallback to dir listing link ----
cf5="$tmp/0095-adrmiss.md"
cat > "$cf5" <<'EOF'
---
id: 95
slug: adrmiss
status: in-progress
spec:
plan:
results:
branch: feat/adrmiss
pr:
adrs: [999]
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

x
EOF
render "$cf5" --adrs-dir "$tmp/adrs" >/dev/null 2>&1
if grep -qF '| ADRs | [ADR-0999](https://github.com/danielhanold/docket/blob/docket/docs/adrs) |' "$cf5"; then ok "F: missing ADR falls back to dir listing"; else no "F: missing ADR falls back to dir listing"; grep -F '| ADRs' "$cf5" || true; fi

# ---- Case G: plan/results pinned to feature branch while in-progress ----
cf6="$tmp/0094-build.md"
cat > "$cf6" <<'EOF'
---
id: 94
slug: build
status: in-progress
spec:
plan: docs/superpowers/plans/2026-06-21-build.md
results: docs/results/2026-06-21-build-results.md
branch: feat/build
pr:
adrs: []
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

x
EOF
render "$cf6" >/dev/null 2>&1
if grep -qF '| Plan | [2026-06-21-build.md](https://github.com/danielhanold/docket/blob/feat/build/docs/superpowers/plans/2026-06-21-build.md) |' "$cf6" \
   && grep -qF '| Results | [2026-06-21-build-results.md](https://github.com/danielhanold/docket/blob/feat/build/docs/results/2026-06-21-build-results.md) |' "$cf6"; then
  ok "G: plan/results pinned to feature branch while in-progress"
else
  no "G: plan/results pinned to feature branch while in-progress"; grep -F '| Plan\|| Results' "$cf6" || true
fi

# ---- Case H: plan/results flip to integration branch once done ----
cf7="$tmp/0093-done.md"
cat > "$cf7" <<'EOF'
---
id: 93
slug: done
status: done
spec:
plan: docs/superpowers/plans/2026-06-21-done.md
results:
branch: feat/done
pr:
adrs: []
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

x
EOF
render "$cf7" >/dev/null 2>&1
if grep -qF '| Plan | [2026-06-21-done.md](https://github.com/danielhanold/docket/blob/main/docs/superpowers/plans/2026-06-21-done.md) |' "$cf7"; then
  ok "H: plan flips to integration branch at done"
else
  no "H: plan flips to integration branch at done"; grep -F '| Plan' "$cf7" || true
fi

# ---- Case I: killed-from-in-progress => plan row points at PR (pr present) ----
cf8="$tmp/0092-killed.md"
cat > "$cf8" <<'EOF'
---
id: 92
slug: killed
status: killed
spec:
plan: docs/superpowers/plans/2026-06-21-killed.md
results:
branch: feat/killed
pr: https://github.com/danielhanold/docket/pull/50
adrs: []
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

x
EOF
render "$cf8" >/dev/null 2>&1
if grep -qF '| Plan | [2026-06-21-killed.md](https://github.com/danielhanold/docket/pull/50) |' "$cf8"; then
  ok "I: killed plan row points at PR"
else
  no "I: killed plan row points at PR"; grep -F '| Plan' "$cf8" || true
fi

# ---- Case I2: killed with NO pr => plan/results rows omitted, block stays well-formed ----
cf8b="$tmp/0090-killednopr.md"
cat > "$cf8b" <<'EOF'
---
id: 90
slug: killednopr
status: killed
spec: docs/superpowers/specs/2026-06-21-knp-design.md
plan: docs/superpowers/plans/2026-06-21-knp.md
results:
branch: feat/knp
pr:
adrs: []
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

x
EOF
render "$cf8b" >/dev/null 2>&1
if ! grep -qF '| Plan |' "$cf8b" \
   && grep -qF '<!-- docket:artifacts:start (generated — do not hand-edit) -->' "$cf8b" \
   && grep -qF '<!-- docket:artifacts:end -->' "$cf8b" \
   && grep -qF '| Spec |' "$cf8b"; then
  ok "I2: killed-no-pr omits plan row, block well-formed"
else
  no "I2: killed-no-pr omits plan row, block well-formed"; sed -n '/## Artifacts/,/artifacts:end/p' "$cf8b"
fi

# ---- Case J: non-GitHub remote => bare code-formatted paths; PR stays a URL ----
cf9="$tmp/0091-fallback.md"
cat > "$cf9" <<'EOF'
---
id: 91
slug: fallback
status: in-progress
spec: docs/superpowers/specs/2026-06-21-fallback-design.md
plan:
results:
branch: feat/fallback
pr: https://example.com/pr/7
adrs: []
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

x
EOF
# No --repo, and a GIT mock that reports a non-github origin => fallback mode.
cat > "$tmp/git-nongh" <<'EOF'
#!/usr/bin/env bash
if [ "$1" = "-C" ]; then shift 2; fi
case "$1 $2" in "remote get-url") echo "git@gitlab.com:foo/bar.git" ;; *) exec git "$@" ;; esac
EOF
chmod +x "$tmp/git-nongh"
DOCKET_CONFIG="$tmp/docket-config.sh" GIT="$tmp/git-nongh" bash "$SCRIPT" --change-file "$cf9" >/dev/null 2>&1
if grep -qF '| Spec | `docs/superpowers/specs/2026-06-21-fallback-design.md` |' "$cf9" \
   && grep -qF '| PR | https://example.com/pr/7 |' "$cf9"; then
  ok "J: non-GitHub remote => bare paths, PR stays URL"
else
  no "J: non-GitHub remote => bare paths, PR stays URL"; sed -n '/Artifacts/,/artifacts:end/p' "$cf9"
fi

# ---- Case K: change-template.md ships the empty marker block as the first body section ----
tpl="$ROOT/skills/docket-new-change/change-template.md"
if grep -qF '## Artifacts' "$tpl" \
   && grep -qF '<!-- docket:artifacts:start (generated — do not hand-edit) -->' "$tpl" \
   && grep -qF '<!-- docket:artifacts:end -->' "$tpl" \
   && awk '/^## Artifacts/{a=NR} /^## Why/{w=NR} END{exit !(a>0 && w>0 && a<w)}' "$tpl"; then
  ok "K: template ships empty marker block before ## Why"
else
  no "K: template ships empty marker block before ## Why"
fi

# ---- Case L: non-URL pr (bare number) renders verbatim in GitHub mode, never a broken link ----
cfL="$tmp/0089-barepr.md"
cat > "$cfL" <<'EOF'
---
id: 89
slug: barepr
status: implemented
spec:
plan:
results:
branch: feat/barepr
pr: 44
adrs: []
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

x
EOF
render "$cfL" >/dev/null 2>&1
if grep -qF '| PR | 44 |' "$cfL" && ! grep -qF '[#44](44)' "$cfL"; then
  ok "L: non-URL pr renders verbatim, no broken link"
else
  no "L: non-URL pr renders verbatim, no broken link"; grep -F '| PR' "$cfL" || true
fi

# ---- Case M: killed + non-URL pr => plan row renders filename as plain text, no broken link ----
cfM="$tmp/0088-killedbarepr.md"
cat > "$cfM" <<'EOF'
---
id: 88
slug: killedbarepr
status: killed
spec:
plan: docs/superpowers/plans/2026-06-21-kbp.md
results:
branch: feat/kbp
pr: 99
adrs: []
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

x
EOF
render "$cfM" >/dev/null 2>&1
if grep -qF '| Plan | 2026-06-21-kbp.md |' "$cfM" && ! grep -qF '](99)' "$cfM"; then
  ok "M: killed + non-URL pr => plan filename plain text, no broken link"
else
  no "M: killed + non-URL pr => plan filename plain text, no broken link"; grep -F '| Plan' "$cfM" || true
fi

# ---- Case N: the derived Stacked children row (change 0298) ----
# Fixtures live in a REAL changes-dir shape (<changes>/active, <changes>/archive) because the child
# scan derives the directory to walk from the change file's own location. The flat fixtures above
# deliberately stay flat: they prove the scan is a no-op when there is no active/ or archive/ beside
# the change file.
mkdir -p "$tmp/changes/active" "$tmp/changes/archive"
mkchange(){ # mkchange <destpath> <id> <slug> <status> <stacked_on> <branch> <plan>
  cat > "$1" <<EOF
---
id: $2
slug: $3
title: "Change $2"
status: $4
spec:
plan: ${7:-}
results:
branch: ${6:-}
pr:
stacked_on: ${5:-}
adrs: []
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

x
EOF
}

cfP="$tmp/changes/active/0030-parent.md"
cfC="$tmp/changes/active/0031-child.md"
cfSM="$tmp/changes/active/0034-stackmerged.md"
cfArch="$tmp/changes/archive/2026-08-12-0033-archived-child.md"
mkchange "$cfP"    30 parent          in-progress    "" feat/parent
mkchange "$cfC"    31 child           proposed       30 ""
mkchange "$cfArch" 33 archived-child  done           30 feat/archived-child
mkchange "$cfSM"   34 stackmerged     stacked-merged 30 feat/beta \
  docs/superpowers/plans/2026-08-12-stackmerged.md

# Change 32 omits stacked_on from frontmatter entirely and opens a BODY line with it. An unanchored
# read runs past the closing --- and returns `30`, which is numeric and would pass the scan's own
# well-formedness guard — so this file is the discriminator between fm_field and field, and the
# "no phantom child" assert below is what reddens when the scan is mutated to the unanchored read.
cat > "$tmp/changes/active/0032-prose.md" <<'EOF'
---
id: 32
slug: prose
title: "Change 32"
status: proposed
spec:
plan:
results:
branch:
pr:
adrs: []
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

The stacking relationship is spelled out here, in prose, and never in frontmatter:

stacked_on: 30
EOF

# Change 35 spells the link PADDED and with a trailing inline comment — both legitimate, both
# repaired by fm_field's own reader. It pins the candidate prefilter to the KEY's shape: a prefilter
# narrowed to a value-shaped pattern would drop this child silently and no other leg would notice.
cat > "$tmp/changes/active/0035-padded.md" <<'EOF'
---
id: 35
slug: padded
title: "Change 35"
status: proposed
spec:
plan:
results:
branch:
pr:
stacked_on: 0030   # padded, with a trailing comment
adrs: []
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

x
EOF

render "$cfC"  >/dev/null 2>&1
render "$cfSM" >/dev/null 2>&1
render "$cfP"  >/dev/null 2>&1

if grep -qF '| Stacked children |' "$cfP" && grep -qF '#0031' "$cfP"; then
  ok "N: a parent renders its stacked children"
else
  no "N: a parent renders its stacked children"; sed -n '/## Artifacts/,/artifacts:end/p' "$cfP"
fi

if grep -qF '[#0031](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0031-child.md) Change 31 (proposed)' "$cfP"; then
  ok "N2: a child cell links to the metadata branch and names title + status"
else
  no "N2: a child cell links to the metadata branch and names title + status"; grep -F '| Stacked children' "$cfP" || true
fi

if grep -qF '#0033' "$cfP" && grep -qF '/blob/docket/docs/changes/archive/2026-08-12-0033-archived-child.md' "$cfP"; then
  ok "N3: an archived child is found by the scan too"
else
  no "N3: an archived child is found by the scan too"; grep -F '| Stacked children' "$cfP" || true
fi

# Discriminating leg for the fm_field/field mutation.
if ! grep -qF '#0032' "$cfP"; then
  ok "N4: a body line opening stacked_on does not create a phantom child"
else
  no "N4: a body line opening stacked_on does not create a phantom child"; grep -F '| Stacked children' "$cfP" || true
fi

if ! grep -qF '| Stacked children |' "$cfC"; then
  ok "N5: a child with no children of its own renders no such row"
else
  no "N5: a child with no children of its own renders no such row"; sed -n '/## Artifacts/,/artifacts:end/p' "$cfC"
fi

# stacked-merged is NON-terminal: the feature branch still carries the code, so plan/results keep
# the branch ref. Only `done` flips to the integration branch.
if grep -qF '/blob/feat/beta/docs/superpowers/plans/' "$cfSM" && ! grep -qF '/blob/main/docs/superpowers/plans/' "$cfSM"; then
  ok "N6: a stacked-merged change keeps branch-addressed plan links"
else
  no "N6: a stacked-merged change keeps branch-addressed plan links"; grep -F '| Plan' "$cfSM" || true
fi

if grep -qF '#0035' "$cfP"; then
  ok "N8: a padded, inline-commented stacked_on still names its parent"
else
  no "N8: a padded, inline-commented stacked_on still names its parent"; grep -F '| Stacked children' "$cfP" || true
fi

cp "$cfP" "$tmp/parent_after1.md"
render "$cfP" >/dev/null 2>&1
if diff -u "$tmp/parent_after1.md" "$cfP" >/dev/null; then
  ok "N7: the derived row is idempotent"
else
  no "N7: the derived row is idempotent"; diff -u "$tmp/parent_after1.md" "$cfP" || true
fi

exit $fail
