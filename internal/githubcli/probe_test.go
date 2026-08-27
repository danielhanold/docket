package githubcli

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// strPtr returns a pointer to s, for nullable fixture fields.
func strPtr(s string) *string { return &s }

// probePRJSONWithDecision renders one PR view object in gh's nested shape with
// an explicit reviewDecision: a string value, or JSON null when decision is nil.
// ensPRJSON deliberately stays decision-free — it feeds the standard-field
// list/create/edit tests, whose absent-field decode this change must preserve.
//
// This helper (and strPtr) stay UNTAGGED — change 0333's integration partition —
// because the fast review-decision tests below consume them; the tagged
// ViewPullRequest subprocess tests (probe_integration_test.go) share them too,
// which an untagged declaration keeps visible to the tagged build.
func probePRJSONWithDecision(number int, state string, decision *string) string {
	m := map[string]any{
		"number":      number,
		"url":         fmt.Sprintf("https://github.com/acme/widget/pull/%d", number),
		"state":       state,
		"isDraft":     false,
		"headRefName": ensHead,
		"headRefOid":  ensHeadOid,
		"baseRefName": ensBase,
		"title":       ensTitle,
		"body":        ensBody,
	}
	if decision == nil {
		m["reviewDecision"] = nil
	} else {
		m["reviewDecision"] = *decision
	}
	b, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// TestVersionExcludesReviewDecision: the write-CAS token must not depend on
// review state — the same PR yields one token whether it arrived approved via
// the exact view or decision-free via a standard read. The Approved inequality
// assert keeps the fixture honest: if both documents decoded to the same
// Approved, equal versions would prove nothing.
func TestVersionExcludesReviewDecision(t *testing.T) {
	approved, err := decodePullRequest("probe", []byte(probePRJSONWithDecision(7, "OPEN", strPtr("APPROVED"))))
	if err != nil {
		t.Fatalf("decode approved: %v", err)
	}
	plain, err := decodePullRequest("probe", []byte(probePRJSONWithDecision(7, "OPEN", nil)))
	if err != nil {
		t.Fatalf("decode plain: %v", err)
	}
	if approved.Approved == plain.Approved {
		t.Fatalf("fixture vacuous: both documents decode to Approved=%v", approved.Approved)
	}
	if approved.Version != plain.Version {
		t.Errorf("Version differs on review state alone:\n approved %s\n plain    %s", approved.Version, plain.Version)
	}
}

// TestStandardFieldSetExcludesReviewDecision: only the exact-number view widens.
// The standard list/create/edit set must not gain review state, and the view
// set must be exactly the standard set plus reviewDecision.
func TestStandardFieldSetExcludesReviewDecision(t *testing.T) {
	if strings.Contains(prJSONFields, "reviewDecision") {
		t.Fatalf("prJSONFields gained reviewDecision; only ViewPullRequest requests review state")
	}
	if prViewJSONFields != prJSONFields+",reviewDecision" {
		t.Fatalf("prViewJSONFields = %q, want prJSONFields+%q", prViewJSONFields, ",reviewDecision")
	}
}
