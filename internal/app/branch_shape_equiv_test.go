package app

import (
	"errors"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
)

// branchShapeEquivFixture is the equivalence contract between the two copies of
// the branch-shape rule: this app layer's recordedBranch (decides
// errBranchMalformed) and the domain layer's malformedBranchRef (decides the
// finalize branch-malformed skip token). The layering forces the duplication —
// domain cannot import app — so this fixture is asserted against BOTH real
// functions, once here over recordedBranch and once in the domain-package test
// over malformedBranchRef, keeping the SAME literal rows. If one copy's shape
// rule drifts from the other, the corresponding assertion reddens.
//
// Every value is NON-EMPTY on purpose: the empty/absent axis is recordedBranch's
// errBranchMissing, which malformedBranchRef does not model, so feeding an empty
// value would confound the malformed axis this fixture pins. Keep this slice
// byte-identical to branchShapeEquivFixture in internal/domain.
var branchShapeEquivFixture = []struct {
	value     string
	malformed bool
}{
	// Malformed: cannot be a plain feature-branch ref.
	{"refs/heads/x", true}, // refs/ prefix
	{"refs/x", true},       // refs/ prefix
	{"-lead", true},        // leading dash
	{"a b", true},          // space
	{"a\tb", true},         // tab
	{"a@{b", true},         // @{ sequence
	{"a..b", true},         // .. sequence
	{"a\x00b", true},       // embedded NUL

	// Accepted: valid plain feature-branch refs.
	{"feat/foo", false},
	{"feature/renamed-head", false},
	{"hotfix/x", false},
	{"release-2/y", false},
}

// TestRecordedBranchAgainstEquivFixture asserts recordedBranch's malformed vs
// accepted verdict for every row of the shared equivalence fixture: a malformed
// row is errBranchMalformed, an accepted row returns the value verbatim with a
// nil error. Its twin, TestMalformedBranchRefAgainstEquivFixture in
// internal/domain, asserts malformedBranchRef over the same rows; together they
// pin that the two duplicated shape rules agree, so an unmirrored edit to either
// copy reddens. Every fixture value is non-empty, so errBranchMissing never
// confounds the malformed axis.
func TestRecordedBranchAgainstEquivFixture(t *testing.T) {
	for _, tc := range branchShapeEquivFixture {
		got, err := recordedBranch(branchChange(
			domain.OptionalString{State: domain.FieldPresent, Value: tc.value}))
		if tc.malformed {
			if !errors.Is(err, errBranchMalformed) {
				t.Errorf("recordedBranch(%q) err = %v; want errBranchMalformed", tc.value, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("recordedBranch(%q) err = %v; want nil", tc.value, err)
			continue
		}
		if got != tc.value {
			t.Errorf("recordedBranch(%q) = %q; want %q", tc.value, got, tc.value)
		}
	}
}
