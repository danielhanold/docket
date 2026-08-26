package githubcli

import (
	"context"
	"reflect"
	"testing"
)

// MergePullRequest and ProbeMerged are driven through the protocol-faithful fake
// gh. Every decision reads an authoritative snapshot; the merge selects the
// effective method (rebase → merge commit → squash) permitted by the repository
// settings and the base branch's active rules, carries an exact
// --match-head-commit and NEVER --delete-branch; and post-merge truth is
// established by reprobe, never by the merge process exit code.

const (
	mrgBase        = "main"
	mrgMergedAt    = "2026-08-18T12:00:00Z"
	mrgMergeCommit = "3333333333333333333333333333333333333333"

	// Repository merge-method settings payloads, in gh's repos/<o>/<n> shape.
	repoAllTrue     = `{"allow_rebase_merge":true,"allow_merge_commit":true,"allow_squash_merge":true}`
	repoAllFalse    = `{"allow_rebase_merge":false,"allow_merge_commit":false,"allow_squash_merge":false}`
	repoMergeSquash = `{"allow_rebase_merge":false,"allow_merge_commit":true,"allow_squash_merge":true}`
	repoSquashOnly  = `{"allow_rebase_merge":false,"allow_merge_commit":false,"allow_squash_merge":true}`
	// An empty active-rules array imposes no branch restriction.
	rulesNone = `[]`
)

func mrgRepo() Repository { return Repository{Host: "github.com", Owner: "acme", Name: "widget"} }

// openPR renders an open PR merge snapshot at the given head oid and mergeability.
func openPR(oid, mergeable string) string {
	return mergePRDoc(7, "OPEN", ensHead, oid, mrgBase, ensTitle, ensBody, "", "", mergeable)
}

// mergedPR renders a merged PR merge snapshot with mergedAt and mergeCommit set.
func mergedPR() string {
	return mergePRDoc(7, "MERGED", ensHead, ensHeadOid, mrgBase, ensTitle, ensBody, mrgMergedAt, mrgMergeCommit, "MERGEABLE")
}

func mrgViewArm(stdout string, exit int) fakeArm {
	return fakeArm{ArgvPrefix: []string{"pr", "view"}, Stdout: stdout, Exit: exit}
}

func mrgMergeArm(exit int) fakeArm {
	return fakeArm{ArgvPrefix: []string{"pr", "merge"}, Exit: exit}
}

// mrgRepoSettingsArm scripts the `gh api ... repos/acme/widget` capability read.
// The exact endpoint segment distinguishes it from the branch-rules read, which
// carries a longer path.
func mrgRepoSettingsArm(stdout string, exit int) fakeArm {
	return fakeArm{ArgvPrefix: []string{"api", "--hostname", "github.com", "repos/acme/widget"}, Stdout: stdout, Exit: exit}
}

// mrgBranchRulesArm scripts the `gh api ... repos/acme/widget/rules/branches/main`
// active-rules read for the mrgBase base branch.
func mrgBranchRulesArm(stdout string, exit int) fakeArm {
	return fakeArm{ArgvPrefix: []string{"api", "--hostname", "github.com", "repos/acme/widget/rules/branches/main"}, Stdout: stdout, Exit: exit}
}

// assertMergeFlag proves exactly the given method flag reached the `pr merge`
// invocation in the witness log.
func assertMergeFlag(t *testing.T, log *witnessLog, flag string) {
	t.Helper()
	found := false
	for _, r := range log.records(t) {
		if hasArgvPrefix(r.Argv, []string{"pr", "merge"}) && argvContains(r.Argv, flag) {
			found = true
		}
	}
	if !found {
		t.Fatalf("no pr merge invocation carried %s", flag)
	}
}

func assertMergedFacts(t *testing.T, f MergedFacts) {
	t.Helper()
	if f.HeadOID != ensHeadOid {
		t.Errorf("HeadOID = %q, want %q", f.HeadOID, ensHeadOid)
	}
	if f.BaseRef != mrgBase {
		t.Errorf("BaseRef = %q, want %q", f.BaseRef, mrgBase)
	}
	if f.HeadBranch != ensHead {
		t.Errorf("HeadBranch = %q, want %q", f.HeadBranch, ensHead)
	}
	if f.MergedAtUTC != mrgMergedAt {
		t.Errorf("MergedAtUTC = %q, want %q", f.MergedAtUTC, mrgMergedAt)
	}
	if f.MergeCommit != mrgMergeCommit {
		t.Errorf("MergeCommit = %q, want %q", f.MergeCommit, mrgMergeCommit)
	}
	if f.Version == "" {
		t.Error("Version is empty")
	}
}

func TestMergeExpectedHead(t *testing.T) {
	head := ObjectRef(ensHeadOid)

	t.Run("merged", func(t *testing.T) {
		c, log := newFakeClient(t, fakeScenario{
			Sequential: true,
			Invocations: []fakeArm{
				mrgViewArm(openPR(ensHeadOid, "MERGEABLE"), 0), // decision probe
				mrgRepoSettingsArm(repoAllTrue, 0),             // repository settings
				mrgBranchRulesArm(rulesNone, 0),                // branch rules
				mrgMergeArm(0),                                 // act
				mrgViewArm(mergedPR(), 0),                      // verify
			},
		})
		res, err := c.MergePullRequest(context.Background(), mrgRepo(), 7, head, false)
		if err != nil {
			t.Fatalf("MergePullRequest: %v", err)
		}
		if res.Outcome != MergeMerged {
			t.Fatalf("outcome = %q, want %q", res.Outcome, MergeMerged)
		}
		// All three methods enabled and no branch restriction selects rebase.
		if res.Method != MethodRebase {
			t.Fatalf("method = %q, want %q", res.Method, MethodRebase)
		}
		assertMergedFacts(t, res.Facts)
		assertMergeFlag(t, log, "--rebase")
		// The merge argv carries an exact --match-head-commit for the expected oid.
		var matched bool
		for _, r := range log.records(t) {
			if hasArgvPrefix(r.Argv, []string{"pr", "merge"}) {
				matched = argvHasFlagValue(r.Argv, "--match-head-commit", ensHeadOid)
			}
		}
		if !matched {
			t.Fatal("merge did not carry --match-head-commit <expected oid>")
		}
	})

	t.Run("head-moved", func(t *testing.T) {
		// GitHub reports a head other than the expected one: refuse, no merge.
		c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
			mrgViewArm(openPR(ensOtherOid, "MERGEABLE"), 0),
		}})
		res, err := c.MergePullRequest(context.Background(), mrgRepo(), 7, head, false)
		if err != nil {
			t.Fatalf("MergePullRequest: %v", err)
		}
		if res.Outcome != MergeHeadMoved {
			t.Fatalf("outcome = %q, want %q", res.Outcome, MergeHeadMoved)
		}
		if res.Method != "" {
			t.Fatalf("head-moved carried a method %q, want empty", res.Method)
		}
		if n := countArgv(log.records(t), "pr", "merge"); n != 0 {
			t.Fatalf("pr merge issued %d times on a moved head, want 0", n)
		}
		if n := countArgv(log.records(t), "api"); n != 0 {
			t.Fatalf("capability probes issued %d times before a moved-head refusal, want 0", n)
		}
	})

	t.Run("already-merged", func(t *testing.T) {
		// The PR is already merged before the attempt: no merge call, facts returned.
		c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{mrgViewArm(mergedPR(), 0)}})
		res, err := c.MergePullRequest(context.Background(), mrgRepo(), 7, head, false)
		if err != nil {
			t.Fatalf("MergePullRequest: %v", err)
		}
		if res.Outcome != MergeAlreadyMerged {
			t.Fatalf("outcome = %q, want %q", res.Outcome, MergeAlreadyMerged)
		}
		// Already-merged recovery makes no method choice of its own.
		if res.Method != "" {
			t.Fatalf("already-merged carried a method %q, want empty", res.Method)
		}
		assertMergedFacts(t, res.Facts)
		if n := countArgv(log.records(t), "pr", "merge"); n != 0 {
			t.Fatalf("pr merge issued %d times on an already-merged PR, want 0", n)
		}
		// Already-merged recovery performs no capability probes.
		if n := countArgv(log.records(t), "api"); n != 0 {
			t.Fatalf("capability probes issued %d times on already-merged recovery, want 0", n)
		}
	})

	t.Run("conflicting-not-mergeable", func(t *testing.T) {
		c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
			mrgViewArm(openPR(ensHeadOid, "CONFLICTING"), 0),
		}})
		res, err := c.MergePullRequest(context.Background(), mrgRepo(), 7, head, false)
		if err != nil {
			t.Fatalf("MergePullRequest: %v", err)
		}
		if res.Outcome != MergeNotMergeable {
			t.Fatalf("outcome = %q, want %q", res.Outcome, MergeNotMergeable)
		}
		if n := countArgv(log.records(t), "pr", "merge"); n != 0 {
			t.Fatalf("pr merge issued on a conflicting PR, want 0")
		}
	})

	t.Run("unknown-mergeability-never-authorizes", func(t *testing.T) {
		c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
			mrgViewArm(openPR(ensHeadOid, "UNKNOWN"), 0),
		}})
		res, err := c.MergePullRequest(context.Background(), mrgRepo(), 7, head, false)
		if err != nil {
			t.Fatalf("MergePullRequest: %v", err)
		}
		if res.Outcome != MergeUnknown {
			t.Fatalf("outcome = %q, want %q (UNKNOWN mergeability must never authorize a merge)", res.Outcome, MergeUnknown)
		}
		if n := countArgv(log.records(t), "pr", "merge"); n != 0 {
			t.Fatalf("pr merge issued on UNKNOWN mergeability, want 0")
		}
	})

	t.Run("denied", func(t *testing.T) {
		// The merge is issued but rejected (non-zero exit) and the PR sits still open,
		// still mergeable, at the same head: an authoritative policy/permission denial.
		c, _ := newFakeClient(t, fakeScenario{
			Sequential: true,
			Invocations: []fakeArm{
				mrgViewArm(openPR(ensHeadOid, "MERGEABLE"), 0),
				mrgRepoSettingsArm(repoAllTrue, 0),
				mrgBranchRulesArm(rulesNone, 0),
				mrgMergeArm(1),
				mrgViewArm(openPR(ensHeadOid, "MERGEABLE"), 0),
			},
		})
		res, err := c.MergePullRequest(context.Background(), mrgRepo(), 7, head, false)
		if err != nil {
			t.Fatalf("MergePullRequest: %v", err)
		}
		if res.Outcome != MergeDenied {
			t.Fatalf("outcome = %q, want %q", res.Outcome, MergeDenied)
		}
		// The denial carries the method that was actually attempted.
		if res.Method != MethodRebase {
			t.Fatalf("denied method = %q, want %q", res.Method, MethodRebase)
		}
	})

	t.Run("response-loss-recovers", func(t *testing.T) {
		// The merge exits non-zero (lost response) but actually LANDED: the verify
		// reprobe recovers merged; a later independent ProbeMerged reports
		// already-merged with the mergedAt + merge commit.
		c, _ := newFakeClient(t, fakeScenario{
			Sequential: true,
			Invocations: []fakeArm{
				mrgViewArm(openPR(ensHeadOid, "MERGEABLE"), 0), // decision
				mrgRepoSettingsArm(repoAllTrue, 0),             // repository settings
				mrgBranchRulesArm(rulesNone, 0),                // branch rules
				mrgMergeArm(1),                                 // act: lost response
				mrgViewArm(mergedPR(), 0),                      // MergePullRequest verify
				mrgViewArm(mergedPR(), 0),                      // ProbeMerged
			},
		})
		res, err := c.MergePullRequest(context.Background(), mrgRepo(), 7, head, false)
		if err != nil {
			t.Fatalf("MergePullRequest: %v", err)
		}
		if res.Outcome != MergeMerged {
			t.Fatalf("MergePullRequest outcome = %q, want %q", res.Outcome, MergeMerged)
		}
		// The merge command WAS issued on the recovered path, so the method rides.
		if res.Method != MethodRebase {
			t.Fatalf("recovered-merge method = %q, want %q", res.Method, MethodRebase)
		}
		pout, pfacts, perr := c.ProbeMerged(context.Background(), mrgRepo(), 7)
		if perr != nil {
			t.Fatalf("ProbeMerged: %v", perr)
		}
		if pout != MergeAlreadyMerged {
			t.Fatalf("ProbeMerged outcome = %q, want %q", pout, MergeAlreadyMerged)
		}
		assertMergedFacts(t, pfacts)
	})

	t.Run("probe-merged-open-is-not-merged", func(t *testing.T) {
		c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
			mrgViewArm(openPR(ensHeadOid, "MERGEABLE"), 0),
		}})
		out, facts, err := c.ProbeMerged(context.Background(), mrgRepo(), 7)
		if err != nil {
			t.Fatalf("ProbeMerged: %v", err)
		}
		if out == MergeMerged || out == MergeAlreadyMerged {
			t.Fatalf("ProbeMerged on an open PR returned a merged outcome %q", out)
		}
		if out != MergeNotMergeable {
			t.Fatalf("ProbeMerged open outcome = %q, want %q", out, MergeNotMergeable)
		}
		if facts.MergedAtUTC != "" || facts.MergeCommit != "" {
			t.Fatalf("open PR carried merge facts: %+v", facts)
		}
	})

	t.Run("probe-merged-malformed-is-unknown", func(t *testing.T) {
		c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{mrgViewArm("{ not json", 0)}})
		out, _, err := c.ProbeMerged(context.Background(), mrgRepo(), 7)
		if out != MergeUnknown {
			t.Fatalf("ProbeMerged malformed outcome = %q, want %q", out, MergeUnknown)
		}
		if err == nil {
			t.Fatal("unknown outcome must carry a diagnostic error")
		}
	})
}

// TestMergeNeverDeletesBranch proves no merge invocation ever requests branch
// deletion — cleanup is an independent, separately-owned suffix.
func TestMergeNeverDeletesBranch(t *testing.T) {
	c, log := newFakeClient(t, fakeScenario{
		Sequential: true,
		Invocations: []fakeArm{
			mrgViewArm(openPR(ensHeadOid, "MERGEABLE"), 0),
			mrgRepoSettingsArm(repoAllTrue, 0),
			mrgBranchRulesArm(rulesNone, 0),
			mrgMergeArm(0),
			mrgViewArm(mergedPR(), 0),
		},
	})
	if _, err := c.MergePullRequest(context.Background(), mrgRepo(), 7, ObjectRef(ensHeadOid), true); err != nil {
		t.Fatalf("MergePullRequest: %v", err)
	}
	for _, r := range log.records(t) {
		for _, a := range r.Argv {
			if a == "--delete-branch" {
				t.Fatalf("a gh invocation requested --delete-branch: %v", r.Argv)
			}
		}
	}
}

// TestMergeSelectsMergeWhenRebaseDisabled: repository settings with rebase off
// but merge and squash on select the merge commit (the next priority).
func TestMergeSelectsMergeWhenRebaseDisabled(t *testing.T) {
	c, log := newFakeClient(t, fakeScenario{
		Sequential: true,
		Invocations: []fakeArm{
			mrgViewArm(openPR(ensHeadOid, "MERGEABLE"), 0),
			mrgRepoSettingsArm(repoMergeSquash, 0),
			mrgBranchRulesArm(rulesNone, 0),
			mrgMergeArm(0),
			mrgViewArm(mergedPR(), 0),
		},
	})
	res, err := c.MergePullRequest(context.Background(), mrgRepo(), 7, ObjectRef(ensHeadOid), false)
	if err != nil {
		t.Fatalf("MergePullRequest: %v", err)
	}
	if res.Outcome != MergeMerged {
		t.Fatalf("outcome = %q, want %q", res.Outcome, MergeMerged)
	}
	if res.Method != MethodMerge {
		t.Fatalf("method = %q, want %q", res.Method, MethodMerge)
	}
	assertMergeFlag(t, log, "--merge")
}

// TestMergeSelectsSquashOnly: a squash-only repository selects squash, the
// last-priority method.
func TestMergeSelectsSquashOnly(t *testing.T) {
	c, log := newFakeClient(t, fakeScenario{
		Sequential: true,
		Invocations: []fakeArm{
			mrgViewArm(openPR(ensHeadOid, "MERGEABLE"), 0),
			mrgRepoSettingsArm(repoSquashOnly, 0),
			mrgBranchRulesArm(rulesNone, 0),
			mrgMergeArm(0),
			mrgViewArm(mergedPR(), 0),
		},
	})
	res, err := c.MergePullRequest(context.Background(), mrgRepo(), 7, ObjectRef(ensHeadOid), false)
	if err != nil {
		t.Fatalf("MergePullRequest: %v", err)
	}
	if res.Outcome != MergeMerged {
		t.Fatalf("outcome = %q, want %q", res.Outcome, MergeMerged)
	}
	if res.Method != MethodSquash {
		t.Fatalf("method = %q, want %q", res.Method, MethodSquash)
	}
	assertMergeFlag(t, log, "--squash")
}

// TestMergeMethodUnavailableIssuesNoMerge: a cleanly observed empty effective set
// (all repository methods disabled, no branch restriction) is method-unavailable
// — a value outcome (nil error), no method, no merge issued. The scenario has no
// pr merge arm, so any issued merge would exit fakeExitUnmatched and redden the
// witness assertion. RepoMethods names the empty repository set; BranchMethods
// names the unrestricted branch set so a human can see the conflict.
func TestMergeMethodUnavailableIssuesNoMerge(t *testing.T) {
	c, log := newFakeClient(t, fakeScenario{
		Sequential: true,
		Invocations: []fakeArm{
			mrgViewArm(openPR(ensHeadOid, "MERGEABLE"), 0),
			mrgRepoSettingsArm(repoAllFalse, 0),
			mrgBranchRulesArm(rulesNone, 0),
		},
	})
	res, err := c.MergePullRequest(context.Background(), mrgRepo(), 7, ObjectRef(ensHeadOid), false)
	if err != nil {
		t.Fatalf("method-unavailable must be a value outcome with a nil error, got %v", err)
	}
	if res.Outcome != MergeMethodUnavailable {
		t.Fatalf("outcome = %q, want %q", res.Outcome, MergeMethodUnavailable)
	}
	if res.Method != "" {
		t.Fatalf("method = %q, want empty (no merge was issued)", res.Method)
	}
	if len(res.RepoMethods) != 0 {
		t.Fatalf("RepoMethods = %v, want empty", res.RepoMethods)
	}
	if !reflect.DeepEqual(res.BranchMethods, []MergeMethod{MethodRebase, MethodMerge, MethodSquash}) {
		t.Fatalf("BranchMethods = %v, want all three", res.BranchMethods)
	}
	if n := countArgv(log.records(t), "pr", "merge"); n != 0 {
		t.Fatalf("pr merge issued %d times on an empty permitted set, want 0", n)
	}
}

// TestMergeProbeFailureIssuesNoMerge: a failed capability probe is unknown
// (retain) with a non-null error, and issues no merge.
func TestMergeProbeFailureIssuesNoMerge(t *testing.T) {
	c, log := newFakeClient(t, fakeScenario{
		Sequential: true,
		Invocations: []fakeArm{
			mrgViewArm(openPR(ensHeadOid, "MERGEABLE"), 0),
			mrgRepoSettingsArm("", 1), // gh api exits non-zero
		},
	})
	res, err := c.MergePullRequest(context.Background(), mrgRepo(), 7, ObjectRef(ensHeadOid), false)
	if err == nil {
		t.Fatal("a failed capability probe must carry a diagnostic error")
	}
	if res.Outcome != MergeUnknown {
		t.Fatalf("outcome = %q, want %q", res.Outcome, MergeUnknown)
	}
	if n := countArgv(log.records(t), "pr", "merge"); n != 0 {
		t.Fatalf("pr merge issued %d times after a probe failure, want 0", n)
	}
}

// TestMergeDeniedNeverRetriesAnotherMethod: an authoritative denial of the
// selected method is never retried with a lower-priority method — exactly one
// pr merge invocation reaches the witness log.
func TestMergeDeniedNeverRetriesAnotherMethod(t *testing.T) {
	c, log := newFakeClient(t, fakeScenario{
		Sequential: true,
		Invocations: []fakeArm{
			mrgViewArm(openPR(ensHeadOid, "MERGEABLE"), 0),
			mrgRepoSettingsArm(repoAllTrue, 0),
			mrgBranchRulesArm(rulesNone, 0),
			mrgMergeArm(1),                                 // the selected --rebase is rejected
			mrgViewArm(openPR(ensHeadOid, "MERGEABLE"), 0), // reprobe: still open, mergeable, same head
		},
	})
	res, err := c.MergePullRequest(context.Background(), mrgRepo(), 7, ObjectRef(ensHeadOid), false)
	if err != nil {
		t.Fatalf("MergePullRequest: %v", err)
	}
	if res.Outcome != MergeDenied {
		t.Fatalf("outcome = %q, want %q", res.Outcome, MergeDenied)
	}
	if res.Method != MethodRebase {
		t.Fatalf("method = %q, want %q", res.Method, MethodRebase)
	}
	if n := countArgv(log.records(t), "pr", "merge"); n != 1 {
		t.Fatalf("pr merge issued %d times; a denial must never retry another method (want exactly 1)", n)
	}
}

// TestFakeLazyMergeability proves an UNKNOWN mergeability round-trips through the
// decoder as UNKNOWN — never silently read as clean/mergeable.
func TestFakeLazyMergeability(t *testing.T) {
	snap, err := decodeMergeSnapshot("test", []byte(openPR(ensHeadOid, "UNKNOWN")))
	if err != nil {
		t.Fatalf("decodeMergeSnapshot: %v", err)
	}
	if snap.mergeable != "UNKNOWN" {
		t.Fatalf("mergeable = %q, want %q", snap.mergeable, "UNKNOWN")
	}
}

// argvHasFlagValue reports whether argv contains flag immediately followed by
// value.
func argvHasFlagValue(argv []string, flag, value string) bool {
	for i := 0; i+1 < len(argv); i++ {
		if argv[i] == flag && argv[i+1] == value {
			return true
		}
	}
	return false
}
