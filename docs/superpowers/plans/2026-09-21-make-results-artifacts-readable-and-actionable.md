<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0440 — Make results artifacts readable and actionable](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0440-make-results-artifacts-readable-and-actionable.md)**
<!-- docket:backlink:end -->
# Make Results Artifacts Readable and Actionable — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Adopt the approved human-readable results specification for future results artifacts: a required "Human action:" statement after the title, a new section reading order (Outcome → Human actions and testing → Verification performed → Known issues and follow-ups), matching authoring/convention guidance, and a new structural check in the shared Go validator.

**Architecture:** Three coordinated surfaces change together: (1) `internal/app/results_content.go` gains a final-phase action-statement check (shape-keyed, two new stable reasons) with tests; (2) the canonical template `skills/docket-implement-next/results-template.md` is rewritten to the new reading order; (3) authoring prose in `skills/docket-implement-next/SKILL.md` Step 6.5 and `skills/docket-convention/SKILL.md` is reconciled, along with the repoguard prose sentinels and budget ceilings that pin the old wording. The embedded asset tree is regenerated last (a suite drift gate enforces it).

**Tech Stack:** Go (stdlib only in the touched files), Markdown skill bodies, `cmd/genassets`, the `internal/suiterunner`-driven suite via `go run ./cmd/docket development test`.

**Spec:** `docs/superpowers/specs/2026-09-21-make-results-artifacts-readable-and-actionable-design.md` (on the `docket` metadata branch; synchronized copy at `.docket/docs/superpowers/specs/…` from the repo root).

## Global Constraints

- Future artifacts only: never rewrite anything under `docs/results/` — historical results are frozen point-in-time records.
- Out of scope: checkpoint ownership, exact-head evidence, merge policy, post-merge behavior, readability scoring, new review rounds/lifecycle states/configuration, automatic follow-up creation. Do not touch `change_attach.go`, evidence code, or gate code.
- The action statement and `## Outcome` are required at the final boundary; every other section stays conditional (omitted when empty, never padded with filler).
- New validator reasons are additive, lowercase-hyphen, stable machine strings: `results-action-statement-missing` and `results-action-statement-empty`. All existing checks (parse, title, placeholder, required substantive Outcome, empty-section, filler-section) are preserved unchanged.
- Key the new check on syntactic shape (label + colon form), never an enumerated list of statement spellings; mutation-test it (delete the check, watch the new tests redden, restore; defeat Go's test cache with `-count=1`).
- These exact sentinel phrases must survive the prose edits (pinned by `internal/repoguard/prose_contracts_test.go`): in `skills/docket-implement-next/SKILL.md` — "Step 6.5 — Results (required)", "required for every change, trivial included", "never commit or move HEAD beneath a live gate, a running worker, or a transferred drive"; in `skills/docket-convention/SKILL.md` — "required close-out artifacts (one per implemented change, trivial included; change 0410)" and "required close-out artifact for every implemented change, trivial included (change 0410)".
- Cross-references in maintained source anchor on symbol names or verbatim-quoted clauses, never line numbers (ADR-0054).
- Every commit lands on branch `refactor/make-results-artifacts-readable-and-actionable`; stage by explicit path, never `git add -A`.
- The build gate command is `go run ./cmd/docket development test` (run once at the end-of-build gate, owned by the build role; per-task steps run focused package tests).

---

### Task 1: Action-statement check in the shared results validator

**Files:**
- Modify: `internal/app/results_content.go`
- Test: `internal/app/results_content_test.go`

**Interfaces:**
- Consumes: existing `ValidateResultsContent(source []byte, phase ResultsPhase) []ResultsContentFinding`, `isResultsFillerBody`, the `lines`/`fenced`/`inManaged`/`headings`/`body` machinery already built inside `ValidateResultsContent`.
- Produces: two new reason constants `reasonResultsActionStatementMissing = "results-action-statement-missing"` and `reasonResultsActionStatementEmpty = "results-action-statement-empty"`; a new package-level helper `parseResultsActionStatement(text string) (body string, ok bool)`. Later tasks (template, prose) rely on the validated shape being: a line whose stripped form is `Human action` + colon (emphasis markers tolerated), located after the H1 title and before the first H2, with a substantive statement.

- [ ] **Step 1: Write the failing tests**

Append to `internal/app/results_content_test.go`. Also note (Step 2 confirms) that several EXISTING final-phase fixtures will start failing once the check exists; Step 3 fixes the code, Step 4 fixes the fixtures — do both before expecting green.

```go
// Change 0440: the final boundary requires a substantive "Human action:"
// statement between the H1 title and the first H2. Detection is shape-keyed
// (label + colon, emphasis tolerated), never an enumerated statement list.
func TestValidateResultsContentActionStatement(t *testing.T) {
	cases := []struct {
		name  string
		src   string
		phase ResultsPhase
		want  []string
	}{
		{
			name:  "bold statement accepted final",
			src:   "# T — Results\n\n**Human action:** No required action or additional functional testing identified.\n\n## Outcome\n\nReal outcome prose.\n",
			phase: ResultsPhaseFinal,
			want:  nil,
		},
		{
			name:  "plain unbolded statement accepted final (shape, not spelling)",
			src:   "# T — Results\n\nhuman action: Important verification remains; follow the scenario below.\n\n## Outcome\n\nReal outcome prose.\n",
			phase: ResultsPhaseFinal,
			want:  nil,
		},
		{
			name:  "colon-outside-bold form accepted final",
			src:   "# T — Results\n\n**Human action**: No required action.\n\n## Outcome\n\nReal outcome prose.\n",
			phase: ResultsPhaseFinal,
			want:  nil,
		},
		{
			name:  "wrapped statement paragraph accepted final",
			src:   "# T — Results\n\n**Human action:**\nImportant verification remains before relying on this feature.\n\n## Outcome\n\nReal outcome prose.\n",
			phase: ResultsPhaseFinal,
			want:  nil,
		},
		{
			name:  "missing statement refused final",
			src:   "# T — Results\n\n## Outcome\n\nReal outcome prose.\n",
			phase: ResultsPhaseFinal,
			want:  []string{"results-action-statement-missing"},
		},
		{
			name:  "empty statement refused final",
			src:   "# T — Results\n\n**Human action:**\n\n## Outcome\n\nReal outcome prose.\n",
			phase: ResultsPhaseFinal,
			want:  []string{"results-action-statement-empty"},
		},
		{
			name:  "filler statement refused final",
			src:   "# T — Results\n\n**Human action:** None.\n\n## Outcome\n\nReal outcome prose.\n",
			phase: ResultsPhaseFinal,
			want:  []string{"results-action-statement-empty"},
		},
		{
			name:  "statement below first H2 does not count",
			src:   "# T — Results\n\n## Outcome\n\n**Human action:** No required action.\n\nReal outcome prose.\n",
			phase: ResultsPhaseFinal,
			want:  []string{"results-action-statement-missing"},
		},
		{
			name:  "statement only inside fence does not count",
			src:   "# T — Results\n\n```\n**Human action:** No required action.\n```\n\n## Outcome\n\nReal outcome prose.\n",
			phase: ResultsPhaseFinal,
			want:  []string{"results-action-statement-missing"},
		},
		{
			name:  "missing statement accepted checkpoint",
			src:   "# T — Results\n\n## Outcome\n\nBuild landed; review pending.\n",
			phase: ResultsPhaseCheckpoint,
			want:  nil,
		},
		{
			name:  "pending assessment prose accepted final (settledness is coordinator judgment)",
			src:   "# T — Results\n\n**Human action:** Assessment pending until review completes.\n\n## Outcome\n\nReal outcome prose.\n",
			phase: ResultsPhaseFinal,
			want:  nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := reasonsOf(ValidateResultsContent([]byte(tc.src), tc.phase))
			if !sameReasons(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the new tests to verify they fail**

Run: `go test ./internal/app/ -run TestValidateResultsContentActionStatement -count=1 -v`
Expected: FAIL — the missing/empty/filler cases get no findings because the check does not exist yet (accepted-final cases already pass).

- [ ] **Step 3: Implement the check**

In `internal/app/results_content.go`:

3a. Add the two reasons to the existing const block (after `reasonResultsFillerSection`):

```go
	reasonResultsActionStatementMissing = "results-action-statement-missing"
	reasonResultsActionStatementEmpty   = "results-action-statement-empty"
```

3b. Add the shape parser near `parseResultsHeading`:

```go
// parseResultsActionStatement recognizes the required action-statement line
// (change 0440): optional emphasis markers, the label "Human action" in any
// case, optional emphasis around the colon, then the statement text. It
// returns the same-line text after the colon (emphasis and whitespace
// trimmed) and whether the line is an action-statement line at all. Detection
// is keyed on this label-plus-colon SHAPE — never on an enumerated list of
// statement spellings — so `**Human action:** …`, `**Human action**: …`, and
// a plain `Human action: …` all match.
func parseResultsActionStatement(text string) (string, bool) {
	s := strings.TrimLeft(text, " \t")
	s = strings.TrimLeft(s, "*_")
	s = strings.TrimLeft(s, " \t")
	const label = "human action"
	if len(s) < len(label) || !strings.EqualFold(s[:len(label)], label) {
		return "", false
	}
	s = strings.TrimLeft(s[len(label):], " \t*_")
	if s == "" || s[0] != ':' {
		return "", false
	}
	body := strings.TrimSpace(s[1:])
	body = strings.TrimSpace(strings.Trim(body, "*_"))
	return body, true
}
```

3c. In `ValidateResultsContent`, inside the final-phase section (after the `if phase != ResultsPhaseFinal { return findings }` early return; place it before the `outcomeIdx` scan), add:

```go
	// Final phase (change 0440): a substantive "Human action:" statement must
	// sit between the H1 title and the first H2. The statement's body is its
	// same-line remainder plus its immediate continuation lines (the rest of
	// that paragraph), so a wrapped statement is not misread as empty.
	stmtFrom := 0
	for _, h := range headings {
		if h.level == 1 {
			stmtFrom = h.line + 1
			break
		}
	}
	stmtTo := len(lines)
	for _, h := range headings {
		if h.level == 2 && h.line >= stmtFrom {
			stmtTo = h.line
			break
		}
	}
	stmtFound := false
	for i := stmtFrom; i < stmtTo; i++ {
		if fenced[i] || inManaged(lines[i].start) || isHeading[i] {
			continue
		}
		stmtBody, ok := parseResultsActionStatement(strings.TrimRight(lines[i].text, "\r"))
		if !ok {
			continue
		}
		stmtFound = true
		parts := []string{stmtBody}
		for j := i + 1; j < stmtTo && body[j] && !isHeading[j] && !fenced[j]; j++ {
			parts = append(parts, strings.TrimSpace(strings.TrimRight(lines[j].text, "\r")))
		}
		joined := strings.TrimSpace(strings.Join(parts, "\n"))
		if joined == "" || isResultsFillerBody(joined) {
			findings = append(findings, ResultsContentFinding{
				Reason:  reasonResultsActionStatementEmpty,
				Message: "the Human action statement after the title has no substantive text",
			})
		}
		break
	}
	if !stmtFound {
		findings = append(findings, ResultsContentFinding{
			Reason:  reasonResultsActionStatementMissing,
			Message: "the final results artifact has no \"Human action:\" statement between the title and the first section",
		})
	}
```

3d. Update the two stale comment sites in the same file so the documented final contract names the statement (verbatim replacements):

- In the file-top comment, replace "while the final content contract — a required, substantive ## Outcome and no empty/filler sections — binds only at the implemented boundary." with "while the final content contract — a required substantive Human action statement, a required, substantive ## Outcome, and no empty/filler sections — binds only at the implemented boundary."
- In the `ResultsPhaseFinal` const comment, replace "adds the full content contract: a required, substantive `## Outcome` and no empty or filler sections." with "adds the full content contract: a required substantive Human action statement, a required, substantive `## Outcome`, and no empty or filler sections."
- In the `isResultsPlaceholderLine` doc comment, replace "especially in ## Findings and limitations and ## Follow-ups" with "especially in ## Known issues and follow-ups" (the merged section name Task 2 introduces).

- [ ] **Step 4: Update existing final-phase fixtures**

Run: `go test ./internal/app/ -count=1` and fix every newly failing case in `internal/app/results_content_test.go` by inserting the line `**Human action:** No required action.\n\n` immediately after the `# T — Results\n\n` (or `# Change title — Results\n\n`) title of each FINAL-phase fixture that is not specifically about the statement. Known sites (verify against actual failures — do not trust this list blindly):
- `TestValidateResultsContentOutcomeOnlyFinal` (its `src` gains the statement line after the H1).
- `TestValidateResultsContentFillerSectionRefusedFinal`.
- In `TestValidateResultsContentTable`: every case with `phase: ResultsPhaseFinal` ("valid outcome-only final", "missing outcome refused final", "empty outcome body refused final", "empty subsection under filled parent refused final", "filled sub under bodyless parent accepted final", "None in prose is legal final", and the remaining final cases in the table).
- In `TestResultsPlaceholderRedesign` and `TestValidateResultsContentScaffoldFinalContainsPlaceholder`: for final-phase cases whose `want` enumerates exact reasons, either add the statement line or extend `want` with `results-action-statement-missing` — prefer adding the statement line so each case keeps testing exactly one thing; leave checkpoint-phase fixtures untouched.

Do NOT weaken any existing expectation; the only legal edit is adding the statement line to a fixture or (where a case deliberately lacks a title/H2 structure that makes insertion unnatural) adding the new reason to `want` with a one-line comment saying why.

- [ ] **Step 5: Run the package tests to verify they pass**

Run: `go test ./internal/app/ -count=1`
Expected: PASS.

- [ ] **Step 6: Mutation-test the new check**

Temporarily delete the whole statement block added in Step 3c (keep a copy; `git diff` is your backup — do NOT `git checkout --` the file, that restores to HEAD and destroys Steps 3–4). Run `go test ./internal/app/ -run TestValidateResultsContentActionStatement -count=1` — Expected: FAIL (missing/empty/filler cases redden). Then separately mutate only the empty/filler branch (replace the `joined == "" || isResultsFillerBody(joined)` condition with `false`) and confirm the empty and filler cases redden. Restore the exact Step 3c code and re-run to green with `-count=1`.

- [ ] **Step 7: Commit**

```bash
git add internal/app/results_content.go internal/app/results_content_test.go
git commit -m "feat(app): final results require a substantive Human action statement (change 0440)"
```

---

### Task 2: Rewrite the canonical results template + its repoguard pins

**Files:**
- Modify: `skills/docket-implement-next/results-template.md` (full rewrite)
- Modify: `internal/repoguard/prose_contracts_test.go` (the `change_0410_results_template` sentinel)
- Modify: `internal/repoguard/budgets_test.go` (the `docket-implement-next/results-template.md` ceiling row + note comment)

**Interfaces:**
- Consumes: the validator shape from Task 1 (statement line after the H1, before the first H2).
- Produces: the canonical section roster later tasks' prose must name verbatim: `**Human action:**` statement, `## Outcome`, `## Human actions and testing` (H3 items labeled `### Important — …` / `### Optional — …`), `## Verification performed`, `## Known issues and follow-ups`.

- [ ] **Step 1: Update the repoguard sentinel first (the failing test)**

In `internal/repoguard/prose_contracts_test.go`, replace the `change_0410_results_template` entry (keep the 0410 entry comment, extend it) with:

```go
	// change 0410 introduced the canonical required template; change 0440
	// re-shaped it around the reader: a required Human action statement after
	// the title, and Human testing / Findings and limitations / Follow-ups
	// merged into Human actions and testing + Known issues and follow-ups.
	// Absent phrases are the REMOVED old section headings and the retired
	// optional-template triggers (assert-detects-removal).
	{sentinel: "change_0440_results_template", file: "skills/docket-implement-next/results-template.md",
		present: []string{
			"**Human action:**",
			"## Outcome",
			"## Human actions and testing",
			"## Verification performed",
			"## Known issues and follow-ups",
		},
		absent: []string{
			"## Human testing",
			"## Findings and limitations",
			"## Follow-ups",
			"OPTIONAL: write one only",
			"## Verify (human)",
		}},
```

(Note `## Human testing` is not a substring of `## Human actions and testing`, and `## Follow-ups` is not a substring of `## Known issues and follow-ups` — the absent pins are real, not self-defeating.)

- [ ] **Step 2: Run repoguard to verify it fails**

Run: `go test ./internal/repoguard/ -run TestProseContracts -count=1`
Expected: FAIL — the template still carries the old headings. (If the test function has a different name, find it with `grep -n "func Test" internal/repoguard/prose_contracts_test.go` and use that name.)

- [ ] **Step 3: Rewrite the template**

Replace the entire content of `skills/docket-implement-next/results-template.md` with:

```markdown
<!-- results-template.md — REQUIRED close-out artifact for every implemented change (trivial
     included; change 0410). Authored and consolidated by the coordinator in the FEATURE worktree,
     committed on <type>/<slug> at each checkpoint and finally before the implemented transition.
     Written for a mid-level engineer with little knowledge of Docket internals (change 0440):
     lead with what the reader must do and what changed; implementation detail belongs in the
     linked PR, plan, and evidence. Angle-bracket instructions are authoring guidance only —
     remove them from actual artifacts. The Human action statement and Outcome are required at
     finalization; omit any other section, including its subsections, when there is no
     substantive content. The generated docket:backlink block above the title is owned by the
     artifact.backlink operation — never hand-author its markers, and do not add empty header
     fields for unavailable links. Content rules and the checkpoint lifecycle are normative in
     docket-implement-next's Step 6.5. -->
# <Change title> — Results

**Human action:** <Whether human action is needed, in one or two sentences, consistent
with the sections below. During implementation this may say the assessment is pending;
final results give a settled assessment.>

## Outcome

<The original problem, the delivered behavior, and any material departure from the
agreed design — lead with observable effects. Explain unfamiliar Docket concepts when
necessary; include method names, stored fields, or internal identifiers only when they
help the reader understand a consequence or take action.>

## Human actions and testing

### Important — <short name of the action>

<Why this matters, when the action matters, and what remains uncertain if skipped.>

<Prerequisites, setup, and starting state.>

1. <Concrete step.>
   Expected: <Observable result.>
2. <Concrete step.>
   Expected: <Observable result.>

<Cleanup, only when the procedure changes persistent state.>

### Optional — <short name of the walkthrough>

<Why a reader might want this. An optional walkthrough may exercise behavior automated
tests already cover; a routine suite rerun is not a functional walkthrough.>

<Setup, numbered steps with observable expected results, and cleanup — complete enough
to run without reconstructing missing commands or knowing Docket internals.>

## Verification performed

<Concise account of checks actually performed, their outcomes, and links to durable,
accessible evidence. Identify skipped, failed, or incomplete verification explicitly.
No test logs, per-test inventories, or chronological build diary — and never imply the
human checks proposed above were already performed.>

## Known issues and follow-ups

### <Problem or follow-up>

<When it occurs and what the person experiences; its practical impact; whether it is
confirmed or suspected; any available workaround; and the suggested next action, linking
an existing change when available. Keep each entry understandable without following its
technical links. A fixed finding belongs here only when it explains a remaining risk or
a consequential design decision.>
```

- [ ] **Step 4: Re-run repoguard prose contracts**

Run: `go test ./internal/repoguard/ -run TestProseContracts -count=1`
Expected: PASS for the template sentinel. (SKILL/convention sentinels are untouched until Task 3.)

- [ ] **Step 5: Re-pin the template's budget ceiling**

Run: `go test ./internal/repoguard/ -count=1`. The budgets test will report the template's actual new line/word counts. In `internal/repoguard/budgets_test.go`, update the row `{"docket-implement-next/results-template.md", 51, 257}` to the exact new counts and change its trailing comment to `// 0440: reader-first template — action statement + merged Known issues (see note above)`. Then extend the 0410 note comment block (the one beginning "Change 0410 re-baselined the required-results workflow surfaces") with a short paragraph after it:

```go
// Change 0440 re-baselined the same surfaces once more for the reader-first
// results shape (required Human action statement; Human actions and testing;
// Known issues and follow-ups): results-template.md and the Step 6.5 /
// convention prose that names the sections. Authored contract documentation,
// not slack — ceilings stay pinned at the exact new counts.
```

- [ ] **Step 6: Verify repoguard is green**

Run: `go test ./internal/repoguard/ -count=1`
Expected: PASS (a genassets-drift failure, if this suite includes it, is expected until Task 4 — only proceed if the remaining failures are the embedded-tree drift and nothing else; otherwise fix here).

- [ ] **Step 7: Commit**

```bash
git add skills/docket-implement-next/results-template.md internal/repoguard/prose_contracts_test.go internal/repoguard/budgets_test.go
git commit -m "feat(skills): reader-first results template with required Human action statement (change 0440)"
```

---

### Task 3: Reconcile authoring and convention prose

**Files:**
- Modify: `skills/docket-implement-next/SKILL.md` (Step 6.5)
- Modify: `skills/docket-convention/SKILL.md` ("Results artifact shape and lifecycle" paragraph)
- Modify: `internal/repoguard/prose_contracts_test.go` (add a 0440 prose sentinel)
- Modify: `internal/repoguard/budgets_test.go` (re-pin SKILL/convention ceilings if counts changed)

**Interfaces:**
- Consumes: the Task 2 section roster (exact heading spellings) and the Task 1 validator behavior (missing/empty statement refuses at the implemented boundary).
- Produces: normative Step 6.5 prose the coordinator follows when authoring results; the convention paragraph other skills cite.

- [ ] **Step 1: Add the failing prose sentinel**

Append to the contracts table in `internal/repoguard/prose_contracts_test.go` (after the `change_0410_build_results` entry):

```go
	// change 0440 — Step 6.5 and the convention now describe the reader-first
	// results shape. Present phrases bind the action-statement requirement and
	// the optional-walkthrough allowance; absent phrases are the retired
	// old-section wording and the retired blanket prohibition on manually
	// checking automated behavior (assert-detects-removal).
	{sentinel: "change_0440_results_prose", file: "skills/docket-implement-next/SKILL.md",
		present: []string{
			"**Human action:**",
			"**Human actions and testing**",
			"Known issues and follow-ups",
			"An Optional walkthrough MAY exercise behavior automated tests already cover",
		},
		absent: []string{
			"only functional scenarios the automated tests do **not** cover",
			"under Findings and limitations or Verification performed",
		}},
	{sentinel: "change_0440_convention_results", file: "skills/docket-convention/SKILL.md",
		present: []string{
			"`**Human action:**` statement",
			"`## Human actions and testing`",
			"`## Known issues and follow-ups`",
		},
		absent: []string{
			"`## Human testing`",
			"`## Findings and limitations`",
			"`## Follow-ups`",
		}},
```

- [ ] **Step 2: Run repoguard to verify it fails**

Run: `go test ./internal/repoguard/ -run TestProseContracts -count=1`
Expected: FAIL on both new sentinels.

- [ ] **Step 3: Edit Step 6.5 in `skills/docket-implement-next/SKILL.md`**

Three verbatim replacements (all inside "### Step 6.5 — Results (required)"; the pinned sentinel phrases in Global Constraints must survive):

3a. In the first paragraph, replace the sentence beginning "`## Outcome` is required; every other section is **conditional**" with:

> Immediately after the H1 title, a **`**Human action:**` statement** says whether human action is needed — during implementation it may say the assessment is pending, but final results give a settled assessment consistent with the rest of the artifact, and the implemented boundary refuses a missing or empty statement. `## Outcome` is required; every other section is **conditional** — omit an empty heading and its subsections rather than filling it, and never write a `None`/`N/A`/`not applicable` body (whole-section filler refuses at the implemented boundary).

3b. Replace the entire "**Human testing** carries only functional scenarios…" paragraph with:

> **Human actions and testing** labels every item **Important** or **Optional**, written for a mid-level engineer with little knowledge of Docket internals. An **Important** item marks a meaningful verification gap or necessary human judgment (never a new automated merge requirement) — give the reason, when the action matters, and what remains uncertain if skipped. An Optional walkthrough MAY exercise behavior automated tests already cover — a routine suite rerun is not a functional walkthrough. Every item carries prerequisites and setup, concrete steps with observable expected results, and cleanup when it changes persistent state — complete enough to run without reconstructing missing commands. Do not infer coverage from a green suite: inspect the relevant tests and evidence, and where coverage is genuinely uncertain report that uncertainty under Known issues and follow-ups or Verification performed rather than authoring a speculative checklist.

3c. Replace the "**Follow-ups** hold out-of-scope work…" paragraph with:

> **Known issues and follow-ups** merges unresolved findings, limitations, and follow-up work into one plain-language section — each entry gives when the problem occurs and what the person experiences, its practical impact, whether it is confirmed or suspected, any workaround, and the suggested next action. Follow-ups recorded there hold out-of-scope work for **human triage** — link an existing change when one is known, and **never** mint a change, issue, ADR, or learning automatically. An in-scope defect stays fix/block/halt work regardless of where it is written. Final consolidation verifies that no material coordinator handoff remains only in chat, and the final report links the durable results file.

- [ ] **Step 4: Edit the convention paragraph in `skills/docket-convention/SKILL.md`**

In the "**Results artifact shape and lifecycle.**" paragraph, replace the sentence "Its template carries a required `## Outcome` section and the conditional sections `## Human testing`, `## Verification performed`, `## Findings and limitations`, and `## Follow-ups` — omit any conditional section, subsections and all, when it would hold no substantive content, and never pad one with `None`/`N/A` filler." with:

> Its template opens with a required `**Human action:**` statement immediately after the title (a settled assessment at finalization; change 0440) and carries a required `## Outcome` section plus the conditional sections `## Human actions and testing` (items labeled Important or Optional; optional walkthroughs may cover automated behavior), `## Verification performed`, and `## Known issues and follow-ups` — omit any conditional section, subsections and all, when it would hold no substantive content, and never pad one with `None`/`N/A` filler.

Leave the rest of the paragraph (file location, `results:` field rule, checkpoint sentence, follow-up triage sentence) byte-identical — the `change_0410_convention_results` sentinel pins its phrases.

- [ ] **Step 5: Sweep for stragglers**

Run from the worktree root:

```bash
out=$(grep -rn "## Human testing\|Findings and limitations\|## Follow-ups" --include="*.md" skills/ README.md 2>/dev/null | grep -v "internal/assets")
printf '%s\n' "$out"
```

Expected: no hits in maintained skill bodies (historical records under `docs/` are exempt and untouched). Fix any maintained-source hit the grep surfaces (excluding the embedded tree, regenerated in Task 4).

- [ ] **Step 6: Re-pin budget ceilings and run repoguard**

Run: `go test ./internal/repoguard/ -count=1`. If the budgets test reddens on `docket-implement-next/SKILL.md` (current row ceilings 210/7547) or `docket-convention/SKILL.md` (current row ceilings 400/7969), update those rows to the exact new counts reported, appending `; 0440: reader-first results prose` to each row's comment. The Task 2 note comment already covers the rationale.
Expected after pinning: PASS (except, possibly, the genassets drift gate — resolved in Task 4).

- [ ] **Step 7: Commit**

```bash
git add skills/docket-implement-next/SKILL.md skills/docket-convention/SKILL.md internal/repoguard/prose_contracts_test.go internal/repoguard/budgets_test.go
git commit -m "docs(skills): reconcile results authoring and convention prose to reader-first shape (change 0440)"
```

---

### Task 4: Regenerate the embedded asset tree and prove the branch green

**Files:**
- Modify (generated): `internal/assets/embedded/tree/skills/docket-implement-next/results-template.md`, `internal/assets/embedded/tree/skills/docket-implement-next/SKILL.md`, `internal/assets/embedded/tree/skills/docket-convention/SKILL.md` (plus whatever else genassets rewrites)

**Interfaces:**
- Consumes: Tasks 1–3 committed source.
- Produces: an embedded tree byte-identical to source, satisfying the suite's genassets `-check` drift gate.

- [ ] **Step 1: Regenerate**

Run from the worktree root: `go run ./cmd/genassets -repo .`
Expected: exit 0; `git status --porcelain` shows only paths under `internal/assets/embedded/`.

- [ ] **Step 2: Verify the drift gate is satisfied**

Run: `go run ./cmd/genassets -repo . -check` (if `-check` is not the flag's spelling, read the flag names with `go run ./cmd/genassets -h` and use the check/verify mode it documents).
Expected: clean (no drift reported).

- [ ] **Step 3: Run the focused packages once more**

Run: `go test ./internal/app/ ./internal/repoguard/ -count=1`
Expected: PASS — all sentinels, budgets, drift, and validator tests green together.

- [ ] **Step 4: Commit the regenerated tree**

```bash
git add internal/assets/embedded/
git commit -m "chore(assets): regenerate embedded tree for change 0440 results shape"
```

- [ ] **Step 5: Full suite at the build gate**

Run: `go run ./cmd/docket development test`
Expected: SUITE green. Read the budget report even on green — act on any `SERIAL CONFIRMED OVER BUDGET:` line; note `BUDGET WATCH:`/`PARALLEL-SENSITIVE:` lines in the build record. Any red is fixed on this branch before the task completes (never by weakening a test).

---

## Self-Review (performed)

- **Spec coverage:** action statement required + validated (Task 1); reading order and section content rules in the template (Task 2); authoring/convention guidance including the Important/Optional labels, optional-walkthrough allowance, and the merged known-issues section (Task 3); generated copies (Task 4); acceptance criterion "structural validation rejects a missing or empty action statement" is Task 1's core tests. Checkpoint-vs-final distinction preserved: the new check binds only at `ResultsPhaseFinal`.
- **Out-of-scope check:** no task touches `docs/results/`, evidence, gates, lifecycle, or config; no readability scoring; nothing auto-creates follow-ups.
- **Type consistency:** reason strings, helper name `parseResultsActionStatement`, and heading spellings are identical across all four tasks.
