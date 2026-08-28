package reposetup

import (
	"strings"
	"testing"
)

// destructiveSubstrings are the command shapes a remedy must never print: a
// conflict or dirty-worktree remedy names a human disposition, never a
// destructive recovery. Keyed on shape, not an allowlist of exact spellings.
var destructiveSubstrings = []string{
	"reset --hard", "--force", "force-push", "push -f", "rm -rf",
	"branch -D", "checkout --", "discard", "delete",
}

func assertNoDestructiveCommand(t *testing.T, remedy string) {
	t.Helper()
	for _, d := range destructiveSubstrings {
		if strings.Contains(remedy, d) {
			t.Fatalf("remedy names a destructive command %q: %q", d, remedy)
		}
	}
}

// baseResolvedFacts returns a Facts value whose required remote probes are all
// proven Present — the caller then perturbs one field to reach a target state.
func baseResolvedFacts() Facts {
	return Facts{
		RemoteConfigured:    PresencePresent,
		RemoteDefaultBranch: BranchFact{Presence: PresencePresent, Tip: "aaa"},
		RemoteIntegration:   BranchFact{Presence: PresencePresent, Tip: "bbb"},
		RemoteMetadata:      BranchFact{Presence: PresenceAbsent},
		LiveSurface:         PresenceAbsent,
	}
}

func findingByCode(fs []Finding, code string) (Finding, bool) {
	for _, f := range fs {
		if f.Code == code {
			return f, true
		}
	}
	return Finding{}, false
}

func TestHealthHealthyIsEmpty(t *testing.T) {
	got := EvaluateHealth(Classification{State: StateHealthy}, Facts{}, nil)
	if len(got) != 0 {
		t.Fatalf("healthy classification must yield no findings, got %d: %+v", len(got), got)
	}
}

func TestHealthEveryNonHealthyStateYieldsFinding(t *testing.T) {
	cases := []Classification{
		{State: StateFresh, Reasons: []string{"no-metadata-no-surface"}},
		{State: StateLegacy, Reasons: []string{"legacy-live-surface"}},
		{State: StateNeedsReview, Reasons: []string{"pending-review-paths"}},
		{State: StatePartial, Reasons: []string{"metadata-seeded"}},
		{State: StatePartial, Reasons: []string{"integration-pruned-attach-incomplete"}},
		{State: StateConflict, Reasons: []string{"metadata-root-foreign"}},
		{State: StateConflict, Reasons: []string{"postconditions-unmet"}},
		{State: StateUnknown, Reasons: []string{"remote-configured-unknown"}},
	}
	for _, c := range cases {
		if got := EvaluateHealth(c, Facts{}, nil); len(got) < 1 {
			t.Fatalf("state %q reasons %v must yield >=1 finding, got 0", c.State, c.Reasons)
		}
	}
}

func TestHealthRemedyFresh(t *testing.T) {
	f := baseResolvedFacts() // RemoteMetadata Absent, LiveSurface Absent -> fresh
	c := Classify(f)
	if c.State != StateFresh {
		t.Fatalf("fixture did not classify fresh: %+v", c)
	}
	got := EvaluateHealth(c, f, nil)
	if len(got) != 1 {
		t.Fatalf("fresh should yield exactly one finding, got %d: %+v", len(got), got)
	}
	if !strings.Contains(got[0].Remedy, "docket repository init") {
		t.Fatalf("fresh remedy must name `docket repository init`: %q", got[0].Remedy)
	}
	if strings.Contains(got[0].Remedy, "docket repository migrate") {
		t.Fatalf("fresh remedy must NOT name the neighbor command migrate: %q", got[0].Remedy)
	}
}

func TestHealthRemedyLegacy(t *testing.T) {
	f := baseResolvedFacts()
	f.LiveSurface = PresencePresent // -> legacy
	c := Classify(f)
	if c.State != StateLegacy {
		t.Fatalf("fixture did not classify legacy: %+v", c)
	}
	got := EvaluateHealth(c, f, nil)
	if len(got) != 1 {
		t.Fatalf("legacy should yield exactly one finding, got %d: %+v", len(got), got)
	}
	if !strings.Contains(got[0].Remedy, "docket repository migrate") {
		t.Fatalf("legacy remedy must name `docket repository migrate`: %q", got[0].Remedy)
	}
	if strings.Contains(got[0].Remedy, "docket repository init") {
		t.Fatalf("legacy remedy must NOT name the neighbor command init: %q", got[0].Remedy)
	}
}

func TestHealthRemedyNeedsReview(t *testing.T) {
	paths := []string{"docs/changes/active/0001-foo.md", "docs/changes/BOARD.md"}
	c := Classification{State: StateNeedsReview, Reasons: []string{"pending-review-paths"}}
	f := Facts{PendingReviewPaths: paths}
	got := EvaluateHealth(c, f, nil)
	if len(got) != 1 {
		t.Fatalf("needs-review should yield exactly one finding, got %d: %+v", len(got), got)
	}
	for _, p := range paths {
		if !strings.Contains(got[0].Remedy, p) {
			t.Fatalf("needs-review remedy must list the exact pending path %q: %q", p, got[0].Remedy)
		}
	}
	if strings.Contains(got[0].Remedy, "docket repository init") ||
		strings.Contains(got[0].Remedy, "docket repository migrate") {
		t.Fatalf("needs-review remedy must not name a neighbor command: %q", got[0].Remedy)
	}
}

func TestHealthRemedyPartial(t *testing.T) {
	for _, reason := range []string{"metadata-seeded", "metadata-seeded-live-surface", "integration-pruned-attach-incomplete"} {
		c := Classification{State: StatePartial, Reasons: []string{reason}}
		got := EvaluateHealth(c, Facts{}, nil)
		if len(got) != 1 {
			t.Fatalf("partial %q should yield one finding, got %d", reason, len(got))
		}
		if !strings.Contains(got[0].Remedy, "docket repository migrate") {
			t.Fatalf("partial %q remedy must name the safe continuation `docket repository migrate`: %q", reason, got[0].Remedy)
		}
		if strings.Contains(got[0].Remedy, "docket repository init") {
			t.Fatalf("partial %q remedy must NOT name init: %q", reason, got[0].Remedy)
		}
	}
}

func TestHealthRemedyConflictNeverDestructive(t *testing.T) {
	reasons := []string{
		"metadata-root-foreign", "docket-dir-foreign", "metadata-worktree-dirty",
		"local-metadata-diverged", "surfaces-drift", "postconditions-unmet",
	}
	for _, reason := range reasons {
		c := Classification{State: StateConflict, Reasons: []string{reason}}
		got := EvaluateHealth(c, Facts{}, nil)
		if len(got) != 1 {
			t.Fatalf("conflict %q should yield one finding, got %d", reason, len(got))
		}
		r := got[0].Remedy
		assertNoDestructiveCommand(t, r)
		if strings.Contains(r, "docket repository init") || strings.Contains(r, "docket repository migrate") {
			t.Fatalf("conflict %q remedy must not name init/migrate (human disposition only): %q", reason, r)
		}
		if got[0].Severity != SeverityError {
			t.Fatalf("conflict %q should be an error, got %q", reason, got[0].Severity)
		}
	}
}

func TestHealthRemedyDirtyMetadataWorktree(t *testing.T) {
	c := Classification{State: StateConflict, Reasons: []string{"metadata-worktree-dirty"}}
	got := EvaluateHealth(c, Facts{}, nil)
	r := got[0].Remedy
	lr := strings.ToLower(r)
	if !strings.Contains(lr, "commit") || !strings.Contains(lr, "inspect") {
		t.Fatalf("dirty metadata worktree remedy must say commit/inspect: %q", r)
	}
	assertNoDestructiveCommand(t, r)
}

func TestHealthInitCommandAppearsOnlyInFresh(t *testing.T) {
	// The `docket repository init` command must be unique to the fresh remedy;
	// it must not leak into any other state's findings.
	others := []Classification{
		{State: StateLegacy, Reasons: []string{"legacy-live-surface"}},
		{State: StateNeedsReview, Reasons: []string{"pending-review-paths"}},
		{State: StatePartial, Reasons: []string{"metadata-seeded"}},
		{State: StateConflict, Reasons: []string{"metadata-root-foreign", "docket-dir-foreign",
			"metadata-worktree-dirty", "local-metadata-diverged", "surfaces-drift", "postconditions-unmet"}},
		{State: StateUnknown, Reasons: []string{"remote-configured-unknown", "remote-default-unknown",
			"remote-integration-unknown", "metadata-presence-unknown", "live-surface-unknown"}},
	}
	for _, c := range others {
		for _, fnd := range EvaluateHealth(c, Facts{}, nil) {
			if strings.Contains(fnd.Remedy, "docket repository init") {
				t.Fatalf("state %q finding %q leaked the init command: %q", c.State, fnd.Code, fnd.Remedy)
			}
		}
	}
}

func TestHealthDeterministicCategoryOrder(t *testing.T) {
	// A synthetic classification spanning every category, reasons deliberately
	// out of category order, plus frontmatter findings. Output must be
	// remote/topology, integration-tree, local worktree, surface, frontmatter.
	c := Classification{
		State: StateConflict,
		Reasons: []string{
			"surfaces-drift",          // surface (4)
			"metadata-worktree-dirty", // local worktree (3)
			"pending-review-paths",    // integration-tree (2)
			"metadata-root-foreign",   // remote/topology (1)
		},
	}
	fm := []RepairFinding{{Path: "docs/changes/active/0002-x.md", Field: "title", Repairable: true, Code: RepairQuoteScalar, Message: "unsafe scalar"}}
	got := EvaluateHealth(c, Facts{PendingReviewPaths: []string{"p"}}, fm)
	wantCodes := []string{
		"metadata-root-foreign",   // topology
		"pending-review-paths",    // integration
		"metadata-worktree-dirty", // worktree
		"surfaces-drift",          // surface
		string(RepairQuoteScalar), // frontmatter last
	}
	if len(got) != len(wantCodes) {
		t.Fatalf("expected %d findings, got %d: %+v", len(wantCodes), len(got), got)
	}
	for i, code := range wantCodes {
		if got[i].Code != code {
			t.Fatalf("finding %d: want code %q, got %q (full order %v)", i, code, got[i].Code, codesOf(got))
		}
	}
}

func codesOf(fs []Finding) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.Code
	}
	return out
}

func TestHealthFrontmatterFindingCarriesRepairablePointer(t *testing.T) {
	c := Classification{State: StateNeedsReview, Reasons: []string{"pending-review-paths"}}
	tru := RepairFinding{Path: "a.md", Field: "title", Repairable: true, Code: RepairQuoteScalar, Message: "m1"}
	fls := RepairFinding{Path: "b.md", Field: "depends_on", Repairable: false, Message: "m2"}
	got := EvaluateHealth(c, Facts{PendingReviewPaths: []string{"p"}}, []RepairFinding{tru, fls})

	// Non-frontmatter finding must leave Repairable nil.
	nr, ok := findingByCode(got, "pending-review-paths")
	if !ok {
		t.Fatalf("missing pending-review-paths finding")
	}
	if nr.Repairable != nil {
		t.Fatalf("non-frontmatter finding must have nil Repairable, got %v", *nr.Repairable)
	}

	// The two frontmatter findings come last, carrying a non-nil Repairable.
	last := got[len(got)-2:]
	for _, fnd := range last {
		if fnd.Repairable == nil {
			t.Fatalf("frontmatter finding %q must carry a non-nil Repairable pointer", fnd.Code)
		}
	}
	if *last[0].Repairable != true {
		t.Fatalf("first frontmatter finding should be repairable=true")
	}
	if *last[1].Repairable != false {
		t.Fatalf("second frontmatter finding should be repairable=false")
	}
}

func TestCheckExitMatrix(t *testing.T) {
	cases := []struct {
		state State
		want  int
	}{
		{StateHealthy, 0},
		{StateUnknown, 2},
		{StateFresh, 1},
		{StateLegacy, 1},
		{StateNeedsReview, 1},
		{StatePartial, 1},
		{StateConflict, 1},
	}
	for _, tc := range cases {
		c := Classification{State: tc.state}
		if got := CheckExit(c, nil); got != tc.want {
			t.Fatalf("CheckExit(%q) = %d, want %d", tc.state, got, tc.want)
		}
	}
}

func TestCheckExitHealthyWithFindingsIsNotClean(t *testing.T) {
	// Defensive: a healthy classification carrying findings is contradictory;
	// never report the clean 0 while findings stand.
	c := Classification{State: StateHealthy}
	findings := []Finding{{Code: "x", Severity: SeverityError}}
	if got := CheckExit(c, findings); got != 1 {
		t.Fatalf("healthy+findings must not report 0, got %d", got)
	}
}
