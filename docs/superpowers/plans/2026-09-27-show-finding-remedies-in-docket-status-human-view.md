<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0456 — Show finding remedies in docket status human view](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-27-0456-show-finding-remedies-in-docket-status-human-view.md)**
<!-- docket:backlink:end -->
# Show Finding Remedies in Docket Status Human View — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `docket status`'s human view renders every finding severity (errors, warnings, notices) grouped under per-severity headings, counts all three in the health line, and prints each finding's remedy on an indented continuation line.

**Architecture:** A confined rewrite of section 5 of `StatusResult.HumanText` in `internal/app/status_human.go`: `countFindings` gains a notice tally, the health line always prints three pluralized counts (ok still means zero errors and zero warnings), findings render in one block per non-empty severity in fixed order, and `writeFinding` drops its padded severity column (the heading carries it) and emits a `remedy:` continuation block. Golden tests in `internal/app/status_human_test.go` are rewritten to the new layout and extended.

**Tech Stack:** Go (stdlib only — `fmt`, `strings`); table of goldens in `internal/app/status_human_test.go`.

**Spec:** `docs/superpowers/specs/2026-09-27-show-finding-remedies-in-docket-status-human-view-design.md` (lives on the `docket` metadata branch; readable at `.docket/docs/superpowers/specs/…` from the primary tree). Change: `docs/changes/active/0456-show-finding-remedies-in-docket-status-human-view.md`.

## Global Constraints

- Code changes are confined to `internal/app/status_human.go`; test changes to `internal/app/status_human_test.go`. No new files, types, flags, or schema changes.
- The JSON document is untouched: `StatusFinding`, `StatusSummary` (no notice counter is added), and `Findings` ordering stay exactly as they are.
- Group order is fixed: `errors:` → `warnings:` → `notices:`. An empty severity emits no heading. Within a group, rows keep landed report order (the order of `r.Findings`).
- Health line always prints three counts using the existing `pluralize` helper (`internal/app/config.go`, `func pluralize(n int, singular, plural string) string`). `ok` means zero errors AND zero warnings; notices never make a repository unhealthy.
- The severity set is closed: `error | warning | notice`. No catch-all group for any other string.
- Remedy continuation: four-space indent, `remedy: ` prefix on the first line, each subsequent line of a multi-line remedy indented four spaces; trailing newlines trimmed; empty remedy emits nothing.
- Sections 1–4 of the report and the failure `reason:`/`message:` lines are unchanged.
- Every `go test` invocation in this plan uses `-count=1` (Go's test cache can serve a stale pass; a cached result is absence of evidence, never a verdict).
- The build gate at the end of the build runs the complete configured suite (`build.test_command`), not only these tests — that gate belongs to the executing skill, not to any task here.

## Review Focus

The spec's silence on an input is not permission for it to break the program. Each line below names an input class the spec implies but its Testing section does not explicitly enumerate; the pinning test is added to Task 1 (they are all renderer inputs, and Task 1 owns the renderer).

1. A remedy that is only newlines (`"\n"`) — after trailing-newline trim it is empty, so no `remedy:` line appears at all (not a bare `remedy: ` stub). Pinned in `TestStatusHumanTextRemedies`.
2. A finding whose severity is outside the closed set (e.g. a raw `"info"` that skipped `normalizeSeverity`) — it is counted nowhere and rendered nowhere, deterministically, rather than crashing or leaking into a group. Pinned in `TestStatusHumanTextUnknownSeverityDropped`.
3. A failure result (`Reason` set) that also carries findings — the `reason:`/`message:` block and the grouped health section both render; the new grouping does not disturb the failure path. Pinned in `TestStatusHumanTextFailureReasonWithFindings`.
4. A notice that carries a full entity locator (not just `Field`) — `findingLocator` is severity-blind, so the locator renders in the notices group exactly as in the errors group. Pinned in `TestStatusHumanTextGroupOrderInterleaved` (notice with entity+identity).
5. A remedy on a non-error severity — the continuation block is severity-independent, so a notice's remedy renders too. Pinned in `TestStatusHumanTextRemedies` (multi-line remedy on a notice).

---

### Task 1: Rewrite the status human renderer's health section and its goldens

**Files:**
- Modify: `internal/app/status_human.go` (the `HumanText` doc comment and its section-5 body; `countFindings`; `writeFinding`)
- Test: `internal/app/status_human_test.go` (rewrite the five existing goldens; add six tests)

**Interfaces:**
- Consumes: `StatusResult`, `StatusFinding` (fields `Code, Severity, Entity, Identity, Field, Path, Message, Remedy`), `NewStatusResult(ResultApplied, StatusResult)`, `findingLocator(f StatusFinding) string` (unchanged), `pluralize(n int, singular, plural string) string` from `internal/app/config.go`.
- Produces: `countFindings(findings []StatusFinding) (errs, warns, notices int)` — three return values now; `writeFinding(b *strings.Builder, f StatusFinding)` — same signature, new row shape (no severity column, remedy continuation). Both are package-private with no callers outside `status_human.go` (verified: a repo grep for `countFindings|writeFinding` matches only `internal/app/status_human.go`).

- [ ] **Step 1: Rewrite the five existing goldens and add the six new tests (failing)**

In `internal/app/status_human_test.go`, update the `want` tails of the four whole-report goldens to the new layout. Only the health section changes; every line above `"health:"` stays byte-identical. The exact replacements:

`TestStatusHumanTextHealthy` — replace the final line:

```go
		"health: ok (0 errors, 0 warnings)"
```

with:

```go
		"health: ok (0 errors, 0 warnings, 0 notices)"
```

`TestStatusHumanTextUnhealthy` — replace the final three lines:

```go
		"health: 1 error, 1 warning\n" +
		"  error   artifact-missing change 0003 (spec) — linked spec not found\n" +
		"  warning unmet-dependency change 0003 — depends on unbuilt change 1"
```

with:

```go
		"health: 1 error, 1 warning, 0 notices\n" +
		"\n" +
		"errors:\n" +
		"  artifact-missing change 0003 (spec) — linked spec not found\n" +
		"\n" +
		"warnings:\n" +
		"  unmet-dependency change 0003 — depends on unbuilt change 1"
```

`TestStatusHumanTextFilteredEmptyProjection` — replace the final two lines:

```go
		"health: 1 error, 0 warnings\n" +
		"  error   parse-error change 0042 — frontmatter decode failed"
```

with:

```go
		"health: 1 error, 0 warnings, 0 notices\n" +
		"\n" +
		"errors:\n" +
		"  parse-error change 0042 — frontmatter decode failed"
```

`TestStatusHumanTextEmptyReady` — replace the final line:

```go
		"health: ok (0 errors, 0 warnings)"
```

with:

```go
		"health: ok (0 errors, 0 warnings, 0 notices)"
```

`TestStatusHumanTextDeterministic` is kept verbatim (its premise — byte-identical re-render — is unchanged; per the learnings ledger, a test whose premise survives is kept, not deleted).

Then append these six tests to the same file:

```go
// TestStatusHumanTextNoticesOnlyHealthy: notices alone never make a repository
// unhealthy — the health line stays "ok" with a real notice count, and the
// notices render under their own heading. Config-derived notices carry only a
// Field (no entity, no path), so findingLocator prints no locator for them.
func TestStatusHumanTextNoticesOnlyHealthy(t *testing.T) {
	r := NewStatusResult(ResultApplied, StatusResult{
		Context: StatusContext{
			DefaultBranch:         "main",
			DefaultBranchRevision: "111111111111",
			IntegrationBranch:     "main",
			IntegrationRevision:   "111111111111",
		},
		Summary: StatusSummary{
			TotalChanges: 1, ActiveChanges: 1, DisplayedChanges: 1,
		},
		Changes: []StatusChange{
			{ID: 1, Title: "One", Readiness: "build-ready", Ready: true},
		},
		Ready: []int{1},
		Findings: []StatusFinding{
			{Code: "deferred-setting", Severity: "notice", Field: "build.checkpoint", Message: ".docket.yml: build.checkpoint names a deferred capability"},
			{Code: "inert-setting", Severity: "notice", Field: "runners.opencode.permissions", Message: ".docket.yml: runners.opencode.permissions is inert"},
		},
	})
	got := r.HumanText()
	want := "" +
		"default branch: main @ 111111111111\n" +
		"integration branch: main @ 111111111111\n" +
		"\n" +
		"changes: 1 total, 1 active, 1 displayed\n" +
		"records: 0 adrs, 0 learnings\n" +
		"\n" +
		"ready queue: 1\n" +
		"\n" +
		"displayed changes:\n" +
		"  #1 One — build-ready; unmet deps: none; base: (default)\n" +
		"\n" +
		"health: ok (0 errors, 0 warnings, 2 notices)\n" +
		"\n" +
		"notices:\n" +
		"  deferred-setting — .docket.yml: build.checkpoint names a deferred capability\n" +
		"  inert-setting — .docket.yml: runners.opencode.permissions is inert"
	if got != want {
		t.Errorf("%q\n!=\n%q", got, want)
	}
}

// TestStatusHumanTextSingularNotice: the notice count pluralizes like the
// other severities — "1 notice", never "1 notices".
func TestStatusHumanTextSingularNotice(t *testing.T) {
	r := NewStatusResult(ResultApplied, StatusResult{
		Context: StatusContext{
			DefaultBranch:         "main",
			DefaultBranchRevision: "222222222222",
			IntegrationBranch:     "main",
			IntegrationRevision:   "222222222222",
		},
		Findings: []StatusFinding{
			{Code: "inert-setting", Severity: "notice", Field: "x", Message: "m"},
		},
	})
	got := r.HumanText()
	if !strings.Contains(got, "health: ok (0 errors, 0 warnings, 1 notice)\n") {
		t.Errorf("singular notice count missing from:\n%s", got)
	}
	if strings.Contains(got, "1 notices") {
		t.Errorf("plural used for a single notice:\n%s", got)
	}
}

// TestStatusHumanTextGroupOrderInterleaved: findings interleaved across all
// three severities render as errors, then warnings, then notices — each group
// preserving the relative landed order of r.Findings, each preceded by a blank
// line and its heading, with no severity column on the rows. The notice with an
// entity locator proves findingLocator is severity-blind.
func TestStatusHumanTextGroupOrderInterleaved(t *testing.T) {
	r := NewStatusResult(ResultApplied, StatusResult{
		Context: StatusContext{
			DefaultBranch:         "main",
			DefaultBranchRevision: "333333333333",
			IntegrationBranch:     "main",
			IntegrationRevision:   "333333333333",
		},
		Findings: []StatusFinding{
			{Code: "notice-first", Severity: "notice", Field: "a", Message: "n1"},
			{Code: "error-first", Severity: "error", Entity: "change", Identity: "0001", Message: "e1"},
			{Code: "warn-only", Severity: "warning", Entity: "change", Identity: "0002", Message: "w1"},
			{Code: "error-second", Severity: "error", Entity: "change", Identity: "0003", Message: "e2"},
			{Code: "notice-second", Severity: "notice", Entity: "change", Identity: "0004", Message: "n2"},
		},
	})
	got := r.HumanText()
	want := "" +
		"default branch: main @ 333333333333\n" +
		"integration branch: main @ 333333333333\n" +
		"\n" +
		"changes: 0 total, 0 active, 0 displayed\n" +
		"records: 0 adrs, 0 learnings\n" +
		"\n" +
		"ready queue: (empty)\n" +
		"\n" +
		"displayed changes: (none)\n" +
		"\n" +
		"health: 2 errors, 1 warning, 2 notices\n" +
		"\n" +
		"errors:\n" +
		"  error-first change 0001 — e1\n" +
		"  error-second change 0003 — e2\n" +
		"\n" +
		"warnings:\n" +
		"  warn-only change 0002 — w1\n" +
		"\n" +
		"notices:\n" +
		"  notice-first — n1\n" +
		"  notice-second change 0004 — n2"
	if got != want {
		t.Errorf("%q\n!=\n%q", got, want)
	}
}

// TestStatusHumanTextRemedies pins the remedy continuation contract: a
// single-line remedy renders as one four-space-indented "remedy:" line; a
// multi-line remedy indents every subsequent line to the same four-space
// column with trailing newlines trimmed; a remedy that is only newlines emits
// nothing; the block is severity-independent (the multi-line remedy rides a
// notice).
func TestStatusHumanTextRemedies(t *testing.T) {
	r := NewStatusResult(ResultApplied, StatusResult{
		Context: StatusContext{
			DefaultBranch:         "main",
			DefaultBranchRevision: "444444444444",
			IntegrationBranch:     "main",
			IntegrationRevision:   "444444444444",
		},
		Findings: []StatusFinding{
			{Code: "branch-malformed", Severity: "error", Entity: "change", Identity: "0454", Field: "branch", Message: "branch is not a valid git branch name",
				Remedy: "run: docket change repair-identity --id 454"},
			{Code: "newline-only", Severity: "warning", Entity: "change", Identity: "0002", Message: "w1",
				Remedy: "\n"},
			{Code: "multi-line", Severity: "notice", Field: "x", Message: "n1",
				Remedy: "first step\nsecond step\n"},
		},
	})
	got := r.HumanText()
	want := "" +
		"default branch: main @ 444444444444\n" +
		"integration branch: main @ 444444444444\n" +
		"\n" +
		"changes: 0 total, 0 active, 0 displayed\n" +
		"records: 0 adrs, 0 learnings\n" +
		"\n" +
		"ready queue: (empty)\n" +
		"\n" +
		"displayed changes: (none)\n" +
		"\n" +
		"health: 1 error, 1 warning, 1 notice\n" +
		"\n" +
		"errors:\n" +
		"  branch-malformed change 0454 (branch) — branch is not a valid git branch name\n" +
		"    remedy: run: docket change repair-identity --id 454\n" +
		"\n" +
		"warnings:\n" +
		"  newline-only change 0002 — w1\n" +
		"\n" +
		"notices:\n" +
		"  multi-line — n1\n" +
		"    remedy: first step\n" +
		"    second step"
	if got != want {
		t.Errorf("%q\n!=\n%q", got, want)
	}
}

// TestStatusHumanTextUnknownSeverityDropped: the DTO severity set is closed
// (error | warning | notice); a finding carrying any other string is counted
// nowhere and rendered nowhere — there is deliberately no catch-all group.
func TestStatusHumanTextUnknownSeverityDropped(t *testing.T) {
	r := NewStatusResult(ResultApplied, StatusResult{
		Context: StatusContext{
			DefaultBranch:         "main",
			DefaultBranchRevision: "555555555555",
			IntegrationBranch:     "main",
			IntegrationRevision:   "555555555555",
		},
		Findings: []StatusFinding{
			{Code: "stray", Severity: "info", Entity: "change", Identity: "0001", Message: "should not render"},
			{Code: "real-error", Severity: "error", Entity: "change", Identity: "0002", Message: "e1"},
		},
	})
	got := r.HumanText()
	if !strings.Contains(got, "health: 1 error, 0 warnings, 0 notices\n") {
		t.Errorf("unknown severity leaked into a count:\n%s", got)
	}
	if strings.Contains(got, "stray") || strings.Contains(got, "should not render") {
		t.Errorf("unknown-severity finding rendered:\n%s", got)
	}
	if !strings.Contains(got, "\nerrors:\n  real-error change 0002 — e1") {
		t.Errorf("real error missing (non-vacuity companion through the same render):\n%s", got)
	}
}

// TestStatusHumanTextFailureReasonWithFindings: a failure classification and
// grouped findings coexist — the reason/message block renders in its usual
// slot and the health section still groups what it has.
func TestStatusHumanTextFailureReasonWithFindings(t *testing.T) {
	r := NewStatusResult(ResultApplied, StatusResult{
		Reason:  "metadata-branch-missing",
		Message: "the docket branch was not found",
		Context: StatusContext{
			DefaultBranch:         "main",
			DefaultBranchRevision: "666666666666",
			IntegrationBranch:     "main",
			IntegrationRevision:   "666666666666",
		},
		Findings: []StatusFinding{
			{Code: "inert-setting", Severity: "notice", Field: "x", Message: "n1"},
		},
	})
	got := r.HumanText()
	if !strings.Contains(got, "\nreason: metadata-branch-missing\nmessage: the docket branch was not found\n") {
		t.Errorf("failure block missing:\n%s", got)
	}
	if !strings.Contains(got, "health: ok (0 errors, 0 warnings, 1 notice)\n\nnotices:\n  inert-setting — n1") {
		t.Errorf("grouped health missing on failure result:\n%s", got)
	}
}
```

The file's import block becomes:

```go
import (
	"strings"
	"testing"
)
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -count=1 -run 'TestStatusHumanText' ./internal/app/`
Expected: FAIL — the four rewritten goldens and all six new tests red against the old renderer (the old health line has two counts, a severity column, and no groups/remedies). `TestStatusHumanTextDeterministic` stays green. If anything red is a compile error rather than an assertion diff, fix the test code first — a test that cannot compile has proven nothing.

- [ ] **Step 3: Implement the renderer**

In `internal/app/status_human.go`:

(a) In the `HumanText` doc comment, replace the section-5 clause

```go
// (4) one line per displayed active change; (5) health totals followed by the
// ordered error and warning findings. Every empty state — an empty ready queue,
```

with

```go
// (4) one line per displayed active change; (5) health totals followed by the
// errors, warnings, and notices, each under its own heading, with each
// finding's remedy on an indented continuation line. Every empty state — an
// empty ready queue,
```

(b) Replace the whole section-5 body (from the `// 5. health totals…` comment through the end of its `if/else`, i.e. the block currently reading `errs, warns := countFindings(r.Findings)` … the two severity loops) with:

```go
	// 5. health totals, then one block per non-empty severity — errors,
	// warnings, notices, in that fixed order — each preceded by a blank line and
	// its heading, rows in landed report order. Counts come from the rendered
	// findings themselves so the header can never disagree with the rows beneath
	// it (the ConfigInspectionResult blockerCount pattern). Notices never make a
	// repository unhealthy: ok still means zero errors and zero warnings.
	errs, warns, notices := countFindings(r.Findings)
	if errs == 0 && warns == 0 {
		fmt.Fprintf(&b, "\nhealth: ok (%d %s, %d %s, %d %s)\n",
			errs, pluralize(errs, "error", "errors"),
			warns, pluralize(warns, "warning", "warnings"),
			notices, pluralize(notices, "notice", "notices"))
	} else {
		fmt.Fprintf(&b, "\nhealth: %d %s, %d %s, %d %s\n",
			errs, pluralize(errs, "error", "errors"),
			warns, pluralize(warns, "warning", "warnings"),
			notices, pluralize(notices, "notice", "notices"))
	}
	for _, group := range []struct{ severity, heading string }{
		{"error", "errors"},
		{"warning", "warnings"},
		{"notice", "notices"},
	} {
		wroteHeading := false
		for _, f := range r.Findings {
			if f.Severity != group.severity {
				continue
			}
			if !wroteHeading {
				fmt.Fprintf(&b, "\n%s:\n", group.heading)
				wroteHeading = true
			}
			writeFinding(&b, f)
		}
	}
```

(c) Replace `countFindings` (function and doc comment) with:

```go
// countFindings tallies every severity the human report surfaces — the status
// DTO's closed set of error, warning, and notice. Any other severity string is
// counted nowhere; the renderer deliberately has no catch-all group.
func countFindings(findings []StatusFinding) (errs, warns, notices int) {
	for _, f := range findings {
		switch f.Severity {
		case "error":
			errs++
		case "warning":
			warns++
		case "notice":
			notices++
		}
	}
	return errs, warns, notices
}
```

(d) Replace `writeFinding` (function and doc comment) with:

```go
// writeFinding renders one finding row — its code, a locator when the finding
// names an entity or a path, and the explanatory message — followed, when the
// finding carries a remedy, by a four-space-indented "remedy:" continuation
// block. The severity column is gone: the group heading above the row carries
// it. A multi-line remedy keeps every subsequent line at the same four-space
// column so the block cannot collapse into the next row; trailing newlines are
// trimmed, and an empty remedy emits nothing.
func writeFinding(b *strings.Builder, f StatusFinding) {
	fmt.Fprintf(b, "  %s", f.Code)
	if loc := findingLocator(f); loc != "" {
		fmt.Fprintf(b, " %s", loc)
	}
	fmt.Fprintf(b, " — %s\n", f.Message)
	remedy := strings.TrimRight(f.Remedy, "\n")
	if remedy == "" {
		return
	}
	for i, line := range strings.Split(remedy, "\n") {
		if i == 0 {
			fmt.Fprintf(b, "    remedy: %s\n", line)
		} else {
			fmt.Fprintf(b, "    %s\n", line)
		}
	}
}
```

Nothing else in the file changes: `findingLocator`, `shortRevision`, `formatDeps`, `effectiveBase`, and sections 1–4 of `HumanText` are untouched.

- [ ] **Step 4: Run the focused tests to verify they pass**

Run: `go test -count=1 -run 'TestStatusHumanText' ./internal/app/`
Expected: PASS (all eleven `TestStatusHumanText*` tests).

- [ ] **Step 5: Run the whole package**

Run: `go test -count=1 ./internal/app/`
Expected: PASS — nothing else in the package pins the health line (verified by a repo grep for `health: ok` / `countFindings` / `writeFinding` outside `status_human*.go`; the only other `health:` hit, `internal/reposetup/health.go`, is a different renderer and out of scope). If some other test reds on the new layout, that is a discovered consumer: update its expectation to the new layout in this same task — do not weaken the renderer.

- [ ] **Step 6: Commit**

```bash
git add internal/app/status_human.go internal/app/status_human_test.go
git commit -m "feat(status): show all finding severities and remedies in human view (change 0456)"
```

---

### Task 2: Mutation-probe the new goldens

The spec names five mutation probes; each must turn at least one test red or the guard is decoration. No commit comes out of this task — its deliverable is the verified evidence that the Task 1 tests actually detect the states they exist to forbid, and a byte-identical restored tree.

**Files:**
- Modify (transiently, restored after each probe): `internal/app/status_human.go`
- Test: `internal/app/status_human_test.go` (read-only here)

**Interfaces:**
- Consumes: the Task 1 implementation exactly as committed.
- Produces: nothing durable — the working tree must be clean at the end.

Procedure for every probe (the file was just committed in Task 1, but use the backup idiom unconditionally — `git checkout --` restores to HEAD and as a habit it destroys uncommitted work; the `cp` backup has no precondition):

```bash
f=internal/app/status_human.go
cp "$f" "$f.bak"
# <apply the probe's edit with the Edit tool>
git diff --stat -- "$f"   # MUST show the file changed — a probe that never landed proves nothing
go test -count=1 -run 'TestStatusHumanText' ./internal/app/
# expect the named test(s) to FAIL
mv -f "$f.bak" "$f"
go test -count=1 -run 'TestStatusHumanText' ./internal/app/
# expect PASS again before the next probe
```

Before believing any probe's reading, the `git diff` step must show the edit landed (an in-place substitution that silently failed to match yields a green run with nothing mutated, which reads exactly like a robust guard), and every run carries `-count=1` (a cached pass is a pre-mutation verdict wearing a green suit).

- [ ] **Step 1: Probe — remove the notices group**

Edit: delete the line `{"notice", "notices"},` from the group table in `HumanText`.
Expected red: `TestStatusHumanTextNoticesOnlyHealthy`, `TestStatusHumanTextGroupOrderInterleaved`, `TestStatusHumanTextRemedies` (at least one; all three assert a `notices:` block). Restore; re-run; PASS.

- [ ] **Step 2: Probe — swap the group order**

Edit: reorder the group table to `{"notice", "notices"}, {"warning", "warnings"}, {"error", "errors"}`.
Expected red: `TestStatusHumanTextGroupOrderInterleaved` (and the other whole-report goldens with 2+ groups, e.g. `TestStatusHumanTextUnhealthy`, `TestStatusHumanTextRemedies`). Restore; re-run; PASS.

- [ ] **Step 3: Probe — drop the remedy continuation line**

Edit: in `writeFinding`, change `remedy := strings.TrimRight(f.Remedy, "\n")` to `remedy := ""` (making the early `return` unconditional in effect).
Expected red: `TestStatusHumanTextRemedies`. Restore; re-run; PASS.

- [ ] **Step 4: Probe — count notices toward "not ok"**

Edit: change the health condition `if errs == 0 && warns == 0 {` to `if errs == 0 && warns == 0 && notices == 0 {`.
Expected red: `TestStatusHumanTextNoticesOnlyHealthy`, `TestStatusHumanTextSingularNotice`, `TestStatusHumanTextFailureReasonWithFindings` (each asserts `health: ok (…, N notices)` with N > 0). Restore; re-run; PASS.

- [ ] **Step 5: Probe — drop the multi-line continuation indent**

Edit: in `writeFinding`'s loop, change the subsequent-line branch `fmt.Fprintf(b, "    %s\n", line)` to `fmt.Fprintf(b, "%s\n", line)`.
Expected red: `TestStatusHumanTextRemedies` (its multi-line remedy pins the four-space column on `    second step`). Restore; re-run; PASS.

- [ ] **Step 6: Verify the tree is byte-identical to the Task 1 commit**

Run: `git status --porcelain` and `git diff`
Expected: empty output for both — no probe residue, no `.bak` files. Then one final `go test -count=1 ./internal/app/` — expected PASS.
