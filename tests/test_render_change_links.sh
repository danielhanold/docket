#!/usr/bin/env bash
# tests/test_render_change_links.sh — golden + idempotency + lifecycle + ADR + fallback tests
# for scripts/render-change-links.sh. Plain bash; hermetic fixtures; no network/gh.
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$ROOT/scripts/render-change-links.sh"
fail=0
note(){ printf '%s\n' "$*"; }
ok(){ printf 'ok   - %s\n' "$1"; }
no(){ printf 'NOT OK - %s\n' "$1"; fail=1; }

# A stub docket-config.sh: prints the export lines render-change-links.sh evals.
# METADATA_WORKTREE empty => ADR lookup falls back to ADRS_DIR (the fixture dir we pass via --adrs-dir).
make_config_stub(){ # $1 dir
  cat > "$1/docket-config.sh" <<'EOF'
#!/usr/bin/env bash
echo "METADATA_BRANCH=docket"
echo "INTEGRATION_BRANCH=main"
echo "ADRS_DIR=docs/adrs"
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
if awk '/^---[[:space:]]*$/{n++} n==2 && /docket:artifacts:start/{seen_start=1} /^## Why/{ if(seen_start) print "OK"; exit }' "$cf2" | grep -qx OK \
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

exit $fail
