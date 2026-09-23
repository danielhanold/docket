<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0414 — Results placeholder heuristic false-positives on uppercase HTML tags and URI schemes](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-23-0414-results-placeholder-heuristic-false-positives-on-uppercase-h.md)**
<!-- docket:backlink:end -->
# Placeholder Heuristic Corrections (Change 0414) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop plan attachment and results validation from rejecting valid content — whole-word planning-token prose (todo/fixme/tbd/tktk/xxx/placeholder, uppercase in real documents) in plans, and uppercase HTML tags / URI schemes in results — while still rejecting genuinely unfilled slots and raw template scaffolding.

**Architecture:** Two focused corrections behind existing seams. (1) Plan attachment replaces the whole-blob `placeholderTokenRE` scan with a whole-slot check: only a section (or pre-heading preamble) whose *entire* authored body reduces to one bare placeholder token refuses, under the existing `placeholder-token` reason. (2) Results validation replaces the "`<` + uppercase letter" capitalization guess with prompts *derived from the shipped embedded results template* (`internal/assets.Open`), matched as whitespace-normalized substrings outside code, comments, frontmatter, and managed blocks. Both reuse `internal/app`'s existing line/fence/heading machinery; no new Markdown framework, no configuration, no new ADR.

**Tech Stack:** Go (stdlib only — `strings`, `bytes`, `regexp`, `sync`), `internal/document`, `internal/assets`, `go test`.

**Spec:** `docs/superpowers/specs/2026-09-22-results-placeholder-heuristic-false-positives-on-uppercase-h-design.md` (on the `docket` metadata branch; also readable at `.docket/docs/superpowers/specs/…` from the primary tree). Change file: `docs/changes/active/0414-results-placeholder-heuristic-false-positives-on-uppercase-h.md` (same branch).

## Global Constraints

- **Self-hosting constraint — read first.** This plan file is itself attached through `change.attach-plan` under the *currently installed* binary, which still refuses any plan whose bytes contain an uppercase whole-word planning token. Therefore this plan never spells those six tokens in uppercase, and every test fixture that needs one constructs it at runtime through the `tok(...)` helper defined in Task 1 (`strings.ToUpper` over the lowercase spelling). Keep that convention when adjusting fixtures: the *test files* may technically carry uppercase spellings, but the fixtures as planned use `tok(...)` so the code the plan quotes and the code you write stay identical. The retired regex is case-sensitive, so lowercase mentions in this plan and in comments are safe.
- No new dependencies, no new configuration keys, no new lifecycle states, no new ADR (spec: "No new ADR is required unless implementation discovers a material departure").
- No hand-enumerated list of HTML tags, URI schemes, or template phrases anywhere (ADR-0050 / enumerated-floor).
- Reason-string vocabulary is additive only. Preserve `placeholder-token` for plan filler and `results-placeholder` for results scaffolding; the one new reason is `results-template-invalid` (validator setup failure).
- Do not run `ResultsPhaseCheckpoint`/`ResultsPhaseFinal` rules on plans; do not apply the plan token-slot rule to results.
- Keep `change.attach-plan`, `change.attach-results`, `ChangeMarkImplemented`, `RunVerify` at their existing validation seams; commit identity, single-artifact delta, path trailer, backlink, exact-version transactions, checkpoint/final split, and evidence rules are untouched.
- Every new guard is mutation-tested (strip it, watch tests redden, restore from a backup copy — never `git checkout --` over uncommitted work) and every `go test` re-run during mutation probes passes `-count=1` (Go's result cache otherwise serves the pre-mutation tree).
- No retrospective edits to historical artifacts: files under `docs/superpowers/plans/` (other than this plan), archived changes, specs, results, and Accepted ADRs keep their existing text even where it mentions the old heuristics.
- The final gate runs the WHOLE suite via the command `build.test_command` resolves to (read it from config — `.docket.yml` / `docket` config resolution — never a second copy), from this feature checkout, and reads the budget report even on green.

---

### Task 1: Plan whole-slot filler detector (`planPlaceholderSlot`)

**Files:**
- Create: `internal/app/plan_content.go`
- Create: `internal/app/plan_content_test.go`
- Modify: `internal/app/results_content.go` (extract the fence-classification loop into a shared `fenceMask` helper; add `frontmatterEnd`)

**Interfaces:**
- Consumes: `splitResultsLines`, `parseResultsHeading`, `fenceRunLen`, `document.Parse` (all existing in `internal/app` / `internal/document`).
- Produces: `planPlaceholderSlot(source []byte) (slot string, found bool)` — Task 2 wires this into `changeAttach`. Also `fenceMask(lines []rcLine) []bool`, `frontmatterEnd(lines []rcLine) int` (Task 4 reuses both), and the test helper `tok(s string) string` (Task 2's fixtures reuse it; `workflow_integration_test.go` is the same `app` package, so it is visible there).

Semantics (from the spec, "Plan validation"): an unfilled plan slot is a heading's **direct body** (lines strictly between it and the next heading of any level), or the document's pre-heading body, whose entire authored content — frontmatter and managed blocks excluded — reduces (whitespace-trimmed, case-insensitive, one optional terminal period) to one of the six tokens (tbd, todo, fixme, tktk, xxx, placeholder — any case). Fenced lines stay **in** the slot body (code is substantive content, and the fence marker lines themselves keep a fenced bare token from reducing to the token), but fenced lines never parse as headings. This is a closed vocabulary applied to a whole slot, never a word search inside content.

- [ ] **Step 1: Extract `fenceMask` and add `frontmatterEnd` in `results_content.go`**

Move the fence-classification loop out of `ValidateResultsContent` verbatim into a helper, upgrading it to be run-length-aware (the spec's "a shorter embedded run must not close a longer fence" — a deliberate, narrow scanner correction; CommonMark closes a fence only with an equal-or-longer run of the same character). Replace the inline loop in `ValidateResultsContent` with `fenced := fenceMask(lines)`:

```go
// fenceMask classifies each line as inside (or a marker of) a ``` / ~~~
// fenced code block. A fence closes only on a run of the SAME character at
// least as long as the run that opened it (CommonMark), so a shorter embedded
// run — a ``` example inside a ```` fence — cannot close the outer fence
// (change 0414). Fence-marker lines themselves classify as fenced.
func fenceMask(lines []rcLine) []bool {
	fenced := make([]bool, len(lines))
	fenceChar := byte(0)
	fenceLen := 0
	for i, ln := range lines {
		trimmed := strings.TrimLeft(strings.TrimRight(ln.text, "\r"), " \t")
		if run := fenceRunLen(trimmed); run >= 3 {
			if fenceChar == 0 {
				fenced[i] = true
				fenceChar = trimmed[0]
				fenceLen = run
				continue
			}
			fenced[i] = true
			if trimmed[0] == fenceChar && run >= fenceLen {
				fenceChar = 0
				fenceLen = 0
			}
			continue
		}
		fenced[i] = fenceChar != 0
	}
	return fenced
}

// frontmatterEnd returns the index of the first line after a leading YAML
// frontmatter block (a first line of exactly "---" closed by a later "---" or
// "..." line), or 0 when the document has none. An unclosed opener is not
// frontmatter — nothing is excluded.
func frontmatterEnd(lines []rcLine) int {
	if len(lines) == 0 || strings.TrimRight(lines[0].text, "\r") != "---" {
		return 0
	}
	for i := 1; i < len(lines); i++ {
		t := strings.TrimRight(lines[i].text, "\r")
		if t == "---" || t == "..." {
			return i + 1
		}
	}
	return 0
}
```

Run `go test ./internal/app/ -count=1` — the existing results tests must stay green after the extraction (behavior-preserving apart from the run-length correction, which no current test pins the other way; if one does, its premise is exactly what this change corrects — update it per test-premise-deleted-not-regated and say so in the commit message).

- [ ] **Step 2: Write the failing unit tests** (`internal/app/plan_content_test.go`)

```go
package app

import (
	"strings"
	"testing"
)

// tok returns the uppercase planning token named by its lowercase spelling.
// Constructed at runtime so no fixture SOURCE carries the uppercase word:
// until change 0414 merges and installs, the deployed attach path still
// refuses any plan whose bytes carry one — including this change's own plan,
// which quotes these fixtures verbatim.
func tok(s string) string { return strings.ToUpper(s) }

// Change 0414: plan attachment refuses only a WHOLE-SLOT placeholder — a
// section (or the pre-heading preamble) whose entire authored body reduces to
// one bare token — never a token mentioned inside substantive content.
func TestPlanPlaceholderSlot(t *testing.T) {
	cases := []struct {
		name     string
		src      string
		wantSlot string
		want     bool
	}{
		// Human-approved acceptance boundary (spec "Outcome and agreed boundary").
		{name: "instruction naming a token attaches",
			src:  "# Plan\n\n## Task 1\n\nRemove the " + tok("todo") + " in retry.go and replace it with bounded retry logic.\n",
			want: false},
		{name: "agreed ambiguous decision sentence attaches",
			src:  "# Plan\n\n## Error handling\n" + tok("todo") + ": decide whether failed requests should retry or stop.\n",
			want: false},
		{name: "every token legal inside substantive prose",
			src: "# Plan\n\n## T\n\nHandle " + tok("tbd") + ", " + tok("todo") + ", " + tok("fixme") + ", " +
				tok("tktk") + ", " + tok("xxx") + ", and " + tok("placeholder") + " markers found in source files.\n",
			want: false},
		{name: "token inside a fenced code example attaches",
			src:  "# Plan\n\n## T\n\n```go\n// " + tok("todo") + "\n```\n",
			want: false},
		{name: "token inside inline code attaches",
			src:  "# Plan\n\n## T\n\n`" + tok("tbd") + "`\n",
			want: false},
		{name: "empty section is not filler (authoring judgment owns it)",
			src:  "# Plan\n\n## T\n\n## U\n\nReal content.\n",
			want: false},
		{name: "token in a list item is not a bare-token slot",
			src:  "# Plan\n\n## T\n\n- " + tok("todo") + "\n",
			want: false},
		{name: "frontmatter content never classifies",
			src:  "---\nnote: " + tok("todo") + "\n---\n# Plan\n\n## T\n\nReal step.\n",
			want: false},
		{name: "fenced example plus trailing bare token is not a filler-only slot",
			// The fence lines are part of the slot body, so the body is more
			// than one bare token — substantive by the whole-slot rule.
			src:  "# Plan\n\n## T\n\n```\n## Fake heading\n```\n\n" + tok("tbd") + "\n",
			want: false},

		// Whole-slot filler refuses, naming the slot.
		{name: "bare-token section body refuses",
			src: "# Plan\n\n## Error handling\n\n" + tok("tbd") + "\n", wantSlot: `section "Error handling"`, want: true},
		{name: "case-insensitive with one terminal period",
			src: "# Plan\n\n## T\n\ntodo.\n", wantSlot: `section "T"`, want: true},
		{name: "another token as a whole slot refuses",
			src: "# Plan\n\n## A\n\n" + tok("fixme") + "\n\n## B\n\nreal\n", wantSlot: `section "A"`, want: true},
		{name: "pre-heading preamble that is only a token refuses",
			src: tok("placeholder") + "\n\n# Plan\n\n## T\n\nReal.\n", wantSlot: "the document preamble", want: true},
		{name: "H1 direct body that is only a token refuses",
			src: "# Plan\n\n" + tok("tktk") + "\n\n## T\n\nReal.\n", wantSlot: `section "Plan"`, want: true},
		{name: "CRLF filler slot refuses",
			src: "# Plan\r\n\r\n## T\r\n\r\n" + tok("xxx") + "\r\n", wantSlot: `section "T"`, want: true},

		// Fence and structure edges (section-slice-needs-a-named-terminator).
		{name: "tilde fence hides heading-shaped lines too",
			src:  "# Plan\n\n## T\n\n~~~\n## Fake\n" + tok("todo") + "\n~~~\nReal prose after.\n",
			want: false},
		{name: "shorter backtick run inside a longer fence does not close it",
			src:  "# Plan\n\n## T\n\n````\n```\n" + tok("todo") + "\n```\n````\n",
			want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			slot, found := planPlaceholderSlot([]byte(tc.src))
			if found != tc.want {
				t.Fatalf("planPlaceholderSlot(%q) found=%v want=%v (slot %q)", tc.src, found, tc.want, slot)
			}
			if found && slot != tc.wantSlot {
				t.Fatalf("planPlaceholderSlot(%q) slot=%q want %q", tc.src, slot, tc.wantSlot)
			}
		})
	}
}

// A managed (generated) block is not author content: a token inside one — the
// backlink block is the live case — never classifies as a slot body.
func TestPlanPlaceholderSlotIgnoresManagedBlocks(t *testing.T) {
	src := "<!-- docket:backlink:start (generated — do not hand-edit) -->\n> " + tok("todo") +
		"\n<!-- docket:backlink:end -->\n# Plan\n\n## T\n\nReal step.\n"
	if slot, found := planPlaceholderSlot([]byte(src)); found {
		t.Fatalf("managed-block token classified as filler (slot %q)", slot)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/app/ -run 'TestPlanPlaceholderSlot' -count=1 -v`
Expected: FAIL — `undefined: planPlaceholderSlot` (compile error).

- [ ] **Step 4: Implement `internal/app/plan_content.go`**

```go
package app

import (
	"fmt"
	"strings"

	"github.com/danielhanold/docket/internal/document"
)

// This file is the plan-side placeholder check (change 0414). change.attach-plan
// used to refuse any plan whose bytes contained a whole-word planning token
// (todo, fixme, tbd, tktk, xxx, placeholder — uppercase in real documents),
// which rejected legitimate instructions ("remove the marker in retry.go"),
// code examples, and human-approved ambiguous prose. The corrected rule
// refuses only a WHOLE-SLOT filler: a section (or the pre-heading preamble)
// whose entire authored body IS one bare token — an unfilled slot, not a
// mention. Completeness beyond that is plan authoring's and review's job;
// attachment verifies structure, never judges prose.

// planFillerTokens is the closed placeholder vocabulary a whole slot may not
// reduce to (compared lowercased). It mirrors the writing-plans
// "No Placeholders" token set; it is a slot test, never a word search.
var planFillerTokens = map[string]bool{
	"tbd": true, "todo": true, "fixme": true, "tktk": true, "xxx": true, "placeholder": true,
}

// isPlanFillerBody reports whether a slot's entire body reduces
// (whitespace-trimmed, lowercased, one optional trailing period removed) to a
// bare placeholder token. An empty body is NOT filler — empty sections stay
// with authoring/review judgment, exactly like results' isResultsFillerBody.
func isPlanFillerBody(body string) bool {
	s := strings.ToLower(strings.TrimSpace(body))
	if s == "" {
		return false
	}
	s = strings.TrimSpace(strings.TrimSuffix(s, "."))
	return planFillerTokens[s]
}

// planPlaceholderSlot reports the first unfilled slot of a committed plan: a
// heading's direct body (to the next heading of any level), or the document's
// pre-heading preamble, whose entire authored content is one bare placeholder
// token. Frontmatter and generated managed blocks are not author content;
// fenced lines never parse as headings but DO count as slot content (code is
// substantive). The returned slot names the offending section for the refusal
// message. A non-parsing document returns not-found: at the attach seam the
// backlink guard (verifyBacklink) has already refused it before this runs.
func planPlaceholderSlot(source []byte) (string, bool) {
	doc, err := document.Parse(source)
	if err != nil {
		return "", false
	}
	lines := splitResultsLines(source)
	blocks := doc.Blocks()
	inManaged := func(start int) bool {
		for _, b := range blocks {
			if start >= b.Start.Start && start < b.End.End {
				return true
			}
		}
		return false
	}
	fenced := fenceMask(lines)
	fmEnd := frontmatterEnd(lines)

	type hdr struct {
		line int
		text string
	}
	var headings []hdr
	for i, ln := range lines {
		if i < fmEnd || fenced[i] || inManaged(ln.start) {
			continue
		}
		if _, ht, ok := parseResultsHeading(strings.TrimRight(ln.text, "\r")); ok {
			headings = append(headings, hdr{line: i, text: ht})
		}
	}

	slotBody := func(from, to int) string {
		var parts []string
		for i := from; i < to; i++ {
			if i < fmEnd || inManaged(lines[i].start) {
				continue
			}
			t := strings.TrimRight(lines[i].text, "\r")
			if strings.TrimSpace(t) != "" {
				parts = append(parts, t)
			}
		}
		return strings.Join(parts, "\n")
	}

	firstHeading := len(lines)
	if len(headings) > 0 {
		firstHeading = headings[0].line
	}
	if isPlanFillerBody(slotBody(0, firstHeading)) {
		return "the document preamble", true
	}
	for hi, h := range headings {
		end := len(lines)
		if hi+1 < len(headings) {
			end = headings[hi+1].line
		}
		if isPlanFillerBody(slotBody(h.line+1, end)) {
			return fmt.Sprintf("section %q", h.text), true
		}
	}
	return "", false
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/app/ -run 'TestPlanPlaceholderSlot' -count=1 -v`
Expected: PASS (all cases).

- [ ] **Step 6: Commit**

```bash
git add internal/app/plan_content.go internal/app/plan_content_test.go internal/app/results_content.go
git commit -m "feat(app): whole-slot plan placeholder detector (change 0414)"
```

---

### Task 2: Wire the slot detector into `changeAttach`; retire `placeholderTokenRE`

**Files:**
- Modify: `internal/app/change_attach.go` (the `placeholderTokenRE` var, its file-top doc comment, the `ReasonAttachPlaceholderToken` comment, and check `(11, plan only)` inside `changeAttach`)
- Modify: `internal/app/workflow_integration_test.go` (the `"plan carries an unresolved placeholder token"` refusal row and the plan-attach success fixture)

**Interfaces:**
- Consumes: `planPlaceholderSlot(source []byte) (string, bool)` and `tok(s string) string` from Task 1.
- Produces: unchanged public surface — `ChangeAttachPlan` still refuses with `Reason == ReasonAttachPlaceholderToken` (`"placeholder-token"`), now only for whole-slot filler, with a message naming the slot.

- [ ] **Step 1: Update the integration tests first (they must fail against current code)**

In `workflow_integration_test.go`, the refusal row `"plan carries an unresolved placeholder token"` currently commits a plan whose body is one substantive "finish this section" sentence carrying the token. Its *premise* (any token word refuses) is what this change deletes, but what the row *guards* — the `placeholder-token` refusal path exists and opens no transaction — survives (test-premise-deleted-not-regated). Re-point the fixture at a genuine whole-slot filler and keep the row:

```go
		{
			name: "plan whose section body is only a placeholder token",
			build: func(t *testing.T) ChangeAttachRequest {
				f.reset(t)
				// A whole-slot filler: the section's entire body is the bare
				// token. A plan that merely MENTIONS a token attaches (the
				// success test proves that direction).
				withFillerSlot := attachBacklinkBlock(f.id, "A change", f.recPath) +
					"\n# Plan\n\n## Error handling\n\n" + tok("tbd") + "\n"
				head := f.commitPlan(t, map[string]string{f.planPath: withFillerSlot}, f.planPath)
				return ChangeAttachRequest{ID: f.id, Version: f.version, Path: f.planPath, Commit: head}
			},
			reason: ReasonAttachPlaceholderToken,
		},
```

Then extend the plan-attach **success** fixture (the test that later asserts `plan: '<path>'` reached the origin record) so its plan body contains both human-approved examples — this is the acceptance direction, proven at the real seam:

```go
	// Change 0414 acceptance: a plan that INSTRUCTS about a token, and the
	// human-approved ambiguous decision sentence, both attach.
	good := attachBacklinkBlock(f.id, "A change", f.recPath) +
		"\n# Plan\n\n## Task 1\n\nRemove the " + tok("todo") + " in retry.go and replace it with bounded retry logic.\n\n" +
		"## Error handling\n" + tok("todo") + ": decide whether failed requests should retry or stop.\n"
```

(Adapt the exact variable/fixture name to what the success test uses today — read the function around the `plan: '` assertion before editing; keep the backlink and trailer plumbing exactly as it is.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/app/ -run 'TestIntegrationWorkflowRepoChangeAttach' -count=1 -v`
Expected: FAIL — the extended success fixture now refuses with `placeholder-token` under the old whole-blob regex (the renamed refusal row passes either way; the failure proving the change is the acceptance direction).

- [ ] **Step 3: Replace check (11) in `change_attach.go`**

Delete the `placeholderTokenRE` var (and the `regexp` import if nothing else in the file uses it — grep the file before removing). Replace the check:

```go
	// (11, plan only) No whole-slot placeholder filler (change 0414): a section
	// or the pre-heading preamble whose ENTIRE authored body is one bare
	// planning token (tbd/todo/fixme/tktk/xxx/placeholder, any case) is an
	// unfilled slot and refuses. A plan that merely mentions such a token — an
	// instruction, a code example, ambiguous prose — is build-actionable and
	// attaches; completeness judgment stays with plan authoring and review.
	if kind == attachKindPlan {
		if slot, found := planPlaceholderSlot(blob.Blob.Bytes); found {
			return attachRefusal(opKey, ResultInvalidState, kind, ReasonAttachPlaceholderToken,
				fmt.Sprintf("%s of the plan contains only a placeholder token; fill the slot with real content", slot))
		}
	}
```

Update the two comments that state the old rule as fact: the file-top doc comment ("hold no unresolved planning placeholder token" → "carry no placeholder-only plan slot") and the `ReasonAttachPlaceholderToken` constant comment:

```go
	// ReasonAttachPlaceholderToken: a plan slot — a section body or the
	// pre-heading preamble — contains only a bare placeholder token (an
	// unfilled slot; change 0414). A token mentioned inside substantive
	// content never refuses.
	ReasonAttachPlaceholderToken = "placeholder-token"
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/app/ -run 'TestIntegrationWorkflowRepoChangeAttach|TestPlanPlaceholderSlot' -count=1 -v`
Expected: PASS. Then `go build ./...` and `go vet ./internal/app/` clean.

- [ ] **Step 5: Mutation-prove the guard (both directions), then restore**

Backups first: `cp internal/app/plan_content.go "$SCRATCH/plan_content.go.bak"` and same for `change_attach.go` (use the session scratchpad; never `git checkout --` an uncommitted file — that restores HEAD, not your edit).

1. Strip the guard: make `planPlaceholderSlot` return `"", false` unconditionally. Verify the mutation applied (`grep -n 'return "", false' internal/app/plan_content.go`). Run `go test ./internal/app/ -run 'TestPlanPlaceholderSlot|TestIntegrationWorkflowRepoChangeAttach' -count=1`. Expected: FAIL (filler rows + refusal integration row). Restore from backup.
2. Restore the old behavior: temporarily re-add the retired whole-word alternation regex (the six tokens, uppercase, pipe-separated, word-boundary-bounded — recover the exact line from `git log -p -- internal/app/change_attach.go` rather than retyping it) matched against `blob.Blob.Bytes` as the check body. Verify the mutation applied, then run the same tests. Expected: FAIL (the acceptance rows — the retry.go instruction and the agreed decision sentence). Restore from backup.
3. Re-run green: `go test ./internal/app/ -count=1`. Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/app/change_attach.go internal/app/workflow_integration_test.go
git commit -m "fix(app): plan attach refuses whole-slot filler only, not token mentions (change 0414)"
```

---

### Task 3: Template prompt derivation (`extractTemplatePrompts` + embedded cache)

**Files:**
- Create: `internal/app/results_prompts.go`
- Create: `internal/app/results_prompts_test.go`

**Interfaces:**
- Consumes: `internal/assets.Open`, `splitResultsLines`, `fenceMask`, `frontmatterEnd` (Task 1).
- Produces (Task 4 consumes all of these):
  - `extractTemplatePrompts(template []byte) ([]string, error)` — pure, injectable-input derivation.
  - `var resultsTemplatePrompts func() ([]string, error)` — cached production derivation from the embedded asset (a `func` var so tests can stub setup failure).
  - `maskInlineLiterals(s string) string`, `normalizeWS(s string) string`, `authorProse(lines []rcLine, fenced []bool, fmEnd int, inManaged func(int) bool) string`, and `const literalMask = "\x00"`.

- [ ] **Step 1: Write the failing tests** (`internal/app/results_prompts_test.go`)

```go
package app

import (
	"reflect"
	"strings"
	"testing"
)

// Change 0414: reserved authoring prompts are DERIVED from the results
// template's angle-bracket instruction spans — comments and code excluded,
// wrapped prompts collapsed, whitespace normalized, case and punctuation kept
// exact, first-appearance order, deduplicated.
func TestExtractTemplatePrompts(t *testing.T) {
	tmpl := "<!-- authoring notes: <not a prompt> -->\n" +
		"# <Change title> — Results\n\n" +
		"**Human action:** <Whether action is needed, in one or two\nsentences, wrapped across lines.>\n\n" +
		"## Steps\n\n" +
		"1. <Concrete step.>\n   Expected: <Observable result.>\n" +
		"2. <Concrete step.>\n" +
		"### Important — <short name of the action>\n\n" +
		"```\n<Fenced example, not a prompt>\n```\n\n" +
		"Literal `<inline code, not a prompt>` here.\n"
	got, err := extractTemplatePrompts([]byte(tmpl))
	if err != nil {
		t.Fatalf("extractTemplatePrompts: %v", err)
	}
	want := []string{
		"<Change title>",
		"<Whether action is needed, in one or two sentences, wrapped across lines.>",
		"<Concrete step.>",
		"<Observable result.>",
		"<short name of the action>",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("prompts = %#v\nwant %#v", got, want)
	}
}

func TestExtractTemplatePromptsCRLFAndReflowIdentity(t *testing.T) {
	lf := "# <Change title> — Results\n\n<A wrapped\nprompt.>\n"
	crlf := strings.ReplaceAll("# <Change title> — Results\n\n<A wrapped prompt.>\n", "\n", "\r\n")
	a, err := extractTemplatePrompts([]byte(lf))
	if err != nil {
		t.Fatalf("lf: %v", err)
	}
	b, err := extractTemplatePrompts([]byte(crlf))
	if err != nil {
		t.Fatalf("crlf: %v", err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("reflow/CRLF changed prompt identity: %#v vs %#v", a, b)
	}
}

// A missing, malformed, or empty prompt source is a setup FAILURE — never a
// silently-empty prompt set that would validate everything as filled.
func TestExtractTemplatePromptsFailsClosed(t *testing.T) {
	for name, tmpl := range map[string]string{
		"no prompts at all":      "# Title — Results\n\nProse only.\n",
		"only commented prompts": "<!-- <Hidden> -->\n# Title\n\nProse.\n",
		"unterminated span":      "# Title\n\n<Never closed\n",
	} {
		t.Run(name, func(t *testing.T) {
			if got, err := extractTemplatePrompts([]byte(tmpl)); err == nil {
				t.Fatalf("want error, got prompts %#v", got)
			}
		})
	}
}

// The production path: prompts derived from the EMBEDDED canonical template
// must succeed, be nonempty, and cover the known headline slots — including
// the lowercase "<short name of the action>" the retired uppercase heuristic
// could not see.
func TestResultsTemplatePromptsEmbedded(t *testing.T) {
	prompts, err := resultsTemplatePrompts()
	if err != nil {
		t.Fatalf("resultsTemplatePrompts (embedded): %v", err)
	}
	if len(prompts) == 0 {
		t.Fatal("embedded template yielded zero prompts")
	}
	wantContains := []string{"<Change title>", "<short name of the action>", "<Concrete step.>", "<Observable result.>"}
	for _, w := range wantContains {
		found := false
		for _, p := range prompts {
			if p == w {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("embedded prompts missing %q; got %#v", w, prompts)
		}
	}
}

func TestMaskInlineLiterals(t *testing.T) {
	cases := map[string]struct{ in, mustNotContain string }{
		"html comment masked":       {"a <!-- <P> --> b", "<P>"},
		"unclosed comment masks on": {"a <!-- <P> and <Q>", "<P>"},
		"inline code masked":        {"a `<P>` b", "<P>"},
		"double-backtick span":      {"a ``x `<P>` y`` b", "<P>"},
		"escaped bracket masked":    {`a \<P> b`, "<P"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := maskInlineLiterals(tc.in); strings.Contains(got, tc.mustNotContain) {
				t.Fatalf("maskInlineLiterals(%q) = %q still contains %q", tc.in, got, tc.mustNotContain)
			}
		})
	}
	// An UNCLOSED backtick run is literal text, not a mask-to-the-end.
	if got := maskInlineLiterals("a ` b <P>"); !strings.Contains(got, "<P>") {
		t.Fatalf("unclosed backtick swallowed following text: %q", got)
	}
	// Masking must not SPLICE fragments into a prompt: the sentinel is
	// non-whitespace, so text around a masked span stays non-contiguous.
	spliced := normalizeWS(maskInlineLiterals("<Change `x` title>"))
	if strings.Contains(spliced, "<Change title>") {
		t.Fatalf("mask spliced fragments into a prompt: %q", spliced)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/app/ -run 'TestExtractTemplatePrompts|TestResultsTemplatePromptsEmbedded|TestMaskInlineLiterals' -count=1 -v`
Expected: FAIL — undefined symbols (compile error).

- [ ] **Step 3: Implement `internal/app/results_prompts.go`**

```go
package app

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/danielhanold/docket/internal/assets"
)

// This file derives the reserved authoring prompts of the canonical results
// template (change 0414). Results validation used to GUESS scaffolding from
// capitalization — "< then an uppercase letter" — which false-positived on
// uppercase HTML tags (<BR>, <DETAILS>) and URI schemes (<MAILTO:…>,
// <HTTPS://…>) and missed lowercase prompts. The corrected detector matches
// only the prompts the shipped template actually emits: derived here from the
// embedded asset, never hand-enumerated (ADR-0050), never guessed.

// resultsTemplateAssetPath is the canonical results template in the embedded
// bundle — the same file skills/docket-implement-next/results-template.md the
// coordinator copies when authoring results.
const resultsTemplateAssetPath = "skills/docket-implement-next/results-template.md"

// literalMask replaces a literal-example span (code, comment, escape, fenced
// or managed line) in a prose view. It is deliberately non-whitespace and can
// never occur in a prompt, so a masked span can neither match a prompt nor
// splice its neighbors into one under whitespace normalization.
const literalMask = "\x00"

var wsRunRE = regexp.MustCompile(`[ \t\r\n\f\v]+`)

// normalizeWS collapses every whitespace run to one space and trims the ends,
// so reflow and CRLF-vs-LF never change a prompt's identity. Case and
// punctuation stay exact.
func normalizeWS(s string) string {
	return strings.TrimSpace(wsRunRE.ReplaceAllString(s, " "))
}

// authorProse renders the author-content line stream of a document: leading
// frontmatter, fenced-code lines, and generated managed-block lines become the
// mask sentinel; everything else passes through with newlines preserved.
func authorProse(lines []rcLine, fenced []bool, fmEnd int, inManaged func(int) bool) string {
	var b strings.Builder
	for i, ln := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		if i < fmEnd || fenced[i] || inManaged(ln.start) {
			b.WriteString(literalMask)
			continue
		}
		b.WriteString(ln.text)
	}
	return b.String()
}

// maskInlineLiterals replaces literal-example spans inside a prose stream with
// the mask sentinel: HTML comments (<!-- … -->; an unclosed opener masks to the
// end), inline code spans (a backtick run closed by an EQUAL-length run, per
// CommonMark; an unclosed run is literal text), and a backslash-escaped angle
// bracket (\< or \>).
func maskInlineLiterals(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		if strings.HasPrefix(s[i:], "<!--") {
			b.WriteString(literalMask)
			end := strings.Index(s[i+4:], "-->")
			if end < 0 {
				return b.String()
			}
			i += 4 + end + 3
			continue
		}
		if s[i] == '`' {
			run := 0
			for i+run < len(s) && s[i+run] == '`' {
				run++
			}
			if off := findEqualBacktickRun(s[i+run:], run); off >= 0 {
				b.WriteString(literalMask)
				i += run + off + run
				continue
			}
			b.WriteString(s[i : i+run])
			i += run
			continue
		}
		if s[i] == '\\' && i+1 < len(s) && (s[i+1] == '<' || s[i+1] == '>') {
			b.WriteString(literalMask)
			i += 2
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// findEqualBacktickRun returns the offset in s of the first backtick run of
// exactly length n (a longer or shorter run does not close the span), or -1.
func findEqualBacktickRun(s string, n int) int {
	for j := 0; j < len(s); {
		if s[j] != '`' {
			j++
			continue
		}
		run := 0
		for j+run < len(s) && s[j+run] == '`' {
			run++
		}
		if run == n {
			return j
		}
		j += run
	}
	return -1
}

// extractTemplatePrompts derives the reserved authoring prompts from a results
// template: every complete angle-bracket instruction span outside HTML
// comments and code, whitespace-normalized, deduplicated, in first-appearance
// order. The derivation reads the CANONICAL template, never a submitted
// artifact. A template with an unterminated span or no prompts at all is a
// setup error — the caller must fail closed, not validate against nothing.
func extractTemplatePrompts(template []byte) ([]string, error) {
	lines := splitResultsLines(template)
	fenced := fenceMask(lines)
	prose := maskInlineLiterals(authorProse(lines, fenced, frontmatterEnd(lines), func(int) bool { return false }))
	var prompts []string
	seen := map[string]bool{}
	for i := 0; i < len(prose); i++ {
		if prose[i] != '<' {
			continue
		}
		end := strings.IndexByte(prose[i:], '>')
		if end < 0 {
			return nil, fmt.Errorf("results template carries an unterminated angle-bracket prompt (opened at byte offset %d of the prose view)", i)
		}
		span := prose[i : i+end+1]
		i += end
		if strings.Contains(span, literalMask) {
			continue // a masked fragment inside is a literal example, not a prompt
		}
		p := normalizeWS(span)
		if len(p) <= 2 {
			continue // "<>" carries no instruction
		}
		if !seen[p] {
			seen[p] = true
			prompts = append(prompts, p)
		}
	}
	if len(prompts) == 0 {
		return nil, fmt.Errorf("results template carries no angle-bracket authoring prompts")
	}
	return prompts, nil
}

// resultsTemplatePrompts returns the prompts derived from the EMBEDDED
// canonical template. The embedded asset is immutable for a given binary, so
// the derivation runs once and is cached. It is a func var only so tests can
// stub the setup-failure path; production never reassigns it.
var resultsTemplatePrompts func() ([]string, error) = sync.OnceValues(func() ([]string, error) {
	body, err := assets.Open(resultsTemplateAssetPath)
	if err != nil {
		return nil, fmt.Errorf("embedded results template unavailable: %w", err)
	}
	return extractTemplatePrompts(body)
})
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/app/ -run 'TestExtractTemplatePrompts|TestResultsTemplatePromptsEmbedded|TestMaskInlineLiterals' -count=1 -v`
Expected: PASS. If `TestResultsTemplatePromptsEmbedded` fails on a missing expected prompt, read the actual `skills/docket-implement-next/results-template.md` — the expectation list must quote the shipped template verbatim, not this plan.

- [ ] **Step 5: Commit**

```bash
git add internal/app/results_prompts.go internal/app/results_prompts_test.go
git commit -m "feat(app): derive results authoring prompts from the embedded template (change 0414)"
```

---

### Task 4: Replace the results capitalization heuristic with prompt matching

**Files:**
- Modify: `internal/app/results_content.go` (replace the placeholder scan and the action-statement scaffold check; delete `isResultsPlaceholderLine`, `isResultsScaffoldBody`, `stripResultsLeadMarkers` and their doc comments; add `reasonResultsTemplateInvalid`)
- Modify: `internal/app/results_content_test.go` (premise updates + new acceptance/rejection matrix)
- Modify: `internal/app/workflow_integration_test.go` (the results-scaffold fixture comment; see Step 1)

**Interfaces:**
- Consumes: `resultsTemplatePrompts`, `extractTemplatePrompts` (stub target), `maskInlineLiterals`, `normalizeWS`, `authorProse`, `fenceMask`, `frontmatterEnd` from Tasks 1 and 3.
- Produces: `ValidateResultsContent(source []byte, phase ResultsPhase) []ResultsContentFinding` — signature unchanged; new possible reason `"results-template-invalid"`; `reasonResultsPlaceholder` now fires only on a derived-prompt occurrence. A helper both call sites share: `firstTemplatePrompt(prose string, prompts []string) (string, bool)`.

- [ ] **Step 1: Rewrite the placeholder tests (they must fail against current code)**

In `results_content_test.go`:

**Keep unchanged** (they must pass before and after): the Finding-1 prose cases (planning-token words in real results prose) and the Finding-2 lowercase-HTML/autolink cases in `TestResultsPlaceholderRedesign`.

**Premise-changed rejection cases:** the current "unfilled scaffold …" cases use invented spans (`<What was delivered and how the behavior changed.>`, `<Finding>`, `<Human action.>`) that are NOT emitted template prompts. What those blocks guard — "raw template scaffolding refuses with `results-placeholder`" — survives; their fixtures move to real emitted prompts (test-premise-deleted-not-regated). Replace them with:

```go
		// Rejection: ACTUAL emitted template prompts, in every slot shape the
		// template uses (change 0414 — derived prompts, not capitalization).
		{
			name:  "unfilled H1 title prompt refused checkpoint",
			src:   "# <Change title> — Results\n\n## Outcome\n\nReal outcome prose.\n",
			phase: ResultsPhaseCheckpoint,
			want:  []string{"results-placeholder"},
		},
		{
			name:  "unfilled inline heading prompt refused checkpoint (lowercase prompt)",
			src:   "# T — Results\n\n## Human actions and testing\n\n### Important — <short name of the action>\n\nReal body.\n",
			phase: ResultsPhaseCheckpoint,
			want:  []string{"results-placeholder"},
		},
		{
			name:  "unfilled inline Expected prompt refused checkpoint",
			src:   "# T — Results\n\n## Outcome\n\nReal.\n\n## Human actions and testing\n\n1. Run the tool.\n   Expected: <Observable result.>\n",
			phase: ResultsPhaseCheckpoint,
			want:  []string{"results-placeholder"},
		},
		{
			name:  "wrapped prompt reflowed across different line breaks still refused",
			src:   "# T — Results\n\n## Outcome\n\n<The original problem,\nthe delivered behavior, and any material departure from the agreed design — lead with observable effects. Explain unfamiliar Docket concepts when necessary; include method names, stored fields, or internal identifiers\nonly when they help the reader understand a consequence or take action.>\n",
			phase: ResultsPhaseCheckpoint,
			want:  []string{"results-placeholder"},
		},

		// Deliberate consequences (spec "Consequences are deliberate"):
		{
			name:  "uppercase HTML tags accepted checkpoint",
			src:   "# T — Results\n\n## Outcome\n\nAdded markup:\n\n<BR>\n<DETAILS><SUMMARY>More</SUMMARY>Body</DETAILS>\n<DIV CLASS=\"x\">attribute-bearing</DIV>\n<CUSTOM-TAG>custom</CUSTOM-TAG>\n",
			phase: ResultsPhaseCheckpoint,
			want:  nil,
		},
		{
			name:  "uppercase URI schemes accepted final",
			src:   "# T — Results\n\n**Human action:** <MAILTO:person@example.com> is the contact; see <HTTPS://example.com>.\n\n## Outcome\n\nReal outcome prose.\n",
			phase: ResultsPhaseFinal,
			want:  nil,
		},
		{
			name:  "custom placeholder-looking prose is not guessed",
			src:   "# T — Results\n\n## Outcome\n\n<What was delivered and how the behavior changed.>\n",
			phase: ResultsPhaseCheckpoint,
			want:  nil,
		},
		{
			name:  "prompt quoted in inline code is a literal example",
			src:   "# T — Results\n\n## Outcome\n\nThe validator now rejects `<Change title>` when left unfilled.\n",
			phase: ResultsPhaseCheckpoint,
			want:  nil,
		},
		{
			name:  "prompt inside a fenced example is a literal example",
			src:   "# T — Results\n\n## Outcome\n\nReal.\n\n```\n# <Change title> — Results\n```\n",
			phase: ResultsPhaseCheckpoint,
			want:  nil,
		},
		{
			name:  "escaped opening bracket is a literal example",
			src:   "# T — Results\n\n## Outcome\n\nAuthors must replace \\<Change title> with the real title.\n",
			phase: ResultsPhaseCheckpoint,
			want:  nil,
		},
		{
			name:  "unescaped emitted prompt amid substantive content still refused",
			src:   "# T — Results\n\n## Outcome\n\nWe shipped the fix and verified it end to end.\n<Concrete step.>\nMore real prose after.\n",
			phase: ResultsPhaseCheckpoint,
			want:  []string{"results-placeholder"},
		},
```

Two fixture-accuracy obligations before committing: (a) check the wrapped-Outcome prompt text against the shipped template — it must be verbatim (only line breaks may differ); (b) check whether the H1-prompt case ALSO reports `results-title-missing` (an unfilled `# <Change title>` line is still an H1 for the title check, so expected reasons stay `["results-placeholder"]`; if the run disagrees, read the actual findings and set the expectation to what the code truthfully reports, keeping `results-placeholder` present).

For the **action statement**: in `TestValidateResultsContentActionStatement`, the "statement is still a template prompt" case must use the real Human-action prompt and expect BOTH findings at final (the whole-document scan sees the prompt too; the specific empty-statement reason is preserved on top):

```go
		{
			name:  "statement still the template prompt refused final",
			src:   "# T — Results\n\n**Human action:** <Whether human action is needed, in one or two sentences, consistent\nwith the sections below. During implementation this may say the assessment is pending;\nfinal results give a settled assessment.>\n\n## Outcome\n\nReal outcome prose.\n",
			phase: ResultsPhaseFinal,
			want:  []string{"results-placeholder", "results-action-statement-empty"},
		},
		{
			name:  "statement beginning with legitimate uppercase markup accepted final",
			src:   "# T — Results\n\n**Human action:** <DETAILS> rendering must be checked by eye in the PR preview.\n\n## Outcome\n\nReal outcome prose.\n",
			phase: ResultsPhaseFinal,
			want:  nil,
		},
```

Read `sameReasons`/`reasonsOf` before editing: if `sameReasons` is order-sensitive, list reasons in emission order (placeholder is emitted in the both-phases block, before final-phase findings).

**Other existing tests:** `TestResultsPlaceholderScaffoldMessageIsScaffold` and `TestValidateResultsContentScaffoldFinalContainsPlaceholder` (and any table rows elsewhere using invented spans as scaffold fixtures) — same premise migration: keep each test's guarded property, re-point its fixture at a real emitted prompt, and update message-content expectations to the new message (it names the prompt: `unfilled template prompt "<Change title>"`). Grep the whole package for `isResultsScaffoldBody|isResultsPlaceholderLine|stripResultsLeadMarkers|scaffold` and visit every hit.

**Integration test:** in `workflow_integration_test.go`, `TestIntegrationWorkflowRepoChangeAttachResultsCheckpointContent`'s scaffold fixture keeps `# <Change title> — Results` (a real prompt — still refuses), but its Outcome body `<What was delivered and how the behavior changed.>` is an invented span: replace it with the template's real Outcome prompt text and update the comment ("angle-bracket placeholders" → "unfilled emitted template prompts").

Add a **setup-failure** test (stub the func var, restore via defer):

```go
// A missing/unusable prompt source is a validator SETUP failure: a stable
// results-template-invalid finding whose message blames the template, never a
// pass and never an accusation against the author's document.
func TestValidateResultsContentTemplateSetupFailure(t *testing.T) {
	orig := resultsTemplatePrompts
	defer func() { resultsTemplatePrompts = orig }()
	resultsTemplatePrompts = func() ([]string, error) {
		return nil, fmt.Errorf("boom: no template")
	}
	fs := ValidateResultsContent([]byte("# T — Results\n\n## Outcome\n\nReal prose.\n"), ResultsPhaseCheckpoint)
	if len(fs) != 1 || fs[0].Reason != "results-template-invalid" {
		t.Fatalf("findings = %#v, want exactly one results-template-invalid", fs)
	}
	if !strings.Contains(fs[0].Message, "validator setup failure") {
		t.Fatalf("message must name the failure as the validator's, got %q", fs[0].Message)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/app/ -run 'TestResultsPlaceholder|TestValidateResultsContent|TestIntegrationWorkflowRepoChangeAttachResults' -count=1 -v`
Expected: FAIL — uppercase-markup acceptance cases refuse under the old uppercase heuristic; inline/lowercase prompt cases pass when they must refuse; the setup-failure test does not compile yet.

- [ ] **Step 3: Implement in `results_content.go`**

Add the reason:

```go
	// reasonResultsTemplateInvalid: the canonical results template could not
	// supply authoring prompts (missing/malformed/empty embedded asset). A
	// validator SETUP failure — it blames the binary's template, never the
	// author's document, and it fails closed (change 0414).
	reasonResultsTemplateInvalid = "results-template-invalid"
```

Replace the "Both phases: no unfilled authoring placeholder" per-line loop with:

```go
	// Both phases (change 0414): no complete occurrence of a reserved authoring
	// prompt DERIVED from the shipped results template — outside code,
	// comments, frontmatter, and managed blocks. Markup and autolinks pass
	// regardless of capitalization (they are not emitted prompts); a literal
	// copy of a prompt passes only in code or behind an escaped bracket;
	// custom placeholder-looking prose is the author's and reviewer's call.
	prompts, perr := resultsTemplatePrompts()
	if perr != nil {
		findings = append(findings, ResultsContentFinding{
			Reason: reasonResultsTemplateInvalid,
			Message: fmt.Sprintf("the canonical results template could not supply authoring prompts "+
				"(validator setup failure — the artifact was not judged): %v", perr),
		})
	} else if p, ok := firstTemplatePrompt(authorProse(lines, fenced, frontmatterEnd(lines), inManaged), prompts); ok {
		findings = append(findings, ResultsContentFinding{
			Reason:  reasonResultsPlaceholder,
			Message: fmt.Sprintf("the results artifact still carries the unfilled template prompt %q", p),
		})
	}
```

Add the shared matcher (next to the deleted helpers):

```go
// firstTemplatePrompt reports the first reserved prompt (template order) whose
// complete, whitespace-normalized text occurs in the prose view. The prose is
// masked for inline literals here, so callers pass raw prose (whole-document
// author view, or an action-statement body).
func firstTemplatePrompt(prose string, prompts []string) (string, bool) {
	scan := normalizeWS(maskInlineLiterals(prose))
	for _, p := range prompts {
		if strings.Contains(scan, p) {
			return p, true
		}
	}
	return "", false
}
```

In the final-phase action-statement check, replace the scaffold-shape call on the joined statement body:

```go
		stillPrompt := false
		if perr == nil {
			_, stillPrompt = firstTemplatePrompt(joined, prompts)
		}
		if joined == "" || isResultsFillerBody(joined) || stillPrompt {
```

(`perr != nil` already produced the setup finding; the statement check then judges only emptiness/filler — never guesses.)

Delete `isResultsPlaceholderLine`, `isResultsScaffoldBody`, `stripResultsLeadMarkers` and their doc comments entirely (those comments are the "capitalization separates scaffolding from HTML" prose this change retires). Update the file-top comment's description of the placeholder check. Run `go build ./...` to catch orphaned references, and grep the package for the three deleted names — every remaining hit is a test to migrate (Step 1 should have caught them all).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/app/ -count=1`
Expected: PASS — including the untouched Finding-1/Finding-2 acceptance cases, `TestResultsTemplateFailsCheckpointValidation` (the raw shipped template must STILL fail: its emitted prompts are exactly what the derived set matches), and the attach integration tests.

- [ ] **Step 5: Mutation-prove both directions, then restore**

Backup `results_content.go` and `results_prompts.go` to the scratchpad first; verify each mutation landed with a grep before trusting a red/green reading; always `-count=1`.

1. Strip the guard: make `firstTemplatePrompt` return `"", false` unconditionally → `TestResultsTemplateFailsCheckpointValidation`, the rejection matrix, and the wrapped-prompt case must FAIL. Restore.
2. Restore the old heuristic: reimplement the scan as the retired per-line rule — lead-marker-stripped line begins `<` followed by an ASCII uppercase letter (`len(s) >= 2 && s[0] == '<' && s[1] >= 'A' && s[1] <= 'Z'`) → the uppercase HTML/URI acceptance cases and the lowercase-prompt rejection case must FAIL. Restore.
3. Mask removal probe: make `maskInlineLiterals` the identity function → the inline-code and escaped-bracket literal-example cases must FAIL. Restore.
4. Re-run `go test ./internal/app/ -count=1`: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/app/results_content.go internal/app/results_content_test.go internal/app/workflow_integration_test.go
git commit -m "fix(app): results placeholder detection matches derived template prompts, not capitalization (change 0414)"
```

---

### Task 5: Template↔validator contract — every slot, independently derived

**Files:**
- Modify: `internal/app/results_contract_test.go`

**Interfaces:**
- Consumes: `ValidateResultsContent`, `extractTemplatePrompts`, `resultsTemplatePrompts`, `reasonResultsPlaceholder`, `repoRootFromCaller`.
- Produces: test-only helpers `testScanTemplateSlots` and `testFillSlots` (local to the file).

Spec acceptance 6 verbatim: derive the slot population from the authored template **with coverage that detects an omitted slot; do not rely solely on the same production extractor as its own oracle**. So the contract test carries its own independent scanner, compares it against the production extractor in both directions (a mirror correspondence — set equality, not subset; correspondence-guard-runs-one-way), and exercises each slot alone in an otherwise-filled fixture.

- [ ] **Step 1: Write the failing test** (append to `results_contract_test.go`)

```go
// testTemplateSlot is one angle-bracket authoring span located by the
// test-local scanner: its byte range in the authored template and its
// whitespace-normalized text.
type testTemplateSlot struct {
	start, end int // [start, end) byte offsets in the template source
	text       string
}

// testScanTemplateSlots is the INDEPENDENT slot oracle (change 0414): a
// deliberately separate implementation from extractTemplatePrompts, so the
// production extractor is never its own oracle. It blanks HTML comments
// in place (offsets preserved), then records every <...> span. The template
// has no fenced code today; if one is ever added, this scanner and the
// production extractor will disagree and the set-equality assert below reddens
// — which is the request to extend both deliberately.
func testScanTemplateSlots(t *testing.T, src []byte) []testTemplateSlot {
	t.Helper()
	body := []byte(string(src))
	for {
		open := bytes.Index(body, []byte("<!--"))
		if open < 0 {
			break
		}
		close := bytes.Index(body[open:], []byte("-->"))
		if close < 0 {
			t.Fatalf("authored template has an unclosed HTML comment at byte %d", open)
		}
		for i := open; i < open+close+3; i++ {
			body[i] = ' '
		}
	}
	var slots []testTemplateSlot
	for i := 0; i < len(body); i++ {
		if body[i] != '<' {
			continue
		}
		j := bytes.IndexByte(body[i:], '>')
		if j < 0 {
			t.Fatalf("authored template has an unterminated < span at byte %d", i)
		}
		slots = append(slots, testTemplateSlot{
			start: i, end: i + j + 1,
			text: strings.Join(strings.Fields(string(body[i:i+j+1])), " "),
		})
		i += j
	}
	return slots
}

// testFillSlots returns the template with every slot EXCEPT keep replaced by
// substantive filler text (keep < 0 fills all). Splicing runs back-to-front so
// recorded offsets stay valid.
func testFillSlots(src []byte, slots []testTemplateSlot, keep int) []byte {
	out := []byte(string(src))
	for i := len(slots) - 1; i >= 0; i-- {
		if i == keep {
			continue
		}
		out = append(out[:slots[i].start], append([]byte("Real substantive content"), out[slots[i].end:]...)...)
	}
	return out
}

// TestResultsTemplateEverySlotFailsValidation exercises EVERY authoring slot
// of the shipped template independently: an otherwise fully-filled template
// with exactly one slot left raw must fail checkpoint validation with
// results-placeholder — including the lowercase, wrapped, inline-Expected,
// title, and Human-action slots the retired uppercase heuristic could not all
// see. The slot population comes from the test-local scanner, and the
// fully-filled control must PASS, so a scanner that misses a slot (leaving raw
// scaffolding behind in the control) reddens here rather than passing
// vacuously.
func TestResultsTemplateEverySlotFailsValidation(t *testing.T) {
	root := repoRootFromCaller(t)
	src, err := os.ReadFile(filepath.Join(root, "skills", "docket-implement-next", "results-template.md"))
	if err != nil {
		t.Fatalf("read shipped results template (fail closed): %v", err)
	}
	slots := testScanTemplateSlots(t, src)
	if len(slots) < 10 {
		t.Fatalf("population floor: found only %d slots; the template carries more — the scanner is broken", len(slots))
	}

	// Mirror correspondence with the production extractor, both directions
	// via unique-set equality (correspondence-guard-runs-one-way).
	prod, err := extractTemplatePrompts(src)
	if err != nil {
		t.Fatalf("extractTemplatePrompts(authored source): %v", err)
	}
	uniq := map[string]bool{}
	for _, s := range slots {
		uniq[s.text] = true
	}
	prodSet := map[string]bool{}
	for _, p := range prod {
		prodSet[p] = true
	}
	for p := range prodSet {
		if !uniq[p] {
			t.Errorf("production extractor emits %q; the independent scan never saw it", p)
		}
	}
	for s := range uniq {
		if !prodSet[s] {
			t.Errorf("independent scan found slot %q; the production extractor missed it", s)
		}
	}

	// The embedded derivation must equal the authored-source derivation: the
	// asset guards prove byte parity, this pins the derivation seam end to end.
	embedded, err := resultsTemplatePrompts()
	if err != nil {
		t.Fatalf("resultsTemplatePrompts (embedded): %v", err)
	}
	if !reflect.DeepEqual(embedded, prod) {
		t.Fatalf("embedded prompts != authored-source prompts:\n%#v\n%#v", embedded, prod)
	}

	// Control: with EVERY slot filled the template passes checkpoint — proves
	// the fill works, so each per-slot failure below is attributable to the
	// one retained slot, not to leftover scaffolding.
	filled := testFillSlots(src, slots, -1)
	if fs := ValidateResultsContent(filled, ResultsPhaseCheckpoint); len(fs) != 0 {
		t.Fatalf("fully-filled template must pass checkpoint validation; got %v", fs)
	}

	// Each slot alone must refuse.
	for i, s := range slots {
		fixture := testFillSlots(src, slots, i)
		fs := ValidateResultsContent(fixture, ResultsPhaseCheckpoint)
		found := false
		for _, f := range fs {
			if f.Reason == reasonResultsPlaceholder {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("slot %d %q alone did not fail checkpoint validation; findings %v", i, s.text, fs)
		}
	}
}
```

Add the imports the file now needs (`bytes`, `reflect`, `strings`). The existing `TestResultsTemplateFailsCheckpointValidation` stays exactly as it is — the raw template must still fail.

- [ ] **Step 2: Run test to verify it exercises reality**

Run: `go test ./internal/app/ -run 'TestResultsTemplate' -count=1 -v`
Expected: PASS (Tasks 3–4 are in place). Then prove it is not vacuous: temporarily mutate `testFillSlots` to also fill the kept slot (remove the `if i == keep { continue }` guard) — the per-slot loop must go RED on every slot ("did not fail"); restore. Also re-run mutation 1 from Task 4 Step 5 (matcher stubbed to never match) and confirm this test reddens too.

- [ ] **Step 3: Commit**

```bash
git add internal/app/results_contract_test.go
git commit -m "test(app): every results-template slot independently fails validation (change 0414)"
```

---

### Task 6: Prose sweep, whole-suite gate, and budget read

**Files:**
- Modify: whatever the greps below surface (expected: nothing beyond comments already updated in Tasks 2 and 4; possibly none)

**Interfaces:** none produced; this task closes the change's documentation and verification obligations.

- [ ] **Step 1: Sweep maintained prose that equates a token mention with an unfinished plan**

Derive the sites — never hand-list them (AGENTS.md):

```bash
grep -rni -E -e 'placeholder token' -e 'placeholderTokenRE' -e 'tbd\|todo' -e 'todo\|fixme' \
  --include='*.go' --include='*.md' internal/ skills/ docs/ README.md 2>/dev/null | grep -viF -- '_test.go'
```

(The token patterns are spelled lowercase with `-i` deliberately — see the self-hosting constraint in Global Constraints.) Sort hits into (a) maintained source/skills/docs — update any sentence stating the old whole-word rule as current fact; (b) point-in-time records (`docs/superpowers/plans/` other than this plan, `docs/results/`, archived changes, specs, Accepted ADRs) — leave verbatim, they were true when written. Expected state: `skills/docket-implement-next/SKILL.md`'s "no unresolved placeholder" phrasing is still accurate under the new semantics (an unfilled slot IS an unresolved placeholder) — leave it unless a sentence spells out the whole-word token rule. **If any file under `skills/` changes**, regenerate the embedded bundle through the repo's existing generation path (see `internal/assets/generate.go` and its generator command — run the documented generator, never hand-edit `internal/assets/embedded/`), and commit the regenerated tree with the prose change; the manifest/parity tests redden if this is skipped.

- [ ] **Step 2: Run the whole suite at the build gate**

Resolve the command from config — read `build.test_command` via docket's config resolution (check `.docket.yml` in this checkout; do not copy a command from memory) — and run it from this feature worktree. Expected: SUITE green.

- [ ] **Step 3: Read the budget report**

Even on green: act on any `SERIAL CONFIRMED OVER BUDGET:` line (authoritative breach); note `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` lines as screening findings (confirm serially per `tests/README.md` before treating as real). The new per-slot contract test runs the validator once per slot plus controls on a small document — cheap — but verify `internal/app`'s row did not move materially.

- [ ] **Step 4: Commit (only if the sweep changed anything)**

```bash
git add -u
git commit -m "docs: retire whole-word placeholder-rule prose (change 0414)"
```

---

## Self-Review Notes (spec coverage map)

- Spec §1 (plan whole-slot rule, message, prose boundary) → Tasks 1–2. Acceptance 1–4 → Task 1 matrix + Task 2 integration fixtures.
- Spec §2 (derived prompts, normalization, masking, action statement, setup failure, caching) → Tasks 3–4. Acceptance 5, 7, 8 → Task 4 matrix + `TestValidateResultsContentTemplateSetupFailure` + `TestResultsTemplatePromptsEmbedded`.
- Acceptance 6 (per-slot, independent oracle, omitted-slot detection) → Task 5.
- Acceptance 9 (existing guarantees keep proving themselves) → untouched tests must stay green in Tasks 2/4/6; premise-migrated tests keep their guarded property per test-premise-deleted-not-regated.
- Acceptance 10 (mutation proofs, applied-mutation verification, backup-restore) → Task 2 Step 5, Task 4 Step 5, Task 5 Step 2.
- Acceptance 11 (whole suite via resolved `build.test_command`, budget read) → Task 6.
- Known limits (structural check only; ambiguous prose accepted deliberately; prompts tied to the shipped template, no compatibility registry) → encoded in Task 1/4 comments and the "custom placeholder-looking prose is not guessed" acceptance case.
- Self-hosting: this plan file carries no uppercase whole-word planning token, so the currently installed attach path accepts it; fixtures build tokens via `tok(...)`.
