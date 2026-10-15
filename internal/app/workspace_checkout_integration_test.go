//go:build integration

package app

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/workspace"
)

// checkoutDeps wires the fixture's real workspace service and the given fake
// GitHub into the checkout operation's seams.
func (f *rebaseFixture) checkoutDeps(gh WorkspacePRReader) WorkspaceCheckoutDeps {
	return WorkspaceCheckoutDeps{Service: f.svc, GitHub: gh}
}

// TestIntegrationWorkspaceCheckoutAttachesBuiltElsewhere: on a clone that never
// built the change, with a clean foreign worktree holding the fetched branch,
// checkout removes that worktree (reporting it) and attaches the branch at the
// PR head in the owned path; a pinned rerun is a no-op.
func TestIntegrationWorkspaceCheckoutAttachesBuiltElsewhere(t *testing.T) {
	requireRealGit(t)
	ctx := context.Background()
	f := setupRebaseFixture(t, planRepoModes()[0])
	f.forgetLocalWorkspace(t)
	b := f.target.FeatureBranch()
	runGit(t, f.repo.invocation, "fetch", "-q", "origin", "refs/heads/"+b+":refs/heads/"+b)
	foreign := filepath.Join(f.repo.root, "gh-checkout")
	runGit(t, f.repo.invocation, "worktree", "add", "-q", foreign, b)
	canon, err := filepath.EvalSymlinks(foreign)
	if err != nil {
		t.Fatal(err)
	}
	gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}

	res := WorkspaceCheckout(ctx, f.deps, f.checkoutDeps(gh), f.repo.invocation, WorkspaceCheckoutRequest{ID: f.id})
	if res.Result != ResultApplied || res.Disposition != "attached" || res.Path != f.wp || res.Head != f.head || res.Operation != OperationWorkspaceCheckout {
		t.Fatalf("= %q/%q %s@%s (%s: %s)", res.Result, res.Disposition, res.Path, res.Head, res.Reason, res.Message)
	}
	if len(res.Findings) != 1 || res.Findings[0].Code != FindingForeignWorktreeRemoved || res.Findings[0].Path != canon {
		t.Errorf("findings = %+v; want one %s naming %s", res.Findings, FindingForeignWorktreeRemoved, canon)
	}
	if insp := WorkspaceInspect(ctx, f.deps, WorkspaceDeps{Service: f.svc}, f.repo.invocation, WorkspaceIDRequest{ID: f.id}); insp.State != "ready" {
		t.Errorf("inspect = %q; want ready", insp.State)
	}
	again := WorkspaceCheckout(ctx, f.deps, f.checkoutDeps(gh), f.repo.invocation, WorkspaceCheckoutRequest{ID: f.id, Revision: f.revision, Head: f.head})
	if again.Result != ResultNoOp || again.Disposition != "existing" {
		t.Errorf("rerun = %q/%q; want no-op/existing", again.Result, again.Disposition)
	}
}

// checkoutRefusalRow is one refusal the checkout operation reports before it
// changes anything.
type checkoutRefusalRow struct {
	name   string
	gh     *fakeRebaseGitHub
	req    WorkspaceCheckoutRequest
	result Result
	disp   string
	reason string
}

// checkoutRefusalRows lists every app-level refusal plus the service's remote
// head disagreement, each over a fixture whose local workspace was forgotten.
func checkoutRefusalRows(f *rebaseFixture) []checkoutRefusalRow {
	gh := func(prs ...githubcli.PullRequest) *fakeRebaseGitHub {
		return &fakeRebaseGitHub{repo: retargetRepo(), prs: prs}
	}
	open := []githubcli.PullRequest{f.prForHead(f.head, "")}
	other := strings.Repeat("e", 40)
	baseMoved := f.prForHead(f.head, "")
	baseMoved.BaseBranch = "develop"
	return []checkoutRefusalRow{
		{"revision-without-head", gh(open...), WorkspaceCheckoutRequest{ID: f.id, Revision: f.revision},
			ResultInvalidInput, "", ReasonWorkspaceCheckoutPinIncomplete},
		{"malformed-head", gh(open...), WorkspaceCheckoutRequest{ID: f.id, Revision: f.revision, Head: strings.ToUpper(f.head)},
			ResultInvalidInput, "", ReasonWorkspaceMalformedHead},
		{"revision-drift", gh(open...), WorkspaceCheckoutRequest{ID: f.id, Revision: strings.Repeat("0", 40), Head: f.head},
			ResultContended, "contended", ReasonWorkspaceRevisionDrift},
		{"head-flag-mismatch", gh(open...), WorkspaceCheckoutRequest{ID: f.id, Revision: f.revision, Head: other},
			ResultContended, "contended", ReasonWorkspaceHeadMismatch},
		{"pr-head-not-remote", gh(f.prForHead(other, "")), WorkspaceCheckoutRequest{ID: f.id},
			ResultContended, "contended", workspace.CheckoutReasonHeadMismatch},
		{"no-open-pr", gh(), WorkspaceCheckoutRequest{ID: f.id},
			ResultBlocked, "blocked", ReasonWorkspaceCheckoutPRNotOpen},
		{"pr-base-mismatch", gh(baseMoved), WorkspaceCheckoutRequest{ID: f.id},
			ResultBlocked, "blocked", ReasonWorkspaceCheckoutPRBaseMismatch},
		{"pr-probe-error", &fakeRebaseGitHub{repo: retargetRepo(), findErr: errors.New("gh down")}, WorkspaceCheckoutRequest{ID: f.id},
			ResultExternalFailed, "unknown", ReasonWorkspaceCheckoutPRProbe},
		{"repo-unresolved", &fakeRebaseGitHub{repoErr: errors.New("no remote")}, WorkspaceCheckoutRequest{ID: f.id},
			ResultExternalFailed, "unknown", ReasonWorkspaceCheckoutRepoUnresolved},
	}
}

// Every refusal leaves the clone as a machine that never built the change.
func TestIntegrationWorkspaceCheckoutRefusesBeforeAnyChange(t *testing.T) {
	requireRealGit(t)
	ctx := context.Background()
	f := setupRebaseFixture(t, planRepoModes()[0])
	f.forgetLocalWorkspace(t)
	for _, tc := range checkoutRefusalRows(f) {
		t.Run(tc.name, func(t *testing.T) {
			res := WorkspaceCheckout(ctx, f.deps, f.checkoutDeps(tc.gh), f.repo.invocation, tc.req)
			if res.Result != tc.result || res.Disposition != tc.disp || res.Reason != tc.reason {
				t.Fatalf("= %q/%q/%q (%s); want %q/%q/%q", res.Result, res.Disposition, res.Reason, res.Message, tc.result, tc.disp, tc.reason)
			}
		})
	}
	if insp := WorkspaceInspect(ctx, f.deps, WorkspaceDeps{Service: f.svc}, f.repo.invocation, WorkspaceIDRequest{ID: f.id}); insp.State != "absent" {
		t.Errorf("after the refusals the workspace is %q; want absent", insp.State)
	}
}

// TestIntegrationWorkspaceCheckoutRefusesNotImplemented: checkout takes over only
// an implemented change, leaving an in-progress change's workspace untouched.
func TestIntegrationWorkspaceCheckoutRefusesNotImplemented(t *testing.T) {
	requireRealGit(t)
	f := setupRebaseFixtureStatus(t, planRepoModes()[0], "in-progress")
	gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
	res := WorkspaceCheckout(context.Background(), f.deps, f.checkoutDeps(gh), f.repo.invocation, WorkspaceCheckoutRequest{ID: f.id})
	if res.Result != ResultBlocked || res.Reason != ReasonWorkspaceNotImplemented || f.localHead() != f.head {
		t.Fatalf("= %q/%q, head %s; want blocked/%s with the workspace untouched", res.Result, res.Reason, f.localHead(), ReasonWorkspaceNotImplemented)
	}
}
