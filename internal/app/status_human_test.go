package app

import "testing"

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
		"health: ok (0 errors, 0 warnings)"
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
		"health: 1 error, 1 warning\n" +
		"  error   artifact-missing change 0003 (spec) — linked spec not found\n" +
		"  warning unmet-dependency change 0003 — depends on unbuilt change 1"
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
		"health: 1 error, 0 warnings\n" +
		"  error   parse-error change 0042 — frontmatter decode failed"
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
		"health: ok (0 errors, 0 warnings)"
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
