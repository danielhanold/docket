//go:build integration

package githubcli

import (
	"context"
	"testing"
)

// The merge-method and branch-rule probes drive real fake-gh `gh api` subprocesses
// (change 0333's integration partition), so they ride the TestIntegrationMerge
// shard with the rest of the merge/retarget behavior. The pure method-set algebra
// (selectMergeMethod, methodSet) stays in the untagged mergemethod_test.go.

// TestIntegrationMergeProbeRepoMergeMethods: the three booleans decode explicitly
// from gh's repository endpoint; the argv is exact (api, --hostname, repos/o/n).
func TestIntegrationMergeProbeRepoMergeMethods(t *testing.T) {
	c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{{
		ArgvPrefix: []string{"api", "--hostname", "github.com", "repos/octo/widgets"},
		Stdout:     `{"allow_rebase_merge":false,"allow_merge_commit":true,"allow_squash_merge":true}`,
	}}})
	set, f := c.probeRepoMergeMethods(context.Background(), Repository{Host: "github.com", Owner: "octo", Name: "widgets"})
	if f != nil {
		t.Fatalf("unexpected failure: %v", f)
	}
	if set != (methodSet{merge: true, squash: true}) {
		t.Fatalf("decoded set: %+v", set)
	}
}

// TestIntegrationMergeProbeRepoMergeMethodsFailsClosed: a missing boolean,
// malformed JSON, or a non-zero gh exit is a typed Failure — never a permissive
// default set.
func TestIntegrationMergeProbeRepoMergeMethodsFailsClosed(t *testing.T) {
	cases := []struct {
		name string
		arm  fakeArm
	}{
		{"missing field", fakeArm{ArgvPrefix: []string{"api"}, Stdout: `{"allow_rebase_merge":true,"allow_squash_merge":true}`}},
		{"malformed json", fakeArm{ArgvPrefix: []string{"api"}, Stdout: `{"allow_rebase`}},
		{"http failure", fakeArm{ArgvPrefix: []string{"api"}, Exit: 1, Stderr: "gh: Not Found (HTTP 404)"}},
	}
	for _, cse := range cases {
		c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{cse.arm}})
		set, f := c.probeRepoMergeMethods(context.Background(), Repository{Host: "github.com", Owner: "o", Name: "n"})
		if f == nil || set != (methodSet{}) {
			t.Errorf("%s: must fail closed, got set=%+v f=%v", cse.name, set, f)
		}
	}
}

// TestIntegrationMergeProbeBranchMergeRules: rules restrict by intersection; linear
// history removes merge; no method-specific rule contributes no restriction; the
// base branch is path-escaped so "feat/parent" is ONE endpoint segment.
func TestIntegrationMergeProbeBranchMergeRules(t *testing.T) {
	rules := `[
		{"type":"pull_request","parameters":{"allowed_merge_methods":["merge","squash"]}},
		{"type":"pull_request","parameters":{"allowed_merge_methods":["squash","rebase"]}},
		{"type":"required_linear_history","parameters":{}}
	]`
	c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{{
		ArgvPrefix: []string{"api", "--hostname", "github.com", "repos/octo/widgets/rules/branches/feat%2Fparent"},
		Stdout:     rules,
	}}})
	set, unavailable, f := c.probeBranchMergeRules(context.Background(), Repository{Host: "github.com", Owner: "octo", Name: "widgets"}, "feat/parent")
	if f != nil {
		t.Fatalf("unexpected failure: %v", f)
	}
	if unavailable {
		t.Fatal("a readable rules array must not report rules-unavailable")
	}
	// intersection of the two allowed_merge_methods rules is {squash}; linear
	// history removes merge (already absent). rebase excluded by the first rule.
	if set != (methodSet{squash: true}) {
		t.Fatalf("composed branch set: %+v", set)
	}
}

// TestIntegrationMergeProbeBranchMergeRulesNoRestriction: an empty rules array (or
// rules of unrelated types, or a pull_request rule without allowed_merge_methods)
// contributes no restriction — all three methods stay permitted.
func TestIntegrationMergeProbeBranchMergeRulesNoRestriction(t *testing.T) {
	for name, body := range map[string]string{
		"empty array":    `[]`,
		"unrelated rule": `[{"type":"deletion","parameters":{}}]`,
		"pr rule no key": `[{"type":"pull_request","parameters":{"required_approving_review_count":0}}]`,
	} {
		c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{{ArgvPrefix: []string{"api"}, Stdout: body}}})
		set, unavailable, f := c.probeBranchMergeRules(context.Background(), Repository{Host: "github.com", Owner: "o", Name: "n"}, "main")
		if f != nil || unavailable || set != (methodSet{rebase: true, merge: true, squash: true}) {
			t.Errorf("%s: want unrestricted set, got %+v f=%v", name, set, f)
		}
	}
}

// TestIntegrationMergeProbeBranchMergeRulesFailsClosed: malformed payloads, an
// EMPTY allowed_merge_methods array, an unknown token, and a failed request are all
// unobservable/invalid policy — a typed Failure, never a guess.
func TestIntegrationMergeProbeBranchMergeRulesFailsClosed(t *testing.T) {
	cases := map[string]fakeArm{
		"malformed json": {ArgvPrefix: []string{"api"}, Stdout: `{"not":"an array"}`},
		"empty methods":  {ArgvPrefix: []string{"api"}, Stdout: `[{"type":"pull_request","parameters":{"allowed_merge_methods":[]}}]`},
		"unknown token":  {ArgvPrefix: []string{"api"}, Stdout: `[{"type":"pull_request","parameters":{"allowed_merge_methods":["MERGE"]}}]`},
		"http failure":   {ArgvPrefix: []string{"api"}, Exit: 1, Stderr: "gh: Server Error (HTTP 500)"},
	}
	for name, arm := range cases {
		c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{arm}})
		if _, _, f := c.probeBranchMergeRules(context.Background(), Repository{Host: "github.com", Owner: "o", Name: "n"}, "main"); f == nil {
			t.Errorf("%s: must fail closed", name)
		}
	}
}

// planGateBody is GitHub's literal answer to a branch-rules read on a private
// repository whose plan has no branch rules (reproduced 2026-10-05 against the
// 0366 fixture repository). `gh api` prints it on stdout and exits 1.
const planGateBody = `{"message":"Upgrade to GitHub Pro or make this repository public to enable this feature.","documentation_url":"https://docs.github.com/rest/repos/rules#get-rules-for-a-branch","status":"403"}`

// TestIntegrationMergeProbeBranchMergeRulesPlanGate: GitHub's plan-gate 403 means
// the branch has no rules — every method is permitted and the probe reports
// rules-unavailable with no failure. The Team wording matches too.
func TestIntegrationMergeProbeBranchMergeRulesPlanGate(t *testing.T) {
	bodies := map[string]string{
		"pro":  planGateBody,
		"team": `{"message":"Upgrade to GitHub Team or make this repository public to enable this feature.","status":"403"}`,
	}
	for name, body := range bodies {
		c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{{
			ArgvPrefix: []string{"api"}, Stdout: body, Exit: 1,
			Stderr: "gh: Upgrade to GitHub Pro or make this repository public to enable this feature. (HTTP 403)",
		}}})
		set, unavailable, f := c.probeBranchMergeRules(context.Background(), Repository{Host: "github.com", Owner: "o", Name: "n"}, "main")
		if f != nil {
			t.Fatalf("%s: plan-gate answer must not be a failure, got %v", name, f)
		}
		if !unavailable {
			t.Fatalf("%s: plan-gate answer must report rules-unavailable", name)
		}
		if set != (methodSet{rebase: true, merge: true, squash: true}) {
			t.Fatalf("%s: plan-gate answer must permit every method, got %+v", name, set)
		}
	}
}

// TestIntegrationMergeProbeBranchMergeRulesPlanGateLookalikesFailClosed: only the
// exact plan-gate answer proceeds. Every lookalike stays a typed failure with an
// empty set and rules-unavailable false.
func TestIntegrationMergeProbeBranchMergeRulesPlanGateLookalikesFailClosed(t *testing.T) {
	cases := map[string]string{
		"other 403 message":         `{"message":"Resource not accessible by integration","status":"403"}`,
		"404 body":                  `{"message":"Not Found","documentation_url":"https://docs.github.com/rest","status":"404"}`,
		"non-JSON stdout":           `Upgrade to GitHub Pro or make this repository public to enable this feature.`,
		"plan-gate text, 404":       `{"message":"Upgrade to GitHub Pro or make this repository public to enable this feature.","status":"404"}`,
		"plan-gate text, numeric":   `{"message":"Upgrade to GitHub Pro or make this repository public to enable this feature.","status":403}`,
		"plan-gate text, no status": `{"message":"Upgrade to GitHub Pro or make this repository public to enable this feature."}`,
		"empty plan name":           `{"message":"Upgrade to GitHub  or make this repository public to enable this feature.","status":"403"}`,
		"empty stdout":              ``,
	}
	for name, body := range cases {
		c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{{ArgvPrefix: []string{"api"}, Stdout: body, Exit: 1}}})
		set, unavailable, f := c.probeBranchMergeRules(context.Background(), Repository{Host: "github.com", Owner: "o", Name: "n"}, "main")
		if f == nil || unavailable || set != (methodSet{}) {
			t.Errorf("%s: must fail closed, got set=%+v unavailable=%v f=%v", name, set, unavailable, f)
		}
	}
}
