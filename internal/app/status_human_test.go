package app

import (
	"strings"
	"testing"
)

// The four golden fixtures below are frozen renderer output: each `want` was
// derived from the HumanText renderer and then frozen. They assert the whole
// multi-line report so any drift in section order, spacing, or empty-state
// wording reddens (spec §Human report).

// healthyStatusResult exercises docket mode, both a default and a stacked base,
// a non-empty ready queue, revision truncation to 12 chars, and a healthy
// (zero-finding) repository.
func healthyStatusResult() StatusResult {
	return NewStatusResult(ResultApplied, StatusResult{
		Context: StatusContext{
			DefaultBranch:         "main",
			DefaultBranchRevision: "a1b2c3d4e5f6DEADBEEF",
			IntegrationBranch:     "develop",
			IntegrationRevision:   "b2c3d4e5f6a1FEEDFACE",
			MetadataRevision:      "c3d4e5f6a1b2CAFED00D",
		},
		Summary: StatusSummary{
			TotalChanges: 5, ActiveChanges: 3, DisplayedChanges: 3,
			ReadyChanges: 2, ADRs: 4, Learnings: 2,
		},
		Changes: []StatusChange{
			{ID: 7, Title: "Alpha change", Readiness: "build-ready", Ready: true},
			{ID: 12, Title: "Beta change", Readiness: "build-ready", EffectiveBase: "feat/0007", Ready: true},
		},
		Ready: []int{7, 12},
	})
}

func TestStatusHumanTextHealthy(t *testing.T) {
	got := healthyStatusResult().HumanText()
	want := "" +
		"default branch: main @ a1b2c3d4e5f6\n" +
		"integration branch: develop @ b2c3d4e5f6a1\n" +
		"metadata branch: docket @ c3d4e5f6a1b2\n" +
		"\n" +
		"changes: 5 total, 3 active, 3 displayed\n" +
		"records: 4 adrs, 2 learnings\n" +
		"\n" +
		"ready queue: 7, 12\n" +
		"\n" +
		"displayed changes:\n" +
		"  #7 Alpha change — build-ready; unmet deps: none; base: (default)\n" +
		"  #12 Beta change — build-ready; unmet deps: none; base: feat/0007\n" +
		"\n" +
		"health: ok (0 errors, 0 warnings, 0 notices)"
	if got != want {
		t.Errorf("%q\n!=\n%q", got, want)
	}
}

func TestStatusHumanTextUnhealthy(t *testing.T) {
	r := NewStatusResult(ResultApplied, StatusResult{
		Context: StatusContext{
			DefaultBranch:         "main",
			DefaultBranchRevision: "dddddddddddd",
			IntegrationBranch:     "main",
			IntegrationRevision:   "dddddddddddd",
		},
		Summary: StatusSummary{
			TotalChanges: 3, ActiveChanges: 2, DisplayedChanges: 2,
			ReadyChanges: 1, ADRs: 1, Learnings: 0,
			ErrorFindings: 1, WarningFindings: 1,
		},
		Changes: []StatusChange{
			{ID: 3, Title: "Gamma change", Readiness: "blocked", UnmetDeps: []int{1}},
			{ID: 9, Title: "Delta change", Readiness: "build-ready"},
		},
		Ready: []int{9},
		Findings: []StatusFinding{
			{Code: "artifact-missing", Severity: "error", Entity: "change", Identity: "0003", Field: "spec", Message: "linked spec not found"},
			{Code: "unmet-dependency", Severity: "warning", Entity: "change", Identity: "0003", Message: "depends on unbuilt change 1"},
		},
	})
	got := r.HumanText()
	want := "" +
		"default branch: main @ dddddddddddd\n" +
		"integration branch: main @ dddddddddddd\n" +
		"\n" +
		"changes: 3 total, 2 active, 2 displayed\n" +
		"records: 1 adrs, 0 learnings\n" +
		"\n" +
		"ready queue: 9\n" +
		"\n" +
		"displayed changes:\n" +
		"  #3 Gamma change — blocked; unmet deps: 1; base: (default)\n" +
		"  #9 Delta change — build-ready; unmet deps: none; base: (default)\n" +
		"\n" +
		"health: 1 error, 1 warning, 0 notices\n" +
		"\n" +
		"errors:\n" +
		"  artifact-missing change 0003 (spec) — linked spec not found\n" +
		"\n" +
		"warnings:\n" +
		"  unmet-dependency change 0003 — depends on unbuilt change 1"
	if got != want {
		t.Errorf("%q\n!=\n%q", got, want)
	}
}

// TestStatusHumanTextFilteredEmptyProjection: a projection that filtered every
// change away still reports the full-corpus health finding — filters narrow the
// display, never the health surface.
func TestStatusHumanTextFilteredEmptyProjection(t *testing.T) {
	r := NewStatusResult(ResultApplied, StatusResult{
		Context: StatusContext{
			DefaultBranch:         "main",
			DefaultBranchRevision: "0f0f0f0f0f0f",
			IntegrationBranch:     "develop",
			IntegrationRevision:   "a1a1a1a1a1a1",
			MetadataRevision:      "b2b2b2b2b2b2",
		},
		Summary: StatusSummary{
			TotalChanges: 4, ActiveChanges: 3, DisplayedChanges: 0,
			ReadyChanges: 0, ADRs: 2, Learnings: 1, ErrorFindings: 1,
		},
		Findings: []StatusFinding{
			{Code: "parse-error", Severity: "error", Entity: "change", Identity: "0042", Message: "frontmatter decode failed"},
		},
	})
	got := r.HumanText()
	want := "" +
		"default branch: main @ 0f0f0f0f0f0f\n" +
		"integration branch: develop @ a1a1a1a1a1a1\n" +
		"metadata branch: docket @ b2b2b2b2b2b2\n" +
		"\n" +
		"changes: 4 total, 3 active, 0 displayed\n" +
		"records: 2 adrs, 1 learnings\n" +
		"\n" +
		"ready queue: (empty)\n" +
		"\n" +
		"displayed changes: (none)\n" +
		"\n" +
		"health: 1 error, 0 warnings, 0 notices\n" +
		"\n" +
		"errors:\n" +
		"  parse-error change 0042 — frontmatter decode failed"
	if got != want {
		t.Errorf("%q\n!=\n%q", got, want)
	}
}

// TestStatusHumanTextEmptyReady: a healthy repo with displayed changes but an
// empty ready queue — the empty queue is an explicit line, not a dropped section.
func TestStatusHumanTextEmptyReady(t *testing.T) {
	r := NewStatusResult(ResultApplied, StatusResult{
		Context: StatusContext{
			DefaultBranch:         "main",
			DefaultBranchRevision: "999999999999",
			IntegrationBranch:     "main",
			IntegrationRevision:   "999999999999",
		},
		Summary: StatusSummary{
			TotalChanges: 2, ActiveChanges: 2, DisplayedChanges: 2,
		},
		Changes: []StatusChange{
			{ID: 1, Title: "One", Readiness: "blocked", UnmetDeps: []int{2}},
			{ID: 2, Title: "Two", Readiness: "in-progress"},
		},
	})
	got := r.HumanText()
	want := "" +
		"default branch: main @ 999999999999\n" +
		"integration branch: main @ 999999999999\n" +
		"\n" +
		"changes: 2 total, 2 active, 2 displayed\n" +
		"records: 0 adrs, 0 learnings\n" +
		"\n" +
		"ready queue: (empty)\n" +
		"\n" +
		"displayed changes:\n" +
		"  #1 One — blocked; unmet deps: 2; base: (default)\n" +
		"  #2 Two — in-progress; unmet deps: none; base: (default)\n" +
		"\n" +
		"health: ok (0 errors, 0 warnings, 0 notices)"
	if got != want {
		t.Errorf("%q\n!=\n%q", got, want)
	}
}

// TestStatusHumanTextDeterministic: identical input renders byte-identically.
func TestStatusHumanTextDeterministic(t *testing.T) {
	r := healthyStatusResult()
	if r.HumanText() != r.HumanText() {
		t.Errorf("HumanText is not deterministic across renders")
	}
}

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
				Remedy: "run: docket change relink --id 454"},
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
		"    remedy: run: docket change relink --id 454\n" +
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

// TestStatusHumanTextRemedyBlankInteriorLine: a blank interior remedy line
// renders as a bare empty line, never as four trailing spaces.
func TestStatusHumanTextRemedyBlankInteriorLine(t *testing.T) {
	r := NewStatusResult(ResultApplied, StatusResult{
		Context: StatusContext{
			DefaultBranch:         "main",
			DefaultBranchRevision: "666666666666",
			IntegrationBranch:     "main",
			IntegrationRevision:   "666666666666",
		},
		Findings: []StatusFinding{
			{Code: "gap", Severity: "error", Message: "e1", Remedy: "step one\n\nstep two"},
		},
	})
	got := r.HumanText()
	wantFrag := "    remedy: step one\n\n    step two"
	if !strings.Contains(got, wantFrag) {
		t.Errorf("missing %q in:\n%q", wantFrag, got)
	}
	for _, line := range strings.Split(got, "\n") {
		if strings.TrimRight(line, " \t") != line {
			t.Errorf("trailing whitespace on line %q", line)
		}
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
