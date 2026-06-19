# Close-out Scripts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract docket's shared terminal-transition close-out mechanics (archive → terminal-publish → branch/worktree cleanup) into three deterministic, fail-closed, hermetically-tested shell scripts, and rewire the four call sites to invoke them.

**Architecture:** Three small scripts under `scripts/`, each sourcing the existing `scripts/lib/docket-frontmatter.sh` (no new parser). `done` and `killed` are unified through one archive primitive. The scripts own the deterministic git plumbing *and* the CAS-retry loops; the model authors each commit message (passed as `--message`, each script ships a default). Each script is **fail-closed**: it self-verifies its postconditions and exits non-zero with a diagnostic on any deviation, so the rewired skills trust the exit code (proceed on 0, abort-and-report on non-zero) instead of re-confirming every mechanical step. Tests are hermetic: a temp repo with a local *bare* origin carrying `docket` + `main` branches — no `gh`, no network.

**Tech Stack:** Bash (`set -uo pipefail` + explicit `die()`, the house style — **not** `set -e`, which trips on `git diff --cached --quiet`), POSIX-portable `sed`/`git`, the `field`/`list_field`/`has_section` accessors from `scripts/lib/docket-frontmatter.sh`. Test harness mirrors `tests/test_render_board.sh` / `tests/test_github_mirror.sh` (`assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }`, `mktemp -d` + `trap`, exit `$fail`).

## Global Constraints

- **No behavior change.** This is a faithful extraction (like 0011 `github-mirror.sh`, 0022 `render-board.sh`). The archive-filename contract (`archive/<DATE>-<id>-<slug>.md`, UTC, id zero-padded to 4 digits), the terminal-publish copy-set rules, the **Accepted**-ADR gate, and the racing-sweep idempotency guarantees are reproduced **exactly**, not redesigned. The single source of the procedures being extracted is `skills/docket-finalize-change/SKILL.md` (the *Per-change steps* archive, the *Clean up* step, and the *Terminal publish (docket-mode)* section) — port it verbatim, do not invent.
- **Source the existing lib; no new parser.** `. "$(dirname "$0")/lib/docket-frontmatter.sh"`. Use `field`, `list_field`, `has_section`.
- **`GIT="${GIT:-git}"` seam** in every script (thin seam; the primary test mechanism is a real hermetic bare origin, not a git mock). Close-out makes **no** `gh` calls — no `GH` seam needed.
- **`set -uo pipefail`, never `set -e`.** Fail-closed via explicit checks + a `die(){ printf '%s\n' "<script>: $*" >&2; exit 1; }` helper. Never `producer | grep -q` or `producer | head` (SIGPIPE → 141 under pipefail, LEARNINGS #11/#16): capture into a variable first, then grep/head the variable.
- **Change-file-only archive commit, tree-identical across concurrent archivers** — the archive commit stages and commits **only** the change file path(s). `BOARD.md` regen is a separate caller step; never bundled.
- **Normal CAS push, never force-push.** Only the feature branch is ever force-pushed, and that lives in the merge gate (out of scope here). The integration-branch publish is a normal push with a `pull --rebase`-and-retry loop.
- **Distribution:** scripts are invoked by repo-relative path (`scripts/<name>.sh`), exactly like the existing two. No `install.sh` / `link-skills.sh` registration is needed or wanted (LEARNINGS #12: confirm auto-discovery before editing plumbing — confirmed, path-based).
- **Test rigor (LEARNINGS #2/#5/#15/#21):** every wiring/sentinel assert anchors to exactly **one** unique phrase it owns; mutation-test each by deleting the real clause (must flip to `NOT OK`). Golden/fixture frontmatter must use **real-shaped values** (full-URL `pr:`, ≥2 ADRs including one `Accepted` + one `Proposed`) and **plurality** (≥2 of every list it renders).
- **Commit messages:** `<type>(0025): <desc>` (e.g. `feat(0025): …`, `test(0025): …`, `docs(0025): …`). Commit only the files each step names.

---

## File Structure

**Create:**
- `scripts/archive-change.sh` — the archive primitive (dated `git mv` + frontmatter + change-file-only commit + push), `done`/`killed` unified.
- `scripts/terminal-publish.sh` — the docket-mode copy of terminal records onto the integration branch (CAS push + teardown), with the Accepted-ADR gate at the copy site and the main-mode no-op guard.
- `scripts/cleanup-feature-branch.sh` — provenance-guarded worktree + branch removal.
- `tests/test_closeout.sh` — hermetic bare-origin tests for all three scripts + the call-site wiring sentinels.

**Modify (rewire call sites; all on the integration branch — SKILL.md files are code, not metadata):**
- `skills/docket-finalize-change/SKILL.md` — the single source: archive step 3, clean-up step 4, and the *Terminal publish (docket-mode)* procedure shrink to "author a message → call the script, trust the exit code."
- `skills/docket-status/SKILL.md` — the merge-sweep archive loop invokes the scripts.
- `skills/docket-new-change/SKILL.md` — the proposed-kill clause invokes the scripts.
- `skills/docket-implement-next/SKILL.md` — the reconcile-kill clause invokes the scripts.

**Out of scope (do NOT touch):** the merge-gate spine (rebase → suite → force-push), `docket-config.sh`, the harvest, the health checks, the `github` surface. The optional ADR-0002 `## Update` is a **metadata** edit (ADR lives on the `docket` branch) handled outside this feature branch — see the closing note; do not edit any `docs/adrs/` file here.

---

## Task 1: `scripts/archive-change.sh` — the archive primitive

**Files:**
- Create: `scripts/archive-change.sh`
- Test: `tests/test_closeout.sh` (new file; this task creates it and the archive section)

**Interfaces:**
- Consumes: `scripts/lib/docket-frontmatter.sh` (`field`).
- Produces (CLI contract relied on by Task 4/5 rewires):
  ```
  archive-change.sh --changes-dir DIR --id N --outcome done|killed --date YYYY-MM-DD
                    [--message MSG] [--results PATH] [--reason TEXT] [--remote R]
  ```
  - `--changes-dir DIR` — path to the metadata tree's `docs/changes` (e.g. `.docket/docs/changes`). The script derives the worktree root (`git -C DIR rev-parse --show-toplevel`) and the changes-dir path relative to it.
  - `--id N` — bare integer id; the file is `active/<zero-padded-4>-<slug>.md`.
  - `--outcome done|killed`, `--date YYYY-MM-DD` (caller-computed UTC; merge date for `done`, kill-commit date for `killed`).
  - `--results PATH` — (`done`) value written into the `results:` field, if given.
  - `--reason TEXT` — (`killed`) body text for an appended `## Why killed` section.
  - `--message MSG` — commit message; default `docket(<id>): <outcome> — archived (status <outcome>, <date>)`.
  - `--remote R` — default `origin`.
  - **Behavior:** (1) reuse-existing probe — if `archive/*-<pad>-*.md` already exists, the change is already archived → no-op exit 0 (idempotent across re-runs and day boundaries); (2) else `active/<pad>-<slug>.md` must exist → `mkdir -p archive`; `git mv` to `archive/<DATE>-<pad>-<slug>.md`; (3) set frontmatter `status:`/`updated:`, plus `results:` (`done`, if `--results`) or append `## Why killed` (`killed`, from `--reason`); (4) commit **change-file-only** (`--message` or default); push the current branch with `pull --rebase` retry on non-fast-forward; (5) self-verify postconditions, `die` non-zero on any deviation.
  - Exit 0 ⇒ archived (or already-archived no-op); non-zero ⇒ a diagnostic on stderr and nothing half-done the caller must guess about.

- [ ] **Step 1: Scaffold the script skeleton (arg parsing, lib source, helpers)**

Create `scripts/archive-change.sh`:

```bash
#!/usr/bin/env bash
# scripts/archive-change.sh — the shared terminal-transition archive primitive (change 0025).
# Moves a change from active/ to a dated archive/ name on the metadata branch, sets its terminal
# frontmatter, commits CHANGE-FILE-ONLY, and pushes with a rebase-retry loop. `done` and `killed`
# are unified here; finalize, the docket-status sweep, and the two kill paths all invoke this.
# Fail-closed: self-verifies its postconditions and exits non-zero with a diagnostic on deviation.
# Idempotent: a reuse-existing-archive probe makes a racing/resumed run a safe no-op.
#
# Usage:
#   archive-change.sh --changes-dir DIR --id N --outcome done|killed --date YYYY-MM-DD
#                     [--message MSG] [--results PATH] [--reason TEXT] [--remote R]
#
# Mock seam: GIT="${GIT:-git}".
set -uo pipefail

. "$(dirname "$0")/lib/docket-frontmatter.sh"

GIT="${GIT:-git}"
CHANGES_DIR="" ID="" OUTCOME="" DATE="" MESSAGE="" RESULTS="" REASON="" REMOTE="origin"

die(){ printf '%s\n' "archive-change: $*" >&2; exit 1; }
log(){ printf '%s\n' "archive-change: $*" >&2; }

while [ $# -gt 0 ]; do
  case "$1" in
    --changes-dir) CHANGES_DIR="$2"; shift ;;
    --id) ID="$2"; shift ;;
    --outcome) OUTCOME="$2"; shift ;;
    --date) DATE="$2"; shift ;;
    --message) MESSAGE="$2"; shift ;;
    --results) RESULTS="$2"; shift ;;
    --reason) REASON="$2"; shift ;;
    --remote) REMOTE="$2"; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
  shift
done

[ -n "$CHANGES_DIR" ] || die "missing --changes-dir"
[ -n "$ID" ]          || die "missing --id"
case "$OUTCOME" in done|killed) ;; *) die "missing/invalid --outcome (done|killed)" ;; esac
[ -n "$DATE" ]        || die "missing --date"
[ -d "$CHANGES_DIR" ] || die "changes dir not found: $CHANGES_DIR"

pad="$(printf '%04d' "$ID")"
WT="$($GIT -C "$CHANGES_DIR" rev-parse --show-toplevel)" || die "not a git worktree: $CHANGES_DIR"
# changes-dir path relative to the worktree root (git mv/commit want worktree-relative paths)
REL="$(cd "$CHANGES_DIR" && pwd)"; REL="${REL#"$WT"/}"
```

- [ ] **Step 2: Write the failing test — `done` archive happy path**

Create `tests/test_closeout.sh` with the harness + a hermetic-fixture helper + the first archive test. The fixture builds a temp repo with a bare origin carrying `docket` + `main`:

```bash
#!/usr/bin/env bash
# tests/test_closeout.sh — verifies change 0025: the deterministic close-out scripts
# (archive-change.sh, terminal-publish.sh, cleanup-feature-branch.sh) and the call-site wiring.
# Hermetic: a temp repo with a local *bare* origin carrying docket + main; no gh, no network.
# Run: bash tests/test_closeout.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ARCHIVE="$REPO/scripts/archive-change.sh"
PUBLISH="$REPO/scripts/terminal-publish.sh"
CLEANUP="$REPO/scripts/cleanup-feature-branch.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

git_quiet(){ git "$@" >/dev/null 2>&1; }

# new_repo: prints "<work> <origin>" — a fresh clone with a bare origin holding docket + main.
# docket branch: docs/changes/active/0007-sample.md, its spec, one Accepted + one Proposed ADR.
# main branch: a trivial baseline (the integration branch publish target).
new_repo(){
  local root work origin
  root="$(mktemp -d)"; origin="$root/origin.git"; work="$root/work"
  git_quiet init --bare "$origin"
  git_quiet clone "$origin" "$work"
  git -C "$work" config user.email t@t; git -C "$work" config user.name t
  # --- main branch (baseline) ---
  git -C "$work" checkout -b main
  echo "code" > "$work/README.md"; git -C "$work" add README.md
  git_quiet -C "$work" commit -m "main baseline"
  git_quiet -C "$work" push -u origin main
  # --- docket branch (orphan metadata) ---
  git -C "$work" checkout --orphan docket
  git -C "$work" rm -rf . >/dev/null 2>&1 || true
  mkdir -p "$work/docs/changes/active" "$work/docs/changes/archive" \
           "$work/docs/superpowers/specs" "$work/docs/adrs"
  cat > "$work/docs/changes/active/0007-sample.md" <<'EOF'
---
id: 7
slug: sample
title: Sample change
status: implemented
priority: medium
created: 2026-06-01
updated: 2026-06-01
spec: docs/superpowers/specs/2026-06-01-sample.md
adrs: [3, 5]
pr: https://github.com/o/r/pull/42
results:
---

## Why
Body.
EOF
  echo "# spec" > "$work/docs/superpowers/specs/2026-06-01-sample.md"
  cat > "$work/docs/adrs/0003-accepted.md" <<'EOF'
---
id: 3
slug: accepted
title: An accepted decision
status: Accepted
date: 2026-06-01
---
## Decision
Yes.
EOF
  cat > "$work/docs/adrs/0005-proposed.md" <<'EOF'
---
id: 5
slug: proposed
title: A proposed decision
status: Proposed
date: 2026-06-01
---
## Decision
Maybe.
EOF
  git -C "$work" add -A; git_quiet -C "$work" commit -m "docket metadata"
  git_quiet -C "$work" push -u origin docket
  # leave the work clone parked on docket (the metadata working tree)
  printf '%s %s\n' "$work" "$origin"
}

assert "archive-change.sh exists and is executable" '[ -x "$ARCHIVE" ]'

# --- archive-change.sh: done happy path ---
read -r W _ < <(new_repo)
"$ARCHIVE" --changes-dir "$W/docs/changes" --id 7 --outcome done --date 2026-06-18 \
  --results docs/results/2026-06-18-sample-results.md >/dev/null 2>&1
assert "done: active file is gone" '[ ! -e "$W/docs/changes/active/0007-sample.md" ]'
assert "done: dated archive file exists" '[ -e "$W/docs/changes/archive/2026-06-18-0007-sample.md" ]'
assert "done: status set to done" '[ "$(. "$REPO/scripts/lib/docket-frontmatter.sh"; field "$W/docs/changes/archive/2026-06-18-0007-sample.md" status)" = done ]'
assert "done: updated set to date" '[ "$(. "$REPO/scripts/lib/docket-frontmatter.sh"; field "$W/docs/changes/archive/2026-06-18-0007-sample.md" updated)" = 2026-06-18 ]'
assert "done: results link written" '[ "$(. "$REPO/scripts/lib/docket-frontmatter.sh"; field "$W/docs/changes/archive/2026-06-18-0007-sample.md" results)" = docs/results/2026-06-18-sample-results.md ]'
assert "done: pushed (origin docket == local)" '[ "$(git -C "$W" rev-parse @)" = "$(git -C "$W" rev-parse origin/docket)" ]'
assert "done: commit touched ONLY the change file" '[ "$(git -C "$W" show --name-only --format= HEAD | grep -c .)" -eq 2 ] && git -C "$W" show --name-only --format= HEAD | grep -q "0007-sample.md"'
```

(The "ONLY the change file" assert expects exactly two pathnames in the commit — the `active/` deletion and the `archive/` addition of the same change file — and nothing else.)

- [ ] **Step 3: Run the test — verify it fails**

Run: `bash tests/test_closeout.sh`
Expected: the `done:` asserts print `NOT OK` (script body is a no-op skeleton; the file is never moved).

- [ ] **Step 4: Implement the archive logic**

Append to `scripts/archive-change.sh` (after the scaffold):

```bash
# cas_push BRANCH: push current HEAD to REMOTE/BRANCH, rebasing on non-fast-forward.
cas_push(){
  local br="$1"
  until $GIT -C "$WT" push "$REMOTE" "$br"; do
    $GIT -C "$WT" pull --rebase "$REMOTE" "$br" || die "rebase during push failed for $br"
  done
}

# set_field FILE KEY VALUE — replace a top-level frontmatter scalar in place (portable sed).
set_field(){
  local f="$1" k="$2" v="$3" t; t="$(mktemp)"
  sed -E "s|^($k:)[[:space:]]*.*|\1 $v|" "$f" > "$t" && mv "$t" "$f"
}

branch="$($GIT -C "$WT" rev-parse --abbrev-ref HEAD)"
src="active/$pad-"          # slug unknown until matched
shopt -s nullglob
active_matches=("$WT/$REL/active/$pad-"*.md)
archive_matches=("$WT/$REL/archive/"*"-$pad-"*.md)
shopt -u nullglob

# (1) reuse-existing-archive probe — already archived ⇒ idempotent no-op.
if [ "${#archive_matches[@]}" -gt 0 ]; then
  log "already archived (${archive_matches[0]##*/}); no-op"
  exit 0
fi
[ "${#active_matches[@]}" -eq 1 ] || die "expected exactly one active/$pad-*.md, found ${#active_matches[@]}"

active_file="${active_matches[0]}"
base="$(basename "$active_file")"           # <pad>-<slug>.md
slug="${base#"$pad-"}"; slug="${slug%.md}"
dest_rel="$REL/archive/$DATE-$pad-$slug.md"
src_rel="$REL/active/$base"

# (2) dated move
mkdir -p "$WT/$REL/archive"
$GIT -C "$WT" mv "$src_rel" "$dest_rel" || die "git mv failed"

# (3) frontmatter
dest="$WT/$dest_rel"
set_field "$dest" status "$OUTCOME"
set_field "$dest" updated "$DATE"
if [ "$OUTCOME" = done ] && [ -n "$RESULTS" ]; then
  set_field "$dest" results "$RESULTS"
fi
if [ "$OUTCOME" = killed ]; then
  { printf '\n## Why killed\n\n'; printf '%s\n' "${REASON:-Killed.}"; } >> "$dest"
fi

# (4) commit CHANGE-FILE-ONLY + push
[ -n "$MESSAGE" ] || MESSAGE="docket($pad): $OUTCOME — archived (status $OUTCOME, $DATE)"
$GIT -C "$WT" add "$dest_rel"
$GIT -C "$WT" commit -m "$MESSAGE" -- "$src_rel" "$dest_rel" >/dev/null || die "commit failed"
cas_push "$branch"

# (5) fail-closed self-verification
[ ! -e "$WT/$src_rel" ]                                   || die "postcondition: active file still present"
[ -e "$dest" ]                                            || die "postcondition: archive file missing"
[ "$(field "$dest" status)"  = "$OUTCOME" ]              || die "postcondition: status not $OUTCOME"
[ "$(field "$dest" updated)" = "$DATE" ]                || die "postcondition: updated not $DATE"
[ "$($GIT -C "$WT" rev-parse @)" = "$($GIT -C "$WT" rev-parse "$REMOTE/$branch")" ] \
  || die "postcondition: push did not land on $REMOTE/$branch"
log "archived $base -> $DATE-$pad-$slug.md ($OUTCOME)"
```

`chmod +x scripts/archive-change.sh`.

- [ ] **Step 5: Run the test — verify the `done` path passes**

Run: `bash tests/test_closeout.sh`
Expected: all `done:` asserts print `ok`.

- [ ] **Step 6: Add the `killed` + idempotency + fail-closed tests**

Append to `tests/test_closeout.sh`:

```bash
# --- archive-change.sh: killed path + ## Why killed ---
read -r W _ < <(new_repo)
"$ARCHIVE" --changes-dir "$W/docs/changes" --id 7 --outcome killed --date 2026-06-18 \
  --reason "Obsoleted by 0009." >/dev/null 2>&1
assert "killed: dated archive file exists" '[ -e "$W/docs/changes/archive/2026-06-18-0007-sample.md" ]'
assert "killed: status set to killed" '[ "$(. "$REPO/scripts/lib/docket-frontmatter.sh"; field "$W/docs/changes/archive/2026-06-18-0007-sample.md" status)" = killed ]'
assert "killed: ## Why killed section appended with reason" 'grep -qF "## Why killed" "$W/docs/changes/archive/2026-06-18-0007-sample.md" && grep -qF "Obsoleted by 0009." "$W/docs/changes/archive/2026-06-18-0007-sample.md"'

# --- archive-change.sh: idempotent reuse-existing (second run is a no-op, no new file across a day boundary) ---
read -r W _ < <(new_repo)
"$ARCHIVE" --changes-dir "$W/docs/changes" --id 7 --outcome done --date 2026-06-18 >/dev/null 2>&1
head1="$(git -C "$W" rev-parse @)"
"$ARCHIVE" --changes-dir "$W/docs/changes" --id 7 --outcome done --date 2026-06-19 >/dev/null 2>&1  # different date!
rc=$?
assert "idempotent: second run exits 0 (no-op)" "[ $rc -eq 0 ]"
assert "idempotent: no second archive file minted across day boundary" '[ "$(ls "$W/docs/changes/archive/" | grep -c -- "-0007-sample.md")" -eq 1 ]'
assert "idempotent: reuses 2026-06-18 name (ignores later --date)" '[ -e "$W/docs/changes/archive/2026-06-18-0007-sample.md" ]'
assert "idempotent: no new commit on the no-op run" '[ "$(git -C "$W" rev-parse @)" = "$head1" ]'

# --- archive-change.sh: fail-closed when the id is absent (neither active nor archive) ---
read -r W _ < <(new_repo)
"$ARCHIVE" --changes-dir "$W/docs/changes" --id 99 --outcome done --date 2026-06-18 >/dev/null 2>&1
assert "fail-closed: missing id exits non-zero" '[ $? -ne 0 ]'
```

- [ ] **Step 7: Run the full archive section — verify it passes**

Run: `bash tests/test_closeout.sh`
Expected: every `ok - ` for archive (`done`, `killed`, idempotent, fail-closed). No `NOT OK`. **Mutation-check each new assert**: temporarily break the script (e.g. comment out `set_field "$dest" status`) and confirm the matching assert flips to `NOT OK`, then restore.

- [ ] **Step 8: Commit**

```bash
git add scripts/archive-change.sh tests/test_closeout.sh
git commit -m "feat(0025): archive-change.sh — shared dated-archive primitive (done|killed), fail-closed + idempotent"
```

---

## Task 2: `scripts/terminal-publish.sh` — copy terminal records onto the integration branch

**Files:**
- Create: `scripts/terminal-publish.sh`
- Test: `tests/test_closeout.sh` (append the terminal-publish section)

**Interfaces:**
- Consumes: `scripts/lib/docket-frontmatter.sh` (`field`, `list_field`); the archived change file on `origin/<metadata-branch>` (produced by Task 1 — ordering is load-bearing: the archived path must exist on the remote before publish copies it).
- Produces (CLI contract relied on by Task 4/5 rewires):
  ```
  terminal-publish.sh --id N --outcome done|killed --integration-branch B --metadata-branch M
                      --changes-dir REL --adrs-dir REL [--message MSG] [--remote R]
  ```
  - `--changes-dir REL` / `--adrs-dir REL` — repo-relative (e.g. `docs/changes`, `docs/adrs`), as the paths exist on the metadata branch.
  - **Mode guard:** if `M == B` (main-mode) → log + no-op exit 0 (there is no `docket` branch to copy from).
  - **Copy-set (built as a list):** the archived change file (always); the `spec:` path **iff** non-empty; each `adrs:` entry **whose ADR file `status:` is `Accepted`** (the Accepted gate fires here; `Proposed`/draft ADRs are skipped).
  - Provisions a transient `pub-<id>` worktree in a temp dir on `origin/<integration>`; `checkout origin/<metadata-branch> -- <copyset>`; guarded commit (`diff --cached --quiet ||`); **CAS push** `HEAD:<integration>` with re-copy-on-conflict retry; teardown.
  - Fail-closed: after the push, re-fetch and assert every copy-set path is present on `origin/<integration>`, and `pub-<id>` is torn down; `die` non-zero otherwise.
  - `--message` default `docket(<id>): publish terminal record (<outcome>)`. `--remote` default `origin`.

- [ ] **Step 1: Scaffold (args, mode guard, helpers)**

Create `scripts/terminal-publish.sh`:

```bash
#!/usr/bin/env bash
# scripts/terminal-publish.sh — the shared "Terminal publish (docket-mode)" procedure (change 0025).
# Copies a change's terminal records (archived change file + its spec + its Accepted ADRs) from
# origin/<metadata-branch> onto the integration branch, via a transient worktree, with a CAS push.
# docket-mode only: a no-op in main-mode (metadata-branch == integration-branch). Fail-closed:
# re-fetches and asserts the full copy-set landed before exiting 0. Idempotent and re-run safe.
#
# Usage:
#   terminal-publish.sh --id N --outcome done|killed --integration-branch B --metadata-branch M
#                       --changes-dir REL --adrs-dir REL [--message MSG] [--remote R]
#
# Mock seam: GIT="${GIT:-git}".
set -uo pipefail

. "$(dirname "$0")/lib/docket-frontmatter.sh"

GIT="${GIT:-git}"
ID="" OUTCOME="" INT_BRANCH="" META_BRANCH="" CHANGES_DIR="" ADRS_DIR="" MESSAGE="" REMOTE="origin"

die(){ printf '%s\n' "terminal-publish: $*" >&2; exit 1; }
log(){ printf '%s\n' "terminal-publish: $*" >&2; }

while [ $# -gt 0 ]; do
  case "$1" in
    --id) ID="$2"; shift ;;
    --outcome) OUTCOME="$2"; shift ;;
    --integration-branch) INT_BRANCH="$2"; shift ;;
    --metadata-branch) META_BRANCH="$2"; shift ;;
    --changes-dir) CHANGES_DIR="$2"; shift ;;
    --adrs-dir) ADRS_DIR="$2"; shift ;;
    --message) MESSAGE="$2"; shift ;;
    --remote) REMOTE="$2"; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
  shift
done

[ -n "$ID" ] || die "missing --id"
case "$OUTCOME" in done|killed) ;; *) die "missing/invalid --outcome" ;; esac
[ -n "$INT_BRANCH" ] && [ -n "$META_BRANCH" ] || die "missing --integration-branch/--metadata-branch"
[ -n "$CHANGES_DIR" ] && [ -n "$ADRS_DIR" ]   || die "missing --changes-dir/--adrs-dir"

# Mode guard: main-mode has no docket branch to copy from.
if [ "$META_BRANCH" = "$INT_BRANCH" ]; then
  log "main-mode (metadata-branch == integration-branch); no-op"
  exit 0
fi

pad="$(printf '%04d' "$ID")"
[ -n "$MESSAGE" ] || MESSAGE="docket($pad): publish terminal record ($OUTCOME)"
```

- [ ] **Step 2: Write the failing test — copy-set incl. the Accepted gate**

Append to `tests/test_closeout.sh` (the fixture's change has `spec:` set, `adrs: [3, 5]` with 3 Accepted and 5 Proposed):

```bash
# --- terminal-publish.sh: copy-set built correctly with the Accepted-ADR gate ---
read -r W _ < <(new_repo)
# precondition: archive the change on docket first (publish copies the archived path)
"$ARCHIVE" --changes-dir "$W/docs/changes" --id 7 --outcome done --date 2026-06-18 >/dev/null 2>&1
( cd "$W" && "$PUBLISH" --id 7 --outcome done --integration-branch main --metadata-branch docket \
    --changes-dir docs/changes --adrs-dir docs/adrs ) >/dev/null 2>&1
# inspect what landed on origin/main
git -C "$W" fetch origin main >/dev/null 2>&1
ls_main(){ git -C "$W" ls-tree -r --name-only origin/main; }
assert "publish: archived change file on integration branch" 'ls_main | grep -q "docs/changes/archive/2026-06-18-0007-sample.md"'
assert "publish: spec on integration branch" 'ls_main | grep -q "docs/superpowers/specs/2026-06-01-sample.md"'
assert "publish: Accepted ADR-0003 on integration branch" 'ls_main | grep -q "docs/adrs/0003-accepted.md"'
assert "publish: Proposed ADR-0005 SKIPPED (gate)" '! ls_main | grep -q "docs/adrs/0005-proposed.md"'
assert "publish: pub-7 worktree torn down" '! git -C "$W" worktree list | grep -q "pub-7"'
assert "publish: BOARD.md never published" '! ls_main | grep -q "docs/changes/BOARD.md"'
```

- [ ] **Step 3: Run — verify it fails**

Run: `bash tests/test_closeout.sh`
Expected: the `publish:` asserts print `NOT OK` (script is a guarded skeleton; nothing copied).

- [ ] **Step 4: Implement copy-set construction (with the Accepted gate)**

Append to `scripts/terminal-publish.sh`:

```bash
# --- build the copy-set from origin/<metadata-branch> (authoritative remote bytes) ---
$GIT fetch "$REMOTE" "$META_BRANCH" >/dev/null 2>&1 || die "fetch $REMOTE/$META_BRANCH failed"
metaref="$REMOTE/$META_BRANCH"

# locate the archived change file path on the metadata branch by id
tree="$($GIT ls-tree -r --name-only "$metaref" -- "$CHANGES_DIR/archive")"
change_path="$(printf '%s\n' "$tree" | grep -E "/[0-9]{4}-[0-9]{2}-[0-9]{2}-$pad-[^/]*\.md$" | head -n1)"
[ -n "$change_path" ] || die "no archived change file for id $ID on $metaref"

# read its frontmatter via a temp dump (field operates on files)
tmpd="$(mktemp -d)"
$GIT show "$metaref:$change_path" > "$tmpd/change.md" || die "cannot read $change_path"
spec_path="$(field "$tmpd/change.md" spec)"
adr_ids="$(list_field "$tmpd/change.md" adrs)"

copyset=("$change_path")
[ -n "$spec_path" ] && copyset+=("$spec_path")

# Accepted gate: include an ADR only if its status: is Accepted on the metadata branch
adr_tree="$($GIT ls-tree -r --name-only "$metaref" -- "$ADRS_DIR")"
for aid in $adr_ids; do
  apad="$(printf '%04d' "$aid")"
  apath="$(printf '%s\n' "$adr_tree" | grep -E "/$apad-[^/]*\.md$" | head -n1)"
  [ -n "$apath" ] || { log "adr $aid: file not found on $metaref; skipping"; continue; }
  $GIT show "$metaref:$apath" > "$tmpd/adr.md" || { log "adr $aid: unreadable; skipping"; continue; }
  if [ "$(field "$tmpd/adr.md" status)" = "Accepted" ]; then
    copyset+=("$apath")
  else
    log "adr $aid: not Accepted; skipped by gate"
  fi
done
```

- [ ] **Step 5: Implement the worktree provision + CAS publish + teardown + self-verify**

Append to `scripts/terminal-publish.sh`:

```bash
# --- provision a transient integration checkout on a throwaway branch ---
pub="$(mktemp -d)/pub"
$GIT worktree prune
$GIT worktree add -B "pub-$ID" "$pub" "$REMOTE/$INT_BRANCH" >/dev/null 2>&1 \
  || die "could not provision pub-$ID worktree"

teardown(){
  $GIT -C "$pub" checkout --detach >/dev/null 2>&1
  $GIT worktree remove --force "$pub" >/dev/null 2>&1
  $GIT branch -D "pub-$ID" >/dev/null 2>&1 || true
  rm -rf "$(dirname "$pub")" "$tmpd"
}

# --- copy the terminal records from the metadata remote tip and CAS-push ---
$GIT -C "$pub" fetch "$REMOTE" "$META_BRANCH" >/dev/null 2>&1 || { teardown; die "fetch in pub failed"; }
$GIT -C "$pub" checkout "$metaref" -- "${copyset[@]}" || { teardown; die "checkout copyset failed"; }
if ! $GIT -C "$pub" diff --cached --quiet; then
  $GIT -C "$pub" commit -m "$MESSAGE" >/dev/null || { teardown; die "publish commit failed"; }
fi
# push HEAD explicitly (a bare push resolves the stale local <integration> ref); CAS retry loop
until $GIT -C "$pub" push "$REMOTE" "HEAD:$INT_BRANCH"; do
  if $GIT -C "$pub" pull --rebase "$REMOTE" "$INT_BRANCH"; then :; else
    $GIT -C "$pub" checkout "$metaref" -- "${copyset[@]}"
    $GIT -C "$pub" rebase --continue || { teardown; die "CAS rebase --continue failed"; }
  fi
done

# --- fail-closed: re-fetch and assert the full copy-set landed on origin/<integration> ---
$GIT fetch "$REMOTE" "$INT_BRANCH" >/dev/null 2>&1 || { teardown; die "post-push fetch failed"; }
landed="$($GIT ls-tree -r --name-only "$REMOTE/$INT_BRANCH")"
for p in "${copyset[@]}"; do
  printf '%s\n' "$landed" | grep -qxF "$p" || { teardown; die "postcondition: $p missing on $REMOTE/$INT_BRANCH"; }
done

teardown
# teardown removed the worktree; assert it is gone (registration pruned)
$GIT worktree list | grep -q "pub-$ID" && die "postcondition: pub-$ID worktree survived"
log "published ${#copyset[@]} record(s) for id $ID onto $INT_BRANCH"
exit 0
```

`chmod +x scripts/terminal-publish.sh`.

- [ ] **Step 6: Run — verify the copy-set/gate section passes**

Run: `bash tests/test_closeout.sh`
Expected: all `publish:` asserts `ok`.

- [ ] **Step 7: Add the idempotency, CAS-retry-under-competing-push, and main-mode no-op tests**

Append to `tests/test_closeout.sh`:

```bash
# --- terminal-publish.sh: guarded no-op re-run (byte-identical second publish) ---
read -r W _ < <(new_repo)
"$ARCHIVE" --changes-dir "$W/docs/changes" --id 7 --outcome done --date 2026-06-18 >/dev/null 2>&1
( cd "$W" && "$PUBLISH" --id 7 --outcome done --integration-branch main --metadata-branch docket --changes-dir docs/changes --adrs-dir docs/adrs ) >/dev/null 2>&1
git -C "$W" fetch origin main >/dev/null 2>&1; before="$(git -C "$W" rev-parse origin/main)"
( cd "$W" && "$PUBLISH" --id 7 --outcome done --integration-branch main --metadata-branch docket --changes-dir docs/changes --adrs-dir docs/adrs ) >/dev/null 2>&1
rc=$?; git -C "$W" fetch origin main >/dev/null 2>&1; after="$(git -C "$W" rev-parse origin/main)"
assert "publish: re-run exits 0" "[ $rc -eq 0 ]"
assert "publish: re-run is a no-op (no new integration commit)" '[ "$before" = "$after" ]'

# --- terminal-publish.sh: CAS retry under a competing push between provision and push ---
# Inject a competing commit on origin/main mid-flight via GIT wrapper that fires once before the
# pub push. Simpler hermetic form: push a competing commit, then run publish and assert it still lands.
read -r W _ < <(new_repo)
"$ARCHIVE" --changes-dir "$W/docs/changes" --id 7 --outcome done --date 2026-06-18 >/dev/null 2>&1
# competing writer advances origin/main
comp="$(mktemp -d)"; git clone "$(git -C "$W" remote get-url origin)" "$comp" >/dev/null 2>&1
git -C "$comp" checkout main >/dev/null 2>&1
echo more >> "$comp/README.md"; git -C "$comp" -c user.email=c@c -c user.name=c commit -am "competing" >/dev/null 2>&1
git -C "$comp" push origin main >/dev/null 2>&1
( cd "$W" && "$PUBLISH" --id 7 --outcome done --integration-branch main --metadata-branch docket --changes-dir docs/changes --adrs-dir docs/adrs ) >/dev/null 2>&1
rc=$?; git -C "$W" fetch origin main >/dev/null 2>&1
assert "publish: CAS push succeeds despite a competing advance" "[ $rc -eq 0 ]"
assert "publish: copy-set landed atop the competing commit" 'git -C "$W" ls-tree -r --name-only origin/main | grep -q "docs/changes/archive/2026-06-18-0007-sample.md"'
assert "publish: competing commit preserved (not clobbered)" 'git -C "$W" log origin/main --oneline | grep -q competing'

# --- terminal-publish.sh: main-mode no-op ---
read -r W _ < <(new_repo)
( cd "$W" && "$PUBLISH" --id 7 --outcome done --integration-branch main --metadata-branch main --changes-dir docs/changes --adrs-dir docs/adrs ) >/dev/null 2>&1
assert "publish: main-mode exits 0 (no-op)" "[ $? -eq 0 ]"
assert "publish: main-mode created no pub worktree" '! git -C "$W" worktree list | grep -q "pub-7"'
```

- [ ] **Step 8: Run — verify the full terminal-publish section passes**

Run: `bash tests/test_closeout.sh`
Expected: all `publish:` asserts `ok`. Mutation-check the Accepted-gate assert (force ADR-0005 to `Accepted` in the fixture → "Proposed ADR-0005 SKIPPED" must flip to `NOT OK`), then restore.

- [ ] **Step 9: Commit**

```bash
git add scripts/terminal-publish.sh tests/test_closeout.sh
git commit -m "feat(0025): terminal-publish.sh — copy terminal records onto the integration branch (Accepted gate, CAS, main-mode no-op)"
```

---

## Task 3: `scripts/cleanup-feature-branch.sh` — provenance-guarded teardown

**Files:**
- Create: `scripts/cleanup-feature-branch.sh`
- Test: `tests/test_closeout.sh` (append the cleanup section)

**Interfaces:**
- Consumes: nothing from the lib (pure git).
- Produces (CLI contract relied on by Task 4/5 rewires):
  ```
  cleanup-feature-branch.sh --slug S [--worktrees-dir DIR] [--remote R]
  ```
  - Removes the worktree at `<worktrees-dir>/<slug>` (default `.worktrees/<slug>`, resolved relative to the repo root) **only if** its real path is under the repo root's `.worktrees/` directory — the provenance guard (never the `.docket/` metadata worktree, never an out-of-tree path). Refuse (`die`) otherwise.
  - Then deletes the local `feat/<slug>` branch and, if it exists, the remote `feat/<slug>`.
  - Fail-closed: self-verifies the worktree and branch are gone. `--remote` default `origin`.

- [ ] **Step 1: Write the failing test — happy path + provenance refusal**

Append to `tests/test_closeout.sh`:

```bash
# --- cleanup-feature-branch.sh: removes a worktree under .worktrees/<slug> + its branch ---
read -r W _ < <(new_repo)
git -C "$W" worktree add "$W/.worktrees/sample" -b feat/sample main >/dev/null 2>&1
git -C "$W" push -u origin feat/sample >/dev/null 2>&1
( cd "$W" && "$CLEANUP" --slug sample ) >/dev/null 2>&1
assert "cleanup: worktree removed" '[ ! -e "$W/.worktrees/sample" ]'
assert "cleanup: local branch deleted" '! git -C "$W" rev-parse --verify -q feat/sample >/dev/null'
assert "cleanup: remote branch deleted" '! git -C "$W" ls-remote --exit-code origin feat/sample >/dev/null 2>&1'

# --- cleanup-feature-branch.sh: provenance guard refuses an out-of-.worktrees path ---
read -r W _ < <(new_repo)
out="$(mktemp -d)/elsewhere"
git -C "$W" worktree add "$out" -b feat/evil main >/dev/null 2>&1
( cd "$W" && "$CLEANUP" --slug evil --worktrees-dir "$(dirname "$out")" ) >/dev/null 2>&1
assert "cleanup: refuses a worktree outside .worktrees/ (non-zero)" '[ $? -ne 0 ]'
assert "cleanup: out-of-tree worktree survives the refusal" '[ -e "$out" ]'
```

- [ ] **Step 2: Run — verify it fails**

Run: `bash tests/test_closeout.sh`
Expected: the `cleanup:` asserts print `NOT OK` (script missing).

- [ ] **Step 3: Implement the cleanup script**

Create `scripts/cleanup-feature-branch.sh`:

```bash
#!/usr/bin/env bash
# scripts/cleanup-feature-branch.sh — provenance-guarded teardown of a finished change's feature
# branch + worktree (change 0025). Removes the worktree ONLY if it resolves under the repo root's
# .worktrees/ (never the .docket/ metadata worktree, never an out-of-tree path), then deletes the
# local and remote feat/<slug> branch. Fail-closed: self-verifies both are gone.
#
# Usage: cleanup-feature-branch.sh --slug S [--worktrees-dir DIR] [--remote R]
#
# Mock seam: GIT="${GIT:-git}".
set -uo pipefail

GIT="${GIT:-git}"
SLUG="" WORKTREES_DIR=".worktrees" REMOTE="origin"

die(){ printf '%s\n' "cleanup-feature-branch: $*" >&2; exit 1; }
log(){ printf '%s\n' "cleanup-feature-branch: $*" >&2; }

while [ $# -gt 0 ]; do
  case "$1" in
    --slug) SLUG="$2"; shift ;;
    --worktrees-dir) WORKTREES_DIR="$2"; shift ;;
    --remote) REMOTE="$2"; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
  shift
done
[ -n "$SLUG" ] || die "missing --slug"

root="$($GIT rev-parse --show-toplevel)" || die "not in a git repo"
canon(){ ( cd "$1" 2>/dev/null && pwd -P ); }   # realpath of an existing dir, else empty

target="$WORKTREES_DIR/$SLUG"
allowed_root="$(canon "$root")/.worktrees"

# provenance guard: the worktree, if present, must resolve under <root>/.worktrees/
if [ -e "$target" ]; then
  rp="$(canon "$target")"
  case "$rp/" in
    "$allowed_root/"*) ;;   # under .worktrees/ — allowed
    *) die "refusing to remove worktree outside .worktrees/: $rp" ;;
  esac
  $GIT worktree remove --force "$target" >/dev/null 2>&1 || die "worktree remove failed: $target"
fi

# delete local + remote feat/<slug>
$GIT branch -D "feat/$SLUG" >/dev/null 2>&1 || true
if $GIT ls-remote --exit-code "$REMOTE" "feat/$SLUG" >/dev/null 2>&1; then
  $GIT push "$REMOTE" --delete "feat/$SLUG" >/dev/null 2>&1 || die "remote branch delete failed"
fi

# fail-closed self-verification
[ ! -e "$target" ] || die "postcondition: worktree still present"
$GIT rev-parse --verify -q "feat/$SLUG" >/dev/null && die "postcondition: local branch still present"
log "cleaned up feat/$SLUG"
exit 0
```

`chmod +x scripts/cleanup-feature-branch.sh`.

- [ ] **Step 4: Run — verify the cleanup section passes**

Run: `bash tests/test_closeout.sh`
Expected: all `cleanup:` asserts `ok`. Mutation-check the provenance assert (temporarily weaken the guard `case` to always-allow → "refuses … (non-zero)" + "survives" must flip to `NOT OK`), then restore.

- [ ] **Step 5: Commit**

```bash
git add scripts/cleanup-feature-branch.sh tests/test_closeout.sh
git commit -m "feat(0025): cleanup-feature-branch.sh — provenance-guarded worktree + branch teardown"
```

---

## Task 4: Rewire `docket-finalize-change` (the single source) to invoke the scripts

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md` — *Per-change steps* step 3 (Archive), step 4 (Clean up), and the *Terminal publish (docket-mode)* section.
- Test: `tests/test_closeout.sh` (append the finalize wiring sentinels).

**Interfaces:**
- Consumes: the three scripts' CLI contracts from Tasks 1–3.
- Produces: prose that invokes the scripts; the wiring sentinels in Task 5 also assert this file.

**Context:** This skill is the documented **single source** of the shared procedure — the convention and three other skills reference *“the Terminal publish (docket-mode) procedure in docket-finalize-change”* by name. So the rewire must (a) keep the section heading and the cross-ref anchors intact (LEARNINGS #20 — name-based cross-refs must still resolve), (b) replace the *mechanics* (the bash) with a script invocation while keeping finalize the documented owner of *when/what* (exactly as `docket-status` delegates rendering to `render-board.sh` while owning when to render), and (c) preserve the load-bearing semantics in prose: change-file-only commit, the Accepted gate, archive-before-publish ordering, the trust-the-exit-code / abort-and-report contract.

- [ ] **Step 1: Rewrite step 3 (Archive) to invoke `archive-change.sh`**

In `skills/docket-finalize-change/SKILL.md`, replace the mechanics of *Per-change steps* step 3 (a–e) with: compute the UTC merge date (unchanged — `gh mergedAt` / `TZ=UTC git show`), author the commit message, then invoke `scripts/archive-change.sh --changes-dir .docket/<changes_dir> --id <id> --outcome done --date <merge-date> [--results <path>] --message "<msg>"`; **trust the exit code** — 0 ⇒ archived (idempotent no-op if already archived), non-zero ⇒ abort-and-report. Keep the explanatory sentence that this is step 1 of terminal-publish (archive-on-`docket`-first) and that in `docket`-mode the terminal-publish invocation follows. Keep the cross-reference note that the procedure is identical to `docket-status`'s sweep. Do not restate the `git mv`/frontmatter/commit bash — the script owns it.

- [ ] **Step 2: Rewrite the *Terminal publish (docket-mode)* section to invoke `terminal-publish.sh`**

Replace the Step 1–4 bash blocks (archive-first, provision `pub-<T>`, copy + CAS push, teardown) with: keep the section heading, the "single source" sentence, the two entry shapes (change publish `T=<id>` vs ADR-only publish `T=adr-<NN>`), the main-mode skip note, and the copy-set definition (change file always; spec iff set; Accepted ADRs only — the **Accepted gate** stays documented here). Then state that the **change-publish** path runs `scripts/archive-change.sh` (step 1, archive-first) followed by `scripts/terminal-publish.sh --id <id> --outcome <done|killed> --integration-branch <int> --metadata-branch <meta> --changes-dir <changes_dir> --adrs-dir <adrs_dir> --message "<msg>"`, trusting the exit code (the script self-verifies the copy-set landed and tears down `pub-<id>`). Preserve the **ADR-only publish** path (`T=adr-<NN>`, from `docket-adr`) as documented prose — `terminal-publish.sh` as specced handles change publishes; note that the ADR-only variant remains `docket-adr`'s responsibility (it is out of this change's scope — the spec scopes terminal-publish.sh to the change copy-set; do not silently extend it). Keep ordering ("step 1 archives so step 3 can copy the archived path") explicit.

- [ ] **Step 3: Rewrite step 4 (Clean up) to invoke `cleanup-feature-branch.sh`**

Replace the prose mechanics of step 4 with: invoke `scripts/cleanup-feature-branch.sh --slug <slug>`; trust the exit code (the provenance guard + self-verification live in the script). Keep the one-sentence statement of the invariant (only worktrees under `.worktrees/<slug>` are removed; never `.docket/`).

- [ ] **Step 4: Write the finalize wiring sentinels**

Append to `tests/test_closeout.sh`:

```bash
FINALIZE="$REPO/skills/docket-finalize-change/SKILL.md"
assert "wiring(finalize): invokes archive-change.sh" 'grep -q "scripts/archive-change.sh" "$FINALIZE"'
assert "wiring(finalize): invokes terminal-publish.sh" 'grep -q "scripts/terminal-publish.sh" "$FINALIZE"'
assert "wiring(finalize): invokes cleanup-feature-branch.sh" 'grep -q "scripts/cleanup-feature-branch.sh" "$FINALIZE"'
assert "wiring(finalize): Terminal publish section heading preserved (cross-ref anchor)" 'grep -qF "## Terminal publish (docket-mode)" "$FINALIZE"'
assert "wiring(finalize): Accepted gate still documented" 'grep -qiE "whose ADR is .?Accepted|Accepted. gate" "$FINALIZE"'
assert "wiring(finalize): no leftover raw archive bash (git mv active/)" '! grep -qE "git mv .*active/" "$FINALIZE"'
```

(The last assert is the de-duplication proof: the raw `git mv active/…` mechanics must be **gone** from the prose, now living only in the script. Mutation-check it by pasting a `git mv active/X` line back — it must flip to `NOT OK`.)

- [ ] **Step 5: Run — verify the finalize wiring passes**

Run: `bash tests/test_closeout.sh`
Expected: all `wiring(finalize):` asserts `ok`. Mutation-check each per the Global Constraints (delete the real `scripts/archive-change.sh` reference → its assert flips to `NOT OK`; restore).

- [ ] **Step 6: Commit**

```bash
git add skills/docket-finalize-change/SKILL.md tests/test_closeout.sh
git commit -m "docs(0025): rewire docket-finalize-change to invoke the close-out scripts (single source)"
```

---

## Task 5: Rewire the three referencing call sites + final verification

**Files:**
- Modify: `skills/docket-status/SKILL.md` (merge-sweep archive loop), `skills/docket-new-change/SKILL.md` (proposed-kill clause), `skills/docket-implement-next/SKILL.md` (reconcile-kill clause).
- Test: `tests/test_closeout.sh` (append the remaining wiring sentinels).

**Interfaces:**
- Consumes: the script CLI contracts (Tasks 1–3) and finalize as the single source (Task 4).
- Produces: the final wired state of all four call sites.

**Context:** These three sites already *reference* the shared procedure by name rather than restating its bash, so each edit is light: where the site currently says "run the shared *Terminal publish (docket-mode)* procedure," add that the mechanics are now the scripts (`archive-change.sh` then, in `docket`-mode, `terminal-publish.sh`; cleanup via `cleanup-feature-branch.sh` where a branch/worktree exists), invoked with an authored `--message`, trusting the exit code. Keep every cross-ref to finalize as the single source intact. Do **not** duplicate the script CLI documentation — point at finalize.

- [ ] **Step 1: Rewire `docket-status` merge-sweep**

In `skills/docket-status/SKILL.md`, in the merge-sweep section, where it archives each swept `done` change and (in `docket`-mode) publishes the terminal record: state that the archive + terminal-publish mechanics are `scripts/archive-change.sh` then `scripts/terminal-publish.sh` (the same invocations finalize uses — it is the single source), and that cleanup of any leftover feature branch/worktree uses `scripts/cleanup-feature-branch.sh`. Keep "the sweep only archives already-merged PRs; it never merges" and the idempotent-racing-finalize note. Trust the exit code.

- [ ] **Step 2: Rewire `docket-new-change` proposed-kill**

In `skills/docket-new-change/SKILL.md`, in the proposed-kill clause: the `docket`-mode path sets `status: killed` + `## Why killed` then runs the shared terminal-publish — restate the *mechanics* as `scripts/archive-change.sh --outcome killed --reason "<why killed text>" …` (which performs the dated archive move + the `## Why killed` insertion + the change-file-only commit) followed by `scripts/terminal-publish.sh --outcome killed …`. Keep the `main`-mode degradation note (archive in place, no terminal-publish — which is `terminal-publish.sh`'s own mode-guard no-op) and the "writes markdown only" framing. Keep the must-land Board pass (`render-board.sh`) unchanged.

- [ ] **Step 3: Rewire `docket-implement-next` reconcile-kill**

In `skills/docket-implement-next/SKILL.md`, in the Step 3 reconcile-kill escape hatch: restate the mechanics as `scripts/archive-change.sh --outcome killed --reason "…" …` then (`docket`-mode) `scripts/terminal-publish.sh --outcome killed …`, and prune any already-created feature worktree/branch via `scripts/cleanup-feature-branch.sh --slug <slug>`. Keep the `main`-mode degradation note and the loop-back-to-Step-1 control flow. Trust the exit code.

- [ ] **Step 4: Write the remaining wiring sentinels**

Append to `tests/test_closeout.sh`:

```bash
STATUS="$REPO/skills/docket-status/SKILL.md"
NEWCHG="$REPO/skills/docket-new-change/SKILL.md"
IMPL="$REPO/skills/docket-implement-next/SKILL.md"
assert "wiring(status): sweep invokes archive-change.sh"   'grep -q "scripts/archive-change.sh" "$STATUS"'
assert "wiring(status): sweep invokes terminal-publish.sh" 'grep -q "scripts/terminal-publish.sh" "$STATUS"'
assert "wiring(new-change): proposed-kill invokes archive-change.sh"   'grep -q "scripts/archive-change.sh" "$NEWCHG"'
assert "wiring(new-change): proposed-kill invokes terminal-publish.sh" 'grep -q "scripts/terminal-publish.sh" "$NEWCHG"'
assert "wiring(implement-next): reconcile-kill invokes archive-change.sh"     'grep -q "scripts/archive-change.sh" "$IMPL"'
assert "wiring(implement-next): reconcile-kill invokes cleanup-feature-branch.sh" 'grep -q "scripts/cleanup-feature-branch.sh" "$IMPL"'
```

- [ ] **Step 5: Run the FULL suite — everything green**

Run: `bash tests/test_closeout.sh`
Expected: every line `ok - `, no `NOT OK`, exit 0. Then run the surrounding suites that touch these files to confirm no regression:
Run: `bash tests/test_render_board.sh && bash tests/test_convention_extraction.sh && bash tests/test_composition_wiring.sh`
Expected: each exits 0 (if any asserts on the edited skill prose, confirm still green; investigate any failure before proceeding).

- [ ] **Step 6: Add the exit-code line + run-all check to test_closeout.sh and commit**

Ensure `tests/test_closeout.sh` ends with:

```bash
exit "$fail"
```

```bash
git add skills/docket-status/SKILL.md skills/docket-new-change/SKILL.md skills/docket-implement-next/SKILL.md tests/test_closeout.sh
git commit -m "docs(0025): rewire the status sweep + two kill paths to invoke the close-out scripts"
```

---

## Closing notes (build-time, outside the feature-branch tasks)

- **ADR-0002 `## Update` (the spec §2 / change Open-question review item).** Decide at review whether ADR-0002 ("terminal-publish single-sourced in finalize") wants a one-line `## Update` clarifying that finalize remains the documented owner while delegating the *mechanics* to `terminal-publish.sh` (exactly as `docket-status` delegates to `render-board.sh`). If yes, this is a **metadata** edit on the `docket` branch (the ADR lives in `.docket/docs/adrs/`), made via the `docket-adr` flow during Step 6 — **never** in this feature worktree. ADR-0002 is already in the change's `adrs: [1,2,7]`, so terminal-publish re-copies it onto `main` on merge (LEARNINGS #17 — the atomic-delivery pattern). No **new** ADR is required (faithful extraction).
- **No silent scope override (LEARNINGS #21).** If the build finds a spec rationale wrong but its conclusion sound, record it in the results file; do not silently re-scope.

## Self-Review

- **Spec coverage:** §4a→Task 1, §4b→Task 2, §4c→Task 3, §3 git-write boundary + fail-closed→baked into every script's commit/CAS/self-verify steps, §5 rewire four call sites→Tasks 4–5, §8 testing (archive done/killed/reuse, publish copy-set+gate+CAS+teardown+main-mode, cleanup removal+guard, fail-closed deviations)→test sections in Tasks 1–3, §2 ADR-0002 review item→closing note. No gaps.
- **Placeholder scan:** every code step shows complete bash; no TBD/TODO.
- **Type/contract consistency:** the CLI flags in the Task 4/5 invocation prose match the `Interfaces` blocks of Tasks 1–3 (`--changes-dir`, `--id`, `--outcome`, `--date`, `--results`, `--reason`, `--integration-branch`, `--metadata-branch`, `--adrs-dir`, `--slug`, `--message`).
