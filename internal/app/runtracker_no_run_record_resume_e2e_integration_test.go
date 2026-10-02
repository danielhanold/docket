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
// resume start must print `run-started <key> <run-context>`. Parsed
// positionally (as AGENTS.md tells a parent), the <run-context> links a gate start
// for the resumed worktree, which admission takes under the worktree lock alone:
// change 0491 deleted the run launch check and retired the run tracker's run id, so
// the started line is two tokens and the misrouted 0382 call (the run context
// presented as a third token) has no slot left to misroute through. The resume inspect path uses the raw temp
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
	if len(fields) != 3 || fields[0] != "run-started" {
		t.Fatalf("started line %q must be `run-started <key> <run-context>`", start.HumanText())
	}
	key, runCtx := fields[1], fields[2]
	if key != start.Key || runCtx != start.RunContext {
		t.Fatalf("positional fields (%q,%q) disagree with the result (%q,%q)", key, runCtx, start.Key, start.RunContext)
	}

	svc, res, reason := NewBuildGateDriveService(common, "/bin/true", buildEffWithMaxAttempts("/bin/echo ok", 4))
	if svc == nil {
		t.Fatalf("build service: %s (%s)", res, reason)
	}
	req := GateDriveStartRequest{
		RepoDir: common, Worktree: worktree, ChangeID: "5", Phase: "build",
		RunRoot: testsupport.TempDir(t), Cwd: worktree, RunContext: runCtx,
	}

	// A gate start carrying the parsed run context is admitted for the resumed worktree.
	ticket, aerr := svc.engine.Admit(svc.startRequest(req))
	if aerr != nil {
		r, why := mapDriveFailure(aerr)
		t.Fatalf("the parsed run context must admit for the resumed worktree, got (%s, %q): %v", r, why, aerr)
	}
	t.Cleanup(func() { _ = svc.engine.AbandonAdmission(ticket) })
}
