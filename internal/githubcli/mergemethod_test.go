package githubcli

import (
	"context"
	"reflect"
	"testing"
)

// TestSelectMergeMethodPriority covers every effective-set combination: the
// selection is the fixed preference order rebase → merge → squash, else
// unavailable. Mutation discipline: each row that removes the previously
// selected method and requires the next one IS the mutation check — a selection
// that ignores the removal reddens the row.
func TestSelectMergeMethodPriority(t *testing.T) {
	cases := []struct {
		name string
		eff  methodSet
		want MergeMethod
		ok   bool
	}{
		{"all enabled -> rebase", methodSet{rebase: true, merge: true, squash: true}, MethodRebase, true},
		{"rebase only", methodSet{rebase: true}, MethodRebase, true},
		{"rebase+squash -> rebase", methodSet{rebase: true, squash: true}, MethodRebase, true},
		{"merge+squash -> merge", methodSet{merge: true, squash: true}, MethodMerge, true},
		{"merge only", methodSet{merge: true}, MethodMerge, true},
		{"squash only -> squash", methodSet{squash: true}, MethodSquash, true},
		{"empty -> unavailable", methodSet{}, "", false},
	}
	for _, c := range cases {
		got, ok := selectMergeMethod(c.eff)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: got (%q,%v) want (%q,%v)", c.name, got, ok, c.want, c.ok)
		}
	}
}

// TestMethodSetIntersect: repository permissions and branch rules compose by
// intersection, per element.
func TestMethodSetIntersect(t *testing.T) {
	a := methodSet{rebase: true, merge: true}
	b := methodSet{merge: true, squash: true}
	if got := a.intersect(b); got != (methodSet{merge: true}) {
		t.Fatalf("intersect: got %+v", got)
	}
	if got := a.intersect(methodSet{}); got != (methodSet{}) {
		t.Fatalf("intersect with empty must be empty: got %+v", got)
	}
}

// TestMergeFlag: the closed vocabulary renders exactly one gh flag per method;
// anything outside the vocabulary renders NOTHING (the act path guards on it).
func TestMergeFlag(t *testing.T) {
	for m, want := range map[MergeMethod]string{
		MethodRebase: "--rebase", MethodMerge: "--merge", MethodSquash: "--squash", MergeMethod("bogus"): "",
	} {
		if got := m.mergeFlag(); got != want {
			t.Errorf("mergeFlag(%q) = %q, want %q", m, got, want)
		}
	}
}

// TestMethodSetList: the diagnostic rendering names permitted methods in
// preference order.
func TestMethodSetList(t *testing.T) {
	got := methodSet{rebase: true, squash: true}.list()
	if !reflect.DeepEqual(got, []MergeMethod{MethodRebase, MethodSquash}) {
		t.Fatalf("list: got %v", got)
	}
	if l := (methodSet{}).list(); len(l) != 0 {
		t.Fatalf("empty set must list nothing, got %v", l)
	}
}

// TestProbeRepoMergeMethods: the three booleans decode explicitly from gh's
// repository endpoint; the argv is exact (api, --hostname, repos/o/n).
func TestProbeRepoMergeMethods(t *testing.T) {
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

// TestProbeRepoMergeMethodsFailsClosed: a missing boolean, malformed JSON, or a
// non-zero gh exit is a typed Failure — never a permissive default set.
func TestProbeRepoMergeMethodsFailsClosed(t *testing.T) {
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

// TestProbeBranchMergeRules: rules restrict by intersection; linear history
// removes merge; no method-specific rule contributes no restriction; the base
// branch is path-escaped so "feat/parent" is ONE endpoint segment.
func TestProbeBranchMergeRules(t *testing.T) {
	rules := `[
		{"type":"pull_request","parameters":{"allowed_merge_methods":["merge","squash"]}},
		{"type":"pull_request","parameters":{"allowed_merge_methods":["squash","rebase"]}},
		{"type":"required_linear_history","parameters":{}}
	]`
	c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{{
		ArgvPrefix: []string{"api", "--hostname", "github.com", "repos/octo/widgets/rules/branches/feat%2Fparent"},
		Stdout:     rules,
	}}})
	set, f := c.probeBranchMergeRules(context.Background(), Repository{Host: "github.com", Owner: "octo", Name: "widgets"}, "feat/parent")
	if f != nil {
		t.Fatalf("unexpected failure: %v", f)
	}
	// intersection of the two allowed_merge_methods rules is {squash}; linear
	// history removes merge (already absent). rebase excluded by the first rule.
	if set != (methodSet{squash: true}) {
		t.Fatalf("composed branch set: %+v", set)
	}
}

// TestProbeBranchMergeRulesNoRestriction: an empty rules array (or rules of
// unrelated types, or a pull_request rule without allowed_merge_methods)
// contributes no restriction — all three methods stay permitted.
func TestProbeBranchMergeRulesNoRestriction(t *testing.T) {
	for name, body := range map[string]string{
		"empty array":    `[]`,
		"unrelated rule": `[{"type":"deletion","parameters":{}}]`,
		"pr rule no key": `[{"type":"pull_request","parameters":{"required_approving_review_count":0}}]`,
	} {
		c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{{ArgvPrefix: []string{"api"}, Stdout: body}}})
		set, f := c.probeBranchMergeRules(context.Background(), Repository{Host: "github.com", Owner: "o", Name: "n"}, "main")
		if f != nil || set != (methodSet{rebase: true, merge: true, squash: true}) {
			t.Errorf("%s: want unrestricted set, got %+v f=%v", name, set, f)
		}
	}
}

// TestProbeBranchMergeRulesFailsClosed: malformed payloads, an EMPTY
// allowed_merge_methods array, an unknown token, and a failed request are all
// unobservable/invalid policy — a typed Failure, never a guess.
func TestProbeBranchMergeRulesFailsClosed(t *testing.T) {
	cases := map[string]fakeArm{
		"malformed json": {ArgvPrefix: []string{"api"}, Stdout: `{"not":"an array"}`},
		"empty methods":  {ArgvPrefix: []string{"api"}, Stdout: `[{"type":"pull_request","parameters":{"allowed_merge_methods":[]}}]`},
		"unknown token":  {ArgvPrefix: []string{"api"}, Stdout: `[{"type":"pull_request","parameters":{"allowed_merge_methods":["MERGE"]}}]`},
		"http failure":   {ArgvPrefix: []string{"api"}, Exit: 1, Stderr: "gh: Server Error (HTTP 500)"},
	}
	for name, arm := range cases {
		c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{arm}})
		if _, f := c.probeBranchMergeRules(context.Background(), Repository{Host: "github.com", Owner: "o", Name: "n"}, "main"); f == nil {
			t.Errorf("%s: must fail closed", name)
		}
	}
}
