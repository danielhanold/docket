<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0354 — Halt-report authoring writes a duplicate Run halted heading, wedging docket change resume-halted](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-08-0354-halt-report-authoring-writes-a-duplicate-run-halted-heading.md)**
<!-- docket:backlink:end -->
# Halt-Report Body Validation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `change.halt` refuse any authored report whose body would create a second structural `## ` heading or leave an unterminated code fence, before any metadata effect, so a recorded halt is always removable by `change.resume-halted` — and state the body-only authoring contract in the callers' documentation.

**Architecture:** A new fence-aware report-body validator in `internal/render/section.go` (`ValidateSectionBody`), backed by the exact scanner `ApplySectionEdits` already uses (`scanH2Headings` + `fenceRun`/`isBareFence`/`codeFenceRE`), exposed via two `errors.Is`-comparable sentinel errors. `validateHaltShape` in `internal/app/change_halt.go` calls it in the configuration-independent shape check — before `PinContext`, the transaction engine, or any write — mapping failures onto the existing `invalid-section-markdown` finding code with `field: report`. Skill and request documentation state the operation-owned-wrapper / caller-owned-body contract.

**Tech Stack:** Go (stdlib only), the repo's existing test layout (`internal/render/section_test.go`, `internal/app/change_halt_test.go`, `internal/app/change_integration_test.go` under `//go:build integration`, `internal/cli/change_test.go`).

**Spec:** `docs/superpowers/specs/2026-09-07-halt-report-authoring-writes-a-duplicate-run-halted-heading-design.md` (synchronized copy read from the metadata worktree; merges to the `docket` branch)

## Global Constraints

- Diagnostics must **never echo authored report text** — tests seed a distinctive marker string and assert its absence from output.
- `ApplySectionEdits` behavior for all existing callers is **unchanged** — no signature change, no new rejection inside it. Its duplicate-owned-heading guard stays.
- Fence recognition (delimiter char, run length, up-to-3-space indent, bare-close rule, CRLF) derives from the **existing scanner** — no independent reimplementation (learnings: `validator-must-match-the-reader-it-feeds`).
- Validation keys on the **condition** (what the section scanner would recognize), never on residue or an enumerated list of heading spellings (AGENTS.md: shape, not spellings).
- Every new guard is **mutation-tested** with `-count=1` (learnings: `cached-runner-serves-a-mutated-tree`); strip the guarded thing, watch the test redden, restore.
- No automatic repair, stripping, demotion, or renaming of authored content; no change to resume's acknowledgement/version/workspace safeguards; frozen historical artifacts (archived changes, specs, plans, results files) are not edited.
- Cross-references in maintained source anchor on symbol names or verbatim-quoted clauses, never line numbers (ADR-0054).
- Full suite at the build gate: `go run ./cmd/docket development test` from the feature-worktree root (resolved from `build.test_command`).

---

### Task 1: `render.ValidateSectionBody` — fence-aware body validator

**Files:**
- Modify: `internal/render/section.go` (add sentinel errors, `scanH2HeadingsFinalFence`, `ValidateSectionBody`; re-express `scanH2Headings` as a wrapper)
- Test: `internal/render/section_test.go`

**Interfaces:**
- Consumes: the existing private scanner pieces `splitLines`, `fenceRun`, `isBareFence`, `codeFenceRE`, `h2Prefix`, `h2Heading` (all already in `section.go`).
- Produces: `func ValidateSectionBody(body []byte) error` returning `nil`, `ErrSectionBodyHeading`, or `ErrSectionBodyUnterminatedFence` (exported `errors.New` sentinels, compared with `errors.Is`). Task 2 depends on exactly these three outcomes. Error strings carry no body content.

- [ ] **Step 1: Write the failing tests**

Append to `internal/render/section_test.go` (add `"errors"` to its imports):

```go
// --- ValidateSectionBody ----------------------------------------------------

// TestValidateSectionBody proves the report-body validator applies the exact
// fence-aware rules the section scanner uses: a column-zero "## " heading
// outside fenced code is rejected wherever it appears, heading-shaped text
// inside a properly closed backtick or tilde fence is content, and a fence
// left open at end of body is rejected so it cannot hide a following section
// from the recovery scanner. LF and CRLF, and scanner-sensitive fence-length
// cases, are exercised on both sides.
func TestValidateSectionBody(t *testing.T) {
	cases := []struct {
		name string
		body string
		want error
	}{
		{"empty", "", nil},
		{"prose", "Paused pending infra; suite red on internal/app.\n", nil},
		{"list-and-h3", "### 2026-08-26\n\n- first\n- second\n\n#### deeper\n\ntail\n", nil},
		{"inline-mention", "the `## Run halted` section is removed on resume\n", nil},
		{"blockquote-heading-shape", "> ## quoted heading shape\n", nil},
		{"indented-heading-is-content", "    ## deeply indented\n", nil},
		{"fenced-backtick-heading", "```\n## Run halted\n```\n", nil},
		{"fenced-tilde-heading", "~~~\n## Run halted\n~~~\n", nil},
		{"fenced-crlf-heading", "```\r\n## Run halted\r\n```\r\n", nil},
		{"longer-fence-swallows-shorter-close", "````\n```\n## inside\n````\n", nil},
		{"other-char-run-does-not-close", "```\n~~~\n## inside\n```\n", nil},
		{"close-may-be-longer-than-open", "```\ntext\n`````\n### fine\n", nil},

		{"leading-bare-halt-heading", "## Run halted\n\nreport\n", ErrSectionBodyHeading},
		{"later-bare-halt-heading", "prose first\n\n## Run halted\n\nmore\n", ErrSectionBodyHeading},
		{"dated-halt-heading", "## Run halted — 2026-08-26\n\nreport\n", ErrSectionBodyHeading},
		{"arbitrary-structural-h2", "report\n\n## Notes\n", ErrSectionBodyHeading},
		{"crlf-structural-h2", "report\r\n\r\n## Notes\r\n", ErrSectionBodyHeading},
		{"heading-after-closed-fence", "```\nx\n```\n## Escaped\n", ErrSectionBodyHeading},

		{"unterminated-backtick-fence", "```\n## hidden\n", ErrSectionBodyUnterminatedFence},
		{"unterminated-tilde-fence", "~~~\ntext\n", ErrSectionBodyUnterminatedFence},
		{"inner-shorter-run-never-closes", "````\ntext\n```\n", ErrSectionBodyUnterminatedFence},
		{"unterminated-crlf-fence", "```\r\ntext\r\n", ErrSectionBodyUnterminatedFence},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ValidateSectionBody([]byte(tc.body)); !errors.Is(got, tc.want) {
				t.Errorf("ValidateSectionBody(%q) = %v, want %v", tc.body, got, tc.want)
			}
		})
	}
}

// TestValidateSectionBodyErrorsCarryNoBodyContent proves the sentinel error
// strings are static: an authored marker string never appears in the error a
// caller might surface (redaction is a diagnostics contract, not a courtesy).
func TestValidateSectionBodyErrorsCarryNoBodyContent(t *testing.T) {
	for _, body := range []string{"## zz-authored-marker-zz\n", "```\nzz-authored-marker-zz\n"} {
		err := ValidateSectionBody([]byte(body))
		if err == nil {
			t.Fatalf("body %q unexpectedly valid", body)
		}
		if strings.Contains(err.Error(), "zz-authored-marker-zz") {
			t.Errorf("error echoes body content: %q", err.Error())
		}
	}
}
```

Note the tricky rows before trusting them against your implementation:
- `other-char-run-does-not-close`: inside an open backtick fence, a `~~~` line is neither an opener nor a closer (`run[0] == fenceChar` fails in `scanH2Headings`'s close condition), so the fence stays open until the final ```` ``` ````. Valid.
- `longer-fence-swallows-shorter-close`: a ```` ```` ```` opener is only closed by a run of ≥ 4 backticks, so the inner ```` ``` ```` and the heading are content.
- `close-may-be-longer-than-open`: the scanner accepts a longer bare run as a closer (`len(run) >= len(fence)`), so what follows is outside the fence — and `### fine` is H3, not structural.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/render/ -run 'TestValidateSectionBody' -count=1`
Expected: FAIL to compile — `ValidateSectionBody`, `ErrSectionBodyHeading`, `ErrSectionBodyUnterminatedFence` undefined.

- [ ] **Step 3: Implement the validator**

In `internal/render/section.go`, add `"errors"` to the import block. Then change `scanH2Headings` into a wrapper over a variant that also returns the scanner's final fence state — the body of the loop is **moved verbatim**, not rewritten:

```go
// scanH2Headings returns the top-level "## " headings in src, in source order,
// skipping fenced code blocks so that marker-shaped or heading-shaped example
// text inside a fence is treated as authored content.
func scanH2Headings(src []byte) []h2Heading {
	heads, _ := scanH2HeadingsFinalFence(src)
	return heads
}

// scanH2HeadingsFinalFence is scanH2Headings plus the scanner's final fence
// state: the still-open fence's delimiter run at end of input, or "" when every
// fence closed. ValidateSectionBody keys its unterminated-fence rejection on
// this so "closed" means exactly what the section scanner would treat as closed.
func scanH2HeadingsFinalFence(src []byte) ([]h2Heading, string) {
	var heads []h2Heading
	fence := ""          // the open fence's delimiter run; "" when not inside a fence
	fenceChar := byte(0) // '`' or '~'
	for _, ln := range splitLines(src) {
		text := src[ln.start:ln.textEnd]
		if run, ok := fenceRun(text); ok {
			switch {
			case fence == "":
				fence, fenceChar = run, run[0]
			case run[0] == fenceChar && len(run) >= len(fence) && isBareFence(text, run):
				fence, fenceChar = "", 0
			}
			continue
		}
		if fence != "" {
			continue // inside a fenced code block: heading-shaped text is content
		}
		if bytes.HasPrefix(text, h2Prefix) {
			heads = append(heads, h2Heading{heading: string(text), start: ln.start})
		}
	}
	return heads, fence
}
```

Then add the sentinels and the entry point (near `ApplySectionEdits`):

```go
// Section-body validation sentinels. The strings are static by contract: a
// caller may surface them verbatim, so they must never carry authored content.
var (
	// ErrSectionBodyHeading: the body carries a column-zero "## " heading the
	// section scanner would recognize outside fenced code — it would become a
	// structural section of its own, escaping the enclosing owned section.
	ErrSectionBodyHeading = errors.New("section body carries a column-zero \"## \" heading outside fenced code")
	// ErrSectionBodyUnterminatedFence: a code fence is still open at end of
	// body — once embedded, it would swallow the next section's heading and
	// hide it from the recovery scanner.
	ErrSectionBodyUnterminatedFence = errors.New("section body leaves a code fence unterminated at end of input")
)

// ValidateSectionBody reports whether body is safe to embed as ONE owned
// section's body: no structural top-level heading outside fenced code, and no
// fence left open at end of input. It applies the same scanner ApplySectionEdits
// uses (scanH2Headings' fence-aware rules), so "valid" means the recovery
// scanner will later see exactly one section. Errors are errors.Is-comparable
// sentinels and never echo body content. ApplySectionEdits itself is unchanged:
// callers opt in at their own write boundary.
func ValidateSectionBody(body []byte) error {
	heads, fence := scanH2HeadingsFinalFence(body)
	if len(heads) > 0 {
		return ErrSectionBodyHeading
	}
	if fence != "" {
		return ErrSectionBodyUnterminatedFence
	}
	return nil
}
```

- [ ] **Step 4: Run the new tests and the whole render package**

Run: `go test ./internal/render/ -count=1`
Expected: PASS — including every pre-existing `TestApplySectionEdits*` test (the wrapper refactor must be behavior-neutral).

- [ ] **Step 5: Mutation-test the two rejection branches**

Each probe: apply the mutation, run `go test ./internal/render/ -run TestValidateSectionBody -count=1`, confirm FAIL, restore the exact original text (re-apply from this plan, not `git checkout`, if you have other uncommitted edits in the file — learnings: `mutation-restore-needs-a-backup-copy`), re-run to confirm PASS.

1. In `ValidateSectionBody`, replace `if len(heads) > 0` with `if false` → every `ErrSectionBodyHeading` row must redden.
2. Replace `if fence != ""` with `if false` → every `ErrSectionBodyUnterminatedFence` row must redden.
3. In `scanH2HeadingsFinalFence`, change the close condition's `len(run) >= len(fence)` to `len(run) > 0` (any bare same-char run closes) → `longer-fence-swallows-shorter-close` (now the heading escapes) and `inner-shorter-run-never-closes` must redden. This proves the fence distinction is exercised, not just present.

- [ ] **Step 6: Commit**

```bash
git add internal/render/section.go internal/render/section_test.go
git commit -m "feat(0354): render.ValidateSectionBody - fence-aware section-body validator"
```

---

### Task 2: Wire the validator into `validateHaltShape`

**Files:**
- Modify: `internal/app/change_halt.go` (the `validateHaltShape` function; add `"errors"` import; add helper `haltReportBodyDiagnostic`)
- Test: `internal/app/change_halt_test.go`

**Interfaces:**
- Consumes: `render.ValidateSectionBody`, `render.ErrSectionBodyHeading`, `render.ErrSectionBodyUnterminatedFence` (Task 1); existing `lifecycleFinding`, `FCInvalidSectionMarkdown`, `StatusFinding.Field`.
- Produces: `ChangeHalt` returns `ResultInvalidInput` carrying one `StatusFinding{Code: "invalid-section-markdown", Field: "report"}` for a structurally malformed report, **before** `PinContext`, corpus reads, or the engine. Tasks 3–4 rely on this envelope shape.

- [ ] **Step 1: Write the failing tests**

Append to `internal/app/change_halt_test.go` (it already imports `context`, `strings`, `testing`):

```go
// --- halt report-body validation (change 0354) ------------------------------

// TestValidateHaltShapeRejectsStructuralReport proves the report-body gate: a
// report that would create another structural H2 (bare or dated halt heading,
// or any other column-zero "## " heading outside fenced code) or leave a fence
// unterminated yields exactly one invalid-section-markdown finding on field
// "report", and the diagnostic never echoes the authored bytes.
func TestValidateHaltShapeRejectsStructuralReport(t *testing.T) {
	cases := map[string]string{
		"leading-bare-halt-heading": "## Run halted\n\nzz-authored-marker-zz\n",
		"later-bare-halt-heading":   "zz-authored-marker-zz\n\n## Run halted\n",
		"dated-halt-heading":        "## Run halted — 2026-08-26\n\nzz-authored-marker-zz\n",
		"arbitrary-structural-h2":   "zz-authored-marker-zz\n\n## Notes\n",
		"unterminated-fence":        "```\nzz-authored-marker-zz\n",
	}
	for name, report := range cases {
		t.Run(name, func(t *testing.T) {
			findings := validateHaltShape(HaltRequest{ID: 3, Version: blobV, Report: report})
			if len(findings) != 1 {
				t.Fatalf("findings = %+v, want exactly one", findings)
			}
			f := findings[0]
			if f.Code != string(FCInvalidSectionMarkdown) || f.Field != "report" {
				t.Errorf("finding code=%q field=%q, want %q/report", f.Code, f.Field, FCInvalidSectionMarkdown)
			}
			if strings.Contains(f.Message, "zz-authored-marker-zz") {
				t.Errorf("diagnostic echoes authored report: %q", f.Message)
			}
		})
	}
}

// TestValidateHaltShapeAcceptsValidBodies proves the body-only contract's
// positive side: prose, lists, H3-or-deeper subsections, and heading examples
// inside closed backtick and tilde fences (LF and CRLF, scanner-sensitive
// fence lengths) pass the shape check untouched.
func TestValidateHaltShapeAcceptsValidBodies(t *testing.T) {
	cases := map[string]string{
		"prose":                "Suite red on internal/app; see run 7.\n",
		"list-and-h3":          "### Symptoms\n\n- red suite\n- stale lease\n",
		"fenced-backtick":      "example:\n\n```\n## Run halted\n```\n",
		"fenced-tilde":         "~~~\n## Run halted\n~~~\n",
		"fenced-crlf":          "```\r\n## Run halted\r\n```\r\n",
		"fence-length-shelter": "````\n```\n## inside\n````\n",
	}
	for name, report := range cases {
		t.Run(name, func(t *testing.T) {
			if findings := validateHaltShape(HaltRequest{ID: 3, Version: blobV, Report: report}); len(findings) != 0 {
				t.Errorf("valid body refused: %+v", findings)
			}
		})
	}
}

// TestChangeHaltValidatesBeforeAnyEffect proves ordering: with ZERO deps (a nil
// Reader, Engine, and Client — any pin, corpus read, or engine touch would
// panic), a malformed report still returns a clean invalid-input envelope, so
// the validation runs before repository preparation and the transaction engine.
func TestChangeHaltValidatesBeforeAnyEffect(t *testing.T) {
	got := ChangeHalt(context.Background(), PlanningDeps{}, "",
		HaltRequest{ID: 3, Version: blobV, Report: "## Run halted\n\nzz-authored-marker-zz\n"})
	if got.Result != ResultInvalidInput {
		t.Fatalf("result = %q, want %q", got.Result, ResultInvalidInput)
	}
	if len(got.Findings) != 1 || got.Findings[0].Code != string(FCInvalidSectionMarkdown) || got.Findings[0].Field != "report" {
		t.Fatalf("findings = %+v, want one invalid-section-markdown on field report", got.Findings)
	}
}
```

(`blobV` is the package's existing 40-hex test version constant, already used by `TestChangeResumeHaltedRequiresAcknowledgement`.)

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/app/ -run 'TestValidateHaltShape|TestChangeHaltValidatesBeforeAnyEffect' -count=1`
Expected: FAIL — the structural cases produce zero findings today (`validateHaltShape` checks only emptiness and size), so `TestValidateHaltShapeRejectsStructuralReport` and `TestChangeHaltValidatesBeforeAnyEffect` redden; `TestValidateHaltShapeAcceptsValidBodies` passes (it pins current-and-future behavior).

- [ ] **Step 3: Implement the wiring**

In `internal/app/change_halt.go`, add `"errors"` to the import block, then extend `validateHaltShape` and add the diagnostic mapper:

```go
// validateHaltShape runs the configuration-independent request checks for
// `change halt`. Beyond presence and size, the report BODY must be embeddable
// as one owned section: the operation alone owns the "## Run halted" H2 and
// the dated sub-heading, so a body carrying its own structural H2 or an
// unterminated code fence is refused here — before repository preparation,
// the transaction engine, or any metadata effect (change 0354).
func validateHaltShape(req HaltRequest) []StatusFinding {
	findings := dropFindingCode(validateLifecycleShape("id", req.ID, "", req.Version), FCEmptyPath)
	if strings.TrimSpace(req.Report) == "" {
		findings = append(findings, lifecycleFinding(FCEmptyReport, "report must be a non-empty authored bounded halt report"))
	}
	boundAuthored(&findings, "report", req.Report)
	if err := render.ValidateSectionBody([]byte(req.Report)); err != nil {
		f := lifecycleFinding(FCInvalidSectionMarkdown, haltReportBodyDiagnostic(err))
		f.Field = "report"
		findings = append(findings, f)
	}
	return findings
}

// haltReportBodyDiagnostic maps a section-body validation error onto an
// actionable diagnostic. The text is static by contract — it never echoes the
// authored report (HaltResult redaction).
func haltReportBodyDiagnostic(err error) string {
	if errors.Is(err, render.ErrSectionBodyUnterminatedFence) {
		return "report leaves a code fence unterminated; close the fence so the sections after the halt marker stay visible to recovery"
	}
	return "report carries a column-zero \"## \" heading outside fenced code; the operation owns the \"## Run halted\" heading and its dated sub-heading — author body text, lists, or \"###\"-or-deeper subsections, and put heading examples inside closed code fences"
}
```

`ChangeHalt` needs no change: its first statement already routes non-empty `validateHaltShape` findings to `ResultInvalidInput` before `haltPinAndFence`.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/app/ -run 'TestValidateHaltShape|TestChangeHaltValidatesBeforeAnyEffect|TestChangeHalt' -count=1`
Expected: PASS — including the pre-existing `TestChangeHaltPreservesCheckpoints` (its report `"Blocked on infra; see run 7.\n"` is a valid body).

- [ ] **Step 5: Mutation-test the wiring**

Delete the entire `if err := render.ValidateSectionBody(...)` block from `validateHaltShape`, run `go test ./internal/app/ -run 'TestValidateHaltShape|TestChangeHaltValidatesBeforeAnyEffect' -count=1`, confirm FAIL (every structural case), restore, confirm PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/app/change_halt.go internal/app/change_halt_test.go
git commit -m "fix(0354): change.halt refuses report bodies carrying structural H2s or open fences"
```

---

### Task 3: CLI end-to-end diagnostic assertion

**Files:**
- Test: `internal/cli/change_test.go` (append near `TestChangeHaltReachesOperation`)

**Interfaces:**
- Consumes: the package's existing `runCLIStdin` helper and `testsupport.TempDir`; Task 2's envelope shape (`"result":"invalid-input"`, finding `"code":"invalid-section-markdown"`, `"field":"report"`).
- Produces: nothing new — an end-to-end guard only.

- [ ] **Step 1: Write the failing-if-unwired test**

```go
// TestChangeHaltRejectsStructuralReport proves the end-to-end diagnostic: a
// report body carrying its own structural "## " heading is refused as
// invalid-input with the invalid-section-markdown finding on field "report",
// before any repository work (the repo-dir is an empty temp dir), and the
// document never echoes the authored report bytes (change 0354).
func TestChangeHaltRejectsStructuralReport(t *testing.T) {
	out, errS, _ := runCLIStdin(t, `{"report":"## Run halted\n\nzz-authored-marker-zz\n"}`, "change", "halt",
		"--id", "3", "--version", "1234123412341234123412341234123412341234",
		"--input", "-", "--repo-dir", testsupport.TempDir(t), "--json")
	if errS != "" {
		t.Fatalf("unexpected stderr %q", errS)
	}
	for _, want := range []string{`"result":"invalid-input"`, `"code":"invalid-section-markdown"`, `"field":"report"`} {
		if !strings.Contains(out, want) {
			t.Errorf("document missing %s: %q", want, out)
		}
	}
	if strings.Contains(out, "zz-authored-marker-zz") {
		t.Errorf("document echoes the authored report: %q", out)
	}
}
```

Note the request rides stdin as a JSON string, so the heading reaches the operation as real newline-separated Markdown (`\n` escapes decode inside the JSON value).

- [ ] **Step 2: Run the test**

Run: `go test ./internal/cli/ -run TestChangeHaltRejectsStructuralReport -count=1`
Expected: PASS immediately if Tasks 1–2 landed. Then mutation-check its key: re-apply Task 2's Step-5 mutation (delete the `ValidateSectionBody` block from `validateHaltShape`), re-run, confirm FAIL (learnings: `plan-supplied-test-code-is-unverified` — prove the assert CAN redden), restore, confirm PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/cli/change_test.go
git commit -m "test(0354): CLI end-to-end diagnostic for a structurally malformed halt report"
```

---

### Task 4: Integration proof — cycle, re-halt, no-effects, corrupted-record refusal

**Files:**
- Test: `internal/app/change_integration_test.go` (`//go:build integration`; append near `TestIntegrationChangeResumeHalted`)

**Interfaces:**
- Consumes: existing integration helpers, all already used by `TestIntegrationChangeResumeHalted` in the same file and by `setupHaltedFixture` in `change_halt_test.go`: `planRepoModes`, `setupRebaseFixtureStatus`, `setupHaltedFixture`, `lifecycleChange`, `groomPath`, `originFile`, `blobVersionAt`, `writerAdvance`, `fakeResumeWorkspace`, `ChangeHalt`, `ChangeResumeHalted`, `workspace.StateReady`.
- Produces: nothing new — durable regression guards only.

- [ ] **Step 1: Write the tests**

```go
// TestIntegrationChangeHaltResumeCycle proves the whole contract over real git:
// a real halt write on a record that already carries a halted marker AND a
// section AFTER it (so fence or boundary leakage cannot pass unnoticed)
// replaces the section in place — one structural halt heading, one
// operation-generated date wrapper, the authored fenced example intact — and
// an authorized quiescent resume then removes the COMPLETE report, fenced
// bytes included, preserving the surrounding sections byte-for-byte
// (change 0354).
func TestIntegrationChangeHaltResumeCycle(t *testing.T) {
	for _, m := range planRepoModes() {
		t.Run(m.name, func(t *testing.T) {
			f := setupHaltedFixture(t, m)
			recPath := groomPath(f.id, f.slug)

			// Seed a section AFTER the halt section: the leakage tripwire.
			pre, _ := originFile(t, f.repo.origin, f.branch, recPath)
			f.repo.writerAdvance(t, f.branch, map[string]string{
				recPath: strings.TrimRight(pre, "\n") + "\n\n## Follow-up notes\n\nKeep me byte-identical.\n",
			})
			f.version = blobVersionAt(t, f.repo.origin, f.branch, recPath)

			// Re-halt with a valid body carrying a fenced heading example.
			report := "Wedged on resume.\n\n```\n## Run halted\n```\n"
			halted := ChangeHalt(context.Background(), f.deps, f.repo.invocation,
				HaltRequest{ID: f.id, Version: f.version, Report: report})
			if halted.Result != ResultApplied || halted.Disposition != HaltDispHalted {
				t.Fatalf("halt: result=%q disp=%q reason=%q", halted.Result, halted.Disposition, halted.Reason)
			}

			rec, _ := originFile(t, f.repo.origin, f.branch, recPath)
			// Exactly one structural halt heading: the operation's own, at column
			// zero. The fenced example also contains the heading BYTES, so count
			// structural occurrences (line starts) minus the fenced one by shape:
			// the structural heading is followed by the dated H3 the operation owns.
			if got := strings.Count(rec, "\n## Run halted\n"); got != 1 {
				t.Errorf("structural halt headings = %d, want 1:\n%s", got, rec)
			}
			if got := strings.Count(rec, "\n### 2026-08-16\n"); got != 1 {
				t.Errorf("operation date wrappers = %d, want exactly 1:\n%s", got, rec)
			}
			for _, want := range []string{"Wedged on resume.", "```\n## Run halted\n```", "## Follow-up notes\n\nKeep me byte-identical.\n"} {
				if !strings.Contains(rec, want) {
					t.Errorf("halt lost content %q:\n%s", want, rec)
				}
			}

			// Authorized quiescent resume removes the COMPLETE report.
			version := blobVersionAt(t, f.repo.origin, f.branch, recPath)
			resumed := ChangeResumeHalted(context.Background(), f.deps,
				WorkspaceDeps{Service: fakeResumeWorkspace{kind: workspace.StateReady, head: f.head}}, f.repo.invocation,
				ResumeRequest{ID: f.id, Version: version, AcknowledgeQuiescent: true})
			if resumed.Result != ResultApplied || resumed.Disposition != HaltDispResumed {
				t.Fatalf("resume: result=%q disp=%q reason=%q", resumed.Result, resumed.Disposition, resumed.Reason)
			}
			final, _ := originFile(t, f.repo.origin, f.branch, recPath)
			for _, gone := range []string{"## Run halted", "Wedged on resume.", "```"} {
				if strings.Contains(final, gone) {
					t.Errorf("resume left report residue %q:\n%s", gone, final)
				}
			}
			for _, kept := range []string{"## Why\n\nOriginal why.", "## Follow-up notes\n\nKeep me byte-identical.\n"} {
				if !strings.Contains(final, kept) {
					t.Errorf("resume lost surrounding content %q:\n%s", kept, final)
				}
			}
		})
	}
}

// TestIntegrationChangeHaltMalformedReportHasNoEffects proves the refusal is
// effect-free over real git: an invalid-input halt leaves the origin record
// byte-identical and creates no commit (change 0354).
func TestIntegrationChangeHaltMalformedReportHasNoEffects(t *testing.T) {
	for _, m := range planRepoModes() {
		t.Run(m.name, func(t *testing.T) {
			f := setupRebaseFixtureStatus(t, m, "in-progress")
			recPath := groomPath(f.id, f.slug)
			before, _ := originFile(t, f.repo.origin, f.branch, recPath)
			got := ChangeHalt(context.Background(), f.deps, f.repo.invocation,
				HaltRequest{ID: f.id, Version: f.version, Report: "## Run halted\n\ndoubled\n"})
			if got.Result != ResultInvalidInput {
				t.Fatalf("result=%q, want %q", got.Result, ResultInvalidInput)
			}
			if got.Revision != "" {
				t.Errorf("a refused halt reported a committed revision %q", got.Revision)
			}
			after, _ := originFile(t, f.repo.origin, f.branch, recPath)
			if before != after {
				t.Errorf("refused halt changed the record:\nbefore:\n%s\nafter:\n%s", before, after)
			}
		})
	}
}

// TestIntegrationChangeHaltCorruptedRecordStillRefused proves the existing
// duplicate-owned-heading guard survives: a record SEEDED with two halt
// sections (the historical corruption this change prevents, not repairs)
// still refuses a new, valid halt write and writes nothing (change 0354).
func TestIntegrationChangeHaltCorruptedRecordStillRefused(t *testing.T) {
	for _, m := range planRepoModes() {
		t.Run(m.name, func(t *testing.T) {
			f := setupRebaseFixtureStatus(t, m, "in-progress")
			recPath := groomPath(f.id, f.slug)
			corrupted := strings.TrimRight(lifecycleChange(f.id, f.slug, "in-progress"), "\n") +
				"\n\n## Run halted\n\n### 2026-08-14\n\nFirst.\n\n## Run halted\n\n### 2026-08-15\n\nSecond.\n"
			f.repo.writerAdvance(t, f.branch, map[string]string{recPath: corrupted})
			f.version = blobVersionAt(t, f.repo.origin, f.branch, recPath)

			got := ChangeHalt(context.Background(), f.deps, f.repo.invocation,
				HaltRequest{ID: f.id, Version: f.version, Report: "A valid body.\n"})
			if got.Result == ResultApplied || got.Disposition == HaltDispHalted {
				t.Fatalf("halt applied over a corrupted record: result=%q disp=%q", got.Result, got.Disposition)
			}
			after, _ := originFile(t, f.repo.origin, f.branch, recPath)
			if after != corrupted {
				t.Errorf("refused halt changed the corrupted record")
			}
		})
	}
}
```

Adaptation notes for the implementer (verify, don't assume — `verify-the-claim`):
- `setupHaltedFixture` lives in `change_halt_test.go` (no build tag) and IS visible to the tagged integration file only if both compile together — they do: the integration build includes untagged files. Confirm with `go vet -tags integration ./internal/app/`.
- The fixture's clock stamps `2026-08-16` (the existing resume test asserts `claimed_at: '2026-08-16T12:00:00Z'`); if the date-wrapper assert reddens on the date, read the fixture's `testClock()` and pin the actual date — the invariant under test is "exactly one operation-generated `### <date>` wrapper", not which date.
- The exact refusal surface in the corrupted-record test is `ApplySectionEdits`'s duplicate-owned-heading error routed through `refuseHalt("marker-edit-failed", ...)`; assert on result/disposition and the unchanged record, not on the message string.
- If `f.head` is not a field the cycle test can reach, mirror how `TestIntegrationChangeResumeHalted` obtains it in this same file.

- [ ] **Step 2: Run the tests**

Run: `go test -tags integration ./internal/app/ -run 'TestIntegrationChangeHaltResumeCycle|TestIntegrationChangeHaltMalformed|TestIntegrationChangeHaltCorrupted' -count=1`
Expected: PASS. Then prove the cycle test's key can redden: temporarily change the cycle test's seeded report to `"## Run halted\n\nWedged.\n"` — the halt call must now refuse and the test must FAIL at the `halt:` assert; restore.

- [ ] **Step 3: Commit**

```bash
git add internal/app/change_integration_test.go
git commit -m "test(0354): halt-resume cycle, no-effects refusal, and corrupted-record guards over real git"
```

---

### Task 5: State the body-only contract in the skill and request documentation

**Files:**
- Modify: `skills/docket-implement-next/SKILL.md` (the Step-3 halt-authoring clause — the sentence beginning "**write that halt into git before stopping**: the `change.halt` operation")
- Modify: `internal/app/change_halt.go` (the `HaltRequest` doc comment — the schema registry (`internal/app/schema_registry.go` entry `{ID: "change.halt", Request: HaltRequest{}...}`) derives the request surface from this struct)
- Modify: `internal/cli/change.go` (the `changeHaltInput` doc comment)

**Interfaces:** documentation only; no code surface changes.

- [ ] **Step 1: Update the Step-3 halt clause in `skills/docket-implement-next/SKILL.md`**

In the clause that reads "...records a bounded authored run-halted report into a `## Run halted` section on the in-progress change in one exact-version transaction, leaving the branch, lease, workspace, and evidence untouched (the authored Markdown travels as a request file, never a shell-escaped flag; a non-in-progress record is refused, writing nothing).", insert immediately after that parenthetical:

```markdown
The `report` is the section **body only**: the operation alone writes the `## Run halted` heading and the dated `###` sub-heading, so the body starts with prose, a list, or a `###`-or-deeper subsection — a valid report body is, e.g., `Suite red on internal/app after task 3; evidence at run 7.` A body carrying any column-zero `## ` heading of its own (a repeated or dated halt heading included) or an unterminated code fence is refused as `invalid-input` (`invalid-section-markdown`, field `report`), writing nothing — put heading examples inside closed code fences and report the failed halt rather than claiming durable halt state exists.
```

- [ ] **Step 2: Update the two request-documentation comments**

`internal/app/change_halt.go`, `HaltRequest` doc comment — replace the sentence "Report is the authored bounded halt report recorded in the marker." with:

```go
// exact submitted record; Report is the authored bounded halt report recorded
// in the marker — the section BODY only, starting with prose, a list, or a
// "###"-or-deeper subsection. The operation alone owns the "## Run halted"
// heading and the dated sub-heading; a body carrying its own column-zero "## "
// heading outside fenced code, or an unterminated code fence, is refused as
// invalid-input (invalid-section-markdown, field "report") before any effect.
```

`internal/cli/change.go`, `changeHaltInput` doc comment — extend it with one sentence:

```go
// changeHaltInput is the bounded request-file payload for `change halt`: the
// authored run-halted report — the section body only (the operation owns the
// "## Run halted" heading and dated sub-heading; a body with its own
// column-zero "## " heading or an open code fence is refused). The scalar
// identity (id, version) rides on flags — only the authored Markdown travels
// through the request file (Global Constraints).
```

- [ ] **Step 3: Sweep maintained callers for conflicting guidance**

Run (capture to a variable first if piping — pipefail rule):

```bash
grep -rn "Run halted" skills/ internal/ cmd/ README.md docs/ --include='*.md' --include='*.go' | grep -v -e '_test.go' -e 'docs/changes/' -e 'docs/superpowers/specs/' -e 'docs/superpowers/plans/' -e 'docs/adrs/'
```

Sort hits into maintained source vs point-in-time records; edit **only maintained** surfaces, and only where they actually instruct a caller to author the heading or contradict the body-only contract. Expected findings (verify each against its text, fix only real conflicts):
- `skills/docket-convention/SKILL.md` `## Run halted` bullet — already states the heading is bare and the section is written by the run; consistent, likely no edit.
- `skills/docket-convention/references/dummy-mode.md` — the `### In plain terms` block is H3, inside the body; consistent, no edit.
- `skills/docket-implement-next/references/edge-paths.md` and the SKILL.md derived-views paragraph ("The skill authors the halt report as a request file and hands it across") — consistent, no edit.
- Anything that shows a halt report EXAMPLE containing its own `## Run halted` line is a real conflict: rewrite the example as body-only.

- [ ] **Step 4: Verify the docs land clean**

Run: `go test ./internal/app/ ./internal/cli/ -count=1` (comment edits must not break compilation) and re-read the edited SKILL.md clause in place for marker/format integrity.

- [ ] **Step 5: Commit**

```bash
git add skills/docket-implement-next/SKILL.md internal/app/change_halt.go internal/cli/change.go
git commit -m "docs(0354): state the halt report body-only contract in skill and request docs"
```

(Stage exactly these paths plus any file Step 3 actually edited.)

---

### Task 6: Full-suite build gate

**Files:** none (verification only)

- [ ] **Step 1: Run the whole suite**

From the feature-worktree root, run the build gate's resolved suite command (from `build.test_command` — never a second copy): `go run ./cmd/docket development test`

Expected: SUITE pass. Treat any `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` line as a screening finding and a `SERIAL CONFIRMED OVER BUDGET:` line as an authoritative breach to act on (`tests/README.md` owns the confirm procedure).

- [ ] **Step 2: Fix anything red, re-run, and leave the tree clean and committed**

No uncommitted files; every change from Tasks 1–5 is in its own commit on `fix/halt-report-authoring-writes-a-duplicate-run-halted-heading`.

---

## Self-Review (completed at authoring)

- **Spec coverage:** report-body validator backed by the existing scanner + final-fence exposure (Task 1); called from `validateHaltShape` before pin/engine with `invalid-input` / `invalid-section-markdown` / field `report`, no report echo (Task 2); CLI end-to-end diagnostic (Task 3); halt→resume cycle with a following section, re-halt single-section replacement, no-effects refusal, seeded-corruption still refused by the duplicate-owned-heading guard (Task 4); skill + request documentation body-only contract with a valid example and a maintained-caller sweep (Task 5); mutation tests with uncached execution (Tasks 1–3); full suite at the gate (Task 6). `ApplySectionEdits` callers untouched; no auto-repair; resume safeguards untouched.
- **Placeholder scan:** none — every step carries its code or exact command.
- **Type consistency:** `ValidateSectionBody([]byte) error`, `ErrSectionBodyHeading`, `ErrSectionBodyUnterminatedFence`, `scanH2HeadingsFinalFence` are spelled identically in Tasks 1–2; finding shape `Code`/`Field`/`Message` matches `StatusFinding` in `internal/app/status_result.go`.
