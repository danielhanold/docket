<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0550 — Let change.groom set a change's priority](../../changes/archive/2026-10-15-0550-let-change-groom-set-a-change-s-priority.md)**
<!-- docket:backlink:end -->

# Let change.groom set a change's priority — Implementation Plan

> **For agentic workers:** this plan is executed by `docket-build` (one tier agent per task under the `docket-build-task` contract, then one full-suite gate). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `change.groom` accepts an optional `priority` field that rewrites the record's `priority:` line, re-rendering the board and `## Artifacts` block, all in the same atomic metadata commit as the rest of the groom.

**Architecture:** Follow the `title` (retitle, change 0461) precedent in `internal/app/change_groom.go` exactly. Add one request field. Add shape checks in `validateChangeGroomShape`: `ParsePriority` membership, the abstain refusal, and widening the `empty-revise` minimum. Add one `upsertField` call in the first patch pass of `changeGroomOp.Plan`. The candidate snapshot already drives the artifact block and the inline board, so the new priority reaches both without further code. After that, update the docs: the `docket-groom-next` skill paragraph and its embedded copy, the prose-contract sentinel, the skill size budget, and the glossary.

**Tech Stack:** Go (`internal/app`, `internal/repoguard`), Markdown skill files, `go generate ./internal/assets/`.

**Spec:** `docs/superpowers/specs/2026-10-14-let-change-groom-set-a-change-s-priority-design.md` (on the `docket` metadata branch; in the primary checkout it is at `/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-10-14-let-change-groom-set-a-change-s-priority-design.md`).

## Global Constraints

- The new request field is `ChangeGroomRequest.Priority string`, with JSON key `priority`, `omitempty`, tagged `docket:"enum=priorities"`. Empty means unchanged.
- Unknown value → existing finding code `unknown-priority` (`FCUnknownPriority`), message exactly `unknown priority %q; valid values are critical, high, medium, low`. The match is case-sensitive (`domain.ParsePriority`), checked at the shape stage before any engine call or write.
- Abstain with any non-empty `priority` → refused. Use a new code `invalid-priority` (`FCInvalidPriority`), which mirrors how abstain refuses `title` with `invalid-title`. It is distinct from `unknown-priority`. On abstain, do **not** also run the `ParsePriority` check (the `title` check skips `validateTitle` on abstain in the same way).
- `revise` minimum input becomes: non-empty `spec_markdown`, OR a replace/remove section edit, OR a `title`, OR a `priority`. The `empty-revise` message must name `priority`.
- Write: `upsertField(&ps, doc1, "priority", document.String(o.req.Priority))` in the first patch pass, next to the retitle write. `updated:` is bumped as for every groom. Nothing else changes: `spec:`, `trivial:`, `auto_groomable:`, slug, record path, spec path, and branch stay as they are.
- Re-sending the current priority is not an error.
- `change.groom` keeps refusing non-`proposed` changes. There is no new gate and no `change.set-priority` command.
- `docket-auto-groom` instructions are NOT changed. Autonomous grooming never re-prioritizes.
- Run `go generate ./internal/assets/` after editing any `skills/**` file, so the embedded copy matches.
- Comment cross-references anchor on symbol names or quoted clauses, never line numbers (ADR-0054, `TestCommentAnchorStyle`).
- Mutation checks run with `-count=1` (the Go test cache can serve a stale PASS). Back up a file before mutating it (`cp f f.bak` … `mv -f f.bak f`), never restore with `git checkout --`, which would discard the uncommitted work under test. Before trusting a mutation result, confirm the mutation actually landed (`/usr/bin/grep -cF`).
- The build gate is `build.test_command` = `go run ./cmd/docket development test` (from `.docket.yml`), run on the whole suite by `docket-build` at the end. Each task runs only its focused `go test`.

## Review Focus

1. **Case and whitespace variants** (`High`, `HIGH`, ` high`, `high `, `urgent`): each is refused with `unknown-priority` at the shape stage and the engine is never called. People type `High`. Pinned in Task 1 Step 1.
2. **Abstain carrying a *valid* priority, and abstain carrying a *bogus* priority:** both yield `invalid-priority` and never `unknown-priority`, so an autonomous caller gets one clear "abstain cannot re-rank" diagnostic. Pinned in Task 1 Step 1.
3. **Priority combined with `title` and a section edit in one revise:** all three land in the single record write, and the board row shows both the new title and the new priority. Pinned in Task 2 Step 1.
4. **Re-sending the current priority over an already-settled tree:** the plan is empty (the engine's clean no-op), and the delta verifier never fails on an unchanged declared path. Pinned in Task 2 Step 1.
5. **A priority-only revise of a non-`proposed` change** (in-progress, deferred): refused with `not-revisable`, and nothing is written. A deferred change must be revived first. Pinned in Task 2 Step 1.

---

### Task 1: Request field, shape validation, and the `invalid-priority` code

**Files:**
- Modify: `internal/app/change_groom.go` (struct `ChangeGroomRequest`; func `validateChangeGroomShape`)
- Modify: `internal/app/finding_codes.go` (const block near `FCInvalidTitle`; `AllFindingCodes`)
- Test: `internal/app/change_groom_test.go` (append new tests at end of file)
- Test: `internal/app/schema_test.go` (func `TestReflectDescriptorChangeGroomRequest`)
- Test: `internal/app/finding_codes_test.go` (func `TestShapeValidatorCodesAreRegistered`)

**Interfaces:**
- Produces: `ChangeGroomRequest.Priority string` (JSON `priority`). `FCInvalidPriority FindingCode = "invalid-priority"`. A test helper `priorityOnlyReviseRequest(priority string) ChangeGroomRequest`, used by Task 2.

- [ ] **Step 1: Write the failing tests**

Append to `internal/app/change_groom_test.go`:

```go
// priorityOnlyReviseRequest is a revise carrying nothing but a priority — no
// spec body, no spec_revision, no section edits, no title (change 0550).
func priorityOnlyReviseRequest(priority string) ChangeGroomRequest {
	r := validReviseRequest()
	r.SpecMarkdown, r.SpecRevision, r.Sections = "", "", nil
	r.Priority = priority
	return r
}

func TestChangeGroomPriorityShapeValidation(t *testing.T) {
	withPriority := func(r ChangeGroomRequest, p string) ChangeGroomRequest {
		r.Priority = p
		return r
	}
	cases := []struct {
		name string
		req  ChangeGroomRequest
		code string // "" means the request must pass shape validation
		not  string // a code that must NOT be emitted ("" = no check)
	}{
		{"spec accepts priority", withPriority(validGroomSpecRequest(), "high"), "", ""},
		{"trivial accepts priority", withPriority(titledTrivialRequest("Renamed widget"), "low"), "", ""},
		{"re-enable accepts priority", withPriority(reEnableRequest(), "critical"), "", ""},
		{"revise accepts a priority alone", priorityOnlyReviseRequest("high"), "", ""},
		{"revise accepts priority with sections", withPriority(validReviseRequest(), "medium"), "", ""},
		{"unknown word", priorityOnlyReviseRequest("urgent"), "unknown-priority", ""},
		{"capitalized", priorityOnlyReviseRequest("High"), "unknown-priority", ""},
		{"upper-case", priorityOnlyReviseRequest("HIGH"), "unknown-priority", ""},
		{"leading space", priorityOnlyReviseRequest(" high"), "unknown-priority", ""},
		{"trailing space", priorityOnlyReviseRequest("high "), "unknown-priority", ""},
		{"unknown on spec outcome", withPriority(validGroomSpecRequest(), "urgent"), "unknown-priority", ""},
		{"abstain refuses a valid priority", withPriority(abstainRequest(), "high"), "invalid-priority", "unknown-priority"},
		{"abstain refuses a bogus priority once", withPriority(abstainRequest(), "urgent"), "invalid-priority", "unknown-priority"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			findings := validateChangeGroomShape(c.req)
			if c.code == "" {
				if len(findings) != 0 {
					t.Fatalf("unexpected shape findings: %v", findings)
				}
				return
			}
			if !hasFindingCode(findings, c.code) {
				t.Errorf("missing finding %q; got %v", c.code, findings)
			}
			if c.not != "" && hasFindingCode(findings, c.not) {
				t.Errorf("unexpected finding %q alongside %q; got %v", c.not, c.code, findings)
			}
		})
	}
}

// TestChangeGroomUnknownPriorityMessageMatchesCreate pins the shared wording
// with change.create's unknown-priority finding.
func TestChangeGroomUnknownPriorityMessageMatchesCreate(t *testing.T) {
	findings := validateChangeGroomShape(priorityOnlyReviseRequest("urgent"))
	want := `unknown priority "urgent"; valid values are critical, high, medium, low`
	for _, f := range findings {
		if f.Code == string(FCUnknownPriority) {
			if f.Message != want {
				t.Errorf("message = %q, want %q", f.Message, want)
			}
			return
		}
	}
	t.Fatalf("no unknown-priority finding; got %v", findings)
}

// TestChangeGroomEmptyReviseNamesPriority pins the widened empty-revise rule:
// with no spec_markdown, no effective section edit, no title, and no priority
// the revise is refused, and the diagnostic names priority too.
func TestChangeGroomEmptyReviseNamesPriority(t *testing.T) {
	findings := validateChangeGroomShape(priorityOnlyReviseRequest(""))
	var msg string
	for _, f := range findings {
		if f.Code == string(FCEmptyRevise) {
			msg = f.Message
		}
	}
	if msg == "" {
		t.Fatalf("empty revise not refused; findings %v", findings)
	}
	for _, want := range []string{"spec_markdown", "section edit", "title", "priority"} {
		if !strings.Contains(msg, want) {
			t.Errorf("empty-revise message %q does not name %q", msg, want)
		}
	}
}

func TestChangeGroomBadPriorityRefusedWithoutEngineCall(t *testing.T) {
	cases := []struct {
		name string
		req  ChangeGroomRequest
		code string
	}{
		{"unknown priority", priorityOnlyReviseRequest("urgent"), "unknown-priority"},
		{"abstain with priority", func() ChangeGroomRequest { r := abstainRequest(); r.Priority = "high"; return r }(), "invalid-priority"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			engine := &recordingEngine{}
			reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
			deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

			res := ChangeGroom(context.Background(), deps, "", c.req)

			if res.Result != ResultInvalidInput {
				t.Fatalf("result = %q, want invalid-input", res.Result)
			}
			if len(engine.calls) != 0 {
				t.Errorf("engine called %d times on a shape failure, want 0 (nothing may be written)", len(engine.calls))
			}
			if !hasFindingCode(res.Findings, c.code) {
				t.Errorf("missing %q; got %v", c.code, res.Findings)
			}
		})
	}
}
```

In `internal/app/schema_test.go`, inside `TestReflectDescriptorChangeGroomRequest`, right after the existing `title` assertion, add:

```go
	// priority (change 0550): optional, drawn from the same closed vocabulary
	// change.create's priority names.
	if f := fieldByKey(t, d, "priority"); f.Required || f.Type != "string" || f.Enum != "priorities" {
		t.Errorf("priority = %+v, want optional string with enum priorities", f)
	}
```

In `internal/app/finding_codes_test.go`, inside `TestShapeValidatorCodesAreRegistered`, after the line `emitted = append(emitted, validateChangeGroomShape(ChangeGroomRequest{Outcome: GroomRevise, Title: "a\nb"})...)`, add:

```go
	emitted = append(emitted, validateChangeGroomShape(ChangeGroomRequest{Outcome: GroomAbstain, Priority: "high"})...)
	emitted = append(emitted, validateChangeGroomShape(ChangeGroomRequest{Outcome: GroomRevise, Priority: "urgent"})...)
```

and add `FCInvalidPriority, FCUnknownPriority,` to the `floor` slice (for example, at the end of the `FCInvalidPRNumber, FCInvalidAttempt, FCEmptyHead,` line).

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/homer/dev/docket/.worktrees/let-change-groom-set-a-change-s-priority && go test -count=1 ./internal/app/ -run 'TestChangeGroomPriority|TestChangeGroomUnknownPriority|TestChangeGroomEmptyReviseNamesPriority|TestChangeGroomBadPriority|TestReflectDescriptorChangeGroomRequest|TestShapeValidatorCodesAreRegistered'`
Expected: compile FAIL — `r.Priority undefined (type ChangeGroomRequest has no field or method Priority)` and `undefined: FCInvalidPriority`.

- [ ] **Step 3: Implement**

In `internal/app/finding_codes.go`, add the constant directly after `FCInvalidTitle              FindingCode = "invalid-title"` in the change-create / groom request-shape block (gofmt will realign):

```go
	FCInvalidPriority           FindingCode = "invalid-priority"
```

Add it to `AllFindingCodes` in sorted string order. `invalid-pr_number` < `invalid-priority` < `invalid-related` (because `_` < `i` and `p` < `r`), so the entry goes between `FCInvalidPRNumber,` and `FCInvalidRelated,`:

```go
	FCInvalidPRNumber,
	FCInvalidPriority,
	FCInvalidRelated,
```

(`TestFindingCodeRegistryIntegrity` in `finding_codes_test.go` enforces this order.)

In `internal/app/change_groom.go`, add the field to `ChangeGroomRequest` directly after `Title`:

```go
	// Priority, when non-empty, re-prioritizes the change (change 0550). The
	// spec, trivial, revise, and re-enable outcomes accept it; abstain refuses
	// it, since an autonomous abstain cannot re-rank the proposal. Empty leaves
	// the priority unchanged. It draws on the same closed vocabulary as
	// change.create's priority.
	Priority string `json:"priority,omitempty" docket:"enum=priorities"`
```

In `validateChangeGroomShape`:

1. In the `case GroomRevise:` arm, widen the empty-revise condition and message:

```go
		} else if !hasEffectiveSectionEdit(req.Sections) && req.Title == "" && req.Priority == "" {
			addShape(FCEmptyRevise, "the revise outcome requires a non-empty spec_markdown, at least one replace/remove section edit, a title, or a priority")
		}
```

2. In the `case GroomAbstain:` arm, directly after the `if req.Title != "" { addShape(FCInvalidTitle, …) }` block:

```go
		if req.Priority != "" {
			addShape(FCInvalidPriority, "priority is not accepted by the abstain outcome; an abstain cannot re-rank the proposal")
		}
```

3. Directly after the shared title block (`if req.Title != "" && req.Outcome != GroomAbstain { … validateTitle … }`), add:

```go
	// A priority re-prioritizes the change on every other outcome (change 0550);
	// an empty one means "unchanged". The match is case-sensitive and uses the
	// same message as change.create's unknown-priority finding.
	if req.Priority != "" && req.Outcome != GroomAbstain {
		if _, ok := domain.ParsePriority(req.Priority); !ok {
			addShape(FCUnknownPriority, fmt.Sprintf("unknown priority %q; valid values are critical, high, medium, low", req.Priority))
		}
	}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/homer/dev/docket/.worktrees/let-change-groom-set-a-change-s-priority && go test -count=1 ./internal/app/ -run 'TestChangeGroom|TestReflectDescriptor|TestShapeValidatorCodesAreRegistered|TestFindingCodeRegistryIntegrity|TestVocabularyConstCompleteness|TestNoInlineFindingCodeLiterals|TestNestedRequiredTagMatchesValidator'`
Expected: PASS. If a schema golden or registry test elsewhere in `./internal/app/` fails because the `change.groom` request descriptor gained a field, update that expectation to include `priority` (enum `priorities`) and re-run `go test -count=1 ./internal/app/`.

- [ ] **Step 5: Mutation-check the two guards**

1. Back up the file: `cp internal/app/change_groom.go /tmp/cg.bak`. Delete the `if _, ok := domain.ParsePriority(req.Priority); !ok { … }` block, leaving the outer `if` with an empty body or removing it entirely. Confirm the mutation landed: `/usr/bin/grep -cF 'domain.ParsePriority(req.Priority)' internal/app/change_groom.go` must print `0`. Run `go test -count=1 ./internal/app/ -run 'TestChangeGroomPriorityShapeValidation|TestChangeGroomBadPriority'`. Expected: FAIL on the `unknown word` / `capitalized` / `unknown priority` cases. Restore with `mv -f /tmp/cg.bak internal/app/change_groom.go`.
2. Back up again. Change `&& req.Title == "" && req.Priority == ""` back to `&& req.Title == ""`. Confirm with `/usr/bin/grep -cF 'req.Priority == ""' internal/app/change_groom.go` → `0`. Run `go test -count=1 ./internal/app/ -run 'TestChangeGroomPriorityShapeValidation'`. Expected: FAIL on `revise accepts a priority alone` (`empty-revise`). Restore with `mv -f`.
3. Re-run Step 4's command. Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git -C /Users/homer/dev/docket/.worktrees/let-change-groom-set-a-change-s-priority add internal/app/change_groom.go internal/app/finding_codes.go internal/app/change_groom_test.go internal/app/schema_test.go internal/app/finding_codes_test.go
git -C /Users/homer/dev/docket/.worktrees/let-change-groom-set-a-change-s-priority commit -m "feat(groom): accept and validate an optional priority on change.groom"
```

---

### Task 2: Write `priority:` in the groom transaction

**Files:**
- Modify: `internal/app/change_groom.go` (func `changeGroomOp.Plan`, first patch pass; doc comment on `changeGroomOp.Plan` if it lists the typed fields)
- Test: `internal/app/change_groom_test.go` (append)

**Interfaces:**
- Consumes: `ChangeGroomRequest.Priority` and `priorityOnlyReviseRequest(priority string) ChangeGroomRequest` from Task 1. Existing helpers: `groomPlanFor`, `baseGroomOp`, `groomedRecordBytes`, `assertPlanPaths`, `planPaths`, `groomableChange`, `abstainedChange`, `optedOutStub`, `reviseFixtureFiles`, `reviseFixtureAtStatus`, `reviseSettledFiles`, `validGroomSpecRequest`, `reEnableRequest`, `validReviseRequest`, `groomPath`, constants `reviseClaimFields`, `reviseSpecPath`.
- Produces: nothing new for later tasks.

Fixture notes: `groomableChange` stores `priority: medium` (unquoted) and `title: 'A change'`, type `feat`. The document writer single-quotes every string scalar it writes, as the existing title and updated asserts show (`title: 'Renamed widget'`, `updated: '2026-08-16'`). A written priority therefore reads `priority: 'high'`. Proposed and groomed board rows render as `| [0002](active/0002-add-a-widget.md) | <title> | \`<priority>\` | \`feat\` | …` (see `boardSectionRow` in `internal/render/board.go`).

- [ ] **Step 1: Write the failing tests**

Append to `internal/app/change_groom_test.go`:

```go
// assertPriorityLanded pins that the record carries the new priority (and not
// the old medium) and that the inline board row shows it.
func assertPriorityLanded(t *testing.T, plan transaction.MutationPlan, want string) {
	t.Helper()
	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	if !strings.Contains(rec, "\npriority: '"+want+"'\n") {
		t.Errorf("record priority not rewritten to %q:\n%s", want, rec)
	}
	if strings.Contains(rec, "\npriority: medium\n") {
		t.Errorf("old priority survived:\n%s", rec)
	}
	board := string(groomedRecordBytes(t, plan, "docs/changes/BOARD.md"))
	if !strings.Contains(board, "| A change | `"+want+"` |") {
		t.Errorf("board row does not show priority %q:\n%s", want, board)
	}
	if strings.Contains(board, "| A change | `medium` |") {
		t.Errorf("board row still shows the old priority:\n%s", board)
	}
}

// TestChangeGroomPlanPriorityLandsOnEveryAcceptingOutcome pins spec test 1:
// spec, trivial, revise, and re-enable each write the new priority to the
// record and the board row in the same plan.
func TestChangeGroomPlanPriorityLandsOnEveryAcceptingOutcome(t *testing.T) {
	trivial := ChangeGroomRequest{
		ChangeID: 2, Path: groomPath(2, "add-a-widget"),
		Revision: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Outcome: GroomTrivial,
		Sections: []SectionEditRequest{{Heading: "## Why", Intent: "replace", Markdown: "Too small to design.\n"}},
	}
	cases := []struct {
		name string
		rec  string
		req  ChangeGroomRequest
	}{
		{"spec", groomableChange(2, "add-a-widget"), validGroomSpecRequest()},
		{"trivial", groomableChange(2, "add-a-widget"), trivial},
		{"revise", groomableChange(2, "add-a-widget"), priorityOnlyReviseRequest("")},
		{"re-enable", abstainedChange(2, "add-a-widget"), reEnableRequest()},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := c.req
			req.Priority = "high"
			files := map[string]string{
				groomPath(2, "add-a-widget"): c.rec,
				"docs/changes/BOARD.md":      "# Backlog\n\nold\n",
			}
			plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{"inline"}, req))
			if opRes.Refused {
				t.Fatalf("unexpected refusal: %v", opRes.Findings)
			}
			assertPriorityLanded(t, plan, "high")
			if rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget"))); !strings.Contains(rec, "updated: '2026-08-16'") {
				t.Errorf("updated not bumped:\n%s", rec)
			}
		})
	}
}

// TestChangeGroomPlanPriorityOnlyReviseKeepsGroomState pins spec test 2: a
// revise carrying only a priority passes shape validation (the widened
// empty-revise rule) and changes no groom state.
func TestChangeGroomPlanPriorityOnlyReviseKeepsGroomState(t *testing.T) {
	req := priorityOnlyReviseRequest("critical")
	if findings := validateChangeGroomShape(req); len(findings) != 0 {
		t.Fatalf("priority-only revise refused at the shape stage: %v", findings)
	}
	files := map[string]string{
		groomPath(2, "add-a-widget"): optedOutStub(2, "add-a-widget"),
		"docs/changes/BOARD.md":      "# Backlog\n\nold\n",
	}
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{"inline"}, req))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		groomPath(2, "add-a-widget"): transaction.MutationReplace,
		"docs/changes/BOARD.md":      transaction.MutationReplace,
	})
	assertPriorityLanded(t, plan, "critical")
	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	for _, want := range []string{"\nspec:\n", "trivial: false", "auto_groomable: false", "title: 'A change'", "slug: add-a-widget", "Original why."} {
		if !strings.Contains(rec, want) {
			t.Errorf("priority-only revise changed more than priority; missing %q:\n%s", want, rec)
		}
	}
	board := string(groomedRecordBytes(t, plan, "docs/changes/BOARD.md"))
	if !strings.Contains(board, "needs-grooming") {
		t.Errorf("board row is no longer needs-grooming:\n%s", board)
	}
}

// TestChangeGroomPlanOmittedPriorityLeavesFieldByteIdentical pins spec test 3:
// a groom without priority never touches the stored priority: line (the
// fixture stores it unquoted, so any rewrite would re-quote it).
func TestChangeGroomPlanOmittedPriorityLeavesFieldByteIdentical(t *testing.T) {
	files := map[string]string{groomPath(2, "add-a-widget"): groomableChange(2, "add-a-widget")}
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{}, stubSectionReviseRequest()))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	// Non-vacuity: this is the groomed record (the section edit landed).
	if !strings.Contains(rec, "Sharpened why.") {
		t.Fatalf("not reading the groomed record:\n%s", rec)
	}
	if !strings.Contains(rec, "\npriority: medium\n") || strings.Contains(rec, "priority: 'medium'") {
		t.Errorf("priority: line not byte-identical when priority is omitted:\n%s", rec)
	}
}

// TestChangeGroomPlanPriorityWithTitleAndSections pins Review Focus 3: one
// revise carrying a title, a priority, and a section edit lands all three in
// the single record write and the board row.
func TestChangeGroomPlanPriorityWithTitleAndSections(t *testing.T) {
	req := stubSectionReviseRequest()
	req.Title = "Renamed widget"
	req.Priority = "low"
	files := map[string]string{
		groomPath(2, "add-a-widget"): groomableChange(2, "add-a-widget"),
		"docs/changes/BOARD.md":      "# Backlog\n\nold\n",
	}
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{"inline"}, req))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	for _, want := range []string{"title: 'Renamed widget'", "\npriority: 'low'\n", "Sharpened why."} {
		if !strings.Contains(rec, want) {
			t.Errorf("record missing %q:\n%s", want, rec)
		}
	}
	board := string(groomedRecordBytes(t, plan, "docs/changes/BOARD.md"))
	if !strings.Contains(board, "| Renamed widget | `low` |") {
		t.Errorf("board row lacks the new title and priority together:\n%s", board)
	}
}

// TestChangeGroomPlanSamePriorityIsNoOp pins Review Focus 4: resending the
// stored priority over a settled tree declares nothing, the engine's clean
// no-op, never an unchanged replace the delta verifier rejects.
func TestChangeGroomPlanSamePriorityIsNoOp(t *testing.T) {
	files := map[string]string{
		groomPath(2, "add-a-widget"): groomableChange(2, "add-a-widget"),
		"docs/changes/BOARD.md":      "# Backlog\n\nold\n",
	}
	settle, opRes := groomPlanFor(t, files, baseGroomOp([]string{"inline"}, priorityOnlyReviseRequest("high")))
	if opRes.Refused {
		t.Fatalf("settling revise refused: %v", opRes.Findings)
	}
	files[groomPath(2, "add-a-widget")] = string(groomedRecordBytes(t, settle, groomPath(2, "add-a-widget")))
	files["docs/changes/BOARD.md"] = string(groomedRecordBytes(t, settle, "docs/changes/BOARD.md"))

	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{"inline"}, priorityOnlyReviseRequest("high")))
	if opRes.Refused {
		t.Fatalf("same-priority revise refused: %v", opRes.Findings)
	}
	if len(plan.Files) != 0 {
		t.Errorf("same-priority revise declared files %v, want an empty (no-op) plan", planPaths(plan))
	}
}

// TestChangeGroomPlanPriorityOnNonProposedRefused pins Review Focus 5: the
// revise gate still admits only proposed changes; a deferred change must be
// revived before it can be re-prioritized.
func TestChangeGroomPlanPriorityOnNonProposedRefused(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
	}{
		{"in-progress", reviseFixtureAtStatus(groomPath(2, "add-a-widget"), "in-progress", reviseClaimFields)},
		{"deferred", reviseFixtureAtStatus(groomPath(2, "add-a-widget"), "deferred", "")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan, opRes := groomPlanFor(t, c.files, baseGroomOp([]string{}, priorityOnlyReviseRequest("high")))
			if !opRes.Refused || len(plan.Files) != 0 {
				t.Fatalf("want a refusal writing nothing, got refused=%v files=%v", opRes.Refused, planPaths(plan))
			}
			found := false
			for _, f := range opRes.Findings {
				found = found || f.Code == "not-revisable"
			}
			if !found {
				t.Errorf("missing not-revisable; got %v", opRes.Findings)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/homer/dev/docket/.worktrees/let-change-groom-set-a-change-s-priority && go test -count=1 ./internal/app/ -run 'TestChangeGroomPlanPriority|TestChangeGroomPlanOmittedPriority|TestChangeGroomPlanSamePriority'`
Expected: FAIL. `LandsOnEveryAcceptingOutcome`, `PriorityOnlyReviseKeepsGroomState`, and `PriorityWithTitleAndSections` report `record priority not rewritten`. `SamePriorityIsNoOp` may already pass or fail depending on the settle step. `OmittedPriority…` and `…NonProposedRefused` should already PASS, which is acceptable: they pin existing behavior that must survive the change.

- [ ] **Step 3: Implement**

In `changeGroomOp.Plan` (`internal/app/change_groom.go`), directly after the retitle block that ends with `upsertField(&ps, doc1, "title", document.String(o.req.Title))` and its closing `}`, add:

```go
	if o.req.Priority != "" {
		// Re-prioritize (change 0550). Validated against domain.ParsePriority at
		// the shape stage; upsertField tolerates a record lacking the key, as the
		// retitle does. The candidate snapshot carries the new priority into the
		// artifact block and the inline board with no further code.
		upsertField(&ps, doc1, "priority", document.String(o.req.Priority))
	}
```

Also update the `// First patch pass: the typed field edits (spec/trivial/updated and the` comment so it names title and priority: `// First patch pass: the typed field edits (spec/trivial/updated/title/priority and the`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/homer/dev/docket/.worktrees/let-change-groom-set-a-change-s-priority && go test -count=1 ./internal/app/ -run 'TestChangeGroom'`
Expected: PASS (all groom tests, old and new).

If `assertPriorityLanded` fails only because the writer emitted `priority: high` unquoted, check how `title`/`updated` render in the same record. If the writer genuinely leaves this scalar unquoted, change the expected literal in `assertPriorityLanded`, `TestChangeGroomPlanPriorityWithTitleAndSections`, and the `OmittedPriority` negative check to match the writer's real output. Never weaken the "old value gone" and board assertions.

- [ ] **Step 5: Mutation-check the write**

Back up with `cp internal/app/change_groom.go /tmp/cg.bak`. Delete the new `if o.req.Priority != "" { upsertField(... "priority" ...) }` block. Confirm with `/usr/bin/grep -cF '"priority", document.String' internal/app/change_groom.go` → `0`. Run `go test -count=1 ./internal/app/ -run 'TestChangeGroomPlanPriority'`. Expected: FAIL (`LandsOnEveryAcceptingOutcome`, `PriorityOnlyReviseKeepsGroomState`, `PriorityWithTitleAndSections`). Restore with `mv -f /tmp/cg.bak internal/app/change_groom.go` and re-run Step 4. Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git -C /Users/homer/dev/docket/.worktrees/let-change-groom-set-a-change-s-priority add internal/app/change_groom.go internal/app/change_groom_test.go
git -C /Users/homer/dev/docket/.worktrees/let-change-groom-set-a-change-s-priority commit -m "feat(groom): write priority in the groom transaction and re-render the board"
```

---

### Task 3: Document the field (groom-next skill, embedded copy, guards, glossary)

**Files:**
- Modify: `skills/docket-groom-next/SKILL.md` (Step 1 stub-edit route sentence; new paragraph after the `**Retitle.**` paragraph in Step 4)
- Regenerate: `internal/assets/embedded/tree/skills/docket-groom-next/SKILL.md` (via `go generate ./internal/assets/`, never hand-edited)
- Modify: `internal/repoguard/prose_contracts_test.go` (new sentinel row after the `change_0461_retitle` row)
- Modify: `internal/repoguard/budgets_test.go` (`skillBudgets` row `{"docket-groom-next/SKILL.md", 76, 2141}`)
- Modify: `docs/reference/glossary.md` (`### Groom outcome \`revise\``, `### Priority`, the `priorities` vocabulary row)
- Do NOT modify: `skills/docket-auto-groom/SKILL.md`

**Interfaces:**
- Consumes: the wire contract from Tasks 1–2: request key `priority`, values `critical`/`high`/`medium`/`low`, codes `unknown-priority` and `invalid-priority`.
- Produces: nothing consumed by code.

- [ ] **Step 1: Write the failing prose-contract sentinel**

In `internal/repoguard/prose_contracts_test.go`, directly after the `change_0461_retitle` row (`{sentinel: "change_0461_retitle", file: "skills/docket-groom-next/SKILL.md", present: …}`), add:

```go
	// change 0550 — change.groom carries an optional priority on the same
	// outcomes as title; a priority alone is a valid revise; abstain refuses it.
	{sentinel: "change_0550_priority", file: "skills/docket-groom-next/SKILL.md",
		present: []string{"may also carry `priority`", "a `priority` alone is a valid revise", "`invalid-priority`", "`unknown-priority`",
			"owned-section, `title`, `priority`, or relationship edits"}},
```

Run: `cd /Users/homer/dev/docket/.worktrees/let-change-groom-set-a-change-s-priority && go test -count=1 ./internal/repoguard/ -run 'TestProseContracts$'`.
Expected: FAIL. `change_0550_priority` is missing each present phrase.

- [ ] **Step 2: Edit the skill**

In `skills/docket-groom-next/SKILL.md` Step 1, replace the clause
`apply Step 4's revise exit directly with the requested owned-section, \`title\`, or relationship edits`
with
`apply Step 4's revise exit directly with the requested owned-section, \`title\`, \`priority\`, or relationship edits`.

Directly after the `**Retitle.** …` paragraph in Step 4 (it ends `… writes nothing — surface it.`), insert one blank line and this paragraph:

```markdown
**Reprioritize.** Exits 1 (spec), 2 (trivial), 5 (revise), and 6 (re-enable) may also carry `priority` (`critical`, `high`, `medium`, or `low`) in the same `change.groom` request; the transaction rewrites `priority:` and re-renders the board and the `## Artifacts` block, and a `priority` alone is a valid revise. Abstain refuses it (`invalid-priority`) and an unknown value refuses with `unknown-priority`; either writes nothing — surface it. A deferred change is revived before it can be re-prioritized.
```

Re-run the Step 1 command. Expected: PASS.

- [ ] **Step 3: Regenerate the embedded copy and re-baseline the size budget**

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/let-change-groom-set-a-change-s-priority && go generate ./internal/assets/ && wc -lw skills/docket-groom-next/SKILL.md
```
Record the printed `<lines> <words>` (expected: 78 lines, about 2,215 words). In `internal/repoguard/budgets_test.go`, change the row `{"docket-groom-next/SKILL.md", 76, 2141},` to the exact new counts. Prepend this note to its trailing comment: `// 0550: the Step-4 reprioritize paragraph and the stub-edit priority route (76/2141 -> <lines>/<words>); 0532: …` (keep the rest of the existing comment unchanged after the new entry).

Run: `go test -count=1 ./internal/repoguard/ ./internal/assets/`
Expected: PASS (`TestSkillSizeBudgets` within the new ceiling; the embedded-asset tests see a regenerated copy that matches).

- [ ] **Step 4: Mutation-check the sentinel**

Back up the skill (`cp skills/docket-groom-next/SKILL.md /tmp/sk.bak`). Delete the `**Reprioritize.**` paragraph and confirm with `/usr/bin/grep -cF '**Reprioritize.**' skills/docket-groom-next/SKILL.md` → `0`. Run the Step 1 test command. Expected: FAIL on `change_0550_priority`. Restore with `mv -f /tmp/sk.bak skills/docket-groom-next/SKILL.md`, re-run, and expect PASS. Run `git -C /Users/homer/dev/docket/.worktrees/let-change-groom-set-a-change-s-priority status --porcelain` and confirm that the embedded copy is still the regenerated one (re-run `go generate ./internal/assets/` if in doubt).

- [ ] **Step 5: Update the glossary**

In `docs/reference/glossary.md`:

1. Under `### Groom outcome \`revise\``, change `edit owned sections, the\ntitle, and relationship fields.` to `edit owned sections, the\ntitle, the priority, and relationship fields.` (keep the existing line wrapping style, around 100 columns). After the sentence ending `renames\nnothing — the slug and every path stay put.`, add: `A \`priority\` alone is also a valid revise: it rewrites \`priority:\` and the board row.`
2. Under `### Priority`, after the existing line, add one paragraph: ``Set by `change create` (default `medium`) and changed on a `proposed` change by `change groom`'s optional `priority` (any outcome but `abstain`).``
3. In the vocabulary table, change the `priorities` row's last cell to `` `change create` (request `priority`), `change groom` (request `priority`), `status --priority` ``.

Do not cite change numbers in the glossary (docs describe current behavior only).

- [ ] **Step 6: Run the focused packages**

Run: `cd /Users/homer/dev/docket/.worktrees/let-change-groom-set-a-change-s-priority && go test -count=1 ./internal/app/ ./internal/repoguard/ ./internal/assets/`
Expected: PASS. (`docket-build` runs the whole-suite gate `go run ./cmd/docket development test` afterwards. Read its budget report even when the run is green.)

- [ ] **Step 7: Commit**

```bash
git -C /Users/homer/dev/docket/.worktrees/let-change-groom-set-a-change-s-priority add skills/docket-groom-next/SKILL.md internal/assets/embedded/tree/skills/docket-groom-next/SKILL.md internal/repoguard/prose_contracts_test.go internal/repoguard/budgets_test.go docs/reference/glossary.md
git -C /Users/homer/dev/docket/.worktrees/let-change-groom-set-a-change-s-priority commit -m "docs(groom): document change.groom's optional priority"
```

If `go generate` also rewrote `internal/assets/embedded/manifest.json` (check `git status --porcelain`), stage it in the same commit.

---

## Spec coverage map

| Spec item | Task |
|---|---|
| Request field `priority`, `omitempty`, `enum=priorities` | 1 |
| `unknown-priority`, same message, case-sensitive, before any write | 1 (shape + no-engine-call test) |
| Abstain refuses priority with its own code distinct from `unknown-priority` | 1 (`invalid-priority`) |
| `empty-revise` widened and message names `priority` | 1 |
| Write via `upsertField` next to retitle; `updated:` bumped; board + artifacts re-rendered from candidate | 2 |
| Groom state, slug, paths, branch untouched; resend is not an error | 2 |
| Proposed-only (deferred → revive first) | 2 (refusal test), 3 (skill text) |
| Skill paragraph next to Retitle; `go generate`; auto-groom untouched | 3 |
| Tests 1–5 and mutation check 6 | 1 (tests 4, 5; mutations), 2 (tests 1–3; mutation) |
| Whole suite at the build gate | `docket-build` gate |
