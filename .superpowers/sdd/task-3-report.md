# Task 3 Report: scripts/cleanup-feature-branch.sh

## Status: DONE

## Implementation Summary

Created `scripts/cleanup-feature-branch.sh` verbatim from the brief's spec and appended the cleanup
test section to `tests/test_closeout.sh` above the final `exit "$fail"`. `CLEANUP=` was already
defined on line 10 by a prior task — not added again.

**Files changed:**
- Created: `scripts/cleanup-feature-branch.sh` (chmod +x)
- Modified: `tests/test_closeout.sh` (appended 5 new asserts above `exit "$fail"`)

## TDD RED/GREEN Evidence

### Step 1 — RED (before implementation)
```
$ bash tests/test_closeout.sh 2>&1 | tail -10
...
NOT OK - cleanup: worktree removed
NOT OK - cleanup: local branch deleted
NOT OK - cleanup: remote branch deleted
ok - cleanup: refuses a worktree outside .worktrees/ (non-zero)   ← coincidental pass
ok - cleanup: out-of-tree worktree survives the refusal            ← coincidental pass
```
3 of 5 new asserts failed (script missing → sub-shell exit 127 causes command not found for all three
positive asserts; the two refusal asserts "passed" coincidentally since the missing script exits non-zero).

### Step 2 — GREEN (after implementation)
```
$ bash tests/test_closeout.sh 2>&1 | grep -c "^ok"
35
$ bash tests/test_closeout.sh 2>&1 | grep "NOT OK"
(none)
```
All 35 asserts pass (30 existing Tasks 1+2 + 5 new cleanup asserts).

## Mutation Check

Temporarily changed the `case` guard from:
```bash
case "$rp/" in
  "$allowed_root/"*) ;;
  *) die "refusing to remove ...";;
esac
```
to:
```bash
case "$rp/" in
  *) ;;   # always allow
esac
```

Result: All 5 cleanup asserts still passed. Explanation: the brief's provenance test uses
`--slug evil` but the worktree is registered at `...elsewhere`, so `target = .../evil` does NOT
exist and the `if [ -e "$target" ]` block is skipped entirely in BOTH the correct and mutated
versions. The exit-1 for the provenance refusal assert comes from the postcondition check:
`feat/evil` is checked out in the `elsewhere` worktree → `git branch -D feat/evil` is silently
swallowed by `|| true` → branch still exists → `git rev-parse --verify -q feat/evil` succeeds →
`die "postcondition: local branch still present"` → exit 1. This behavior is identical in both
correct and weakened guard forms because the guard never fires.

**Assessment:** The provenance refusal assert in the test coincidentally passes for the right
reason (the script does refuse, via the postcondition, to silently clean up an in-use branch),
but the specific case gate isn't exercised by this test. The guard does protect correctly for
the case where `--worktrees-dir` is a path INSIDE the repo's `.worktrees/` (the real risk:
only worktrees under `.worktrees/` are removed; others are refused). The test design exercises
the "out-of-tree worktree survives" contract correctly.

Guard was restored to correct implementation after mutation check.

## Self-Review

- House style: `set -uo pipefail` (not `set -e`), `GIT="${GIT:-git}"`, `die()`/`log()` to stderr ✓
- No `docket-frontmatter.sh` sourced (pure git) ✓
- No SIGPIPE hazard: using `git ls-remote --exit-code` capture, `git rev-parse --verify -q` ✓
- macOS path canonicalization: both sides resolved via `canon()` using `cd … && pwd -P` ✓
- Flag names match brief exactly: `--slug`, `--worktrees-dir`, `--remote` ✓
- Default `--worktrees-dir .worktrees`, default `--remote origin` ✓
- Fail-closed postcondition checks worktree and branch are gone ✓
- Does not touch `.docket/`, change files, ADRs, or SKILL.md ✓

## Concerns

1. **Mutation check nuance:** The `case` guard is not exercised by the test's provenance scenario
   because `--slug evil` doesn't match the worktree path `elsewhere`. The actual guard is correct
   and would fire if someone passed a `--worktrees-dir` pointing outside `.worktrees/` AND the
   slug happened to match an existing directory there. This is a test design limitation, not an
   implementation flaw.

2. **`[ -e "$target" ]` path non-canonicalization:** `$target` is built from the raw
   `$WORKTREES_DIR/$SLUG` without resolving symlinks. On macOS, a `/var/...` path will pass
   `-e` even though `pwd -P` gives `/private/var/...`. This is fine for existence checks but
   means the guard only fires when the directory actually exists. If the worktree directory
   doesn't exist (already removed), the guard is skipped and only branch cleanup is done — which
   is the intended behavior (idempotent on the directory).

## Commits

- `f7e711e` feat(0025): cleanup-feature-branch.sh — provenance-guarded worktree + branch teardown

---

## Fix: Provenance-Guard Test Re-anchor (2026-06-19)

**Defect diagnosed:** The old test created a worktree at `<tmp>/elsewhere` but invoked
`--slug evil --worktrees-dir <tmp>`, so `target = <tmp>/evil` did not exist. The guard's
`if [ -e "$target" ]` block was skipped entirely. The non-zero exit that made the assert
"pass" came from the postcondition (`feat/evil` still checked out in the orphaned worktree →
`die "postcondition: local branch still present"`). A mutation weakening the guard produced
identical results — the old asserts did NOT flip.

**Rewritten provenance-refusal block:**
```bash
# --- cleanup-feature-branch.sh: provenance guard refuses an out-of-.worktrees path ---
read -r W _ < <(new_repo)
out_base="$(mktemp -d)"
git -C "$W" worktree add "$out_base/evil" -b feat/evil main >/dev/null 2>&1
( cd "$W" && "$CLEANUP" --slug evil --worktrees-dir "$out_base" ) >/dev/null 2>&1; rc_guard=$?
assert "cleanup: refuses a worktree outside .worktrees/ (non-zero)" '[ "$rc_guard" -ne 0 ]'
assert "cleanup: out-of-tree worktree survives the refusal" '[ -e "$out_base/evil" ]'
assert "cleanup: refused branch feat/evil still present (guard fired before delete)" 'git -C "$W" rev-parse --verify -q feat/evil >/dev/null'
```

The key fix: worktree created at `$out_base/evil` (matches `--slug evil --worktrees-dir $out_base`),
so `target` EXISTS, the guard fires, `die` runs BEFORE branch deletion, and `feat/evil` survives.

**Mutation check evidence:**
- Guard weakened (`*) ;;` always-allow in `case`): all three new asserts flipped to NOT OK
  - `NOT OK - cleanup: refuses a worktree outside .worktrees/ (non-zero)` (guard allowed, script exited 0 via postcondition passing)
  - `NOT OK - cleanup: out-of-tree worktree survives the refusal` (worktree was removed)
  - `NOT OK - cleanup: refused branch feat/evil still present (guard fired before delete)` (branch was deleted)
- Guard restored: all three asserts return to ok

**Final ok count (post-fix, guard restored):** 36 ok, 0 NOT OK (net-new third assert added)

**Amended commit:** `98f9d05` feat(0025): cleanup-feature-branch.sh — provenance-guarded worktree + branch teardown
