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

// TestIntegrationRunStartNoRunRecordResumeEndToEnd0382 reproduces change 0382's resumed run (change 0463).
// The change was claimed by an UNTRACKED first dispatch, so no run exists. The
// resume start must print `run-started <key> <run-id> <run-context>`. Parsed
// positionally (as AGENTS.md tells a parent), the <run-id> is admitted by the real
// run launch gate for the resumed worktree and recorded on its execution slot.
// The misrouted 0382 call (the run context presented as the run id) is refused
// with the named unknown-run-id. The resume inspect path uses the raw temp
// spelling and the start uses the symlink-resolved one (Review Focus 1).
func TestIntegrationRunStartNoRunRecordResumeEndToEnd0382(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	worktree, err := filepath.EvalSymlinks(repoDir)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	common, err := runTrackerGitCommonDir(repoDir)
	if err != nil {
		t.Fatalf("runTrackerGitCommonDir: %v", err)
	}
	if _, _, found, ferr := FindRunByChange(repoDir, "5"); ferr != nil || found {
		t.Fatalf("the untracked claim must leave no run: found=%v err=%v", found, ferr)
	}

	// Start the resume through the REAL outer-scope store, as production composes it.
	reader := &fakeReader{pin: mainPin(t), corpus: []StatusBlob{inProgressChangeBlob(5, "epsilon", "v5", "")}}
	deps := workspaceDepsFor(t, reader)
	wdeps := WorkspaceDeps{Service: resumeInspectService(repoDir)}
	store := gatedrive.OpenStore(common)
	start := RunStart(context.Background(), deps, wdeps, RunTrackerScopeDeps{Prepare: store.PrepareScope}, repoDir, "implement-next", 5)
	if !start.Started {
		t.Fatalf("the no-run-record resume must start: %q", start.HumanText())
	}

	fields := strings.Fields(strings.SplitN(start.HumanText(), "\n", 2)[0])
	if len(fields) != 4 || fields[0] != "run-started" {
		t.Fatalf("started line %q must be `run-started <key> <run-id> <run-context>`", start.HumanText())
	}
	key, runID, runCtx := fields[1], fields[2], fields[3]
	if key != start.Key || runID != start.RunID || runCtx != start.RunContext {
		t.Fatalf("positional fields (%q,%q,%q) disagree with the result (%q,%q,%q)", key, runID, runCtx, start.Key, start.RunID, start.RunContext)
	}

	svc, res, reason := NewBuildGateDriveService(common, "/bin/true", buildEffWithMaxAttempts("/bin/echo ok", 4))
	if svc == nil {
		t.Fatalf("build service: %s (%s)", res, reason)
	}
	req := GateDriveStartRequest{
		RepoDir: common, Worktree: worktree, ChangeID: "5", Phase: "build",
		RunRoot: testsupport.TempDir(t), Cwd: worktree, RunContext: runCtx, RunID: runID,
	}

	// The misrouted 0382 call: the run context presented as the run id.
	bad := req
	bad.RunID = runCtx
	if _, berr := svc.engine.Admit(svc.startRequest(bad)); berr == nil {
		t.Fatalf("the run context must never admit as a run id")
	} else if r, why := mapDriveFailure(berr); r != ResultInvalidInput || why != ReasonUnknownRunID {
		t.Fatalf("misrouted run id refused as (%s, %q), want (invalid-input, unknown-run-id)", r, why)
	}

	// The correctly parsed run id is admitted by the real launch gate.
	ticket, aerr := svc.engine.Admit(svc.startRequest(req))
	if aerr != nil {
		r, why := mapDriveFailure(aerr)
		t.Fatalf("the parsed run id must admit for the resumed worktree, got (%s, %q): %v", r, why, aerr)
	}
	t.Cleanup(func() { _ = svc.engine.AbandonAdmission(ticket) })
	slot, _, lerr := store.LoadWorktreeExecution(worktree)
	if lerr != nil {
		t.Fatalf("LoadWorktreeExecution: %v", lerr)
	}
	if slot.RunID != runID {
		t.Fatalf("worktree slot RunID = %q, want the started run id %q", slot.RunID, runID)
	}
}
