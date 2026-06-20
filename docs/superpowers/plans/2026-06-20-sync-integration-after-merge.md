# Fast-forward the local integration branch after a docket merge — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a best-effort, FF-only helper that fast-forwards the clone's local integration-branch checkout after a docket merge lands, and wire it into docket's two merge sites — so the symlinked skills the harness loads stop drifting commits behind `origin/<integration_branch>`.

**Architecture:** A small guarded shell script `scripts/sync-integration-branch.sh` does the mechanics (on-branch + clean + true-FF triple gate, then `git merge --ff-only`); it is **best-effort like `github-mirror.sh`** — every skip condition (wrong branch, dirty tree, non-FF divergence, fetch failure, not-a-repo) is a normal `exit 0` with a one-line note, never an abort. Both merge sites (`docket-finalize-change`, the `docket-status` merge sweep) invoke it once at end of run. `docket-convention`'s Branch model gets a one-sentence pointer as the single documented source.

**Tech Stack:** POSIX-ish bash (`set -uo pipefail`), git, a hermetic bash test harness in the `tests/test_closeout.sh` style (temp repo + local bare origin, no network, no `gh`).

## Global Constraints

- **Best-effort, never fail-closed.** The helper runs *after* the merge has already landed; it must **never abort or alter the close-out**. Runtime conditions (wrong branch, dirty tree, non-FF, fetch failure, not-a-git-repo) → `exit 0` + stderr note. Only a genuine **usage** error (missing required `--integration-branch`, unknown flag) → `exit 2`, mirroring `github-mirror.sh`'s split (missing `--changes-dir` → 2; dir-not-found → 0).
- **FF-only triple gate** (verbatim from spec §3): act **only** when the checkout is *on* `<integration_branch>` AND the working tree is clean AND `origin/<branch>` is strictly ahead with the local tip as an ancestor (a true fast-forward). Never a merge commit, never a checkout switch, never a stash.
- **Mock seam:** `GIT="${GIT:-git}"` (same convention as `cleanup-feature-branch.sh` / `render-board.sh`).
- **All notes to stderr**, stdout stays clean (same as `github-mirror.sh` / `cleanup-feature-branch.sh`).
- **Hermetic test, pristine stderr.** Silence the `warning: You appear to have cloned an empty repository` in fixture builders so a green run leaves the *helper's* stderr the only stderr (LEARNINGS 2026-06-19 #22/#26). Never `producer | grep -q` under `pipefail` — capture into a var first (LEARNINGS #16/#11). On macOS, resolve paths with `pwd -P` before comparing (`mktemp` → `/var`, git → `/private/var`; LEARNINGS #25). Every assertion must be **non-vacuous** — each skip case advances `origin` first so a working FF *would* move the tip, proving the guard (not "already current") is what blocked it (LEARNINGS #2/#6).

---

### Task 1: The `sync-integration-branch.sh` helper + its hermetic test

**Files:**
- Create: `scripts/sync-integration-branch.sh`
- Create: `tests/test_sync_integration_branch.sh`

**Interfaces:**
- Consumes: nothing (leaf script).
- Produces: CLI `sync-integration-branch.sh --integration-branch BR [--clone-dir DIR] [--remote R]`.
  - `--integration-branch` **required**; missing → `exit 2`.
  - `--clone-dir` defaults to the script's own repo root, resolved `cd "$(dirname "$0")/.." && pwd -P`.
  - `--remote` defaults to `origin`.
  - Behavior: skip-with-note + `exit 0` unless on `<integration_branch>`, clean, fetch succeeds, and a true FF is available; otherwise `git merge --ff-only`. Mock seam `GIT="${GIT:-git}"`.

- [ ] **Step 1: Write the failing test**

Create `tests/test_sync_integration_branch.sh` with all six hermetic cases from spec §4. Model the fixture + assert helpers on `tests/test_closeout.sh`.

```bash
#!/usr/bin/env bash
# tests/test_sync_integration_branch.sh — verifies change 0029: the best-effort, FF-only
# sync-integration-branch.sh helper. Hermetic: a temp clone with a local *bare* origin holding
# main; no gh, no network. Run: bash tests/test_sync_integration_branch.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
HELPER="$REPO/scripts/sync-integration-branch.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

git_quiet(){ git "$@" >/dev/null 2>&1; }   # silences the empty-bare-clone warning in fixtures

# new_repo: prints "<work> <origin>" — a fresh clone with a bare origin holding main@C0.
# C0 carries skills/sentinel.txt so an FF can be observed in the working tree.
new_repo(){
  local root origin work
  root="$(mktemp -d)"; root="$(cd "$root" && pwd -P)"   # macOS /var vs /private/var
  origin="$root/origin.git"; work="$root/work"
  git_quiet init --bare "$origin"
  git_quiet clone "$origin" "$work"
  git -C "$work" config user.email t@t; git -C "$work" config user.name t
  git -C "$work" checkout -b main >/dev/null 2>&1
  mkdir -p "$work/skills"; echo "v0" > "$work/skills/sentinel.txt"
  git -C "$work" add skills/sentinel.txt; git_quiet -C "$work" commit -m "C0 baseline"
  git_quiet -C "$work" push -u origin main
  printf '%s %s' "$work" "$origin"
}

# advance_origin WORK: push a new commit (C1, sentinel=v1) to origin WITHOUT moving WORK's
# local main — emulates origin advancing under a stale primary checkout. Uses a throwaway clone.
advance_origin(){
  local work="$1" origin tmp
  origin="$(git -C "$work" remote get-url origin)"
  tmp="$(mktemp -d)"; git_quiet clone "$origin" "$tmp/c"
  git -C "$tmp/c" config user.email t@t; git -C "$tmp/c" config user.name t
  git -C "$tmp/c" checkout main >/dev/null 2>&1
  echo "v1" > "$tmp/c/skills/sentinel.txt"
  git -C "$tmp/c" add skills/sentinel.txt; git_quiet -C "$tmp/c" commit -m "C1 advance"
  git_quiet -C "$tmp/c" push origin main
}

# --- Case 1: FF case — origin advanced, clone on main & clean → FF to origin tip ---
read -r W O < <(new_repo)
advance_origin "$W"
before="$(git -C "$W" rev-parse HEAD)"
out="$("$HELPER" --clone-dir "$W" --integration-branch main 2>&1)"; rc=$?
after="$(git -C "$W" rev-parse HEAD)"
remote_tip="$(git -C "$W" rev-parse origin/main 2>/dev/null)"
sentinel="$(cat "$W/skills/sentinel.txt")"
assert "FF: exit 0"                       "[ $rc -eq 0 ]"
assert "FF: local advanced past C0"       "[ '$after' != '$before' ]"
assert "FF: local now equals origin tip"  "[ '$after' = '$remote_tip' ]"
assert "FF: working tree updated to v1"   "[ '$sentinel' = 'v1' ]"

# --- Case 2: dirty tree — uncommitted change blocks the FF even though origin advanced ---
read -r W O < <(new_repo)
advance_origin "$W"
echo "dirty" >> "$W/skills/sentinel.txt"        # uncommitted edit
before="$(git -C "$W" rev-parse HEAD)"
out="$("$HELPER" --clone-dir "$W" --integration-branch main 2>&1)"; rc=$?
after="$(git -C "$W" rev-parse HEAD)"
assert "dirty: exit 0"                     "[ $rc -eq 0 ]"
assert "dirty: tip unchanged (no FF)"      "[ '$after' = '$before' ]"
assert "dirty: note mentions clean/dirty"  "printf '%s' \"\$out\" | grep -qiE 'clean|dirty|uncommitted'"

# --- Case 3: wrong branch — clone on a feature branch → skip even though origin advanced ---
read -r W O < <(new_repo)
advance_origin "$W"
git -C "$W" checkout -b feat/x >/dev/null 2>&1
mainref_before="$(git -C "$W" rev-parse main)"
out="$("$HELPER" --clone-dir "$W" --integration-branch main 2>&1)"; rc=$?
mainref_after="$(git -C "$W" rev-parse main)"
cur="$(git -C "$W" symbolic-ref --short -q HEAD)"
assert "wrong-branch: exit 0"              "[ $rc -eq 0 ]"
assert "wrong-branch: still on feat/x"     "[ '$cur' = 'feat/x' ]"
assert "wrong-branch: main ref untouched"  "[ '$mainref_after' = '$mainref_before' ]"
assert "wrong-branch: note mentions branch" "printf '%s' \"\$out\" | grep -qiE 'branch|not on'"

# --- Case 4: non-FF divergence — local has a commit origin doesn't → skip, no merge commit ---
read -r W O < <(new_repo)
echo "local-only" > "$W/skills/sentinel.txt"
git -C "$W" add skills/sentinel.txt; git_quiet -C "$W" commit -m "C1prime local"
advance_origin "$W"                              # origin gets a DIFFERENT C1
before="$(git -C "$W" rev-parse HEAD)"
out="$("$HELPER" --clone-dir "$W" --integration-branch main 2>&1)"; rc=$?
after="$(git -C "$W" rev-parse HEAD)"
parents="$(git -C "$W" rev-list --parents -n1 HEAD | wc -w | tr -d ' ')"
assert "non-FF: exit 0"                    "[ $rc -eq 0 ]"
assert "non-FF: tip unchanged"             "[ '$after' = '$before' ]"
assert "non-FF: no merge commit (1 parent)" "[ '$parents' = '2' ]"   # 'sha parent' = 2 words ⇒ single parent

# --- Case 5: already current — origin not advanced → no-op ---
read -r W O < <(new_repo)
before="$(git -C "$W" rev-parse HEAD)"
out="$("$HELPER" --clone-dir "$W" --integration-branch main 2>&1)"; rc=$?
after="$(git -C "$W" rev-parse HEAD)"
assert "current: exit 0"                   "[ $rc -eq 0 ]"
assert "current: tip unchanged"            "[ '$after' = '$before' ]"
assert "current: note mentions current"    "printf '%s' \"\$out\" | grep -qiE 'current|up.to.date|already'"

# --- Case 6: fetch failure — origin advanced then made unreachable → skip with note ---
read -r W O < <(new_repo)
advance_origin "$W"                              # origin is now ahead (C1)...
git -C "$W" remote set-url origin /nonexistent/path.git   # ...but unreachable
before="$(git -C "$W" rev-parse HEAD)"
out="$("$HELPER" --clone-dir "$W" --integration-branch main 2>&1)"; rc=$?
after="$(git -C "$W" rev-parse HEAD)"
assert "fetch-fail: exit 0"                "[ $rc -eq 0 ]"
assert "fetch-fail: tip unchanged (no FF)" "[ '$after' = '$before' ]"
assert "fetch-fail: note mentions fetch"   "printf '%s' \"\$out\" | grep -qiE 'fetch'"

# --- Case 7: usage error — missing required --integration-branch → exit 2 ---
read -r W O < <(new_repo)
out="$("$HELPER" --clone-dir "$W" 2>&1)"; rc=$?
assert "usage: missing --integration-branch exits 2" "[ $rc -eq 2 ]"

if [ "$fail" -eq 0 ]; then echo "ALL PASS"; else echo "FAILURES"; fi
exit "$fail"
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_sync_integration_branch.sh`
Expected: FAIL — every case errors because `scripts/sync-integration-branch.sh` does not exist yet (non-zero exit / "No such file"). `fail=1`, final line `FAILURES`.

- [ ] **Step 3: Write the helper script**

Create `scripts/sync-integration-branch.sh`:

```bash
#!/usr/bin/env bash
# scripts/sync-integration-branch.sh — best-effort, FF-only sync of a clone's local
# integration-branch checkout to its remote after a docket merge (change 0029). Runs at docket's
# two merge sites (docket-finalize-change, the docket-status merge sweep) so the skills symlinked
# from the primary checkout stop drifting behind origin/<integration_branch>.
#
# Best-effort like github-mirror.sh (NOT fail-closed like archive-change.sh): the merge has
# already landed, so this is downstream housekeeping. Every runtime skip — wrong branch, dirty
# tree, non-FF divergence, fetch failure, not-a-repo — is a normal exit 0 with a one-line note.
# It never aborts or alters the close-out. Only a usage error (missing --integration-branch,
# unknown flag) exits non-zero.
#
# Triple gate (acts only when ALL hold): on <integration-branch> AND clean tree AND origin/<branch>
# strictly ahead with the local tip an ancestor (a true fast-forward). Then: git merge --ff-only.
#
# Usage: sync-integration-branch.sh --integration-branch BR [--clone-dir DIR] [--remote R]
#   --clone-dir defaults to the script's own repo root.  --remote defaults to origin.
#
# Mock seam: GIT="${GIT:-git}".
set -uo pipefail

GIT="${GIT:-git}"
BRANCH="" CLONE_DIR="" REMOTE="origin"

note(){ printf '%s\n' "sync-integration-branch: $*" >&2; }
die(){  printf '%s\n' "sync-integration-branch: $*" >&2; exit 2; }

while [ $# -gt 0 ]; do
  case "$1" in
    --integration-branch) BRANCH="$2"; shift ;;
    --clone-dir) CLONE_DIR="$2"; shift ;;
    --remote) REMOTE="$2"; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
  shift
done
[ -n "$BRANCH" ] || die "missing --integration-branch"

# --clone-dir defaults to this script's repo root.
if [ -z "$CLONE_DIR" ]; then
  CLONE_DIR="$(cd "$(dirname "$0")/.." && pwd -P)"
fi

# not-a-repo → best-effort skip (never abort the close-out).
if ! "$GIT" -C "$CLONE_DIR" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  note "not a git work tree: $CLONE_DIR — skipping"; exit 0
fi

# Gate 1: on the integration branch? (detached HEAD → empty → skip)
cur="$("$GIT" -C "$CLONE_DIR" symbolic-ref --short -q HEAD || true)"
if [ "$cur" != "$BRANCH" ]; then
  note "checkout is on '${cur:-(detached)}', not '$BRANCH' — skipping"; exit 0
fi

# Gate 2: clean working tree? (any porcelain output — tracked or untracked-non-ignored — blocks)
if [ -n "$("$GIT" -C "$CLONE_DIR" status --porcelain 2>/dev/null)" ]; then
  note "working tree not clean — skipping (no fast-forward onto local edits)"; exit 0
fi

# Fetch the branch (cheap/no-op for the merge sites, which already fetched). Swallow git's own
# stderr; on failure emit our own note and skip.
if ! "$GIT" -C "$CLONE_DIR" fetch "$REMOTE" "$BRANCH" >/dev/null 2>&1; then
  note "fetch of $REMOTE/$BRANCH failed — skipping (best-effort)"; exit 0
fi

local_tip="$("$GIT" -C "$CLONE_DIR" rev-parse HEAD)"
remote_tip="$("$GIT" -C "$CLONE_DIR" rev-parse FETCH_HEAD)"

# Already current?
if [ "$local_tip" = "$remote_tip" ]; then
  note "$BRANCH already current ($local_tip) — nothing to fast-forward"; exit 0
fi

# Gate 3: true fast-forward? (local tip must be an ancestor of the fetched tip)
if ! "$GIT" -C "$CLONE_DIR" merge-base --is-ancestor "$local_tip" "$remote_tip"; then
  note "$REMOTE/$BRANCH has diverged from local (not a fast-forward) — skipping"; exit 0
fi

# All gates pass: fast-forward only.
if "$GIT" -C "$CLONE_DIR" merge --ff-only FETCH_HEAD >/dev/null 2>&1; then
  note "fast-forwarded $BRANCH ${local_tip:0:9}..${remote_tip:0:9}"
else
  note "fast-forward merge failed unexpectedly — skipping (best-effort)"
fi
exit 0
```

Then make it executable:

```bash
chmod +x scripts/sync-integration-branch.sh
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_sync_integration_branch.sh`
Expected: every line `ok - …`, final line `ALL PASS`, exit 0.

- [ ] **Step 5: Confirm pristine stderr on the green run**

Run: `bash tests/test_sync_integration_branch.sh 2>/tmp/sync-stderr.txt >/dev/null; echo "rc=$?"; wc -c < /tmp/sync-stderr.txt`
Expected: `rc=0` and `0` bytes of stderr (the fixture's `git_quiet` silenced the empty-bare-clone warnings; the helper's own notes were captured via `2>&1` inside each case, not leaked). If non-zero bytes, find and silence the leak before committing (LEARNINGS #22/#26).

- [ ] **Step 6: Commit**

```bash
git add scripts/sync-integration-branch.sh tests/test_sync_integration_branch.sh
git commit -m "feat(0029): best-effort FF-only sync-integration-branch.sh + hermetic test"
```

---

### Task 2: Wire the two merge sites + the convention pointer

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md` (after the close-out's step 5 "Board")
- Modify: `skills/docket-status/SKILL.md` (after the merge sweep)
- Modify: `skills/docket-convention/SKILL.md` (one sentence in the Branch model)

**Interfaces:**
- Consumes: the `scripts/sync-integration-branch.sh` CLI from Task 1.
- Produces: prose call-sites + a single documented source. No automated test — verified by whole-branch review reading for meaning (LEARNINGS 2026-06-10 #5; the regression-guard sentinel was considered and declined in 0028's scope).

- [ ] **Step 1: Add the finalize call site (after step 5 "Board")**

In `skills/docket-finalize-change/SKILL.md`, the numbered close-out ends at step 5 (Board, the line beginning `5. **Board** — regenerate \`BOARD.md\``). Add a new step 6 immediately after step 5's paragraph (before the `**Note:**` paragraph that begins "This archive procedure is **identical**…"):

```markdown
6. **Sync the integration checkout (best-effort)** — once at the very end of the run (after the board step, so a batch finalize fast-forwards once after all its merges): `scripts/sync-integration-branch.sh --integration-branch <integration_branch>`. This fast-forwards the clone's local `<integration_branch>` checkout to the tip the merges just pushed, keeping the skills symlinked from it current (change 0029). It is **best-effort like the board** (per the convention's Branch model): FF-only, guarded (on-branch + clean + true-FF), and it **never aborts or alters the close-out** — every skip (wrong branch, dirty tree, non-FF, fetch failure) is a normal exit 0. A no-op in `main`-mode where the metadata working tree already *is* the integration checkout.
```

- [ ] **Step 2: Add the status merge-sweep call site**

In `skills/docket-status/SKILL.md`, the `## Merge sweep` section's numbered steps run through `h. **Harvest learnings**` (around line 156). Add a final sweep step after the harvest step (still inside the per-sweep procedure's end, as the single end-of-pass action mirroring finalize):

```markdown
   i. **Sync the integration checkout (best-effort)** — once after the sweep's merges + publishes complete: `scripts/sync-integration-branch.sh --integration-branch <integration_branch>`. Same best-effort, FF-only helper finalize runs (change 0029) — it fast-forwards the clone's local `<integration_branch>` checkout so the symlinked skills track the just-swept merges. Omitting it would leave swept close-outs stale. Best-effort like the board: never aborts the sweep; a no-op in `main`-mode.
```

(If the sweep's per-change steps are lettered `a..h` inside a loop, place this as the post-loop end-of-pass action so it runs **once** per sweep, not once per swept change — match the single end-of-run placement finalize uses. Read the surrounding step structure and adjust the letter/indent to fit; the contract is "once at end of the sweep.")

- [ ] **Step 3: Add the convention Branch-model pointer**

In `skills/docket-convention/SKILL.md`, the `### Branch model` paragraph (the one beginning "Metadata (change files, `BOARD.md`, ADRs, specs) commits to `metadata_branch`…") ends with the terminal-publish sentence in parentheses. Append one sentence at the very end of that paragraph as the single documented source:

```markdown
 After a merge lands on `origin/<integration_branch>`, both merge sites (`docket-finalize-change` and the `docket-status` sweep) run the best-effort, FF-only `scripts/sync-integration-branch.sh` once at end of run to fast-forward the clone's *local* `<integration_branch>` checkout — keeping the skills symlinked from the primary checkout current in `docket`-mode, where it is otherwise never advanced (a no-op in `main`-mode and on any non-FF/dirty/feature-branch tree).
```

- [ ] **Step 4: Verify the wiring reads correctly**

Run: `grep -n 'sync-integration-branch.sh' skills/docket-finalize-change/SKILL.md skills/docket-status/SKILL.md skills/docket-convention/SKILL.md`
Expected: exactly one reference in each of the three files (3 lines total). Re-read each surrounding paragraph to confirm the placement reads as "once at end of run, best-effort" and does not duplicate the convention's single source.

- [ ] **Step 5: Run the full test suite (no regression)**

Run: `for t in tests/test_*.sh; do echo "== $t =="; bash "$t" >/tmp/t.out 2>&1 || { echo "FAILED: $t"; tail -5 /tmp/t.out; }; done; echo done`
Expected: no `FAILED:` lines — the prose edits touch no script behavior, so every existing test still passes alongside the new one.

- [ ] **Step 6: Commit**

```bash
git add skills/docket-finalize-change/SKILL.md skills/docket-status/SKILL.md skills/docket-convention/SKILL.md
git commit -m "docs(0029): wire sync-integration-branch.sh into finalize + status sweep; convention pointer"
```

---

## Self-Review

**Spec coverage:**
- §3 helper (`--clone-dir`/`--integration-branch`/`--remote`, triple gate, `merge --ff-only`, best-effort, `--clone-dir` default) → Task 1 Step 3. ✓
- §3 both call sites (finalize after board; status sweep) + one-line prose each + convention pointer → Task 2 Steps 1–3. ✓
- §4 test cases (FF / dirty / wrong-branch / non-FF / already-current / fetch-failure) → Task 1 Step 1 cases 1–6 (+ case 7 usage-error, + pristine-stderr check in Step 5). ✓
- §2 non-goals (no Step 0 guard, no auto-restart, no re-link, no dirty/diverged touch) → respected; nothing in the plan adds them. ✓
- §5 caveats / §6 risk → behavioral (guards), covered by the triple gate + tests. ✓

**Placeholder scan:** No TBD/TODO/"handle edge cases"; every code step shows complete content. ✓

**Type consistency:** The CLI flags (`--integration-branch`, `--clone-dir`, `--remote`), the `note`/`die` helpers, and `FETCH_HEAD`/`merge-base --is-ancestor`/`merge --ff-only` usage are identical across the script (Task 1) and every call site (Task 2). Test invokes the same flags. ✓
