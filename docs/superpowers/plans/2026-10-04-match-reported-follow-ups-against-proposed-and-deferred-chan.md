<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0510 — Match reported follow-ups against proposed and deferred changes](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-04-0510-match-reported-follow-ups-against-proposed-and-deferred-chan.md)**
<!-- docket:backlink:end -->
# Match Reported Follow-ups Against Proposed and Deferred Changes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: execution is by `docket-build` (each task runs under the `docket-build-task` contract). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** At final results consolidation, `docket-implement-next` checks every reported out-of-scope follow-up against the backlog's `proposed` and `deferred` changes and records one verdict plus a state-matched next action per follow-up, in both the results file and the final report.

**Architecture:** Prose-only change to the docket skill bodies. The authored source is the root `skills/` tree; `internal/assets/embedded/` (manifest + `tree/`) is **generated from it** by `cmd/genassets` (`go generate ./internal/assets/`) and drift-guarded by `TestEmbeddedMatchesAuthored` (`internal/assets/embedded_test.go`). New wording is pinned by a new whitespace-collapsed doc-contract test in `internal/repoguard/prose_contracts_test.go`, reusing the existing `docSectionContract` / `scanDocSection` machinery. Three of the four edited files sit exactly at their `skillBudgets` ceilings (`internal/repoguard/budgets_test.go`), so those rows are re-baselined in-diff.

**Tech Stack:** Markdown skill bodies; Go 1.27 tests (`go test`); `cmd/genassets`.

**Spec:** `docs/superpowers/specs/2026-10-04-match-reported-follow-ups-against-proposed-and-deferred-chan-design.md` (on the `docket` metadata branch; read it from the metadata working tree).

## Global Constraints

- **Edit the root `skills/` files only. Never hand-edit `internal/assets/embedded/**`** — regenerate it with `go generate ./internal/assets/` (run from the worktree root) in the same commit as the prose edit. The spec's sentence "The root `skills/` tree is a derived copy of the embedded tree" is backwards; `cmd/genassets` reads `skills/` (`DefaultAllowedRoots`) and writes the embedded bundle (verified at plan time against `internal/assets/generate.go` and commit `bb2bc43ff`).
- No new operation, request field, gate, or halt. The backlog match only recommends; it never writes to any change record.
- Candidates are `status` `proposed` or `deferred` only, excluding the change being built. Never `in-progress`, `blocked`, `implemented`, `stacked-merged`, or archived.
- Every candidate's `## Why` and `## What changes` are read — never a title or keyword comparison (#302's failure).
- Verdict vocabulary, verbatim: **Fits #N**, **Related to #N**, **No existing change fits (checked K)**.
- Failure posture: a failed `status` read or unreadable candidate writes entries without verdicts plus one line under `## Verification performed`; never halt, loop-retry, or block the implemented transition.
- Keep "never mint" in every edited sentence that had it.
- Skill bodies ship into consuming repos: no sentence may be true only in this repository (learning `distributed-body-has-no-local-repo`).
- Cross-references anchor on a heading/clause name (e.g. "Step 6.5 *Backlog match*"), never a line number (AGENTS.md, ADR-0054).
- Size budget rows: when a file's count rises past its ceiling, set the new ceiling to the **exact new count** (measured with `go test`'s own counters — `wc -l` lines and `strings.Fields` words; `wc -w` matches `strings.Fields` for these files) and prepend a `0510: ... (old -> new);` note to that row's comment, matching the existing ratchet style.
- Mutation probes restore from a **backup copy** (`cp`), never `git checkout --` (learning `mutation-restore-needs-a-backup-copy`), and run tests with `-count=1` (learning `cached-runner-serves-a-mutated-tree`).

## Review Focus

Each line is pinned by a `present` clause in Task 1's contract rows; a reviewer should confirm the prose actually says it, not just that the test is green.

1. **Title-only matching** — a run that only reads candidates whose titles look similar misses the general-form parent (#302). Expect the prose to require reading *every* candidate's `## Why` / `## What changes`. Pinned by the "For **every** candidate, not only those with similar titles" clause.
2. **Backlog read fails** (`status --json` errors, a record path is unreadable) — the run must still finish with a complete results file and name the skipped check under Verification performed, never halt. Pinned by the "never halt, retry in a loop, or block the implemented transition on it" clause.
3. **Matching against a claimed or built change** — folding scope into an `in-progress` / `implemented` change confuses the run or finalize that owns it. Pinned by the "keep the changes whose `status` is `proposed` or `deferred`, excluding this change" clause.
4. **A verdict on a non-follow-up entry** (an in-scope limitation, a verification-coverage note) — would read as a capture recommendation for something that is not new work. Pinned by the "an in-scope limitation or a verification-coverage note gets no verdict" clause.
5. **Chat summary and results file disagree** — the final report lists follow-ups without verdicts while the results file has them. Pinned by the Step 6.5 "The final report's follow-up list carries the same verdict per item" clause and the final-report enumeration clause.

---

## File Structure

| File | Responsibility | Task |
|---|---|---|
| `skills/docket-implement-next/SKILL.md` | Step 3 note, Step 6 report rule, Step 6.5 *Known issues* sentence + new *Backlog match* paragraph, final-report enumeration | 1 |
| `skills/docket-implement-next/references/fix-loop.md` | *Beyond-the-branch findings are reported* carries the verdict pointer | 2 |
| `skills/docket-implement-next/results-template.md` | `## Known issues and follow-ups` placeholder names the verdict and next action | 2 |
| `skills/docket-convention/SKILL.md` | *Results artifact shape and lifecycle* follow-up sentence | 2 |
| `internal/repoguard/prose_contracts_test.go` | New `TestFollowUpBacklogMatchDocContracts` + its two contract tables | 1 (create), 2 (extend) |
| `internal/repoguard/budgets_test.go` | Re-baseline `skillBudgets` rows that exceed their ceilings | 1, 2 |
| `internal/assets/embedded/manifest.json`, `internal/assets/embedded/tree/skills/...` | Generated bundle — regenerated, never hand-edited | 1, 2 |

Each task leaves the tree green (learning `intermediate-task-state-buildable`): every commit carries its prose, its guard rows, its budget re-baseline, and its regenerated bundle together.

---

### Task 1: implement-next SKILL.md — the Backlog match contract and its guard

**Files:**
- Modify: `skills/docket-implement-next/SKILL.md` (Step 3's "Adjacent follow-up work" paragraph; Step 6's "**Triage the returned findings**" paragraph; Step 6.5's "**Known issues and follow-ups**" paragraph and a new paragraph after it; the "The final report **enumerates**" paragraph under *Terminal disposition (driver contract)*)
- Modify: `internal/repoguard/budgets_test.go` (`skillBudgets` row `"docket-implement-next/SKILL.md"`)
- Modify (test): `internal/repoguard/prose_contracts_test.go` (append after `TestResultsReviewPlacementDocContracts`)
- Regenerate: `internal/assets/embedded/` via `go generate ./internal/assets/`

**Interfaces:**
- Consumes: existing `docSectionContract` struct, `scanDocSection(content string, c docSectionContract) []string`, `collapseWS(s string) string`, `guardRoot(t *testing.T) string` (all in package `repoguard`).
- Produces (Task 2 extends these):
  - `type docFileClauseContract struct { change, file string; present, absent []string }`
  - `var followUpBacklogMatchDocContracts []docSectionContract`
  - `var followUpBacklogMatchFileClauses []docFileClauseContract`
  - `const followUpBacklogMatchFloor = 14` (Task 2 raises it to 20)
  - `func TestFollowUpBacklogMatchDocContracts(t *testing.T)`
  - The Step 6.5 paragraph is titled `**Backlog match.**`; other files point at it as "Step 6.5 *Backlog match*".

- [ ] **Step 1: Write the failing test**

Append to `internal/repoguard/prose_contracts_test.go`, directly after the closing brace of `TestResultsReviewPlacementDocContracts`:

```go
// change 0510 — at final consolidation implement-next matches every reported
// out-of-scope follow-up against the backlog's proposed and deferred changes
// and records one verdict plus a state-matched next action, in the results file
// and the final report. Present clauses bind each claim inside its section and
// are matched whitespace-collapsed (phrase-grep-over-wrapped-prose,
// prose-guard-binds-phrase-to-claim). Absent clauses are the retired
// "link an existing change when one is known" family, so restoring the old
// wording reddens (assert-detects-removal-not-replacement). Mutation-tested at
// introduction.
var followUpBacklogMatchDocContracts = []docSectionContract{
	{change: "change_0510_step65_backlog_match", file: "skills/docket-implement-next/SKILL.md",
		section: "### Step 6.5 — Results (required)", terminator: "### Step 7 — PR + stop",
		present: []string{
			"final consolidation checks each against the backlog and names any match",
			"**Backlog match.** Once, at checkpoint (iv) final consolidation",
			"an in-scope limitation or a verification-coverage note gets no verdict",
			"keep the changes whose `status` is `proposed` or `deferred`, excluding this change",
			"For **every** candidate, not only those with similar titles, read the `## Why` and `## What changes` sections",
			"**Fits #N**",
			"**Related to #N**",
			"**No existing change fits (checked K)**",
			"The final report's follow-up list carries the same verdict per item",
			"The match only recommends: it never edits, creates, revives, defers, or kills any change",
			"never halt, retry in a loop, or block the implemented transition on it",
		},
		absent: []string{"link an existing change when one is known"}},
	{change: "change_0510_step3_verdict_pointer", file: "skills/docket-implement-next/SKILL.md",
		section: "### Step 3 — Reconcile ⭐", terminator: "### Step 4 — Worktree + plan",
		present: []string{
			"so nothing is minted",
			"guided by the backlog verdict final consolidation attaches (Step 6.5 *Backlog match*)",
		}},
	{change: "change_0510_step6_verdict_pointer", file: "skills/docket-implement-next/SKILL.md",
		section: "### Step 6 — Review + ADRs", terminator: "### Step 6.5 — Results (required)",
		present: []string{
			"reported as follow-up work in the final report, carrying the backlog verdict final consolidation attaches",
		}},
	{change: "change_0510_final_report_verdict", file: "skills/docket-implement-next/SKILL.md",
		section: "### Terminal disposition (driver contract)", terminator: "### Atomic board rendering",
		present: []string{
			"any follow-up work **reported for deliberate capture**, each with its backlog verdict (Step 6.5 *Backlog match*)",
		}},
}

// docFileClauseContract pins clauses that live in a file's LAST section, which
// has no closing heading for scanDocSection to key on. The file itself is the
// section, so present and absent are both matched file-wide, whitespace-collapsed.
type docFileClauseContract struct {
	change  string
	file    string   // slash path relative to repo root
	present []string // clauses required anywhere in the file
	absent  []string // retired clauses that must appear nowhere in the file
}

var followUpBacklogMatchFileClauses = []docFileClauseContract{}

// followUpBacklogMatchFloor is the population floor over both tables: a
// collapse means rows were lost or the tables were gutted.
const followUpBacklogMatchFloor = 14

func TestFollowUpBacklogMatchDocContracts(t *testing.T) {
	root := guardRoot(t)

	checks := 0
	for _, c := range followUpBacklogMatchDocContracts {
		checks += len(c.present) + len(c.absent)
	}
	for _, c := range followUpBacklogMatchFileClauses {
		checks += len(c.present) + len(c.absent)
	}
	if checks < followUpBacklogMatchFloor {
		t.Fatalf("population floor: only %d backlog-match doc clauses (expected >= %d)", checks, followUpBacklogMatchFloor)
	}

	cache := map[string]string{}
	read := func(rel, change string) string {
		if s, ok := cache[rel]; ok {
			return s
		}
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read contract file %s (%s): %v (fail closed)", rel, change, err)
		}
		cache[rel] = string(b)
		return cache[rel]
	}

	var violations []string
	for _, c := range followUpBacklogMatchDocContracts {
		for _, msg := range scanDocSection(read(c.file, c.change), c) {
			violations = append(violations, fmt.Sprintf("[%s] %s", c.change, msg))
		}
	}
	for _, c := range followUpBacklogMatchFileClauses {
		whole := collapseWS(read(c.file, c.change))
		for _, p := range c.present {
			if !strings.Contains(whole, collapseWS(p)) {
				violations = append(violations, fmt.Sprintf("[%s] %s: missing required clause %q", c.change, c.file, p))
			}
		}
		for _, a := range c.absent {
			if strings.Contains(whole, collapseWS(a)) {
				violations = append(violations, fmt.Sprintf("[%s] %s: retired clause is present: %q", c.change, c.file, a))
			}
		}
	}
	if len(violations) != 0 {
		t.Errorf("backlog-match doc-contract violations (%d):\n%s", len(violations), strings.Join(violations, "\n"))
	}
}
```

The four section/terminator headings above were verified verbatim against the current `skills/docket-implement-next/SKILL.md` at plan time; re-check with `grep -n -F -- '### Step' skills/docket-implement-next/SKILL.md` if the test reports a missing heading.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -count=1 ./internal/repoguard/ -run TestFollowUpBacklogMatchDocContracts`
Expected: FAIL listing missing required clauses for all four `change_0510_*` rows and `retired clause is present: "link an existing change when one is known"`. (Population floor passes: 12 + 2 + 1 + 1 = 16 ≥ 14; Task 2 brings it to 22 against a floor of 20.)

- [ ] **Step 3: Edit Step 3's adjacent-follow-up note**

In `skills/docket-implement-next/SKILL.md`, replace exactly:

```
Adjacent follow-up work this pass surfaces is **noted for the final report** — automatic change capture is deferred from Go v1, so nothing is minted; a human captures reported work deliberately with `docket change create`.
```

with:

```
Adjacent follow-up work this pass surfaces is **noted for the final report** — automatic change capture is deferred from Go v1, so nothing is minted; a human captures reported work deliberately with `docket change create`, guided by the backlog verdict final consolidation attaches (Step 6.5 *Backlog match*).
```

- [ ] **Step 4: Edit Step 6's review-finding report rule**

In the "**Triage the returned findings, then FIX them in-branch.**" paragraph, replace exactly:

```
A finding that is genuinely distinct beyond-the-branch work is reported as follow-up work in the final report; a finding about this branch's own diff never is.
```

with:

```
A finding that is genuinely distinct beyond-the-branch work is reported as follow-up work in the final report, carrying the backlog verdict final consolidation attaches; a finding about this branch's own diff never is.
```

- [ ] **Step 5: Edit Step 6.5's Known-issues sentence and add the Backlog match paragraph**

In the "**Known issues and follow-ups** merges unresolved findings" paragraph, replace exactly:

```
Follow-ups recorded there hold out-of-scope work for **human triage** — link an existing change when one is known, and **never** mint a change, issue, ADR, or learning automatically.
```

with:

```
Follow-ups recorded there hold out-of-scope work for **human triage** — final consolidation checks each against the backlog and names any match (*Backlog match* below), and the run **never** mints a change, issue, ADR, or learning automatically.
```

Then insert a new paragraph (one line, preceded and followed by a blank line) **between** that "**Known issues and follow-ups**" paragraph and the "**Review findings.**" paragraph:

```
**Backlog match.** Once, at checkpoint (iv) final consolidation, after the Known-issues entries are settled, match each out-of-scope follow-up against the backlog; an in-scope limitation or a verification-coverage note gets no verdict, and earlier checkpoints never run the match. Run the `status` operation (argv from the capability catalog) with `--json` and keep the changes whose `status` is `proposed` or `deferred`, excluding this change — claimed, built, and archived changes are never candidates. For **every** candidate, not only those with similar titles, read the `## Why` and `## What changes` sections of the record at its `path` in the metadata working tree, and its linked spec only when those leave the match undecided. Give each follow-up exactly one verdict: **Fits #N** — #N's stated scope covers it, or would with a small extension that does not change what #N is for (name the closest; mention any others); **Related to #N** — same area, but folding it in would change what #N is for; or **No existing change fits (checked K)**, K being the number of proposed and deferred changes checked. End the entry's suggested next action with the verdict and the action for #N's state: a needs-grooming `proposed` #N — edit it through `docket-groom-next <N>` (`change.groom` `revise`), and it stays needs-grooming; a groomed `proposed` #N — edit its owned sections, revising its spec with `docket-groom-next <N>` when the spec must change; a `deferred` #N — revive it, then edit it, weighing whether the follow-up strengthens the case to revive; related or no fit — capture a new change with `docket change create`, linking any related #N under `related:`. The final report's follow-up list carries the same verdict per item. The match only recommends: it never edits, creates, revives, defers, or kills any change, writes only this run's results file and final report, and is never a gate. If the `status` read fails or a candidate record cannot be read, write the entries without verdicts and add one line to `## Verification performed` saying the backlog check did not run, naming any unreadable candidates — never halt, retry in a loop, or block the implemented transition on it.
```

- [ ] **Step 6: Edit the final-report enumeration**

Under `### Terminal disposition (driver contract)`, replace exactly:

```
any follow-up work **reported for deliberate capture**, and which disposition ended the run.
```

with:

```
any follow-up work **reported for deliberate capture**, each with its backlog verdict (Step 6.5 *Backlog match*), and which disposition ended the run.
```

- [ ] **Step 7: Run the contract test to verify it passes**

Run: `go test -count=1 ./internal/repoguard/ -run TestFollowUpBacklogMatchDocContracts`
Expected: PASS.

- [ ] **Step 8: Re-baseline the size budget**

Run: `wc -l < skills/docket-implement-next/SKILL.md; wc -w < skills/docket-implement-next/SKILL.md`
Expected: about 218 lines (was 216) and roughly 8,700 words (was 8,356) — use the measured values, called `L` and `W` below.

In `internal/repoguard/budgets_test.go`, change the row
`{"docket-implement-next/SKILL.md", 216, 8356},` to `{"docket-implement-next/SKILL.md", L, W},` and prepend to that row's trailing comment:
`0510: +Step 6.5 Backlog match paragraph (proposed/deferred candidates, three verdicts, state-matched next action, never-gate failure posture) and verdict pointers in Step 3, Step 6, and the final-report enumeration (216/8356 -> L/W); `
(substitute the measured numbers for `L` and `W`; keep `gofmt` column alignment).

Run: `go test -count=1 ./internal/repoguard/ -run TestSkillSizeBudgets`
Expected: PASS.

- [ ] **Step 9: Regenerate the embedded bundle**

Run (from the worktree root): `go generate ./internal/assets/ && go run ./cmd/genassets -check`
Expected: the generate step prints `genassets: wrote internal/assets/embedded (...)`; the check prints `... matches the authored roots ...`. `git status --porcelain` shows modified `internal/assets/embedded/manifest.json` and `internal/assets/embedded/tree/skills/docket-implement-next/SKILL.md` only (plus the three files edited above).

Run: `go test -count=1 ./internal/assets/ -run TestEmbeddedMatchesAuthored`
Expected: PASS.

- [ ] **Step 10: Mutation-check the guard**

```bash
WT=/Users/homer/dev/docket/.worktrees/match-reported-follow-ups-against-proposed-and-deferred-chan
BK="$(mktemp "${TMPDIR:-/tmp}/skill-0510.XXXXXX")"
cp "$WT/skills/docket-implement-next/SKILL.md" "$BK"
# Mutation A: restore the retired sentence (the absent clause must redden).
perl -0pi -e 's/final consolidation checks each against the backlog and names any match \(\*Backlog match\* below\), and the run \*\*never\*\* mints/link an existing change when one is known, and **never** mint/' "$WT/skills/docket-implement-next/SKILL.md"
go test -count=1 ./internal/repoguard/ -run TestFollowUpBacklogMatchDocContracts   # Expected: FAIL naming the retired clause and the missing step65 clause
cp -f "$BK" "$WT/skills/docket-implement-next/SKILL.md"
# Mutation B: weaken candidate reading to title-only (the every-candidate clause must redden).
perl -0pi -e 's/For \*\*every\*\* candidate, not only those with similar titles, read/For candidates with similar titles, read/' "$WT/skills/docket-implement-next/SKILL.md"
go test -count=1 ./internal/repoguard/ -run TestFollowUpBacklogMatchDocContracts   # Expected: FAIL naming the "For **every** candidate" clause
cp -f "$BK" "$WT/skills/docket-implement-next/SKILL.md"
rm -f "$BK"
go test -count=1 ./internal/repoguard/ -run TestFollowUpBacklogMatchDocContracts   # Expected: PASS
git -C "$WT" diff --stat   # confirm the SKILL.md diff is the intended edit, not a mutation leftover
```

Each mutation must be confirmed to have landed (the FAIL output names the expected clause) before the restore; a mutation that stays green is a defect in the guard — fix the row, do not proceed.

- [ ] **Step 11: Commit**

```bash
WT=/Users/homer/dev/docket/.worktrees/match-reported-follow-ups-against-proposed-and-deferred-chan
git -C "$WT" add skills/docket-implement-next/SKILL.md internal/repoguard/prose_contracts_test.go internal/repoguard/budgets_test.go internal/assets/embedded/manifest.json internal/assets/embedded/tree/skills/docket-implement-next/SKILL.md
git -C "$WT" commit -m "feat(implement-next): match reported follow-ups against proposed and deferred changes"
```

---

### Task 2: fix-loop, results template, and convention wording

**Files:**
- Modify: `skills/docket-implement-next/references/fix-loop.md` (section `## Beyond-the-branch findings are reported` — the file's last section)
- Modify: `skills/docket-implement-next/results-template.md` (the `## Known issues and follow-ups` placeholder — the file's last section)
- Modify: `skills/docket-convention/SKILL.md` (the "**Results artifact shape and lifecycle.**" paragraph under `### Directory layout (paths relative to the configured knobs)`)
- Modify: `internal/repoguard/budgets_test.go` (rows for `fix-loop.md` and `results-template.md`; the convention row has headroom — 390/7848 against 400/7969 — and stays unchanged unless the measured count exceeds it)
- Modify (test): `internal/repoguard/prose_contracts_test.go` (extend Task 1's tables and floor)
- Regenerate: `internal/assets/embedded/` via `go generate ./internal/assets/`

**Interfaces:**
- Consumes: from Task 1 — `followUpBacklogMatchDocContracts`, `followUpBacklogMatchFileClauses`, `docFileClauseContract`, `followUpBacklogMatchFloor`, `TestFollowUpBacklogMatchDocContracts`; the Step 6.5 paragraph name `*Backlog match*`.
- Produces: nothing new for later tasks.

- [ ] **Step 1: Extend the contract tables (failing)**

In `internal/repoguard/prose_contracts_test.go`:

Append this row inside `followUpBacklogMatchDocContracts` (after the `change_0510_final_report_verdict` row):

```go
	{change: "change_0510_convention_backlog_match", file: "skills/docket-convention/SKILL.md",
		section: "### Directory layout (paths relative to the configured knobs)", terminator: "### Change manifest (frontmatter at the top of each change file)",
		present: []string{
			"the run checks proposed and deferred changes and names any match, but never mints a change, issue, ADR, or learning automatically",
		},
		absent: []string{"link an existing change when one is known"}},
```

Replace `var followUpBacklogMatchFileClauses = []docFileClauseContract{}` with:

```go
var followUpBacklogMatchFileClauses = []docFileClauseContract{
	{change: "change_0510_fix_loop_verdict_pointer", file: "skills/docket-implement-next/references/fix-loop.md",
		present: []string{
			"never minted: automatic change capture is deferred from Go v1",
			"The report carries the backlog verdict final consolidation attaches (SKILL.md Step 6.5 *Backlog match*)",
		}},
	{change: "change_0510_template_verdict", file: "skills/docket-implement-next/results-template.md",
		present: []string{
			"end that action with the backlog verdict — Fits #N, Related to #N, or No existing change fits (checked K) — and the next step for #N's state",
		},
		absent: []string{"linking an existing change when available"}},
}
```

Change `const followUpBacklogMatchFloor = 14` to `const followUpBacklogMatchFloor = 20`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -count=1 ./internal/repoguard/ -run TestFollowUpBacklogMatchDocContracts`
Expected: FAIL naming the convention, fix-loop, and template present clauses, plus `retired clause is present` for both absent clauses. (The template's retired phrase wraps across two lines in the source; the whitespace-collapsed match must still find it — if it does not report it, the guard is broken.)

- [ ] **Step 3: Edit fix-loop.md**

In `skills/docket-implement-next/references/fix-loop.md`, replace exactly:

```
A genuinely distinct, beyond-the-branch finding is **reported as follow-up work in the final
report**, never minted: automatic change capture is deferred from Go v1, so a human captures
reported work deliberately with `docket change create`.
```

with:

```
A genuinely distinct, beyond-the-branch finding is **reported as follow-up work in the final
report**, never minted: automatic change capture is deferred from Go v1, so a human captures
reported work deliberately with `docket change create`. The report carries the backlog verdict
final consolidation attaches (SKILL.md Step 6.5 *Backlog match*).
```

Keep the file's trailing newline.

- [ ] **Step 4: Edit results-template.md**

In `skills/docket-implement-next/results-template.md`, replace exactly:

```
<When it occurs and what the person experiences; its practical impact; whether it is
confirmed or suspected; any available workaround; and the suggested next action, linking
an existing change when available. Keep each entry understandable without following its
technical links. A fixed finding belongs here only when it explains a remaining risk or
a consequential design decision.>
```

with:

```
<When it occurs and what the person experiences; its practical impact; whether it is
confirmed or suspected; any available workaround; and the suggested next action. For
out-of-scope follow-up work, end that action with the backlog verdict — Fits #N, Related
to #N, or No existing change fits (checked K) — and the next step for #N's state: edit
#N, revise its spec, revive it first, or capture a new change. Keep each entry
understandable without following its technical links. A fixed finding belongs here only
when it explains a remaining risk or a consequential design decision.>
```

- [ ] **Step 5: Edit the convention's results-lifecycle sentence**

In `skills/docket-convention/SKILL.md`, replace exactly:

```
Follow-ups recorded here hold out-of-scope work for human triage — link an existing change when one is known, but never mint a change, issue, ADR, or learning automatically.
```

with:

```
Follow-ups recorded here hold out-of-scope work for human triage — the run checks proposed and deferred changes and names any match, but never mints a change, issue, ADR, or learning automatically.
```

- [ ] **Step 6: Run the contract test to verify it passes**

Run: `go test -count=1 ./internal/repoguard/ -run TestFollowUpBacklogMatchDocContracts`
Expected: PASS.

- [ ] **Step 7: Re-baseline the size budgets**

Run:

```bash
WT=/Users/homer/dev/docket/.worktrees/match-reported-follow-ups-against-proposed-and-deferred-chan
for f in docket-implement-next/references/fix-loop.md docket-implement-next/results-template.md docket-convention/SKILL.md; do printf '%s %s %s\n' "$f" "$(wc -l < "$WT/skills/$f" | tr -d ' ')" "$(wc -w < "$WT/skills/$f" | tr -d ' ')"; done
```

Expected: fix-loop about 193 lines / ~2,005 words (was 192/1991); results-template about 70 lines / ~540 words (was 68/511); convention about 390 lines / ~7,850 words (ceiling 400/7969 — no change needed).

In `internal/repoguard/budgets_test.go`, set the `fix-loop.md` and `results-template.md` rows to the measured counts and prepend to each comment, respectively:
- `0510: +backlog-verdict pointer in Beyond-the-branch findings are reported (192/1991 -> L/W); `
- `0510: +backlog verdict and state-matched next action in the Known issues placeholder (68/511 -> L/W); `

Touch the convention row only if its measured count exceeds 400/7969 (then the same style: `0510: +backlog-match follow-up sentence (...)`).

Run: `go test -count=1 ./internal/repoguard/ -run TestSkillSizeBudgets`
Expected: PASS.

- [ ] **Step 8: Regenerate the embedded bundle**

Run (from the worktree root): `go generate ./internal/assets/ && go run ./cmd/genassets -check && go test -count=1 ./internal/assets/ -run TestEmbeddedMatchesAuthored`
Expected: `... matches the authored roots ...` and PASS. `git status --porcelain` shows the three embedded tree copies (`tree/skills/docket-implement-next/references/fix-loop.md`, `tree/skills/docket-implement-next/results-template.md`, `tree/skills/docket-convention/SKILL.md`) and `manifest.json` modified, alongside the three authored files and two test files.

- [ ] **Step 9: Mutation-check the new rows**

```bash
WT=/Users/homer/dev/docket/.worktrees/match-reported-follow-ups-against-proposed-and-deferred-chan
T="$WT/skills/docket-implement-next/results-template.md"
BK="$(mktemp "${TMPDIR:-/tmp}/tmpl-0510.XXXXXX")"
cp "$T" "$BK"
# Restore the retired, line-wrapped phrase (proves the collapsed absent match sees across a wrap).
perl -0pi -e 's/and the suggested next action\. For\n/and the suggested next action, linking\nan existing change when available. For\n/' "$T"
go test -count=1 ./internal/repoguard/ -run TestFollowUpBacklogMatchDocContracts   # Expected: FAIL "retired clause is present: \"linking an existing change when available\""
cp -f "$BK" "$T"; rm -f "$BK"
F="$WT/skills/docket-implement-next/references/fix-loop.md"
BK="$(mktemp "${TMPDIR:-/tmp}/fixloop-0510.XXXXXX")"
cp "$F" "$BK"
# Drop the verdict pointer sentence (proves the file-wide present branch reddens).
perl -0pi -e 's/ The report carries the backlog verdict\nfinal consolidation attaches \(SKILL\.md Step 6\.5 \*Backlog match\*\)\.//' "$F"
go test -count=1 ./internal/repoguard/ -run TestFollowUpBacklogMatchDocContracts   # Expected: FAIL naming the fix-loop present clause
cp -f "$BK" "$F"; rm -f "$BK"
go test -count=1 ./internal/repoguard/ -run TestFollowUpBacklogMatchDocContracts   # Expected: PASS
git -C "$WT" diff --stat
```

A mutation that leaves the test green is a guard defect — fix the row before committing.

- [ ] **Step 10: Run the repoguard and assets packages whole**

Run: `go test -count=1 ./internal/repoguard/ ./internal/assets/`
Expected: PASS (catches any other repoguard prose/anchor guard the new test-file text or skill prose trips, e.g. `TestCommentAnchorStyle`).

- [ ] **Step 11: Commit**

```bash
WT=/Users/homer/dev/docket/.worktrees/match-reported-follow-ups-against-proposed-and-deferred-chan
git -C "$WT" add skills/docket-implement-next/references/fix-loop.md skills/docket-implement-next/results-template.md skills/docket-convention/SKILL.md internal/repoguard/prose_contracts_test.go internal/repoguard/budgets_test.go internal/assets/embedded/manifest.json internal/assets/embedded/tree/skills/docket-implement-next/references/fix-loop.md internal/assets/embedded/tree/skills/docket-implement-next/results-template.md internal/assets/embedded/tree/skills/docket-convention/SKILL.md
git -C "$WT" commit -m "docs(results): carry the backlog verdict in fix-loop, the results template, and the convention"
```

If the convention budget row was not touched, `budgets_test.go` is still staged for the fix-loop/template rows; if `git status` shows no change to a listed path, drop it from the `add` rather than forcing it.

---

## Build gate

`docket-build` runs the whole suite once at the end through the configured `build.test_command` (read from config; do not substitute a copy). Read the budget report on a green run for `BUDGET WATCH:` / `SERIAL CONFIRMED OVER BUDGET:` lines.

## Results-file notes for the coordinator

- Record under Verification performed that the spec's "root `skills/` is a derived copy of the embedded tree" premise was inverted at plan time: `skills/` is authored and the embedded bundle is generated from it, and the build edited `skills/` and regenerated.
- The behavior is prose a model follows; no Go test can drive a run's backlog match. A human check worth offering as Optional: on the next implement-next run that reports a follow-up, confirm its Known-issues entry ends with one of the three verdicts and a next action matching #N's state.
