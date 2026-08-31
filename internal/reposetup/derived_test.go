package reposetup

import "testing"

// TestDerivedFindingLiftRepairable proves a repairable derived-view difference
// lifts to a warning Finding that carries the code, the file ref, a non-nil
// repairable:true flag, and a remedy naming migrate.
func TestDerivedFindingLiftRepairable(t *testing.T) {
	df := DerivedFinding{
		View:       DerivedViewBoard,
		Code:       CodeBoardStale,
		Path:       "docs/changes/BOARD.md",
		Repairable: true,
		Message:    "board bytes differ from the canonical render",
	}
	f := df.Finding()
	if f.Code != CodeBoardStale {
		t.Errorf("Code = %q, want %q", f.Code, CodeBoardStale)
	}
	if f.Severity != SeverityWarning {
		t.Errorf("Severity = %q, want warning", f.Severity)
	}
	if f.Ref != "docs/changes/BOARD.md" {
		t.Errorf("Ref = %q, want the file path", f.Ref)
	}
	if f.Repairable == nil || !*f.Repairable {
		t.Errorf("Repairable = %v, want a non-nil true", f.Repairable)
	}
}

// TestDerivedFindingLiftMalformed proves a non-repairable derived-view finding
// (a malformed managed marker) lifts to an error Finding whose repairable flag
// is a non-nil false.
func TestDerivedFindingLiftMalformed(t *testing.T) {
	df := DerivedFinding{
		View:       DerivedViewArtifactLinks,
		Code:       CodeArtifactLinksMalformed,
		Path:       "docs/changes/active/0001-x.md",
		Repairable: false,
		Message:    "unbalanced artifact-links markers",
	}
	f := df.Finding()
	if f.Severity != SeverityError {
		t.Errorf("Severity = %q, want error", f.Severity)
	}
	if f.Repairable == nil || *f.Repairable {
		t.Errorf("Repairable = %v, want a non-nil false", f.Repairable)
	}
}
