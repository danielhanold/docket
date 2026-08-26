package githubcli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func probeRepo() Repository { return Repository{Host: "github.com", Owner: "acme", Name: "widget"} }

// TestFindOpenPullRequestsByHeadReturnsOpen: the read-only probe lists the open
// PRs for the exact head branch, decodes them, and NEVER mutates — no create or
// edit invocation is witnessed.
func TestFindOpenPullRequestsByHeadReturnsOpen(t *testing.T) {
	c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
		ensListOpenArm(ensList(ensMatchPR(42))),
	}})

	got, err := c.FindOpenPullRequestsByHead(context.Background(), probeRepo(), ensHead)
	if err != nil {
		t.Fatalf("FindOpenPullRequestsByHead: %v", err)
	}
	if len(got) != 1 || got[0].Number != 42 || got[0].State != StateOpen || got[0].HeadCommit != ensHeadOid {
		t.Fatalf("probe result mismatch: %+v", got)
	}

	recs := log.records(t)
	if n := countArgv(recs, "pr", "create"); n != 0 {
		t.Errorf("probe issued %d pr create invocations, want 0 (read-only)", n)
	}
	if n := countArgv(recs, "pr", "edit"); n != 0 {
		t.Errorf("probe issued %d pr edit invocations, want 0 (read-only)", n)
	}
	if n := countArgv(recs, "pr", "list", "--repo", ensRepoSpec, "--head", ensHead, "--state", "open"); n != 1 {
		t.Errorf("probe list count = %d, want exactly 1", n)
	}
}

// TestFindOpenPullRequestsByHeadDropsTerminal: a terminal-state PR in the
// response is filtered out — only open PRs are returned.
func TestFindOpenPullRequestsByHeadDropsTerminal(t *testing.T) {
	closed := ensPRJSON(7, "CLOSED", false, ensHead, ensOtherOid, ensBase, ensTitle, ensBody)
	c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
		ensListOpenArm(ensList(ensMatchPR(42), closed)),
	}})

	got, err := c.FindOpenPullRequestsByHead(context.Background(), probeRepo(), ensHead)
	if err != nil {
		t.Fatalf("FindOpenPullRequestsByHead: %v", err)
	}
	if len(got) != 1 || got[0].Number != 42 {
		t.Fatalf("terminal PR not filtered: %+v", got)
	}
}

// TestFindOpenPullRequestsByHeadErrorNotAbsence: a non-zero exit is a typed
// external failure, never an empty "no PR" result (probe-error-is-not-clean-absence).
func TestFindOpenPullRequestsByHeadErrorNotAbsence(t *testing.T) {
	c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
		{ArgvPrefix: []string{"pr", "list", "--repo", ensRepoSpec, "--head", ensHead, "--state", "open"}, Exit: 1, Stderr: "boom"},
	}})

	got, err := c.FindOpenPullRequestsByHead(context.Background(), probeRepo(), ensHead)
	if err == nil {
		t.Fatalf("want a typed failure on non-zero exit, got nil (result %+v)", got)
	}
	if got != nil {
		t.Errorf("errored probe returned %v, want nil slice", got)
	}
	if f, ok := AsFailure(err); !ok || f.Kind != KindExternal {
		t.Errorf("failure = %v, want an external-kind Failure", err)
	}
}

// TestViewPullRequestByNumber: the exact-number read decodes the full normalized
// snapshot — number, open state, and the head branch verbatim from headRefName.
func TestViewPullRequestByNumber(t *testing.T) {
	doc := ensPRJSON(7, "OPEN", false, "feature/renamed-head", ensHeadOid, ensBase, ensTitle, ensBody)
	c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
		{ArgvPrefix: []string{"pr", "view", "7", "--repo", ensRepoSpec, "--json"}, Stdout: doc, Exit: 0},
	}})

	pr, err := c.ViewPullRequest(context.Background(), probeRepo(), 7)
	if err != nil {
		t.Fatalf("ViewPullRequest: %v", err)
	}
	if pr.Number != 7 {
		t.Errorf("Number = %d, want 7", pr.Number)
	}
	if pr.State != StateOpen {
		t.Errorf("State = %q, want %q", pr.State, StateOpen)
	}
	if pr.HeadBranch != "feature/renamed-head" {
		t.Errorf("HeadBranch = %q, want %q", pr.HeadBranch, "feature/renamed-head")
	}
	if pr.Approved {
		t.Errorf("Approved = true for a view response with no reviewDecision field; absent must read false")
	}
}

// TestViewPullRequestMergedState: a MERGED PR decodes to StateMerged with no
// error — a terminal state is a clean read, not a failure.
func TestViewPullRequestMergedState(t *testing.T) {
	doc := ensPRJSON(7, "MERGED", false, ensHead, ensHeadOid, ensBase, ensTitle, ensBody)
	c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
		{ArgvPrefix: []string{"pr", "view", "7", "--repo", ensRepoSpec, "--json"}, Stdout: doc, Exit: 0},
	}})

	pr, err := c.ViewPullRequest(context.Background(), probeRepo(), 7)
	if err != nil {
		t.Fatalf("ViewPullRequest: %v", err)
	}
	if pr.State != StateMerged {
		t.Fatalf("State = %q, want %q", pr.State, StateMerged)
	}
}

// TestViewPullRequestErrorIsNotAbsence: a non-zero exit and an undecodable JSON
// document are each a returned error carrying the zero PR — never a zero-value
// snapshot read as truth (probe-error-is-not-clean-absence).
func TestViewPullRequestErrorIsNotAbsence(t *testing.T) {
	t.Run("non-zero-exit", func(t *testing.T) {
		c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
			{ArgvPrefix: []string{"pr", "view", "7", "--repo", ensRepoSpec, "--json"}, Exit: 1, Stderr: "boom"},
		}})
		pr, err := c.ViewPullRequest(context.Background(), probeRepo(), 7)
		if err == nil {
			t.Fatalf("want a typed failure on non-zero exit, got nil (pr %+v)", pr)
		}
		if pr != (PullRequest{}) {
			t.Errorf("errored read returned %+v, want the zero PR", pr)
		}
	})
	t.Run("undecodable-json", func(t *testing.T) {
		c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
			{ArgvPrefix: []string{"pr", "view", "7", "--repo", ensRepoSpec, "--json"}, Stdout: "{ not json", Exit: 0},
		}})
		pr, err := c.ViewPullRequest(context.Background(), probeRepo(), 7)
		if err == nil {
			t.Fatalf("want an error on undecodable JSON, got nil (pr %+v)", pr)
		}
		if pr != (PullRequest{}) {
			t.Errorf("errored read returned %+v, want the zero PR", pr)
		}
	})
}

// TestViewPullRequestRejectsNonPositive: a non-positive number is invalid input
// refused before any gh process runs.
func TestViewPullRequestRejectsNonPositive(t *testing.T) {
	c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{}})

	if _, err := c.ViewPullRequest(context.Background(), probeRepo(), 0); err == nil {
		t.Errorf("number 0: want a validation failure, got nil")
	}
	if recs := log.records(t); len(recs) != 0 {
		t.Errorf("a rejected read issued %d gh invocations, want 0", len(recs))
	}
}

// TestFindOpenPullRequestsByHeadRejectsBadInput: an invalid repository identity
// or empty head branch is refused before any invocation.
func TestFindOpenPullRequestsByHeadRejectsBadInput(t *testing.T) {
	c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{}})

	if _, err := c.FindOpenPullRequestsByHead(context.Background(), probeRepo(), ""); err == nil {
		t.Errorf("empty head branch: want a validation failure, got nil")
	}
	if _, err := c.FindOpenPullRequestsByHead(context.Background(), Repository{}, ensHead); err == nil {
		t.Errorf("invalid repository: want a validation failure, got nil")
	}
	if recs := log.records(t); len(recs) != 0 {
		t.Errorf("a rejected probe issued %d gh invocations, want 0", len(recs))
	}
}

// strPtr returns a pointer to s, for nullable fixture fields.
func strPtr(s string) *string { return &s }

// probePRJSONWithDecision renders one PR view object in gh's nested shape with
// an explicit reviewDecision: a string value, or JSON null when decision is nil.
// ensPRJSON deliberately stays decision-free — it feeds the standard-field
// list/create/edit tests, whose absent-field decode this change must preserve.
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

// TestViewPullRequestRequestsReviewDecision pins the exact --json field set the
// exact-number view sends, as ONE LITERAL STRING. Matching against the
// prViewJSONFields constant instead would stay green if the constant silently
// lost the field (defaulted-param-hides-caller-wiring); the fake answers only a
// matching argv, so a view that requests anything else errors here.
func TestViewPullRequestRequestsReviewDecision(t *testing.T) {
	doc := probePRJSONWithDecision(7, "OPEN", strPtr("APPROVED"))
	c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
		{ArgvPrefix: []string{"pr", "view", "7", "--repo", ensRepoSpec, "--json",
			"number,url,state,isDraft,headRefName,headRefOid,baseRefName,title,body,reviewDecision"}, Stdout: doc, Exit: 0},
	}})
	pr, err := c.ViewPullRequest(context.Background(), probeRepo(), 7)
	if err != nil {
		t.Fatalf("ViewPullRequest: %v", err)
	}
	if !pr.Approved {
		t.Errorf("Approved = false, want true for reviewDecision APPROVED")
	}
}

// TestViewPullRequestReviewDecisionMapping: the strict mapping — only APPROVED
// is true; REVIEW_REQUIRED, CHANGES_REQUESTED, and JSON null are false.
func TestViewPullRequestReviewDecisionMapping(t *testing.T) {
	cases := []struct {
		name     string
		decision *string
		want     bool
	}{
		{"approved", strPtr("APPROVED"), true},
		{"review-required", strPtr("REVIEW_REQUIRED"), false},
		{"changes-requested", strPtr("CHANGES_REQUESTED"), false},
		{"null", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := probePRJSONWithDecision(7, "OPEN", tc.decision)
			c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
				{ArgvPrefix: []string{"pr", "view", "7"}, Stdout: doc, Exit: 0},
			}})
			pr, err := c.ViewPullRequest(context.Background(), probeRepo(), 7)
			if err != nil {
				t.Fatalf("ViewPullRequest: %v", err)
			}
			if pr.Approved != tc.want {
				t.Errorf("Approved = %v, want %v", pr.Approved, tc.want)
			}
		})
	}
}

// TestViewPullRequestEmptyReviewDecisionIsNoDecision: GitHub returns
// reviewDecision "" — an empty string, not JSON null — for a repository whose
// branch protection requires a PR but zero approvals. Empty string means the
// same thing as null: no required-review decision, so Approved is false and
// decode succeeds. Complements TestViewPullRequestUnknownReviewDecisionFailsClosed,
// which keeps genuinely unknown non-empty vocabulary failing closed.
func TestViewPullRequestEmptyReviewDecisionIsNoDecision(t *testing.T) {
	doc := probePRJSONWithDecision(7, "OPEN", strPtr(""))
	c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
		{ArgvPrefix: []string{"pr", "view", "7"}, Stdout: doc, Exit: 0},
	}})
	pr, err := c.ViewPullRequest(context.Background(), probeRepo(), 7)
	if err != nil {
		t.Fatalf("empty-string reviewDecision errored; want no-decision success: %v", err)
	}
	if pr.Approved {
		t.Errorf("Approved = true, want false for empty-string reviewDecision")
	}
}

// TestViewPullRequestUnknownReviewDecisionFailsClosed: unknown non-null
// vocabulary is invalid external state — a typed invalid-state Failure and the
// zero PR, never a silently chosen boolean.
func TestViewPullRequestUnknownReviewDecisionFailsClosed(t *testing.T) {
	doc := probePRJSONWithDecision(7, "OPEN", strPtr("DISMISSED"))
	c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
		{ArgvPrefix: []string{"pr", "view", "7"}, Stdout: doc, Exit: 0},
	}})
	pr, err := c.ViewPullRequest(context.Background(), probeRepo(), 7)
	if err == nil {
		t.Fatalf("unknown reviewDecision decoded cleanly; want typed invalid-state failure")
	}
	f, ok := AsFailure(err)
	if !ok {
		t.Fatalf("error is not a typed *Failure: %v", err)
	}
	if f.Kind != KindInvalidState {
		t.Errorf("Kind = %v, want KindInvalidState", f.Kind)
	}
	if pr != (PullRequest{}) {
		t.Errorf("returned PR is not the zero value alongside the error")
	}
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
