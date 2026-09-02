//go:build integration

package app

import (
	"github.com/danielhanold/docket/internal/testsupport"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/reposetup"
)

// --- repository prepare scenarios (its own TestIntegrationRepoPrepare shard) -----
//
// These exercise the shared Step-0 `repository prepare` service end to end against
// the same bare-upstream + clone fixture the init/check/migrate shards use. Prepare
// is the only operation that attaches or fast-forwards the local `.docket` worktree,
// so its real-git correctness (worktree registration, the clean-behind fast-forward
// composed from the worktree primitives, and hooks disabling via the mechanism
// scripts/disable-worktree-hooks.sh documents) is provable only against real
// repositories — the clean-behind fast-forward in particular (prepareFastForward-
// Worktree) has no unit coverage of its multi-step remove/delete/re-add sequence.
//
// The crux these tests prove: the remote docket branch is a REAL MULTI-COMMIT chain
// (an init-seed parentless root plus a descendant), not a single seed commit. The
// earlier copied predicate in prepareAugment (`len(roots)==1 && roots[0]==tip`)
// misclassified every such chain as RootForeign — the root is not the tip once a
// descendant lands — and refused every real docket repository. prepareAugment now
// consumes the shared verifyMetadataOwnership verifier (the same one
// repository_check.go uses), which proves the init-seed root owns the chain
// (RootParentless) regardless of how many descendants sit on top. Under the old
// predicate these scenarios refuse with metadata-root-foreign; under the verifier
// they classify owned and reach their true disposition.
//
// Every refusal scenario asserts the `.docket` worktree bytes and tip are untouched
// AFTER the refusal (positive evidence), not merely that the disposition is refused:
// prepare never overwrites, resets, or stashes local content.

// newPrepareOrigin builds a repository whose remote carries the docket metadata
// topology as a MULTI-COMMIT chain: newInitRepo (fresh, docket-mode), a single init
// that publishes the parentless docket root and attaches the local `.docket`
// worktree, then one descendant commit pushed onto the docket branch so the origin
// (and the in-sync local `.docket`) hold a root != tip chain. This is deliberately
// not a single seed commit: a single-commit docket branch would pass even the old
// root-equals-tip predicate, so it would not prove the rewire. The chain is the
// healthy topology prepare converges on and the real shape every docket repo has.
func newPrepareOrigin(t *testing.T) *initRepo {
	t.Helper()
	r := newInitRepo(t, healthySetupYML, nil)
	if res := r.runInit(t); res.Result != ResultApplied {
		t.Fatalf("init for prepare fixture = %q (%s), want applied", res.Result, res.HumanText())
	}
	// Advance the docket branch by one descendant commit inside the attached
	// `.docket` worktree and push it, so origin/docket is a root != tip chain and the
	// local `.docket` is in sync at that chain tip.
	dot := filepath.Join(r.invocation, ".docket")
	writeRepoFile(t, dot, "docs/changes/BOARD.md", "# Board\n\nseeded chain commit\n")
	runGit(t, dot, "add", "-A")
	runGit(t, dot, "commit", "-q", "-m", "advance docket chain")
	runGit(t, dot, "push", "-q", "origin", "docket")
	if n := r.originDocketCommitCount(t); n < 2 {
		t.Fatalf("prepare fixture docket branch has %d commits, want a multi-commit chain (>= 2)", n)
	}
	return r
}

// originDocketCommitCount counts the commits reachable from the origin docket tip —
// the fixture is only a genuine chain (root != tip) when this is >= 2.
func (r *initRepo) originDocketCommitCount(t *testing.T) int {
	t.Helper()
	out := runGit(t, r.origin, "rev-list", "--count", "refs/heads/docket")
	n := 0
	for _, c := range out {
		if c < '0' || c > '9' {
			continue
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// runPrepareAt drives RunRepositoryPrepare against dir with a fresh isolated client.
func runPrepareAt(t *testing.T, dir string) RepositoryPrepareResult {
	t.Helper()
	client := newGitClient(t)
	return RunRepositoryPrepare(t.Context(), SetupDeps{Git: client, RepoDir: dir}, PrepareOptions{})
}

// freshClone makes a brand-new clone of the origin with no `.docket` worktree — the
// attach target: a healthy remote topology whose local metadata checkout is absent.
func (r *initRepo) freshClone(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(testsupport.TempDir(t), "prepare-clone")
	runGit(t, r.root, "clone", "-q", r.origin, dir)
	gitIdentity(t, dir)
	return dir
}

// advanceRemoteDocket pushes one commit onto the origin's docket branch from an
// independent clone (keeping the single parentless root) and returns the new tip. It
// is how a clone's local metadata branch is made strictly behind the remote.
func (r *initRepo) advanceRemoteDocket(t *testing.T, rel, content, msg string) string {
	t.Helper()
	w := filepath.Join(testsupport.TempDir(t), "docket-writer")
	runGit(t, r.root, "clone", "-q", r.origin, w)
	gitIdentity(t, w)
	runGit(t, w, "fetch", "-q", "origin", "docket")
	runGit(t, w, "checkout", "-q", "-b", "docket", "origin/docket")
	writeRepoFile(t, w, rel, content)
	runGit(t, w, "add", "-A")
	runGit(t, w, "commit", "-q", "-m", msg)
	runGit(t, w, "push", "-q", "origin", "docket")
	return runGit(t, r.origin, "rev-parse", "refs/heads/docket")
}

// commitInDocket commits a file inside the invocation clone's `.docket` worktree,
// advancing the LOCAL docket branch only (never the remote). It returns the new
// local tip. It is how a clone's local metadata branch is made ahead of the remote.
func (r *initRepo) commitInDocket(t *testing.T, rel, content, msg string) string {
	t.Helper()
	dot := filepath.Join(r.invocation, ".docket")
	writeRepoFile(t, dot, rel, content)
	runGit(t, dot, "add", "-A")
	runGit(t, dot, "commit", "-q", "-m", msg)
	return runGit(t, dot, "rev-parse", "HEAD")
}

// dotDocketHead resolves the `.docket` worktree HEAD in the invocation clone.
func (r *initRepo) dotDocketHead(t *testing.T) string {
	t.Helper()
	return runGit(t, filepath.Join(r.invocation, ".docket"), "rev-parse", "HEAD")
}

// prepareFinding reports whether a prepare result carries a finding with the code.
func prepareFinding(res RepositoryPrepareResult, code string) bool {
	for _, f := range res.Findings {
		if f.Code == code {
			return true
		}
	}
	return false
}

// TestIntegrationRepoPrepareMultiCommitChainClassifiesOwned is the load-bearing
// proof of the prepareAugment rewire: a healthy remote whose docket branch is a real
// multi-commit chain (root != tip) is classified OWNED (RootParentless) and prepared,
// never refused as a foreign metadata root. Under the old copied root-equals-tip
// predicate this refused with metadata-root-foreign; the shared verifier proves the
// init-seed root owns the whole chain.
func TestIntegrationRepoPrepareMultiCommitChainClassifiesOwned(t *testing.T) {
	r := newPrepareOrigin(t)
	if n := r.originDocketCommitCount(t); n < 2 {
		t.Fatalf("fixture docket branch has %d commits, want a genuine chain", n)
	}
	clone := r.freshClone(t)

	res := runPrepareAt(t, clone)
	if res.Disposition != PrepareDispositionApplied {
		t.Fatalf("prepare over a multi-commit docket chain = %q (%s), want applied", res.Disposition, res.HumanText())
	}
	if res.RepositoryState != string(reposetup.StateHealthy) {
		t.Errorf("RepositoryState = %q, want healthy (an owned chain is not foreign)", res.RepositoryState)
	}
	// The refusal the old root-equals-tip predicate produced must NOT appear.
	if prepareFinding(res, "metadata-root-foreign") {
		t.Errorf("a real multi-commit docket chain was misclassified foreign: %+v", res.Findings)
	}
}

// TestIntegrationRepoPrepareAttachesWorktree proves the attach effect: a healthy
// remote topology whose local `.docket` checkout is absent gets the worktree
// attached on the docket branch at the published (chain) tip, with hooks disabled.
func TestIntegrationRepoPrepareAttachesWorktree(t *testing.T) {
	r := newPrepareOrigin(t)
	clone := r.freshClone(t)
	dot := filepath.Join(clone, ".docket")
	if _, err := os.Stat(dot); !os.IsNotExist(err) {
		t.Fatalf("fresh clone already has a .docket worktree (err=%v); attach cannot be proven", err)
	}

	res := runPrepareAt(t, clone)
	if res.Disposition != PrepareDispositionApplied {
		t.Fatalf("prepare disposition = %q (%s), want applied", res.Disposition, res.HumanText())
	}
	if res.RepositoryState != string(reposetup.StateHealthy) {
		t.Errorf("RepositoryState = %q, want healthy", res.RepositoryState)
	}

	// The worktree is now attached on the docket branch at the remote tip.
	if _, err := os.Stat(dot); err != nil {
		t.Fatalf(".docket worktree was not attached: %v", err)
	}
	branch := runGit(t, dot, "rev-parse", "--abbrev-ref", "HEAD")
	if branch != "docket" {
		t.Errorf(".docket HEAD branch = %q, want docket", branch)
	}
	remoteTip := runGit(t, r.origin, "rev-parse", "refs/heads/docket")
	if head := runGit(t, dot, "rev-parse", "HEAD"); head != remoteTip {
		t.Errorf(".docket HEAD = %s, want the remote docket tip %s", head, remoteTip)
	}

	// Hooks disabled the docket way: a per-worktree core.hooksPath at an absolute,
	// existing directory (the disable-worktree-hooks.sh behavioral oracle).
	hooksPath := runGit(t, dot, "config", "--worktree", "core.hooksPath")
	if hooksPath == "" || !filepath.IsAbs(hooksPath) {
		t.Errorf("core.hooksPath = %q, want an absolute empty hooks dir", hooksPath)
	}
	if info, err := os.Stat(hooksPath); err != nil || !info.IsDir() {
		t.Errorf("core.hooksPath %q is not an existing directory (err=%v)", hooksPath, err)
	}
}

// TestIntegrationRepoPrepareFastForwardsBehind proves the clean-behind fast-forward:
// a clean `.docket` worktree whose local docket branch is a strict ancestor of the
// advanced remote tip is fast-forwarded to that tip, losing no registration and
// leaving hooks disabled.
func TestIntegrationRepoPrepareFastForwardsBehind(t *testing.T) {
	r := newPrepareOrigin(t)
	oldTip := r.dotDocketHead(t)
	newTip := r.advanceRemoteDocket(t, "board.md", "board v2\n", "advance docket board")
	if newTip == oldTip {
		t.Fatal("advancing the remote docket branch did not move the tip")
	}

	res := runPrepareAt(t, r.invocation)
	if res.Disposition != PrepareDispositionApplied {
		t.Fatalf("prepare disposition = %q (%s), want applied", res.Disposition, res.HumanText())
	}

	// The worktree AND the local branch now sit at the advanced remote tip.
	if head := r.dotDocketHead(t); head != newTip {
		t.Errorf(".docket HEAD = %s after fast-forward, want the advanced tip %s", head, newTip)
	}
	if local := runGit(t, r.invocation, "rev-parse", "refs/heads/docket"); local != newTip {
		t.Errorf("local docket branch = %s, want the advanced tip %s", local, newTip)
	}
	branch := runGit(t, filepath.Join(r.invocation, ".docket"), "rev-parse", "--abbrev-ref", "HEAD")
	if branch != "docket" {
		t.Errorf(".docket HEAD branch = %q after fast-forward, want docket", branch)
	}
	// The fast-forwarded content is present and hooks stay disabled.
	if got := mustReadFile(t, filepath.Join(r.invocation, ".docket", "board.md")); string(got) != "board v2\n" {
		t.Errorf("board.md = %q, want the advanced content", got)
	}
	hooksPath := runGit(t, filepath.Join(r.invocation, ".docket"), "config", "--worktree", "core.hooksPath")
	if hooksPath == "" || !filepath.IsAbs(hooksPath) {
		t.Errorf("core.hooksPath = %q after fast-forward, want an absolute empty hooks dir", hooksPath)
	}
}

// TestIntegrationRepoPrepareRefusesDirtyWorktree proves the dirty refusal: a
// modified tracked file in the `.docket` worktree refuses preparation, and the file
// is byte-identical afterward — prepare never reverts or resets local content. The
// tracked file is the chain's BOARD.md; the dirty guard fires after the (owned) root
// classification, so this also reaches the dirty disposition only because the chain
// classifies owned.
func TestIntegrationRepoPrepareRefusesDirtyWorktree(t *testing.T) {
	r := newPrepareOrigin(t)
	boardPath := filepath.Join(r.invocation, ".docket", "docs/changes/BOARD.md")
	dirtyBytes := "locally edited, uncommitted\n"
	if err := os.WriteFile(boardPath, []byte(dirtyBytes), 0o644); err != nil {
		t.Fatalf("dirty the tracked board file: %v", err)
	}
	tipBefore := r.dotDocketHead(t)

	res := runPrepareAt(t, r.invocation)
	if res.Disposition != PrepareDispositionRefused {
		t.Fatalf("prepare disposition = %q (%s), want refused", res.Disposition, res.HumanText())
	}
	if !prepareFinding(res, "metadata-worktree-dirty") {
		t.Errorf("findings %+v, want a metadata-worktree-dirty refusal", res.Findings)
	}

	// Positive evidence: the modified file bytes and the tip are untouched.
	if got := mustReadFile(t, boardPath); string(got) != dirtyBytes {
		t.Errorf("BOARD.md = %q after refusal, want the untouched dirty bytes %q", got, dirtyBytes)
	}
	if tipAfter := r.dotDocketHead(t); tipAfter != tipBefore {
		t.Errorf(".docket HEAD moved from %s to %s on a dirty refusal; it must be untouched", tipBefore, tipAfter)
	}
}

// TestIntegrationRepoPrepareRefusesAhead proves the ahead refusal: a local docket
// branch carrying a commit the remote does not have refuses, leaving the local
// branch and its committed content untouched.
func TestIntegrationRepoPrepareRefusesAhead(t *testing.T) {
	r := newPrepareOrigin(t)
	aheadTip := r.commitInDocket(t, "ahead.txt", "ahead of remote\n", "local-only docket commit")

	res := runPrepareAt(t, r.invocation)
	if res.Disposition != PrepareDispositionRefused {
		t.Fatalf("prepare disposition = %q (%s), want refused", res.Disposition, res.HumanText())
	}
	if !prepareFinding(res, "local-metadata-ahead") {
		t.Errorf("findings %+v, want a local-metadata-ahead refusal", res.Findings)
	}

	// The local branch and its content are untouched.
	if tip := r.dotDocketHead(t); tip != aheadTip {
		t.Errorf(".docket HEAD = %s after refusal, want the untouched ahead tip %s", tip, aheadTip)
	}
	if got := mustReadFile(t, filepath.Join(r.invocation, ".docket", "ahead.txt")); string(got) != "ahead of remote\n" {
		t.Errorf("ahead.txt = %q after refusal, want its untouched content", got)
	}
}

// TestIntegrationRepoPrepareRefusesDiverged proves the diverged refusal: a local
// docket branch and the remote docket branch that share a root but neither is an
// ancestor of the other refuses, leaving the local branch untouched.
func TestIntegrationRepoPrepareRefusesDiverged(t *testing.T) {
	r := newPrepareOrigin(t)
	// Remote advances one way; the local `.docket` branch advances a different way.
	r.advanceRemoteDocket(t, "remote.txt", "remote side\n", "remote docket commit")
	localTip := r.commitInDocket(t, "local.txt", "local side\n", "local docket commit")

	res := runPrepareAt(t, r.invocation)
	if res.Disposition != PrepareDispositionRefused {
		t.Fatalf("prepare disposition = %q (%s), want refused", res.Disposition, res.HumanText())
	}
	if !prepareFinding(res, "local-metadata-diverged") {
		t.Errorf("findings %+v, want a local-metadata-diverged refusal", res.Findings)
	}

	// The local branch is untouched (no implicit reconciliation).
	if tip := r.dotDocketHead(t); tip != localTip {
		t.Errorf(".docket HEAD = %s after refusal, want the untouched local tip %s", tip, localTip)
	}
	if got := mustReadFile(t, filepath.Join(r.invocation, ".docket", "local.txt")); string(got) != "local side\n" {
		t.Errorf("local.txt = %q after refusal, want its untouched content", got)
	}
}

// TestIntegrationRepoPrepareIsIdempotent proves re-run idempotence: a second prepare
// over an already-attached, current worktree is a no-op — no second worktree, no
// moved tip.
func TestIntegrationRepoPrepareIsIdempotent(t *testing.T) {
	r := newPrepareOrigin(t)
	clone := r.freshClone(t)

	first := runPrepareAt(t, clone)
	if first.Disposition != PrepareDispositionApplied {
		t.Fatalf("first prepare = %q (%s), want applied", first.Disposition, first.HumanText())
	}
	dot := filepath.Join(clone, ".docket")
	tipAfterFirst := runGit(t, dot, "rev-parse", "HEAD")

	second := runPrepareAt(t, clone)
	if second.Disposition != PrepareDispositionNoOp {
		t.Fatalf("second prepare = %q (%s), want no-op", second.Disposition, second.HumanText())
	}
	if second.RepositoryState != string(reposetup.StateHealthy) {
		t.Errorf("second prepare RepositoryState = %q, want healthy", second.RepositoryState)
	}

	// Exactly one `.docket` worktree registration, and the tip did not move.
	wts := runGit(t, clone, "worktree", "list", "--porcelain")
	if got := strings.Count(wts, filepath.Join(clone, ".docket")); got != 1 {
		t.Errorf(".docket worktree registered %d times, want exactly one:\n%s", got, wts)
	}
	if tip := runGit(t, dot, "rev-parse", "HEAD"); tip != tipAfterFirst {
		t.Errorf(".docket HEAD moved from %s to %s on an idempotent re-run", tipAfterFirst, tip)
	}
}
