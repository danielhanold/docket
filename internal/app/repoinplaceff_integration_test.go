//go:build integration

package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/reposetup"
	"github.com/danielhanold/docket/internal/testsupport"
)

// --- in-place fast-forward scenarios (TestIntegrationRepoInPlaceFF shard) ----------
//
// prepare fast-forwards a clean, behind .docket inside the existing worktree:
// nothing removed, compare-and-swap on the router's observed tip, no repository
// hook, refuse rather than discard. Each scenario starts from a check-healthy
// repository whose remote docket branch is one commit ahead of .docket.

func newBehindHealthyRepo(t *testing.T) (r *initRepo, oldTip, newTip string) {
	t.Helper()
	r = newHealthyRepo(t)
	oldTip = r.dotDocketHead(t)
	newTip = r.advanceRemoteDocket(t, "notes/advance.txt", "advanced\n", "advance docket")
	return r, oldTip, newTip
}

func requireFastForwarded(t *testing.T, r *initRepo, newTip string) {
	t.Helper()
	res := runPrepareAt(t, r.invocation)
	if res.Disposition != PrepareDispositionApplied {
		t.Fatalf("prepare = %q (%s), want applied", res.Disposition, res.HumanText())
	}
	if head := r.dotDocketHead(t); head != newTip {
		t.Fatalf(".docket HEAD = %s, want %s", head, newTip)
	}
}

func requireCheckHealthy(t *testing.T, r *initRepo) {
	t.Helper()
	res := r.runCheck(t)
	if res.RepositoryState != string(reposetup.StateHealthy) || res.CheckExitCode() != 0 {
		t.Fatalf("check after prepare: state=%q findings=%+v exit=%d", res.RepositoryState, res.Findings, res.CheckExitCode())
	}
}

func writeHook(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestIntegrationRepoInPlaceFFLockedWorktree(t *testing.T) {
	r, _, newTip := newBehindHealthyRepo(t)
	dot := filepath.Join(r.invocation, ".docket")
	runGit(t, r.invocation, "worktree", "lock", dot)
	requireFastForwarded(t, r, newTip)
	if list := runGit(t, r.invocation, "worktree", "list", "--porcelain"); !strings.Contains(list, "\nlocked") {
		t.Fatalf(".docket lock lost:\n%s", list)
	}
}

// TestIntegrationRepoInPlaceFFRepositoryHooksNeverRun installs failing, file-writing
// post-checkout and post-merge hooks through the repository's shared hooks path.
// A remove/re-add (or a porcelain merge) would run them.
func TestIntegrationRepoInPlaceFFRepositoryHooksNeverRun(t *testing.T) {
	r, _, newTip := newBehindHealthyRepo(t)
	hooks := filepath.Join(testsupport.TempDir(t), "shared-hooks")
	sentinel := filepath.Join(testsupport.TempDir(t), "hook-ran")
	for _, h := range []string{"post-checkout", "post-merge"} {
		writeHook(t, hooks, h, "echo "+h+" >> '"+sentinel+"'\nexit 1\n")
	}
	runGit(t, r.invocation, "config", "core.hooksPath", hooks)
	requireFastForwarded(t, r, newTip)
	if b, err := os.ReadFile(sentinel); err == nil {
		t.Fatalf("a repository hook ran: %q", b)
	}
	requireCheckHealthy(t, r)
}

func TestIntegrationRepoInPlaceFFIgnoredFileAndDirectoryIdentity(t *testing.T) {
	r, _, newTip := newBehindHealthyRepo(t)
	dot := filepath.Join(r.invocation, ".docket")
	exclude := filepath.Join(r.invocation, ".git", "info", "exclude")
	f, err := os.OpenFile(exclude, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("*.local\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	sentinel := filepath.Join(dot, "sentinel.local")
	if err := os.WriteFile(sentinel, []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(dot)
	if err != nil {
		t.Fatal(err)
	}
	requireFastForwarded(t, r, newTip)
	after, err := os.Stat(dot)
	if err != nil || !os.SameFile(before, after) {
		t.Fatalf(".docket directory was removed and re-created (err=%v)", err)
	}
	if b, err := os.ReadFile(sentinel); err != nil || string(b) != "keep\n" {
		t.Fatalf("ignored file lost: %q, %v", b, err)
	}
	requireCheckHealthy(t, r)
}

// TestIntegrationRepoInPlaceFFStaleObservedTipRefuses hands the execute step the
// tip the router saw, after a commit landed in .docket: it must refuse and keep it.
func TestIntegrationRepoInPlaceFFStaleObservedTipRefuses(t *testing.T) {
	r, oldTip, newTip := newBehindHealthyRepo(t)
	runGit(t, r.invocation, "fetch", "-q", "origin", "docket")
	runGit(t, r.invocation, "cat-file", "-e", newTip+"^{commit}") // premise: target object is local
	client := newGitClient(t)
	_, sc, err := GatherSetupFacts(context.Background(), SetupDeps{Git: client, RepoDir: r.invocation}, true)
	if err != nil {
		t.Fatal(err)
	}
	moved := r.commitInDocket(t, "late.txt", "late\n", "commit after routing")

	v := prepareVerdict{disposition: PrepareDispositionApplied, action: prepareActionFastForward, targetRev: newTip, observedTip: oldTip}
	if err := prepareExecute(context.Background(), client, sc, v); err == nil {
		t.Fatal("fast-forward from a stale observed tip succeeded; want refusal")
	}
	if got := runGit(t, r.invocation, "rev-parse", "refs/heads/docket"); got != moved {
		t.Fatalf("local docket = %s, want the late commit %s kept", got, moved)
	}
	if b := mustReadFile(t, filepath.Join(r.invocation, ".docket", "late.txt")); string(b) != "late\n" {
		t.Fatalf("late.txt = %q", b)
	}
}

// TestIntegrationRepoInPlaceFFHooksOffMissing removes the per-worktree hooks-off
// setting; the forced empty hooks dir must still keep the shared hooks silent inside
// .docket. Git runs a hook from the worktree root, so each hook records $PWD, and the
// primary's own fetches (which do run reference-transaction) are told apart from the
// fast-forward. post-index-change is deliberately NOT installed: prepare's own
// `git status` clean probe inside .docket may write the index and would fire it
// before the fast-forward starts, which is not what this test measures.
func TestIntegrationRepoInPlaceFFHooksOffMissing(t *testing.T) {
	r, _, newTip := newBehindHealthyRepo(t)
	dot := filepath.Join(r.invocation, ".docket")
	hooks := filepath.Join(testsupport.TempDir(t), "shared-hooks")
	sentinel := filepath.Join(testsupport.TempDir(t), "hook-ran")
	for _, h := range []string{"reference-transaction", "post-checkout", "post-merge"} {
		writeHook(t, hooks, h, "echo \""+h+" $PWD\" >> '"+sentinel+"'\nexit 0\n")
	}
	runGit(t, r.invocation, "config", "core.hooksPath", hooks)
	runGit(t, dot, "config", "--worktree", "--unset", "core.hooksPath")

	dotReal, err := filepath.EvalSymlinks(dot)
	if err != nil {
		t.Fatal(err)
	}
	inDocket := func(line string) bool {
		return strings.HasSuffix(line, " "+dot) || strings.HasSuffix(line, " "+dotReal)
	}

	// Premise: with hooks-off missing, a ref update inside .docket does run the hook.
	runGit(t, dot, "update-ref", "refs/docket-test/premise", "HEAD")
	runGit(t, dot, "update-ref", "-d", "refs/docket-test/premise")
	premise, _ := os.ReadFile(sentinel)
	ran := false
	for _, line := range strings.Split(strings.TrimSpace(string(premise)), "\n") {
		ran = ran || inDocket(line)
	}
	if !ran {
		t.Fatalf("premise: the shared hook did not run inside .docket:\n%s", premise)
	}
	if err := os.Remove(sentinel); err != nil {
		t.Fatal(err)
	}

	requireFastForwarded(t, r, newTip)
	b, _ := os.ReadFile(sentinel)
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if inDocket(line) {
			t.Fatalf("a repository hook ran inside .docket during prepare: %q", line)
		}
	}
}

func TestIntegrationRepoInPlaceFFIdempotent(t *testing.T) {
	r, _, newTip := newBehindHealthyRepo(t)
	requireFastForwarded(t, r, newTip)
	second := runPrepareAt(t, r.invocation)
	if second.Disposition != PrepareDispositionNoOp {
		t.Fatalf("second prepare = %q (%s), want no-op", second.Disposition, second.HumanText())
	}
	hooksPath := runGit(t, filepath.Join(r.invocation, ".docket"), "config", "--worktree", "core.hooksPath")
	if hooksPath == "" {
		t.Fatal("per-worktree hooks-off setting was removed")
	}
	requireCheckHealthy(t, r)
}

// TestIntegrationRepoInPlaceFFInterruptedReadsDirty simulates an interruption after
// the branch moved but before the tree did: it must read dirty, never clean, and
// prepare must refuse without removing anything.
func TestIntegrationRepoInPlaceFFInterruptedReadsDirty(t *testing.T) {
	r, oldTip, newTip := newBehindHealthyRepo(t)
	dot := filepath.Join(r.invocation, ".docket")
	runGit(t, r.invocation, "fetch", "-q", "origin", "docket")
	runGit(t, dot, "update-ref", "refs/heads/docket", newTip, oldTip)

	res := r.runCheck(t)
	if !checkCodes(res)["metadata-worktree-dirty"] {
		t.Fatalf("interrupted fast-forward: findings %+v, want metadata-worktree-dirty", res.Findings)
	}
	pr := runPrepareAt(t, r.invocation)
	if pr.Disposition != PrepareDispositionRefused {
		t.Fatalf("prepare = %q, want refused", pr.Disposition)
	}
	if _, err := os.Stat(dot); err != nil {
		t.Fatalf(".docket removed: %v", err)
	}
}

// removeDocketWorktree deregisters a clean .docket, leaving the local branch.
func removeDocketWorktree(t *testing.T, r *initRepo) {
	t.Helper()
	runGit(t, r.invocation, "worktree", "remove", filepath.Join(r.invocation, ".docket"))
}

func TestIntegrationRepoInPlaceFFAttachBehindAdvancesBranch(t *testing.T) {
	r, _, newTip := newBehindHealthyRepo(t)
	removeDocketWorktree(t, r)
	res := runPrepareAt(t, r.invocation)
	if res.Disposition != PrepareDispositionApplied {
		t.Fatalf("prepare = %q (%s), want applied", res.Disposition, res.HumanText())
	}
	if head := r.dotDocketHead(t); head != newTip {
		t.Fatalf(".docket attached at %s, want the remote tip %s (never the stale local tip)", head, newTip)
	}
	if local := runGit(t, r.invocation, "rev-parse", "refs/heads/docket"); local != newTip {
		t.Fatalf("local docket = %s, want %s", local, newTip)
	}
	requireCheckHealthy(t, r)
}

func TestIntegrationRepoInPlaceFFAttachAheadRefuses(t *testing.T) {
	r := newHealthyRepo(t)
	aheadTip := r.commitInDocket(t, "ahead.txt", "ahead\n", "local-only docket commit")
	removeDocketWorktree(t, r)
	res := runPrepareAt(t, r.invocation)
	if res.Disposition != PrepareDispositionRefused || !prepareFinding(res, "local-metadata-ahead") {
		t.Fatalf("prepare = %q %+v, want refused local-metadata-ahead", res.Disposition, res.Findings)
	}
	if local := runGit(t, r.invocation, "rev-parse", "refs/heads/docket"); local != aheadTip {
		t.Fatalf("local docket = %s, want untouched %s", local, aheadTip)
	}
	if _, err := os.Stat(filepath.Join(r.invocation, ".docket")); !os.IsNotExist(err) {
		t.Fatalf(".docket attached on a refusal (err=%v)", err)
	}
}

func TestIntegrationRepoInPlaceFFAttachDivergedRefuses(t *testing.T) {
	r := newHealthyRepo(t)
	r.advanceRemoteDocket(t, "notes/remote.txt", "remote\n", "remote side")
	localTip := r.commitInDocket(t, "local.txt", "local\n", "local side")
	removeDocketWorktree(t, r)
	res := runPrepareAt(t, r.invocation)
	if res.Disposition != PrepareDispositionRefused || !prepareFinding(res, "local-metadata-diverged") {
		t.Fatalf("prepare = %q %+v, want refused local-metadata-diverged", res.Disposition, res.Findings)
	}
	if local := runGit(t, r.invocation, "rev-parse", "refs/heads/docket"); local != localTip {
		t.Fatalf("local docket = %s, want untouched %s", local, localTip)
	}
	if _, err := os.Stat(filepath.Join(r.invocation, ".docket")); !os.IsNotExist(err) {
		t.Fatalf(".docket attached on a refusal (err=%v)", err)
	}
}
