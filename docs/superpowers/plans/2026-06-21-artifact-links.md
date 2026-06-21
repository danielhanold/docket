# Artifact links — `## Artifacts` block renderer — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a generated, branch-aware `## Artifacts` link block to the top of every docket change body, rendered by a new deterministic script and wired into every skill that writes a backing frontmatter field.

**Architecture:** A new `scripts/render-change-links.sh` parses one change file's frontmatter (via the shared `scripts/lib/docket-frontmatter.sh` helpers), computes a GitHub blob/pull URL per artifact pinned to the branch that artifact actually lives on (spec/ADRs → `docket`; plan/results → the feature branch while building, the integration branch once `done`; PR → its URL verbatim), and replaces a marker-bounded block in place. Frontmatter stays the single source of truth; the script is the sole writer of the block (ADR-0012 script-vs-model boundary). Skills call the renderer immediately after any field write. Same inputs ⇒ byte-identical file (idempotent).

**Tech Stack:** Plain bash (POSIX-ish, `set -uo pipefail`), `awk` for marker-anchored block replacement, the existing `scripts/lib/docket-frontmatter.sh` helpers (`field`, `list_field`), and the plain-bash golden-file test pattern under `tests/`.

## Global Constraints

- **Sole writer / single source of truth:** the script is the ONLY writer of the block; frontmatter field *values* are the source of truth, the block is a derived *view* (ADR-0012). No skill hand-edits the block.
- **Idempotent:** running the renderer twice on the same file with no field change is a byte-for-byte no-op. Every task that adds rendering behavior MUST keep an idempotency assertion green.
- **Offline / graceful degradation:** no network, no `gh`. Non-GitHub or absent remote ⇒ fallback mode = bare code-formatted paths (`` `docs/...md` ``); PR stays its URL (it is one). Same posture as `render-board.sh`/`github-mirror.sh`.
- **Omit-until-set:** a frontmatter field that is unset produces NO row. When NO artifact field is set, the block still exists but contains only the heading + marker pair (no table header).
- **Per-artifact ref (verbatim from spec):** Spec → `METADATA_BRANCH` (no re-point). ADRs → `METADATA_BRANCH` (no re-point). Plan → `branch:` while non-terminal, `INTEGRATION_BRANCH` once `status: done`. Results → same as Plan. PR → the `pr:` URL verbatim.
- **Placement:** the `## Artifacts` block is the FIRST body section — immediately after the closing `---` of frontmatter, above `## Why`. No new `# Title` H1.
- **Markers (exact, fixed strings):**
  - start: `<!-- docket:artifacts:start (generated — do not hand-edit) -->`
  - end: `<!-- docket:artifacts:end -->`
- **Frontmatter reads via the shared helper only** — `field`/`list_field` from `scripts/lib/docket-frontmatter.sh`, always via command substitution `$(...)` (strips trailing newline — safe; never a raw pipe of these, per LEARNINGS 2026-06-20 #32 / 2026-06-19 #22).
- **No `producer | grep -q` / `producer | head`** under `pipefail` (LEARNINGS 2026-06-16 #11/#16) — capture into a var or use a glob array; never pipe a producer into an early-closing consumer.
- **Golden fixtures use real-shaped values** (LEARNINGS 2026-06-19 #22): full-URL `pr:` (`https://github.com/owner/repo/pull/NN`, never a bare number) and plurality (≥2 ADRs in the list). The renderer is smoke-tested against the real backlog before the PR (recorded in the results file, since the real change files live on `docket`, not the integration branch — LEARNINGS 2026-06-12 #6).
- **Mock seams:** `GIT="${GIT:-git}"`, `DOCKET_CONFIG="${DOCKET_CONFIG:-<scriptdir>/docket-config.sh}"` — tests stub config + remote hermetically without invoking real git/config.
- **Every new test assertion is proved non-vacuous** by a mutation check (delete the guarded clause ⇒ test flips to FAIL) (LEARNINGS 2026-06-04 #2).

---

## File Structure

- **Create** `scripts/render-change-links.sh` — the renderer (Tasks 1–3).
- **Create** `tests/test_render_change_links.sh` — golden + idempotency + insertion + lifecycle + ADR + fallback tests (Tasks 1–3).
- **Modify** `skills/docket-new-change/change-template.md` — ship the empty marker block as the first body section (Task 4).
- **Create** assertion in `tests/test_render_change_links.sh` (or a small dedicated test) for the template shape (Task 4).
- **Modify** the six field-writing skill bodies to call the renderer after each field write (Task 5):
  - `skills/docket-new-change/SKILL.md`, `skills/docket-groom-next/SKILL.md`, `skills/docket-auto-groom/SKILL.md`, `skills/docket-implement-next/SKILL.md`, `skills/docket-finalize-change/SKILL.md`, `skills/docket-status/SKILL.md`.
- **Modify** `skills/docket-convention/SKILL.md` — document the generated `## Artifacts` section under "Change body sections" and note the renderer in the derived-views/script family (Task 5).
- **Create** a sync-style coverage check — `tests/test_change_links_coverage.sh` — asserting every field-writing skill body invokes `render-change-links.sh` (Task 6).

---

## Task 1: Renderer core — frontmatter parse, Spec + PR rows, marker replace/insert, idempotency

**Files:**
- Create: `scripts/render-change-links.sh`
- Test: `tests/test_render_change_links.sh`

**Interfaces:**
- Consumes: `scripts/lib/docket-frontmatter.sh` → `field FILE KEY`, `list_field FILE KEY`.
- Produces: CLI `render-change-links.sh --change-file FILE [--repo OWNER/REPO] [--adrs-dir DIR]`; edits FILE in place; exits 0 on success, 2 on usage error, 1 on config-resolution failure. Mock seams `GIT`, `DOCKET_CONFIG`. Marker constants `START_MARKER`/`END_MARKER` as in Global Constraints.

- [ ] **Step 1: Write the failing test harness + first cases**

Create `tests/test_render_change_links.sh`. Model it on `tests/test_render_board.sh` (plain bash, temp fixtures, `diff -u` golden compare, idempotency re-run). Use these helpers and the first two cases:

```bash
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

exit $fail
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_render_change_links.sh`
Expected: FAIL — `scripts/render-change-links.sh` does not exist (cases A–D NOT OK / errors).

- [ ] **Step 3: Write the renderer (core)**

Create `scripts/render-change-links.sh`:

```bash
#!/usr/bin/env bash
# scripts/render-change-links.sh — deterministic, idempotent renderer for the per-change
# `## Artifacts` link block (change 0035). Reads ONE change file's frontmatter + resolved
# config and rewrites the marker-bounded block in place. Frontmatter is the single source of
# truth; this script is the SOLE writer of the block (ADR-0012 script-vs-model boundary).
# Offline (no gh, no network); does NOT commit (the calling skill commits). Same inputs =>
# byte-identical file.
#
# Usage: render-change-links.sh --change-file FILE [--repo OWNER/REPO] [--adrs-dir DIR]
#   --repo      build GitHub blob/pull URLs; default derives OWNER/REPO from the origin remote
#               of the change file's repo. Absent/non-GitHub remote => fallback (bare paths).
#   --adrs-dir  LOCAL dir to resolve ADR slugs; default METADATA_WORKTREE/ADRS_DIR from config.
#   Mock seams: GIT="${GIT:-git}", DOCKET_CONFIG="${DOCKET_CONFIG:-<scriptdir>/docket-config.sh}".
set -uo pipefail

START_MARKER='<!-- docket:artifacts:start (generated — do not hand-edit) -->'
END_MARKER='<!-- docket:artifacts:end -->'

GIT="${GIT:-git}"
SCRIPTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOCKET_CONFIG="${DOCKET_CONFIG:-$SCRIPTDIR/docket-config.sh}"
CHANGE_FILE=""
REPO=""
ADRS_DIR_LOCAL=""
REPO_EXPLICIT=0
while [ $# -gt 0 ]; do
  case "$1" in
    --change-file) CHANGE_FILE="$2"; shift ;;
    --repo) REPO="$2"; REPO_EXPLICIT=1; shift ;;
    --adrs-dir) ADRS_DIR_LOCAL="$2"; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) printf 'render-change-links: unknown argument: %s\n' "$1" >&2; exit 2 ;;
  esac
  shift
done
[ -n "$CHANGE_FILE" ] || { printf 'render-change-links: missing --change-file\n' >&2; exit 2; }
[ -f "$CHANGE_FILE" ] || { printf 'render-change-links: change file not found: %s\n' "$CHANGE_FILE" >&2; exit 2; }

# shellcheck source=/dev/null
source "$SCRIPTDIR/lib/docket-frontmatter.sh"

# Resolve config (branches + adrs dir). Mockable via DOCKET_CONFIG.
cfg="$("$DOCKET_CONFIG" --export 2>/dev/null)" || { printf 'render-change-links: config resolution failed\n' >&2; exit 1; }
eval "$cfg"
METADATA_BRANCH="${METADATA_BRANCH:-docket}"
INTEGRATION_BRANCH="${INTEGRATION_BRANCH:-main}"
ADRS_DIR="${ADRS_DIR:-docs/adrs}"          # repo-relative, for URLs
METADATA_WORKTREE="${METADATA_WORKTREE:-}"

if [ -z "$ADRS_DIR_LOCAL" ]; then
  if [ -n "$METADATA_WORKTREE" ]; then ADRS_DIR_LOCAL="$METADATA_WORKTREE/$ADRS_DIR"; else ADRS_DIR_LOCAL="$ADRS_DIR"; fi
fi

# Derive OWNER/REPO + GitHub mode from the origin remote (render-board.sh pattern), unless --repo.
GITHUB=0
if [ "$REPO_EXPLICIT" = 1 ]; then
  GITHUB=1
else
  url="$("$GIT" -C "$(dirname "$CHANGE_FILE")" remote get-url origin 2>/dev/null || true)"
  case "$url" in
    git@github.com:*|https://github.com/*|ssh://git@github.com/*)
      REPO="${url%.git}"
      REPO="${REPO#git@github.com:}"; REPO="${REPO#https://github.com/}"; REPO="${REPO#ssh://git@github.com/}"
      GITHUB=1 ;;
    *) GITHUB=0 ;;
  esac
fi

blob(){ printf 'https://github.com/%s/blob/%s/%s' "$REPO" "$1" "$2"; }  # ref, repo-rel-path

# Read frontmatter (command substitution strips trailing newline — safe).
status="$(field "$CHANGE_FILE" status)"
branch="$(field "$CHANGE_FILE" branch)"
spec="$(field "$CHANGE_FILE" spec)"
plan="$(field "$CHANGE_FILE" plan)"
results="$(field "$CHANGE_FILE" results)"
pr="$(field "$CHANGE_FILE" pr)"
adrs="$(list_field "$CHANGE_FILE" adrs)"   # space-separated ids, "" when [] / unset

# plan/results ref: integration branch once done, else the feature branch.
build_ref="$branch"
[ "$status" = "done" ] && build_ref="$INTEGRATION_BRANCH"

# Emit one artifact row to stdout (nothing if it must be omitted). $1 label, $2 path.
build_row(){
  local label="$1" path="$2" text; text="$(basename "$path")"
  if [ "$GITHUB" != 1 ]; then printf '| %s | `%s` |\n' "$label" "$path"; return; fi
  if [ "$status" = "killed" ]; then
    # feature branch gone, not merged: link to the PR if there is one, else omit.
    [ -n "$pr" ] && printf '| %s | [%s](%s) |\n' "$label" "$text" "$pr"
    return
  fi
  printf '| %s | [%s](%s) |\n' "$label" "$text" "$(blob "$build_ref" "$path")"
}

rows=""
# Spec — always on METADATA_BRANCH.
if [ -n "$spec" ]; then
  if [ "$GITHUB" = 1 ]; then rows+="| Spec | [$(basename "$spec")]($(blob "$METADATA_BRANCH" "$spec")) |"$'\n'
  else rows+="| Spec | \`$spec\` |"$'\n'; fi
fi
# Plan / Results — lifecycle-pinned (build_row).
[ -n "$plan" ]    && rows+="$(build_row Plan "$plan")"$'\n'
[ -n "$results" ] && rows+="$(build_row Results "$results")"$'\n'
# PR — its URL verbatim (#NN text).
if [ -n "$pr" ]; then
  num="${pr##*/}"
  if [ "$GITHUB" = 1 ]; then rows+="| PR | [#$num]($pr) |"$'\n'; else rows+="| PR | $pr |"$'\n'; fi
fi
# ADRs handled in Task 2 (appended after this block).

# build_row may emit an empty line (killed + no pr). Strip blank lines from rows.
rows="$(printf '%s' "$rows" | sed '/^$/d')"
[ -n "$rows" ] && rows="$rows"$'\n'

# Assemble the marker-bounded block into a temp file.
block_file="$(mktemp)"; trap 'rm -f "$block_file"' EXIT
{
  printf '%s\n' "$START_MARKER"
  if [ -n "$rows" ]; then printf '| Artifact | Link |\n|---|---|\n'; printf '%s' "$rows"; fi
  printf '%s\n' "$END_MARKER"
} > "$block_file"

out="$(mktemp)"
if grep -qF "$START_MARKER" "$CHANGE_FILE"; then
  # Replace inclusive marker block (fixed-string match via index()).
  awk -v startm="$START_MARKER" -v endm="$END_MARKER" -v blk="$block_file" '
    BEGIN { while ((getline line < blk) > 0) block = block line ORS }
    index($0, startm) { printf "%s", block; inblk=1; next }
    inblk && index($0, endm) { inblk=0; next }
    !inblk { print }
  ' "$CHANGE_FILE" > "$out"
else
  # Insert as the first body section, right after the frontmatter close (2nd ---).
  awk -v blk="$block_file" '
    BEGIN { while ((getline line < blk) > 0) block = block line ORS }
    { print }
    /^---[[:space:]]*$/ { n++; if (n==2) { print ""; print "## Artifacts"; print ""; printf "%s", block } }
  ' "$CHANGE_FILE" > "$out"
fi
mv "$out" "$CHANGE_FILE"
```

- [ ] **Step 4: Run the test to verify cases A–D pass**

Run: `bash tests/test_render_change_links.sh`
Expected: cases A, B, C, D all `ok`. (ADR/lifecycle/fallback cases come in Tasks 2–3.)

- [ ] **Step 5: Mutation-check the new assertions (non-vacuity)**

Temporarily break the renderer (e.g. comment out the PR row append), re-run the test, confirm case A flips to `NOT OK`; restore. Confirm `chmod +x scripts/render-change-links.sh tests/test_render_change_links.sh`.

- [ ] **Step 6: Commit**

```bash
cd "$(git rev-parse --show-toplevel)"
chmod +x scripts/render-change-links.sh tests/test_render_change_links.sh
git add scripts/render-change-links.sh tests/test_render_change_links.sh
git commit -m "feat(0035): render-change-links.sh — spec/PR rows, marker replace+insert, idempotent"
```

---

## Task 2: ADR rows — slug resolution, multi-ADR list, missing-file fallback

**Files:**
- Modify: `scripts/render-change-links.sh` (append ADR cell to `rows`)
- Modify: `tests/test_render_change_links.sh` (add ADR cases)

**Interfaces:**
- Consumes: `adrs` (space-separated ids from `list_field`), `ADRS_DIR` (repo-rel, URLs), `ADRS_DIR_LOCAL` (slug lookup), `METADATA_BRANCH`.
- Produces: a single `| ADRs | ... |` row, comma-separated `[ADR-NNNN](url)` links (zero-padded id); missing ADR file ⇒ link text `ADR-NNNN` to the `ADRS_DIR` listing (no broken deep link).

- [ ] **Step 1: Write the failing test (append to the test file, before `exit $fail`)**

```bash
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
```

- [ ] **Step 2: Run the test to verify cases E–F fail**

Run: `bash tests/test_render_change_links.sh`
Expected: A–D `ok`; E, F `NOT OK` (no ADR row yet).

- [ ] **Step 3: Append the ADR cell builder in `scripts/render-change-links.sh`**

Insert immediately AFTER the PR-row block and BEFORE the `# build_row may emit an empty line` comment:

```bash
# ADRs — each id on METADATA_BRANCH; slug resolved from the local ADR file; missing => dir link.
if [ -n "$adrs" ]; then
  adr_cell=""
  for id in $adrs; do
    padded="$(printf '%04d' "$id")"
    m=( "$ADRS_DIR_LOCAL"/"${padded}"-*.md )   # glob, not `ls | head` (pipefail-safe)
    if [ -e "${m[0]}" ]; then
      relpath="$ADRS_DIR/$(basename "${m[0]}")"
      if [ "$GITHUB" = 1 ]; then link="[ADR-$padded]($(blob "$METADATA_BRANCH" "$relpath"))"; else link="\`$relpath\`"; fi
    else
      if [ "$GITHUB" = 1 ]; then link="[ADR-$padded]($(blob "$METADATA_BRANCH" "$ADRS_DIR"))"; else link="ADR-$padded"; fi
    fi
    if [ -n "$adr_cell" ]; then adr_cell+=", $link"; else adr_cell="$link"; fi
  done
  rows+="| ADRs | $adr_cell |"$'\n'
fi
```

- [ ] **Step 4: Run the test to verify cases A–F pass**

Run: `bash tests/test_render_change_links.sh`
Expected: A–F all `ok`.

- [ ] **Step 5: Mutation-check**

Temporarily change `padded` to `$id` (drop zero-pad), re-run: case E must flip to `NOT OK`; restore.

- [ ] **Step 6: Commit**

```bash
git add scripts/render-change-links.sh tests/test_render_change_links.sh
git commit -m "feat(0035): ADR rows — slug resolution, multi-ADR list, missing-file fallback"
```

---

## Task 3: Lifecycle ref flip, killed edge, and non-GitHub fallback

**Files:**
- Modify: `tests/test_render_change_links.sh` (add cases G–J)
- Modify: `scripts/render-change-links.sh` only if a case exposes a gap (lifecycle + killed logic already drafted in Task 1's `build_row`; this task proves them and adds the fallback case).

**Interfaces:**
- Consumes: `status`, `branch`, `INTEGRATION_BRANCH`, `pr`, `GITHUB`.
- Produces: plan/results pinned to `feat/<slug>` while `in-progress`/`implemented`, flipped to `INTEGRATION_BRANCH` at `done`; killed-from-in-progress points plan/results at the PR (or omits if no PR); non-GitHub remote ⇒ bare code-formatted paths (PR stays a URL).

- [ ] **Step 1: Write the failing tests (append before `exit $fail`)**

```bash
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

# ---- Case I: killed-from-in-progress => plan/results point at PR; omit when no PR ----
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
```

- [ ] **Step 2: Run the test**

Run: `bash tests/test_render_change_links.sh`
Expected: G, H, J pass on Task 1's drafted logic; I passes on the `killed` branch in `build_row`. If any fail, fix the renderer minimally (most likely the `git-nongh` mock wiring or the `build_row` killed branch).

- [ ] **Step 3: Fix the renderer if a case exposes a gap**

If case J fails because the `GIT -C <dir> remote get-url origin` argument order does not match the mock, adjust the mock or the renderer's remote call so the non-GitHub path is exercised. Keep GitHub-mode (cases A–I) green.

- [ ] **Step 4: Re-run the full suite**

Run: `bash tests/test_render_change_links.sh`
Expected: A–J all `ok`.

- [ ] **Step 5: Mutation-check the lifecycle flip**

Temporarily force `build_ref="$branch"` unconditionally (drop the done flip), re-run: case H flips to `NOT OK`; restore.

- [ ] **Step 6: Commit**

```bash
git add scripts/render-change-links.sh tests/test_render_change_links.sh
git commit -m "feat(0035): lifecycle ref flip, killed edge, non-GitHub fallback"
```

---

## Task 4: `change-template.md` — ship the empty marker block

**Files:**
- Modify: `skills/docket-new-change/change-template.md`
- Modify: `tests/test_render_change_links.sh` (template-shape assertion)

**Interfaces:**
- Consumes: nothing.
- Produces: a `## Artifacts` section as the FIRST body section of the template, containing only the marker pair (no table), above `## Why`.

- [ ] **Step 1: Write the failing assertion (append before `exit $fail`)**

```bash
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
```

- [ ] **Step 2: Run the test to verify case K fails**

Run: `bash tests/test_render_change_links.sh`
Expected: K `NOT OK` (template has no block yet).

- [ ] **Step 3: Edit `skills/docket-new-change/change-template.md`**

Insert this block immediately after the closing `---` of the frontmatter and before `## Why`:

```markdown

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->
```

The file then reads: frontmatter `---` … `---`, blank line, `## Artifacts`, blank, the two markers, blank, `## Why`, …

- [ ] **Step 4: Run the test to verify case K passes**

Run: `bash tests/test_render_change_links.sh`
Expected: A–K all `ok`.

- [ ] **Step 5: Commit**

```bash
git add skills/docket-new-change/change-template.md tests/test_render_change_links.sh
git commit -m "feat(0035): change-template ships the empty ## Artifacts marker block"
```

---

## Task 5: Wire the renderer into the six skill bodies + convention doc

**Files:**
- Modify: `skills/docket-new-change/SKILL.md`, `skills/docket-groom-next/SKILL.md`, `skills/docket-auto-groom/SKILL.md`, `skills/docket-implement-next/SKILL.md`, `skills/docket-finalize-change/SKILL.md`, `skills/docket-status/SKILL.md`
- Modify: `skills/docket-convention/SKILL.md`

**Interfaces:**
- Consumes: the renderer CLI from Tasks 1–3.
- Produces: each field-writing skill body contains the literal string `scripts/render-change-links.sh` in the step where it writes a backing field, instructing the skill to re-render the block (against the metadata-worktree copy of the change file) as part of that same field-write commit. The convention documents the generated `## Artifacts` section and the renderer in the script family. NO convention prose is copied into operating skills (guarded by `tests/test_convention_extraction.sh`).

- [ ] **Step 1: Confirm the exact field-write site in each skill**

Run (read-only, to anchor each edit precisely):

```bash
cd "$(git rev-parse --show-toplevel)"
grep -nE 'spec:|plan:|pr:|adrs:|results:|status: done|sweep' skills/docket-new-change/SKILL.md skills/docket-groom-next/SKILL.md skills/docket-auto-groom/SKILL.md skills/docket-implement-next/SKILL.md skills/docket-finalize-change/SKILL.md skills/docket-status/SKILL.md
```

Treat the spec's call-site list as a FLOOR, not a ceiling (LEARNINGS 2026-06-20 #32): also grep for any other field-write the spec under-named, and wire those too.

- [ ] **Step 2: Add the renderer instruction at each site**

In each skill, at the moment it writes a backing field (in the metadata working tree), add one instruction line of the shape:

> After writing `<field>:`, regenerate the change's `## Artifacts` block: `scripts/render-change-links.sh --change-file <metadata-worktree change file>` (in `docket`-mode pass the `.docket/`-prefixed path and `--adrs-dir .docket/<adrs_dir>`). The block edit rides with this same field-write commit; the renderer is the sole writer of the block.

Per-skill field sites:
- `docket-new-change`: after writing `spec:` at draft time (and note the template already seeds the empty block).
- `docket-groom-next`: after writing `spec:` (trivial verdict has no spec — block stays empty until build).
- `docket-auto-groom`: after writing `spec:`.
- `docket-implement-next`: after `plan:` (step 4), after `adrs:` (step 6), after `pr:`/`results:` (step 7). Add the renderer call to each of those existing metadata writes.
- `docket-finalize-change`: at the `done` transition (the archive/board-refresh step) so plan/results re-point to the integration branch.
- `docket-status`: in the sweep's `done` transition, same re-point.

Keep each addition minimal and in the skill's existing voice; do NOT restate convention prose (extraction invariant).

- [ ] **Step 3: Update `skills/docket-convention/SKILL.md`**

Two minimal edits:
1. Under **"Change body sections"**, add a bullet for the generated `## Artifacts` block as the first body section (marker-bounded, rendered by `render-change-links.sh` from frontmatter; never hand-edited), mirroring how `## Reconcile log` and `## Auto-groom blocked` are described.
2. Where the derived-views/script family is described (near `render-board.sh`, `github-mirror.sh`, the ADR-index renderer), name `scripts/render-change-links.sh` as the per-change link-block renderer (sole writer; ADR-0012 boundary; offline fallback to bare paths).

- [ ] **Step 4: Run the convention-extraction + existing suites to confirm no copy/regression**

Run:
```bash
bash tests/test_convention_extraction.sh
bash tests/test_render_change_links.sh
```
Expected: both pass (no convention prose copied into skills; renderer suite still green).

- [ ] **Step 5: Commit**

```bash
git add skills/docket-new-change/SKILL.md skills/docket-groom-next/SKILL.md skills/docket-auto-groom/SKILL.md skills/docket-implement-next/SKILL.md skills/docket-finalize-change/SKILL.md skills/docket-status/SKILL.md skills/docket-convention/SKILL.md
git commit -m "feat(0035): wire render-change-links into field-writing skills + convention doc"
```

---

## Task 6: Sync-style coverage check + real-data smoke test

**Files:**
- Create: `tests/test_change_links_coverage.sh`

**Interfaces:**
- Consumes: the wired skill bodies from Task 5.
- Produces: a check (exit non-zero on failure) asserting every field-writing skill body invokes `render-change-links.sh`; mirrors the wiring-sentinel pattern in `tests/test_render_board.sh` and the extraction-invariant style of `tests/test_convention_extraction.sh`.

- [ ] **Step 1: Write the failing coverage test**

```bash
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
```

- [ ] **Step 2: Run the test**

Run: `bash tests/test_change_links_coverage.sh`
Expected: all `ok` (Task 5 wired every skill). If any skill is `NOT OK`, return to Task 5 and wire it.

- [ ] **Step 3: Mutation-check non-vacuity**

Temporarily remove the renderer line from one skill, re-run: that skill's assertion flips to `NOT OK`; restore.

- [ ] **Step 4: Real-data smoke test (record in results, not a repo test)**

The real change files live on `docket`, not the integration branch, so a repo test can't see them (LEARNINGS 2026-06-12 #6). Smoke-test the renderer against the real backlog and confirm idempotency + a clean diff:

```bash
# From the repo root, against the live metadata worktree:
cp .docket/docs/changes/active/0035-artifact-links.md /tmp/smoke-0035.md
scripts/render-change-links.sh --change-file /tmp/smoke-0035.md --adrs-dir .docket/docs/adrs
scripts/render-change-links.sh --change-file /tmp/smoke-0035.md --adrs-dir .docket/docs/adrs   # idempotent
sed -n '/## Artifacts/,/artifacts:end/p' /tmp/smoke-0035.md
```
Expected: a well-formed block with Spec + ADRs rows pinned to `docket`; second run byte-identical. Record the output in the results file. (Do NOT commit the /tmp copy.)

- [ ] **Step 5: Run the full suite**

Run:
```bash
for t in tests/test_render_change_links.sh tests/test_change_links_coverage.sh tests/test_convention_extraction.sh; do echo "== $t =="; bash "$t"; done
```
Expected: all green.

- [ ] **Step 6: Commit**

```bash
chmod +x tests/test_change_links_coverage.sh
git add tests/test_change_links_coverage.sh
git commit -m "test(0035): coverage check — every field-writing skill invokes the renderer"
```

---

## Self-Review

**1. Spec coverage:**
- Scope (spec/plan/results/ADRs/PR) → Tasks 1–3. ✓
- Link form (absolute GitHub blob URLs, branch-pinned; bare-path fallback) → Tasks 1, 3. ✓
- Mechanism (deterministic script, frontmatter source of truth, sole writer, ADR-0012) → Task 1 + Global Constraints. ✓
- Per-artifact ref table (spec/ADRs stable on `docket`; plan/results flip at `done`) → Tasks 1–3 (cases G/H). ✓
- Placement (first body section, after frontmatter, before `## Why`) → Task 1 case C + Task 4. ✓
- Omit-until-set + empty block → Task 1 case D; full matrix across Tasks 1–3. ✓
- Renderer Inputs/Behavior (config via `docket-config.sh --export`; remote via `render-board.sh` pattern per the reconcile correction; marker replace/insert; idempotent) → Task 1. ✓
- Call sites (6 skills + `done` transition in finalize & status sweep) → Task 5; coverage gate → Task 6. ✓
- Edge cases (killed-from-in-progress, trivial/no-spec, offline, marker-absent older files) → Tasks 1–3 (cases C, D, I, J). ✓
- `change-template.md` empty block → Task 4. ✓
- Convention doc update → Task 5. ✓
- Testing approach (golden, idempotency, ref correctness, marker insertion, non-GitHub fallback, sync-style coverage check) → Tasks 1–6. ✓

**2. Placeholder scan:** No TBD/TODO/"handle edge cases"/"similar to Task N" — all code is inline. ✓

**3. Type consistency:** `build_row`, `blob`, `START_MARKER`/`END_MARKER`, `build_ref`, `ADRS_DIR`/`ADRS_DIR_LOCAL`, `GITHUB` used identically across Tasks 1–3. The renderer CLI (`--change-file`/`--repo`/`--adrs-dir`) is identical everywhere it is called (tests + skills + coverage). ✓
