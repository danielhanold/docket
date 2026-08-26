package domain

import "testing"

// branchShapeEquivFixture is the equivalence contract between the two copies of
// the branch-shape rule: this domain layer's malformedBranchRef (decides the
// finalize branch-malformed skip token) and the app layer's recordedBranch
// (decides errBranchMalformed). The layering forces the duplication — domain
// cannot import app — so this fixture is asserted against BOTH real functions,
// once here over malformedBranchRef and once in the sibling app-package test
// over recordedBranch, keeping the SAME literal rows. If one copy's shape rule
// drifts from the other, the corresponding assertion reddens.
//
// Every value is NON-EMPTY on purpose: the empty/absent axis is recordedBranch's
// errBranchMissing, which malformedBranchRef does not model, so feeding an empty
// value would confound the malformed axis this fixture pins. Keep this slice
// byte-identical to branchShapeEquivFixture in internal/app.
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

// TestMalformedBranchRefAgainstEquivFixture asserts malformedBranchRef's verdict
// for every row of the shared equivalence fixture. Its twin,
// TestRecordedBranchAgainstEquivFixture in internal/app, asserts recordedBranch
// over the same rows; together they pin that the two duplicated shape rules
// agree, so an unmirrored edit to either copy reddens.
func TestMalformedBranchRefAgainstEquivFixture(t *testing.T) {
	for _, tc := range branchShapeEquivFixture {
		if got := malformedBranchRef(tc.value); got != tc.malformed {
			t.Errorf("malformedBranchRef(%q) = %v; want %v", tc.value, got, tc.malformed)
		}
	}
}
