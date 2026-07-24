# Artifact back-links Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stamp a generated, marker-bounded back-link block at the top of every artifact docket touches (spec, plan, results, PR body), pointing home to the change file on `metadata_branch` — the reciprocal of change 0035's forward `## Artifacts` block.

**Architecture:** A new deterministic renderer `render-artifact-backlink.sh` (sibling of `render-change-links.sh`, sharing its idioms) is the sole writer of a `docket:backlink` block. It is wired into the facade, into the creation-time skill steps that write each artifact, and into the terminal close-out for durability — with plan/results re-render folded into `terminal-publish.sh`'s existing publish commit under `terminal_publish: true`.

**Tech Stack:** POSIX-ish bash (the configured Bash 4+ runtime via `DOCKET_BASH_PATH`), `awk`/`grep`/`printf`, `git` plumbing, plain-bash hermetic tests under `tests/`.

## Global Constraints

- **Sole-writer / ADR-0012 boundary:** the `docket:backlink` block is written only by `render-artifact-backlink.sh`; skills never hand-edit it. Same inputs → byte-identical block (idempotent, offline, no `gh`).
- **Mirror the sibling exactly:** reuse `render-change-links.sh`'s idioms — `set -uo pipefail`, `GIT="${GIT:-git}"`, `DOCKET_CONFIG="${DOCKET_CONFIG:-<scriptdir>/docket-config.sh}"` mock seams, config via `docket-config.sh --export`, marker-block replace-vs-insert via `awk` with `index()` fixed-string matching, in-place edit via `mktemp` + `mv`.
- **Atomic generated write** (LEARNINGS `atomic-generated-write`): render to a temp file, `mv` into place — never redirect straight into the target. Targets are non-executable markdown data files, so mode-loss does not apply.
- **Model-authored title is untrusted input** (LEARNINGS `model-authored-values-are-untrusted-input`): write the change `title:` into the block via `printf '%s'` into a block temp file that `awk` inserts **verbatim** — never a `sed`/string-interpolated replacement (which reinterprets `&`, `\1`). `fm_field` returns a single line, so no newline/structural injection.
- **Frontmatter-scoped reads** (LEARNINGS `frontmatter-anchored-read`): read `id:` and `title:` via `fm_field` (first `---…---` block only), not `field` — this repo's own subject matter is field names, and a body line opening `title:` must never win.
- **Strict renderer, best-effort callers** (LEARNINGS `best-effort-helper-on-a-sole-deliverable-path`): the renderer's output IS its deliverable, so it exits non-zero on failure (config → 1, bad args → 2). The best-effort posture lives at the CALL SITES that opt into it (the PR-body re-render at close-out; the terminal-publish re-stamp within the publish).
- **Uniform target:** every back-link points to the change on `metadata_branch` at its current canonical path (`active/…` while live, `archive/…` once terminal). `terminal_publish` changes only whether the close-out re-render fires, never the link target.
- **Scope excludes ADRs** (already back-referenced by `change:` frontmatter + the index), BOARD.md, the forward `## Artifacts` block, URL schemes beyond GitHub-blob + bare-path, and a one-time back-fill over already-terminal changes.
- **Wiring sentinels anchor on the producer** (LEARNINGS `specified-but-unreachable`, `correspondence-guard-runs-one-way`): each call-site sentinel greps the paragraph that performs the write, not prose that merely defines it.
- Every commit message ends with `Claude-Session: https://claude.ai/code/session_01K8zaLYGgiCcMXMs96qnWmo`.
- No GitHub Actions CI — the plain-bash suite (`tests/*.sh`) is the de-facto gate. The whole suite must be run at the build gate, never only the enumerated tests.

---

### Task 1: The renderer `render-artifact-backlink.sh` + contract + tests

**Files:**
- Create: `scripts/render-artifact-backlink.sh`
- Create: `scripts/render-artifact-backlink.md`
- Test: `tests/test_render_artifact_backlink.sh`

**Interfaces:**
- Produces (for later tasks): a CLI `render-artifact-backlink.sh --artifact-file FILE --change-file CHANGE [--repo OWNER/REPO]`. Reads `id`+`title` from the change frontmatter; derives the change's repo-relative path as `<CHANGES_DIR>/<active|archive>/<basename>`; stamps/replaces a `docket:backlink` block at the TOP of the artifact. Exit 0 (written/unchanged), 1 (config resolution failed), 2 (missing/invalid `--artifact-file`/`--change-file`, unknown flag).
- Block shape (GitHub mode):
  ```
  <!-- docket:backlink:start (generated — do not hand-edit) -->
  > ↩ **[Change 0136 — <title>](https://github.com/<owner>/<repo>/blob/<metadata_branch>/<changes_dir>/<active|archive>/<file>)**
  <!-- docket:backlink:end -->
  ```
  Fallback (no/non-GitHub remote): `> ↩ **Change 0136 — <title>** — ` + a backtick-fenced bare path.

- [ ] **Step 1: Write the failing test file**

Create `tests/test_render_artifact_backlink.sh`. It mirrors `tests/test_render_change_links.sh`'s harness (hermetic tmp fixtures, a stubbed `docket-config.sh` passed via `DOCKET_CONFIG`, explicit `--repo` for GitHub mode). The config stub must export `METADATA_BRANCH` and `CHANGES_DIR`.

```bash
#!/usr/bin/env bash
# tests/test_render_artifact_backlink.sh — golden + idempotency + insert/replace + fallback +
# frontmatter-extraction + arg/config-failure tests for scripts/render-artifact-backlink.sh
# (change 0136). Plain bash; hermetic fixtures; no network/gh.
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$ROOT/scripts/render-artifact-backlink.sh"
fail=0
ok(){ printf 'ok   - %s\n' "$1"; }
no(){ printf 'NOT OK - %s\n' "$1"; fail=1; }

make_config_stub(){ # $1 dir
  cat > "$1/docket-config.sh" <<'EOF'
#!/usr/bin/env bash
echo "METADATA_BRANCH=docket"
echo "CHANGES_DIR=docs/changes"
EOF
  chmod +x "$1/docket-config.sh"
}

# Render in GitHub mode with hermetic config + explicit --repo.
render(){ # $1 artifact-file  $2 change-file ; extra args follow
  local af="$1" cf="$2"; shift 2
  DOCKET_CONFIG="$tmp/docket-config.sh" GIT=git \
    bash "$SCRIPT" --artifact-file "$af" --change-file "$cf" --repo danielhanold/docket "$@"
}

tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
make_config_stub "$tmp"

# A change file at an active/ canonical path.
mkdir -p "$tmp/docs/changes/active" "$tmp/docs/changes/archive"
cf_active="$tmp/docs/changes/active/0136-artifact-backlinks.md"
cat > "$cf_active" <<'EOF'
---
id: 136
slug: artifact-backlinks
title: Artifact back-links — a link home
status: in-progress
---

## Why

Body prose. Note this line mentions title: not-the-real-title to prove the anchored read.
EOF

# ---- Case A: insert into a spec that has NO block yet (GitHub mode, active path) ----
spec="$tmp/spec.md"
printf '# Some spec heading\n\nSpec body.\n' > "$spec"
render "$spec" "$cf_active" || no "case A: render exits 0"
cat > "$tmp/golden_A.md" <<'EOF'
<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0136 — Artifact back-links — a link home](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0136-artifact-backlinks.md)**
<!-- docket:backlink:end -->

# Some spec heading

Spec body.
EOF
if diff -u "$tmp/golden_A.md" "$spec" >/dev/null; then ok "case A: block inserted at top, GitHub mode, active path"; else no "case A: block inserted at top, GitHub mode, active path"; diff -u "$tmp/golden_A.md" "$spec" || true; fi

# ---- Case B: idempotency — a second run is byte-identical ----
cp "$spec" "$tmp/spec_before.md"
render "$spec" "$cf_active" || no "case B: second render exits 0"
if diff -u "$tmp/spec_before.md" "$spec" >/dev/null; then ok "case B: second render is byte-identical (idempotent)"; else no "case B: second render is byte-identical (idempotent)"; fi

# ---- Case C: replace an existing (stale) block in place ----
plan="$tmp/plan.md"
cat > "$plan" <<'EOF'
<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0136 — STALE TITLE](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/OLD.md)**
<!-- docket:backlink:end -->

# Plan heading

Plan body.
EOF
render "$plan" "$cf_active" || no "case C: render exits 0"
cat > "$tmp/golden_C.md" <<'EOF'
<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0136 — Artifact back-links — a link home](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0136-artifact-backlinks.md)**
<!-- docket:backlink:end -->

# Plan heading

Plan body.
EOF
if diff -u "$tmp/golden_C.md" "$plan" >/dev/null; then ok "case C: stale block replaced in place"; else no "case C: stale block replaced in place"; diff -u "$tmp/golden_C.md" "$plan" || true; fi

# ---- Case D: archive/ canonical path is reflected in the URL ----
cf_arch="$tmp/docs/changes/archive/2026-07-30-0136-artifact-backlinks.md"
cp "$cf_active" "$cf_arch"
results="$tmp/results.md"
printf '# Results\n' > "$results"
render "$results" "$cf_arch" || no "case D: render exits 0"
grep -qF 'blob/docket/docs/changes/archive/2026-07-30-0136-artifact-backlinks.md' "$results" \
  && ok "case D: archive path reflected in URL" || no "case D: archive path reflected in URL"

# ---- Case E: bare-path fallback when no --repo and a non-GitHub remote ----
# A dir whose origin remote is not github => fallback. Use a git repo with a non-github origin.
bare="$tmp/bare"; mkdir -p "$bare"; ( cd "$bare" && git init -q && git remote add origin https://example.com/x.git )
art_fb="$bare/spec.md"; printf '# Heading\n' > "$art_fb"
DOCKET_CONFIG="$tmp/docket-config.sh" GIT=git bash "$SCRIPT" --artifact-file "$art_fb" --change-file "$cf_active" || no "case E: render exits 0"
grep -qF '> ↩ **Change 0136 — Artifact back-links — a link home** — `docs/changes/active/0136-artifact-backlinks.md`' "$art_fb" \
  && ok "case E: bare-path fallback (no GitHub remote)" || { no "case E: bare-path fallback (no GitHub remote)"; sed -n '1,4p' "$art_fb"; }

# ---- Case F: arg validation ----
DOCKET_CONFIG="$tmp/docket-config.sh" bash "$SCRIPT" --change-file "$cf_active" >/dev/null 2>&1; [ $? -eq 2 ] && ok "missing --artifact-file exits 2" || no "missing --artifact-file exits 2"
DOCKET_CONFIG="$tmp/docket-config.sh" bash "$SCRIPT" --artifact-file "$spec" >/dev/null 2>&1; [ $? -eq 2 ] && ok "missing --change-file exits 2" || no "missing --change-file exits 2"
DOCKET_CONFIG="$tmp/docket-config.sh" bash "$SCRIPT" --artifact-file "$tmp/nope.md" --change-file "$cf_active" >/dev/null 2>&1; [ $? -eq 2 ] && ok "nonexistent --artifact-file exits 2" || no "nonexistent --artifact-file exits 2"
DOCKET_CONFIG="$tmp/docket-config.sh" bash "$SCRIPT" --artifact-file "$spec" --change-file "$cf_active" --bogus >/dev/null 2>&1; [ $? -eq 2 ] && ok "unknown flag exits 2" || no "unknown flag exits 2"

# ---- Case G: config-resolution failure exits 1 ----
printf '#!/usr/bin/env bash\nexit 3\n' > "$tmp/badconfig.sh"; chmod +x "$tmp/badconfig.sh"
art_g="$tmp/g.md"; printf '# H\n' > "$art_g"
DOCKET_CONFIG="$tmp/badconfig.sh" bash "$SCRIPT" --artifact-file "$art_g" --change-file "$cf_active" --repo danielhanold/docket >/dev/null 2>&1
[ $? -eq 1 ] && ok "config failure exits 1" || no "config failure exits 1"

exit $fail
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_render_artifact_backlink.sh`
Expected: FAIL — the script does not exist yet (multiple `NOT OK` lines / non-zero exit).

- [ ] **Step 3: Write the renderer `scripts/render-artifact-backlink.sh`**

```bash
#!/usr/bin/env bash
# scripts/render-artifact-backlink.sh — deterministic, idempotent renderer for the `docket:backlink`
# block stamped at the TOP of an artifact (spec, plan, or results) pointing HOME to its change file
# on metadata_branch (change 0136). The reciprocal of render-change-links.sh's forward `## Artifacts`
# block, sharing its idioms. Frontmatter (id, title) + the change-file path are the single source of
# truth; this script is the SOLE writer of the block (ADR-0012 script-vs-model boundary). Offline
# (no gh, no network); does NOT commit (the calling skill/script commits). Same inputs =>
# byte-identical file.
#
# Usage: render-artifact-backlink.sh --artifact-file FILE --change-file CHANGE [--repo OWNER/REPO]
#   --artifact-file  the spec/plan/results markdown file to update in place.
#   --change-file    the change file at its CURRENT canonical path (active/… while live, archive/…
#                    once terminal). id + title are read from its frontmatter; the URL path is
#                    derived from this path (so terminal_publish never changes the link TARGET).
#   --repo           build GitHub blob URLs; default derives OWNER/REPO from the artifact file's
#                    origin remote. Absent/non-GitHub remote => bare-path fallback.
#   Mock seams: GIT="${GIT:-git}", DOCKET_CONFIG="${DOCKET_CONFIG:-<scriptdir>/docket-config.sh}".
set -uo pipefail

START_MARKER='<!-- docket:backlink:start (generated — do not hand-edit) -->'
END_MARKER='<!-- docket:backlink:end -->'

GIT="${GIT:-git}"
SCRIPTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ -n "${DOCKET_CONFIG:-}" ]; then DOCKET_CONFIG_EXPLICIT=1; else DOCKET_CONFIG_EXPLICIT=0; DOCKET_CONFIG="$SCRIPTDIR/docket-config.sh"; fi
ARTIFACT_FILE=""
CHANGE_FILE=""
REPO=""
REPO_EXPLICIT=0
while [ $# -gt 0 ]; do
  case "$1" in
    --artifact-file) ARTIFACT_FILE="$2"; shift ;;
    --change-file) CHANGE_FILE="$2"; shift ;;
    --repo) REPO="$2"; REPO_EXPLICIT=1; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) printf 'render-artifact-backlink: unknown argument: %s\n' "$1" >&2; exit 2 ;;
  esac
  shift
done
[ -n "$ARTIFACT_FILE" ] || { printf 'render-artifact-backlink: missing --artifact-file\n' >&2; exit 2; }
[ -f "$ARTIFACT_FILE" ] || { printf 'render-artifact-backlink: artifact file not found: %s\n' "$ARTIFACT_FILE" >&2; exit 2; }
[ -n "$CHANGE_FILE" ]   || { printf 'render-artifact-backlink: missing --change-file\n' >&2; exit 2; }
[ -f "$CHANGE_FILE" ]   || { printf 'render-artifact-backlink: change file not found: %s\n' "$CHANGE_FILE" >&2; exit 2; }

# shellcheck source=/dev/null
source "$SCRIPTDIR/lib/docket-frontmatter.sh"

# Resolve config (metadata_branch + changes_dir). Mockable via DOCKET_CONFIG.
if [ "$DOCKET_CONFIG_EXPLICIT" -eq 1 ]; then
  cfg="$("$DOCKET_CONFIG" --export 2>/dev/null)" || { printf 'render-artifact-backlink: config resolution failed\n' >&2; exit 1; }
else
  cfg="$("${DOCKET_BASH_PATH:?run docket/install.sh}" "$DOCKET_CONFIG" --export 2>/dev/null)" || { printf 'render-artifact-backlink: config resolution failed\n' >&2; exit 1; }
fi
eval "$cfg"
METADATA_BRANCH="${METADATA_BRANCH:-docket}"
CHANGES_DIR="${CHANGES_DIR:-docs/changes}"

# Derive OWNER/REPO + GitHub mode from the artifact file's origin remote (render-change-links pattern),
# unless --repo is explicit.
GITHUB=0
if [ "$REPO_EXPLICIT" = 1 ]; then
  GITHUB=1
else
  url="$("$GIT" -C "$(dirname "$ARTIFACT_FILE")" remote get-url origin 2>/dev/null || true)"
  case "$url" in
    git@github.com:*|https://github.com/*|ssh://git@github.com/*)
      REPO="${url%.git}"
      REPO="${REPO#git@github.com:}"; REPO="${REPO#https://github.com/}"; REPO="${REPO#ssh://git@github.com/}"
      GITHUB=1 ;;
    *) GITHUB=0 ;;
  esac
fi

# Read id + title from the change frontmatter. fm_field is FRONTMATTER-SCOPED (first ---…--- block
# only): id/title are mandatory keys, but in a repo whose subject matter IS field names a body line
# opening `title:`/`id:` must never win (LEARNINGS frontmatter-anchored-read).
id="$(fm_field "$CHANGE_FILE" id)"
title="$(fm_field "$CHANGE_FILE" title)"
padded="$(printf '%04d' "$id" 2>/dev/null)" || padded="$id"

# Canonical repo-relative path of the change file, derived from the path the caller passed (its
# CURRENT canonical location — active/… or archive/…). Deterministic + offline.
sub="$(basename "$(dirname "$CHANGE_FILE")")"     # active | archive
relpath="$CHANGES_DIR/$sub/$(basename "$CHANGE_FILE")"

# Assemble the marker-bounded block into a temp file. The model-authored title is written with
# printf '%s' — VERBATIM, never a sed/string interpolation (which reinterprets & and \1 in a real
# title); the awk step below inserts the block bytes literally (LEARNINGS
# model-authored-values-are-untrusted-input). fm_field returns a single line => no newline injection.
block_file="$(mktemp)"; trap 'rm -f "$block_file"' EXIT
{
  printf '%s\n' "$START_MARKER"
  if [ "$GITHUB" = 1 ]; then
    printf '> ↩ **[Change %s — %s](https://github.com/%s/blob/%s/%s)**\n' "$padded" "$title" "$REPO" "$METADATA_BRANCH" "$relpath"
  else
    printf '> ↩ **Change %s — %s** — `%s`\n' "$padded" "$title" "$relpath"
  fi
  printf '%s\n' "$END_MARKER"
} > "$block_file"

out="$(mktemp)"
if grep -qF "$START_MARKER" "$ARTIFACT_FILE"; then
  # Replace the inclusive marker region in place (fixed-string match via index()).
  awk -v startm="$START_MARKER" -v endm="$END_MARKER" -v blk="$block_file" '
    BEGIN { while ((getline line < blk) > 0) block = block line ORS }
    index($0, startm) { printf "%s", block; inblk=1; next }
    inblk && index($0, endm) { inblk=0; next }
    !inblk { print }
  ' "$ARTIFACT_FILE" > "$out"
else
  # Insert the block as the very first lines, then one blank line, then the original content.
  awk -v blk="$block_file" '
    BEGIN { while ((getline line < blk) > 0) printf "%s\n", line; print "" }
    { print }
  ' "$ARTIFACT_FILE" > "$out"
fi
mv "$out" "$ARTIFACT_FILE"
```

- [ ] **Step 4: Make the script executable**

Run: `chmod +x scripts/render-artifact-backlink.sh`

- [ ] **Step 5: Run the test to verify it passes**

Run: `bash tests/test_render_artifact_backlink.sh`
Expected: PASS — all `ok` lines, exit 0.

- [ ] **Step 6: Write the contract `scripts/render-artifact-backlink.md`**

Model it on `scripts/render-change-links.md`. Full content:

```markdown
# render-artifact-backlink.sh — artifact back-link block renderer

## Purpose

Stamps a marker-bounded `docket:backlink` block at the **top** of an artifact (spec, plan, or
results), pointing home to its change file on `metadata_branch` at the change's current canonical
path. The reciprocal of `render-change-links.sh`'s forward `## Artifacts` block. Frontmatter
(`id`, `title`) + the change-file path are the single source of truth; this script is the **sole
writer** of the block (ADR-0012 script-vs-model boundary). Skills never construct or patch the
block by hand. Offline: no network, no `gh`. Deterministic and idempotent: same inputs →
byte-identical block. Introduced in change 0136.

## Usage

```
render-artifact-backlink.sh --artifact-file FILE --change-file CHANGE [--repo OWNER/REPO]
```

| Flag | Required | Description |
|---|---|---|
| `--artifact-file FILE` | yes | The spec/plan/results markdown file to update in place. |
| `--change-file CHANGE` | yes | The change file at its current canonical path (`active/…` or `archive/…`). `id` + `title` are read from its frontmatter; the URL path is derived from this path. |
| `--repo OWNER/REPO` | no | Build GitHub `blob/` URLs. Defaults to deriving `OWNER/REPO` from the artifact file's `origin` remote. Absent or non-GitHub remote: bare code-formatted path fallback. |

Mock seams: `GIT="${GIT:-git}"`, `DOCKET_CONFIG="${DOCKET_CONFIG:-<scriptdir>/docket-config.sh}"`.

## Behavior

**Validation.** Exits 2 if `--artifact-file` or `--change-file` is missing, does not exist, or an
unknown flag is passed. Exits 1 if `docket-config.sh --export` fails.

**Config.** Resolves `METADATA_BRANCH` and `CHANGES_DIR` from `docket-config.sh --export`.

**Link construction.** Reads `id` and `title` from `--change-file` frontmatter via the
frontmatter-scoped `fm_field` (first `---…---` block only). The change's repo-relative path is
`<CHANGES_DIR>/<active|archive>/<basename>`, derived from the path the caller passed — its current
canonical location, so `terminal_publish` never changes the link target. GitHub mode links to
`blob/<metadata_branch>/<relpath>`; fallback renders the bare code-formatted path.

**Block shape.**
```
<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change NNNN — <title>](<url>)**      # GitHub mode
> ↩ **Change NNNN — <title>** — `<relpath>` # fallback
<!-- docket:backlink:end -->
```

**Placement.** If the start marker exists, the inclusive marker region is replaced in place via
`awk`. If absent, the block is inserted as the very first lines of the file, followed by one blank
line, then the original content. No template seeding is needed — superpowers artifacts are not
docket-templated, so first-write always inserts.

**Untrusted title.** The model-authored `title` is written with `printf '%s'` into a block temp
file that `awk` inserts verbatim — never a `sed`/string-interpolated replacement. `fm_field`
returns a single line, so no structural/newline injection.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Block written (or unchanged). |
| 1 | `docket-config.sh` resolution failed. |
| 2 | Missing/invalid argument (`--artifact-file`/`--change-file` absent or missing, unknown flag). |

## Invariants

- **Sole writer.** The `docket:backlink` block is never hand-edited. Re-run to regenerate.
- **In-place edit.** Modifies `--artifact-file` via a temp file + `mv`; the caller commits.
- **Offline.** No network calls in either mode.
- **Deterministic.** Same inputs → byte-identical block.
- **No git writes.** Never touches the git index; the caller owns the commit.
- **Uniform target.** The link always points to the change on `metadata_branch`; `terminal_publish`
  changes only whether the close-out re-render fires.
```

- [ ] **Step 7: Run the contract-coverage test**

Run: `bash tests/test_script_contracts_coverage.sh`
Expected: PASS — `contract present for render-artifact-backlink.sh` and `script present for render-artifact-backlink.md`.

- [ ] **Step 8: Commit**

```bash
git add scripts/render-artifact-backlink.sh scripts/render-artifact-backlink.md tests/test_render_artifact_backlink.sh
git commit -m "feat(0136): render-artifact-backlink.sh — the back-link block renderer

Claude-Session: https://claude.ai/code/session_01K8zaLYGgiCcMXMs96qnWmo"
```

---

### Task 2: Wire the renderer into the `docket.sh` facade

**Files:**
- Modify: `scripts/docket.sh` (the `WRAPPED_OPS` array + the usage-comment op list)
- Modify: `scripts/docket.md` (the op inventory table)
- Test: `tests/test_docket_facade.sh` (existing — enforces `docket.sh` op set == `docket.md` op set)

**Interfaces:**
- Consumes: Task 1's `render-artifact-backlink.sh`.
- Produces: `docket.sh render-artifact-backlink …` routes to the helper, so skills/close-out invoke it through the single facade.

- [ ] **Step 1: Run the facade test to confirm the current green baseline**

Run: `bash tests/test_docket_facade.sh`
Expected: PASS (baseline before edits).

- [ ] **Step 2: Add the op to `WRAPPED_OPS` in `scripts/docket.sh`**

In the `WRAPPED_OPS="…"` line, add `render-artifact-backlink` immediately after `render-change-links`:

```
WRAPPED_OPS="docket-status board-refresh archive-change terminal-publish cleanup-feature-branch github-mirror sync-integration-branch render-change-links render-artifact-backlink render-adr-index render-learnings-index adr-checks board-checks reclaim-claims mint-stub runner-dispatch mark-publish-deferred backfill-change-types"
```

- [ ] **Step 3: Add the op to the usage-comment list in `scripts/docket.sh`**

In the header comment block (the `# Usage:` … lines that document each op), add a line right after the `render-change-links` comment line, matching the existing two-space-aligned style:

```
#   render-artifact-backlink  artifact back-link block (pure renderer)
```

- [ ] **Step 4: Add the row to the op inventory table in `scripts/docket.md`**

Immediately after the `render-change-links` row, add:

```
| `render-artifact-backlink` | `render-artifact-backlink.sh` | artifact back-link block (pure renderer) |
```

- [ ] **Step 5: Run the facade test to verify it still passes**

Run: `bash tests/test_docket_facade.sh`
Expected: PASS — the `docket.sh op set == docket.md documented op set` assertion holds with the new op present in both.

- [ ] **Step 6: Commit**

```bash
git add scripts/docket.sh scripts/docket.md
git commit -m "feat(0136): expose render-artifact-backlink through the docket.sh facade

Claude-Session: https://claude.ai/code/session_01K8zaLYGgiCcMXMs96qnWmo"
```

---

### Task 3: terminal-publish.sh re-stamps plan/results back-links inside the publish commit

**Files:**
- Modify: `scripts/terminal-publish.sh`
- Modify: `scripts/terminal-publish.md`
- Test: `tests/test_terminal_publish.sh` (existing — add a re-stamp case)

**Interfaces:**
- Consumes: Task 1's `render-artifact-backlink.sh` (invoked directly by path from terminal-publish's own script dir).
- Produces: under `--enabled true` + change mode, the single publish commit ALSO carries a freshly-stamped `docket:backlink` block on the change's `plan:`/`results:` files (when those files exist in the checked-out integration worktree), pointing at the archived change on `metadata_branch`. No additional commit. Inert under `--enabled false`, in `main`-mode, and for absent plan/results.

**Design notes for the implementer:**
- The copy-set (which includes `change_path`, the archived change file) is checked out into the transient `pub` worktree via `$GIT -C "$pub" checkout "$metaref" -- "${copyset[@]}"`. So `$pub/$change_path` exists on disk with the correct `archive/` parent — pass it as `--change-file`.
- The plan/results **field values** are read from the change frontmatter (already fetched to `$tmpd/change.md`): `plan="$(field "$tmpd/change.md" plan)"`, `results="$(field "$tmpd/change.md" results)"`. Each is repo-relative, so the on-disk path is `$pub/$plan` / `$pub/$results`.
- The re-stamp must run in BOTH copyset-checkout sites — the initial copy AND the CAS-rebase-conflict replay — so factor it into a function `restamp_build_artifacts` (mirroring `refresh_adr_index`) and call it right after each `checkout … -- "${copyset[@]}"`, then `git -C "$pub" add` the stamped files so they join the same commit.
- Per-artifact best-effort WITHIN the publish (LEARNINGS `best-effort-helper-on-a-sole-deliverable-path`, applied at the call site): a missing field or a field whose file is not present on the integration branch (e.g. a killed change whose plan never merged) is SKIPPED, never a die. The renderer's own strictness is unchanged; the best-effort posture is this caller's choice, matching the spec.
- The renderer derives repo (from `$pub`'s origin remote) and config (metadata_branch/changes_dir) itself, exactly as it does everywhere — no `--repo` needed.

- [ ] **Step 1: Write the failing re-stamp test**

Append to `tests/test_terminal_publish.sh` a case that drives a `--enabled true` change-mode publish where the change carries `plan:`/`results:` present on the integration branch, and asserts (a) each file gained a `docket:backlink` block pointing at the archived change, and (b) the publish still made exactly ONE commit. Follow the existing harness in that file for provisioning the fake origin, the docket/main branches, and the archived change fixture. Sketch of the new assertions (adapt variable names to the file's harness):

```bash
# ---- re-stamp: plan/results back-links land inside the single publish commit (change 0136) ----
# (Set up: an archived change 0142 on docket carrying plan: docs/superpowers/plans/p.md and
#  results: docs/results/r.md; those two files already present on the integration branch.)
commits_before="$(git -C "$origin_clone" rev-list --count "$INT")"
DOCKET_BASH_PATH="$(command -v bash)" GIT=git bash "$SCRIPT" \
  --id 142 --outcome done --integration-branch "$INT" --metadata-branch "$META" \
  --changes-dir docs/changes --adrs-dir docs/adrs --enabled true --remote origin >/dev/null 2>&1 \
  || no "restamp: publish exits 0"
git -C "$origin_clone" fetch -q origin
# plan + results on the integration branch now carry the back-link block
git -C "$work" checkout -q "origin/$INT" -- docs/superpowers/plans/p.md docs/results/r.md
grep -qF 'docket:backlink:start' "$work/docs/superpowers/plans/p.md" && ok "restamp: plan carries back-link" || no "restamp: plan carries back-link"
grep -qF 'docket:backlink:start' "$work/docs/results/r.md" && ok "restamp: results carries back-link" || no "restamp: results carries back-link"
# still exactly ONE new commit on the integration branch (the publish commit)
commits_after="$(git -C "$origin_clone" rev-list --count "origin/$INT")"
[ "$((commits_after - commits_before))" -eq 1 ] && ok "restamp: rides the single publish commit" || no "restamp: rides the single publish commit ($commits_before -> $commits_after)"
```

Also add a companion assertion that with `--enabled false` the plan/results files are left untouched (no `docket:backlink` block) — reuse the existing `--enabled false` no-op case in the file, adding a grep that the block is absent.

- [ ] **Step 2: Run the test to verify the new case fails**

Run: `bash tests/test_terminal_publish.sh`
Expected: FAIL on the new `restamp: …` assertions (the block is not yet written).

- [ ] **Step 3: Add the `restamp_build_artifacts` function to `scripts/terminal-publish.sh`**

Define it near `refresh_adr_index()` (after `teardown()` is defined, before the copy/commit block). It is a no-op in ADR mode (no `$ID`) and best-effort per artifact:

```bash
# change 0136: re-stamp the change's plan/results back-links inside the publish commit, pointing at
# the archived change on the metadata branch. Change mode only; per-artifact best-effort — a missing
# field or a file not present on the integration branch is skipped, never a die (the publish's own
# success does not hinge on a cosmetic back-link). The renderer resolves repo + config itself.
restamp_build_artifacts(){
  [ -n "$ID" ] || return 0
  local art rel
  for rel in "$(field "$tmpd/change.md" plan)" "$(field "$tmpd/change.md" results)"; do
    [ -n "$rel" ] || continue
    art="$pub/$rel"
    [ -f "$art" ] || continue
    "$DOCKET_BASH_PATH" "$(dirname "$0")/render-artifact-backlink.sh" \
      --artifact-file "$art" --change-file "$pub/$change_path" >/dev/null 2>&1 || continue
    $GIT -C "$pub" add -- "$rel" || true
  done
}
```

Note: `$change_path` is in scope (set in the change branch above the worktree provisioning). `field` is sourced from `lib/docket-frontmatter.sh` at the top of terminal-publish.sh.

- [ ] **Step 4: Call `restamp_build_artifacts` at both copyset-checkout sites**

After the initial copy (right after `refresh_adr_index` on the first checkout):

```bash
$GIT -C "$pub" checkout "$metaref" -- "${copyset[@]}" || { teardown; die "checkout copyset failed"; }
refresh_adr_index
restamp_build_artifacts
```

And inside the CAS-rebase conflict branch, right after its `refresh_adr_index`:

```bash
    $GIT -C "$pub" checkout "$metaref" -- "${copyset[@]}"
    refresh_adr_index   # regenerate deterministically on conflict — never a 3-way-merge of the index
    restamp_build_artifacts
```

- [ ] **Step 5: Run the terminal-publish test to verify it passes**

Run: `bash tests/test_terminal_publish.sh`
Expected: PASS — the re-stamp assertions and the single-commit assertion hold; the `--enabled false` no-op leaves plan/results untouched.

- [ ] **Step 6: Update `scripts/terminal-publish.md`**

Add a subsection documenting the new re-stamp step: under `--enabled true` change mode, after the copy-set is checked out and before the publish commit, `restamp_build_artifacts` runs `render-artifact-backlink.sh` against the change's `plan:`/`results:` files if present on the integration-branch worktree, staged into the same commit. Document the guards: inert under `--enabled false` (knob guard exits first), inert in `main`-mode (mode guard exits first), inert in `--adr` mode (no `$ID`), and missing/absent plan/results is a skip, never a failure. State that it runs at both checkout sites (initial + CAS replay) so the rebased commit also carries it, and that it does not add a commit.

- [ ] **Step 7: Run the contract-coverage + facade tests**

Run: `bash tests/test_script_contracts_coverage.sh && bash tests/test_docket_facade.sh`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add scripts/terminal-publish.sh scripts/terminal-publish.md tests/test_terminal_publish.sh
git commit -m "feat(0136): terminal-publish re-stamps plan/results back-links in the publish commit

Claude-Session: https://claude.ai/code/session_01K8zaLYGgiCcMXMs96qnWmo"
```

---

### Task 4: Wire the creation-time stamps + close-out re-renders into the skills, and pin them with a coverage sentinel test

**Files:**
- Modify: `skills/docket-new-change/SKILL.md` (spec stamp in §2; kill close-out references the reference)
- Modify: `skills/docket-groom-next/SKILL.md` (spec stamp after spec write)
- Modify: `skills/docket-auto-groom/SKILL.md` (spec stamp after spec write)
- Modify: `skills/docket-implement-next/SKILL.md` (plan stamp in §4; results stamp in §6.5; PR-body back-link line in §7)
- Modify: `skills/docket-convention/references/terminal-close-out.md` (spec re-render in step 2; note plan/results re-render rides terminal-publish in step 3; best-effort PR-body re-render)
- Modify: `skills/docket-convention/SKILL.md` (add `render-artifact-backlink.sh` to the derived-view script family + name the `docket:backlink` block beside the `## Artifacts` block description)
- Create: `tests/test_artifact_backlink_coverage.sh`

**Interfaces:**
- Consumes: the facade op `docket.sh render-artifact-backlink` (Task 2) and terminal-publish's fold-in (Task 3).
- Produces: prose call sites (the producers) that the coverage sentinels anchor on.

**Design notes for the implementer:**
- These skill files ARE the source of truth in this repo (dogfooded); editing them is exactly the deliverable. They install to `~/.claude/skills/` via `install.sh` (a human step; not part of this build).
- Every stamp call goes through the facade: `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh render-artifact-backlink --artifact-file <path> --change-file <change>`.
- Spec stamps: the spec lives on `metadata_branch` beside the change; the stamp edit rides the same spec-write commit as the existing `render-change-links` call. Add the stamp instruction immediately adjacent to each existing "record `spec:` then render the `## Artifacts` block" step.
- Plan stamp (implement §4): after the plan file is written on the feature branch and `plan:` is recorded, stamp the plan's back-link on the feature branch so it rides the PR. `--change-file` is the change on `.docket/…` (metadata worktree) at its `active/` path.
- PR-body line (implement §7): when docket authors the PR body, include a back-link line at the top pointing to the change on `metadata_branch`. This is skill-side (no file), built with the same link logic — the renderer does NOT touch the PR body (per its contract). Phrase it as a first line of the PR body, e.g. `↩ Change <padded-id> — <title>` linking to the change on docket. Best-effort at close-out re-render.
- Results stamp (§6.5): when a results file is written in the feature worktree, stamp its back-link there too, committed with the results file.
- The coverage sentinels must anchor on the PRODUCER paragraph (LEARNINGS `specified-but-unreachable`) — grep for `docket.sh render-artifact-backlink` in each skill that performs an on-disk stamp, and for the PR-body back-link instruction in `docket-implement-next`. Since a skill that stamps the spec is a MIRROR set with the change-links coverage (same skills), model it on `tests/test_change_links_coverage.sh`.

- [ ] **Step 1: Write the failing coverage sentinel test**

Create `tests/test_artifact_backlink_coverage.sh`:

```bash
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

# (2) Every skill that WRITES a spec/plan/results artifact invokes the renderer through the facade.
SPEC_SKILLS=( docket-new-change docket-groom-next docket-auto-groom )
for s in "${SPEC_SKILLS[@]}"; do
  f="$ROOT/skills/$s/SKILL.md"
  if grep -qF 'docket.sh render-artifact-backlink' "$f"; then ok "$s stamps the spec back-link"; else no "$s stamps the spec back-link"; fi
done

# implement-next stamps plan (§4) and results (§6.5) on disk, and adds a PR-body back-link (§7).
impl="$ROOT/skills/docket-implement-next/SKILL.md"
if grep -qF 'docket.sh render-artifact-backlink' "$impl"; then ok "docket-implement-next stamps plan/results back-links"; else no "docket-implement-next stamps plan/results back-links"; fi
if grep -qiE 'back-link line|back-link at the top of the (PR|body)|docket:backlink' "$impl"; then ok "docket-implement-next includes a PR-body back-link"; else no "docket-implement-next includes a PR-body back-link"; fi

# (3) The terminal close-out re-renders the spec back-link at close-out (producer paragraph).
tco="$ROOT/skills/docket-convention/references/terminal-close-out.md"
if grep -qF 'docket.sh render-artifact-backlink' "$tco"; then ok "close-out re-renders the spec back-link"; else no "close-out re-renders the spec back-link"; fi

# (4) The convention names the renderer in the derived-view script family.
if grep -qF 'render-artifact-backlink.sh' "$ROOT/skills/docket-convention/SKILL.md"; then ok "convention names the back-link renderer"; else no "convention names the back-link renderer"; fi

exit $fail
```

- [ ] **Step 2: Run the coverage test to verify it fails**

Run: `bash tests/test_artifact_backlink_coverage.sh`
Expected: FAIL — every skill-wiring assertion is `NOT OK` (nothing wired yet); the script-present assertion passes (Task 1 built it).

- [ ] **Step 3: Wire the spec stamp into `docket-new-change` §2**

In `skills/docket-new-change/SKILL.md` §2, immediately after the existing sentence that records `spec:` and runs `docket.sh render-change-links …`, add a sibling instruction to stamp the spec's back-link:

> After writing `spec:` and regenerating the `## Artifacts` block, stamp the spec's back-link home (the reciprocal of the forward block): `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh render-artifact-backlink --artifact-file .docket/<spec-path> --change-file .docket/<changes_dir>/active/<id>-<slug>.md` — the block edit rides the same spec-write commit; the renderer is the sole writer of the `docket:backlink` block.

- [ ] **Step 4: Wire the spec stamp into `docket-groom-next` and `docket-auto-groom`**

In each of `skills/docket-groom-next/SKILL.md` and `skills/docket-auto-groom/SKILL.md`, find the step where the groomed spec is written and `spec:` recorded (the `render-change-links` call, if present, or the spec-write commit step), and add the same `docket.sh render-artifact-backlink --artifact-file <spec> --change-file <change>` instruction so the spec is stamped in the same commit. (If a skill does not currently call `render-change-links`, add the back-link stamp at its spec-write/commit step regardless — the spec is still an artifact that must carry the block.)

- [ ] **Step 5: Wire the plan + PR-body + results stamps into `docket-implement-next`**

In `skills/docket-implement-next/SKILL.md`:
- **§4 (Worktree + plan):** after "Record the plan path in `plan:` per the field-write rule," add: after the plan file is written on the feature branch, stamp its back-link there — `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh render-artifact-backlink --artifact-file .worktrees/<slug>/<plan-path> --change-file .docket/<changes_dir>/active/<id>-<slug>.md` — committed with the plan on `feat/<slug>` so it rides the PR.
- **§6.5 (Results close-out):** when a results file is authored in the feature worktree, add a sentence to stamp its back-link there with the same renderer, committed with the results file.
- **§7 (PR + stop):** in the finish step, add: when docket authors the PR body, prepend a back-link line pointing to the change on `metadata_branch` — e.g. a first body line `↩ Change <padded-id> — <title>` linking to the change file on `docket` (built with the same GitHub-blob/bare-path logic; skill-side, since the PR body is not a file the renderer edits). Best-effort — never block the PR on it.

- [ ] **Step 6: Wire the close-out re-renders into `terminal-close-out.md`**

In `skills/docket-convention/references/terminal-close-out.md`:
- **Step 2 (re-render `## Artifacts`):** add a paragraph that, beside the existing `render-change-links` re-render on the archived change file, also re-renders the **spec** back-link on `docket` — `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh render-artifact-backlink --artifact-file .docket/<spec-path> --change-file .docket/<changes_dir>/archive/<UTC-date>-<id>-<slug>.md` — pointing at the now-archived change path, committed in the same follow-on metadata commit. Must-land like the forward re-render it accompanies (skip only if there is no `spec:`).
- **Step 3 (publish):** add a note that the change's **plan/results** back-links are re-rendered inside `terminal-publish.sh` when `terminal_publish: true` (Task 3), folded into the publish commit — no separate step, and a no-op under the default `terminal_publish: false` (stamp-once, accepted stale after archive).
- Add a best-effort note that the **PR body** back-link is re-rendered via `gh pr edit` at close-out where a driver edits the PR body — best-effort like the GitHub board mirror (a network failure logs and continues, never aborts close-out). Keep this light: the reference owns ordering; the driver skills own whether they touch the PR body.

- [ ] **Step 7: Name the renderer in the convention's derived-view family**

In `skills/docket-convention/SKILL.md`:
- In the **Derived-view script family** paragraph, add `render-artifact-backlink.sh` (the per-artifact `docket:backlink` back-link renderer; offline, GitHub-blob-or-bare-path) alongside `render-change-links.sh`.
- In the `## Artifacts` block bullet (change body sections), add a sibling sentence naming the reciprocal `docket:backlink` block stamped at the top of each artifact by `render-artifact-backlink.sh` (sole writer; marker-bounded `<!-- docket:backlink:start … -->` / `<!-- docket:backlink:end -->`).

- [ ] **Step 8: Run the coverage test to verify it passes**

Run: `bash tests/test_artifact_backlink_coverage.sh`
Expected: PASS — every producer sentinel is `ok`.

- [ ] **Step 9: Run the existing change-links coverage + skill-facade wiring tests (no regressions)**

Run: `bash tests/test_change_links_coverage.sh && bash tests/test_skill_facade_wiring.sh`
Expected: PASS (the new wiring did not disturb the 0035 sentinels or facade-wiring assertions).

- [ ] **Step 10: Commit**

```bash
git add skills/ tests/test_artifact_backlink_coverage.sh
git commit -m "feat(0136): stamp + re-render artifact back-links at every call site

Claude-Session: https://claude.ai/code/session_01K8zaLYGgiCcMXMs96qnWmo"
```

---

### Task 5: Whole-suite gate

**Files:** none (verification task).

- [ ] **Step 1: Run the entire test suite**

Run every test, not only the ones this change added (LEARNINGS `atomic-generated-write` war story: mode-loss and other breakage surfaces only in the whole-suite run). From the repo root:

```bash
for t in tests/test_*.sh; do printf '### %s\n' "$t"; bash "$t" || printf 'SUITE-FAIL: %s\n' "$t"; done
```

Expected: every test file exits 0; no `SUITE-FAIL` line. Investigate and fix any failure before proceeding — a red suite at the gate is a build failure, not noise.

- [ ] **Step 2: Verify script mode bits survived**

Run: `git diff --summary origin/main..HEAD | grep -i 'mode change' || echo "no mode changes"`
Expected: `no mode changes` (the renderer is a new `100755` file; no existing script demoted to `100644`). If a `mode change` line appears for an existing script, restore its bit with `chmod 755` and re-commit.

- [ ] **Step 3: Confirm the branch is clean and all tasks' commits are present**

Run: `git status --porcelain && git log --oneline origin/main..HEAD`
Expected: clean tree; commits for Tasks 1–4 present.

---

## Notes carried forward for the review + close-out steps (not build tasks)

- The plan file for THIS change (0136) is not itself back-stamped, because §4's plan-stamp instruction does not exist in the running skill at build time — expected bootstrap gap, not a defect.
- Review focus (whole-branch, LEARNINGS `foundational-test-discipline`): the sentinels are sampling; read the actual call-site prose for MEANING, verify each producer paragraph truly invokes the renderer, and confirm the terminal-publish re-stamp reaches both checkout sites.
- Any non-obvious decision made during the build (e.g. the fallback block shape, the `fm_field` choice for id/title, the per-artifact best-effort posture inside terminal-publish) is a candidate for a `docket-adr` at review time.
