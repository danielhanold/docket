//go:build integration

package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/reposetup"
)

// --- local-copy relationship scenarios (TestIntegrationRepoSyncCheck shard) --------
//
// Every typed metadata write pushes from a detached worktree and leaves the local
// .docket copy clean and behind. These prove check keys on the real ancestry
// relationship: behind is healthy with no finding; ahead, diverged, and dirty each
// report exactly their own finding.

func checkCodes(res RepositoryCheckResult) map[string]bool {
	got := map[string]bool{}
	for _, f := range res.Findings {
		got[f.Code] = true
	}
	return got
}

func TestIntegrationRepoSyncCheckBehindIsHealthy(t *testing.T) {
	r := newHealthyRepo(t)
	oldTip := r.dotDocketHead(t)
	newTip := r.advanceRemoteDocket(t, "notes/advance.txt", "advanced\n", "advance docket")

	res := r.runCheck(t)
	if res.RepositoryState != string(reposetup.StateHealthy) || len(res.Findings) != 0 || res.CheckExitCode() != 0 {
		t.Fatalf("behind-only .docket: state=%q findings=%+v exit=%d, want healthy, none, 0",
			res.RepositoryState, res.Findings, res.CheckExitCode())
	}
	// Positive evidence the copy really was behind (check is read-only).
	if res.Revisions["local-metadata"] != oldTip || res.Revisions["remote-metadata"] != newTip {
		t.Fatalf("revisions = %v, want local %s behind remote %s", res.Revisions, oldTip, newTip)
	}
	if head := r.dotDocketHead(t); head != oldTip {
		t.Fatalf("check moved .docket from %s to %s", oldTip, head)
	}

	ct := r.runConfigureTests(t)
	if ct.Result == ResultInvalidState {
		t.Fatalf("configure-tests refused a behind-only repository: %s", ct.HumanText())
	}
}

func TestIntegrationRepoSyncCheckAheadReportsAheadOnly(t *testing.T) {
	r := newHealthyRepo(t)
	r.commitInDocket(t, "ahead.txt", "ahead\n", "local-only docket commit")
	res := r.runCheck(t)
	got := checkCodes(res)
	if !got["local-metadata-ahead"] || got["local-metadata-diverged"] || got["metadata-worktree-dirty"] || res.CheckExitCode() != 1 {
		t.Fatalf("ahead: findings=%+v exit=%d, want local-metadata-ahead only, exit 1", res.Findings, res.CheckExitCode())
	}
}

func TestIntegrationRepoSyncCheckDivergedReportsDivergedOnly(t *testing.T) {
	r := newHealthyRepo(t)
	r.advanceRemoteDocket(t, "notes/remote.txt", "remote\n", "remote side")
	r.commitInDocket(t, "local.txt", "local\n", "local side")
	res := r.runCheck(t)
	got := checkCodes(res)
	if !got["local-metadata-diverged"] || got["local-metadata-ahead"] || got["metadata-worktree-dirty"] {
		t.Fatalf("diverged: findings=%+v, want local-metadata-diverged only", res.Findings)
	}
}

func TestIntegrationRepoSyncCheckDirtyBehindReportsDirtyOnly(t *testing.T) {
	r := newHealthyRepo(t)
	r.advanceRemoteDocket(t, "notes/advance.txt", "advanced\n", "advance docket")
	if err := os.WriteFile(filepath.Join(r.invocation, ".docket", "scratch.txt"), []byte("dirt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := r.runCheck(t)
	got := checkCodes(res)
	if !got["metadata-worktree-dirty"] || got["local-metadata-diverged"] || got["local-metadata-ahead"] {
		t.Fatalf("dirty+behind: findings=%+v, want metadata-worktree-dirty only", res.Findings)
	}
}
