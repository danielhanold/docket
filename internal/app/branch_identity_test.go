package app

import (
	"errors"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
)

// branchChange builds a change whose recorded branch: field is set to the given
// (state, value), so recordedBranch reads exactly that field.
func branchChange(branch domain.OptionalString) domain.Change {
	return domain.NewChange(domain.ChangeSpec{
		ID:     7,
		Slug:   "widget",
		Status: domain.StatusInProgress,
		Branch: branch,
	})
}

// TestRecordedBranch tables recordedBranch: a present valid value is returned
// verbatim; an absent or present-empty field is errBranchMissing; a value that
// cannot be a branch ref is errBranchMalformed. It fails closed — the helper
// never reconstructs a branch from the slug or type.
func TestRecordedBranch(t *testing.T) {
	present := func(v string) domain.OptionalString {
		return domain.OptionalString{State: domain.FieldPresent, Value: v}
	}
	tests := []struct {
		name    string
		branch  domain.OptionalString
		want    string
		wantErr error
	}{
		{"present valid", present("feat/widget"), "feat/widget", nil},
		{"present distinct name", present("feature/renamed-head"), "feature/renamed-head", nil},
		{"absent", domain.OptionalString{State: domain.FieldAbsent}, "", errBranchMissing},
		{"present empty", present(""), "", errBranchMissing},
		{"ref-qualified", present("refs/heads/x"), "", errBranchMalformed},
		{"dotdot", present("a..b"), "", errBranchMalformed},
		{"leading dash", present("-lead"), "", errBranchMalformed},
		{"whitespace", present("a b"), "", errBranchMalformed},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := recordedBranch(branchChange(tc.branch))
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("recordedBranch err = %v; want %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("recordedBranch = %q; want %q", got, tc.want)
			}
		})
	}
}
