<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0498 — Results file puts the whole-branch review under Human actions and testing](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-04-0498-results-file-puts-the-whole-branch-review-under-human-action.md)**
<!-- docket:backlink:end -->
# Results file: name the home for whole-branch review outcomes — Implementation Plan

> **For agentic workers:** executed by the resolved build skill `docket-build` (task-by-task under
> the `docket-build-task` contract). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give whole-branch review outcomes a named home in the results file: one summary line
under Verification performed, an entry under Known issues for every unfixed or reported finding,
and Human actions and testing kept to human work only. The full table stays in the PR body.

**Architecture:** This is a prose-only change to three distributed skill files: the results
template, implement-next Step 6.5, and `references/fix-loop.md`. A section-bound,
whitespace-collapsed prose guard in `internal/repoguard` pins it. To support the retired-sentence
check, the existing `docSectionContract` / `scanDocSection` gains an `absent` list instead of a
new mechanism. The embedded asset tree is regenerated with `go generate`. The size-budget
ceilings are re-pinned at the exact new counts. No Go behavior changes.

**Tech Stack:** Markdown skill bodies; Go tests (`go test`); `cmd/genassets` asset generator.

**Spec:** `docs/superpowers/specs/2026-10-04-results-file-puts-the-whole-branch-review-under-human-action-design.md`
(on the `docket` metadata branch).

## Global Constraints

- All prose; **no Go behavior changes** (test code and budget ceilings only).
- The PR body remains the disposition table's durable home. Do not change the PR-body disposition table.
- Do not add, remove, or reorder results sections. Do not add a validator, health check, or merge-boundary refusal on section membership. The change is guidance only and adds no blocking gate.
- Never edit 0494's or any other merged results file.
- No ADR.
- Regenerate `internal/assets/embedded/` only through `go generate ./internal/assets` (which runs `cmd/genassets`), never by hand-copying.
- Skill bodies ship into other repos. Write no sentence that is only true in this repo ("on this repo…").
- Text added to `results-template.md` stays **inside** the existing `## Verification performed` angle-bracket span. It must contain **no backtick, no `<`, and no `>`**. A backtick makes the validator's prompt extractor skip the whole span as a literal example (`extractTemplatePrompts`, "a masked fragment inside is a literal example, not a prompt"), and a `>` would end the prompt early.
- Every mutation probe uses `go test -count=1` (the test cache can serve a stale pass). It restores from a `cp` backup, never `git checkout --`, and confirms the mutation landed before reading the result.
- Stage explicit paths only. Never `git add -A`.

## Review Focus

1. **A template edit that silently weakens placeholder detection.** If the new Verification guidance gets a backtick or `>`, the span stops being a detected prompt, and a results file that pastes the raw guidance would attach. Task 1 Step 6 runs `TestResultsTemplate*` in `internal/app` and checks that the span is still extracted.
2. **A vacuous absent check over wrapped prose.** The retired fix-loop sentence spans a line break, so a raw `strings.Contains` absent check would pass even before the edit. The new absent branch matches whitespace-collapsed, and Task 1 proves it reddens before the edit and again under a re-insertion mutation.
3. **Embedded-tree drift.** The suite's drift gate fails if `internal/assets/embedded/` is stale. Each task regenerates and runs `go run ./cmd/genassets -check -repo .`.
4. **Size budgets.** All three files sit at or near their `skillBudgets` ceilings (`internal/repoguard/budgets_test.go`). Each task re-pins the touched rows at the exact measured counts and runs `TestSkillSizeBudgets`.
5. **Words that drift from the claim.** A present-phrase guard can survive a rewrite that keeps the words but moves them. Each phrase is bound to its section (section/terminator pair) and carries its own subject, and each one is mutation-tested.

---

### Task 1: Guard machinery + results template + fix-loop wording

**Files:**
- Modify: `internal/repoguard/prose_contracts_test.go` (`docSectionContract`, `scanDocSection`, new table + test after `TestRebaseRecoveryDocContracts`)
- Modify: `skills/docket-implement-next/results-template.md` (`## Verification performed` guidance span)
- Modify: `skills/docket-implement-next/references/fix-loop.md` (*Results-checkpoint linkage (change 0410)* paragraph, in `## Recording — the PR-body disposition table`)
- Modify: `internal/repoguard/budgets_test.go` (rows `docket-implement-next/references/fix-loop.md`, `docket-implement-next/results-template.md`)
- Regenerate: `internal/assets/embedded/` (manifest + tree copies of the two skill files)

**Interfaces:**
- Produces: `docSectionContract.absent []string`. Each listed clause must not appear anywhere in the file, matched whitespace-collapsed. `scanDocSection` reports one violation per present absent-clause. Also produces the table `resultsReviewPlacementDocContracts []docSectionContract` and `func TestResultsReviewPlacementDocContracts(t *testing.T)`, which Task 2 appends rows to and whose population floor Task 2 raises.

- [ ] **Step 1: Extend `docSectionContract` with `absent`, and check it in `scanDocSection`**

In `internal/repoguard/prose_contracts_test.go`, add the field to the struct, after `present`:

```go
	absent     []string // retired clauses that must appear NOWHERE in the file, matched whitespace-collapsed
```

In `scanDocSection`, after the existing `for _, p := range c.present { … }` loop and before
`return v`, add:

```go
	whole := collapseWS(content)
	for _, a := range c.absent {
		if strings.Contains(whole, collapseWS(a)) {
			v = append(v, fmt.Sprintf("%s: retired clause is present: %q", c.file, a))
		}
	}
```

Absent is file-wide on purpose. A retired sentence moved to another section must still redden.
Existing rows leave `absent` nil, so their behavior is unchanged.

- [ ] **Step 2: Add the change-0498 table and test with rows for the template and fix-loop**

Append after `TestRebaseRecoveryDocContracts` in the same file:

```go
// change 0498 — whole-branch review outcomes get a named home in the results
// file. The PR body keeps the full disposition table; the final results carry a
// one-line review summary under Verification performed and Known issues entries
// for unfixed/reported findings; Human actions and testing holds only human
// work. Each clause is bound to its section (prose-guard-binds-phrase-to-claim)
// and matched whitespace-collapsed (phrase-grep-over-wrapped-prose). The absent
// clause is the retired fix-loop sentence that gave review findings no home; it
// WRAPS in the source, so only the collapsed match can see it
// (assert-detects-removal-not-replacement). Mutation-tested at introduction.
var resultsReviewPlacementDocContracts = []docSectionContract{
	{change: "change_0498_template_review_summary", file: "skills/docket-implement-next/results-template.md",
		section: "## Verification performed", terminator: "## Known issues and follow-ups",
		present: []string{
			"Give the whole-branch review one line: which review ran (the tier, or the custom review skill) and how its findings ended",
			"full table in the PR body",
			"A fixed finding with no remaining risk appears in the results only through this line",
		}},
	{change: "change_0498_fix_loop_condensation", file: "skills/docket-implement-next/references/fix-loop.md",
		section: "## Recording — the PR-body disposition table", terminator: "## Beyond-the-branch findings are reported",
		present: []string{
			"the **PR body remains the disposition table's durable home**",
			"During the build the results file may hold the full returned findings, their evidence, and their impact, so they survive a halt before the PR exists",
			"final consolidation condenses them to a one-line review summary",
		},
		absent: []string{
			"The results file preserves the findings, their evidence, and their impact for the human",
		}},
}

// TestResultsReviewPlacementDocContracts binds the change 0498 review-placement
// clauses to their sections; scanDocSection's missing-section / -terminator /
// -clause branches are exercised by TestUninstallCollectionDocContracts'
// non_vacuity subtest, and the absent branch by this test's own.
func TestResultsReviewPlacementDocContracts(t *testing.T) {
	root := guardRoot(t)

	// Population floor: a collapse means rows were lost or the table was gutted.
	checks := 0
	for _, c := range resultsReviewPlacementDocContracts {
		checks += len(c.present) + len(c.absent)
	}
	if checks < 7 {
		t.Fatalf("population floor: only %d review-placement doc clauses (expected >= 7)", checks)
	}

	var violations []string
	cache := map[string]string{}
	for _, c := range resultsReviewPlacementDocContracts {
		content, ok := cache[c.file]
		if !ok {
			b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(c.file)))
			if err != nil {
				t.Fatalf("read contract file %s (%s): %v (fail closed)", c.file, c.change, err)
			}
			content = string(b)
			cache[c.file] = content
		}
		for _, msg := range scanDocSection(content, c) {
			violations = append(violations, fmt.Sprintf("[%s] %s", c.change, msg))
		}
	}
	if len(violations) != 0 {
		t.Errorf("review-placement doc-contract violations (%d):\n%s", len(violations), strings.Join(violations, "\n"))
	}

	t.Run("non_vacuity", func(t *testing.T) {
		const doc = "intro\n## Alpha\nbody with a\nwrapped clause here\n## Beta\ntail"
		sec := docSectionContract{file: "x.md", section: "## Alpha", terminator: "## Beta"}
		// A retired clause that WRAPS in the source is still detected.
		c := sec
		c.absent = []string{"with a wrapped clause"}
		if got := scanDocSection(doc, c); len(got) != 1 {
			t.Errorf("scanDocSection missed a wrapped absent clause: %v", got)
		}
		// A retired clause OUTSIDE the section is still detected (file-wide).
		c = sec
		c.absent = []string{"tail"}
		if got := scanDocSection(doc, c); len(got) != 1 {
			t.Errorf("scanDocSection missed an absent clause outside the section: %v", got)
		}
		// A clause that is genuinely gone produces no violation.
		c = sec
		c.absent = []string{"never written"}
		if got := scanDocSection(doc, c); len(got) != 0 {
			t.Errorf("scanDocSection flagged an absent clause that is not there: %v", got)
		}
	})
}
```

- [ ] **Step 3: Run the new test to verify it fails for the right reasons**

Run: `cd /Users/homer/dev/docket/.worktrees/results-file-puts-the-whole-branch-review-under-human-action && go test -count=1 ./internal/repoguard -run 'TestResultsReviewPlacementDocContracts|TestUninstallCollectionDocContracts|TestRebaseRecoveryDocContracts|TestInstallPrerequisiteDocContracts' -v`
Expected: `TestResultsReviewPlacementDocContracts` FAILS. It reports the five new present clauses
missing, plus `retired clause is present: "The results file preserves the findings, …"` (the
proof that the collapsed absent check sees the wrapped sentence), and the
`the **PR body remains the disposition table's durable home**` clause is NOT reported. The
`non_vacuity` subtest and the three pre-existing doc-contract tests PASS.

- [ ] **Step 4: Edit the results template's Verification performed guidance**

In `skills/docket-implement-next/results-template.md`, replace:

```
<Concise account of checks actually performed, their outcomes, and links to durable,
accessible evidence. Identify skipped, failed, or incomplete verification explicitly.
No test logs, per-test inventories, or chronological build diary — and never imply the
human checks proposed above were already performed.>
```

with (no backticks, no `<`/`>` inside the span):

```
<Concise account of checks actually performed, their outcomes, and links to durable,
accessible evidence. Identify skipped, failed, or incomplete verification explicitly.
No test logs, per-test inventories, or chronological build diary — and never imply the
human checks proposed above were already performed. Give the whole-branch review one
line: which review ran (the tier, or the custom review skill) and how its findings
ended — e.g. Whole-branch review (deep tier): 5 findings, all fixed in-branch; full
table in the PR body. Name no PR number: the results are final before the PR opens. A
fixed finding with no remaining risk appears in the results only through this line.>
```

- [ ] **Step 5: Edit the fix-loop Results-checkpoint linkage paragraph**

In `skills/docket-implement-next/references/fix-loop.md`, replace:

```
disposition table's durable home** — the block `docket-finalize-change` reads. The results file
preserves the findings, their evidence, and their impact for the human; it never carries the machine
build-evidence block.
```

with:

```
disposition table's durable home** — the block `docket-finalize-change` reads. During the build the
results file may hold the full returned findings, their evidence, and their impact, so they survive
a halt before the PR exists; final consolidation condenses them to a one-line review summary plus
Known issues entries for the findings not fixed (Step 6.5's *Review findings*). It never carries
the machine build-evidence block.
```

- [ ] **Step 6: Regenerate the embedded tree and run the focused tests**

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/results-file-puts-the-whole-branch-review-under-human-action
go generate ./internal/assets
go run ./cmd/genassets -check -repo .
go test -count=1 ./internal/repoguard -run 'TestResultsReviewPlacementDocContracts|TestProseContracts|TestUninstallCollectionDocContracts' -v
go test -count=1 ./internal/app -run 'TestResultsTemplate' -v
go test -count=1 ./internal/assets
```
Expected: genassets `-check` exits 0. `TestResultsReviewPlacementDocContracts` PASSES (all
subtests), `TestProseContracts` PASSES (the 0440 template row still sees `## Verification
performed`), the `internal/app` `TestResultsTemplate*` tests PASS (the extended Verification span is
still an extracted prompt, and every slot still fails validation), and `internal/assets` PASSES.

- [ ] **Step 7: Re-pin the two size budgets at the exact new counts**

Measure:
```bash
cd /Users/homer/dev/docket/.worktrees/results-file-puts-the-whole-branch-review-under-human-action
for f in skills/docket-implement-next/references/fix-loop.md skills/docket-implement-next/results-template.md; do printf '%s %s lines %s words\n' "$f" "$(wc -l <"$f" | tr -d ' ')" "$(wc -w <"$f" | tr -d ' ')"; done
```
In `internal/repoguard/budgets_test.go`, set each row's `maxLines`/`maxWords` to **exactly** the
measured count (never with headroom). Prepend a `0498:` note to each row's trailing comment in
the house form, for example
`// 0498: +review-findings condensation at final consolidation (190/1958 -> <L>/<W>); 0410: …`
for fix-loop and
`// 0498: +whole-branch review summary line in Verification performed (64/446 -> <L>/<W>); 0440: …`
for the template, where `<L>/<W>` are the measured numbers. Then run:
`go test -count=1 ./internal/repoguard -run TestSkillSizeBudgets -v`
Expected: PASS.

- [ ] **Step 8: Mutation-test every clause (strip → red, restore → green)**

Run this with `bash` (not the interactive shell). It proves each mutation landed before it
reads the result, and it restores from a backup:

```bash
bash <<'EOF'
W=/Users/homer/dev/docket/.worktrees/results-file-puts-the-whole-branch-review-under-human-action
cd "$W" || exit 1
probe() { # $1 = repo-relative file, $2 = whitespace-tolerant perl regex to destroy
  f="$W/$1"; cp "$f" "$f.bak"
  before=$(tr -s '[:space:]' ' ' <"$f" | cksum)
  perl -0pi -e "s/$2/MUTATED/" "$f"
  after=$(tr -s '[:space:]' ' ' <"$f" | cksum)
  if [ "$before" = "$after" ]; then echo "MUTATION DID NOT LAND: $2"
  elif go test -count=1 ./internal/repoguard -run 'TestResultsReviewPlacementDocContracts$' >/dev/null 2>&1; then echo "STILL GREEN (defect): $2"
  else echo "red as expected: $2"; fi
  mv -f "$f.bak" "$f"
}
T=skills/docket-implement-next/results-template.md
L=skills/docket-implement-next/references/fix-loop.md
probe "$T" 'Give\s+the\s+whole-branch\s+review\s+one\s+line'
probe "$T" 'full\s+table\s+in\s+the\s+PR\s+body'
probe "$T" 'appears\s+in\s+the\s+results\s+only\s+through\s+this\s+line'
probe "$L" 'PR\s+body\s+remains\s+the\s+disposition'
probe "$L" 'so\s+they\s+survive\s+a\s+halt\s+before\s+the\s+PR\s+exists'
probe "$L" 'condenses\s+them\s+to\s+a\s+one-line\s+review\s+summary'
# Section binding: move a template clause OUT of its section (after the terminator).
probe "$T" '## Known issues and follow-ups'
# Absent clause: re-insert the retired sentence elsewhere in fix-loop.md.
cp "$W/$L" "$W/$L.bak"
printf '\nThe results file\npreserves the findings, their evidence, and their impact for the human.\n' >>"$W/$L"
if go test -count=1 ./internal/repoguard -run 'TestResultsReviewPlacementDocContracts$' >/dev/null 2>&1; then echo "STILL GREEN (defect): retired sentence"; else echo "red as expected: retired sentence"; fi
mv -f "$W/$L.bak" "$W/$L"
go test -count=1 ./internal/repoguard -run 'TestResultsReviewPlacementDocContracts$' >/dev/null 2>&1 && echo "restored: green" || echo "RESTORE FAILED"
git status --porcelain
EOF
```
Expected: every line reads `red as expected: …`, then `restored: green`. `git status` lists only
this task's intended paths, with no `.bak` files. Any `MUTATION DID NOT LAND` or `STILL GREEN`
line is a defect to fix before committing. (The terminator mutation removes the section's closing
heading, which reddens through the missing-terminator branch.)

- [ ] **Step 9: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/results-file-puts-the-whole-branch-review-under-human-action
git add internal/repoguard/prose_contracts_test.go internal/repoguard/budgets_test.go \
  skills/docket-implement-next/results-template.md skills/docket-implement-next/references/fix-loop.md \
  internal/assets/embedded
git status --porcelain
git commit -m "docs(0498): results template and fix-loop name the review summary home"
```
Expected: `git status --porcelain` after the add shows only staged (`M `) entries for these paths.

---

### Task 2: Step 6.5 — review homes, condensation exception, Human-actions class rule

**Files:**
- Modify: `skills/docket-implement-next/SKILL.md` (`### Step 6.5 — Results (required)`: the **Human actions and testing** paragraph, per-checkpoint mechanics step 2, a new **Review findings** paragraph before **Ownership.**)
- Modify: `internal/repoguard/prose_contracts_test.go` (append a row to `resultsReviewPlacementDocContracts`; raise its population floor)
- Modify: `internal/repoguard/budgets_test.go` (row `docket-implement-next/SKILL.md`)
- Regenerate: `internal/assets/embedded/`

**Interfaces:**
- Consumes: `docSectionContract` with `absent`, `resultsReviewPlacementDocContracts`, and `TestResultsReviewPlacementDocContracts` from Task 1.
- Produces: the `*Review findings*` paragraph name that Task 1's fix-loop sentence points to ("Step 6.5's *Review findings*").

- [ ] **Step 1: Append the Step 6.5 row and raise the population floor**

Append to `resultsReviewPlacementDocContracts` in `internal/repoguard/prose_contracts_test.go`:

```go
	{change: "change_0498_step65_review_homes", file: "skills/docket-implement-next/SKILL.md",
		section: "### Step 6.5 — Results (required)", terminator: "### Step 7 — PR + stop",
		present: []string{
			"The section holds only what a human should do or check — never a record of what the run already checked, which belongs under Verification performed",
			"a checkpoint updates it, never truncates it — except final consolidation, which condenses the review findings",
			"**Review findings.** The PR body is the full review disposition table's home",
			"Checkpoint (ii) MAY persist the full returned findings so they survive a halt before the PR exists; final consolidation condenses them",
			"`## Verification performed` then carries one line naming which review ran",
			"an entry for every finding not fixed (`deferred`, `reverted`, or `recorded`) and every `reported` beyond-the-branch finding",
			"A fixed finding with no remaining risk appears in final results only through that summary line",
		}},
```

In `TestResultsReviewPlacementDocContracts`, change the floor from `checks < 7` / `expected >= 7`
to `checks < 14` / `expected >= 14`.

- [ ] **Step 2: Run to verify it fails**

Run: `cd /Users/homer/dev/docket/.worktrees/results-file-puts-the-whole-branch-review-under-human-action && go test -count=1 ./internal/repoguard -run 'TestResultsReviewPlacementDocContracts' -v`
Expected: FAIL, listing exactly the seven `change_0498_step65_review_homes` clauses as missing. The
Task 1 rows stay satisfied.

- [ ] **Step 3: Add the Human-actions class rule**

In `skills/docket-implement-next/SKILL.md`, Step 6.5, the paragraph that begins
`**Human actions and testing** labels every item` ends with
`…rather than authoring a speculative checklist.` Append to that same line (same paragraph,
one space before):

```
The section holds only what a human should do or check — never a record of what the run already checked, which belongs under Verification performed.
```

- [ ] **Step 4: State the final-consolidation exception to "never truncates"**

Replace the per-checkpoint mechanics line:

```
2. Read and **preserve** the prior results content; a checkpoint updates it, never truncates it.
```

with:

```
2. Read and **preserve** the prior results content; a checkpoint updates it, never truncates it — except final consolidation, which condenses the review findings (*Review findings* below).
```

- [ ] **Step 5: Add the Review findings paragraph**

Insert a new paragraph, with a blank line on each side, immediately **before** the paragraph that
begins `**Ownership.** The coordinator authors and consolidates the artifact.` Keep it on one line,
like its neighbors:

```
**Review findings.** The PR body is the full review disposition table's home (`references/fix-loop.md`). Checkpoint (ii) MAY persist the full returned findings so they survive a halt before the PR exists; final consolidation condenses them. `## Verification performed` then carries one line naming which review ran (the tier, or the custom review skill) and how its findings ended, with no PR number — the PR does not exist yet. `## Known issues and follow-ups` carries an entry for every finding not fixed (`deferred`, `reverted`, or `recorded`) and every `reported` beyond-the-branch finding, plus any fixed finding that still explains a remaining risk or a consequential design decision. A fixed finding with no remaining risk appears in final results only through that summary line.
```

- [ ] **Step 6: Regenerate the embedded tree and run the focused tests**

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/results-file-puts-the-whole-branch-review-under-human-action
go generate ./internal/assets
go run ./cmd/genassets -check -repo .
go test -count=1 ./internal/repoguard -run 'TestResultsReviewPlacementDocContracts|TestProseContracts|TestGateCapture' -v
go test -count=1 ./internal/assets
```
Expected: `-check` exits 0. `TestResultsReviewPlacementDocContracts` PASSES. `TestProseContracts`
PASSES: the `change_0440_results_prose` absent clause `under Findings and limitations or
Verification performed` must stay absent, and the new class-rule sentence does not contain it.
`internal/assets` PASSES. (If no test matches `TestGateCapture`, `go test` reports "no tests to
run" for that pattern, which is fine. The package-level run in Step 8 covers it.)

- [ ] **Step 7: Re-pin the SKILL.md size budget at the exact new counts**

Measure:
`f=skills/docket-implement-next/SKILL.md; printf '%s lines %s words\n' "$(wc -l <"$f" | tr -d ' ')" "$(wc -w <"$f" | tr -d ' ')"`
(from the worktree root). In `internal/repoguard/budgets_test.go`, set the
`docket-implement-next/SKILL.md` row to exactly those counts. Prepend
`0498: +Step 6.5 review-findings homes, condensation exception, Human-actions class rule (214/8161 -> <L>/<W>); `
to its trailing comment, where `<L>/<W>` are the measured numbers. Run
`go test -count=1 ./internal/repoguard -run TestSkillSizeBudgets -v`.
Expected: PASS.

- [ ] **Step 8: Mutation-test the Step 6.5 clauses, then run the whole repoguard package**

```bash
bash <<'EOF'
W=/Users/homer/dev/docket/.worktrees/results-file-puts-the-whole-branch-review-under-human-action
cd "$W" || exit 1
probe() {
  f="$W/$1"; cp "$f" "$f.bak"
  before=$(tr -s '[:space:]' ' ' <"$f" | cksum)
  perl -0pi -e "s/$2/MUTATED/" "$f"
  after=$(tr -s '[:space:]' ' ' <"$f" | cksum)
  if [ "$before" = "$after" ]; then echo "MUTATION DID NOT LAND: $2"
  elif go test -count=1 ./internal/repoguard -run 'TestResultsReviewPlacementDocContracts$' >/dev/null 2>&1; then echo "STILL GREEN (defect): $2"
  else echo "red as expected: $2"; fi
  mv -f "$f.bak" "$f"
}
S=skills/docket-implement-next/SKILL.md
probe "$S" 'never\s+a\s+record\s+of\s+what\s+the\s+run\s+already\s+checked'
probe "$S" 'except\s+final\s+consolidation,\s+which\s+condenses'
probe "$S" '\*\*Review\s+findings\.\*\*'
probe "$S" 'MAY\s+persist\s+the\s+full\s+returned\s+findings'
probe "$S" 'then\s+carries\s+one\s+line\s+naming\s+which\s+review\s+ran'
probe "$S" 'every\s+finding\s+not\s+fixed'
probe "$S" 'appears\s+in\s+final\s+results\s+only\s+through\s+that\s+summary\s+line'
# Section binding: the class rule moved out of Step 6.5 (into Step 7) must redden.
cp "$W/$S" "$W/$S.bak"
perl -0pi -e 's/ The section holds only what a human should do or check — never a record of what the run already checked, which belongs under Verification performed\.//; s/(### Step 7 — PR \+ stop\n)/$1The section holds only what a human should do or check — never a record of what the run already checked, which belongs under Verification performed.\n/' "$W/$S"
if cmp -s "$W/$S" "$W/$S.bak"; then echo "MUTATION DID NOT LAND: relocation"
elif go test -count=1 ./internal/repoguard -run 'TestResultsReviewPlacementDocContracts$' >/dev/null 2>&1; then echo "STILL GREEN (defect): relocation"
else echo "red as expected: relocation"; fi
mv -f "$W/$S.bak" "$W/$S"
go test -count=1 ./internal/repoguard >/dev/null 2>&1 && echo "restored: repoguard green" || echo "REPOGUARD RED"
git status --porcelain
EOF
```
Expected: every probe prints `red as expected: …`, then `restored: repoguard green`. `git status`
lists only this task's intended paths, with no `.bak` files. (The relocation perl uses the UTF-8
em dash literally. If it prints `MUTATION DID NOT LAND`, re-run that probe with `perl -CSD -Mutf8`,
then judge the result.)

- [ ] **Step 9: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/results-file-puts-the-whole-branch-review-under-human-action
git add skills/docket-implement-next/SKILL.md internal/repoguard/prose_contracts_test.go \
  internal/repoguard/budgets_test.go internal/assets/embedded
git status --porcelain
git commit -m "docs(0498): Step 6.5 names the review homes and keeps Human actions to human work"
```

---

## Build gate

The whole suite runs at the end-of-build gate through the configured `build.test_command`
(driven by `docket-build`), never only the tests named above. Read the budget report even on a
green run.

## Self-review against the spec

- Decision 1 (Verification summary line, Known issues entries, fixed-without-risk only through the summary line): template (Task 1 Step 4), Step 6.5 *Review findings* (Task 2 Step 5).
- Decision 2 (build-time insurance kept; final consolidation condenses; explicit exception to "never truncates"): fix-loop (Task 1 Step 5), Step 6.5 mechanics step 2 and *Review findings* (Task 2 Steps 4–5).
- Decision 3 (Human actions class rule, no enumeration): Task 2 Step 3.
- Edits: the template, Step 6.5, and fix-loop are covered. The embedded tree is regenerated via `go generate` in both tasks.
- Testing: section-bound present rows for all three files, with the retired sentence pinned absent. Every clause is mutation-tested, and the whole suite runs at the build gate.
- Out of scope respected: no validator or gate, no merged results edits, no section reorder, no PR-table change, no ADR.
