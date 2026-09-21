package app

import (
	"context"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/workspace"
	"strings"
	"testing"
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

// --- prepare: the feature branch is the recorded branch --------------------

// renamedBranchBlob is an in-progress record whose recorded branch: is a
// deliberately non-derived name — feature/renamed-head, NOT feat/<slug> — so a
// test can prove the operation consumes the recorded branch rather than
// reconstructing it from the slug.
func renamedBranchBlob(id int, slug, version, branch string) StatusBlob {
	src := lifecycleChange(id, slug, "in-progress")
	src = strings.Replace(src, "branch: feat/"+slug+"\n", "branch: "+branch+"\n", 1)
	return StatusBlob{
		Kind:     repository.KindChange,
		Location: repository.LocationActive,
		Path:     groomPath(id, slug),
		Version:  version,
		Data:     []byte(src),
	}
}

// --- prepare: requires the claimed version ---------------------------------

// --- publish: head mismatch refuses without publishing ---------------------

// --- publish: dispositions pass through 1:1 --------------------------------

// TestWorkspaceFenceRefusalMessageIsReasonAware pins the review fix (change 0441):
// the human message must be accurate per fence reason. A run-completed fence is a
// SUCCESSFUL closeout, so its message must NOT claim cancellation; a cancelled or
// superseded fence keeps the existing cancelled-or-superseded wording. The
// machine-readable Reason token rides through unchanged in every case.
func TestWorkspaceFenceRefusalMessageIsReasonAware(t *testing.T) {
	cancelled := workspaceFenceRefusal(3, ErrRunCancelled)
	if cancelled.Reason != "run-cancelled" {
		t.Fatalf("cancelled reason = %q, want run-cancelled", cancelled.Reason)
	}
	if cancelled.Message != "the run that owns this workspace was cancelled or superseded; publish nothing" {
		t.Fatalf("cancelled message unexpectedly changed: %q", cancelled.Message)
	}
	if superseded := workspaceFenceRefusal(3, ErrStaleRunEpoch); superseded.Reason != "stale-run-epoch" ||
		superseded.Message != cancelled.Message {
		t.Fatalf("superseded reason=%q message=%q, want stale-run-epoch + the cancelled-or-superseded wording", superseded.Reason, superseded.Message)
	}
	completed := workspaceFenceRefusal(3, ErrRunCompleted)
	if completed.Reason != "run-completed" {
		t.Fatalf("completed reason = %q, want run-completed (token must be unchanged)", completed.Reason)
	}
	if strings.Contains(completed.Message, "cancelled") || strings.Contains(completed.Message, "superseded") {
		t.Fatalf("run-completed message falsely claims cancellation: %q", completed.Message)
	}
	if !strings.Contains(completed.Message, "finished successfully") {
		t.Fatalf("run-completed message is not accurate for a successful closeout: %q", completed.Message)
	}
}
