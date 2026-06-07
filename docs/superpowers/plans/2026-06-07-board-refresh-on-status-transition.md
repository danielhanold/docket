# Board refresh on status transitions — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `BOARD.md` reflect a change's status as each transition happens, by establishing one explicit invariant and wiring a Board-pass refresh into the four status-write sites that lack it.

**Architecture:** This is a **docs-and-tests** change to the docket skills (`skills/docket-*/SKILL.md`) plus one new grep-based bash test. The board-refresh *rule* goes into the canonical `## Convention` block (`docket-new-change/SKILL.md`) and is propagated byte-identical to the other four skills by `sync-convention.sh`. The *mechanism* (best-effort retry semantics) lives once, in `docket-implement-next`, cross-referenced from its three inline sites. The renderer is never duplicated — every site "runs the Board pass" defined in `docket-status`. Per the bloat constraint, the ×5 (synced) addition is exactly one sentence.

**Tech Stack:** Markdown (agent-instruction skills); Bash (`sync-convention.sh`, test scripts). Spec: `docs/superpowers/specs/2026-06-07-board-refresh-on-status-transition-design.md` (on the `docket` branch).

**Working tree:** All edits happen in this feature worktree (`.worktrees/board-refresh-on-status-transition/`) on `feat/board-refresh-on-status-transition`. Paths below are repo-relative to that worktree.

---

### Task 1: Write the failing test

**Files:**
- Create: `tests/test_board_refresh_on_transition.sh`

- [ ] **Step 1: Write the test**

Create `tests/test_board_refresh_on_transition.sh` with exactly this content:

```bash
#!/usr/bin/env bash
# tests/test_board_refresh_on_transition.sh — verifies change 0004:
# BOARD.md is refreshed on every status transition, not only at Step 0.
# Run: bash tests/test_board_refresh_on_transition.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
SKILLS=(docket-new-change docket-status docket-implement-next docket-finalize-change docket-adr)

# A. The board-refresh invariant lives in the canonical convention → present in ALL five skills.
for s in "${SKILLS[@]}"; do
  assert "board-refresh invariant present in $s" \
    'grep -q "Board refresh on status writes" "skills/'"$s"'/SKILL.md"'
done
assert "convention blocks in sync (sync-convention.sh --check)" \
  'bash sync-convention.sh --check >/dev/null 2>&1'

# B. docket-implement-next wires its three inline refreshes, best-effort.
assert "implement-next defines best-effort board refresh" \
  'grep -q "Best-effort board refresh" skills/docket-implement-next/SKILL.md'
assert "implement-next has 3 best-effort Board-pass site clauses (claim, reconcile-kill, implemented)" \
  '[ "$(grep -c "run the Board pass (best-effort" skills/docket-implement-next/SKILL.md)" -ge 3 ]'

# C. docket-new-change proposed-kill refreshes the board (must-land, not best-effort).
assert "new-change proposed-kill refreshes board (must-land Board pass)" \
  'grep -q "must-land Board pass" skills/docket-new-change/SKILL.md'

# D. terminal-publish stays board-agnostic — the kill gap is fixed at the SITES, not here.
assert "terminal-publish keeps the 'BOARD.md is never published' guarantee" \
  'grep -qF "is **never** published" skills/docket-finalize-change/SKILL.md'

# E. docket-status gains the board/source drift tripwire (a warning).
assert "docket-status has board/source drift health check" \
  'grep -q "Board/source drift" skills/docket-status/SKILL.md'

exit $fail
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_board_refresh_on_transition.sh; echo "exit=$?"`
Expected: multiple `NOT OK` lines (invariant absent, no best-effort wording, etc.) and `exit=1`. (Assert D may already pass — that guarantee already exists; everything else must fail.)

- [ ] **Step 3: Commit**

```bash
git add tests/test_board_refresh_on_transition.sh
git commit -m "test(0004): failing test for board refresh on status transitions"
```

---

### Task 2: Add the board-refresh invariant to the canonical convention, then sync

**Files:**
- Modify: `skills/docket-new-change/SKILL.md` (canonical `## Convention` block — the lifecycle Rules paragraph)
- Generated (by sync): `skills/docket-status/SKILL.md`, `skills/docket-implement-next/SKILL.md`, `skills/docket-finalize-change/SKILL.md`, `skills/docket-adr/SKILL.md`

- [ ] **Step 1: Insert the invariant after the lifecycle Rules paragraph**

In `skills/docket-new-change/SKILL.md`, find this exact text (end of the `**Rules.**` paragraph, immediately before the Build-readiness heading):

```
Reserve explicit `blocked` for external blockers the system can't infer.

### Build-readiness & selection (shared definition)
```

Replace it with:

```
Reserve explicit `blocked` for external blockers the system can't infer.

**Board refresh on status writes.** Any skill that writes a change's `status:` regenerates `BOARD.md` (the Board pass) in a separate commit immediately after — the board is a derived view and must never trail the change files.

### Build-readiness & selection (shared definition)
```

(This is the ENTIRE ×5 addition. The "terminal-publish never touches the board" nuance is intentionally NOT added here — it is already true in terminal-publish and is realized at the kill sites in Tasks 3–4.)

- [ ] **Step 2: Propagate the convention block to the other four skills**

Run: `bash sync-convention.sh`
Expected: `synced .../docket-status/SKILL.md`, `.../docket-implement-next/SKILL.md`, `.../docket-finalize-change/SKILL.md`, `.../docket-adr/SKILL.md` (four lines; canonical `docket-new-change` is untouched).

- [ ] **Step 3: Verify the convention is in sync and the invariant reached all five**

Run: `bash sync-convention.sh --check && echo "IN SYNC" && grep -lc "Board refresh on status writes" skills/docket-*/SKILL.md`
Expected: `convention in sync`, `IN SYNC`, and five files each reporting `1`.

- [ ] **Step 4: Run the test — assert group A now passes**

Run: `bash tests/test_board_refresh_on_transition.sh; echo "exit=$?"`
Expected: the five `board-refresh invariant present in <skill>` asserts and `convention blocks in sync` are now `ok`; B/C/E still `NOT OK`; `exit=1`.

- [ ] **Step 5: Commit**

```bash
git add skills/
git commit -m "feat(0004): board-refresh invariant in canonical convention (synced x5)"
```

---

### Task 3: Wire docket-implement-next's three inline refreshes (best-effort)

**Files:**
- Modify: `skills/docket-implement-next/SKILL.md` (Step 2, Step 3 reconcile-kill bullet, Step 7, plus a new best-effort definition subsection)

- [ ] **Step 1: Add the best-effort definition subsection after Step 7**

Find this exact text (end of Step 7, immediately before the reconcile-pass section):

```
**STOP.** The change stays in `active/` as `implemented` until a human merges it, or approves `docket-finalize-change` to merge it.

## The reconcile pass and the `reconciled` flag
```

Replace it with:

```
**STOP.** The change stays in `active/` as `implemented` until a human merges it, or approves `docket-finalize-change` to merge it.

### Best-effort board refresh

The Board pass this skill runs after its own status writes (claim, reconcile-kill, `implemented`) is **best-effort**: attempt the regen + push with bounded retries, then **log and continue** — never abort the build for it. The build's correctness rests on the change-file CAS, not the board; any residual staleness self-heals at the next must-land Board pass (the next change's Step 0 `docket-status`, a manual `docket-status`, or finalize). The board is always a **separate commit** from the `status:` write (keeping the claim CAS byte-identical across concurrent agents).

## The reconcile pass and the `reconciled` flag
```

- [ ] **Step 2: Append the Board-pass clause to Step 2 (claim)**

Find this exact text (end of Step 2):

```
The arbiter is the re-read (abort if no longer `proposed`), not that any single push succeeds. No worktree yet.
```

Replace it with:

```
The arbiter is the re-read (abort if no longer `proposed`), not that any single push succeeds. No worktree yet.

Then run the Board pass (best-effort — see *Best-effort board refresh*) as a separate commit, so the board reflects the change as `in-progress` rather than build-ready.
```

- [ ] **Step 3: Append the Board-pass clause to the Step 3 reconcile-kill bullet**

Find this exact text (tail of the OBSOLETE bullet):

```
then loop back to Step 1. The `<UTC kill date>` is the same date used for the `archive/<date>-…` filename prefix.
```

Replace it with:

```
then loop back to Step 1. The `<UTC kill date>` is the same date used for the `archive/<date>-…` filename prefix. In both modes, after the kill is archived, run the Board pass (best-effort — see *Best-effort board refresh*) as a separate commit so the board drops the killed change before looping back to Step 1.
```

- [ ] **Step 4: Append the Board-pass clause to Step 7 (`implemented`)**

Find this exact text (the metadata-write paragraph of Step 7):

```
Then, BACK IN THE **METADATA WORKING TREE** (in `docket`-mode, `.docket/`), set `status: implemented` + `pr:` (and `results:` if a results file was written in step 6.5) and commit + push on `metadata_branch` (in `docket`-mode, `origin/docket`) — NEVER in the feature worktree (metadata always lands on `metadata_branch`; this is also what lets the sweep read `pr:`).
```

Replace it with:

```
Then, BACK IN THE **METADATA WORKING TREE** (in `docket`-mode, `.docket/`), set `status: implemented` + `pr:` (and `results:` if a results file was written in step 6.5) and commit + push on `metadata_branch` (in `docket`-mode, `origin/docket`) — NEVER in the feature worktree (metadata always lands on `metadata_branch`; this is also what lets the sweep read `pr:`). Then run the Board pass (best-effort — see *Best-effort board refresh*) as a separate commit, so the board shows the change as `implemented` — needs your merge.
```

- [ ] **Step 5: Run the test — assert group B now passes**

Run: `bash tests/test_board_refresh_on_transition.sh; echo "exit=$?"`
Expected: `implement-next defines best-effort board refresh` and `implement-next has 3 best-effort Board-pass site clauses` are now `ok`. Verify the count directly: `grep -c "run the Board pass (best-effort" skills/docket-implement-next/SKILL.md` → `3`.

- [ ] **Step 6: Confirm the convention block is still in sync (no marker corruption)**

Run: `bash sync-convention.sh --check`
Expected: `convention in sync` (the edits are all OUTSIDE the convention markers).

- [ ] **Step 7: Commit**

```bash
git add skills/docket-implement-next/SKILL.md
git commit -m "feat(0004): best-effort board refresh at implement-next claim/reconcile-kill/implemented"
```

---

### Task 4: Wire docket-new-change's proposed-kill (must-land)

**Files:**
- Modify: `skills/docket-new-change/SKILL.md` (Proposed-kill sub-path — body, outside the convention block)

- [ ] **Step 1: Append the must-land board-refresh clause to the proposed-kill sub-path**

Find this exact text (final paragraph of the `## Proposed-kill sub-path` section):

```
A `proposed` change never had a feature branch or open PR, so there is nothing to clean up — and usually no plan/results, so the kill publishes only what is on `docket`: the change file, plus its `spec:`/`adrs:` if set. This skill still writes markdown only — the terminal-publish copy touches no code.
```

Replace it with:

```
A `proposed` change never had a feature branch or open PR, so there is nothing to clean up — and usually no plan/results, so the kill publishes only what is on `docket`: the change file, plus its `spec:`/`adrs:` if set. This skill still writes markdown only — the terminal-publish copy touches no code.

In both modes, after the kill is archived, refresh `BOARD.md` via the **must-land Board pass** (a separate commit, same as the create path's step 5) so the killed change leaves the board. terminal-publish copies records to the integration branch but never touches `BOARD.md`, so the board refresh is this skill's responsibility.
```

- [ ] **Step 2: Run the test — assert group C now passes**

Run: `bash tests/test_board_refresh_on_transition.sh; echo "exit=$?"`
Expected: `new-change proposed-kill refreshes board (must-land Board pass)` is now `ok`. Only group E remains `NOT OK`.

- [ ] **Step 3: Confirm convention still in sync**

Run: `bash sync-convention.sh --check`
Expected: `convention in sync` (the proposed-kill section is outside the markers).

- [ ] **Step 4: Commit**

```bash
git add skills/docket-new-change/SKILL.md
git commit -m "feat(0004): must-land board refresh at new-change proposed-kill"
```

---

### Task 5: Add the board/source drift tripwire to docket-status health checks

**Files:**
- Modify: `skills/docket-status/SKILL.md` (Health checks list, end)

- [ ] **Step 1: Append the drift health-check bullet**

Find this exact text (last bullet of the `## Health checks` list):

```
- **`depends_on` cycles** — detect circular dependency chains; flag every change in the cycle.
```

Replace it with:

```
- **`depends_on` cycles** — detect circular dependency chains; flag every change in the cycle.
- **Board/source drift** — render the board in-memory from the change files (reusing the shared dependency-resolution pass) and compare it to the committed `BOARD.md`; if any change's rendered status or placement disagrees, **warn** naming the change(s) (a writer skipped the board-refresh invariant), then let the Board pass regenerate as usual — which heals it. A warning, not a failure: a best-effort refresh is allowed to lose a race, and the following regen fixes the drift.
```

- [ ] **Step 2: Run the test — full suite now green**

Run: `bash tests/test_board_refresh_on_transition.sh; echo "exit=$?"`
Expected: every line `ok`, `exit=0`.

- [ ] **Step 3: Confirm convention still in sync**

Run: `bash sync-convention.sh --check`
Expected: `convention in sync` (the health-checks list is outside the markers).

- [ ] **Step 4: Commit**

```bash
git add skills/docket-status/SKILL.md
git commit -m "feat(0004): board/source drift tripwire in docket-status health checks"
```

---

### Task 6: Full verification

**Files:** none (verification only)

- [ ] **Step 1: Run the new test**

Run: `bash tests/test_board_refresh_on_transition.sh; echo "exit=$?"`
Expected: all `ok`, `exit=0`.

- [ ] **Step 2: Run the existing convention + metadata tests (guard against regressions)**

Run: `bash tests/test_sync_convention.sh; echo "exit=$?"` then `bash tests/test_docket_metadata_branch.sh; echo "exit=$?"`
Expected: both end with no `NOT OK` lines and `exit=0` (item A "convention blocks in sync" still green after our canonical edit + sync).

- [ ] **Step 3: Confirm the three inline clauses and the count once more**

Run: `grep -c "run the Board pass (best-effort" skills/docket-implement-next/SKILL.md`
Expected: `3`.

- [ ] **Step 4: Sanity-check the working tree is committed**

Run: `git status --porcelain`
Expected: empty (all changes committed across Tasks 1–5).

---

## Notes for the executor

- **Edit in this worktree only.** Do not touch the main tree's `skills/` or the `.docket/` metadata tree. The change file's `plan:`/`pr:` fields are written on `metadata_branch` by the implementer outside this plan — not here.
- **No code, only docs + one bash test.** There is nothing to run beyond the test scripts; do not attempt to "execute" the skills.
- **The `sync-convention.sh` run is load-bearing** (Task 2 Step 2). If you hand-edit a non-canonical skill's convention block, `sync-convention.sh --check` will fail — always edit the canonical (`docket-new-change`) and re-sync.
