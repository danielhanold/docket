package app

import (
	"context"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/workspace"
)

// --- fakes ----------------------------------------------------------------

// fakeWorkspaceService is a scriptable WorkspaceService: every method records
// its calls and returns a canned outcome, so a test can prove the app layer
// handed the service the target it should have — and, crucially, that it did NOT
// call the service on a pre-delegation refusal.
type fakeWorkspaceService struct {
	prepareCalls []workspace.PrepareRequest
	inspectCalls []workspace.InspectRequest
	publishCalls []workspace.PublishRequest

	prepareWS  workspace.Workspace
	prepareErr error
	inspection workspace.Inspection
	inspectErr error
	publishRes workspace.PublishResult
	publishErr error
}

func (f *fakeWorkspaceService) Prepare(_ context.Context, req workspace.PrepareRequest) (workspace.Workspace, error) {
	f.prepareCalls = append(f.prepareCalls, req)
	return f.prepareWS, f.prepareErr
}

func (f *fakeWorkspaceService) Inspect(_ context.Context, req workspace.InspectRequest) (workspace.Inspection, error) {
	f.inspectCalls = append(f.inspectCalls, req)
	return f.inspection, f.inspectErr
}

func (f *fakeWorkspaceService) PublishHead(_ context.Context, req workspace.PublishRequest) (workspace.PublishResult, error) {
	f.publishCalls = append(f.publishCalls, req)
	return f.publishRes, f.publishErr
}

// inProgressChangeBlob builds an in-progress change record StatusBlob (carrying
// the branch/claimed_at/reconciled fields a claimed record holds) at the given
// version, optionally with extra frontmatter lines spliced in after the
// stacked_on field (e.g. a stacked_on value).
func inProgressChangeBlob(id int, slug, version, stackedOn string) StatusBlob {
	src := lifecycleChange(id, slug, "in-progress")
	if stackedOn != "" {
		src = strings.Replace(src, "stacked_on:\n", "stacked_on: "+stackedOn+"\n", 1)
	}
	return StatusBlob{
		Kind:     repository.KindChange,
		Location: repository.LocationActive,
		Path:     groomPath(id, slug),
		Version:  version,
		Data:     []byte(src),
	}
}

// proposedChangeBlob builds a proposed (unclaimed) change record StatusBlob.
func proposedChangeBlob(id int, slug, version string) StatusBlob {
	return StatusBlob{
		Kind:     repository.KindChange,
		Location: repository.LocationActive,
		Path:     groomPath(id, slug),
		Version:  version,
		Data:     []byte(lifecycleChange(id, slug, "proposed")),
	}
}

// workspaceDepsFor builds the read-only planning seams a workspace operation
// needs over the fake reader plus a real Git client (the app layer discovers the
// repository from repoDir). The workspace service is the fake.
func workspaceDepsFor(t *testing.T, reader StatusReader) PlanningDeps {
	t.Helper()
	return PlanningDeps{
		Client: newGitClient(t),
		Reader: reader,
		Clock:  testClock(),
	}
}

// --- prepare: base resolution comes from the domain ------------------------

// TestWorkspacePrepareResolvesBaseFromDomain: the Target.Base handed to the
// service equals domain.ResolveEffectiveBase's answer for a STACKED change —
// the parent's feature branch, not the integration branch. Mutation: hard-code
// the base to "main" and this reddens (the resolved base is feat/parent).
func TestWorkspacePrepareResolvesBaseFromDomain(t *testing.T) {
	const childVer = "cccccccccccccccccccccccccccccccccccccccc"
	reader := &fakeReader{
		pin: mainPin(t),
		corpus: []StatusBlob{
			inProgressChangeBlob(20, "parent", "pppppppppppppppppppppppppppppppppppppppp", ""),
			inProgressChangeBlob(21, "child", childVer, "20"),
		},
		facts: domain.NewBranchFacts(map[string]bool{"feat/parent": true}),
	}
	svc := &fakeWorkspaceService{prepareWS: workspace.Workspace{Disposition: workspace.PrepareCreated}}
	deps := workspaceDepsFor(t, reader)
	repoDir := newMainModeRepo(t, nil).invocation

	res := WorkspacePrepare(context.Background(), deps, WorkspaceDeps{Service: svc},
		repoDir, WorkspaceIDRequest{ID: 21, Version: childVer})

	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied (reason %q msg %q)", res.Result, res.Reason, res.Message)
	}
	if len(svc.prepareCalls) != 1 {
		t.Fatalf("Prepare called %d times, want 1", len(svc.prepareCalls))
	}
	base := svc.prepareCalls[0].Target.Base
	if base.Kind != domain.BaseResolved || base.Branch != "feat/parent" {
		t.Errorf("Target.Base = %+v, want resolved/feat/parent (the domain's answer, not a hard-coded main)", base)
	}
	if svc.prepareCalls[0].Target.BaseRef != gitcli.RefName("refs/heads/feat/parent") {
		t.Errorf("Target.BaseRef = %q, want refs/heads/feat/parent", svc.prepareCalls[0].Target.BaseRef)
	}
}

// --- prepare: requires the claimed version ---------------------------------

// TestWorkspacePrepareRequiresClaimedVersion: a proposed (unclaimed) change and
// a stale version each refuse before any Git work — the service is never called.
func TestWorkspacePrepareRequiresClaimedVersion(t *testing.T) {
	repoDir := newMainModeRepo(t, nil).invocation

	t.Run("not in-progress", func(t *testing.T) {
		reader := &fakeReader{pin: mainPin(t), corpus: []StatusBlob{proposedChangeBlob(7, "widget", "v7")}}
		svc := &fakeWorkspaceService{}
		res := WorkspacePrepare(context.Background(), workspaceDepsFor(t, reader), WorkspaceDeps{Service: svc},
			repoDir, WorkspaceIDRequest{ID: 7, Version: "v7"})
		if res.Result != ResultInvalidState || res.Reason != ReasonWorkspaceNotInProgress {
			t.Fatalf("result=%q reason=%q, want invalid-state/not-in-progress", res.Result, res.Reason)
		}
		if len(svc.prepareCalls) != 0 {
			t.Errorf("service called on a not-in-progress refusal (%d calls)", len(svc.prepareCalls))
		}
	})

	t.Run("stale version", func(t *testing.T) {
		reader := &fakeReader{pin: mainPin(t), corpus: []StatusBlob{inProgressChangeBlob(7, "widget", "current-v", "")}}
		svc := &fakeWorkspaceService{}
		res := WorkspacePrepare(context.Background(), workspaceDepsFor(t, reader), WorkspaceDeps{Service: svc},
			repoDir, WorkspaceIDRequest{ID: 7, Version: "stale-v"})
		if res.Result != ResultContended || res.Reason != ReasonWorkspaceVersionMismatch {
			t.Fatalf("result=%q reason=%q, want contended/version-mismatch", res.Result, res.Reason)
		}
		if len(svc.prepareCalls) != 0 {
			t.Errorf("service called on a stale-version refusal (%d calls)", len(svc.prepareCalls))
		}
	})
}

// --- publish: head mismatch refuses without publishing ---------------------

// TestWorkspacePublishHeadMismatch: when the reinspected workspace head differs
// from the expected head, the operation refuses and never calls PublishHead.
func TestWorkspacePublishHeadMismatch(t *testing.T) {
	repoDir := newMainModeRepo(t, nil).invocation
	reader := &fakeReader{pin: mainPin(t), corpus: []StatusBlob{inProgressChangeBlob(7, "widget", "v7", "")}}
	svc := &fakeWorkspaceService{
		inspection: workspace.Inspection{Kind: workspace.StateReady, HeadCommit: gitcli.ObjectID("actualhead")},
	}
	res := WorkspacePublish(context.Background(), workspaceDepsFor(t, reader), WorkspaceDeps{Service: svc},
		repoDir, WorkspacePublishRequest{ID: 7, Head: "expectedhead"})

	if res.Result != ResultInvalidState || res.Reason != ReasonWorkspaceHeadMismatch {
		t.Fatalf("result=%q reason=%q, want invalid-state/head-mismatch", res.Result, res.Reason)
	}
	if len(svc.inspectCalls) != 1 {
		t.Errorf("Inspect called %d times, want 1", len(svc.inspectCalls))
	}
	if len(svc.publishCalls) != 0 {
		t.Errorf("PublishHead called on a head mismatch (%d calls)", len(svc.publishCalls))
	}
}

// --- publish: dispositions pass through 1:1 --------------------------------

// TestWorkspacePublishPassesThroughDispositions: each service publish
// disposition maps to a fixed protocol result, with the service's disposition
// carried through verbatim (no force, no retry).
func TestWorkspacePublishPassesThroughDispositions(t *testing.T) {
	repoDir := newMainModeRepo(t, nil).invocation
	const head = "abcdef0000000000000000000000000000000000"

	cases := []struct {
		name       string
		disp       workspace.PublishDisposition
		wantResult Result
	}{
		{"published", workspace.PublishPublished, ResultApplied},
		{"already-published", workspace.PublishAlreadyPublished, ResultNoOp},
		{"contended", workspace.PublishContended, ResultContended},
		{"unknown", workspace.PublishUnknown, ResultExternalFailed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reader := &fakeReader{pin: mainPin(t), corpus: []StatusBlob{inProgressChangeBlob(7, "widget", "v7", "")}}
			svc := &fakeWorkspaceService{
				inspection: workspace.Inspection{Kind: workspace.StateReady, HeadCommit: gitcli.ObjectID(head)},
				publishRes: workspace.PublishResult{Disposition: c.disp, Head: gitcli.ObjectID(head)},
			}
			res := WorkspacePublish(context.Background(), workspaceDepsFor(t, reader), WorkspaceDeps{Service: svc},
				repoDir, WorkspacePublishRequest{ID: 7, Head: head})

			if res.Result != c.wantResult {
				t.Errorf("result = %q, want %q", res.Result, c.wantResult)
			}
			if res.Disposition != string(c.disp) {
				t.Errorf("disposition = %q, want %q (carried verbatim)", res.Disposition, c.disp)
			}
			if len(svc.publishCalls) != 1 {
				t.Errorf("PublishHead called %d times, want 1", len(svc.publishCalls))
			}
		})
	}
}
