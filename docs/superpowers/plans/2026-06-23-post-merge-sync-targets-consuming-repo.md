# Post-merge sync targets the consuming repo Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `sync-integration-branch.sh` fast-forward the **consuming repo's** primary checkout (where a docket merge actually lands) instead of the docket clone the script physically lives in, and make its dirty-tree skip diagnosable.

**Architecture:** Two coordinated edits inside the single script `scripts/sync-integration-branch.sh`; the two skill call sites stay bare and inherit the fix. (1) When `--clone-dir` is unset, default it to the **main worktree of the repo the script was invoked from** (CWD), resolved via `git worktree list --porcelain` (first entry) — correct even when the caller's shell sits in a linked worktree (the sync site runs from `.docket/`). (2) Keep gate 2's clean-tree **condition** unchanged but rewrite its skip **note** to name untracked files as a blocker and state the remedy. Update the contract `.md` to match. The change is locked by two new hermetic cases in the existing test file (the de-facto CI gate per ADR-0014).

**Tech Stack:** POSIX `bash`, `git` (2.31+ for `worktree list --porcelain`; repo runs 2.54.0), the repo's hermetic shell-test convention (temp clone + bare origin, no network, no `gh`).

## Global Constraints

- **Best-effort, FF-only posture is preserved.** Every runtime skip stays a normal `exit 0` with a one-line note; only usage errors (missing `--integration-branch`, unknown flag) exit non-zero (`exit 2`). No new abort paths.
- **Gate *conditions* are unchanged.** Only gate 2's **note text** changes and the default `--clone-dir` **resolution** changes. The triple-gate logic (on integration branch · clean tree · true fast-forward) is untouched.
- **Explicit `--clone-dir` still overrides** the new default. The existing 7 hermetic cases all pass `--clone-dir` explicitly and must stay green unmodified.
- **Portability:** the script and tests run on both macOS (BSD) and Linux (GNU). No GNU-only flags; never lead a bare ERE with `--` (use `grep -E -e` if ever needed); prefer `sed`/`awk` constructs that behave identically on both.
- **Mock seam preserved:** `GIT="${GIT:-git}"`; all git calls go through `"$GIT"`.
- **Dogfooding unchanged:** when docket runs on itself, CWD's main worktree equals the old `dirname "$0"/..`, so the resolved dir is identical.

---

### Task 1: Retarget the default `--clone-dir` to the invoking repo's main worktree

**Files:**
- Modify: `scripts/sync-integration-branch.sh` (header-comment usage line ~17; default-resolution block at lines 40–43)
- Test: `tests/test_sync_integration_branch.sh` (append a new Case 8 after Case 7, before the summary block)

**Interfaces:**
- Consumes: the script's existing arg parsing (`CLONE_DIR` empty when `--clone-dir` is not passed), the `GIT` mock seam, and the existing not-a-repo gate immediately below the resolution block.
- Produces: a resolved `CLONE_DIR` equal to the **main worktree** of `$PWD`'s git worktree set when `--clone-dir` is unset; identical downstream behavior otherwise. No new flags, no signature change.

- [ ] **Step 1: Write the failing test (Case 8 — bare invocation from a linked worktree resolves the MAIN worktree)**

Append to `tests/test_sync_integration_branch.sh`, immediately **before** the final `if [ "$fail" -eq 0 ]; …` summary block:

```bash
# --- Case 8: bare invocation (no --clone-dir) resolves the MAIN worktree of CWD ---
# Retarget fix (change 0041): with no --clone-dir the helper must fast-forward the MAIN worktree of
# the repo it was invoked from — even when the shell sits in a LINKED worktree (the real sync site
# runs from .docket/, a linked worktree on the docket branch). Hermetic: the helper is copied into
# the fixture so the OLD dirname-based default stays sandboxed in the RED phase (its dirname/.. is a
# non-repo temp dir → not-a-repo skip), never the real docket clone.
read -r W O < <(new_repo)
advance_origin "$W"                                   # origin main → C1 (v1); W's local main still C0
root="$(dirname "$W")"                                # $root/work == W (the main worktree)
git_quiet -C "$W" worktree add "$root/linked" -b feat/x   # linked worktree, on feat/x
mkdir -p "$root/bin"; cp "$HELPER" "$root/bin/sync.sh"; chmod +x "$root/bin/sync.sh"
before="$(git -C "$W" rev-parse HEAD)"
out="$(cd "$root/linked" && "$root/bin/sync.sh" --integration-branch main 2>&1)"; rc=$?
after="$(git -C "$W" rev-parse HEAD)"
remote_tip="$(git -C "$W" rev-parse origin/main)"
sentinel="$(cat "$W/skills/sentinel.txt")"
assert "bare-linked: exit 0"                        "[ $rc -eq 0 ]"
assert "bare-linked: MAIN worktree fast-forwarded"  "[ '$after' = '$remote_tip' ]"
assert "bare-linked: main advanced past C0"         "[ '$after' != '$before' ]"
assert "bare-linked: main worktree sentinel = v1"   "[ '$sentinel' = 'v1' ]"
```

- [ ] **Step 2: Run the test to verify Case 8 fails (RED)**

Run: `bash tests/test_sync_integration_branch.sh`
Expected: the four `bare-linked:` assertions print `NOT OK` (the copied helper's old default resolves `dirname/.. = $root`, a non-repo temp dir → not-a-repo skip → W untouched, so `after == before == C0 ≠ remote_tip`). Overall result: `FAILURES`, exit 1. The existing Cases 1–7 still print `ok`.

- [ ] **Step 3: Implement the retarget in `scripts/sync-integration-branch.sh`**

Replace the default-resolution block (currently):

```bash
# --clone-dir defaults to this script's repo root.
if [ -z "$CLONE_DIR" ]; then
  CLONE_DIR="$(cd "$(dirname "$0")/.." && pwd -P)"
fi
```

with:

```bash
# --clone-dir defaults to the MAIN worktree of the repo this script was invoked from (CWD), NOT
# the repo the script physically lives in. git lists the main worktree first and it is reachable
# from any linked worktree in the set, so this resolves the consuming repo's primary checkout even
# when the caller's shell sits in a linked worktree (the sync site runs from the .docket/ metadata
# worktree on the docket branch). `git rev-parse --show-toplevel` would instead return that linked
# worktree (on the docket branch) and gate 1 would skip it — so main-worktree resolution is
# load-bearing. An explicit --clone-dir still overrides. If CWD is not inside a git repo the
# resolution is empty and we fall back to CWD so the not-a-repo gate below emits the standard skip.
if [ -z "$CLONE_DIR" ]; then
  CLONE_DIR="$("$GIT" -C "$PWD" worktree list --porcelain 2>/dev/null | sed -n '1s/^worktree //p')"
  [ -n "$CLONE_DIR" ] || CLONE_DIR="$PWD"
fi
```

Also update the script's header-comment usage line (currently):

```bash
#   --clone-dir defaults to the script's own repo root.  --remote defaults to origin.
```

to:

```bash
#   --clone-dir defaults to the main worktree of the invoking repo (CWD).  --remote defaults to origin.
```

Notes for the implementer:
- `sed -n '1s/^worktree //p'` takes line 1 of the porcelain output and, if it begins with `worktree `, strips that prefix and prints the remainder — **space-safe** for worktree paths containing spaces (the porcelain `worktree ` line preserves the literal path after the prefix). Do not use `awk '{print $2}'`: a path with a space would be truncated.
- Keep the `2>/dev/null` so a non-repo CWD produces empty output (→ the `$PWD` fallback → not-a-repo skip), never a stderr leak.

- [ ] **Step 4: Run the test to verify Case 8 passes (GREEN)**

Run: `bash tests/test_sync_integration_branch.sh`
Expected: all `bare-linked:` assertions print `ok` (the fix resolves `$PWD`'s main worktree = W → FF to C1); Cases 1–7 still `ok`; overall `ALL PASS`, exit 0.

- [ ] **Step 5: Commit**

```bash
git add scripts/sync-integration-branch.sh tests/test_sync_integration_branch.sh
git commit -m "fix(0041): default --clone-dir to the invoking repo's main worktree

sync-integration-branch.sh defaulted --clone-dir to the repo the script lives in
(dirname \$0/..); in a separate-clone install that is the docket clone, never the
consuming repo where the merge landed, so the consuming checkout silently drifted.
Resolve the main worktree of CWD (git worktree list --porcelain, first entry) so a
bare invocation from the linked .docket/ worktree fast-forwards the primary checkout."
```

---

### Task 2: Make gate 2's clean-tree skip note name untracked files as a blocker and state the remedy

**Files:**
- Modify: `scripts/sync-integration-branch.sh` (gate-2 block, currently lines 56–59)
- Test: `tests/test_sync_integration_branch.sh` (append a new Case 9 after Case 8, before the summary block)

**Interfaces:**
- Consumes: the existing `note()` helper (prints `sync-integration-branch: <msg>` to stderr, one line per call) and `CLONE_DIR`.
- Produces: identical control flow (any non-empty `git status --porcelain` → `exit 0`), but a multi-line diagnostic note that (a) states untracked non-ignored files also block the fast-forward and (b) gives the remedy. No condition change.

- [ ] **Step 1: Write the failing test (Case 9 — untracked files block, and the note says so + gives a remedy)**

Append to `tests/test_sync_integration_branch.sh`, immediately **before** the final summary block (after Case 8):

```bash
# --- Case 9: untracked (non-ignored) files block the FF, and the skip note says so + gives a remedy ---
# Gate 2 stays conservative (change 0041): untracked files block the auto-FF exactly like dirty
# tracked edits. What changed is the NOTE — it must name untracked files as a blocker and give the
# remedy, so a consuming repo with stray untracked files (markhaus's design/) gets a diagnosable
# skip instead of a silent drift.
read -r W O < <(new_repo)
advance_origin "$W"                                   # origin ahead (C1): a real drift the untracked file blocks
mkdir -p "$W/design"; echo "stray" > "$W/design/untracked.txt"   # untracked, non-ignored
before="$(git -C "$W" rev-parse HEAD)"
out="$("$HELPER" --clone-dir "$W" --integration-branch main 2>&1)"; rc=$?
after="$(git -C "$W" rev-parse HEAD)"
assert "untracked: exit 0"                               "[ $rc -eq 0 ]"
assert "untracked: tip unchanged (no FF over untracked)" "[ '$after' = '$before' ]"
assert "untracked: note names untracked as a blocker"    "printf '%s' \"\$out\" | grep -qi untracked"
assert "untracked: note gives a remedy"                  "printf '%s' \"\$out\" | grep -qiE 'gitignore|stash|remove|commit'"
```

Why these assertions are non-vacuous: the **current** note is `working tree not clean — skipping (no fast-forward onto local edits)` — it contains neither `untracked` nor any of `gitignore|stash|remove|commit`, so both note assertions are RED until the rewrite. The existing Case 2 (dirty *tracked* tree) asserts `grep -qiE 'clean|dirty|uncommitted'`; the rewritten note keeps the literal `not clean`, so Case 2 stays green.

- [ ] **Step 2: Run the test to verify Case 9 fails (RED)**

Run: `bash tests/test_sync_integration_branch.sh`
Expected: `untracked: note names untracked as a blocker` and `untracked: note gives a remedy` print `NOT OK`; `untracked: exit 0` and `untracked: tip unchanged` print `ok` (the old gate already blocks untracked files — only the note is missing). Overall `FAILURES`, exit 1.

- [ ] **Step 3: Implement the louder note in `scripts/sync-integration-branch.sh`**

Replace the gate-2 block (currently):

```bash
# Gate 2: clean working tree? (any porcelain output — tracked or untracked-non-ignored — blocks)
if [ -n "$("$GIT" -C "$CLONE_DIR" status --porcelain 2>/dev/null)" ]; then
  note "working tree not clean — skipping (no fast-forward onto local edits)"; exit 0
fi
```

with:

```bash
# Gate 2: clean working tree? (any porcelain output — tracked OR untracked-non-ignored — blocks).
# Condition unchanged; the note is explicit so an untracked-only tree is a diagnosable skip, not a
# silent drift (change 0041).
porcelain="$("$GIT" -C "$CLONE_DIR" status --porcelain 2>/dev/null)"
if [ -n "$porcelain" ]; then
  count="$(printf '%s\n' "$porcelain" | wc -l | tr -d ' ')"
  note "working tree not clean — skipping (best-effort; never fast-forwards onto a non-pristine tree)."
  note "  Untracked (non-ignored) files also block the fast-forward, not only tracked edits."
  note "  Remedy: commit or stash tracked changes, and remove or .gitignore untracked paths, then re-run."
  note "  ${count} offending path(s) (git status --porcelain):"
  printf '%s\n' "$porcelain" | head -5 | sed 's/^/    /' >&2
  exit 0
fi
```

Notes for the implementer:
- `note()` writes to stderr; the indented `git status --porcelain` excerpt is also sent to stderr (`>&2`) so the whole diagnostic stays on one stream and the test captures it via `2>&1`.
- `printf '%s\n' "$porcelain" | wc -l` counts entries correctly even when `$porcelain` has no trailing newline (the `printf` adds one); `tr -d ' '` strips BSD `wc`'s leading padding.
- `head -5` caps the excerpt; do not list the whole tree.

- [ ] **Step 4: Run the test to verify Case 9 passes (GREEN)**

Run: `bash tests/test_sync_integration_branch.sh`
Expected: all `untracked:` assertions `ok`; Case 2 (`dirty: note mentions clean/dirty`) still `ok`; all other cases `ok`; overall `ALL PASS`, exit 0.

- [ ] **Step 5: Commit**

```bash
git add scripts/sync-integration-branch.sh tests/test_sync_integration_branch.sh
git commit -m "fix(0041): make gate-2 dirty-tree skip name untracked files + remedy

The clean-tree gate already blocks untracked non-ignored files, but its note only
said 'working tree not clean', surprising a user whose only diff is untracked paths
(markhaus's design/). Gate condition unchanged; the note now states untracked files
also block and gives the remedy (commit/stash tracked edits; remove/.gitignore
untracked paths), so the skip is diagnosable instead of a silent drift."
```

---

### Task 3: Update the contract `scripts/sync-integration-branch.md`

**Files:**
- Modify: `scripts/sync-integration-branch.md` (Usage `--clone-dir` bullet ~lines 24–25; Invariants section ~lines 37–55)

**Interfaces:**
- Consumes: the as-built behavior from Tasks 1 and 2.
- Produces: a contract whose Usage, Invariants, and gate-2 description match the shipped script. No code dependency; verified by re-running the (now green) suite plus a read-back of the three required facts.

- [ ] **Step 1: Update the `--clone-dir` Usage bullet**

Replace (currently):

```markdown
- `--clone-dir` — directory of the git clone to operate on. Defaults to the repo root containing
  the script itself (`dirname "$0"/..`, resolved with `pwd -P`).
```

with:

```markdown
- `--clone-dir` — directory of the git clone to operate on. Defaults to the **main worktree of the
  repo the script is invoked from** (CWD) — the first entry of `git worktree list --porcelain`, which
  git always lists first and which is reachable from any linked worktree in the set. This targets the
  consuming repo's primary checkout even when the caller's shell sits in a linked worktree (the sync
  site runs from the `.docket/` metadata worktree on the `docket` branch). An explicit `--clone-dir`
  overrides; if CWD is not inside a git repo the not-a-repo gate skips (best-effort, exit 0).
```

- [ ] **Step 2: Update the gate-2 Invariant to describe the explicit note**

Replace the gate-2 sub-bullet (currently):

```markdown
  2. The working tree is clean: `git status --porcelain` produces no output (tracked modifications
     and untracked non-ignored files both block the fast-forward).
```

with:

```markdown
  2. The working tree is clean: `git status --porcelain` produces no output (tracked modifications
     and untracked non-ignored files both block the fast-forward). The skip **note** is explicit that
     untracked non-ignored files block too and states the remedy (commit/stash tracked edits; remove
     or `.gitignore` untracked paths), so a consuming repo with stray untracked files gets a
     diagnosable skip rather than a silent drift. The gate **condition** is unchanged — only the note.
```

- [ ] **Step 3: Add an Invariant documenting the default `--clone-dir` resolution**

Immediately after the **FF-only merge** bullet (the one beginning `- **FF-only merge.**`), insert:

```markdown
- **Default `--clone-dir` resolves the invoking repo's main worktree, not the script's repo.** With
  no `--clone-dir`, the dir is the first entry of `git -C "$PWD" worktree list --porcelain` (the main
  worktree — CWD-independent and reachable from any linked worktree). This is why a bare invocation
  from the linked `.docket/` worktree still fast-forwards the consuming repo's primary checkout. Using
  `git rev-parse --show-toplevel` would instead return the *linked* worktree (on the `docket` branch),
  which gate 1 then skips — so main-worktree resolution is load-bearing. An explicit `--clone-dir`
  overrides (the hermetic tests rely on this).
```

- [ ] **Step 4: Verify the suite is still green and the contract states the three required facts**

Run: `bash tests/test_sync_integration_branch.sh`
Expected: `ALL PASS`, exit 0 (the `.md` edit changes no behavior).

Read-back (confirm by eye, no automated assertion — a fragile grep sentinel on prose is a known trap): the contract now states (a) `--clone-dir` defaults to the main worktree of CWD via `git worktree list --porcelain`; (b) untracked non-ignored files block the FF and the note says so + gives the remedy; (c) the gate condition is unchanged.

- [ ] **Step 5: Commit**

```bash
git add scripts/sync-integration-branch.md
git commit -m "docs(0041): contract — main-worktree --clone-dir default + explicit gate-2 note"
```

---

## Out of scope (do not implement)

- **Keeping the docket *skills* clone fresh in consuming repos** — the separate "update docket" workflow (change 0029's out-of-scope). Skills load from the docket clone regardless of the consuming repo's checkout.
- **Relaxing gate 2 to ignore untracked files** — explicitly rejected (owner decision); the sync never fast-forwards over a non-pristine tree. Only the note changes.
- **Changing the skill call sites** (`docket-finalize-change` step 6, `docket-status` sweep) — they stay bare and inherit the corrected default; no edit.
- **The optional convention Branch-model sentence refresh** — the spec marks it "may be folded in … not required for correctness." The current wording ("fast-forward the clone's local `<integration_branch>` checkout … so the primary checkout does not drift") remains accurate under the fix, so leave it untouched to keep scope tight and avoid touching a skill file other loops may edit.

## Self-Review

- **Spec coverage:** Decision 1 (retarget) → Task 1. Decision 2 (louder gate-2 note, condition unchanged) → Task 2. "What changes (build-time scope)" bullet for `.sh` → Tasks 1+2; for `.md` → Task 3; for the test file's two new cases → Task 1 (Case 8: bare-from-linked main-worktree FF) + Task 2 (Case 9: untracked note); "Skill call sites — no change" → Out of scope. All five "Resolved design decisions" honored (script default not skill-passed; main-worktree not `--show-toplevel`; gate stays conservative; skills clone out of scope; dogfooding preserved with no special-casing). No gaps.
- **Placeholder scan:** none — every code/test/prose block is literal and complete.
- **Type/name consistency:** `CLONE_DIR`, `porcelain`, `count`, `note()`, `GIT`, fixture helpers `new_repo`/`advance_origin`/`git_quiet`/`assert` are used consistently with their existing definitions in the script and test file; new fixtures (`W`, `O`, `root`, `linked`, `$root/bin/sync.sh`) are self-contained within their cases.
