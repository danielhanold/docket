<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0458 — Attach refuses a same-path same-day re-attach with verify-delta invalid-state](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-25-0458-attach-refuses-a-same-path-same-day-re-attach-with-verify-de.md)**
<!-- docket:backlink:end -->
# Attach Same-Day Re-Attach No-Op Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make a same-path, same-day re-attach of a plan or results artifact return a clean `no-op` instead of the spurious `invalid-state` / `verify-delta` refusal.

**Architecture:** `changeAttachOp.Plan` (`internal/app/change_attach.go`) today always declares the change record as a `MutationReplace`. When the artifact field, the `updated:` date, and the artifact block all re-render byte-identical (the same path re-attached on the same day), the transaction engine's two-way delta guard (`verifyActualDelta`) rejects the declared-but-unchanged path. The fix copies the guard change 0445 put in `change_groom.go`'s `Plan` — "Declare only paths whose bytes actually change" — so an identical re-render produces an empty `plan.Files`, which the engine already commits as a clean `no-op`. The engine and the board declaration (`includeBoard`, fixed by change 0335) are untouched.

**Tech Stack:** Go; existing plan-closure test harness in `internal/app` (`attachPlanFor`, `newFakeTree`, `assertPlanPaths`).

**Spec:** `docs/changes/active/0458-attach-refuses-a-same-path-same-day-re-attach-with-verify-de.md` on the `docket` metadata branch (trivial change — its `## What changes` section is the spec).

## Global Constraints

- Change only the change-record declaration in `changeAttachOp.Plan`; leave the `includeBoard` call, the engine's delta guard, and attach idempotency keying exactly as they are (spec "Out of scope").
- Copy the shape and the comment of the `change_groom.go` guard ("Declare only paths whose bytes actually change").
- The regression test must cover both `attach-plan` and `attach-results`, with the inline board on, and must fail before the fix (TDD order below enforces this).
- No skill-text or prose changes: `no-op` is already the correct, self-explanatory answer to a re-attach.
- Cross-references in comments anchor on symbol names or quoted clauses, never line numbers (AGENTS.md).
- Defeat Go's test cache on every verification run: `go test ./internal/app/ -run <name> -count=1`.

## Review Focus

- A **first** attach (record actually changes) must still declare the record and commit — the guard must not skip a real change. Covered: the existing `TestChangeAttachPlanPatch` / `TestChangeAttachResultsPatch` assert the record is declared and patched; they must stay green untouched.
- A re-attach of a **different** path (or the same path on a later day, which moves `updated:`) changes bytes and must still declare the record. Not separately tested — it is the same `!bytes.Equal` branch the first-attach tests pin; the byte-equal case is the only new branch and Task 1 pins it.
- With the inline board **on** and the board already settled, the empty-record case must produce a fully empty `plan.Files` (board declared nothing either, per 0335) — Task 1's test runs with `inline` on and asserts `len(plan.Files) == 0`, not merely "no record file".
- Mutation check: reverting the guard (unconditionally appending the record mutation again) must redden Task 1's test — that is exactly the "fails before fix" run in Task 1, observed before Task 2 lands.

---

### Task 1: Regression test — identical re-attach declares nothing

**Files:**
- Test: `internal/app/change_attach_test.go` (append a new test function)

**Interfaces:**
- Consumes: `attachPlanFor(t, files, op)` and `baseAttachOp(surfaces, id, kind, artifact)` from `change_attach_test.go`; `lifecycleChange`, `groomPath`, `lifecycleRecordBytes`, `planPaths` helpers; `attachKindPlan` / `attachKindResults` constants.
- Produces: `TestChangeAttachIdenticalReattachIsNoOp` — the failing regression test Task 2 turns green.

The test settles the tree the same way `TestChangeGroomPlanReviseIdenticalIsNoOp` does: run the op once against a fresh fixture, write the resulting record (and board) bytes back into the fixture map, then run the identical op again and require an empty plan. `testClock()` is fixed, so "same day" holds by construction: the second run stamps the same `updated:` date, patches the same artifact path, and re-renders the same artifact block — byte-identical output.

- [ ] **Step 1: Write the failing test**

Append to `internal/app/change_attach_test.go`:

```go
// --- TestChangeAttachIdenticalReattachIsNoOp --------------------------------

// TestChangeAttachIdenticalReattachIsNoOp pins the same-path same-day
// re-attach (change 0458): once the record already stores the artifact path
// with today's updated date and a rendered artifact block, re-running the
// identical attach must declare NO files — an empty plan the engine commits
// as a clean no-op — never an unchanged replace the engine's delta verifier
// (verifyActualDelta) refuses as invalid-state. Both kinds, inline board on.
func TestChangeAttachIdenticalReattachIsNoOp(t *testing.T) {
	cases := []struct {
		kind     string
		artifact string
	}{
		{attachKindPlan, "docs/superpowers/plans/2026-09-25-widget-plan.md"},
		{attachKindResults, "docs/results/2026-09-25-widget-results.md"},
	}
	for _, c := range cases {
		t.Run(c.kind, func(t *testing.T) {
			recPath := groomPath(3, "widget")
			files := map[string]string{
				recPath:                 lifecycleChange(3, "widget", "in-progress"),
				"docs/changes/BOARD.md": "# Backlog\n\nold\n",
			}
			op := baseAttachOp([]string{"inline"}, 3, c.kind, c.artifact)

			// First attach: settle the record and the board on the tree.
			first, opRes := attachPlanFor(t, files, op)
			if opRes.Refused {
				t.Fatalf("first attach refused: %v", opRes.Findings)
			}
			files[recPath] = lifecycleRecordBytes(t, first, recPath)
			files["docs/changes/BOARD.md"] = lifecycleRecordBytes(t, first, "docs/changes/BOARD.md")

			// Identical re-attach, same clock day: nothing may be declared.
			second, opRes := attachPlanFor(t, files, op)
			if opRes.Refused {
				t.Fatalf("re-attach refused: %v", opRes.Findings)
			}
			if len(second.Files) != 0 {
				t.Errorf("identical re-attach declared files %v, want an empty (no-op) plan", planPaths(second))
			}
		})
	}
}
```

Note on helpers: `lifecycleRecordBytes` returns the declared bytes for any path in a plan (it is used for `BOARD.md` the same way `TestChangeGroomPlanReviseIdenticalIsNoOp` uses `groomedRecordBytes`). If the first attach's board render is byte-identical to the fixture board it may not be declared at all — in that case `lifecycleRecordBytes` will `t.Fatal`; guard the board write-back with a presence check only if that failure actually occurs (with the `"old"` placeholder board it will be declared).

- [ ] **Step 2: Run the test to verify it fails for the right reason**

Run: `go test ./internal/app/ -run TestChangeAttachIdenticalReattachIsNoOp -count=1 -v`

Expected: FAIL, both subtests, with `identical re-attach declared files [docs/changes/... ] , want an empty (no-op) plan` — the record path declared despite unchanged bytes. A failure at the first attach or a refusal is a fixture bug, not the regression; fix the fixture first.

- [ ] **Step 3: Commit the red test**

```bash
git add internal/app/change_attach_test.go
git commit -m "test(app): red — identical same-day re-attach must declare nothing (change 0458)"
```

(The docket suite gate runs at the end of the build, not per task, so an intentionally red test may be committed here; Task 2 turns it green.)

---

### Task 2: Guard the record declaration in changeAttachOp.Plan

**Files:**
- Modify: `internal/app/change_attach.go` — the `files := []transaction.FileMutation{...}` assembly at the end of `changeAttachOp.Plan`
- Modify: `internal/app/change_attach.go` imports — add `"bytes"`

**Interfaces:**
- Consumes: `finalBytes` (the fully re-rendered record) and `src` (the loaded record source, `st.State.Sources[c.Path()]`), both already in scope in `Plan`.
- Produces: no signature changes; `Plan` returns an empty `MutationPlan.Files` when nothing changed byte-wise.

- [ ] **Step 1: Apply the guard**

In `changeAttachOp.Plan`, replace:

```go
	files := []transaction.FileMutation{
		{Path: gitcli.RepoPath(c.Path()), Kind: transaction.MutationReplace, Bytes: finalBytes},
	}
```

with (the `change_groom.go` guard's shape and comment, adapted):

```go
	// Declare only paths whose bytes actually change: the engine's delta
	// verifier rejects a declared path that is not an actual change, so a
	// same-path same-day re-attach that re-renders the record byte-identical
	// (field already set, updated: already today, artifact block unchanged)
	// must not declare it. An empty plan is the engine's clean no-op path —
	// the same skip includeBoard makes (change 0458).
	var files []transaction.FileMutation
	if !bytes.Equal(finalBytes, src) {
		files = append(files, transaction.FileMutation{
			Path: gitcli.RepoPath(c.Path()), Kind: transaction.MutationReplace, Bytes: finalBytes,
		})
	}
```

Add `"bytes"` to the import block (it sorts first, before `"context"`). Leave the `if o.inline { includeBoard(...) }` block and everything else in the function untouched.

- [ ] **Step 2: Run the regression test to verify it passes**

Run: `go test ./internal/app/ -run TestChangeAttachIdenticalReattachIsNoOp -count=1 -v`

Expected: PASS, both subtests.

- [ ] **Step 3: Run the attach and groom tests to verify no first-attach regression**

Run: `go test ./internal/app/ -run 'TestChangeAttach|TestChangeGroom' -count=1`

Expected: PASS — in particular `TestChangeAttachPlanPatch` and `TestChangeAttachResultsPatch` still see the record declared on a real first attach.

- [ ] **Step 4: Run the package**

Run: `go test ./internal/app/ -count=1`

Expected: PASS. (The whole repository suite runs at the build gate per the repo's gate rule; this step is the per-task package check only.)

- [ ] **Step 5: Commit**

```bash
git add internal/app/change_attach.go
git commit -m "fix(app): attach declares the change record only when its bytes change (change 0458)"
```
