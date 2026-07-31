<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0174 — Reuse test git fixtures instead of rebuilding them per assertion](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0174-reuse-test-git-fixtures.md)**
<!-- docket:backlink:end -->

# Reuse test git fixtures Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cut ~180s of suite wall clock by building each affected test file's git fixture once into a template and copying it per call, instead of rebuilding a byte-identical repository from `init`/`clone`/`commit`/`push` at every call site.

**Architecture:** Four test files each own a fixture-builder helper. Each helper keeps its **name, signature, and return contract exactly as they are** and changes only its body: a lazily-built per-file template holds the baseline repository, and every call — including the first — produces its fixture by `cp -R` of the template plus a rewrite of the copy's `remote.origin.url` so the fixture pushes into its **own** origin. No call site changes. Each changed helper gains an explicit fixture-independence assertion block, because fixture coupling manifests as order-dependent flakiness that a single green run cannot rule out.

**Tech Stack:** Bash (`set -uo pipefail`), git plumbing, the repo's existing hand-rolled `assert` test harness (`ok - ` / `NOT OK - ` lines on stdout). No test framework, no new dependencies, no new files.

## Global Constraints

- **No call site may change.** The four helpers keep name, parameter list, and return/print contract byte-identical. The diff is helper bodies plus one new independence block per file.
- **No assertion may be deleted, weakened, or merged.** Every task mechanically proves that the set of pre-existing assertion labels is preserved (`comm -23 before after` must print nothing).
- **The suite must stay green.** `NOT OK - ` count must be 0 in every task's verification run.
- **The URL rewrite is the correctness core, not a detail.** A copied fixture whose `remote.origin.url` still points at the template silently couples fixtures instead of failing loudly.
- Baseline measured 2026-07-31 on this branch's merge base: `test_docket_config` 111s / 396 ok, `test_docket_status` 35s / 405 ok, `test_board_checks` 19s / 177 ok, `test_closeout` 17s / 133 ok. Total 182s, 1111 assertions, 0 failures.
- Shell portability: `command grep` (never bare `grep`, which is ugrep on the dev machine), `LC_ALL=C sort`, no piping a producer into an early-exiting consumer under `pipefail`.

## Settled design decisions (do not re-litigate)

The spec left four open questions. All four were resolved **empirically** during planning; each task's code already reflects the answer. Recorded here so an implementer does not re-derive them:

1. **No assertion depends on fixtures having distinct baseline SHAs.** Every `rev-parse` comparison in the four files is a before/after pair *within one fixture* (e.g. `test_docket_config.sh:262-267`, `test_closeout.sh:186-192`). None compares two fixtures. Template reuse is therefore safe. Each task re-verifies via the label-preservation guard.
2. **`test_board_checks.sh`'s commit ageing is unaffected.** It ages commits by setting `GIT_AUTHOR_DATE`/`GIT_COMMITTER_DATE` explicitly on commits the *tests* make (`tests/test_board_checks.sh:261,265`), and `scripts/board-checks.sh` reads only those (`log -1 --format=%ct`). The baseline commit's own date is never an input to a staleness assertion, and `NOW_EPOCH=1750000000` sits in the past relative to any real build clock either way.
3. **`remote set-head origin -a` does NOT need re-running after a copy.** Verified by direct experiment: `refs/remotes/origin/HEAD` is a local symref (`refs/remotes/origin/main`) that survives `cp -R` and does not depend on the remote URL. `git -C <copy> rev-parse --abbrev-ref origin/HEAD` returns `origin/main` after copy + `set-url`. The copy path is therefore **two commands, not three** — and Task 1 pins this with an assertion so it cannot silently regress.
4. **The four helpers stay four independent bodies. Do NOT extract a shared test library.** The shared part is three lines (`cp -R`, `cp -R`, `set-url`); the variance is the whole rest of each body — three different directory layouts, and completely different seeded content (one plain `main`; `main` + orphan `docket` with plans/results; `main` + orphan `docket` with change files, a spec, and two ADRs; a `seed` + bare clone with no work clone at all). A shared helper would have to be parameterized over all of that, which is the flattening hazard in `learnings/consolidation-flattens-caller-variance.md`. There is also no `tests/lib/` in this repo today (71 flat test files), so introducing one is its own design decision, not a side effect of a performance fix.

## Explicitly NOT a target

`tests/test_closeout.sh`'s **`d1_fixture`** (6 call sites, `test_closeout.sh:559`) is a fifth fixture builder in an in-scope file, and it is deliberately left alone: it calls `git worktree add`, and a worktree's `.git/worktrees/<name>/gitdir` records **absolute paths**, so a `cp -R` copy would produce a repo whose worktree administration points back at the template. It is out of scope for a correctness reason, not an oversight.

## File Structure

No files are created or deleted. Four files are modified, each in exactly two places (helper body; new independence block):

| File | Helper | Call sites | Baseline |
|---|---|---|---|
| `tests/test_docket_config.sh` | `mkrepo` | 121 | 111s |
| `tests/test_docket_status.sh` | `git_repo_setup` | 29 | 35s |
| `tests/test_board_checks.sh` | `new_repo` | 34 | 19s |
| `tests/test_closeout.sh` | `new_repo` | 29 | 17s |

(Call-site counts are exact call counts; the spec's 122/30/37/31 counted string occurrences including each definition and its comment.)

---

### Task 1: `tests/test_docket_config.sh` — template the `mkrepo` helper

The largest single win (111s of 182s) and the canonical two-directory shape: `mkrepo <dir>` builds a clone at `<dir>` whose bare origin is the **sibling** path `<dir>.origin.git`.

**Files:**
- Modify: `tests/test_docket_config.sh:13-25` (the `mkrepo` body)
- Modify: `tests/test_docket_config.sh` (new independence block, inserted after the `tmp=`/`trap` line at line 29 and before the first `mkrepo` call)

**Interfaces:**
- Consumes: nothing from other tasks.
- Produces: the pattern the remaining three tasks follow — a lazily-initialized `<HELPER>_TEMPLATE` global, a `_<helper>_build_template` function holding the *unchanged* original seeding body, and a rewritten public helper that copies + rewrites the URL. Later tasks reuse the shape, not the code.

- [ ] **Step 1: Capture the pre-existing assertion labels (the file is still pristine — do this first)**

```bash
cd /Users/homer/dev/docket/.worktrees/reuse-test-git-fixtures
bash tests/test_docket_config.sh > /tmp/0174-cfg-before.raw 2>&1
command grep -E '^(ok|NOT OK) - ' /tmp/0174-cfg-before.raw \
  | sed -E 's/^(ok|NOT OK) - //' | LC_ALL=C sort > /tmp/0174-cfg-before.txt
wc -l < /tmp/0174-cfg-before.txt
command grep -c '^NOT OK - ' /tmp/0174-cfg-before.raw
```

Expected: `396` labels, `0` failures. If the count differs from 396, STOP — the baseline moved and the plan's numbers need re-deriving before any edit.

- [ ] **Step 2: Write the failing independence test**

Insert this block immediately **after** line 29 (`tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT`) and before the `fake_bash` definition. It must come after `$tmp` exists, because the template lives under `$tmp`.

```bash
# --- fixture independence (change 0174) --------------------------------------
# mkrepo now copies a once-built template. These assertions pin the property that
# makes that safe: fixtures are independent of each other AND of the template.
# The "did advance" assert is anti-vacuity — without it, a silently failed push
# would make every "unchanged" assertion below pass for the wrong reason.
mkrepo "$tmp/indep-a"
mkrepo "$tmp/indep-b"
indep_tpl_before="$(git -C "$MKREPO_TEMPLATE.origin.git" rev-parse refs/heads/main)"
indep_b_before="$(git -C "$tmp/indep-b.origin.git" rev-parse refs/heads/main)"
echo mutated > "$tmp/indep-a/MUTATION.md"
git -C "$tmp/indep-a" add MUTATION.md
git -C "$tmp/indep-a" commit --quiet -m mutate
git -C "$tmp/indep-a" push --quiet origin main
assert "0174 independence: the mutated fixture's own origin DID advance (mutation was real)" \
  '[ "$(git -C "$tmp/indep-a.origin.git" rev-parse refs/heads/main)" != "$indep_tpl_before" ]'
assert "0174 independence: a sibling fixture's origin is untouched" \
  '[ "$(git -C "$tmp/indep-b.origin.git" rev-parse refs/heads/main)" = "$indep_b_before" ]'
assert "0174 independence: the template's origin is untouched" \
  '[ "$(git -C "$MKREPO_TEMPLATE.origin.git" rev-parse refs/heads/main)" = "$indep_tpl_before" ]'
assert "0174 independence: a sibling worktree never sees the mutation" \
  '[ ! -e "$tmp/indep-b/MUTATION.md" ]'
assert "0174 independence: the template worktree never sees the mutation" \
  '[ ! -e "$MKREPO_TEMPLATE/MUTATION.md" ]'
assert "0174 independence: each fixture points at its OWN origin" \
  '[ "$(git -C "$tmp/indep-a" config remote.origin.url)" = "$tmp/indep-a.origin.git" ]'
assert "0174 fixture parity: origin/HEAD still resolves after the copy" \
  '[ "$(git -C "$tmp/indep-b" rev-parse --abbrev-ref origin/HEAD)" = "origin/main" ]'
```

- [ ] **Step 3: Run it to verify it fails**

Run: `cd /Users/homer/dev/docket/.worktrees/reuse-test-git-fixtures && bash tests/test_docket_config.sh 2>&1 | command grep -E '^(ok|NOT OK) - 0174'`

Expected: FAIL. Against the current (non-templated) `mkrepo`, `$MKREPO_TEMPLATE` does not exist, so under `set -u` the block aborts the run — no `0174` lines appear at all and the file exits early. That absence **is** the red state; do not "fix" it by defining the variable without templating the helper.

- [ ] **Step 4: Replace the `mkrepo` body**

Replace lines 13-25 entirely. The seeding body moves verbatim into `_mkrepo_build_template` — do not retype or "improve" it.

```bash
# MKREPO_TEMPLATE: the baseline every mkrepo fixture is copied from; built on first use.
MKREPO_TEMPLATE=""
_mkrepo_build_template(){
  MKREPO_TEMPLATE="$tmp/.mkrepo-template"
  local dir="$MKREPO_TEMPLATE" bare="$MKREPO_TEMPLATE.origin.git"
  git init --quiet --bare "$bare"
  git clone --quiet "$bare" "$dir" 2>/dev/null
  git -C "$dir" config user.email t@t.test
  git -C "$dir" config user.name  Test
  git -C "$dir" checkout --quiet -b main
  : > "$dir/README.md"
  git -C "$dir" add README.md
  git -C "$dir" commit --quiet -m init
  git -C "$dir" push --quiet -u origin main
  git -C "$dir" remote set-head origin -a >/dev/null 2>&1
}
mkrepo(){
  local dir="$1" bare="$1.origin.git"
  [ -n "$MKREPO_TEMPLATE" ] || _mkrepo_build_template
  mkdir -p "$(dirname "$dir")"
  rm -rf "$dir" "$bare"
  cp -R "$MKREPO_TEMPLATE" "$dir"
  cp -R "$MKREPO_TEMPLATE.origin.git" "$bare"
  git -C "$dir" remote set-url origin "$bare"
}
```

Notes for the implementer, each load-bearing:
- `rm -rf "$dir" "$bare"` first: `cp -R src dst` copies *into* `dst` when `dst` already exists, which would silently nest a repo instead of replacing it. This also self-heals a re-call on the same path.
- The template is `$tmp/.mkrepo-template`, dot-prefixed and under the file's own `trap`-cleaned `$tmp`, so it needs no new cleanup path and cannot collide with a caller's `$tmp/<name>`.
- No `remote set-head` in the copy path — see settled decision 3.

- [ ] **Step 5: Run the file and verify green**

```bash
cd /Users/homer/dev/docket/.worktrees/reuse-test-git-fixtures
time bash tests/test_docket_config.sh > /tmp/0174-cfg-after.raw 2>&1
command grep -c '^NOT OK - ' /tmp/0174-cfg-after.raw
command grep -E '^(ok|NOT OK) - ' /tmp/0174-cfg-after.raw \
  | sed -E 's/^(ok|NOT OK) - //' | LC_ALL=C sort > /tmp/0174-cfg-after.txt
```

Expected: `0` failures, and a wall clock far below the 111s baseline.

- [ ] **Step 6: Prove no pre-existing assertion was lost**

```bash
comm -23 /tmp/0174-cfg-before.txt /tmp/0174-cfg-after.txt
```

Expected: **no output.** Any line printed is an assertion that existed before and does not now — a Global Constraint violation. Fix it rather than adjusting the guard.

- [ ] **Step 7: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/reuse-test-git-fixtures
git add tests/test_docket_config.sh
git commit -m "perf(tests): build the docket-config git fixture once and copy it

mkrepo rebuilt a byte-identical repo 121 times (~8-10 git processes each).
Build it once into a template, then copy + repoint remote.origin.url per call.
Signature unchanged, so all 121 call sites are untouched.

Adds an explicit fixture-independence block: a mutation to one fixture must not
reach a sibling or the template, and origin/HEAD must still resolve after the copy."
```

---

### Task 2: `tests/test_docket_status.sh` — template the `git_repo_setup` helper

The shape that differs most: `git_repo_setup <root>` produces **no work clone**. It builds `$root/seed` and bare-clones it to `$root/origin.git`; callers clone their own `work` from `origin.git` afterwards. The URL rewrite target is therefore the **bare** repo, whose `remote.origin.url` records an absolute path back to the template's `seed` (verified directly — `git clone --bare` does set it).

**Files:**
- Modify: `tests/test_docket_status.sh:211-217` (the `git_repo_setup` body)
- Modify: `tests/test_docket_status.sh` (new independence block, immediately after the rewritten helper)

**Interfaces:**
- Consumes: the template pattern established in Task 1 (lazily-built global + private builder + copying public helper). No code dependency.
- Produces: nothing later tasks consume.

- [ ] **Step 1: Capture the pre-existing assertion labels**

```bash
cd /Users/homer/dev/docket/.worktrees/reuse-test-git-fixtures
bash tests/test_docket_status.sh > /tmp/0174-status-before.raw 2>&1
command grep -E '^(ok|NOT OK) - ' /tmp/0174-status-before.raw \
  | sed -E 's/^(ok|NOT OK) - //' | LC_ALL=C sort > /tmp/0174-status-before.txt
wc -l < /tmp/0174-status-before.txt
command grep -c '^NOT OK - ' /tmp/0174-status-before.raw
```

Expected: `405` labels, `0` failures.

- [ ] **Step 2: Write the failing independence test**

Insert immediately after the rewritten helper (i.e. after what is currently line 217, before `write_sync_fixture`). This fixture has no work clone, so independence is proven through clones the block makes itself.

```bash
# --- fixture independence (change 0174) --------------------------------------
# git_repo_setup now copies a once-built template. A fixture's bare origin must be
# its own: pushing into one must not reach a sibling's origin or the template's.
git_repo_setup "$tmp/indep-a"
git_repo_setup "$tmp/indep-b"
git clone -q "$tmp/indep-a/origin.git" "$tmp/indep-a/work" 2>/dev/null
indep_tpl_before="$(git -C "$GIT_REPO_TEMPLATE/origin.git" rev-parse refs/heads/main)"
indep_b_before="$(git -C "$tmp/indep-b/origin.git" rev-parse refs/heads/main)"
git -C "$tmp/indep-a/work" -c user.email=t@t -c user.name=t commit -q --allow-empty -m mutate
git -C "$tmp/indep-a/work" push -q origin main
assert "0174 independence: the mutated fixture's own origin DID advance (mutation was real)" \
  '[ "$(git -C "$tmp/indep-a/origin.git" rev-parse refs/heads/main)" != "$indep_tpl_before" ]'
assert "0174 independence: a sibling fixture's origin is untouched" \
  '[ "$(git -C "$tmp/indep-b/origin.git" rev-parse refs/heads/main)" = "$indep_b_before" ]'
assert "0174 independence: the template's origin is untouched" \
  '[ "$(git -C "$GIT_REPO_TEMPLATE/origin.git" rev-parse refs/heads/main)" = "$indep_tpl_before" ]'
assert "0174 independence: a copied bare origin points at its OWN seed" \
  '[ "$(git -C "$tmp/indep-b/origin.git" config remote.origin.url)" = "$tmp/indep-b/seed" ]'
assert "0174 fixture parity: the copied origin still carries both branches" \
  '[ -n "$(git -C "$tmp/indep-b/origin.git" rev-parse --verify -q refs/heads/docket)" ]'
```

- [ ] **Step 3: Run it to verify it fails**

Run: `cd /Users/homer/dev/docket/.worktrees/reuse-test-git-fixtures && bash tests/test_docket_status.sh 2>&1 | command grep -E '^(ok|NOT OK) - 0174'`

Expected: FAIL — `$GIT_REPO_TEMPLATE` is undefined against the current helper, so under `set -u` the block aborts and no `0174` lines are emitted.

- [ ] **Step 4: Replace the `git_repo_setup` body**

```bash
# GIT_REPO_TEMPLATE: the baseline every git_repo_setup fixture is copied from.
GIT_REPO_TEMPLATE=""
_git_repo_build_template(){
  GIT_REPO_TEMPLATE="$tmp/.git-repo-template"
  mkdir -p "$GIT_REPO_TEMPLATE"
  git init -q -b main "$GIT_REPO_TEMPLATE/seed" \
    && git -C "$GIT_REPO_TEMPLATE/seed" -c user.email=t@t -c user.name=t commit -q --allow-empty -m init \
    && git -C "$GIT_REPO_TEMPLATE/seed" -c user.email=t@t -c user.name=t branch docket \
    && git clone -q --bare "$GIT_REPO_TEMPLATE/seed" "$GIT_REPO_TEMPLATE/origin.git"
}
git_repo_setup(){
  local root="$1"
  [ -n "$GIT_REPO_TEMPLATE" ] || _git_repo_build_template || return 1
  mkdir -p "$root"
  rm -rf "$root/seed" "$root/origin.git"
  cp -R "$GIT_REPO_TEMPLATE/seed" "$root/seed" \
    && cp -R "$GIT_REPO_TEMPLATE/origin.git" "$root/origin.git" \
    && git -C "$root/origin.git" config remote.origin.url "$root/seed"
}
```

The `&&` chain is preserved deliberately: the original helper returned the status of its chain, and callers sit under `set -uo pipefail`. The final `config` call is what makes the copied bare origin stop referring to the template's `seed`.

- [ ] **Step 5: Run the file and verify green**

```bash
cd /Users/homer/dev/docket/.worktrees/reuse-test-git-fixtures
time bash tests/test_docket_status.sh > /tmp/0174-status-after.raw 2>&1
command grep -c '^NOT OK - ' /tmp/0174-status-after.raw
command grep -E '^(ok|NOT OK) - ' /tmp/0174-status-after.raw \
  | sed -E 's/^(ok|NOT OK) - //' | LC_ALL=C sort > /tmp/0174-status-after.txt
```

Expected: `0` failures, wall clock below the 35s baseline.

- [ ] **Step 6: Prove no pre-existing assertion was lost**

```bash
comm -23 /tmp/0174-status-before.txt /tmp/0174-status-after.txt
```

Expected: no output.

- [ ] **Step 7: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/reuse-test-git-fixtures
git add tests/test_docket_status.sh
git commit -m "perf(tests): build the docket-status git fixture once and copy it

git_repo_setup built seed + a bare clone 29 times. Build once, copy per call,
and repoint the copied BARE repo's remote.origin.url at its own seed -- git
clone --bare records an absolute path back to the template otherwise.

Signature and && return contract unchanged; all 29 call sites untouched."
```

---

### Task 3: `tests/test_board_checks.sh` — template the `new_repo` helper

`new_repo` takes no argument, allocates its own root, and **prints** `"<work> <origin>"`. Callers consume it as `read -r W O < <(new_repo)`. Both the print contract and the two-branch seeding (`main` with plans/results, orphan `docket` with the metadata skeleton) must survive untouched.

**Files:**
- Modify: `tests/test_board_checks.sh:51-73` (the `new_repo` body)
- Modify: `tests/test_board_checks.sh` (new independence block, immediately after the rewritten helper)

**Interfaces:**
- Consumes: the template pattern from Task 1.
- Produces: the exact body Task 4 mirrors for the twin `new_repo` in `test_closeout.sh` — same technique, different seeded content.

- [ ] **Step 1: Capture the pre-existing assertion labels**

```bash
cd /Users/homer/dev/docket/.worktrees/reuse-test-git-fixtures
bash tests/test_board_checks.sh > /tmp/0174-bc-before.raw 2>&1
command grep -E '^(ok|NOT OK) - ' /tmp/0174-bc-before.raw \
  | sed -E 's/^(ok|NOT OK) - //' | LC_ALL=C sort > /tmp/0174-bc-before.txt
wc -l < /tmp/0174-bc-before.txt
command grep -c '^NOT OK - ' /tmp/0174-bc-before.raw
```

Expected: `177` labels, `0` failures.

- [ ] **Step 2: Write the failing independence test**

Insert immediately after the rewritten helper, before the `assert "script exists and is executable"` line. The mutation targets the `docket` branch because that is the branch the template leaves checked out.

```bash
# --- fixture independence (change 0174) --------------------------------------
# new_repo now copies a once-built template. Fixtures must not share an origin.
read -r indep_a_w indep_a_o < <(new_repo)
read -r indep_b_w indep_b_o < <(new_repo)
indep_tpl_before="$(git -C "$NEW_REPO_TEMPLATE/tpl/origin.git" rev-parse refs/heads/docket)"
indep_b_before="$(git -C "$indep_b_o" rev-parse refs/heads/docket)"
echo mutated > "$indep_a_w/MUTATION.md"
git -C "$indep_a_w" add MUTATION.md
git_quiet -C "$indep_a_w" commit -m mutate
git_quiet -C "$indep_a_w" push origin docket
assert "0174 independence: the mutated fixture's own origin DID advance (mutation was real)" \
  '[ "$(git -C "$indep_a_o" rev-parse refs/heads/docket)" != "$indep_tpl_before" ]'
assert "0174 independence: a sibling fixture's origin is untouched" \
  '[ "$(git -C "$indep_b_o" rev-parse refs/heads/docket)" = "$indep_b_before" ]'
assert "0174 independence: the template's origin is untouched" \
  '[ "$(git -C "$NEW_REPO_TEMPLATE/tpl/origin.git" rev-parse refs/heads/docket)" = "$indep_tpl_before" ]'
assert "0174 independence: a sibling worktree never sees the mutation" \
  '[ ! -e "$indep_b_w/MUTATION.md" ]'
assert "0174 independence: each fixture points at its OWN origin" \
  '[ "$(git -C "$indep_a_w" config remote.origin.url)" = "$indep_a_o" ]'
assert "0174 fixture parity: the copy is still parked on docket with main present" \
  '[ "$(git -C "$indep_b_w" rev-parse --abbrev-ref HEAD)" = docket ] && [ -n "$(git -C "$indep_b_o" rev-parse --verify -q refs/heads/main)" ]'
```

- [ ] **Step 3: Run it to verify it fails**

Run: `cd /Users/homer/dev/docket/.worktrees/reuse-test-git-fixtures && bash tests/test_board_checks.sh 2>&1 | command grep -E '^(ok|NOT OK) - 0174'`

Expected: FAIL — `$NEW_REPO_TEMPLATE` is undefined against the current helper.

- [ ] **Step 4: Replace the `new_repo` body**

The seeding body moves verbatim into the builder; only the variable names it writes into change (`$work`/`$origin` become template paths).

```bash
# NEW_REPO_TEMPLATE: root holding the once-built baseline (tpl/) plus every copied
# fixture (f1, f2, ...). One mktemp -d per file instead of one per call.
NEW_REPO_TEMPLATE=""
NEW_REPO_N=0
_new_repo_build_template(){
  NEW_REPO_TEMPLATE="$(mktemp -d)"
  local work="$NEW_REPO_TEMPLATE/tpl/work" origin="$NEW_REPO_TEMPLATE/tpl/origin.git"
  mkdir -p "$NEW_REPO_TEMPLATE/tpl"
  git_quiet init --bare "$origin"
  git_quiet clone "$origin" "$work"
  git -C "$work" config user.email t@t; git -C "$work" config user.name t
  # --- main branch: build artifacts that 'done' changes link to ---
  git -C "$work" checkout -b main >/dev/null 2>&1
  mkdir -p "$work/docs/superpowers/plans" "$work/docs/results"
  echo "# plan"    > "$work/docs/superpowers/plans/2026-06-01-present.md"
  echo "# results" > "$work/docs/results/2026-06-01-present-results.md"
  git -C "$work" add -A; git_quiet -C "$work" commit -m "main artifacts"
  git_quiet -C "$work" push -u origin main
  # --- docket branch: orphan metadata ---
  git -C "$work" checkout --orphan docket >/dev/null 2>&1
  git -C "$work" rm -rf . >/dev/null 2>&1 || true
  mkdir -p "$work/docs/changes/active" "$work/docs/changes/archive" "$work/docs/superpowers/specs"
  echo "# present spec" > "$work/docs/superpowers/specs/2026-06-01-present.md"
  git -C "$work" add -A; git_quiet -C "$work" commit -m "docket metadata baseline"
  git_quiet -C "$work" push -u origin docket
  # leave the template parked on docket (the metadata working tree)
}
new_repo(){
  local root work origin
  [ -n "$NEW_REPO_TEMPLATE" ] || _new_repo_build_template
  NEW_REPO_N=$((NEW_REPO_N + 1))
  root="$NEW_REPO_TEMPLATE/f$NEW_REPO_N"; origin="$root/origin.git"; work="$root/work"
  mkdir -p "$root"
  cp -R "$NEW_REPO_TEMPLATE/tpl/origin.git" "$origin"
  cp -R "$NEW_REPO_TEMPLATE/tpl/work" "$work"
  git -C "$work" remote set-url origin "$origin"
  printf '%s %s\n' "$work" "$origin"
}
```

Two notes:
- Per-call roots become numbered subdirectories of one template root instead of a fresh `mktemp -d` each. This file has **no `trap` at all**, so today its 34 fixture roots leak into the temp directory; after this change one root leaks instead of 34. No `trap` is added — see Task 4's note on why adding one here would be a hazard in the twin file.
- The `printf` return contract and the "parked on docket" end state are unchanged.

- [ ] **Step 5: Run the file and verify green**

```bash
cd /Users/homer/dev/docket/.worktrees/reuse-test-git-fixtures
time bash tests/test_board_checks.sh > /tmp/0174-bc-after.raw 2>&1
command grep -c '^NOT OK - ' /tmp/0174-bc-after.raw
command grep -E '^(ok|NOT OK) - ' /tmp/0174-bc-after.raw \
  | sed -E 's/^(ok|NOT OK) - //' | LC_ALL=C sort > /tmp/0174-bc-after.txt
```

Expected: `0` failures, wall clock below the 19s baseline.

- [ ] **Step 6: Prove no pre-existing assertion was lost**

```bash
comm -23 /tmp/0174-bc-before.txt /tmp/0174-bc-after.txt
```

Expected: no output. Pay particular attention here: this is the file whose staleness assertions depend on commit dates (settled decision 2), so a lost or reordered label matters more than elsewhere.

- [ ] **Step 7: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/reuse-test-git-fixtures
git add tests/test_board_checks.sh
git commit -m "perf(tests): build the board-checks git fixture once and copy it

new_repo rebuilt a two-branch repo (main artifacts + orphan docket) 34 times.
Build once, copy per call, repoint remote.origin.url. The print contract
'<work> <origin>' and the parked-on-docket end state are unchanged.

Per-call roots are now numbered subdirs of one template root, so this
trap-less file leaks one temp dir instead of 34."
```

---

### Task 4: `tests/test_closeout.sh` — template the twin `new_repo` helper

Same technique as Task 3 on the twin helper, with different seeded content: a plain `main` baseline plus an orphan `docket` carrying `0007-sample.md`, its spec, and two ADRs (one Accepted, one Proposed).

**Files:**
- Modify: `tests/test_closeout.sh:78-139` (the `new_repo` body)
- Modify: `tests/test_closeout.sh` (new independence block, immediately after the rewritten helper)

**Interfaces:**
- Consumes: the technique proven in Task 3. Nothing is imported — the bodies stay independent by design (settled decision 4).
- Produces: nothing later tasks consume.

- [ ] **Step 1: Capture the pre-existing assertion labels**

```bash
cd /Users/homer/dev/docket/.worktrees/reuse-test-git-fixtures
bash tests/test_closeout.sh > /tmp/0174-co-before.raw 2>&1
command grep -E '^(ok|NOT OK) - ' /tmp/0174-co-before.raw \
  | sed -E 's/^(ok|NOT OK) - //' | LC_ALL=C sort > /tmp/0174-co-before.txt
wc -l < /tmp/0174-co-before.txt
command grep -c '^NOT OK - ' /tmp/0174-co-before.raw
```

Expected: `133` labels, `0` failures.

- [ ] **Step 2: Write the failing independence test**

Insert immediately after the rewritten helper, before `assert "archive-change.sh exists and is executable"`.

```bash
# --- fixture independence (change 0174) --------------------------------------
# new_repo now copies a once-built template. Close-out tests push to origin and
# compare origin refs, so a shared origin would corrupt them silently.
read -r indep_a_w indep_a_o < <(new_repo)
read -r indep_b_w indep_b_o < <(new_repo)
indep_tpl_before="$(git -C "$NEW_REPO_TEMPLATE/tpl/origin.git" rev-parse refs/heads/docket)"
indep_b_before="$(git -C "$indep_b_o" rev-parse refs/heads/docket)"
echo mutated > "$indep_a_w/MUTATION.md"
git -C "$indep_a_w" add MUTATION.md
git_quiet -C "$indep_a_w" commit -m mutate
git_quiet -C "$indep_a_w" push origin docket
assert "0174 independence: the mutated fixture's own origin DID advance (mutation was real)" \
  '[ "$(git -C "$indep_a_o" rev-parse refs/heads/docket)" != "$indep_tpl_before" ]'
assert "0174 independence: a sibling fixture's origin is untouched" \
  '[ "$(git -C "$indep_b_o" rev-parse refs/heads/docket)" = "$indep_b_before" ]'
assert "0174 independence: the template's origin is untouched" \
  '[ "$(git -C "$NEW_REPO_TEMPLATE/tpl/origin.git" rev-parse refs/heads/docket)" = "$indep_tpl_before" ]'
assert "0174 independence: a sibling worktree never sees the mutation" \
  '[ ! -e "$indep_b_w/MUTATION.md" ]'
assert "0174 independence: each fixture points at its OWN origin" \
  '[ "$(git -C "$indep_a_w" config remote.origin.url)" = "$indep_a_o" ]'
assert "0174 fixture parity: the copy still carries the seeded change and both ADRs" \
  '[ -f "$indep_b_w/docs/changes/active/0007-sample.md" ] && [ -f "$indep_b_w/docs/adrs/0003-accepted.md" ] && [ -f "$indep_b_w/docs/adrs/0005-proposed.md" ]'
```

- [ ] **Step 3: Run it to verify it fails**

Run: `cd /Users/homer/dev/docket/.worktrees/reuse-test-git-fixtures && bash tests/test_closeout.sh 2>&1 | command grep -E '^(ok|NOT OK) - 0174'`

Expected: FAIL — `$NEW_REPO_TEMPLATE` is undefined against the current helper.

- [ ] **Step 4: Replace the `new_repo` body**

Move the seeding body verbatim — including both here-docs, unchanged — into the builder.

```bash
# NEW_REPO_TEMPLATE: root holding the once-built baseline (tpl/) plus every copied
# fixture (f1, f2, ...). One mktemp -d per file instead of one per call.
NEW_REPO_TEMPLATE=""
NEW_REPO_N=0
_new_repo_build_template(){
  NEW_REPO_TEMPLATE="$(mktemp -d)"
  local work="$NEW_REPO_TEMPLATE/tpl/work" origin="$NEW_REPO_TEMPLATE/tpl/origin.git"
  mkdir -p "$NEW_REPO_TEMPLATE/tpl"
  git_quiet init --bare "$origin"
  git_quiet clone "$origin" "$work"
  git -C "$work" config user.email t@t; git -C "$work" config user.name t
  # --- main branch (baseline) ---
  git -C "$work" checkout -b main >/dev/null 2>&1
  echo "code" > "$work/README.md"; git -C "$work" add README.md
  git_quiet -C "$work" commit -m "main baseline"
  git_quiet -C "$work" push -u origin main
  # --- docket branch (orphan metadata) ---
  git -C "$work" checkout --orphan docket >/dev/null 2>&1
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
  # leave the template parked on docket (the metadata working tree)
}
new_repo(){
  local root work origin
  [ -n "$NEW_REPO_TEMPLATE" ] || _new_repo_build_template
  NEW_REPO_N=$((NEW_REPO_N + 1))
  root="$NEW_REPO_TEMPLATE/f$NEW_REPO_N"; origin="$root/origin.git"; work="$root/work"
  mkdir -p "$root"
  cp -R "$NEW_REPO_TEMPLATE/tpl/origin.git" "$origin"
  cp -R "$NEW_REPO_TEMPLATE/tpl/work" "$work"
  git -C "$work" remote set-url origin "$origin"
  printf '%s %s\n' "$work" "$origin"
}
```

**Do not add a cleanup `trap` for `$NEW_REPO_TEMPLATE` in this file.** `test_closeout.sh` already installs `trap 'rm -rf "$tmp"' EXIT` at line 554 — a *later* `trap` for the same signal **replaces** an earlier one rather than adding to it, so a trap registered up here would be silently discarded at line 554, and a trap registered there would need editing an unrelated line. Leaking one template root matches what this file already does with its 29 per-call roots.

Leave `d1_fixture` (line 559) completely untouched — it builds `git worktree` fixtures whose administrative files record absolute paths and which therefore cannot be copied.

- [ ] **Step 5: Run the file and verify green**

```bash
cd /Users/homer/dev/docket/.worktrees/reuse-test-git-fixtures
time bash tests/test_closeout.sh > /tmp/0174-co-after.raw 2>&1
command grep -c '^NOT OK - ' /tmp/0174-co-after.raw
command grep -E '^(ok|NOT OK) - ' /tmp/0174-co-after.raw \
  | sed -E 's/^(ok|NOT OK) - //' | LC_ALL=C sort > /tmp/0174-co-after.txt
```

Expected: `0` failures, wall clock below the 17s baseline.

- [ ] **Step 6: Prove no pre-existing assertion was lost**

```bash
comm -23 /tmp/0174-co-before.txt /tmp/0174-co-after.txt
```

Expected: no output.

- [ ] **Step 7: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/reuse-test-git-fixtures
git add tests/test_closeout.sh
git commit -m "perf(tests): build the closeout git fixture once and copy it

The twin of the board-checks new_repo: same technique, different seeded
content (a sample change, its spec, and two ADRs on an orphan docket branch).
Build once, copy per call, repoint remote.origin.url; print contract unchanged.

d1_fixture is deliberately left alone -- it builds git worktrees, whose
administrative files record absolute paths and do not survive a copy."
```

---

### Task 5: Whole-suite verification and measurement

The four tasks each proved their own file. This task proves the *suite* — the property no single-file run can establish, since fixture coupling and cross-file interference are exactly what a per-file green run cannot see.

**Files:**
- No production files. Produces the measurements the results doc reports.

**Interfaces:**
- Consumes: all four rewritten helpers.
- Produces: before/after wall clock and assertion totals.

- [ ] **Step 1: Re-measure the four changed files**

```bash
cd /Users/homer/dev/docket/.worktrees/reuse-test-git-fixtures
for f in test_docket_config test_docket_status test_board_checks test_closeout; do
  s=$(date +%s); out=$(bash "tests/$f.sh" 2>&1); e=$(date +%s)
  ok=$(printf '%s\n' "$out" | command grep -c '^ok - ')
  notok=$(printf '%s\n' "$out" | command grep -c '^NOT OK - ')
  echo "$f: $((e-s))s ok=$ok notok=$notok"
done
```

Expected: every `notok=0`; total wall clock far below the 182s baseline; each `ok` count equal to its baseline (396/405/177/133) **plus** that file's new independence assertions (7/5/6/6).

- [ ] **Step 2: Run the full suite**

Run this as ONE foreground command — the suite takes roughly 6–10 minutes and must not be backgrounded.

```bash
cd /Users/homer/dev/docket/.worktrees/reuse-test-git-fixtures
for f in tests/*.sh; do
  out=$(bash "$f" 2>&1)
  n=$(printf '%s\n' "$out" | command grep -c '^NOT OK - ')
  [ "$n" -eq 0 ] || { echo "FAILED: $f ($n)"; printf '%s\n' "$out" | command grep '^NOT OK - '; }
done; echo "SUITE SWEEP COMPLETE"
```

Expected: no `FAILED:` lines, then `SUITE SWEEP COMPLETE`. The trailing marker matters — without it, a loop that iterated zero times is indistinguishable from a clean run.

- [ ] **Step 3: Prove the sweep actually ran**

```bash
cd /Users/homer/dev/docket/.worktrees/reuse-test-git-fixtures && ls tests/*.sh | wc -l
```

Expected: `71`. If step 2's loop reported nothing *and* this prints 71, the sweep was real. (Guarding the `agent-shell-noop-reads-as-success` trap: a zero-iteration loop also prints no failures.)

- [ ] **Step 4: Confirm no call site or production file was touched**

```bash
cd /Users/homer/dev/docket/.worktrees/reuse-test-git-fixtures
git diff --stat origin/main...HEAD
git diff origin/main...HEAD -- . ':(exclude)tests' ':(exclude)docs' | head
```

Expected: exactly four files under `tests/` plus the plan under `docs/`, and **empty** output from the second command — no `scripts/`, `skills/`, or root-level file changed. This change is test-only by construction.

- [ ] **Step 5: Commit any measurement artifact**

Nothing to commit if steps 1–4 are clean; the numbers are carried into the results document by the implementing skill. If step 1 or 2 surfaced a failure, stop and report rather than adjusting an assertion.

---

## Self-Review

**Spec coverage.** Every spec section maps to a task: the four in-scope helpers are Tasks 1–4 (the spec's scope table, one task per row); the independence invariant gets an explicit block in each of the four, as the spec demands, rather than relying on suite-green; the "assertion count unchanged" verification is the `comm -23` label guard in every task's Step 6, which is strictly stronger than a count (it detects a renamed or swapped assertion that a count would miss); the before/after wall-clock measurement is Task 5 Step 1, using the same method as the spec's table. All four of the spec's open questions are resolved in *Settled design decisions* with the evidence that settled them, and questions 3 and 4 additionally became assertions or explicit non-actions rather than prose claims.

**Placeholder scan.** No TBD/TODO items; every code step carries the literal code to write, including both here-docs reproduced in full in Task 4 rather than a "same as Task 3" reference. The three "expected" outcomes that are absences (`comm` output, the second `git diff`, the red states in Step 3) each state what the absence means, so an implementer cannot read a vacuous pass as a real one.

**Type consistency.** Global names are consistent within each file and deliberately parallel across them: `MKREPO_TEMPLATE`/`_mkrepo_build_template` (Task 1), `GIT_REPO_TEMPLATE`/`_git_repo_build_template` (Task 2), and `NEW_REPO_TEMPLATE`/`NEW_REPO_N`/`_new_repo_build_template` in Tasks 3 and 4 — the two `new_repo` files use the same names because they are separate processes that never share scope. Every independence block references only variables its own task defines. The public helpers' signatures are unchanged, which is what keeps all 213 call sites valid.
