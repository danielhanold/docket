package githubcli

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// --- shared batch fixtures (untagged so the tagged integration corpus reuses
// them, mirroring fixtures_test.go's change-0333 partition) ---

// batchAliasObj renders one aliased pullRequest value in the EXACT nested shape
// `gh api graphql` returns for the batch selection set: the standard view fields
// plus reviewDecision, mergedAt, and the nested mergeCommit{oid} object. A nil
// decision/mergedAt/mergeCommit is emitted as JSON null (gh's shape), so a
// decoder that keys on the object shape round-trips it. The head/base/title/body
// reuse the ens* constants so a batch alias and a single ViewPullRequest of the
// same PR carry byte-identical version inputs.
func batchAliasObj(number int, state string, decision *string, mergedAt, mergeCommitOID string) map[string]any {
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
	if mergedAt == "" {
		m["mergedAt"] = nil
	} else {
		m["mergedAt"] = mergedAt
	}
	if mergeCommitOID == "" {
		m["mergeCommit"] = nil
	} else {
		m["mergeCommit"] = map[string]any{"oid": mergeCommitOID}
	}
	return m
}

// batchEnvelopeBytes wraps an alias map (keyed "pr0","pr1",… or with a nil value
// for a null alias) into the `{"data":{"repository":{…}},"errors":[…]}` envelope
// gh api graphql emits. A nil errs argument omits the errors key entirely (the
// clean-HTTP-200 shape); a non-nil one attaches it even alongside data.
func batchEnvelopeBytes(t *testing.T, aliases map[string]any, errs []any) []byte {
	t.Helper()
	env := map[string]any{"data": map[string]any{"repository": aliases}}
	if errs != nil {
		env["errors"] = errs
	}
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshaling batch envelope: %v", err)
	}
	return b
}

// batchGraphQLErr renders one GraphQL error object (only its message field is
// read by the decoder).
func batchGraphQLErr(message string) map[string]any { return map[string]any{"message": message} }

// TestBatchQueryAliasesOnePerNumber: N numbers produce N deterministically
// ordered aliases pr0/pr1/… each an exact-number pullRequest(number:) field, in
// input order, with the merge-fact subselection present.
func TestBatchQueryAliasesOnePerNumber(t *testing.T) {
	q := batchGraphQLQuery([]int{101, 205, 309})

	for i, n := range []int{101, 205, 309} {
		want := fmt.Sprintf("pr%d: pullRequest(number: %d)", i, n)
		if !strings.Contains(q, want) {
			t.Errorf("query missing alias %q\nquery: %s", want, q)
		}
	}
	// Deterministic input order: pr0 before pr1 before pr2.
	i0, i1, i2 := strings.Index(q, "pr0:"), strings.Index(q, "pr1:"), strings.Index(q, "pr2:")
	if !(i0 >= 0 && i0 < i1 && i1 < i2) {
		t.Errorf("aliases not in deterministic input order: pr0=%d pr1=%d pr2=%d", i0, i1, i2)
	}
	if strings.Contains(q, "pr3:") {
		t.Errorf("query aliased more numbers than requested:\n%s", q)
	}
	// The typed string variables and the merge-fact subselection must be present.
	for _, frag := range []string{"query($owner:String!,$name:String!)", "repository(owner:$owner,name:$name)", "mergeCommit { oid }", "mergedAt", "reviewDecision"} {
		if !strings.Contains(q, frag) {
			t.Errorf("query missing fragment %q\nquery: %s", frag, q)
		}
	}
}

// TestBatchRejectsEmptyOversizedDuplicateInvalid: 0, 26, a duplicate, and a
// non-positive number are each refused as invalid-input before any process runs.
func TestBatchRejectsEmptyOversizedDuplicateInvalid(t *testing.T) {
	oversized := make([]int, 26)
	for i := range oversized {
		oversized[i] = i + 1
	}
	cases := map[string][]int{
		"empty":     {},
		"oversized": oversized,
		"duplicate": {1, 2, 2},
		"zero":      {1, 0, 3},
		"negative":  {1, -4, 3},
	}
	for name, nums := range cases {
		t.Run(name, func(t *testing.T) {
			f := validateBatchNumbers(nums)
			if f == nil {
				t.Fatalf("validateBatchNumbers(%v) = nil, want invalid-input failure", nums)
			}
			if f.Kind != KindInvalidInput {
				t.Errorf("kind = %v, want KindInvalidInput", f.Kind)
			}
		})
	}
	// A clean 25-number set is accepted.
	ok := make([]int, 25)
	for i := range ok {
		ok[i] = i + 1
	}
	if f := validateBatchNumbers(ok); f != nil {
		t.Fatalf("25 unique positive numbers rejected: %v", f)
	}
}

// TestBatchDecodeMergedOpenClosedDraftApproved: each alias normalizes exactly as
// a single ViewPullRequest would — state mapping, draft, approval, and merged
// facts.
func TestBatchDecodeMergedOpenClosedDraftApproved(t *testing.T) {
	numbers := []int{10, 20, 30, 40}
	mergeOID := "3333333333333333333333333333333333333333"
	aliases := map[string]any{
		"pr0": batchAliasObj(10, "MERGED", nil, "2026-08-31T12:00:00Z", mergeOID),
		"pr1": batchAliasObj(20, "OPEN", strPtr("APPROVED"), "", ""),
		"pr2": batchAliasObj(30, "CLOSED", nil, "", ""),
		"pr3": func() map[string]any {
			m := batchAliasObj(40, "OPEN", nil, "", "")
			m["isDraft"] = true
			return m
		}(),
	}
	got, f := decodeBatchResponse(batchOp, numbers, batchEnvelopeBytes(t, aliases, nil))
	if f != nil {
		t.Fatalf("decodeBatchResponse: %v", f)
	}

	if r := got[10]; !r.Found || r.PR.State != StateMerged || r.MergedAtUTC != "2026-08-31T12:00:00Z" || r.MergeCommit != mergeOID {
		t.Errorf("pr 10 (merged) = %+v", r)
	}
	if r := got[20]; !r.Found || r.PR.State != StateOpen || !r.PR.Approved {
		t.Errorf("pr 20 (open approved) = %+v", r)
	}
	if r := got[30]; !r.Found || r.PR.State != StateClosed {
		t.Errorf("pr 30 (closed) = %+v", r)
	}
	if r := got[40]; !r.Found || !r.PR.Draft {
		t.Errorf("pr 40 (draft) = %+v", r)
	}
	// A non-merged result never carries merge facts.
	if got[20].MergedAtUTC != "" || got[20].MergeCommit != "" {
		t.Errorf("open PR carried merge facts: %+v", got[20])
	}
}

// TestBatchNullAliasIsUnknownNeverClosed: a null aliased value is UNKNOWN
// (Found=false), never a closed/absent verdict.
func TestBatchNullAliasIsUnknownNeverClosed(t *testing.T) {
	numbers := []int{10, 20}
	aliases := map[string]any{
		"pr0": nil, // JSON null
		"pr1": batchAliasObj(20, "OPEN", nil, "", ""),
	}
	got, f := decodeBatchResponse(batchOp, numbers, batchEnvelopeBytes(t, aliases, nil))
	if f != nil {
		t.Fatalf("decodeBatchResponse: %v", f)
	}
	r := got[10]
	if r.Found {
		t.Errorf("null alias reported Found=true: %+v", r)
	}
	if r.PR.State == StateClosed {
		t.Errorf("null alias decoded to a closed verdict: %+v", r)
	}
	if r.PR.State != "" {
		t.Errorf("null alias carried a state: %+v", r)
	}
	if !got[20].Found {
		t.Errorf("sibling PR should still decode: %+v", got[20])
	}
}

// TestBatchWrongNumberInAliasIsUnknown: a decoded PR whose number differs from
// the requested alias number is UNKNOWN, never trusted.
func TestBatchWrongNumberInAliasIsUnknown(t *testing.T) {
	numbers := []int{10}
	aliases := map[string]any{
		"pr0": batchAliasObj(99, "OPEN", nil, "", ""), // requested 10, server said 99
	}
	got, f := decodeBatchResponse(batchOp, numbers, batchEnvelopeBytes(t, aliases, nil))
	if f != nil {
		t.Fatalf("decodeBatchResponse: %v", f)
	}
	if got[10].Found {
		t.Errorf("wrong-number alias reported Found=true: %+v", got[10])
	}
}

// TestBatchMalformedFieldIsUnknownForThatPROnly: a malformed required field
// makes THAT PR unknown while its siblings decode — the batch is not failed.
func TestBatchMalformedFieldIsUnknownForThatPROnly(t *testing.T) {
	numbers := []int{10, 20}
	bad := batchAliasObj(10, "OPEN", nil, "", "")
	bad["headRefOid"] = "nothex" // invalid full object id
	aliases := map[string]any{
		"pr0": bad,
		"pr1": batchAliasObj(20, "OPEN", nil, "", ""),
	}
	got, f := decodeBatchResponse(batchOp, numbers, batchEnvelopeBytes(t, aliases, nil))
	if f != nil {
		t.Fatalf("malformed field failed the whole batch, want per-PR unknown: %v", f)
	}
	if got[10].Found {
		t.Errorf("malformed PR reported Found=true: %+v", got[10])
	}
	if !got[20].Found {
		t.Errorf("sibling PR should still decode: %+v", got[20])
	}
}

// TestBatchGraphQLErrorsFailWholeBatchEvenHTTP200: a non-empty top-level errors
// array fails the whole batch as a typed *Failure even when data is present.
func TestBatchGraphQLErrorsFailWholeBatchEvenHTTP200(t *testing.T) {
	numbers := []int{10, 20}
	aliases := map[string]any{
		"pr0": batchAliasObj(10, "OPEN", nil, "", ""),
		"pr1": nil,
	}
	errs := []any{batchGraphQLErr("Something is wrong with the batch")}
	got, f := decodeBatchResponse(batchOp, numbers, batchEnvelopeBytes(t, aliases, errs))
	if f == nil {
		t.Fatalf("errors array tolerated; want a whole-batch failure (got %+v)", got)
	}
	if got != nil {
		t.Errorf("failed batch returned a non-nil map: %+v", got)
	}
}

// TestBatchVersionMatchesSingleViewFixture: a batch alias and a single
// ViewPullRequest decode of the same snapshot produce a byte-identical version —
// the batch reuses the package's one normalization, never a second.
func TestBatchVersionMatchesSingleViewFixture(t *testing.T) {
	single, err := decodePullRequest(probeOp, []byte(probePRJSONWithDecision(7, "OPEN", strPtr("APPROVED"))))
	if err != nil {
		t.Fatalf("decode single view: %v", err)
	}
	aliases := map[string]any{"pr0": batchAliasObj(7, "OPEN", strPtr("APPROVED"), "", "")}
	got, f := decodeBatchResponse(batchOp, []int{7}, batchEnvelopeBytes(t, aliases, nil))
	if f != nil {
		t.Fatalf("decodeBatchResponse: %v", f)
	}
	if !got[7].Found {
		t.Fatalf("pr 7 not found: %+v", got[7])
	}
	if got[7].PR.Version != single.Version {
		t.Errorf("version mismatch:\n batch  %s\n single %s", got[7].PR.Version, single.Version)
	}
	if got[7].PR.Approved != single.Approved {
		t.Errorf("approval mismatch: batch=%v single=%v", got[7].PR.Approved, single.Approved)
	}
}
