<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0509 — Edit an ungroomed stub through a typed operation](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-04-0509-edit-an-ungroomed-stub-through-a-typed-operation.md)**
<!-- docket:backlink:end -->
# Edit an ungroomed stub through a typed operation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use docket-build to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let `change.groom` `outcome: revise` edit a needs-grooming stub's title, owned proposal sections, and relationship fields while it stays needs-grooming. Guard `## Auto-groom blocked` against every `revise`, and update the groom skill, glossary, and CLI help so a human can reach this path.

**Architecture:** Widen the existing `revise` gate in `changeGroomOp.Plan` (`internal/app/change_groom.go`) from "proposed and already groomed" to "proposed". Section splicing, retitle, relationship collections, the artifacts re-render, the inline board, and the exact-blob CAS already run under `revise`, and `revise` never writes `spec:`, `trivial:`, or `auto_groomable:`. So a stub keeps its groom state with no new code path. Add one shape rule to `validateChangeGroomShape` that refuses a `revise` section edit naming `## Auto-groom blocked`, the same check `re-enable` already has. No new outcome, operation, or request field.

**Tech Stack:** Go (`go test`), Markdown skill bodies under `skills/`, the generated embedded asset tree (`go generate ./internal/assets/`).

**Spec:** `docs/superpowers/specs/2026-10-04-edit-an-ungroomed-stub-through-a-typed-operation-design.md` (on the `docket` branch; synchronized copy at `/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-10-04-edit-an-ungroomed-stub-through-a-typed-operation-design.md`)

## Global Constraints

- Widen the existing `revise` outcome. Add no new outcome, operation, or request field.
- New `revise` gate: refuse `not-revisable` unless `c.Status() == domain.StatusProposed`. The message names only the status: `change %04d is not a proposed change (status %q)`.
- The `spec` / `trivial` / `abstain` / `re-enable` gate (`not-groomable`) does not change.
- `revise` still never writes `spec:`, `trivial:`, or `auto_groomable:`.
- New `spec-not-linked` message, verbatim: `change %04d has no linked spec to revise`.
- The `empty-revise` shape rule does not change. A relationships-only revise is still refused.
- A `revise` section edit whose heading is `## Auto-groom blocked` refuses `invalid-section-heading`, for every `revise` target. Do not touch `spec` and `trivial` (out of scope).
- Out of scope: editing changes past `proposed`; editing `type`, `priority`, or `auto_groomable`; changing a stub's groom state through `revise`; guarding `## Auto-groom blocked` on `spec` / `trivial`.
- **Correction to the spec's §3 file location (verified against the code, see the `verify-the-claim` learning):** the authored skill sources are under the repo-root `skills/` directory. `internal/assets/embedded/tree/skills/` is a **generated** copy: `internal/assets/generate.go` carries `//go:generate go run ../../cmd/genassets -repo ../..` with `DefaultAllowedRoots()` rooted at `skills`, and `TestEmbeddedMatchesAuthored` fails with "embedded manifest is stale — run `go generate ./internal/assets/`" whenever the two differ. Edit `skills/docket-groom-next/SKILL.md`, then regenerate with `go generate ./internal/assets/`. Never hand-edit the embedded tree.
- Leave `skills/docket-new-change/SKILL.md` unchanged. Its pointer covers adjusting a *just-landed* spec, which is still an already-groomed change, so the wording stays accurate (spec §3).
- Maintained docs (`docs/reference/glossary.md`, skill bodies, CLI help) describe only current behavior: no change numbers, no "used to" history. ADR citations are fine.
- Cross-references in comments anchor on symbol names or quoted clauses, never line numbers (ADR-0054, `TestCommentAnchorStyle`).
- Every mutation probe and every re-verification run uses `go test -count=1`. A cached `ok` is not evidence. Before mutating, back up with `cp "$f" "$f.bak"`, and restore with `mv -f "$f.bak" "$f"`, never `git checkout --` (which restores HEAD and drops the uncommitted edit). Before reading a probe's result, confirm with `git diff` that the mutation landed.
- Run `gofmt -l internal/app internal/cli internal/repoguard` before each commit that touches Go. The output must be empty.
- The build gate runs the whole suite through `build.test_command` (`go run ./cmd/docket development test`), entered from source, not only the tests named here. Read the budget report even when the run is green.
- Stage explicit paths only. Never `git add -A`.

## Review Focus

1. **An abstained stub (it carries `## Auto-groom blocked` and `auto_groomable: false`) revised on an unrelated section**: the marker section and the flag must both survive untouched, and the board must still read `auto-groom blocked — needs you`. The marker's presence carries state (the `presence-encoded-state` learning), and stubs are the only changes that normally carry it. Pinned in Task 1 (`TestChangeGroomPlanReviseAbstainedStubKeepsMarker`).
2. **A `revise` section edit with `intent: remove` (not only `replace`) on `## Auto-groom blocked`**: removal desynchronizes the marker from the flag just as replacement does, so it must refuse too. Pinned in Task 2 (shape-table row "revise removing ## Auto-groom blocked").
3. **A stub `revise` carrying relationship fields alongside a section edit**: the collections must be written as complete desired values, exactly as on a groomed change. Pinned in Task 1 (`TestChangeGroomPlanReviseNeedsGroomingStubRelationships`).
4. **A stub `revise` carrying `spec_markdown`**: it must refuse `spec-not-linked` with the reworded message and write nothing, not `not-revisable` and not a silently minted spec. Pinned in Task 1 (converted table row plus a message assert).
5. **The guard reached through the real entry point**: `ChangeGroom` must refuse the request as `invalid-input` before any engine call, so nothing is written whatever the change's state. Pinned in Task 2 (`TestChangeGroomReviseAutoGroomBlockedRefusedWithoutEngineCall`).

---

## File Structure

- Modify `internal/app/change_groom.go`: the file-header comment, the `GroomRevise` doc comment, the `revise` gate and its comment in `changeGroomOp.Plan`, the `spec-not-linked` message, and the new `revise` heading check in `validateChangeGroomShape`.
- Modify `internal/app/change_groom_test.go`: convert one refusal-table row, add stub-revise success tests, and add the guard tests.
- Modify `skills/docket-groom-next/SKILL.md`: the authored source. Then regenerate `internal/assets/embedded/` with `go generate ./internal/assets/`.
- Modify `internal/repoguard/budgets_test.go`: the `docket-groom-next/SKILL.md` word ceiling.
- Modify `internal/repoguard/prose_contracts_test.go`: one new row, `change_0509_stub_revise`.
- Modify `docs/reference/glossary.md`: the Groom entry, the Groom outcome `revise` entry, and the `docket-groom-next` entry.
- Modify `internal/cli/change.go`: the `change groom` short help.

---

### Task 1: Widen the `revise` gate to every `proposed` change

**Files:**
- Modify: `internal/app/change_groom.go`: file-header comment (the paragraph starting "This file is the `change groom` planning operation"), the `GroomRevise` constant's doc comment, the "Groom/revise gate" block in `changeGroomOp.Plan`, and the `spec-not-linked` `refuseGroom` call in the "Revise spec-body decision" block
- Test: `internal/app/change_groom_test.go`

**Interfaces:**
- Consumes: the existing test helpers `groomableChange`, `groomPath`, `validReviseRequest`, `titleOnlyReviseRequest`, `abstainedChange`, `fixtureChange`, `groomPlanFor`, `baseGroomOp`, `groomedRecordBytes`, `assertPlanPaths`, `assertGroomReceiptSpecPath`, `planPaths`, `changeGroomReceipt`.
- Produces: refusal code strings unchanged (`not-revisable`, `spec-not-linked`). The `not-revisable` message becomes `change %04d is not a proposed change (status %q)`. The `spec-not-linked` message becomes `change %04d has no linked spec to revise`.

- [ ] **Step 1: Convert the refusal-table row whose premise this change deletes**

In `TestChangeGroomPlanReviseRefusals`, the row named `"not-revisable needs-grooming"` guards "a stub is not revisable", and this change removes that subject. Its request (`validReviseRequest()`, which carries `spec_markdown` + `spec_revision`) now exercises a different, still-real property: a spec-body revise of a stub refuses `spec-not-linked`. Replace the row and its comment:

```go
		// Spec item 6: a needs-grooming change is groom's target, not revise's.
		{"not-revisable needs-grooming", map[string]string{
			groomPath(2, "add-a-widget"): groomableChange(2, "add-a-widget"),
		}, func(r *ChangeGroomRequest) {}, "not-revisable"},
```

with:

```go
		// A needs-grooming stub is revisable, but it links no spec, so a
		// spec-body revise of it refuses before any mutation is assembled.
		{"spec-not-linked needs-grooming", map[string]string{
			groomPath(2, "add-a-widget"): groomableChange(2, "add-a-widget"),
		}, func(r *ChangeGroomRequest) {}, "spec-not-linked"},
```

Keep every other `not-revisable` row (blocked, in-progress, deferred, implemented, done, killed) unchanged. The table loop already asserts the refused plan carries no files.

- [ ] **Step 2: Add the stub-revise success and message tests**

Append these tests to `internal/app/change_groom_test.go`, after `TestChangeGroomPlanReviseRefusals`:

```go
// optedOutStub is the groomable fixture with an explicit auto_groomable: false,
// so a revise that wrote the flag (either way) is observable.
func optedOutStub(id int, slug string) string {
	return strings.Replace(groomableChange(id, slug), "trivial: false\n", "trivial: false\nauto_groomable: false\n", 1)
}

// stubSectionReviseRequest is a sections-only revise of the stub at id 2.
func stubSectionReviseRequest() ChangeGroomRequest {
	r := validReviseRequest()
	r.SpecMarkdown, r.SpecRevision = "", ""
	r.Sections = []SectionEditRequest{{Heading: "## Why", Intent: "replace", Markdown: "Sharpened why.\n"}}
	return r
}

// TestChangeGroomPlanReviseNeedsGroomingStub pins that revise edits a
// needs-grooming stub's owned sections and leaves its groom state alone: no
// spec: written, trivial: still false, auto_groomable: untouched, a fresh
// updated:, the artifacts block intact, and the board row still needs-grooming.
func TestChangeGroomPlanReviseNeedsGroomingStub(t *testing.T) {
	files := map[string]string{
		groomPath(2, "add-a-widget"): optedOutStub(2, "add-a-widget"),
		"docs/changes/BOARD.md":      "# Backlog\n\nold\n",
	}
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{"inline"}, stubSectionReviseRequest()))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		groomPath(2, "add-a-widget"): transaction.MutationReplace,
		"docs/changes/BOARD.md":      transaction.MutationReplace,
	})
	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	if strings.Contains(rec, "Original why.") || !strings.Contains(rec, "Sharpened why.") {
		t.Errorf("## Why section not replaced:\n%s", rec)
	}
	if strings.Contains(rec, "spec: '") {
		t.Errorf("revise of a stub wrote a spec link:\n%s", rec)
	}
	if !strings.Contains(rec, "trivial: false") || strings.Contains(rec, "trivial: true") {
		t.Errorf("revise of a stub changed trivial:\n%s", rec)
	}
	if !strings.Contains(rec, "auto_groomable: false") || strings.Contains(rec, "auto_groomable: true") {
		t.Errorf("revise of a stub changed auto_groomable:\n%s", rec)
	}
	if !strings.Contains(rec, "updated: '2026-08-16'") {
		t.Errorf("updated not stamped from the clock:\n%s", rec)
	}
	if strings.Count(rec, "docket:artifacts:start") != 1 || strings.Count(rec, "docket:artifacts:end") != 1 {
		t.Errorf("artifacts block not intact after the re-render:\n%s", rec)
	}
	board := string(groomedRecordBytes(t, plan, "docs/changes/BOARD.md"))
	if !strings.Contains(board, "needs-grooming") || strings.Contains(board, "build-ready") {
		t.Errorf("board row is no longer needs-grooming:\n%s", board)
	}
	var receipt changeGroomReceipt
	if err := json.Unmarshal(plan.Receipt, &receipt); err != nil || receipt.Outcome != "revise" {
		t.Errorf("receipt = %s (%v), want outcome revise", plan.Receipt, err)
	}
	assertGroomReceiptSpecPath(t, plan, "")
}

// TestChangeGroomPlanTitleOnlyReviseOfNeedsGroomingStub pins that a title alone
// retitles a stub. A stub links no spec, so only the record (and the board) are
// written, and the slug stays put.
func TestChangeGroomPlanTitleOnlyReviseOfNeedsGroomingStub(t *testing.T) {
	files := map[string]string{
		groomPath(2, "add-a-widget"): groomableChange(2, "add-a-widget"),
		"docs/changes/BOARD.md":      "# Backlog\n\nold\n",
	}
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{"inline"}, titleOnlyReviseRequest("Renamed widget")))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		groomPath(2, "add-a-widget"): transaction.MutationReplace,
		"docs/changes/BOARD.md":      transaction.MutationReplace,
	})
	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	for _, want := range []string{"title: 'Renamed widget'", "slug: add-a-widget", "trivial: false"} {
		if !strings.Contains(rec, want) {
			t.Errorf("record missing %q:\n%s", want, rec)
		}
	}
	if strings.Contains(rec, "spec: '") {
		t.Errorf("title-only revise of a stub wrote a spec link:\n%s", rec)
	}
	board := string(groomedRecordBytes(t, plan, "docs/changes/BOARD.md"))
	if !strings.Contains(board, "| Renamed widget |") || !strings.Contains(board, "needs-grooming") {
		t.Errorf("board row not retitled or no longer needs-grooming:\n%s", board)
	}
}

// TestChangeGroomPlanReviseNeedsGroomingStubRelationships pins that a stub revise
// writes relationship collections as complete desired values, as on a groomed
// change.
func TestChangeGroomPlanReviseNeedsGroomingStubRelationships(t *testing.T) {
	files := map[string]string{
		groomPath(2, "add-a-widget"):        groomableChange(2, "add-a-widget"),
		"docs/changes/active/0001-first.md": fixtureChange(1, "first"),
	}
	req := stubSectionReviseRequest()
	req.Related = []int{1}
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{}, req))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	if !strings.Contains(rec, "related: [1]") {
		t.Errorf("related not written as the complete desired value:\n%s", rec)
	}
	if strings.Contains(rec, "spec: '") || strings.Contains(rec, "trivial: true") {
		t.Errorf("stub revise changed its groom state:\n%s", rec)
	}
}

// TestChangeGroomPlanReviseAbstainedStubKeepsMarker pins that a revise of an
// abstained stub on an unrelated section leaves the ## Auto-groom blocked
// marker and auto_groomable: false exactly as they were, so the board keeps
// reading "auto-groom blocked — needs you".
func TestChangeGroomPlanReviseAbstainedStubKeepsMarker(t *testing.T) {
	files := map[string]string{
		groomPath(2, "add-a-widget"): abstainedChange(2, "add-a-widget"),
		"docs/changes/BOARD.md":      "# Backlog\n\nold\n",
	}
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{"inline"}, stubSectionReviseRequest()))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	if !strings.Contains(rec, "Sharpened why.") {
		t.Errorf("## Why section not replaced:\n%s", rec)
	}
	if strings.Count(rec, "## Auto-groom blocked") != 1 || !strings.Contains(rec, "First note.") {
		t.Errorf("abstain marker section not preserved:\n%s", rec)
	}
	if !strings.Contains(rec, "auto_groomable: false") || strings.Contains(rec, "auto_groomable: true") {
		t.Errorf("auto_groomable changed under revise:\n%s", rec)
	}
	board := string(groomedRecordBytes(t, plan, "docs/changes/BOARD.md"))
	if !strings.Contains(board, "auto-groom blocked — needs you") {
		t.Errorf("board lost the auto-groom blocked cell:\n%s", board)
	}
}

// TestChangeGroomPlanReviseSpecNotLinkedMessage pins the reworded refusal: it
// covers both a trivial change and a needs-grooming stub, so it names neither.
func TestChangeGroomPlanReviseSpecNotLinkedMessage(t *testing.T) {
	files := map[string]string{groomPath(2, "add-a-widget"): groomableChange(2, "add-a-widget")}
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{}, validReviseRequest()))
	if !opRes.Refused || len(plan.Files) != 0 {
		t.Fatalf("want a refusal writing nothing, got refused=%v files=%v", opRes.Refused, planPaths(plan))
	}
	want := "change 0002 has no linked spec to revise"
	found := false
	for _, f := range opRes.Findings {
		found = found || (f.Code == "spec-not-linked" && f.Message == want)
	}
	if !found {
		t.Errorf("want spec-not-linked with message %q; got %v", want, opRes.Findings)
	}
}
```

If `fixtureChange` is not visible from this file (it is used by `TestChangeGroomPlanRelationshipsWritten` in the same file, so it should be), use exactly the helper that test uses.

- [ ] **Step 3: Run the new tests and confirm they fail for the right reason**

Run: `go test -count=1 ./internal/app/ -run 'TestChangeGroomPlanReviseRefusals|TestChangeGroomPlanReviseNeedsGroomingStub|TestChangeGroomPlanTitleOnlyReviseOfNeedsGroomingStub|TestChangeGroomPlanReviseAbstainedStubKeepsMarker|TestChangeGroomPlanReviseSpecNotLinkedMessage'`

Expected: FAIL. The new success tests fail with `unexpected refusal: [... not-revisable ...]`. The converted row `spec-not-linked needs-grooming` fails with `missing refusal code "spec-not-linked"`. The message test fails because the code is `not-revisable`. If any of them fails for a different reason (a compile error, a wrong helper name), fix the test first, because test code a plan supplies is unverified.

- [ ] **Step 4: Widen the gate and reword the messages and comments**

In `changeGroomOp.Plan`, replace the gate block:

```go
	// Groom/revise gate. A proposed change is either needs-design (groomable)
	// or already-groomed (revisable) — the two gates are exact complements, so
	// no proposed change satisfies both and none satisfies neither. Neither
	// gate inspects or sets claim metadata.
	if o.req.Outcome == GroomRevise {
		if c.Status() != domain.StatusProposed || (c.Spec().Value == "" && !c.Trivial()) {
			return refuseGroom("not-revisable",
				fmt.Sprintf("change %04d is not an already-groomed proposed change (status %q, spec %q, trivial %v)",
					o.req.ChangeID, c.Status(), c.Spec().Value, c.Trivial()))
		}
	} else if c.Status() != domain.StatusProposed || c.Spec().Value != "" || c.Trivial() {
```

with:

```go
	// Groom/revise gate. The groom gate admits only a proposed, needs-design
	// change. The revise gate admits any proposed change, needs-design or
	// already groomed: revise never writes spec:, trivial:, or auto_groomable:,
	// so it cannot change a change's groom state, and a needs-design change
	// satisfies both gates. Neither gate inspects or sets claim metadata.
	if o.req.Outcome == GroomRevise {
		if c.Status() != domain.StatusProposed {
			return refuseGroom("not-revisable",
				fmt.Sprintf("change %04d is not a proposed change (status %q)", o.req.ChangeID, c.Status()))
		}
	} else if c.Status() != domain.StatusProposed || c.Spec().Value != "" || c.Trivial() {
```

In the "Revise spec-body decision" block, replace:

```go
			return refuseGroom("spec-not-linked",
				fmt.Sprintf("change %04d has no linked spec to revise (spec_markdown was submitted against a trivial-only change)", o.req.ChangeID))
```

with:

```go
			return refuseGroom("spec-not-linked",
				fmt.Sprintf("change %04d has no linked spec to revise", o.req.ChangeID))
```

Replace the `GroomRevise` doc comment:

```go
	// GroomRevise adjusts an already-groomed proposed change: a whole-body
	// replace of its existing linked spec, owned proposal-section edits, or
	// both. It never writes spec: or trivial:, so a change can never flip
	// between spec'd and trivial through this outcome.
```

with:

```go
	// GroomRevise edits any proposed change without changing its groom state:
	// a whole-body replace of its existing linked spec, owned proposal-section
	// edits, a retitle, relationship-field updates, or a combination. It never
	// writes spec:, trivial:, or auto_groomable:, so a change can never flip
	// between needs-grooming, spec'd, and trivial through this outcome, and it
	// never edits the ## Auto-groom blocked section (abstain and re-enable own it).
```

In the file-header comment, replace:

```go
// spec, or a trivial verdict — or, by the third outcome (revise), adjusts an
// already-groomed proposed change in place: a whole-body replace of its existing
// linked spec and/or owned proposal-section edits, never touching spec: or
// trivial:. The abstain outcome records an autonomous groom's abstain on a
```

with:

```go
// spec, or a trivial verdict — or, by the third outcome (revise), edits any
// proposed change in place without changing its groom state: a whole-body
// replace of its existing linked spec and/or owned proposal-section edits,
// never touching spec:, trivial:, or auto_groomable:. The abstain outcome
// records an autonomous groom's abstain on a
```

and replace the header's closing lines:

```go
// it never touches claim metadata. It decides no lifecycle policy beyond the
// groom gate the spec fixes here (proposed, needs-design, not yet trivial) and
// its exact complement, the revise gate (proposed, already spec'd or trivial).
```

with:

```go
// it never touches claim metadata. It decides no lifecycle policy beyond the
// groom gate the spec fixes here (proposed, needs-design, not yet trivial) and
// the revise gate (any proposed change).
```

If the header's exact line breaks differ from what is shown, keep the meaning: delete "already-groomed" and "exact complement", and state that revise applies to any proposed change and never touches `auto_groomable:`. Reflow so the comment still reads cleanly.

Leave the comment above `revivedReviseFiles` in the test file as it is. Its clause "it is still an already-groomed proposed change" describes that fixture and is still true.

- [ ] **Step 5: Run the package tests and confirm they pass**

Run: `go test -count=1 ./internal/app/ -run 'TestChangeGroom'`
Expected: PASS, including every kept `not-revisable` row and `TestChangeGroomPlanReviseTrivialRationale`.

- [ ] **Step 6: Mutation-check the gate**

```bash
f=/Users/homer/dev/docket/.worktrees/edit-an-ungroomed-stub-through-a-typed-operation/internal/app/change_groom.go
cp "$f" "$f.bak"
# Restore the old gate predicate:
perl -0pi -e 's/if c\.Status\(\) != domain\.StatusProposed \{\n(\t+)return refuseGroom\("not-revisable"/if c.Status() != domain.StatusProposed || (c.Spec().Value == "" \&\& !c.Trivial()) {\n$1return refuseGroom("not-revisable"/' "$f"
diff "$f.bak" "$f"   # must show exactly the gate predicate line changed
go test -count=1 ./internal/app/ -run 'TestChangeGroomPlanReviseNeedsGroomingStub|TestChangeGroomPlanTitleOnlyReviseOfNeedsGroomingStub|TestChangeGroomPlanReviseAbstainedStubKeepsMarker'
mv -f "$f.bak" "$f"
```

Expected: while mutated, the three named tests FAIL with `not-revisable`. Before reading the result, confirm that `diff` shows the predicate change. If it prints nothing, the mutation did not land, and the reading means nothing. After the restore, re-run Step 5 and confirm it passes.

- [ ] **Step 7: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/edit-an-ungroomed-stub-through-a-typed-operation
gofmt -l internal/app
git add internal/app/change_groom.go internal/app/change_groom_test.go
git commit -m "feat(groom): revise edits any proposed change, including needs-grooming stubs"
```

---

### Task 2: Refuse a `revise` section edit naming `## Auto-groom blocked`

**Files:**
- Modify: `internal/app/change_groom.go`: the `case GroomRevise:` arm of `validateChangeGroomShape`
- Test: `internal/app/change_groom_test.go`: `TestChangeGroomReviseShapeValidation`, plus one new test

**Interfaces:**
- Consumes: `autoGroomBlockedHeading` (the existing const, `"## Auto-groom blocked"`), `FCInvalidSectionHeading`, `validReviseRequest`, `hasFindingCode`, `recordingEngine`, `fakeChangeReader`, `mainModePin`, `PlanningDeps`, `ChangeGroom`, `ResultInvalidInput`, `testClock`.
- Produces: a new shape finding, `invalid-section-heading`, on `revise`. The message, verbatim: `abstain and re-enable own "## Auto-groom blocked"; a revise section edit may not name it`.

The guard lives in the request-shape validator, which never sees the change's state. So one refusal covers a stub and a groomed change alike. That is how this plan meets the spec's "on both a stub and a groomed change": `validateChangeGroomShape(req)` runs in `ChangeGroom` before any snapshot is read.

- [ ] **Step 1: Write the failing tests**

Add these rows to the `cases` table in `TestChangeGroomReviseShapeValidation`, before its closing `}`:

```go
		// abstain and re-enable own the abstain marker; a revise that replaced or
		// removed it would desynchronize it from auto_groomable:.
		{"revise replacing ## Auto-groom blocked refused", func(r *ChangeGroomRequest) {
			r.SpecMarkdown, r.SpecRevision = "", ""
			r.Sections = []SectionEditRequest{{Heading: "## Auto-groom blocked", Intent: "replace", Markdown: "x\n"}}
		}, "invalid-section-heading"},
		{"revise removing ## Auto-groom blocked refused", func(r *ChangeGroomRequest) {
			r.SpecMarkdown, r.SpecRevision = "", ""
			r.Sections = []SectionEditRequest{{Heading: "## Auto-groom blocked", Intent: "remove"}}
		}, "invalid-section-heading"},
		{"spec-body revise also naming ## Auto-groom blocked refused", func(r *ChangeGroomRequest) {
			r.Sections = append(r.Sections, SectionEditRequest{Heading: "## Auto-groom blocked", Intent: "remove"})
		}, "invalid-section-heading"},
```

Append this test after `TestChangeGroomReviseShapeValidation`:

```go
// TestChangeGroomReviseAutoGroomBlockedRefusedWithoutEngineCall pins that the
// guard fires at the real entry point before any engine call, so nothing is
// written whatever state the target change is in (stub or groomed).
func TestChangeGroomReviseAutoGroomBlockedRefusedWithoutEngineCall(t *testing.T) {
	req := validReviseRequest()
	req.SpecMarkdown, req.SpecRevision = "", ""
	req.Sections = []SectionEditRequest{{Heading: "## Auto-groom blocked", Intent: "remove"}}
	engine := &recordingEngine{}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
	deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeGroom(context.Background(), deps, "", req)

	if res.Result != ResultInvalidInput {
		t.Fatalf("result = %q, want invalid-input", res.Result)
	}
	if len(engine.calls) != 0 {
		t.Errorf("engine called %d times on a shape failure, want 0", len(engine.calls))
	}
	if !hasFindingCode(res.Findings, "invalid-section-heading") {
		t.Errorf("missing invalid-section-heading; got %v", res.Findings)
	}
}
```

- [ ] **Step 2: Run them and confirm they fail**

Run: `go test -count=1 ./internal/app/ -run 'TestChangeGroomReviseShapeValidation|TestChangeGroomReviseAutoGroomBlockedRefusedWithoutEngineCall'`
Expected: FAIL. The three new rows report `missing finding "invalid-section-heading"`. The new test fails on `result = ...`, because today the request passes shape validation and reaches the engine.

- [ ] **Step 3: Add the guard**

In `validateChangeGroomShape`, change the `case GroomRevise:` arm to:

```go
	case GroomRevise:
		if strings.TrimSpace(req.SpecMarkdown) != "" {
			if msg := specMarkdownShapeProblem(req.SpecMarkdown); msg != "" {
				addShape(FCInvalidSpecMarkdown, msg)
			}
		} else if !hasEffectiveSectionEdit(req.Sections) && req.Title == "" {
			addShape(FCEmptyRevise, "the revise outcome requires a non-empty spec_markdown, at least one replace/remove section edit, or a title")
		}
		for _, s := range req.Sections {
			if s.Heading == autoGroomBlockedHeading {
				addShape(FCInvalidSectionHeading, "abstain and re-enable own \"## Auto-groom blocked\"; a revise section edit may not name it")
			}
		}
```

Also extend the `autoGroomBlockedHeading` const comment so its owners are explicit:

```go
// autoGroomBlockedHeading is the abstain marker section: the abstain
// outcome appends to it, the re-enable outcome removes it, and a section
// edit naming it is refused on revise and re-enable. The board's
// "auto-groom blocked — needs you" cell keys on its presence
// (domain.ReadyAutoGroomBlocked).
```

- [ ] **Step 4: Run and confirm they pass**

Run: `go test -count=1 ./internal/app/ -run 'TestChangeGroom'`
Expected: PASS.

- [ ] **Step 5: Mutation-check the guard**

```bash
f=/Users/homer/dev/docket/.worktrees/edit-an-ungroomed-stub-through-a-typed-operation/internal/app/change_groom.go
cp "$f" "$f.bak"
# Neutralize only the revise check (the re-enable check carries a different message):
perl -0pi -e 's/if s\.Heading == autoGroomBlockedHeading \{\n(\t+)addShape\(FCInvalidSectionHeading, "abstain and re-enable own/if false \&\& s.Heading == autoGroomBlockedHeading {\n$1addShape(FCInvalidSectionHeading, "abstain and re-enable own/' "$f"
diff "$f.bak" "$f"   # must show exactly the one mutated line
go test -count=1 ./internal/app/ -run 'TestChangeGroomReviseShapeValidation|TestChangeGroomReviseAutoGroomBlockedRefusedWithoutEngineCall|TestChangeGroomReEnableShapeValidation'
mv -f "$f.bak" "$f"
```

Expected: the three new shape rows and the new entry-point test FAIL. `TestChangeGroomReEnableShapeValidation` still PASSES, which proves the mutation hit only the revise arm. Re-run Step 4 after the restore.

- [ ] **Step 6: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/edit-an-ungroomed-stub-through-a-typed-operation
gofmt -l internal/app
git add internal/app/change_groom.go internal/app/change_groom_test.go
git commit -m "feat(groom): refuse a revise section edit naming ## Auto-groom blocked"
```

---

### Task 3: Skill, glossary, and CLI help wording, with a prose guard

**Files:**
- Modify: `skills/docket-groom-next/SKILL.md` (authored source; four edits, all inside existing lines, so the line count stays at 78)
- Regenerate: `internal/assets/embedded/` via `go generate ./internal/assets/`
- Modify: `internal/repoguard/budgets_test.go`: the `{"docket-groom-next/SKILL.md", 78, 2082}` row
- Modify: `internal/repoguard/prose_contracts_test.go`: add one row after the `change_0461_retitle` row
- Modify: `docs/reference/glossary.md`: the `### Groom` paragraph, the `### Groom outcome \`revise\`` entry, and the `docket-groom-next` entry's **Used for:** line
- Modify: `internal/cli/change.go`: the `changeSubcommand("change", "groom", ...)` short help

**Interfaces:**
- Consumes: Tasks 1–2 behavior (the refusal codes `not-revisable`, `spec-not-linked`, `invalid-section-heading`).
- Produces: the prose-contract row `change_0509_stub_revise`, whose present/absent phrases are listed in Step 1.

- [ ] **Step 1: Write the failing prose guard**

In `internal/repoguard/prose_contracts_test.go`, add this row right after the `change_0461_retitle` row:

```go
	// change 0509 — revise edits any proposed change without changing its groom
	// state; a request to edit (not groom) a needs-grooming stub goes straight
	// to the revise exit with no brainstorm. The absent phrases are the retired
	// already-groomed-only claims.
	{sentinel: "change_0509_stub_revise", file: "skills/docket-groom-next/SKILL.md",
		present: []string{"skip the brainstorm and apply Step 4's revise exit directly", "the stub stays needs-grooming",
			"The change keeps its groom state."},
		absent: []string{"The change stays build-ready.", "the change is already groomed and the human wants it adjusted",
			"no longer an already-groomed `proposed` change", "a revise keeps the row build-ready"}},
```

Run: `go test -count=1 ./internal/repoguard/ -run TestProseContracts`
Expected: FAIL, listing the three missing present phrases and the four present forbidden phrases.

- [ ] **Step 2: Edit `skills/docket-groom-next/SKILL.md`**

Make exactly these four in-line edits. Add no new lines.

(a) In the `## When to use` bullet that begins "An explicit id naming an already-groomed `proposed` change", insert this sentence right after "...through `change.groom` with `outcome: revise`.":

```
 An explicit id with a request to *edit* (not groom) a needs-grooming stub routes there too.
```

(b) In `### Step 1 — Select`, right after the sentence ending "...and exit via Step 4's revise exit.", insert:

```
 When the human asks to *edit* a needs-grooming stub rather than groom it, skip the brainstorm and apply Step 4's revise exit directly with the requested owned-section, `title`, or relationship edits — the stub stays needs-grooming.
```

(c) In Step 4 exit 5, replace:

```
5. **Revise** (explicit-id route only): the change is already groomed and the human wants it adjusted — apply
```

with:

```
5. **Revise** (explicit-id route only): the human wants a `proposed` change adjusted without changing its groom state — apply
```

In the same exit, replace:

```
A typed refusal (`not-revisable`, `spec-not-linked`, `spec-file-missing`, `empty-revise`, `empty-spec_revision`, revision mismatch) writes nothing — surface it. The change stays build-ready.
```

with:

```
A typed refusal (`not-revisable`, `spec-not-linked` — also the answer to `spec_markdown` on a needs-grooming stub, which has no spec — `spec-file-missing`, `empty-revise`, `empty-spec_revision`, `invalid-section-heading` for an edit naming `## Auto-groom blocked`, revision mismatch) writes nothing — surface it. The change keeps its groom state.
```

(d) In `### Step 5 — The transaction lands`, replace `a revise keeps the row build-ready` with `a revise keeps the row's readiness`, and replace `stop only if the change is no longer an already-groomed `proposed` change` with `stop only if the change is no longer `proposed``.

Leave the frontmatter `description:` unchanged. The `docket-new-change` skill stays unchanged too (Global Constraints).

Confirm: `wc -l skills/docket-groom-next/SKILL.md` still prints `78`. Re-read each edited sentence as a worker in an unknown repo (the `distributed-body-has-no-local-repo` learning). None of them names this repo.

- [ ] **Step 3: Raise the word ceiling to the measured count**

Run: `wc -w skills/docket-groom-next/SKILL.md`. Call the result `N`. In `internal/repoguard/budgets_test.go`, change the row `{"docket-groom-next/SKILL.md", 78, 2082},` to `{"docket-groom-next/SKILL.md", 78, N},` with `N` the measured number, and prepend to that row's trailing comment:

```
// 0509: +edit-a-stub route to the revise exit (When to use, Step 1) and groom-state-neutral revise wording in Step 4/5 (word ceiling 2082 -> N);
```

so the comment reads `// 0509: ... (word ceiling 2082 -> N); 0461: ...` with the existing history after it. Keep the line ceiling at 78.

- [ ] **Step 4: Regenerate the embedded tree**

Run: `cd /Users/homer/dev/docket/.worktrees/edit-an-ungroomed-stub-through-a-typed-operation && go generate ./internal/assets/`
Then: `git status --porcelain` must show `internal/assets/embedded/tree/skills/docket-groom-next/SKILL.md` and the embedded manifest as modified, and nothing outside the intended paths. Confirm `diff skills/docket-groom-next/SKILL.md internal/assets/embedded/tree/skills/docket-groom-next/SKILL.md` prints nothing.

- [ ] **Step 5: Update the glossary**

In `docs/reference/glossary.md`, keep the hard wrap at about 100 columns.

In `### Groom`, replace:

```
Taking a stub through design to build-ready. Exits: a linked spec, a trivial verdict, a kill, a
defer, a revise of an already-groomed change, or (autonomous only) an abstain.
```

with:

```
Taking a stub through design to build-ready. Exits: a linked spec, a trivial verdict, a kill, a
defer, a revise of a `proposed` change (it keeps its groom state), or (autonomous only) an abstain.
```

Replace the body of `### Groom outcome \`revise\`` up to (not including) its `**Used for:**` line:

```
The `change.groom` outcome that adjusts a change that is already groomed (`proposed` with a spec or
`trivial: true`). It can replace the linked spec's body and edit owned sections. It never sets
`spec:` or `trivial:`, so a change cannot flip between spec'd and trivial.
```

with:

```
The `change.groom` outcome that edits any `proposed` change without changing its groom state: a
needs-grooming stub stays needs-grooming, and a spec'd or trivial change stays build-ready. It can
replace the linked spec's body (only when the change links a spec) and edit owned sections, the
title, and relationship fields. It never sets `spec:`, `trivial:`, or `auto_groomable:`, and it
never edits `## Auto-groom blocked`, which only abstain and re-enable write.
```

In the same entry's **Used for:** line, replace `**Used for:** fixing a just-landed design.` with `**Used for:** fixing a just-landed design, or sharpening a stub's Why or title without grooming it.`

In the `docket-groom-next` entry, replace:

```
**Used for:** designing stubs together with you. Naming an already-groomed id routes to
`revise`.
```

with:

```
**Used for:** designing stubs together with you. Naming an already-groomed id, or asking to edit a
stub rather than groom it, routes to `revise`.
```

(Reflow the rest of that paragraph, "It runs inline at the session model...", so the wrap stays tidy.)

- [ ] **Step 6: Update the CLI short help**

In `internal/cli/change.go`, replace:

```go
		"Groom a proposed change to build-ready (spec or trivial), record or clear an auto-groom abstain (abstain, re-enable), or revise an already-groomed one, from a JSON request",
```

with:

```go
		"Groom a proposed change to build-ready (spec or trivial), record or clear an auto-groom abstain (abstain, re-enable), or revise any proposed change without changing its groom state, from a JSON request",
```

- [ ] **Step 7: Run the guards and confirm they pass**

Run: `go test -count=1 ./internal/repoguard/ ./internal/assets/ ./internal/cli/`
Expected: PASS. Specifically `TestProseContracts`, the skill budget test, `TestEmbeddedMatchesAuthored`, and `TestCommentAnchorStyle`.

- [ ] **Step 8: Mutation-check the prose row**

```bash
f=/Users/homer/dev/docket/.worktrees/edit-an-ungroomed-stub-through-a-typed-operation/skills/docket-groom-next/SKILL.md
cp "$f" "$f.bak"
perl -pi -e 's/ When the human asks to \*edit\* a needs-grooming stub rather than groom it, skip the brainstorm and apply Step 4.s revise exit directly with the requested owned-section, `title`, or relationship edits — the stub stays needs-grooming\.//' "$f"
diff "$f.bak" "$f"   # must show the Step 1 line changed
go test -count=1 ./internal/repoguard/ -run TestProseContracts
mv -f "$f.bak" "$f"
```

Expected: FAIL naming `skip the brainstorm and apply Step 4's revise exit directly` and `the stub stays needs-grooming`. Re-run Step 7 after the restore.

- [ ] **Step 9: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/edit-an-ungroomed-stub-through-a-typed-operation
gofmt -l internal/repoguard internal/cli
git add skills/docket-groom-next/SKILL.md internal/assets/embedded internal/repoguard/budgets_test.go internal/repoguard/prose_contracts_test.go docs/reference/glossary.md internal/cli/change.go
git commit -m "docs(groom): route stub edits to revise; revise keeps the groom state"
```

---

### Task 4: Whole-suite gate

**Files:** none (verification only)

- [ ] **Step 1: Run the whole suite from source**

Run: `cd /Users/homer/dev/docket/.worktrees/edit-an-ungroomed-stub-through-a-typed-operation && go run ./cmd/docket development test`
Expected: green. Read the budget report even on a green run. A `BUDGET WATCH:` or `PARALLEL-SENSITIVE:` line is a screening finding to record. A `SERIAL CONFIRMED OVER BUDGET:` line is a real breach to act on.

- [ ] **Step 2: Final sweep for stale claims**

Run: `git -C /Users/homer/dev/docket/.worktrees/edit-an-ungroomed-stub-through-a-typed-operation grep -n -i -E "already-groomed proposed change \(status|exact complement|trivial-only change\)" -- internal skills docs/reference`
Expected: no output. Matches under `docs/results/`, `docs/superpowers/`, or `docs/changes/` are point-in-time records and stay as written.
