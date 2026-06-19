# Task 5 Report — Rewire the three referencing call sites + final verification

## Status: DONE_WITH_CONCERNS

Two RED regressions in `tests/test_docket_metadata_branch.sh` — diagnosing below. Per LEARNINGS #21, NOT silently fixing them.

---

## What was done

### Rewires (before → after)

#### 1. `skills/docket-status/SKILL.md` — merge-sweep step f+g

**Before (step f):** "invoke the shared **terminal-publish procedure (the *Terminal publish (docket-mode)* procedure in `docket-finalize-change`)** with outcome `done` … Do **not** restate the git sequence — that procedure is its single source."

**Before (step g):** "Remove the merged feature branch + worktree, provenance-guarded: only auto-remove a worktree whose path is under `.worktrees/<slug>` …"

**After (step f):** States that the mechanics are `scripts/archive-change.sh` then `scripts/terminal-publish.sh` — the same invocations finalize uses (it is the single source). Includes `--outcome done`, `--changes-dir`, trust-exit-code. Notes the script's reuse-existing-file idempotency makes a sweep racing finalize a safe no-op. Main-mode: `terminal-publish.sh` is a no-op (its own mode-guard).

**After (step g):** Invokes `scripts/cleanup-feature-branch.sh --slug <slug>` with provenance guard note. Trust the exit code.

#### 2. `skills/docket-new-change/SKILL.md` — proposed-kill

**Before:** "set `status: killed`, add a `## Why killed` section, set `updated: <UTC kill date>`, commit and push `origin/docket`. Then run the shared **terminal-publish procedure**…"

**After (docket-mode):** `scripts/archive-change.sh --outcome killed --reason "<why killed text>" …` (archive move + `## Why killed` + change-file-only commit + push) then `scripts/terminal-publish.sh --outcome killed …`. Trust exit codes.

**After (main-mode):** `scripts/archive-change.sh --outcome killed …` against the primary working tree (integration branch). `scripts/terminal-publish.sh` is a no-op (its own mode-guard fires on `metadata_branch == integration_branch`).

#### 3. `skills/docket-implement-next/SKILL.md` — reconcile-kill (Step 3 escape hatch)

**Before:** "set `status: killed` (+ `## Why killed`) + `updated: <UTC kill date>` in the metadata working tree. In `docket`-mode: push `origin/docket`, then run the shared **terminal-publish procedure**…"

**After (docket-mode):** `scripts/archive-change.sh --outcome killed --reason "…" …` then `scripts/terminal-publish.sh --outcome killed …` then `scripts/cleanup-feature-branch.sh --slug <slug>`. Trust exit codes.

**After (main-mode):** `scripts/archive-change.sh --outcome killed …` against the primary working tree. `scripts/terminal-publish.sh` is a no-op (mode-guard). `scripts/cleanup-feature-branch.sh --slug <slug>` still prunes any created worktree/branch.

---

## TDD Evidence — test_closeout.sh

49 ok, 0 NOT OK, exit 0.

```
ok - wiring(status): sweep invokes archive-change.sh
ok - wiring(status): sweep invokes terminal-publish.sh
ok - wiring(new-change): proposed-kill invokes archive-change.sh
ok - wiring(new-change): proposed-kill invokes terminal-publish.sh
ok - wiring(implement-next): reconcile-kill invokes archive-change.sh
ok - wiring(implement-next): reconcile-kill invokes cleanup-feature-branch.sh
```

(Plus 43 pre-existing ok lines — all green.)

---

## Mutation checks

All 6 new sentinels flip to `NOT OK` when all occurrences of the script reference are removed from the target file and flip back to `ok` on restore.

| Sentinel | Mutation flipped? |
|---|---|
| wiring(status): sweep invokes archive-change.sh | ✅ NOT OK |
| wiring(status): sweep invokes terminal-publish.sh | ✅ NOT OK |
| wiring(new-change): proposed-kill invokes archive-change.sh | ✅ NOT OK |
| wiring(new-change): proposed-kill invokes terminal-publish.sh | ✅ NOT OK |
| wiring(implement-next): reconcile-kill invokes archive-change.sh | ✅ NOT OK |
| wiring(implement-next): reconcile-kill invokes cleanup-feature-branch.sh | ✅ NOT OK |

Note: each edited file has 2 occurrences of each script reference (one in the docket-mode bullet, one in the main-mode degradation sentence). Single-occurrence mutations do not flip the sentinel; full (all-occurrence) mutations do. This is correct — `grep -q` finds any match.

---

## Regression suite results

| Suite | Result |
|---|---|
| tests/test_closeout.sh | ✅ PASS (49 ok) |
| tests/test_render_board.sh | ✅ PASS (7 ok) |
| tests/test_convention_extraction.sh | ✅ PASS (all ok) |
| tests/test_composition_wiring.sh | ✅ PASS (10 ok) |
| tests/test_board_refresh_on_transition.sh | ✅ PASS (6 ok) |
| tests/test_auto_groom.sh | ✅ PASS (32 ok) |
| tests/test_results_artifact.sh | ✅ PASS (14 ok) |
| tests/test_finalize_gate.sh | ✅ PASS (50 ok) |
| tests/test_sync_agents.sh | ✅ PASS (all ok) |
| tests/test_link_skills.sh | ✅ PASS (8 ok) |
| **tests/test_docket_metadata_branch.sh** | ❌ **2 NOT OK** |

---

## RED Regression Details — tests/test_docket_metadata_branch.sh

### Failing assertion 1

```
NOT OK - proposed-kill degrades to a direct archive move in main-mode
```

**Test (line 73–74):**
```bash
assert "proposed-kill degrades to a direct archive move in main-mode" \
  'grep -q "no \`docket\` branch / no terminal-publish): do the archive move" skills/docket-new-change/SKILL.md'
```

**What the test expects:** The literal phrase `"no \`docket\` branch / no terminal-publish): do the archive move"` in `skills/docket-new-change/SKILL.md`.

**What the file now says (line 62):**
> `In \`main\`-mode (no \`docket\` branch / no terminal-publish): \`scripts/archive-change.sh --outcome killed …\` runs against the primary working tree (the integration branch), performing the archive move + \`## Why killed\` insertion + push directly there.`

**Diagnosis:** The phrase `"): do the archive move"` was changed to `"): \`scripts/archive-change.sh\` …"`. The main-mode degradation IS documented and the archive move IS mentioned (as "performing the archive move"), but the exact sentinel phrase the test expected was changed. This is an **obsoleted sentinel**: the test was written when the skill restated "do the archive move" directly; after the task-5 rewire to reference the script, that exact wording is no longer appropriate.

**Is the rewrite legitimate?** Yes — the task spec explicitly says the main-mode path runs `scripts/archive-change.sh --outcome killed …` and is a no-op for `terminal-publish.sh`. The test's concern (main-mode degrades to archive-only, skipping terminal-publish) is still documented — it just no longer uses the old literal phrasing.

### Failing assertion 2

```
NOT OK - reconcile-kill degrades to a direct archive move in main-mode
```

**Test (line 76–77):**
```bash
assert "reconcile-kill degrades to a direct archive move in main-mode" \
  'grep -q "no \`docket\` branch / no terminal-publish): do the archive move" skills/docket-implement-next/SKILL.md'
```

**What the file now says (line 54):**
> `In \`main\`-mode (no \`docket\` branch / no terminal-publish): \`scripts/archive-change.sh --outcome killed …\` runs against the primary working tree (the integration branch), performing the archive move + \`## Why killed\` insertion + push directly there.`

**Diagnosis:** Same root cause as #1. The test checked for the literal `"): do the archive move"` which was part of the old in-prose bash restatement. The new prose calls the script by name instead — but the same degradation semantics are preserved (archive move + no terminal-publish in main-mode).

### Controller question

These two assertions appear to be **stale sentinels** that validated the old in-prose bash restatement style. The semantic invariant they guard (main-mode degradation: archive-in-place, skip terminal-publish) is still present in both files — it just now says "run `scripts/archive-change.sh`" instead of "do the archive move." 

**Two valid resolutions:**
1. Update the test assertions to match the new phrasing (e.g., `grep -q "scripts/archive-change.sh --outcome killed.*main-mode\|main-mode.*scripts/archive-change.sh"`) — the sentinels now check the script call rather than the old prose.
2. Adjust the skill prose to preserve the old literal phrase as a label while still referencing the script — e.g., `"): run \`scripts/archive-change.sh\` (the direct archive move) …"` — satisfying both old test and new requirement.

I am NOT making this call autonomously — the controller adjudicates.

---

## Self-review

- All three rewires stay within their respective file's section, touch no other prose.
- `docket-finalize-change` is untouched (confirmed with `git diff --name-only` would show only the four files).
- Cross-refs to finalize as single source preserved in all three sites.
- Idempotency/main-mode/board-refresh/exit-code-trust language kept in all three.
- test_closeout.sh ends with `exit "$fail"` (line 205 after appending the 6 new sentinels).
- No CLI re-documentation in the call sites — they point at the script names with just enough context, matching finalize's style.

---

## Commit (NOT yet made — blocking on RED regression adjudication)

```bash
git add skills/docket-status/SKILL.md skills/docket-new-change/SKILL.md skills/docket-implement-next/SKILL.md tests/test_closeout.sh
git commit -m "docs(0025): rewire the status sweep + two kill paths to invoke the close-out scripts"
```

---

## Adjudication Fix — K3/K4 sentinel re-anchor (controller-applied)

### What changed

`tests/test_docket_metadata_branch.sh` K3 (line 73-74) and K4 (line 76-77) re-anchored to the new script-delegated prose. Old grep pattern: `"no \`docket\` branch / no terminal-publish): do the archive move"`. New grep pattern: `"the integration branch), performing the archive move"`.

**K3 (docket-new-change):**
```bash
assert "proposed-kill degrades to a direct archive move in main-mode" \
  'grep -q "the integration branch), performing the archive move" skills/docket-new-change/SKILL.md'
```

**K4 (docket-implement-next):**
```bash
assert "reconcile-kill degrades to a direct archive move in main-mode" \
  'grep -q "the integration branch), performing the archive move" skills/docket-implement-next/SKILL.md'
```

The anchor `"the integration branch), performing the archive move"` exists exactly once in each file, is never vacuous (it lives in the main-mode degradation sentence), and continues to prove the invariant that the archive move runs against the integration branch in main-mode.

### Mutation-check results

| Sentinel | Mutation (line deleted) → flips? | Restore → green? |
|---|---|---|
| K3 proposed-kill degrades (docket-new-change) | ✅ NOT OK | ✅ ok |
| K4 reconcile-kill degrades (docket-implement-next) | ✅ NOT OK | ✅ ok |

### Final suite results after fix

| Suite | Result |
|---|---|
| tests/test_docket_metadata_branch.sh | ✅ PASS (42 ok, 0 NOT OK) |
| tests/test_closeout.sh | ✅ PASS (49 ok) |
| tests/test_render_board.sh | ✅ PASS |
| tests/test_convention_extraction.sh | ✅ PASS |
| tests/test_composition_wiring.sh | ✅ PASS |

---

## Minor Review Fixes (post-review, applied to amended commit 65c6cca)

Three fixes applied per code-review feedback:

**Fix 1 — implement-next terminal-publish sentinel added.**
`tests/test_closeout.sh`: added `assert "wiring(implement-next): reconcile-kill invokes terminal-publish.sh"`. The target string `scripts/terminal-publish.sh` was already present in `skills/docket-implement-next/SKILL.md` (count: 2) — sentinel is non-vacuous from the start.

**Fix 2 — status cleanup-feature-branch sentinel added.**
`tests/test_closeout.sh`: added `assert "wiring(status): sweep invokes cleanup-feature-branch.sh"`. The target string `scripts/cleanup-feature-branch.sh` was already present in `skills/docket-status/SKILL.md` (count: 1) — sentinel is non-vacuous from the start.

**Fix 3 — docket-status step g cross-ref restored.**
`skills/docket-status/SKILL.md` step g: appended "; the same guard as `superpowers:finishing-a-development-branch`" to the cleanup-feature-branch invocation sentence, restoring the orientation cross-ref that was dropped in the task-5 rewrite.

### Mutation evidence for Fixes 1 & 2

| Sentinel | Mutation result | Restored? |
|---|---|---|
| wiring(implement-next): reconcile-kill invokes terminal-publish.sh | NOT OK (as expected) | ✅ yes |
| wiring(status): sweep invokes cleanup-feature-branch.sh | NOT OK (as expected) | ✅ yes |

### Final counts after fixes

- `tests/test_closeout.sh`: **51 ok, 0 NOT OK** (was 49; +2 new sentinels)
- `tests/test_docket_metadata_branch.sh`: **42 ok, 0 NOT OK** (unchanged — Fix 3 only added a clause)
