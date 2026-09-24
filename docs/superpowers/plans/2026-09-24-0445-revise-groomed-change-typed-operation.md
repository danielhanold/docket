<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0445 — Revise a groomed change's spec and owned sections through a typed operation](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0445-revise-a-groomed-change-s-spec-and-owned-sections-through-a.md)**
<!-- docket:backlink:end -->
# Revise a Groomed Change Through change.groom Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `revise` outcome to the existing `change.groom` operation so an already-groomed `proposed` change's spec body and owned proposal sections can be adjusted through one exact-version CAS transaction, and route the skills that hit this gap to the new path.

**Architecture:** No new catalog operation and no new CLI verb. `internal/app/change_groom.go` gains a third `GroomOutcome` (`revise`) gated on the exact complement of the existing groom gate (`status==proposed && (spec!="" || trivial)`). It reuses the existing `SpecMarkdown` field (whole spec-body replace, `MutationReplace` at the change's *existing* spec path) and the existing `Sections` splice; it never writes `spec:` or `trivial:`, so spec'd↔trivial flips stay structurally impossible. Skill docs (`skills/docket-groom-next/SKILL.md`, `skills/docket-new-change/SKILL.md` — the repo's own `skills/` sources, never `~/.claude/skills`) replace the documented hand-edit workaround with the typed path.

**Tech Stack:** Go (packages `internal/app`, `internal/cli`); table-driven tests in `internal/app/change_groom_test.go`; Markdown skill bodies.

**Spec:** `docs/superpowers/specs/2026-09-24-revise-a-groomed-change-s-spec-and-owned-sections-through-a-design.md` (on the `docket` metadata branch; also readable at `.docket/docs/superpowers/specs/…` from the repo root).

## Global Constraints

- `revise` never sets `spec:` or `trivial:` — no code path under the revise branch may call `ps.SetField("spec", …)` or `ps.SetField("trivial", …)` (spec, "Gate" section).
- Spec revise is whole-body replace at the change's **existing** spec path only — never a new path, never a relink, no dated-path minting.
- Repeatable: no one-shot marker; every call is a standard exact-version CAS write (the engine's existing `EntityExpectation`, unchanged).
- Out of scope (do not build): section-level spec patching, revising `in-progress`/terminal changes, autonomous revision, review workflow.
- New refusal codes on the Plan-closure channel (`not-revisable`, `spec-not-linked`, `spec-file-missing`) are plain strings passed to `refuseGroom`, matching the existing `"not-groomable"`/`"spec-path-taken"` house style. New *request-shape* code `FCEmptyRevise` must be added to both the const block and `AllFindingCodes` in `internal/app/finding_codes.go` (the census is guard-tested).
- The `groom_outcomes` schema vocabulary (`internal/app/schema_vocab.go`) must gain `revise`; `TestVocabularyConstCompleteness` holds it in correspondence with the const group.
- Cross-references in maintained source anchor on symbol names or verbatim-quoted clauses, never line numbers (repo AGENTS.md).
- The build gate runs the **whole** suite via the resolved `build.test_command`; per-task runs below are focused checks only, always with `-count=1`.

## Review Focus

The spec's enumerated acceptance items are covered task-by-task below. These five spec-implied inputs were *not* enumerated there; each gets a pinned test in the owning task:

1. **A relationships-only revise request** (`depends_on` set, no `spec_markdown`, no effective section edit) — a caller would expect either a field patch or a clear refusal; the spec's "at least one of spec_markdown/sections" rule makes it a `FCEmptyRevise` refusal, and without a test an implementer could silently count `DependsOn` as an effective edit. → Task 1.
2. **Unparseable `spec_markdown` under revise** — the spec only says "non-empty"; a body that fails `document.Parse` must be refused `invalid-spec_markdown` before the engine, exactly as the `spec` outcome does, or a malformed spec lands in the tree. → Task 1.
3. **A dangling spec link** (`spec:` names a path absent from the tree) — `MutationReplace` against a missing path must not silently mint a file; refuse `spec-file-missing`. → Task 2.
4. **HumanText for an applied revise carrying `spec_markdown`** — the existing `ResultApplied` branch keys on `SpecPath != ""` and would print "groomed (spec …)"; the result must carry the outcome so revise renders "revised". → Task 3.
5. **The outcome enum's other announcement sites** — the `FCInvalidOutcome` message ("one of spec, trivial"), the CLI help line in `internal/cli/change.go` ("spec or trivial"), and the `groom_outcomes` vocabulary must all name `revise`, or the schema/CLI keep denying the outcome exists. → Tasks 1 and 3.

---

### Task 1: `revise` outcome constant, request-shape validation, and finding-code census

**Files:**
- Modify: `internal/app/change_groom.go` (const block, `validateChangeGroomShape`, new helper `hasEffectiveSectionEdit`)
- Modify: `internal/app/finding_codes.go` (const block + `AllFindingCodes`)
- Modify: `internal/app/schema_vocab.go` (`groom_outcomes` vocabulary)
- Modify: `internal/app/finding_codes_test.go` (`TestShapeValidatorCodesAreRegistered` probe list)
- Test: `internal/app/change_groom_test.go`

**Interfaces:**
- Consumes: existing `GroomOutcome`, `ChangeGroomRequest`, `SectionEditRequest`, `validateGroomSections`, `render.SectionReplace`/`SectionRemove`, `document.Parse`.
- Produces: `GroomRevise GroomOutcome = "revise"`, `FCEmptyRevise FindingCode = "empty-revise"`, and `hasEffectiveSectionEdit(sections []SectionEditRequest) bool` — Tasks 2 and 3 rely on these exact names.

- [ ] **Step 1: Write the failing tests**

Append to `internal/app/change_groom_test.go`:

```go
// revisableChange renders a proposed change already groomed to a spec: the
// groomable fixture with spec: linked to specPath.
func revisableChange(id int, slug, specPath string) string {
	return strings.Replace(groomableChange(id, slug), "spec:\n", "spec: '"+specPath+"'\n", 1)
}

// trivialChange renders a proposed change already groomed by trivial verdict.
func trivialChange(id int, slug string) string {
	return strings.Replace(groomableChange(id, slug), "trivial: false", "trivial: true", 1)
}

// validReviseRequest is a well-formed revise request (sections + spec body)
// against the revisable fixture at id 2 / slug add-a-widget.
func validReviseRequest() ChangeGroomRequest {
	return ChangeGroomRequest{
		ChangeID:     2,
		Path:         groomPath(2, "add-a-widget"),
		Version:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Outcome:      GroomRevise,
		SpecMarkdown: "# Design\n\nThe revised design body.\n",
		Sections: []SectionEditRequest{
			{Heading: "## What changes", Intent: "replace", Markdown: "Narrowed what.\n"},
		},
	}
}

func TestChangeGroomReviseShapeValidation(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*ChangeGroomRequest)
		code string // "" means the request must pass shape validation
	}{
		{"valid revise passes", func(r *ChangeGroomRequest) {}, ""},
		{"sections-only revise passes", func(r *ChangeGroomRequest) { r.SpecMarkdown = "" }, ""},
		{"spec-only revise passes", func(r *ChangeGroomRequest) { r.Sections = nil }, ""},
		{"empty revise refused", func(r *ChangeGroomRequest) {
			r.SpecMarkdown = ""
			r.Sections = nil
		}, "empty-revise"},
		{"all-preserve revise refused", func(r *ChangeGroomRequest) {
			r.SpecMarkdown = ""
			r.Sections = []SectionEditRequest{{Heading: "## Why", Intent: "preserve"}}
		}, "empty-revise"},
		{"relationships-only revise refused", func(r *ChangeGroomRequest) {
			r.SpecMarkdown = ""
			r.Sections = nil
			r.DependsOn = []int{1}
		}, "empty-revise"},
		{"unparseable revise spec_markdown refused", func(r *ChangeGroomRequest) {
			r.SpecMarkdown = "---\nid: 1\n"
		}, "invalid-spec_markdown"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := validReviseRequest()
			c.mut(&req)
			findings := validateChangeGroomShape(req)
			if c.code == "" {
				if len(findings) != 0 {
					t.Fatalf("unexpected shape findings: %v", findings)
				}
				return
			}
			if !hasFindingCode(findings, c.code) {
				t.Errorf("missing finding %q; got %v", c.code, findings)
			}
		})
	}
}

func TestChangeGroomEmptyReviseRefusedWithoutEngineCall(t *testing.T) {
	req := validReviseRequest()
	req.SpecMarkdown = ""
	req.Sections = nil
	engine := &recordingEngine{}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
	deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeGroom(context.Background(), deps, "", req)

	if res.Result != ResultInvalidInput {
		t.Fatalf("result = %q, want invalid-input", res.Result)
	}
	if len(engine.calls) != 0 {
		t.Errorf("engine called %d times on an empty revise, want 0", len(engine.calls))
	}
	if !hasFindingCode(res.Findings, "empty-revise") {
		t.Errorf("missing finding empty-revise; got %v", res.Findings)
	}
}
```

Note `TestChangeGroomReviseShapeValidation` calls `validateChangeGroomShape` directly (it is in-package) so the passing cases don't need engine plumbing; the second test pins the no-engine-call property end to end, mirroring `TestChangeGroomTrivialRequiresRationale`.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/app/ -run 'TestChangeGroomRevise|TestChangeGroomEmptyRevise' -count=1`
Expected: compile FAILURE — `undefined: GroomRevise` (the constant does not exist yet).

- [ ] **Step 3: Implement**

In `internal/app/change_groom.go`, extend the outcome const block:

```go
const (
	// GroomSpec lands an authored design spec and links it from the change.
	GroomSpec GroomOutcome = "spec"
	// GroomTrivial marks the change trivial with an authored rationale, writing
	// no spec file.
	GroomTrivial GroomOutcome = "trivial"
	// GroomRevise adjusts an already-groomed proposed change: a whole-body
	// replace of its existing linked spec, owned proposal-section edits, or
	// both. It never writes spec: or trivial:, so a change can never flip
	// between spec'd and trivial through this outcome.
	GroomRevise GroomOutcome = "revise"
)
```

In `validateChangeGroomShape`, add the case (before `default`) and update the `FCInvalidOutcome` message:

```go
	case GroomRevise:
		if strings.TrimSpace(req.SpecMarkdown) != "" {
			if _, perr := document.Parse([]byte(req.SpecMarkdown)); perr != nil {
				addShape(FCInvalidSpecMarkdown, "spec_markdown must parse as a Markdown document: "+perr.Error())
			}
		} else if !hasEffectiveSectionEdit(req.Sections) {
			addShape(FCEmptyRevise, "the revise outcome requires a non-empty spec_markdown or at least one replace/remove section edit")
		}
	default:
		addShape(FCInvalidOutcome, fmt.Sprintf("outcome %q must be one of spec, trivial, revise", req.Outcome))
```

Add the helper beside `hasAuthoredRationale`:

```go
// hasEffectiveSectionEdit reports whether the section edits carry at least one
// replace or remove — the revise outcome's minimum effective input. A
// preserve-only or empty list changes nothing and is refused as an empty
// revise; relationship-field patches alone do not qualify.
func hasEffectiveSectionEdit(sections []SectionEditRequest) bool {
	for _, s := range sections {
		switch render.SectionIntent(s.Intent) {
		case render.SectionReplace, render.SectionRemove:
			return true
		}
	}
	return false
}
```

In `internal/app/finding_codes.go`, add to the const block (beside `FCMissingRationale`):

```go
	FCEmptyRevise               FindingCode = "empty-revise"
```

and insert `FCEmptyRevise,` into `AllFindingCodes` in sorted position (after `FCEmptyReport,`, before `FCEmptySpecMarkdown,` — the census is alphabetically ordered and `TestFindingCodeRegistryIntegrity` guards it).

In `internal/app/schema_vocab.go`, extend the vocabulary:

```go
	v["groom_outcomes"] = Vocabulary{Members: []string{string(GroomSpec), string(GroomTrivial), string(GroomRevise)}}
```

In `internal/app/finding_codes_test.go`, `TestShapeValidatorCodesAreRegistered`, add one probe line after the existing two groom probes so the new code's registration is exercised:

```go
	emitted = append(emitted, validateChangeGroomShape(ChangeGroomRequest{Outcome: GroomRevise})...)
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/app/ -run 'TestChangeGroom|TestFindingCode|TestShapeValidator|TestVocabulary|TestSchemaVocab' -count=1`
Expected: PASS (including the pre-existing groom tests — the `spec`/`trivial` legs are untouched).

- [ ] **Step 5: Mutation-check the census guard**

Temporarily delete the `FCEmptyRevise,` line from `AllFindingCodes` (keep the const), re-run `go test ./internal/app/ -run TestShapeValidatorCodesAreRegistered -count=1`, and confirm it REDDENS (the validator emits an unregistered code). Restore the line and confirm green. This proves the census probe added in Step 3 is load-bearing, not decorative.

- [ ] **Step 6: Commit**

```bash
git add internal/app/change_groom.go internal/app/finding_codes.go internal/app/schema_vocab.go internal/app/finding_codes_test.go internal/app/change_groom_test.go
git commit -m "feat(app): revise groom outcome — request shape, finding code, vocabulary (change 0445)"
```

---

### Task 2: Plan-closure revise gate and mutations

**Files:**
- Modify: `internal/app/change_groom.go` (`Plan`, file-header comment)
- Test: `internal/app/change_groom_test.go`

**Interfaces:**
- Consumes: `GroomRevise` (Task 1), existing `refuseGroom`, `treeHasPath`, `assembleSpecFile`, `render.BacklinkContent`, `render.ApplySectionEdits`, test helpers `groomPlanFor`, `baseGroomOp`, `groomedRecordBytes`, `assertPlanPaths`, `revisableChange`/`trivialChange`/`validReviseRequest` (Task 1).
- Produces: Plan-closure refusal codes `"not-revisable"`, `"spec-not-linked"`, `"spec-file-missing"` (plain strings via `refuseGroom`); a revise `MutationPlan` whose spec mutation is `transaction.MutationReplace` at `c.Spec().Value`. Task 3's receipt work reads the same `specPath` variable this task threads through.

- [ ] **Step 1: Write the failing tests**

Append to `internal/app/change_groom_test.go`:

```go
const reviseSpecPath = "docs/superpowers/specs/2026-08-01-add-a-widget-design.md"

// reviseFixtureFiles is the fake tree for a revisable spec'd change: the
// record with spec: linked, and the spec file itself with a backlink block.
func reviseFixtureFiles() map[string]string {
	return map[string]string{
		groomPath(2, "add-a-widget"): revisableChange(2, "add-a-widget", reviseSpecPath),
		reviseSpecPath: "<!-- docket:backlink:start (generated — do not hand-edit) -->\n" +
			"> old backlink\n" +
			"<!-- docket:backlink:end -->\n\n# Design\n\nThe original design body.\n",
	}
}

func TestChangeGroomPlanReviseSectionsOnly(t *testing.T) {
	files := reviseFixtureFiles()
	files["docs/changes/BOARD.md"] = "# Backlog\n\nold\n"
	req := validReviseRequest()
	req.SpecMarkdown = "" // sections only
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{"inline"}, req))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	// Spec item 1: record + board replaced; the spec file is NOT in the plan,
	// so it stays byte-identical by construction.
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		groomPath(2, "add-a-widget"): transaction.MutationReplace,
		"docs/changes/BOARD.md":      transaction.MutationReplace,
	})
	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	if strings.Contains(rec, "Original what.") || !strings.Contains(rec, "Narrowed what.") {
		t.Errorf("## What changes section not replaced:\n%s", rec)
	}
	if !strings.Contains(rec, "updated: '2026-08-16'") {
		t.Errorf("updated not stamped from the clock:\n%s", rec)
	}
	// Spec item 9: revise never flips the groomed-outcome fields.
	if !strings.Contains(rec, "spec: '"+reviseSpecPath+"'") {
		t.Errorf("spec field changed under revise:\n%s", rec)
	}
	if !strings.Contains(rec, "trivial: false") {
		t.Errorf("trivial field changed under revise:\n%s", rec)
	}
}

func TestChangeGroomPlanReviseSpecBodyOnly(t *testing.T) {
	files := reviseFixtureFiles()
	before := files[groomPath(2, "add-a-widget")]
	req := validReviseRequest()
	req.Sections = nil // spec body only
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{}, req))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	// Spec item 2: the spec file is REPLACED at the existing path, never created
	// at a new dated path.
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		groomPath(2, "add-a-widget"): transaction.MutationReplace,
		reviseSpecPath:               transaction.MutationReplace,
	})
	spec := string(groomedRecordBytes(t, plan, reviseSpecPath))
	if !strings.Contains(spec, "docket:backlink:start") {
		t.Errorf("revised spec file missing backlink block:\n%s", spec)
	}
	if !strings.Contains(spec, "The revised design body.") || strings.Contains(spec, "The original design body.") {
		t.Errorf("spec body not replaced:\n%s", spec)
	}
	// Spec item 2: the record's sections are byte-identical apart from
	// updated:. The docket:artifacts block legitimately re-renders (the same
	// call every groom outcome makes — the empty fixture block gains a Spec
	// row), so compare the authored body AFTER the artifacts block, plus the
	// frontmatter fields, rather than the whole file.
	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	bodyAfterArtifacts := func(s string) string {
		i := strings.Index(s, "docket:artifacts:end")
		if i < 0 {
			t.Fatalf("record lacks the artifacts end marker:\n%s", s)
		}
		return s[i:]
	}
	if got, want := bodyAfterArtifacts(rec), bodyAfterArtifacts(before); got != want {
		t.Errorf("authored body changed under a spec-only revise:\ngot:\n%s\nwant:\n%s", got, want)
	}
	if !strings.Contains(rec, "updated: '2026-08-16'") {
		t.Errorf("updated not stamped:\n%s", rec)
	}
	if !strings.Contains(rec, "spec: '"+reviseSpecPath+"'") || !strings.Contains(rec, "trivial: false") {
		t.Errorf("groomed-outcome fields changed under a spec-only revise:\n%s", rec)
	}
}

func TestChangeGroomPlanReviseBoth(t *testing.T) {
	// Spec item 3: both edits land in one plan.
	plan, opRes := groomPlanFor(t, reviseFixtureFiles(), baseGroomOp([]string{}, validReviseRequest()))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		groomPath(2, "add-a-widget"): transaction.MutationReplace,
		reviseSpecPath:               transaction.MutationReplace,
	})
}

func TestChangeGroomPlanReviseTrivialRationale(t *testing.T) {
	// Spec item 4: sections-only revise of a trivial-verdicted change.
	files := map[string]string{
		groomPath(2, "add-a-widget"): trivialChange(2, "add-a-widget"),
	}
	req := validReviseRequest()
	req.SpecMarkdown = ""
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{}, req))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		groomPath(2, "add-a-widget"): transaction.MutationReplace,
	})
	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	if !strings.Contains(rec, "trivial: true") {
		t.Errorf("trivial verdict lost under revise:\n%s", rec)
	}
	if strings.Contains(rec, "spec: '") {
		t.Errorf("revise of a trivial change wrote a spec link:\n%s", rec)
	}
}

func TestChangeGroomPlanReviseRefusals(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		mut   func(*ChangeGroomRequest)
		code  string
	}{
		// Spec item 5: spec_markdown against a trivial-only (no-spec) change.
		{"spec-not-linked", map[string]string{
			groomPath(2, "add-a-widget"): trivialChange(2, "add-a-widget"),
		}, func(r *ChangeGroomRequest) {}, "spec-not-linked"},
		// Spec item 6: a needs-brainstorm change is groom's target, not revise's.
		{"not-revisable needs-brainstorm", map[string]string{
			groomPath(2, "add-a-widget"): groomableChange(2, "add-a-widget"),
		}, func(r *ChangeGroomRequest) {}, "not-revisable"},
		// Spec item 7: a non-proposed change.
		{"not-revisable blocked", map[string]string{
			groomPath(2, "add-a-widget"): strings.Replace(
				revisableChange(2, "add-a-widget", reviseSpecPath),
				"status: proposed\n", "status: blocked\nblocked_by: 'waiting'\n", 1),
		}, func(r *ChangeGroomRequest) {}, "not-revisable"},
		// Review Focus 3: dangling spec link — spec: names a path absent from
		// the tree; never silently mint a file.
		{"spec-file-missing", map[string]string{
			groomPath(2, "add-a-widget"): revisableChange(2, "add-a-widget", reviseSpecPath),
		}, func(r *ChangeGroomRequest) {}, "spec-file-missing"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := validReviseRequest()
			c.mut(&req)
			plan, opRes := groomPlanFor(t, c.files, baseGroomOp([]string{}, req))
			if !opRes.Refused {
				t.Fatalf("expected a refusal, got plan files %v", planPaths(plan))
			}
			found := false
			for _, f := range opRes.Findings {
				if f.Code == c.code {
					found = true
				}
			}
			if !found {
				t.Errorf("missing refusal code %q; got %v", c.code, opRes.Findings)
			}
			if len(plan.Files) != 0 {
				t.Errorf("refused plan still carries files: %v", planPaths(plan))
			}
		})
	}
}

func TestChangeGroomPlanReviseRepeatable(t *testing.T) {
	// Spec item 11 (plan level): a second revise over the first revise's own
	// output succeeds — no one-shot marker exists.
	files := reviseFixtureFiles()
	plan1, opRes := groomPlanFor(t, files, baseGroomOp([]string{}, validReviseRequest()))
	if opRes.Refused {
		t.Fatalf("first revise refused: %v", opRes.Findings)
	}
	files[groomPath(2, "add-a-widget")] = string(groomedRecordBytes(t, plan1, groomPath(2, "add-a-widget")))
	files[reviseSpecPath] = string(groomedRecordBytes(t, plan1, reviseSpecPath))
	req2 := validReviseRequest()
	req2.SpecMarkdown = "# Design\n\nThe twice-revised body.\n"
	plan2, opRes2 := groomPlanFor(t, files, baseGroomOp([]string{}, req2))
	if opRes2.Refused {
		t.Fatalf("second revise refused: %v", opRes2.Findings)
	}
	spec := string(groomedRecordBytes(t, plan2, reviseSpecPath))
	if !strings.Contains(spec, "The twice-revised body.") {
		t.Errorf("second revise did not land:\n%s", spec)
	}
}
```

(Spec item 10 — stale/contended version — is the engine's existing `EntityExpectation` CAS, wired identically for every outcome by `ChangeGroom`'s `Expected:` block; it is not re-tested per outcome, matching how the `spec`/`trivial` outcomes treat it. Spec item 11's engine-level half is Task 3's integration test.)

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/app/ -run TestChangeGroomPlanRevise -count=1`
Expected: FAIL — every revise plan currently hits the `not-groomable` refusal (the gate does not know `revise` yet), so the happy-path tests report "unexpected refusal" and the refusal tests report the wrong code.

- [ ] **Step 3: Implement**

In `internal/app/change_groom.go` `Plan`, replace the single groom gate with an outcome split. The existing gate block:

```go
	// Groom gate: the change must be proposed, still need design (no spec, not
	// yet trivial). Grooming never inspects or sets claim metadata.
	if c.Status() != domain.StatusProposed || c.Spec().Value != "" || c.Trivial() {
		return refuseGroom("not-groomable", ...)
	}
```

becomes:

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
		return refuseGroom("not-groomable",
			fmt.Sprintf("change %04d is not a proposed, needs-design change (status %q, spec %q, trivial %v)",
				o.req.ChangeID, c.Status(), c.Spec().Value, c.Trivial()))
	}
```

Immediately after the gate (before the section splice), resolve the revise spec decision so the request is refused before any mutation is assembled:

```go
	// Revise spec-body decision, resolved before any mutation is assembled: a
	// non-empty SpecMarkdown replaces the change's EXISTING linked spec — never
	// a new path — and requires both the link and the file to exist.
	reviseSpec := o.req.Outcome == GroomRevise && strings.TrimSpace(o.req.SpecMarkdown) != ""
	if reviseSpec {
		if c.Spec().Value == "" {
			return refuseGroom("spec-not-linked",
				fmt.Sprintf("change %04d has no linked spec to revise (spec_markdown was submitted against a trivial-only change)", o.req.ChangeID))
		}
		exists, err := treeHasPath(ctx, st.Tree, c.Spec().Value)
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, err
		}
		if !exists {
			return refuseGroom("spec-file-missing",
				fmt.Sprintf("change %04d links spec %q but no such file exists on the tree", o.req.ChangeID, c.Spec().Value))
		}
	}
```

The `specPath` computation and the two field-set blocks already key on `o.req.Outcome == GroomSpec` / `== GroomTrivial`, so under revise neither `spec` nor `trivial` is ever set — leave them exactly as they are (this is the structural impossibility the spec requires; do not add any revise arm that touches those fields). For revise, `specPath` (the dated new-path variable) is unused; keep its computation where it is (it is cheap and pure) or guard it under `GroomSpec` along with its `treeHasPath` probe — the probe is already inside the `GroomSpec` block.

In the file-mutation assembly, extend the spec-file block:

```go
	if o.req.Outcome == GroomSpec {
		backlink, err := render.BacklinkContent(gc, o.link)
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change groom: rendering spec backlink: %w", err)
		}
		files = append(files, transaction.FileMutation{
			Path: gitcli.RepoPath(specPath), Kind: transaction.MutationCreate,
			Bytes: assembleSpecFile(backlink, o.req.SpecMarkdown),
		})
	}
	if reviseSpec {
		backlink, err := render.BacklinkContent(gc, o.link)
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change groom: rendering spec backlink: %w", err)
		}
		files = append(files, transaction.FileMutation{
			Path: gitcli.RepoPath(c.Spec().Value), Kind: transaction.MutationReplace,
			Bytes: assembleSpecFile(backlink, o.req.SpecMarkdown),
		})
	}
```

Update the file-header comment (the "one of two authored outcomes" paragraph) to name the third outcome: grooming to build-ready by spec or trivial verdict, or revising an already-groomed proposed change in place.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/app/ -run TestChangeGroomPlan -count=1`
Expected: PASS — all new revise plan tests and every pre-existing groom plan test (`SpecOutcomeFileSet`, `TrivialOutcomeFileSet`, `RefusesNonGroomable`, `RefusesExistingSpecPath`, `SourcePreservation`, `RelationshipsWritten`, `ToleratesMissingUpdatedField` must stay green — the groom gate's behavior for `spec`/`trivial` is byte-for-byte unchanged).

- [ ] **Step 5: Mutation-check the gate complement**

Two hand mutations, each proving one conjunct of the revise gate (per the duplicated-gate learning — the gate is a whole predicate, not a threshold):

1. Change `(c.Spec().Value == "" && !c.Trivial())` to `c.Spec().Value == ""` — run `go test ./internal/app/ -run TestChangeGroomPlanRevise -count=1`; expected: `TestChangeGroomPlanReviseTrivialRationale` REDDENS (a trivial change would be refused). Restore.
2. Delete the `c.Status() != domain.StatusProposed` conjunct from the revise arm — expected: the `not-revisable blocked` case REDDENS. Restore.

Confirm each mutation landed with `grep -c` on the edited expression before trusting the red, then confirm green after restore.

- [ ] **Step 6: Commit**

```bash
git add internal/app/change_groom.go internal/app/change_groom_test.go
git commit -m "feat(app): revise groom outcome — plan-closure gate and spec replace (change 0445)"
```

---

### Task 3: Receipt, result outcome, HumanText, CLI help, and the applied-path integration test

**Files:**
- Modify: `internal/app/change_groom.go` (`ChangeGroomResult`, `HumanText`, `changeGroomResultFromOutcome`, receipt assembly in `Plan`)
- Modify: `internal/cli/change.go` (groom subcommand help line)
- Test: `internal/app/change_groom_test.go`, `internal/app/change_integration_test.go`

**Interfaces:**
- Consumes: `GroomRevise`, `reviseSpec` plumbing (Task 2), `changeGroomReceipt` (already carries `Outcome`), integration helpers `newWorkingRepo`, `newGitClient`, `mustMarshal`, `recordingEngine`, `mainModePin`, `testClock`.
- Produces: `ChangeGroomResult.Outcome string` (JSON `outcome,omitempty`), populated from the receipt on apply; `HumanText` revise case `"change %04d revised — %s"`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/app/change_groom_test.go`:

```go
func TestChangeGroomResultHumanTextRevise(t *testing.T) {
	r := newChangeGroomResult(ResultApplied, ChangeGroomResult{
		ID: 7, Outcome: string(GroomRevise), SpecPath: "docs/superpowers/specs/x.md",
		Revision: "cafebabecafebabecafebabecafebabecafebabe",
	})
	got := r.HumanText()
	want := "change 0007 revised — cafebabecafebabecafebabecafebabecafebabe"
	if got != want {
		// Review Focus 4: a revise carrying a spec path must NOT render as
		// "groomed (spec …)".
		t.Errorf("HumanText = %q, want %q", got, want)
	}
}
```

Append to `internal/app/change_integration_test.go` (inside the `//go:build integration` file, mirroring `TestIntegrationChangeAuthoringGroomAppliedResult`):

```go
func TestIntegrationChangeAuthoringReviseAppliedResult(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	specPath := "docs/superpowers/specs/2026-08-01-add-a-widget-design.md"
	receipt := mustMarshal(t, changeGroomReceipt{
		ID: 2, Op: OperationChangeGroom, Outcome: string(GroomRevise), SpecPath: specPath,
	})
	engine := &recordingEngine{result: transaction.Result{
		Disposition:   transaction.DispositionApplied,
		AppliedCommit: "cafebabecafebabecafebabecafebabecafebabe",
		Receipt:       receipt,
	}}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeGroom(context.Background(), deps, repoDir, validReviseRequest())

	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied", res.Result)
	}
	if res.Outcome != string(GroomRevise) {
		t.Errorf("outcome = %q, want revise", res.Outcome)
	}
	if res.ID != 2 || res.SpecPath != specPath {
		t.Errorf("identity from receipt = (%d, %q)", res.ID, res.SpecPath)
	}
	// Spec items 10/11 (engine-level): the CAS expectation pins the exact
	// submitted version on every call, revise included — repeatability is a
	// second standard call with the freshly-read version, nothing more.
	if len(engine.calls) != 1 {
		t.Fatalf("engine calls = %d, want 1", len(engine.calls))
	}
	exp := engine.calls[0].Expected
	if len(exp) != 1 || string(exp[0].Version.ObjectID) != validReviseRequest().Version {
		t.Errorf("revise did not pin the exact submitted version: %+v", exp)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/app/ -run TestChangeGroomResultHumanTextRevise -count=1`
Expected: compile FAILURE — `unknown field Outcome in struct literal`.

Run: `go test -tags integration ./internal/app/ -run TestIntegrationChangeAuthoringReviseAppliedResult -count=1`
Expected: compile FAILURE for the same reason.

- [ ] **Step 3: Implement**

In `internal/app/change_groom.go`:

`ChangeGroomResult` gains the outcome field (after `ID`):

```go
type ChangeGroomResult struct {
	Envelope
	ID       int             `json:"id,omitempty"`
	Outcome  string          `json:"outcome,omitempty"`
	SpecPath string          `json:"spec_path,omitempty"`
	Revision string          `json:"committed_revision,omitempty"`
	Findings []StatusFinding `json:"findings"`
}
```

`HumanText` gains the revise arm first in the applied branch:

```go
	case ResultApplied:
		if r.Outcome == string(GroomRevise) {
			return fmt.Sprintf("change %04d revised — %s", r.ID, r.Revision)
		}
		if r.SpecPath != "" {
			return fmt.Sprintf("change %04d groomed (spec %s) — %s", r.ID, r.SpecPath, r.Revision)
		}
		return fmt.Sprintf("change %04d groomed (trivial) — %s", r.ID, r.Revision)
```

`changeGroomResultFromOutcome` copies the receipt's outcome:

```go
	if rec, ok := decodeChangeGroomReceipt(res.Receipt); ok {
		out.ID = rec.ID
		out.Outcome = rec.Outcome
		out.SpecPath = rec.SpecPath
	}
```

In `Plan`'s receipt assembly, populate `SpecPath` for a revise that replaced the spec (mirroring the trivial outcome's empty `SpecPath` when only sections changed):

```go
	receiptSpecPath := ""
	if o.req.Outcome == GroomSpec {
		receiptSpecPath = specPath
	}
	if reviseSpec {
		receiptSpecPath = c.Spec().Value
	}
```

Also update the commit-subject line so a revise commit reads `change 0445 groomed (revise)` — the existing `fmt.Sprintf("change %04d groomed (%s)", …)` already does this via the outcome string; leave it unchanged.

In `internal/cli/change.go`, update the groom subcommand's help line (quote it verbatim when searching):

```go
	groom := changeSubcommand("change", "groom",
		"Groom a proposed change to build-ready (spec or trivial), or revise an already-groomed one, from a JSON request",
		...
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/app/ -count=1 && go test -tags integration ./internal/app/ -run TestIntegrationChangeAuthoring -count=1 && go build ./...`
Expected: PASS (the whole untagged `internal/app` package, the authoring integration slice, and a clean build including `internal/cli`). `TestIntegrationChangeAuthoringGroomAppliedResult` must stay green — its receipt carries `Outcome: "spec"` and now also flows into `res.Outcome`, which that test does not assert.

- [ ] **Step 5: Commit**

```bash
git add internal/app/change_groom.go internal/app/change_groom_test.go internal/app/change_integration_test.go internal/cli/change.go
git commit -m "feat(app,cli): revise groom outcome — receipt, result outcome, human text (change 0445)"
```

---

### Task 4: Skill-doc wiring — replace the hand-edit workaround with the typed revise path

**Files:**
- Modify: `skills/docket-groom-next/SKILL.md` ("When to use" bullet, Step 1, Step 4)
- Modify: `skills/docket-new-change/SKILL.md` (Brainstorm-mode step 5)

These are the repo's own `skills/` sources (they ship via the harness sync), never `~/.claude/skills`.

**Interfaces:**
- Consumes: the `change.groom` request vocabulary established in Tasks 1–3 (`outcome: revise`, `spec_markdown`, `sections`, refusals `not-revisable` / `spec-not-linked` / `spec-file-missing` / `empty-revise`).
- Produces: prose only; no code contract.

- [ ] **Step 1: Grep the suite for the prose being removed**

Before editing, prove the deleted sentence has no test dependents (restatements accumulate their own guards — asserts grep the copy, not the source):

```bash
cd <feature-worktree>
grep -rn "clear .spec:. by hand" tests/ internal/ --include='*_test.go' --include='*.sh'
grep -rn "re-groom" tests/ internal/ --include='*_test.go' --include='*.sh'
```

Expected: no hits (verified at plan time — `internal/repoguard/prose_contracts_test.go`'s only `docket-groom-next` sentinel pins the `### Step 3 — Recap, then groom with the human` heading, which this task does not touch). If either grep hits, repoint that assert at the new prose per the relocation rule — do not keep the old sentence to appease a grep.

- [ ] **Step 2: Edit `skills/docket-groom-next/SKILL.md`**

(a) Replace the "When to use" bullet:

```markdown
- Do NOT use to re-groom a change that already has a spec — drift against current reality is the reconcile pass's job in `docket-implement-next`. A human who wants to redo a design can clear `spec:` by hand first.
```

with:

```markdown
- An explicit id naming an already-groomed `proposed` change (has `spec:` or `trivial: true`) is NOT an error — it routes to the revise flow (Step 4, exit 5): adjust the existing spec and owned sections through `change.groom` with `outcome: revise`. Drift against current reality at build time remains the reconcile pass's job in `docket-implement-next`.
```

(b) In **Step 1 — Select**, amend the explicit-id sentence. Replace:

```markdown
Pick the top, or accept an explicit id from the caller; an explicit id that is not needs-brainstorm is an error to report, never a silent re-pick.
```

with:

```markdown
Pick the top, or accept an explicit id from the caller. An explicit id naming an already-groomed `proposed` change (has `spec:` or `trivial: true`) routes to the **revise** flow — recap what is there today (the linked spec's body, or the trivial rationale, and the owned sections), run the resolved brainstorm skill seeded with the current design framed as "what would you like to adjust," and exit via Step 4's revise exit. Any other explicit id that is not needs-brainstorm is an error to report, never a silent re-pick.
```

(c) In **Step 4 — Exit**, retitle the intro from "one of four" to "one of five" (also update the heading's `(one of four; the human confirms which)`) and append a fifth exit after Defer:

```markdown
5. **Revise** (explicit-id route only): the change is already groomed and the human wants it adjusted — apply the `change.groom` operation with `--repo-dir .docket --request <request-file>` with `outcome: revise`, the pinned `path` + `version`, and whichever of `spec_markdown` (a whole-body replace of the *existing* linked spec — the transaction re-stamps its `docket:backlink` block and never mints a new path) and owned-section `sections` edits the adjustment touched, plus any relationship-field updates. It never sets `spec:` or `trivial:`, so a change cannot flip between spec'd and trivial here. Repeatable while the change stays `proposed`. A typed refusal (`not-revisable`, `spec-not-linked`, `spec-file-missing`, `empty-revise`, version mismatch) writes nothing — surface it. The change stays build-ready.
```

- [ ] **Step 3: Edit `skills/docket-new-change/SKILL.md`**

In Brainstorm-mode step 5 (**Groom to build-ready & land**), append one sentence at the end, after "STOP. Never implements.":

```markdown
To adjust a just-landed spec or its owned sections afterwards, run `docket-groom-next <id>` — its explicit-id path routes an already-groomed change to `change.groom` with `outcome: revise`; never re-run this skill or hand-edit the spec file.
```

- [ ] **Step 4: Verify**

```bash
cd <feature-worktree>
/usr/bin/grep -c "by hand first" skills/docket-groom-next/SKILL.md   # expect 0
/usr/bin/grep -c "outcome: revise" skills/docket-groom-next/SKILL.md # expect >= 2 (When-to-use routes + Step 4 exit)
/usr/bin/grep -c "outcome: revise" skills/docket-new-change/SKILL.md # expect 1
go test ./internal/repoguard/ -run TestProse -count=1
```

Expected: counts as annotated; the prose-contract sentinels stay green (the pinned Step 3 heading is untouched).

- [ ] **Step 5: Commit**

```bash
git add skills/docket-groom-next/SKILL.md skills/docket-new-change/SKILL.md
git commit -m "docs(skills): route already-groomed explicit ids to the revise flow (change 0445)"
```

---

## Acceptance-item map (spec "Acceptance and verification")

| Spec item | Test | Task |
|---|---|---|
| 1 sections-only revise | `TestChangeGroomPlanReviseSectionsOnly` | 2 |
| 2 spec-body-only revise | `TestChangeGroomPlanReviseSpecBodyOnly` | 2 |
| 3 both in one commit | `TestChangeGroomPlanReviseBoth` | 2 |
| 4 trivial-rationale revise | `TestChangeGroomPlanReviseTrivialRationale` | 2 |
| 5 spec_markdown vs no-spec change | `TestChangeGroomPlanReviseRefusals/spec-not-linked` | 2 |
| 6 needs-brainstorm refused | `TestChangeGroomPlanReviseRefusals/not-revisable needs-brainstorm` | 2 |
| 7 non-proposed refused | `TestChangeGroomPlanReviseRefusals/not-revisable blocked` | 2 |
| 8 empty/all-preserve refused pre-engine | `TestChangeGroomReviseShapeValidation`, `TestChangeGroomEmptyReviseRefusedWithoutEngineCall` | 1 |
| 9 never writes spec:/trivial: | field asserts in `ReviseSectionsOnly` (spec'd) + `ReviseTrivialRationale` (trivial), byte-equality in `ReviseSpecBodyOnly` | 2 |
| 10 stale version CAS | `TestIntegrationChangeAuthoringReviseAppliedResult` exact-version expectation (mechanism shared with every outcome) | 3 |
| 11 repeated revise | `TestChangeGroomPlanReviseRepeatable` (plan level) + item-10 test's receipt/CAS note | 2/3 |
