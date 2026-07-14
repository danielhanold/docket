# CWD-Independent Repo Root Anchor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Anchor every docket script's repo root to the **main worktree** instead of the caller's CWD, make the two destructive paths fail-closed, and give `docket-finalize-change` a durable root — killing defects D1 (cleanup deletes the remote branch then fails), D2 (preflight mints a nested `.docket/.docket`), and D3 (cleanup deletes the agent's own CWD).

**Architecture:** One new sourced helper, `scripts/lib/docket-root.sh`, owns the single idiom `git worktree list --porcelain | sed -n '1s/^worktree //p'` (git lists the **main worktree first**, and the list is reachable from every worktree in the set). Four consumers adopt it: `docket-config.sh` (its `REPO_DIR` default, plus a new `REPO_ROOT` literal in the **plain** export only), `lib/docket-preflight.sh` (absolute metadata-worktree path + a nested-worktree refusal), `cleanup-feature-branch.sh` (absolute target + a fail-closed CWD refusal placed before **both** destructive steps), and `docket-status.sh` / `render-change-links.sh` (absolute `mw`, which brings the currently-dead artifacts-refresh block at `docket-status.sh:363` to life — budgeted and tested here). The `docket-finalize-change` skill then runs its merge/metadata/cleanup steps from the `REPO_ROOT` literal, which is the only half no script can fix.

**Tech Stack:** POSIX-ish bash (`set -uo pipefail`), git plumbing, the repo's hand-rolled `assert`-based test suite (`tests/test_*.sh`, each a standalone `bash` script printing `ok - …` / `NOT OK - …`).

## Global Constraints

- **The durable root is the MAIN worktree, never `.docket/`.** In `main`-mode there is no `.docket/` (`docket-config.sh` sets `METADATA_WORKTREE=.`), and `.docket/` is itself a linked worktree — the exact shape that misresolves.
- **Skills must NOT derive the root as `dirname $METADATA_WORKTREE`** — in `main`-mode `METADATA_WORKTREE` *is* the repo root, so `dirname` yields the repo's parent. They read the `REPO_ROOT` literal from the `preflight` block.
- **`REPO_ROOT` is emitted in `plain` format ONLY.** `scripts/ensure-claude-settings.sh:24` sets its own `REPO_ROOT` and `eval`s the **shell** export (line 33), reading it after (lines 38, 74) — a shell-format `REPO_ROOT` would silently capture that name.
- **`--repo-dir` keeps its override semantics verbatim** (the whole existing `test_docket_config.sh` suite passes it). Only the *default* changes.
- **`archive-change.sh:53` is correct as-is** — its `git -C "$CHANGES_DIR" rev-parse --show-toplevel` resolves the worktree of the *passed* changes dir, which is what it wants. Do **not** touch it.
- **Do not make the facade (`scripts/docket.sh`) `cd`** — it would silently re-resolve every caller-supplied relative path argument across all wrapped ops.
- **Not-a-repo must stay a soft fallback**, never a new hard error: when main-worktree resolution comes back empty, fall back to the pre-0075 value so the existing not-a-repo gates emit their existing messages.
- **Guards are code (LEARNINGS):** every new assert must be mutation-tested — strip the feature, watch it redden — and every new test must be proven able to *fire*. Anchor an assert to the unique phrase its target owns; never a blunt `! grep` over a literal that can legitimately appear elsewhere.
- **Run the WHOLE suite** (`for f in tests/test_*.sh; do bash "$f" >/dev/null 2>&1 || echo "FAILED: $f"; done`) at the end of every task, not only the tests the task touched.
- Suite baseline: capture `bash tests/test_*.sh` results on the unmodified base BEFORE starting (Task 0 step), so an environment-bound RED is never mistaken for a regression.

---

### Task 0: Baseline the suite

**Files:** none (read-only)

- [ ] **Step 1: Record the pre-change suite state**

```bash
cd /Users/homer/dev/docket/.worktrees/cwd-independent-repo-root-anchor
for f in tests/test_*.sh; do bash "$f" >/dev/null 2>&1 || echo "FAILED: $f"; done
```

Expected: no output (all green), or a *recorded* list of pre-existing failures. Save that list — any test in it that is still red at the end is NOT a regression, and any test NOT in it that goes red IS one.

---

### Task 1: The shared root anchor (`scripts/lib/docket-root.sh`)

**Files:**
- Create: `scripts/lib/docket-root.sh`
- Test: `tests/test_docket_root.sh` (create)

**Interfaces:**
- Consumes: nothing (leaf helper; honors the `GIT="${GIT:-git}"` mock seam).
- Produces — used verbatim by Tasks 2, 3, 4, 5:
  - `docket_main_worktree [dir]` → prints the **absolute** path of the main worktree of the repo containing `dir` (default `$PWD`); prints nothing and returns 0 when `dir` is not in a git repo.
  - `docket_anchor_path <path> [dir]` → prints `<path>` made absolute against that main worktree: an absolute path passes through unchanged; `.` (or empty) becomes the root; a relative path is joined to it; when the repo cannot be resolved, prints `<path>` unchanged.
  - `docket_metadata_worktree` → prints the metadata worktree **absolute**, derived from the `DOCKET_MODE` / `METADATA_WORKTREE` vars already in scope.

- [ ] **Step 1: Write the failing test**

Create `tests/test_docket_root.sh`:

```bash
#!/usr/bin/env bash
# tests/test_docket_root.sh — hermetic tests for scripts/lib/docket-root.sh (change 0075).
# The main-worktree anchor: every docket script must resolve the SAME primary root no matter which
# worktree (or subdirectory) the caller stands in. No network.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
LIB="$REPO/scripts/lib/docket-root.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
tmp="$(mktemp -d)"; tmp="$(cd "$tmp" && pwd -P)"; trap 'rm -rf "$tmp"' EXIT

# shellcheck source=/dev/null
. "$LIB"

# --- fixture: a repo with BOTH docket worktree shapes -------------------------
# work/            <- the main worktree
# work/.docket/    <- a linked worktree (the metadata worktree)
# work/.worktrees/feat-x/  <- a linked worktree (a feature worktree)
# work/sub/        <- a plain subdirectory of the main worktree
work="$tmp/work"
git init --quiet "$work"
git -C "$work" config user.email t@t.test
git -C "$work" config user.name  Test
: > "$work/README.md"
git -C "$work" add README.md
git -C "$work" commit --quiet -m init
git -C "$work" branch --quiet docket
git -C "$work" branch --quiet feat/x
git -C "$work" worktree add --quiet "$work/.docket" docket >/dev/null 2>&1
git -C "$work" worktree add --quiet "$work/.worktrees/feat-x" feat/x >/dev/null 2>&1
mkdir -p "$work/sub"

# --- (A) docket_main_worktree: the SAME root from all four CWDs ---------------
assert "main worktree from the main root" \
  '[ "$( cd "$work" && docket_main_worktree )" = "$work" ]'
assert "main worktree from the .docket/ metadata worktree (NOT .docket itself)" \
  '[ "$( cd "$work/.docket" && docket_main_worktree )" = "$work" ]'
assert "main worktree from a .worktrees/<slug> feature worktree" \
  '[ "$( cd "$work/.worktrees/feat-x" && docket_main_worktree )" = "$work" ]'
assert "main worktree from a plain subdirectory" \
  '[ "$( cd "$work/sub" && docket_main_worktree )" = "$work" ]'

# The contrast that names the bug: --show-toplevel returns the LINKED worktree.
assert "CONTRAST: git rev-parse --show-toplevel returns the linked worktree, which is the defect" \
  '[ "$( cd "$work/.docket" && git rev-parse --show-toplevel )" != "$work" ]'

# --- (B) not a git repo => empty, never an error ------------------------------
outside="$tmp/outside"; mkdir -p "$outside"
assert "outside a git repo: empty output" \
  '[ -z "$( cd "$outside" && docket_main_worktree )" ]'
assert "outside a git repo: exit 0 (soft, never fatal)" \
  '( cd "$outside" && docket_main_worktree >/dev/null )'

# --- (C) explicit dir argument ------------------------------------------------
assert "explicit dir argument resolves that repo's main worktree" \
  '[ "$( cd "$outside" && docket_main_worktree "$work/.docket" )" = "$work" ]'

# --- (D) docket_anchor_path ---------------------------------------------------
assert "anchor: relative path joins the main worktree, from a linked worktree" \
  '[ "$( cd "$work/.docket" && docket_anchor_path .docket )" = "$work/.docket" ]'
assert "anchor: nested relative path joins the main worktree" \
  '[ "$( cd "$work/.worktrees/feat-x" && docket_anchor_path .worktrees/feat-x )" = "$work/.worktrees/feat-x" ]'
assert "anchor: '.' resolves to the main worktree itself (main-mode shape)" \
  '[ "$( cd "$work/sub" && docket_anchor_path . )" = "$work" ]'
assert "anchor: './x' does not produce a doubled slash-dot" \
  '[ "$( cd "$work/sub" && docket_anchor_path ./docs )" = "$work/docs" ]'
assert "anchor: an ABSOLUTE path passes through untouched" \
  '[ "$( cd "$work/.docket" && docket_anchor_path /somewhere/else )" = "/somewhere/else" ]'
assert "anchor: outside a repo, the path passes through unchanged (soft fallback)" \
  '[ "$( cd "$outside" && docket_anchor_path .docket )" = ".docket" ]'

# --- (E) docket_metadata_worktree, from the config vars in scope --------------
assert "metadata worktree: docket-mode => <root>/.docket, resolved from a linked worktree" \
  '[ "$( cd "$work/.worktrees/feat-x" && DOCKET_MODE=docket METADATA_WORKTREE=.docket docket_metadata_worktree )" = "$work/.docket" ]'
assert "metadata worktree: main-mode ('.') => the repo root itself, never its parent" \
  '[ "$( cd "$work/sub" && DOCKET_MODE=main METADATA_WORKTREE=. docket_metadata_worktree )" = "$work" ]'
assert "metadata worktree: docket-mode default when METADATA_WORKTREE is unset" \
  '[ "$( cd "$work/sub" && DOCKET_MODE=docket docket_metadata_worktree )" = "$work/.docket" ]'
assert "metadata worktree: an already-absolute METADATA_WORKTREE is not re-anchored" \
  '[ "$( cd "$work/sub" && DOCKET_MODE=docket METADATA_WORKTREE=/abs/mw docket_metadata_worktree )" = "/abs/mw" ]'

# --- (F) the GIT mock seam is honored -----------------------------------------
assert "honors the GIT seam (a git that prints nothing => empty resolution)" \
  '[ -z "$( cd "$work" && GIT=true docket_main_worktree )" ]'

exit $fail
```

- [ ] **Step 2: Run it to verify it fails**

Run: `bash tests/test_docket_root.sh`
Expected: FAIL — the lib does not exist yet, so sourcing it errors (`No such file or directory`) and every assert is `NOT OK`.

- [ ] **Step 3: Write the implementation**

Create `scripts/lib/docket-root.sh`:

```bash
#!/usr/bin/env bash
# scripts/lib/docket-root.sh — the repo-root anchor (change 0075). The ONE implementation of
# "which tree am I operating on", so that no docket script derives its root from the caller's CWD.
#
# git lists the MAIN worktree FIRST in `worktree list --porcelain`, and the list is reachable from
# EVERY worktree in the set — so this resolves the repo's primary checkout even when the caller
# stands in the .docket/ metadata worktree or a .worktrees/<slug> feature worktree.
# `git rev-parse --show-toplevel` (and a bare `cd "$dir" && pwd -P`) instead return the LINKED
# worktree the caller happens to be in, which is the root cause of D1 (cleanup deleted the remote
# branch and then failed) and D2 (preflight minted a nested <repo>/.docket/.docket).
# See docs/superpowers/specs/2026-07-14-cwd-independent-repo-root-anchor-design.md.
#
#   docket_main_worktree [dir]       absolute path of the main worktree of the repo containing
#                                    <dir> (default $PWD); EMPTY when <dir> is not in a git repo.
#   docket_anchor_path <path> [dir]  <path> made absolute against that main worktree. Absolute
#                                    passes through; "." (or empty) becomes the root; a relative
#                                    path is joined to it. Not a repo => <path> unchanged, so the
#                                    caller's own not-a-repo gate still fires as before.
#   docket_metadata_worktree         the metadata worktree, ABSOLUTE, from the DOCKET_MODE /
#                                    METADATA_WORKTREE vars already in the caller's scope.
#
# Mock seam: GIT="${GIT:-git}".
# This file is a sourced helper: it is documented within its callers' contracts (docket-config.md,
# docket.md, docket-status.md, cleanup-feature-branch.md), not by a co-located .md
# (test_script_contracts_coverage.sh scopes lib/ out).

docket_main_worktree(){
  local dir="${1:-$PWD}" git="${GIT:-git}"
  "$git" -C "$dir" worktree list --porcelain 2>/dev/null | sed -n '1s/^worktree //p'
}

docket_anchor_path(){
  local path="$1" dir="${2:-$PWD}" root
  case "$path" in
    /*) printf '%s\n' "$path"; return 0 ;;   # already absolute — never re-anchor
  esac
  root="$(docket_main_worktree "$dir")"
  if [ -z "$root" ]; then
    printf '%s\n' "$path"                    # not a repo: soft fallback, caller's gate reports it
    return 0
  fi
  case "$path" in
    ""|.) printf '%s\n' "$root" ;;
    ./*)  printf '%s\n' "$root/${path#./}" ;;
    *)    printf '%s\n' "$root/$path" ;;
  esac
}

docket_metadata_worktree(){
  local mw
  if [ "${DOCKET_MODE:-}" = docket ]; then
    mw="${METADATA_WORKTREE:-.docket}"
  else
    mw="${METADATA_WORKTREE:-.}"
  fi
  docket_anchor_path "$mw"
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_docket_root.sh`
Expected: every line `ok - …`, exit 0.

- [ ] **Step 5: Mutation-test the new asserts (guards are code)**

Temporarily replace `docket_main_worktree`'s body with the defect it exists to prevent:

```bash
# in scripts/lib/docket-root.sh, TEMPORARILY:
docket_main_worktree(){ local dir="${1:-$PWD}" git="${GIT:-git}"; "$git" -C "$dir" rev-parse --show-toplevel 2>/dev/null; }
```

Run: `bash tests/test_docket_root.sh`
Expected: RED — the `.docket/`, `.worktrees/<slug>`, and both `docket_metadata_worktree`/`docket_anchor_path` linked-worktree asserts must all fail. If any of them stays green, that assert is decoration: fix it. **Revert the mutation** and confirm green again.

- [ ] **Step 6: Run the whole suite**

Run: `for f in tests/test_*.sh; do bash "$f" >/dev/null 2>&1 || echo "FAILED: $f"; done`
Expected: same set as the Task 0 baseline (nothing new).

- [ ] **Step 7: Commit**

```bash
git add scripts/lib/docket-root.sh tests/test_docket_root.sh
git commit -m "feat(0075): scripts/lib/docket-root.sh — the shared main-worktree anchor"
```

---

### Task 2: Anchor the resolver + emit `REPO_ROOT` (`scripts/docket-config.sh`)

**Files:**
- Modify: `scripts/docket-config.sh` (the `REPO_DIR="."` default near line 36; the source block near line 29; the emit block near line 285)
- Modify: `scripts/docket-config.md` (contract: the `REPO_DIR` default, the new `REPO_ROOT` key)
- Test: `tests/test_docket_config.sh` (append a new section)

**Interfaces:**
- Consumes: `docket_main_worktree` from Task 1.
- Produces: a `REPO_ROOT=<absolute main-worktree path>` line in the **plain** export only — the literal `docket.sh preflight` prints and Task 6's skill reads. `docket.sh preflight`/`env` already `exec docket-config.sh --export --format plain` (`scripts/docket.sh:45,48`), so no facade change is needed.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_docket_config.sh`, immediately **before** its final `exit $fail` line:

```bash
# --- (Z) change 0075: the repo anchor + REPO_ROOT ----------------------------
# The resolver must resolve the SAME primary root no matter which worktree/subdir the caller
# stands in. Every OTHER test in this file passes --repo-dir explicitly, so this section is the
# only coverage of the DEFAULT resolution — which is exactly the thing 0075 changes.
mkrepo "$tmp/z"
z_abs="$(cd "$tmp/z" && pwd -P)"
git -C "$tmp/z" branch --quiet docket
git -C "$tmp/z" worktree add --quiet "$tmp/z/.docket" docket >/dev/null 2>&1
mkdir -p "$tmp/z/sub"

# plain format from the MAIN ROOT: REPO_ROOT present and absolute.
z_root_plain="$(cd "$tmp/z" && bash "$SCRIPT" --export --format plain)"
assert "0075 plain: REPO_ROOT emitted, absolute, = the main worktree" \
  'printf "%s\n" "$z_root_plain" | grep -qxF "REPO_ROOT=$z_abs"'

# plain format from the .docket/ LINKED WORKTREE: byte-identical REPO_ROOT and METADATA_WORKTREE.
# Pre-0075 this yielded REPO_ROOT=<repo>/.docket and METADATA_WORKTREE=<repo>/.docket/.docket.
z_dk_plain="$(cd "$tmp/z/.docket" && bash "$SCRIPT" --export --format plain)"
assert "0075 plain: REPO_ROOT from the .docket/ worktree is the MAIN root, not .docket" \
  'printf "%s\n" "$z_dk_plain" | grep -qxF "REPO_ROOT=$z_abs"'
assert "0075 plain: METADATA_WORKTREE from .docket/ is <root>/.docket, NOT <root>/.docket/.docket" \
  'printf "%s\n" "$z_dk_plain" | grep -qxF "METADATA_WORKTREE=$z_abs/.docket"'

# plain format from a SUBDIRECTORY: the spec's stated behavior CHANGE (§1) — pinned deliberately.
z_sub_plain="$(cd "$tmp/z/sub" && bash "$SCRIPT" --export --format plain)"
assert "0075 plain: REPO_ROOT from <repo>/sub is the repo root (§1 behavior change, pinned)" \
  'printf "%s\n" "$z_sub_plain" | grep -qxF "REPO_ROOT=$z_abs"'
assert "0075 plain: METADATA_WORKTREE from <repo>/sub is <root>/.docket, not <sub>/.docket" \
  'printf "%s\n" "$z_sub_plain" | grep -qxF "METADATA_WORKTREE=$z_abs/.docket"'

# The machine-local layer is read from the REPO ROOT even when invoked from a subdirectory
# (§1: LCFG="$REPO_DIR/.docket.local.yml"). auto_groom is a global-able (non-fenced) key.
printf 'auto_groom: true\n' > "$tmp/z/.docket.local.yml"
z_sub_shell="$(cd "$tmp/z/sub" && bash "$SCRIPT" --export)"
AUTO_GROOM=""; eval "$z_sub_shell"
assert "0075: <repo>/.docket.local.yml is read when invoked from <repo>/sub (§1 behavior change)" \
  '[ "$AUTO_GROOM" = true ]'
rm -f "$tmp/z/.docket.local.yml"

# REPO_ROOT is PLAIN-ONLY: ensure-claude-settings.sh sets its own REPO_ROOT and eval's the SHELL
# export, so a shell-format REPO_ROOT would silently capture that name. Assert BOTH directions so
# the guard is provably able to fire (a bare `! grep` that can never match proves nothing).
z_shell="$(cd "$tmp/z" && bash "$SCRIPT" --export)"
assert "0075 shell: REPO_ROOT is NOT emitted (would capture ensure-claude-settings.sh's own var)" \
  '! printf "%s\n" "$z_shell" | grep -q "^REPO_ROOT="'
assert "0075 control: the plain export DOES carry REPO_ROOT (proves the absence-assert can fire)" \
  'printf "%s\n" "$z_root_plain" | grep -q "^REPO_ROOT="'

# --repo-dir still overrides verbatim, from anywhere (the whole existing suite depends on it).
mkrepo "$tmp/z2"
z2_abs="$(cd "$tmp/z2" && pwd -P)"
z2_plain="$(cd "$tmp/z/.docket" && bash "$SCRIPT" --repo-dir "$tmp/z2" --export --format plain)"
assert "0075: --repo-dir still overrides the anchor verbatim" \
  'printf "%s\n" "$z2_plain" | grep -qxF "REPO_ROOT=$z2_abs"'
```

- [ ] **Step 2: Run it to verify it fails**

Run: `bash tests/test_docket_config.sh 2>&1 | grep 0075`
Expected: the `REPO_ROOT` asserts fail (`NOT OK`) — nothing emits `REPO_ROOT` yet — and the `.docket/`-CWD and `sub/`-CWD asserts fail, because the resolver still defaults to `.` (CWD).

- [ ] **Step 3: Write the implementation**

In `scripts/docket-config.sh`:

(a) Source the new lib next to the existing one (after the `SELF_DIR=` line, near line 29):

```bash
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
. "$SELF_DIR/lib/docket-gitignore-block.sh"
# shellcheck source=/dev/null
. "$SELF_DIR/lib/docket-root.sh"
```

(b) Change the `REPO_DIR` default (line ~36) from `REPO_DIR="."` to:

```bash
REPO_DIR=""   # empty => the MAIN worktree of the repo containing CWD (resolved after arg parsing)
```

(c) Immediately **after** the arg-parsing `while … done` loop (before the `die()` definition), insert:

```bash
# --- repo anchor (change 0075) -----------------------------------------------
# The default repo is the MAIN worktree of the repo containing CWD — never CWD itself. A script
# invoked from the .docket/ metadata worktree, a .worktrees/<slug> feature worktree, or any
# subdirectory must resolve the SAME primary root as one invoked from the top; `cd "$REPO_DIR" &&
# pwd -P` (below) would otherwise absolutize the LINKED worktree, which is what mints a nested
# <repo>/.docket/.docket (D2). `--repo-dir` still overrides verbatim. Not a git repo => fall back
# to CWD so the is-inside-work-tree gate below emits its standard "not a git repo" error.
if [ -z "$REPO_DIR" ]; then
  REPO_DIR="$(docket_main_worktree)"
  [ -n "$REPO_DIR" ] || REPO_DIR="."
fi
```

(d) In the `# --- emit ---` block, add `REPO_ROOT` directly after the `emit METADATA_WORKTREE` line:

```bash
  emit METADATA_WORKTREE "$MW_EMIT"
  # REPO_ROOT — PLAIN FORMAT ONLY (change 0075). The absolute main-worktree path; the literal
  # skills read from the `docket.sh preflight` block for a cwd-independent `cd`. It is deliberately
  # absent from the SHELL format: ensure-claude-settings.sh:24 sets its own REPO_ROOT and eval's
  # the shell export at :33, reading it at :38/:74 — emitting it there would silently capture that
  # name. (REPO_ABS is computed above, in the plain branch.)
  if [ "$FORMAT" = plain ]; then
    emit REPO_ROOT "$REPO_ABS"
  fi
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `bash tests/test_docket_config.sh`
Expected: every line `ok - …` (including the pre-existing sections — the `run()` helper passes `--repo-dir`, so they are untouched).

- [ ] **Step 5: Mutation-test**

Temporarily restore `REPO_DIR="."` as the default (delete the anchor block from step 3c). Run `bash tests/test_docket_config.sh` — the `.docket/`-CWD and `sub/`-CWD asserts must go RED. Restore the anchor. Then temporarily move `emit REPO_ROOT "$REPO_ABS"` out of its `if` (unconditional) — the "shell: REPO_ROOT is NOT emitted" assert must go RED. Restore.

- [ ] **Step 6: Update the contract**

In `scripts/docket-config.md`, update the `--repo-dir` description and the emitted-keys table:
- `--repo-dir DIR` — "repo to resolve against. **Default (change 0075): the MAIN worktree of the repo containing CWD**, not CWD itself, so a call from `.docket/`, `.worktrees/<slug>`, or a subdirectory resolves the same primary root. Falls back to CWD when CWD is not inside a git repo (the existing not-a-repo error then fires)."
- Add a `REPO_ROOT` row to the emitted-keys documentation: "absolute path of the main worktree. **`plain` format only** — the shell format omits it because `ensure-claude-settings.sh` defines its own `REPO_ROOT` and `eval`s the shell export."
- Note the §1 behavior change explicitly: invoked from `<repo>/sub/`, the resolver now reads `<repo>/.docket.local.yml` and targets `<repo>/.docket` (previously `<sub>/…`), and `--bootstrap` seeds `<repo>/.gitignore`.

- [ ] **Step 7: Run the whole suite, then commit**

```bash
for f in tests/test_*.sh; do bash "$f" >/dev/null 2>&1 || echo "FAILED: $f"; done
git add scripts/docket-config.sh scripts/docket-config.md tests/test_docket_config.sh
git commit -m "feat(0075): anchor the resolver to the main worktree; emit REPO_ROOT (plain only)"
```

Expected suite result: the Task 0 baseline set, nothing new. (`tests/test_ensure_claude_settings.sh` is the one to watch — it is the script whose own `REPO_ROOT` the plain-only rule protects.)

---

### Task 3: Preflight — absolute metadata worktree + the nested-worktree refusal (D2)

**Files:**
- Modify: `scripts/lib/docket-preflight.sh:30-46`
- Test: `tests/test_docket_preflight.sh` (fix the existing `:52` assert; append a new D2 section)

**Interfaces:**
- Consumes: `docket_main_worktree`, `docket_anchor_path` (Task 1).
- Produces: after `docket_preflight` returns, the caller's `METADATA_WORKTREE` is an **absolute** path in BOTH modes (docket-mode: `<root>/.docket`; main-mode: `<root>`). Task 5 depends on this.

- [ ] **Step 1: Write the failing test**

In `tests/test_docket_preflight.sh`, section (C) currently asserts the value stays relative. Replace that block (the `--- (C) PROCEED sets config vars in the caller's scope ---` section, through its `assert`) with:

```bash
# --- (C) PROCEED sets config vars in the caller's scope, METADATA_WORKTREE ABSOLUTE (0075) ------
work_abs="$(cd "$work" && pwd -P)"
DOCKET_MODE=""; METADATA_WORKTREE=""
( cd "$work" && . "$LIB" && CONFIG_EXPORT_CMD="bash $tmp/ok-export.sh" docket_preflight "$SCRIPTS" >/dev/null 2>&1 \
  && [ "$DOCKET_MODE" = docket ] && [ "$METADATA_WORKTREE" = "$work_abs/.docket" ] ); rc=$?
assert "PROCEED exposes resolved config vars, with METADATA_WORKTREE anchored ABSOLUTE (0075)" '[ "$rc" -eq 0 ]'

# --- (D) change 0075 / defect D2: preflight from INSIDE the metadata worktree -------------------
# Pre-0075 this created a real <repo>/.docket/.docket worktree and still exited 0. The metadata
# worktree path must be built from the MAIN worktree, so running preflight from a linked worktree
# is a no-op with respect to the worktree set.
before="$(git -C "$work" worktree list --porcelain | grep -c '^worktree ')"
( cd "$work/.docket" && . "$LIB" && CONFIG_EXPORT_CMD="bash $tmp/ok-export.sh" docket_preflight "$SCRIPTS" ) >/dev/null 2>"$tmp/d2.err"; rc=$?
after="$(git -C "$work" worktree list --porcelain | grep -c '^worktree ')"
assert "D2: preflight from inside .docket/ returns zero" '[ "$rc" -eq 0 ]'
assert "D2: preflight from inside .docket/ creates NO second worktree" '[ "$before" = "$after" ]'
assert "D2: no nested <repo>/.docket/.docket directory was minted" '[ ! -d "$work/.docket/.docket" ]'
assert "D2: the worktree list contains no nested .docket/.docket entry" \
  '! git -C "$work" worktree list --porcelain | grep -q "^worktree .*/\.docket/\.docket$"'

# --- (E) D2, the harder shape: the target does not yet exist under the caller's CWD -------------
# A fresh clone whose .docket/ has NOT been created yet, with the caller standing in a linked
# feature worktree. The relative ".docket" would resolve under THAT worktree.
work2="$tmp/dk2"
git clone --quiet "$bare" "$work2" 2>/dev/null
git -C "$work2" config user.email t@t.test; git -C "$work2" config user.name Test
git -C "$work2" fetch --quiet origin docket
git -C "$work2" branch --quiet feat/y
git -C "$work2" worktree add --quiet "$work2/.worktrees/feat-y" feat/y >/dev/null 2>&1
work2_abs="$(cd "$work2" && pwd -P)"
( cd "$work2/.worktrees/feat-y" && . "$LIB" && CONFIG_EXPORT_CMD="bash $tmp/ok-export.sh" docket_preflight "$SCRIPTS" ) >/dev/null 2>"$tmp/d2b.err"; rc=$?
assert "D2b: preflight from a feature worktree returns zero" '[ "$rc" -eq 0 ]'
assert "D2b: the metadata worktree was created at the MAIN root" '[ -d "$work2_abs/.docket" ]'
assert "D2b: NOT under the feature worktree" '[ ! -d "$work2/.worktrees/feat-y/.docket" ]'

# --- (F) the nested-target guard refuses rather than creating debris ----------------------------
# Force the pathological target directly: a metadata worktree path INSIDE an existing LINKED
# worktree is never legitimate, so preflight must refuse (non-zero) and create nothing.
printf 'BOOTSTRAP=PROCEED\nDOCKET_MODE=docket\nMETADATA_BRANCH=docket\nMETADATA_WORKTREE=%s\nINTEGRATION_BRANCH=main\nCHANGES_DIR=docs/changes\n' \
  "$work2_abs/.worktrees/feat-y/.docket" > "$tmp/nested.env"
mkexport "$tmp/nested.env" "$tmp/nested-export.sh"
( cd "$work2" && . "$LIB" && CONFIG_EXPORT_CMD="bash $tmp/nested-export.sh" docket_preflight "$SCRIPTS" ) >/dev/null 2>"$tmp/nested.err"; rc=$?
assert "D2 guard: a metadata target inside a LINKED worktree is refused (non-zero)" '[ "$rc" -ne 0 ]'
assert "D2 guard: the refusal explains itself on stderr" 'grep -qi "inside an existing worktree" "$tmp/nested.err"'
assert "D2 guard: nothing was created at the refused target" '[ ! -d "$work2_abs/.worktrees/feat-y/.docket" ]'
```

Note: section (B) already builds `$work` **with** a `.docket` worktree (its `docket-mode PROCEED created the metadata worktree` assert) and `$bare` is in scope — (D)/(E)/(F) reuse both.

- [ ] **Step 2: Run it to verify it fails**

Run: `bash tests/test_docket_preflight.sh`
Expected: (C) fails (`METADATA_WORKTREE` is still the relative `.docket`), (D) fails (a nested `.docket/.docket` IS minted), (F) fails (no guard exists).

- [ ] **Step 3: Write the implementation**

Replace the body of `docket_preflight` from the `if [ "${DOCKET_MODE:-}" = docket ]` line to the end of the function (`scripts/lib/docket-preflight.sh:30-46`) with:

```bash
  # --- repo anchor (change 0075, defect D2) ------------------------------------------------
  # The eval'd SHELL format keeps METADATA_WORKTREE relative (".docket" / "."), and git — plus the
  # -d test below — would resolve that against the CALLER's CWD. Run from <repo>/.docket that
  # created a real <repo>/.docket/.docket worktree and still exited 0 (observed live, change 0073).
  # Anchor the path to the MAIN worktree before anything touches it. Not a git repo => leave the
  # value alone and let the git calls below fail exactly as they did before.
  local root
  root="$(docket_main_worktree)"
  METADATA_WORKTREE="$(docket_anchor_path "${METADATA_WORKTREE:-}")"

  if [ "${DOCKET_MODE:-}" = docket ]; then
    local wt="${METADATA_WORKTREE:-.docket}"
    local gitc="${root:-.}"
    if [ ! -d "$wt" ]; then
      # Fail-closed guard (change 0075): the metadata worktree must never land INSIDE a LINKED
      # worktree of this repo. The MAIN worktree legitimately contains it (<root>/.docket), so the
      # main worktree — the first entry of `worktree list` — is excluded; every other entry is a
      # linked worktree, and <repo>/.docket/.docket is never a legitimate target. Without this, a
      # caller that hands preflight a bad path silently mints debris that only `git worktree list`
      # reveals.
      if _docket_target_inside_linked_worktree "$git" "$gitc" "$wt"; then
        echo "docket-preflight: refusing to create metadata worktree at $wt — it is inside an existing worktree of this repo" >&2
        return 1
      fi
      "$git" -C "$gitc" worktree add "$wt" "$METADATA_BRANCH" >&2 2>/dev/null \
        || "$git" -C "$gitc" worktree add "$wt" "origin/$METADATA_BRANCH" >&2 \
        || { echo "docket-preflight: cannot create metadata worktree $wt" >&2; return 1; }
    fi
    # change 0063: skip the repo's shared git hooks on the metadata worktree (idempotent;
    # self-heals existing installs). Best-effort — a failure here must not block preflight.
    "$scripts_dir"/disable-worktree-hooks.sh --worktree "$wt" >&2 \
      || echo "docket-preflight: warning — could not disable hooks on $wt (continuing)" >&2
    "$git" -C "$wt" fetch origin "$METADATA_BRANCH" >&2 \
      && "$git" -C "$wt" pull --rebase origin "$METADATA_BRANCH" >&2 \
      || { echo "docket-preflight: metadata worktree sync failed" >&2; return 1; }
  else
    "$git" -C "${root:-.}" pull --rebase >&2 || { echo "docket-preflight: metadata sync failed" >&2; return 1; }
  fi
}

# _docket_target_inside_linked_worktree <git> <repo-dir> <target> — true (0) when <target> lies at
# or inside a LINKED worktree of the repo at <repo-dir>. The MAIN worktree (the first entry of
# `git worktree list --porcelain`) is deliberately EXCLUDED: it is the one worktree that
# legitimately contains the metadata worktree. Every other entry is a linked worktree, and a
# metadata worktree inside one of those is the D2 shape.
_docket_target_inside_linked_worktree(){
  local git="$1" repo_dir="$2" target="$3" first=1 wt
  while IFS= read -r wt; do
    [ -n "$wt" ] || continue
    if [ "$first" = 1 ]; then first=0; continue; fi   # skip the main worktree
    case "$target/" in
      "$wt/"*) return 0 ;;
    esac
  done < <("$git" -C "$repo_dir" worktree list --porcelain 2>/dev/null | sed -n 's/^worktree //p')
  return 1
}
```

Also add the source line at the top of the file, above `docket_preflight(){`:

```bash
# shellcheck source=docket-root.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/docket-root.sh"
```

(`docket.sh` and `docket-status.sh` both `. "$SELF_DIR"/lib/docket-preflight.sh`, so sourcing the anchor here gives both of them the helpers for free — Task 5 relies on that.)

- [ ] **Step 4: Run the tests to verify they pass**

Run: `bash tests/test_docket_preflight.sh`
Expected: all `ok - …`, including the four D2 asserts and the three guard asserts.

- [ ] **Step 5: Mutation-test the D2 asserts**

Temporarily drop the anchor line (`METADATA_WORKTREE="$(docket_anchor_path …)"`). Run `bash tests/test_docket_preflight.sh` from a shell whose CWD is the repo root — the (C), (D) and (E) asserts must go RED (a nested `.docket/.docket` reappears). Restore. Then temporarily neuter the guard (`_docket_target_inside_linked_worktree(){ return 1; }`) — the three (F) asserts must go RED. Restore, re-run green.

- [ ] **Step 6: Run the whole suite, then commit**

```bash
for f in tests/test_*.sh; do bash "$f" >/dev/null 2>&1 || echo "FAILED: $f"; done
git add scripts/lib/docket-preflight.sh tests/test_docket_preflight.sh
git commit -m "fix(0075): preflight anchors the metadata worktree + refuses a nested target (D2)"
```

Watch `tests/test_docket_facade.sh`, `tests/test_docket_status.sh`, `tests/test_metadata_worktree_hooks.sh`, and `tests/test_worktree_hooks_wiring.sh` — they all drive preflight. If one reddens, its fixture is asserting the *relative* value; update the assertion to the absolute path (that is the intended change), never loosen the anchor.

---

### Task 4: Cleanup — absolute target + the fail-closed CWD refusal (D1/D3)

**Files:**
- Modify: `scripts/cleanup-feature-branch.sh` (lines 10–56 — the whole resolution + guard region)
- Modify: `scripts/cleanup-feature-branch.md` (contract)
- Test: `tests/test_closeout.sh` (append a new section — this is the suite that already covers cleanup; verify with `grep -n cleanup-feature-branch tests/test_closeout.sh`. If cleanup's tests live elsewhere, append to that file instead and keep the section text identical.)

**Interfaces:**
- Consumes: `docket_main_worktree` (Task 1).
- Produces: cleanup now exits non-zero **without touching anything** when the caller's CWD is at or inside `<root>/<worktrees-dir>/<slug>`; from every other CWD (including `.docket/`) it performs the full happy path.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_closeout.sh` (before its final `exit $fail`):

```bash
# --- change 0075 / defect D1: cleanup must be CWD-independent and fail-closed -------------------
# The whole class was untested: every pre-0075 cleanup test invoked from the main root, the ONE
# CWD where the relative target happened to resolve. From any linked worktree, `target` never
# existed => the worktree removal was skipped, `git branch -D` failed into `|| true`, and execution
# still REACHED `git push --delete`, which SUCCEEDED — partial, irreversible data loss reported as
# a failure. These three cases pin all three CWD classes.
CLEANUP="$REPO/scripts/cleanup-feature-branch.sh"

# d1_fixture <dir> : a clone with a bare origin, a .docket/ metadata worktree, and a feature
# worktree at .worktrees/wid with branch feat/wid pushed to origin.
d1_fixture(){
  local d="$1" bare="$1/origin.git" work="$1/work"
  mkdir -p "$d"
  git init --quiet --bare "$bare"
  git clone --quiet "$bare" "$work" 2>/dev/null
  git -C "$work" config user.email t@t.test
  git -C "$work" config user.name  Test
  git -C "$work" checkout --quiet -b main
  : > "$work/README.md"
  git -C "$work" add README.md
  git -C "$work" commit --quiet -m init
  git -C "$work" push --quiet -u origin main
  git -C "$work" branch --quiet docket
  git -C "$work" worktree add --quiet "$work/.docket" docket >/dev/null 2>&1
  git -C "$work" branch --quiet feat/wid
  git -C "$work" push --quiet origin feat/wid
  git -C "$work" worktree add --quiet "$work/.worktrees/wid" feat/wid >/dev/null 2>&1
}

# (1) MAIN ROOT — the unchanged happy path.
c1="$tmp/d1-main"; d1_fixture "$c1"
( cd "$c1/work" && bash "$CLEANUP" --slug wid ) >/dev/null 2>&1; rc=$?
assert "D1(main root): cleanup exits zero" '[ $rc -eq 0 ]'
assert "D1(main root): the worktree is removed" '[ ! -e "$c1/work/.worktrees/wid" ]'
assert "D1(main root): the local branch is deleted" \
  '! git -C "$c1/work" rev-parse --verify -q feat/wid >/dev/null'
assert "D1(main root): the remote branch is deleted" \
  '! git -C "$c1/work" ls-remote --exit-code origin feat/wid >/dev/null 2>&1'

# (2) FROM .docket/ — pre-0075 this deleted the REMOTE branch and exited 1, leaving the worktree
#     and the local branch behind. It must now be the same happy path as (1).
c2="$tmp/d1-docket"; d1_fixture "$c2"
( cd "$c2/work/.docket" && bash "$CLEANUP" --slug wid ) >/dev/null 2>&1; rc=$?
assert "D1(.docket/): cleanup exits zero (was 1 — the defect)" '[ $rc -eq 0 ]'
assert "D1(.docket/): the worktree is removed (was: survived)" '[ ! -e "$c2/work/.worktrees/wid" ]'
assert "D1(.docket/): the local branch is deleted (was: survived)" \
  '! git -C "$c2/work" rev-parse --verify -q feat/wid >/dev/null'
assert "D1(.docket/): the remote branch is deleted" \
  '! git -C "$c2/work" ls-remote --exit-code origin feat/wid >/dev/null 2>&1'

# (3) FROM INSIDE THE TARGET — the refusal. THIS is the assertion that would have caught the data
#     loss: the remote branch MUST still exist after a refusal. Nothing destructive may run.
c3="$tmp/d1-inside"; d1_fixture "$c3"
( cd "$c3/work/.worktrees/wid" && bash "$CLEANUP" --slug wid ) >/dev/null 2>"$tmp/d1-inside.err"; rc=$?
assert "D1(inside target): cleanup REFUSES (non-zero)" '[ $rc -ne 0 ]'
assert "D1(inside target): THE REMOTE BRANCH STILL EXISTS (no partial destruction)" \
  'git -C "$c3/work" ls-remote --exit-code origin feat/wid >/dev/null 2>&1'
assert "D1(inside target): the local branch still exists" \
  'git -C "$c3/work" rev-parse --verify -q feat/wid >/dev/null'
assert "D1(inside target): the worktree still exists" '[ -d "$c3/work/.worktrees/wid" ]'
assert "D1(inside target): the refusal names the CWD problem on stderr" \
  'grep -qi "cwd" "$tmp/d1-inside.err"'

# (4) FROM A SUBDIRECTORY OF THE TARGET — the guard is about containment, not equality.
c4="$tmp/d1-inside-sub"; d1_fixture "$c4"
mkdir -p "$c4/work/.worktrees/wid/deep/er"
( cd "$c4/work/.worktrees/wid/deep/er" && bash "$CLEANUP" --slug wid ) >/dev/null 2>&1; rc=$?
assert "D1(inside target, nested subdir): cleanup REFUSES" '[ $rc -ne 0 ]'
assert "D1(inside target, nested subdir): the remote branch still exists" \
  'git -C "$c4/work" ls-remote --exit-code origin feat/wid >/dev/null 2>&1'

# (5) The provenance guard is UNCHANGED: an out-of-tree target is still refused.
c5="$tmp/d1-prov"; d1_fixture "$c5"
mkdir -p "$tmp/outside-wt"
( cd "$c5/work" && bash "$CLEANUP" --slug wid --worktrees-dir "$tmp/outside-wt" ) >/dev/null 2>"$tmp/d1-prov.err"; rc=$?
assert "D1: the .worktrees/ provenance guard is unchanged (out-of-tree target refused)" '[ $rc -ne 0 ]'
```

`tests/test_closeout.sh` already defines `REPO`, `tmp`, `assert`, and `fail`; reuse them (do not redefine).

- [ ] **Step 2: Run it to verify it fails**

Run: `bash tests/test_closeout.sh 2>&1 | grep D1`
Expected: case (2) fails with the *exact defect* — the remote branch is gone but the worktree and local branch survive, rc 1. Case (3) fails — cleanup does not refuse, and the remote branch is deleted. Case (5) already passes (unchanged behavior).

- [ ] **Step 3: Write the implementation**

Replace `scripts/cleanup-feature-branch.sh` lines 30–56 (from `root=` to the end) with:

```bash
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/docket-root.sh
. "$SELF_DIR"/lib/docket-root.sh

# Capture the caller's CWD BEFORE anything cd's (change 0075). The guard below compares it against
# the target; a `cd` first would compare $root against itself and the guard could never fire.
caller_pwd="$(pwd -P)"

# The repo root is the MAIN worktree, never `git rev-parse --show-toplevel` — from the .docket/
# metadata worktree or a .worktrees/<slug> feature worktree that returns the LINKED worktree, which
# made `target` (below) resolve to nothing: the worktree removal was skipped, `git branch -D` fell
# into `|| true`, and execution still reached `git push --delete`, which SUCCEEDED. Partial,
# irreversible destruction that reported failure (defect D1).
root="$(docket_main_worktree)"
[ -n "$root" ] || die "not in a git repo"

canon(){ ( cd "$1" 2>/dev/null && pwd -P ); }   # realpath of an existing dir, else empty

# target is ABSOLUTE, anchored to the main worktree — so the removal block, the guards, and the
# postcondition below all mean the same thing from every CWD. An absolute --worktrees-dir is
# honored verbatim (the provenance guard still governs whether it may be removed).
case "$WORKTREES_DIR" in
  /*) target="$WORKTREES_DIR/$SLUG" ;;
  *)  target="$root/$WORKTREES_DIR/$SLUG" ;;
esac
allowed_root="$root/.worktrees"

# FAIL-CLOSED CWD GUARD (change 0075, defects D1+D3) — refuse when the caller stands AT or INSIDE
# the target. Placed before BOTH destructive steps (the worktree removal AND the remote delete):
# `git worktree remove --force` succeeds with a process CWD inside the target (the process merely
# orphans its CWD) and the caller's NEXT command then cannot start, so the only safe answer is to
# do nothing at all. Refusing takes away nothing that worked: from this CWD the pre-0075 script
# destroyed the remote branch and failed anyway.
target_rp="$(canon "$target")"
if [ -n "$target_rp" ]; then
  case "$caller_pwd/" in
    "$target_rp/"*)
      die "refusing to clean up feat/$SLUG: the caller's CWD is at or inside the target worktree ($caller_pwd) — cd to the repo root ($root) and re-run" ;;
  esac
fi

# provenance guard: the worktree, if present, must resolve under <root>/.worktrees/
if [ -e "$target" ]; then
  rp="$(canon "$target")"
  case "$rp/" in
    "$allowed_root/"*) ;;   # under .worktrees/ — allowed
    *) die "refusing to remove worktree outside .worktrees/: $rp" ;;
  esac
  $GIT -C "$root" worktree remove --force "$target" >/dev/null 2>&1 || die "worktree remove failed: $target"
fi

# delete local + remote feat/<slug> — anchored at the main worktree, never the caller's CWD
$GIT -C "$root" branch -D "feat/$SLUG" >/dev/null 2>&1 || true
if $GIT -C "$root" ls-remote --exit-code "$REMOTE" "feat/$SLUG" >/dev/null 2>&1; then
  $GIT -C "$root" push "$REMOTE" --delete "feat/$SLUG" >/dev/null 2>&1 || die "remote branch delete failed"
fi

# fail-closed self-verification
[ ! -e "$target" ] || die "postcondition: worktree still present"
$GIT -C "$root" rev-parse --verify -q "feat/$SLUG" >/dev/null && die "postcondition: local branch still present"
log "cleaned up feat/$SLUG"
exit 0
```

Keep lines 1–29 (the header comment, `set -uo pipefail`, `GIT`, the defaults, `die`/`log`, the arg loop, the `[ -n "$SLUG" ]` check) as they are — but update the header comment block to name the new guard:

```bash
# scripts/cleanup-feature-branch.sh — provenance-guarded teardown of a finished change's feature
# branch + worktree (change 0025). Removes the worktree ONLY if it resolves under the repo root's
# .worktrees/ (never the .docket/ metadata worktree, never an out-of-tree path), then deletes the
# local and remote feat/<slug> branch. Fail-closed: self-verifies both are gone.
#
# The repo root is the MAIN worktree and the target is ABSOLUTE (change 0075), so the script means
# the same thing from every CWD; and it REFUSES, before any destructive step, when the caller's CWD
# is at or inside the target worktree.
#
# Usage: cleanup-feature-branch.sh --slug S [--worktrees-dir DIR] [--remote R]
#
# Mock seam: GIT="${GIT:-git}".
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `bash tests/test_closeout.sh`
Expected: all `ok - …`, including the five new D1 cases.

- [ ] **Step 5: Mutation-test the refusal**

Temporarily delete the CWD guard block. Run `bash tests/test_closeout.sh 2>&1 | grep "D1(inside target)"` — the four asserts (especially **"THE REMOTE BRANCH STILL EXISTS"**) must go RED. Restore. Then temporarily revert `root=` to `$($GIT rev-parse --show-toplevel)` — case (2) (`D1(.docket/)`) must go RED. Restore, re-run green.

- [ ] **Step 6: Update the contract**

In `scripts/cleanup-feature-branch.md`, add to **Behavior** and **Invariants**:
- The repo root is resolved from the **main worktree** (`git worktree list --porcelain`, first entry), never `--show-toplevel`; `target` is absolute, so behavior is identical from every CWD (including `.docket/`).
- **Refusal (fail-closed):** when the caller's CWD is at or inside the target worktree, the script exits non-zero having attempted **no** destructive step — neither the worktree removal nor the remote branch delete. Remedy: `cd` to the repo root and re-run.
- Add an **Exit codes** row for the refusal (exit 1, message `refusing to clean up feat/<slug>: the caller's CWD is at or inside the target worktree …`).
- Note the D1 history explicitly: pre-0075, invoking from a linked worktree deleted the **remote** branch and *then* failed.

- [ ] **Step 7: Run the whole suite, then commit**

```bash
for f in tests/test_*.sh; do bash "$f" >/dev/null 2>&1 || echo "FAILED: $f"; done
git add scripts/cleanup-feature-branch.sh scripts/cleanup-feature-branch.md tests/test_closeout.sh
git commit -m "fix(0075): cleanup — absolute target + fail-closed CWD refusal (D1/D3)"
```

---

### Task 5: `docket-status.sh` — absolute `mw`, and the §5 landmine

**Files:**
- Modify: `scripts/docket-status.sh` (the six `mw=` resolution sites: lines ~77, ~203, ~222, ~307, ~394, ~412; and the artifacts-refresh block at ~363–370)
- Modify: `scripts/render-change-links.sh` (the `ADRS_DIR_LOCAL` fallback, lines ~43–52)
- Modify: `tests/test_render_board.sh` (the control fixture at ~986–999 — keep it a VERBATIM copy of `backlog_pass`)
- Modify: `scripts/docket-status.md` (contract: the artifacts-refresh step and its failure posture)
- Test: `tests/test_docket_status.sh` (append a §5 section)

**Interfaces:**
- Consumes: `docket_metadata_worktree` (Task 1), reachable because `docket-status.sh` sources `lib/docket-preflight.sh`, which now sources `lib/docket-root.sh` (Task 3).
- Produces: `$mw` is absolute in every `docket-status.sh` function, so `$archived` (built as `$mw/$CHANGES_DIR/archive/…`) is absolute — which is what makes the artifacts-refresh block's pathspec match for the first time.

**Why this task is the sharp edge:** `docket-status.sh:363`'s `git -C "$mw" status --porcelain -- "$archived"` is **dead today** — `$archived` carries the same *relative* `$mw` prefix, so under `git -C .docket` the pathspec matches nothing and the block never runs. An absolute `$mw` brings it alive, including its `return 0` on push failure, which would abandon **`terminal-publish` AND `cleanup`**. That early return must go.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_docket_status.sh` (before its final `exit $fail`):

```bash
# --- change 0075 §5: the artifacts-refresh block (docket-status.sh:363) ------------------------
# This block is DEAD pre-0075 (its pathspec carries the same relative $mw that `git -C "$mw"` is
# already rooted at, so it matches nothing). Anchoring $mw brings it alive for the first time — so
# it is tested here, with the REAL render-change-links.sh, not a no-op mock: a mock that omits the
# tool routes the test straight through the degrade branch and proves nothing (LEARNINGS).
#
# (i) the refreshed ## Artifacts block is actually COMMITTED on the metadata branch
# (ii) a failure inside the block does NOT abandon terminal-publish or cleanup

mkdir -p "$tmp/mock-a5"
# REAL archive-change.sh, render-change-links.sh, terminal-publish.sh, cleanup-feature-branch.sh —
# exec'd by absolute path so their own $(dirname "$0") lib resolution still finds their real files.
for s in archive-change.sh render-change-links.sh terminal-publish.sh cleanup-feature-branch.sh \
         board-refresh.sh render-board.sh board-checks.sh sync-integration-branch.sh; do
  printf '#!/usr/bin/env bash\nexec %q "$@"\n' "$REPO/scripts/$s" > "$tmp/mock-a5/$s"
  chmod +x "$tmp/mock-a5/$s"
done

# Reuse the existing gate_setup fixture builder (a clone + bare origin + .docket worktree + an
# `implemented` change 0060 with a merged PR + a feat/gate-thing worktree and branch).
a5="$tmp/a5-case"
gate_setup "$a5"

# Give the archived change a STALE ## Artifacts block, so render-change-links.sh has something to
# rewrite and the block's `status --porcelain` actually reports a dirty file.
a5_change="$(find "$a5/work/.docket/docs/changes/active" -name '0060-*.md' | head -n1)"
perl -0pi -e 's/(<!-- docket:artifacts:start[^\n]*-->\n)(.*?)(<!-- docket:artifacts:end -->)/$1| Artifact | Link |\n|---|---|\n| STALE | stale-placeholder |\n$3/s' "$a5_change"
git -C "$a5/work/.docket" add -A
git -C "$a5/work/.docket" commit -q -m "stale artifacts block"
git -C "$a5/work/.docket" push -q origin HEAD:docket

cat > "$tmp/fixture-a5.sh" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' \
  'BOOTSTRAP=PROCEED' \
  'METADATA_BRANCH=docket' \
  'INTEGRATION_BRANCH=main' \
  'DOCKET_MODE=docket' \
  'METADATA_WORKTREE=.docket' \
  'CHANGES_DIR=docs/changes' \
  'ADRS_DIR=docs/adrs' \
  'RESULTS_DIR=docs/results' \
  'BOARD_SURFACES=none' \
  'TERMINAL_PUBLISH=true'
EOF

(cd "$a5/work" && \
  CONFIG_EXPORT_CMD="bash $tmp/fixture-a5.sh" GH="$tmp/gh-gate.sh" \
  SCRIPTS_DIR="$tmp/mock-a5" \
  "$SCRIPT" --repo x/y >"$tmp/a5-out.txt" 2>"$tmp/a5-err.txt")
rc=$?
assert "0075 §5: the sweep exits zero" '[ $rc -eq 0 ]'
assert "0075 §5: the change is swept" 'grep -qE "^swept 60 " "$tmp/a5-out.txt"'
git -C "$a5/work" fetch origin docket >/dev/null 2>&1
assert "0075 §5: the refreshed ## Artifacts block is COMMITTED on the metadata branch (the block was DEAD pre-0075)" \
  '! git -C "$a5/work" show origin/docket:docs/changes/archive/2026-07-11-0060-gate-thing.md | grep -q "stale-placeholder"'
assert "0075 §5: the close-out still completed — terminal-publish landed the record on main" \
  'git -C "$a5/work" fetch origin main >/dev/null 2>&1; git -C "$a5/work" ls-tree -r --name-only origin/main | grep -q "docs/changes/archive/2026-07-11-0060-gate-thing.md"'
assert "0075 §5: the close-out still completed — cleanup removed the feature worktree" \
  '[ ! -e "$a5/work/.worktrees/gate-thing" ]'

# (ii) THE LANDMINE: make the artifacts-refresh PUSH fail, and prove the close-out still finishes.
# A non-fast-forwardable metadata remote makes `git -C "$mw" push` fail deterministically.
a5b="$tmp/a5b-case"
gate_setup "$a5b"
a5b_change="$(find "$a5b/work/.docket/docs/changes/active" -name '0060-*.md' | head -n1)"
perl -0pi -e 's/(<!-- docket:artifacts:start[^\n]*-->\n)(.*?)(<!-- docket:artifacts:end -->)/$1| Artifact | Link |\n|---|---|\n| STALE | stale-placeholder |\n$3/s' "$a5b_change"
git -C "$a5b/work/.docket" add -A
git -C "$a5b/work/.docket" commit -q -m "stale artifacts block"
git -C "$a5b/work/.docket" push -q origin HEAD:docket
# Diverge origin/docket behind the local worktree's back => the artifacts-refresh push is rejected.
# (A second clone pushes an unrelated commit onto docket AFTER the sweep's own pull.)
git -C "$a5b/work/.docket" remote set-url --push origin "$a5b/nonexistent-push-target.git"

(cd "$a5b/work" && \
  CONFIG_EXPORT_CMD="bash $tmp/fixture-a5.sh" GH="$tmp/gh-gate.sh" \
  SCRIPTS_DIR="$tmp/mock-a5" \
  "$SCRIPT" --repo x/y >"$tmp/a5b-out.txt" 2>"$tmp/a5b-err.txt")
rc=$?
assert "0075 §5(landmine): the sweep still exits zero when the artifacts push fails" '[ $rc -eq 0 ]'
assert "0075 §5(landmine): the push failure is REPORTED on the report channel" \
  'grep -qE "^sweep-failed 60 render-change-links push-failed$" "$tmp/a5b-out.txt"'
assert "0075 §5(landmine): the failure does NOT abandon cleanup — the feature worktree is gone" \
  '[ ! -e "$a5b/work/.worktrees/gate-thing" ]'
assert "0075 §5(landmine): the failure does NOT abandon cleanup — the remote feat branch is gone" \
  '! git -C "$a5b/work" ls-remote --exit-code origin feat/gate-thing >/dev/null 2>&1'
assert "0075 §5(landmine): the sweep still reports the change as swept" \
  'grep -qE "^swept 60 " "$tmp/a5b-out.txt"'
```

If `gate_setup`'s archived filename or change id differs, read the existing gate-disabled case (around line 880) and mirror its exact paths — do not invent them.

- [ ] **Step 2: Run it to verify it fails**

Run: `bash tests/test_docket_status.sh 2>&1 | grep 0075`
Expected: the "refreshed ## Artifacts block is COMMITTED" assert fails — the block is dead, so `stale-placeholder` survives into the archive. (The landmine asserts may *pass vacuously* pre-fix precisely because the block never runs; that is why the mutation test in step 5 is mandatory.)

- [ ] **Step 3: Write the implementation**

(a) In `scripts/docket-status.sh`, replace each of the **six** occurrences of

```bash
  local mw
  if [ "${DOCKET_MODE:-}" = docket ]; then mw="${METADATA_WORKTREE:-.docket}"; else mw="."; fi
```

with

```bash
  local mw
  mw="$(docket_metadata_worktree)"   # ABSOLUTE (change 0075) — a relative mw resolves against the
                                     # caller's CWD, which is what left the artifacts-refresh block
                                     # below dead and made a linked-worktree CWD misresolve.
```

(the sites at ~77 `board_pass`, ~203, ~222, ~307, ~394 `health_checks`, ~412 `emit_judgment`; keep the two `local mw="$1"` *parameter* lines at ~95 and ~325 as they are — they receive the already-resolved value).

(b) Fix the §5 landmine — the artifacts-refresh block at ~363. Replace:

```bash
  if [ -n "$("$GIT" -C "$mw" status --porcelain -- "$archived" 2>/dev/null)" ]; then
    "$GIT" -C "$mw" add "$archived" >&2
    "$GIT" -C "$mw" commit -q -m "docket($id): refresh artifacts links" >&2
    if ! "$GIT" -C "$mw" push >&2; then
      echo "sweep-failed $id render-change-links push-failed"
      return 0
    fi
  fi
```

with:

```bash
  # Change 0075 §5 — this block was DEAD before the $mw anchor (its pathspec carried the same
  # RELATIVE $mw that `git -C "$mw"` is already rooted at, so it matched nothing and the refreshed
  # ## Artifacts block was silently never committed). Anchoring brings it alive, and its old
  # `return 0` on a failed push would have ABANDONED terminal-publish AND cleanup — a stale link
  # block is a cosmetic problem; a skipped publish and an orphaned worktree + branch are not.
  # So: report the failure on the channel and CONTINUE the close-out.
  if [ -n "$("$GIT" -C "$mw" status --porcelain -- "$archived" 2>/dev/null)" ]; then
    if ! "$GIT" -C "$mw" add "$archived" >&2 \
      || ! "$GIT" -C "$mw" commit -q -m "docket($id): refresh artifacts links" >&2; then
      echo "sweep-failed $id render-change-links commit-failed"
    elif ! "$GIT" -C "$mw" push >&2; then
      echo "sweep-failed $id render-change-links push-failed"
    fi
  fi
```

(c) In `scripts/render-change-links.sh`, anchor the `ADRS_DIR_LOCAL` fallback (lines ~43–52). Add the source next to the frontmatter lib source:

```bash
# shellcheck source=/dev/null
source "$SCRIPTDIR/lib/docket-frontmatter.sh"
# shellcheck source=/dev/null
source "$SCRIPTDIR/lib/docket-root.sh"
```

and replace the fallback:

```bash
if [ -z "$ADRS_DIR_LOCAL" ]; then
  # change 0075: METADATA_WORKTREE arrives RELATIVE from the shell export and would otherwise
  # resolve against the caller's CWD. Anchor it to the main worktree. (Every in-repo caller passes
  # --adrs-dir explicitly; this is the fallback path, audited in the same pass as docket-status.)
  if [ -n "$METADATA_WORKTREE" ]; then
    ADRS_DIR_LOCAL="$(docket_anchor_path "$METADATA_WORKTREE")/$ADRS_DIR"
  else
    ADRS_DIR_LOCAL="$ADRS_DIR"
  fi
fi
```

(d) In `tests/test_render_board.sh` (~986–999), the control fixture copies `backlog_pass` **VERBATIM** from `docket-status.sh` and its comment says so. Update the two fixture lines so the claim stays true:

```bash
printf '%s\n' '#!/usr/bin/env bash' \
  'backlog_pass(){' \
  '  local mw' \
  '  mw="$(docket_metadata_worktree)"' \
  '  local cd_dir="$mw/$CHANGES_DIR"' \
  …
```

(leave the rest of the fixture, and the `render_board_write_free` guard itself, untouched — the guard tests redirect-taint, not root resolution). Re-run `bash tests/test_render_board.sh` and confirm the false-positive control still passes.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `bash tests/test_docket_status.sh` then `bash tests/test_render_board.sh` then `bash tests/test_render_change_links.sh`
Expected: all `ok - …`.

- [ ] **Step 5: Mutation-test the landmine (mandatory — the asserts pass vacuously without this)**

Temporarily restore the `return 0` inside the push-failure branch. Run `bash tests/test_docket_status.sh 2>&1 | grep "landmine"` — the two "does NOT abandon cleanup" asserts must go RED. If they stay green, the fixture is not actually reaching the block (the push is not failing, or the file is not dirty): fix the fixture until the mutation reddens, then restore the fix and re-run green. Then temporarily revert one `mw=` site to the relative form and confirm the "COMMITTED" assert reddens.

- [ ] **Step 6: Update the contract**

In `scripts/docket-status.md`, document under the sweep sequence: the artifacts-refresh step regenerates and **commits** the archived change's `## Artifacts` block on the metadata branch; a `commit`/`push` failure there emits `sweep-failed <id> render-change-links commit-failed|push-failed` and the close-out **continues** (`terminal-publish` and `cleanup` still run) — it is not a terminal abort. Note that `$mw` is absolute (change 0075).

- [ ] **Step 7: Run the whole suite, then commit**

```bash
for f in tests/test_*.sh; do bash "$f" >/dev/null 2>&1 || echo "FAILED: $f"; done
git add scripts/docket-status.sh scripts/docket-status.md scripts/render-change-links.sh \
        tests/test_docket_status.sh tests/test_render_board.sh
git commit -m "fix(0075): absolute mw in docket-status; the artifacts-refresh block commits and no longer abandons close-out"
```

---

### Task 6: Finalize's durable-root posture + the docs surface

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md`
- Modify: `scripts/docket.md` (the `preflight`/`env` key block — add `REPO_ROOT`)
- Test: `tests/test_finalize_gate.sh` (append a prose/wiring sentinel), or `tests/test_skill_facade_wiring.sh` if that is where skill-prose sentinels live — check both with `grep -ln "docket-finalize-change/SKILL.md" tests/*.sh` and append to the file that already scans that skill.

**Interfaces:**
- Consumes: the `REPO_ROOT` literal from Task 2's plain export (printed by `docket.sh preflight`).
- Produces: no code interface — a skill-prose posture plus its guard.

- [ ] **Step 1: Write the failing test**

Append to the test file that already scans `skills/docket-finalize-change/SKILL.md`:

```bash
# --- change 0075: finalize's durable-root posture ----------------------------------------------
# D3 is irreducibly skill-side: `git worktree remove --force` succeeds with the agent's CWD inside
# the target, but its NEXT command cannot start. No script can fix that (a child cannot change its
# parent's CWD), so the SKILL must run its close-out from the durable root.
FIN="$REPO/skills/docket-finalize-change/SKILL.md"
assert "0075: finalize names the REPO_ROOT literal as its durable root" \
  'grep -qF "REPO_ROOT" "$FIN"'
assert "0075: finalize instructs the close-out to run from the durable root" \
  '[ "$(grep -ciE "durable root" "$FIN")" -ge 1 ]'
# The anti-pattern the convention forbids: deriving the root from the metadata worktree. In
# main-mode METADATA_WORKTREE *is* the repo root, so dirname would yield the repo's PARENT.
assert "0075: finalize does NOT derive the root as dirname of the metadata worktree" \
  '! grep -qE "dirname .*METADATA_WORKTREE" "$FIN"'
# The gate's suite run legitimately stays in the feature worktree — only the close-out moves.
assert "0075: finalize still runs the merge-gate suite in the feature worktree" \
  'grep -qiE "suite (run )?(still )?(runs |happens )?in the feature worktree|in the feature worktree" "$FIN"'
```

Verify each pattern is anchored to a phrase the target clause **owns**, and that `grep -c` on it is exactly 1 after step 3 — if a pattern matches two places, tighten it (LEARNINGS: one assert, one clause).

- [ ] **Step 2: Run it to verify it fails**

Run: `bash tests/test_finalize_gate.sh 2>&1 | grep 0075`
Expected: the `REPO_ROOT` and `durable root` asserts fail — the skill says nothing about either yet.

- [ ] **Step 3: Write the implementation**

In `skills/docket-finalize-change/SKILL.md`, add a subsection to the skill's procedure (immediately after its Step-0 preamble pointer, so it governs every later step), and reference it from the merge/cleanup steps:

```markdown
### The durable root (change 0075)

Every step of this skill AFTER the merge gate's suite run — the merge, the metadata writes, the
archive, `terminal-publish`, `cleanup-feature-branch.sh`, and the Board pass — runs from the
**durable root**: the `REPO_ROOT` literal printed in the Step-0 `docket.sh preflight` block. Prefix
those Bash calls with `cd <REPO_ROOT>` (or target them with `git -C <REPO_ROOT>`).

This is not hygiene, it is a correctness requirement: cleanup removes `.worktrees/<slug>`, and
`git worktree remove --force` **succeeds** while the agent's own CWD is inside that directory — the
process merely orphans its CWD, and the agent's **next** Bash call then cannot start (`cd: no such
file or directory`), stranding the run after the destructive step has already landed. A child
process cannot change its parent's CWD, so no script can fix this; only the skill can. The
script-side guard is the backstop: `cleanup-feature-branch.sh` now REFUSES (before any destructive
step) when the caller's CWD is at or inside the target.

Read the root from the `REPO_ROOT=` line of the `preflight` block — never derive it as `dirname` of
`METADATA_WORKTREE` (in `main`-mode `METADATA_WORKTREE` *is* the repo root, so `dirname` yields the
repo's **parent**), and never from `git rev-parse --show-toplevel` (from a linked worktree that
returns the linked worktree).

**The merge gate's suite run is the exception** — it happens **in the feature worktree**, which is
where it belongs. Only the close-out steps move to the durable root.
```

Then, in the skill's cleanup/close-out step, add the one-line pointer: "run from the durable root (see *The durable root*)".

- [ ] **Step 4: Run the tests to verify they pass**

Run: `bash tests/test_finalize_gate.sh`
Expected: all `ok - …`.

- [ ] **Step 5: Update `scripts/docket.md`**

Add `REPO_ROOT` to the documented `preflight`/`env` output keys: "`REPO_ROOT` — absolute path of the repo's **main worktree**. The cwd-independent literal skills `cd` to before any step that can remove the worktree they are standing in. Emitted in the `plain` format only (which is what `preflight`/`env` print)."

- [ ] **Step 6: Mutation-test**

Delete the `REPO_ROOT` mention from the skill; confirm the assert reddens. Restore. Confirm `grep -c "REPO_ROOT" skills/docket-finalize-change/SKILL.md` is ≥1 and that removing *only* the durable-root subsection reddens the `durable root` assert.

- [ ] **Step 7: Run the whole suite, then commit**

```bash
for f in tests/test_*.sh; do bash "$f" >/dev/null 2>&1 || echo "FAILED: $f"; done
git add skills/docket-finalize-change/SKILL.md scripts/docket.md tests/test_finalize_gate.sh
git commit -m "feat(0075): finalize runs its close-out from the durable REPO_ROOT (D3)"
```

---

### Task 7: Whole-suite verification + live smoke

**Files:** none (verification only)

- [ ] **Step 1: Run the whole suite, foreground, once**

```bash
cd /Users/homer/dev/docket/.worktrees/cwd-independent-repo-root-anchor
for f in tests/test_*.sh; do echo "== $f"; bash "$f" 2>&1 | grep "^NOT OK" ; done
```

Expected: no `NOT OK` lines beyond the Task 0 baseline. Any new one is a regression — fix it, never loosen the assert.

- [ ] **Step 2: Smoke the real thing (the fixture-blindness antidote)**

Against the REAL repo (not a fixture), prove the anchor from all three CWD classes:

```bash
cd /Users/homer/dev/docket && bash scripts/docket.sh env | grep -E '^(REPO_ROOT|METADATA_WORKTREE)='
cd /Users/homer/dev/docket/.docket && bash /Users/homer/dev/docket/scripts/docket.sh env | grep -E '^(REPO_ROOT|METADATA_WORKTREE)='
cd /Users/homer/dev/docket/.worktrees/cwd-independent-repo-root-anchor && bash /Users/homer/dev/docket/scripts/docket.sh env | grep -E '^(REPO_ROOT|METADATA_WORKTREE)='
```

Expected: **byte-identical** `REPO_ROOT=/Users/homer/dev/docket` and `METADATA_WORKTREE=/Users/homer/dev/docket/.docket` from all three. Then confirm no debris:

```bash
git -C /Users/homer/dev/docket worktree list
```

Expected: exactly the main worktree, `.docket`, and the feature worktree — no `.docket/.docket`.

- [ ] **Step 3: Commit any fixes**

```bash
git add -A && git commit -m "test(0075): whole-suite green + live smoke across all three CWD classes"
```

(Skip if nothing changed.)

---

## Self-review notes (already applied)

- **Spec §1** → Task 2 (resolver anchor, `--repo-dir` preserved, not-a-repo fallback) + its subdirectory behavior-change pin.
- **Spec §2** → Task 2 (`REPO_ROOT`, plain only, with the both-directions assert).
- **Spec §3** → Task 3 (absolute path, `-C`, nested-worktree refusal, D2 regression).
- **Spec §4** → Task 4 (absolute target, `caller_pwd` before any `cd`, refusal before **both** destructive steps, provenance guard untouched, D1 regression across all three CWD classes).
- **Spec §5** → Task 5 (the dead block comes alive and **commits**; its failure path no longer abandons `terminal-publish`/`cleanup`; `render-change-links.sh` audited).
- **Spec §6** → Task 6 (finalize's durable-root posture; suite run stays in the feature worktree).
- **Spec test plan** → Tasks 1–6 each carry their regression test; Task 7 runs the whole suite plus a live smoke.
- **Spec's "existing test-surface delta"** → `test_docket_config.sh:43,66` (shell values stay relative — unchanged, verified); `test_docket_preflight.sh:52` (updated in Task 3, as the spec predicts); `test_docket_status.sh` mw fixtures (unchanged relative inputs, now anchored at runtime, and Task 5 adds the absolute-path coverage the spec says is missing).
- **Deviation from the spec, recorded deliberately:** the spec calls `test_render_board.sh:989` "a source-text sentinel pinning the literal `mw=` line" and says to leave it untouched. Reading the code, it is not a scan of `docket-status.sh` — it is a *self-contained control fixture* built with `printf`, whose comment claims to be a VERBATIM copy of `backlog_pass`. Leaving it stale would not redden anything but would make that claim false. Task 5(d) therefore updates the two copied lines so the fixture stays verbatim, and re-runs `test_render_board.sh` to confirm the guard's false-positive control still passes. Record this in the results file.
