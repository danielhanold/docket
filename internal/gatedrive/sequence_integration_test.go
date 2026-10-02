//go:build integration

// Real-git, real-process fixtures shared by the integration tests that drive
// drives through real linked worktrees (sequence_race_integration_test.go's
// same-worktree generations).
//
// These helpers compose the real native process supervisor
// (internal/process.Service, the same seam supervisor_integration_test.go
// composes) driving real `/bin/sh -c 'exit N'` commands to genuine verdicts, with
// the real git seam (realGit) fingerprinting real linked worktrees.
//
// The command marker rides in each command's ARGV (as sh's $0), never in a shell
// comment: `exit N` is a shell builtin, so sh never execs it away and the marker
// survives on the sh process's own argv (exec-optimization-erases-the-process-marker).
//
// It reuses this package's real-git fixture helpers
// (fingerprint_integration_test.go: gitInit, writeFile, gitAdd, gitCommit, git) and
// the real-process fixtures (supervisor_integration_test.go: mustService, mustExe,
// skipUnlessSupported, reapSupervisors, stopAllRuns, advanceUntilTerminal,
// runDirsUnder). TestMain (supervisor_integration_test.go) already routes the
// supervisor re-exec role for the whole integration-tagged build.
package gatedrive

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/process"
	"github.com/danielhanold/docket/internal/testsupport"
)

// ---------------------------------------------------------------------------
// Real-git + real-process fixtures.
// ---------------------------------------------------------------------------

// realSeqDriver wires a driver over the REAL process service and the REAL git
// seam with the injected short slice (supervisor_integration_test.go's
// intSlice/intPoll), so slices are bounded by real wall-clock time against a live
// child and fingerprints are computed over a real worktree. It is newIntDriver with realGit{} instead of
// the fake stableGit.
func realSeqDriver(store *Store, svc *process.Service) *Driver {
	d := NewDriver(store, systemClock{}, svc, realGit{})
	d.slice = intSlice
	d.pollInterval = intPoll
	d.sleep = time.Sleep
	return d
}

// seedSeqRepo initializes a temporary git repository with one committed file and
// returns its path. Background maintenance is disabled so realGit's own read-only
// queries never spawn a gc/maintenance child that outlives the test (change 0373).
func seedSeqRepo(t *testing.T) string {
	t.Helper()
	repo := testsupport.TempDir(t)
	gitInit(t, repo)
	git(t, repo, "config", "gc.auto", "0")
	git(t, repo, "config", "maintenance.auto", "false")
	writeFile(t, repo, "seed.txt", "seed\n")
	gitAdd(t, repo, "seed.txt")
	gitCommit(t, repo, "seed")
	return repo
}

// addLinkedWorktree adds a linked worktree of repo on a fresh branch and returns
// its absolute path. The linked worktree shares repo's git common dir, so drives
// started under either worktree land in one shared store.
func addLinkedWorktree(t *testing.T, repo, name, branch string) string {
	t.Helper()
	wt := filepath.Join(testsupport.TempDir(t), name)
	git(t, repo, "worktree", "add", wt, "-b", branch)
	return wt
}

// commonDirOf returns the absolute git common directory a worktree resolves to.
// Two linked worktrees of one repository return the identical path — the shared
// common dir the drive/scope store roots under.
func commonDirOf(t *testing.T, worktree string) string {
	t.Helper()
	out := strings.TrimSpace(git(t, worktree, "rev-parse", "--path-format=absolute", "--git-common-dir"))
	abs, err := filepath.Abs(out)
	if err != nil {
		t.Fatalf("abs common dir: %v", err)
	}
	return abs
}

// seqPassCmd builds a distinguishable suite command whose verdict is a real
// process exit code (0 → PASSED). The marker is a real argv token (sh's $0), so it
// survives exec and is persisted on the drive record's Command.
func seqPassCmd(marker string) []string { return []string{"/bin/sh", "-c", "exit 0", marker} }

// realSeqStart builds a well-formed StartRequest bound to a real worktree, so its
// fingerprint is computed over real git bytes and its launch runs a real command.
func realSeqStart(worktree, branch, runRoot, changeID, taskID string, cmd []string) StartRequest {
	return StartRequest{
		RepoDir:             worktree,
		Worktree:            worktree,
		ChangeID:            changeID,
		TaskID:              taskID,
		Phase:               "build",
		Branch:              branch,
		Ref:                 "refs/heads/" + branch,
		Command:             cmd,
		Cwd:                 worktree,
		ConfigProvenance:    "config:build.test_command",
		Budget:              30 * time.Minute,
		EnvHash:             "seq-env",
		RunRoot:             runRoot,
		IdempotentSuiteGate: true,
	}
}

// driveSeqToTerminal starts req and drives it to a terminal outcome on the test's
// own goroutine (it may call t.Fatalf), asserting every invocation is
// slice-bounded via advanceUntilTerminal.
func driveSeqToTerminal(t *testing.T, d *Driver, req StartRequest) DriveDoc {
	t.Helper()
	doc, err := d.Start(req)
	if err != nil {
		t.Fatalf("start %v: %v", req.Command, err)
	}
	if doc.Outcome == WAITING {
		term, _ := advanceUntilTerminal(t, d, doc.DriveID, doc.Generation)
		return term
	}
	return doc
}

// ---------------------------------------------------------------------------
// Verification 2 (real git): baseline → RED → GREEN as a real-git sequence.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Change 0446 Task 10 — spec AC2: same-worktree generations over real git and
// the real process supervisor. The test itself,
// TestRaceIntegrationGatedriveSameWorktreeGenerations, lives in
// sequence_race_integration_test.go (race shard, change 0466).
// ---------------------------------------------------------------------------
