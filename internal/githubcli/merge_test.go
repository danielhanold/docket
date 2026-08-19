package githubcli

import (
	"context"
	"testing"
)

// MergePullRequest and ProbeMerged are driven through the protocol-faithful fake
// gh. Every decision reads an authoritative snapshot; the merge carries an exact
// --match-head-commit and NEVER --delete-branch; and post-merge truth is
// established by reprobe, never by the merge process exit code.

const (
	mrgBase        = "main"
	mrgMergedAt    = "2026-08-18T12:00:00Z"
	mrgMergeCommit = "3333333333333333333333333333333333333333"
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

func assertMergedFacts(t *testing.T, f MergedFacts) {
	t.Helper()
	if f.HeadOID != ensHeadOid {
		t.Errorf("HeadOID = %q, want %q", f.HeadOID, ensHeadOid)
	}
	if f.BaseRef != mrgBase {
		t.Errorf("BaseRef = %q, want %q", f.BaseRef, mrgBase)
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
				mrgMergeArm(0),            // act
				mrgViewArm(mergedPR(), 0), // verify
			},
		})
		out, facts, err := c.MergePullRequest(context.Background(), mrgRepo(), 7, head, false)
		if err != nil {
			t.Fatalf("MergePullRequest: %v", err)
		}
		if out != MergeMerged {
			t.Fatalf("outcome = %q, want %q", out, MergeMerged)
		}
		assertMergedFacts(t, facts)
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
		out, _, err := c.MergePullRequest(context.Background(), mrgRepo(), 7, head, false)
		if err != nil {
			t.Fatalf("MergePullRequest: %v", err)
		}
		if out != MergeHeadMoved {
			t.Fatalf("outcome = %q, want %q", out, MergeHeadMoved)
		}
		if n := countArgv(log.records(t), "pr", "merge"); n != 0 {
			t.Fatalf("pr merge issued %d times on a moved head, want 0", n)
		}
	})

	t.Run("already-merged", func(t *testing.T) {
		// The PR is already merged before the attempt: no merge call, facts returned.
		c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{mrgViewArm(mergedPR(), 0)}})
		out, facts, err := c.MergePullRequest(context.Background(), mrgRepo(), 7, head, false)
		if err != nil {
			t.Fatalf("MergePullRequest: %v", err)
		}
		if out != MergeAlreadyMerged {
			t.Fatalf("outcome = %q, want %q", out, MergeAlreadyMerged)
		}
		assertMergedFacts(t, facts)
		if n := countArgv(log.records(t), "pr", "merge"); n != 0 {
			t.Fatalf("pr merge issued %d times on an already-merged PR, want 0", n)
		}
	})

	t.Run("conflicting-not-mergeable", func(t *testing.T) {
		c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
			mrgViewArm(openPR(ensHeadOid, "CONFLICTING"), 0),
		}})
		out, _, err := c.MergePullRequest(context.Background(), mrgRepo(), 7, head, false)
		if err != nil {
			t.Fatalf("MergePullRequest: %v", err)
		}
		if out != MergeNotMergeable {
			t.Fatalf("outcome = %q, want %q", out, MergeNotMergeable)
		}
		if n := countArgv(log.records(t), "pr", "merge"); n != 0 {
			t.Fatalf("pr merge issued on a conflicting PR, want 0")
		}
	})

	t.Run("unknown-mergeability-never-authorizes", func(t *testing.T) {
		c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
			mrgViewArm(openPR(ensHeadOid, "UNKNOWN"), 0),
		}})
		out, _, err := c.MergePullRequest(context.Background(), mrgRepo(), 7, head, false)
		if err != nil {
			t.Fatalf("MergePullRequest: %v", err)
		}
		if out != MergeUnknown {
			t.Fatalf("outcome = %q, want %q (UNKNOWN mergeability must never authorize a merge)", out, MergeUnknown)
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
				mrgMergeArm(1),
				mrgViewArm(openPR(ensHeadOid, "MERGEABLE"), 0),
			},
		})
		out, _, err := c.MergePullRequest(context.Background(), mrgRepo(), 7, head, false)
		if err != nil {
			t.Fatalf("MergePullRequest: %v", err)
		}
		if out != MergeDenied {
			t.Fatalf("outcome = %q, want %q", out, MergeDenied)
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
				mrgMergeArm(1),            // act: lost response
				mrgViewArm(mergedPR(), 0), // MergePullRequest verify
				mrgViewArm(mergedPR(), 0), // ProbeMerged
			},
		})
		out, _, err := c.MergePullRequest(context.Background(), mrgRepo(), 7, head, false)
		if err != nil {
			t.Fatalf("MergePullRequest: %v", err)
		}
		if out != MergeMerged {
			t.Fatalf("MergePullRequest outcome = %q, want %q", out, MergeMerged)
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
			mrgMergeArm(0),
			mrgViewArm(mergedPR(), 0),
		},
	})
	if _, _, err := c.MergePullRequest(context.Background(), mrgRepo(), 7, ObjectRef(ensHeadOid), true); err != nil {
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
