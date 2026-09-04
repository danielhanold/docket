<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0261 — Give '## Run halted' a board surface and a health check, like its two family members](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-04-0261-give-run-halted-a-board-surface-and-a-health-check-like-its.md)**
<!-- docket:backlink:end -->
# Run-halted board Readiness cell — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The In progress board table gains a trailing `Readiness` column that renders `run halted — needs you` for an in-progress change carrying the bare `## Run halted` body section, and an empty cell otherwise.

**Architecture:** A pure-render change inside `internal/render/board.go`: the in-progress arms of `boardSectionRow` and `boardSectionTableHeader` plus the package-doc column-layout comment move together, keyed on the **existing** `domain.Change.HasRunHalted()` predicate (decoded from the whole-line `## Run halted` heading in `internal/repository/decode.go` — no new predicate, no decode change). The board golden is re-blessed and a run-halted corpus fixture is added so the populated cell is exercised by the frozen corpus.

**Tech Stack:** Go; table-driven tests in `internal/render/board_test.go`; frozen golden + fixture corpus under `internal/render/testdata/board/`.

**Spec:** `docs/superpowers/specs/2026-08-09-give-run-halted-a-board-surface-and-a-health-check-like-its-design.md` — **read it through the change file's reconcile**: the spec was authored against the deleted Bash tree. Only its section 1 (the board cell) survives; the change file `docs/changes/active/0261-give-run-halted-a-board-surface-and-a-health-check-like-its.md` (metadata branch) is authoritative on scope.

## Global Constraints

- Cell wording is exactly `run halted — needs you` (lowercase, spaced em dash U+2014) — the family idiom of the Blocked table's `finalize blocked — needs you` (`boardBlockedReasonCell`).
- **Do not build:** any health check / `stale-run-halted` / `BOARD_CHECK_IDS` / horizon constant (never ported to Go — out of scope per the change file); any new predicate (`HasRunHalted()` exists); any digest arm (Go readiness is proposed-only; the board cell is the sole surface).
- The heading match is the decode layer's whole-line contract: `## Run halted — 2026-…` (dated variant) must NOT light the cell. The board consumes `HasRunHalted()` and adds no matching of its own.
- Golden re-blessing follows `internal/render/testdata/board/PROVENANCE.md`: a **throwaway** write-the-bytes test hook, hand-review of the diff, hook deleted before commit. Never commit a regeneration backdoor.
- Every test run whose purpose is to observe a change in outcome uses `-count=1` (Go's cache otherwise serves pre-mutation verdicts — learnings: `cached-runner-serves-a-mutated-tree`).
- Mutation-restore by hand-reverting the mutation edit, never `git checkout -- <file>` while the file holds uncommitted work (learnings: `mutation-restore-needs-a-backup-copy`).
- The build gate runs the whole suite via the command `build.test_command` resolves to (`go run ./cmd/docket development test` today) — never only the tests named here.

---

### Task 1: Readiness column in the In progress table (tests, code, golden re-bless)

**Files:**
- Modify: `internal/render/board.go` — the `case BoardSectionInProgress:` arm of `boardSectionRow`, the `case BoardSectionInProgress:` arm of `boardSectionTableHeader`, a new `boardInProgressReadinessCell` helper beside `boardBlockedReasonCell`, and the package-doc column-layout comment (the block listing `in-progress: # | Title | Priority | Type | Spec | Branch`).
- Modify: `internal/render/board_test.go` — two new tests.
- Modify: `internal/render/testdata/board/board.golden` — re-blessed (header + trailing empty cell on the one existing in-progress row).

**Interfaces:**
- Consumes: `domain.Change.HasRunHalted() bool` (internal/domain/entities.go); existing test helpers `boardFrom`, `optString`, `proposedChange`'s `domain.ChangeSpec` construction idiom; `document.Parse`, `repository.InputDocument`, `repository.BuildSnapshot` (already imported by `board_test.go`).
- Produces: `boardInProgressReadinessCell(c domain.Change) string` — returns `"run halted — needs you"` when `c.HasRunHalted()`, else `""`. Task 2's corpus fixture renders through it.

- [ ] **Step 1: Write the two failing tests**

Append to `internal/render/board_test.go` (beside `TestBoardReadinessAutoGroomBlocked`, whose pattern they follow):

```go
// TestBoardInProgressReadinessRunHalted: an in-progress change carrying the
// "## Run halted" marker surfaces the human call to action in the trailing
// Readiness cell; a healthy in-progress row renders the cell empty.
func TestBoardInProgressReadinessRunHalted(t *testing.T) {
	halted := domain.NewChange(domain.ChangeSpec{
		ID: 12, Slug: "halted", Title: "Halted run", Status: domain.StatusInProgress,
		Branch:       domain.OptionalString{State: domain.FieldPresent, Value: "feat/halted"},
		Spec:         optString("docs/superpowers/specs/halted-design.md"),
		HasRunHalted: true,
		Location:     domain.LocationActive, Path: "docs/changes/active/0012-halted.md",
	})
	running := domain.NewChange(domain.ChangeSpec{
		ID: 13, Slug: "running", Title: "Running fine", Status: domain.StatusInProgress,
		Branch:   domain.OptionalString{State: domain.FieldPresent, Value: "feat/running"},
		Spec:     optString("docs/superpowers/specs/running-design.md"),
		Location: domain.LocationActive, Path: "docs/changes/active/0013-running.md",
	})
	out := string(boardFrom(t, halted, running))
	if !strings.Contains(out, "| # | Title | Priority | Type | Spec | Branch | Readiness |") {
		t.Fatalf("In progress header missing trailing Readiness column:\n%s", out)
	}
	if !strings.Contains(out, "| `feat/halted` | run halted — needs you |") {
		t.Fatalf("run-halted Readiness cell not rendered:\n%s", out)
	}
	if !strings.Contains(out, "| `feat/running` |  |") {
		t.Fatalf("healthy in-progress row must render an empty Readiness cell:\n%s", out)
	}
}

// TestBoardRunHaltedWholeLineContract: the cell keys on the decode layer's
// whole-line "## Run halted" heading match (0237's bare-heading contract) — a
// dated variant ("## Run halted — 2026-…") must NOT light the cell. Renders
// through document.Parse + repository.BuildSnapshot (the boardCorpusSnapshot
// path) so the real parse pipeline, not a hand-set spec flag, drives the cell.
func TestBoardRunHaltedWholeLineContract(t *testing.T) {
	source := func(id int, slug, heading string) string {
		return "---\n" +
			"id: " + strconv.Itoa(id) + "\n" +
			"slug: " + slug + "\n" +
			"title: " + slug + "\n" +
			"status: in-progress\n" +
			"priority: high\n" +
			"type: feat\n" +
			"created: 2026-08-01\n" +
			"updated: 2026-08-0" + strconv.Itoa(id) + "\n" +
			"depends_on: []\n" +
			"related: []\n" +
			"discovered_from: []\n" +
			"adrs: []\n" +
			"spec: docs/superpowers/specs/" + slug + "-design.md\n" +
			"plan:\n" +
			"results:\n" +
			"trivial: false\n" +
			"auto_groomable:\n" +
			"branch: feat/" + slug + "\n" +
			"pr:\n" +
			"blocked_by:\n" +
			"reconciled: true\n" +
			"---\n\n## Why\n\nBody.\n\n" + heading + "\n\nSection body.\n"
	}
	var docs []repository.InputDocument
	for _, f := range []struct {
		id      int
		slug    string
		heading string
	}{
		{1, "bare", "## Run halted"},
		{2, "dated", "## Run halted — 2026-08-14"},
	} {
		doc, err := document.Parse([]byte(source(f.id, f.slug, f.heading)))
		if err != nil {
			t.Fatalf("parse fixture %s: %v", f.slug, err)
		}
		docs = append(docs, repository.InputDocument{
			Kind:     repository.KindChange,
			Location: domain.LocationActive,
			Path:     "docs/changes/active/000" + strconv.Itoa(f.id) + "-" + f.slug + ".md",
			Document: doc,
		})
	}
	build, err := repository.BuildSnapshot(repository.BuildInput{Documents: docs})
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	out, err := render.Board(render.BoardInput{Snapshot: build.Snapshot, Presentation: render.DefaultBoardPresentation()})
	if err != nil {
		t.Fatalf("Board: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "| `feat/bare` | run halted — needs you |") {
		t.Fatalf("bare heading did not light the Readiness cell:\n%s", s)
	}
	if !strings.Contains(s, "| `feat/dated` |  |") {
		t.Fatalf("dated heading variant must render an empty Readiness cell:\n%s", s)
	}
}
```

Notes: `strconv`, `document`, `repository`, `render`, `domain`, `strings` are all already imported at the top of `board_test.go`. The frontmatter mirrors the corpus fixture `corpus/active/0001-inline-board-in-progress.md` field-for-field so `BuildSnapshot` accepts it. If `document.Parse` or `BuildSnapshot` rejects the minimal source for a reason the corpus shape does not predict, diff against that fixture — do not weaken the assertions.

- [ ] **Step 2: Run the new tests, verify they fail**

Run: `go test -count=1 -run 'TestBoardInProgressReadinessRunHalted|TestBoardRunHaltedWholeLineContract' ./internal/render/`
Expected: FAIL — the header assert reddens first (no `Readiness` column on the In progress table yet).

- [ ] **Step 3: Implement the column (three sites move together)**

In `internal/render/board.go`:

(a) `boardSectionTableHeader`, `case BoardSectionInProgress:` — replace the return with:

```go
		return "| # | Title | Priority | Type | Spec | Branch | Readiness |\n|---|-------|----------|------|------|--------|-----------|\n"
```

(b) `boardSectionRow`, `case BoardSectionInProgress:` — replace the return with:

```go
	case BoardSectionInProgress:
		return fmt.Sprintf("| [%04d](active/%s) | %s | `%s` | `%s` | [spec](%s) | `%s` | %s |\n",
			id, base, title, priority, ctype, boardSpecLink(c.Spec().Value), c.Branch().Value,
			boardInProgressReadinessCell(c)), nil
```

(c) New helper directly after `boardBlockedReasonCell`:

```go
// boardInProgressReadinessCell renders the In progress Readiness cell: an
// in-progress change carrying the bare "## Run halted" body section (0237's
// stop + surface disposition, decoded as HasRunHalted) surfaces the call to
// action in the family idiom of boardBlockedReasonCell's
// "finalize blocked — needs you"; every other row renders an empty cell.
func boardInProgressReadinessCell(c domain.Change) string {
	if c.HasRunHalted() {
		return "run halted — needs you"
	}
	return ""
}
```

(d) Package-doc column-layout comment — in the block that reads

```
//     in-progress: # | Title | Priority | Type | Spec | Branch
```

append ` | Readiness` to that line (keeping the sibling lines aligned as they are), and in the same doc comment, directly after the sentence describing the Blocked Reason cell ("The Blocked Reason cell is the stored blocked_by text … (empty when no pr:)."), insert:

```
//     The In progress Readiness cell is "run halted — needs you" when the
//     change carries the bare "## Run halted" body section (HasRunHalted),
//     else empty.
```

- [ ] **Step 4: Run the new tests, verify they pass; observe the golden redden**

Run: `go test -count=1 -run 'TestBoardInProgressReadinessRunHalted|TestBoardRunHaltedWholeLineContract' ./internal/render/`
Expected: PASS.

Run: `go test -count=1 ./internal/render/`
Expected: FAIL — exactly `TestBoardGolden` (and only it): the frozen golden still carries the six-column In progress header. Any other failure is a defect to fix before proceeding.

- [ ] **Step 5: Re-bless the golden (throwaway hook, hand-reviewed diff)**

Per `internal/render/testdata/board/PROVENANCE.md`. Add temporarily to `board_test.go`:

```go
func TestRegenBoardGolden(t *testing.T) {
	got, err := render.Board(render.BoardInput{Snapshot: boardCorpusSnapshot(t), Presentation: render.DefaultBoardPresentation()})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("testdata", "board", "board.golden"), got, 0o644); err != nil {
		t.Fatal(err)
	}
}
```

Run: `go test -count=1 -run TestRegenBoardGolden ./internal/render/`

Hand-review `git diff internal/render/testdata/board/board.golden`. The diff must be exactly two lines changed: the In progress header rows gain `| Readiness |` / `|-----------|`, and row `0001` gains a trailing empty cell (`… | \`feat/inline-board\` |  |`). Counts line, every other section, mermaid, and the archive footer must be byte-identical. Then **delete `TestRegenBoardGolden`**.

- [ ] **Step 6: Package green**

Run: `go test -count=1 ./internal/render/`
Expected: PASS (golden included). Also confirm the hook is gone: `grep -c TestRegenBoardGolden internal/render/board_test.go` → `0`.

- [ ] **Step 7: Mutation-test the guard**

The guard is code (AGENTS.md): prove the new asserts detect removal.

1. Mutate: in `boardInProgressReadinessCell`, change `return "run halted — needs you"` to `return ""`. Confirm the mutation landed: `grep -c 'run halted — needs you' internal/render/board.go` must have dropped by 1.
2. Run: `go test -count=1 -run 'TestBoardInProgressReadinessRunHalted|TestBoardRunHaltedWholeLineContract' ./internal/render/` — Expected: BOTH tests FAIL. A green here is a defect in the tests.
3. Restore by hand-reverting the edit (never `git checkout --` — the file holds uncommitted work). Re-run the same command — Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/render/board.go internal/render/board_test.go internal/render/testdata/board/board.golden
git commit -m "feat(0261): In progress Readiness cell surfaces 'run halted — needs you'"
```

(Stage exactly these three paths — never `add -A`.)

---

### Task 2: Run-halted corpus fixture, golden re-bless, PROVENANCE

**Files:**
- Create: `internal/render/testdata/board/corpus/active/0008-run-halted-in-progress.md`
- Modify: `internal/render/testdata/board/board.golden` — re-blessed (same commit as the fixture, per the change file).
- Modify: `internal/render/testdata/board/PROVENANCE.md` — coverage list.

**Interfaces:**
- Consumes: `boardInProgressReadinessCell` (Task 1) via the corpus render path (`boardCorpusSnapshot` → `render.Board`); corpus ids 1–7 (active) and 9–10 (archive) are taken — id 8 is free.
- Produces: a frozen-corpus row exercising the **populated** cell, so `TestBoardGolden` pins it byte-for-byte.

- [ ] **Step 1: Add the fixture**

Create `internal/render/testdata/board/corpus/active/0008-run-halted-in-progress.md` (frontmatter mirrors `0001-inline-board-in-progress.md` field-for-field; `updated: 2026-08-09` sorts it **below** row 0001's 2026-08-10 under the default `updated desc`):

```markdown
---
id: 8
slug: run-halted-in-progress
title: An in-progress run that halted for a human
status: in-progress
priority: high
type: feat
created: 2026-08-05
updated: 2026-08-09
depends_on: []
related: []
discovered_from: []
adrs: []
spec: docs/superpowers/specs/2026-08-05-run-halted-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/run-halted-in-progress
pr:
blocked_by:
reconciled: true
---

## Why

An in-progress change whose autonomous run deliberately stopped needing a human.

## Run halted

The dispatch seam recorded a halt; a human must read this section and resolve it.
```

The `## Run halted` heading is **bare** — whole line, no date, no suffix; the prose lives under it, never on it.

- [ ] **Step 2: Observe the golden redden**

Run: `go test -count=1 ./internal/render/`
Expected: FAIL — exactly `TestBoardGolden` (the corpus grew). Anything else failing is a defect.

- [ ] **Step 3: Re-bless the golden (throwaway hook, hand-reviewed diff)**

Re-add the `TestRegenBoardGolden` hook from Task 1 Step 5, run `go test -count=1 -run TestRegenBoardGolden ./internal/render/`, then hand-review `git diff internal/render/testdata/board/board.golden` against this expectation — every line of it:

- Counts line: `**9 changes**` → `**10 changes**` and `🟢 1 in progress` → `🟢 2 in progress`; every other count unchanged.
- In progress section: heading `## 🟢 In progress (1)` → `(2)`; a new row after 0001:
  `| [0008](active/0008-run-halted-in-progress.md) | An in-progress run that halted for a human | \`high\` | \`feat\` | [spec](../superpowers/specs/2026-08-05-run-halted-design.md) | \`feat/run-halted-in-progress\` | run halted — needs you |`
- Mermaid graph: one new bare node line `  0008` between `  0007` and `  0009:::done` (ascending id order; no edges — `depends_on` is empty).
- Nothing else: Built/Blocked/Groomed/Proposed/Deferred tables and the archive footer byte-identical.

Then **delete `TestRegenBoardGolden`** again and confirm: `grep -c TestRegenBoardGolden internal/render/board_test.go` → `0`.

- [ ] **Step 4: Update PROVENANCE.md**

In `internal/render/testdata/board/PROVENANCE.md`, section "What the corpus covers":

Replace the In progress bullet

```markdown
- **In progress** — id 1, with a `spec:` (Spec cell) and a `branch:` (Branch cell).
```

with

```markdown
- **In progress** — two Readiness bands, sorted `updated desc`:
  - id 1 (updated 2026-08-10), with a `spec:` (Spec cell), a `branch:` (Branch
    cell), and no `## Run halted` section (empty Readiness cell);
  - id 8 (updated 2026-08-09), carrying the bare `## Run halted` body section:
    the `run halted — needs you` Readiness cell (change 0261).
```

And in the "deliberately does **not** cover" list, add one bullet (the whole-line contract is unit-tested, not corpus-tested):

```markdown
- the dated `## Run halted — <date>` variant NOT lighting the Readiness cell
  (whole-line decode contract — `TestBoardRunHaltedWholeLineContract`);
```

- [ ] **Step 5: Verify green**

Run: `go test -count=1 ./internal/render/`
Expected: PASS.

Run: `go test -count=1 ./...`
Expected: PASS (the board bytes feed no other package's golden, but prove it rather than assume it).

- [ ] **Step 6: Commit**

```bash
git add internal/render/testdata/board/corpus/active/0008-run-halted-in-progress.md internal/render/testdata/board/board.golden internal/render/testdata/board/PROVENANCE.md
git commit -m "test(0261): run-halted corpus fixture pins the populated Readiness cell"
```

---

## Self-review notes

- **Spec coverage:** spec §1 (board cell) → Tasks 1–2. Spec §2 (shared predicate) — already exists in Go (`HasRunHalted`), no task by design. Spec §§3–4 (health check, `BOARD_CHECK_IDS`) and the digest arm — out of scope per the change file's 2026-09-04 reconcile; deliberately no task. Spec §5's surviving test bullets (populated cell / empty cell / dated variant NOT matching) → Task 1 Steps 1–2 and Task 2's corpus row.
- **Wording:** `run halted — needs you` appears with the identical em-dash spelling in the helper, both tests, the fixture expectation, the golden diff review, and PROVENANCE.
- **Guard discipline:** the dated-variant assert detects the state being guarded against (a too-loose match), not merely the new wording; the mutation probe (Task 1 Step 7) proves both tests redden when the cell is stripped, with `-count=1` and a grep-confirmed landed mutation.
