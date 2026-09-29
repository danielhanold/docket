//go:build integration

package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/testsupport"
)

// TestIntegrationGateArmEpochlessResumeEndToEnd0382 reproduces change 0382's resumed run (change 0463).
// The change was claimed by an UNARMED first dispatch, so no run epoch exists. The
// resume arm must print `run-started <key> <epoch> <dispatch-context>`. Parsed
// positionally (as AGENTS.md tells a parent), the <epoch> is admitted by the real
// epoch launch gate for the resumed worktree and recorded on its execution slot.
// The misrouted 0382 call (the dispatch context presented as the epoch) is refused
// with the named unknown-run-id. The resume inspect path uses the raw temp
// spelling and the start uses the symlink-resolved one (Review Focus 1).
func TestIntegrationGateArmEpochlessResumeEndToEnd0382(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	worktree, err := filepath.EvalSymlinks(repoDir)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	common, err := gateGitCommonDir(repoDir)
	if err != nil {
		t.Fatalf("gateGitCommonDir: %v", err)
	}
	if _, _, found, ferr := FindEpochByChange(repoDir, "5"); ferr != nil || found {
		t.Fatalf("the unarmed claim must leave no epoch: found=%v err=%v", found, ferr)
	}

	// Arm the resume through the REAL outer-scope store, as production composes it.
	reader := &fakeReader{pin: mainPin(t), corpus: []StatusBlob{inProgressChangeBlob(5, "epsilon", "v5", "")}}
	deps := workspaceDepsFor(t, reader)
	wdeps := WorkspaceDeps{Service: resumeInspectService(repoDir)}
	store := gatedrive.OpenStore(common)
	arm := RunGateBefore(context.Background(), deps, wdeps, GateScopeDeps{Prepare: store.PrepareScope}, repoDir, "implement-next", 5)
	if !arm.Armed {
		t.Fatalf("the epochless resume must arm: %q", arm.HumanText())
	}

	fields := strings.Fields(strings.SplitN(arm.HumanText(), "\n", 2)[0])
	if len(fields) != 4 || fields[0] != "run-started" {
		t.Fatalf("armed line %q must be `run-started <key> <epoch> <dispatch-context>`", arm.HumanText())
	}
	key, epoch, dispatchCtx := fields[1], fields[2], fields[3]
	if key != arm.Key || epoch != arm.Epoch || dispatchCtx != arm.DispatchContext {
		t.Fatalf("positional fields (%q,%q,%q) disagree with the result (%q,%q,%q)", key, epoch, dispatchCtx, arm.Key, arm.Epoch, arm.DispatchContext)
	}

	svc, res, reason := NewTaskGateDriveService(common, "/bin/true", buildEffWithMaxAttempts("go test ./...", 4), []string{"/bin/echo", "ok"})
	if svc == nil {
		t.Fatalf("task service: %s (%s)", res, reason)
	}
	req := GateDriveStartRequest{
		RepoDir: common, Worktree: worktree, ChangeID: "5", TaskID: "task-6", Phase: "build",
		RunRoot: testsupport.TempDir(t), Cwd: worktree, GateContext: dispatchCtx, RunEpochID: epoch,
	}

	// The misrouted 0382 call: the dispatch context presented as the run epoch.
	bad := req
	bad.RunEpochID = dispatchCtx
	if _, berr := svc.engine.Admit(svc.startRequest(bad)); berr == nil {
		t.Fatalf("the dispatch context must never admit as a run epoch")
	} else if r, why := mapDriveFailure(berr); r != ResultInvalidInput || why != ReasonUnknownRunEpoch {
		t.Fatalf("misrouted epoch refused as (%s, %q), want (invalid-input, unknown-run-id)", r, why)
	}

	// The correctly parsed epoch is admitted by the real launch gate.
	ticket, aerr := svc.engine.Admit(svc.startRequest(req))
	if aerr != nil {
		r, why := mapDriveFailure(aerr)
		t.Fatalf("the parsed epoch must admit for the resumed worktree, got (%s, %q): %v", r, why, aerr)
	}
	t.Cleanup(func() { _ = svc.engine.AbandonAdmission(ticket) })
	slot, _, lerr := store.LoadWorktreeExecution(worktree)
	if lerr != nil {
		t.Fatalf("LoadWorktreeExecution: %v", lerr)
	}
	if slot.RunEpochID != epoch {
		t.Fatalf("worktree slot RunEpochID = %q, want the armed epoch %q", slot.RunEpochID, epoch)
	}
}
