package githubcli

import (
	"context"
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
