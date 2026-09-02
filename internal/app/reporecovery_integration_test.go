//go:build integration

package app

import (
	"context"
	"errors"
	"github.com/danielhanold/docket/internal/testsupport"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
)

// This is the real-Git interruption / response-loss / partial-state recovery
// shard (prefix TestIntegrationRepoRecovery). Each test scripts its own legacy
// repository (bare origin + writer + invocation clones) via the established
// internal/app harness (newInitRepo, runMigrate/runMigrateWithHooks, the origin*
// oracles), crashes a run at a chosen boundary through the generalized setupHooks
// seam, and proves the durable remote state is the only recovery journal: no
// path rolls back a published branch, deletes a foreign ref, force-pushes, or
// overwrites a moved branch, and every branch decision keys on a re-read remote
// postcondition. There is one test function per spec interruption boundary; a
// boundary's sub-cases are t.Run subtests.

// --- fixture helpers ---------------------------------------------------------

// gitEnvOut runs git -C dir with extra environment (e.g. a private
// GIT_INDEX_FILE) and returns trimmed combined output, failing the test on a
// non-zero exit. It is the one path that needs a custom env, so it is not folded
// into runGit.
func gitEnvOut(t *testing.T, dir string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(cmd.Environ(), env...)
	if kv := backgroundOffGitEnv(); kv != "" {
		cmd.Env = append(cmd.Env, kv)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git -C %s %s: %v\n%s", dir, strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// seedBashDocket publishes an orphan metadata branch the way the legacy Bash
// migration did: a single parentless commit whose tree is EXACTLY the copy-set
// (docs/changes, docs/adrs, docs/superpowers/specs) with NO docket receipt
// trailers. It composes the tree with `read-tree --prefix` into a private index
// — the same composition the native seed uses — so a native re-run recomposing
// the seed from the same source yields a byte-identical tree and adopts it via
// exact legacy-equivalent tree equality, no receipt required. It returns the
// published seed commit id.
func (r *initRepo) seedBashDocket(t *testing.T) string {
	t.Helper()
	idxDir := testsupport.TempDir(t)
	env := []string{"GIT_INDEX_FILE=" + filepath.Join(idxDir, "seed-index")}
	gitEnvOut(t, r.writer, env, "read-tree", "--empty")
	gitEnvOut(t, r.writer, env, "read-tree", "--prefix=docs/changes/", "main:docs/changes")
	gitEnvOut(t, r.writer, env, "read-tree", "--prefix=docs/adrs/", "main:docs/adrs")
	gitEnvOut(t, r.writer, env, "read-tree", "--prefix=docs/superpowers/specs/", "main:docs/superpowers/specs")
	tree := gitEnvOut(t, r.writer, env, "write-tree")
	commit := gitEnvOut(t, r.writer, env, "commit-tree", tree, "-m", "bash migration seed")
	runGit(t, r.writer, "push", "-q", "origin", commit+":refs/heads/docket")
	return commit
}

// prunePrimaryOnOrigin advances the origin integration branch by the legacy
// prune itself (remove the active dir, board, and README), simulating a repo
// whose remote migration completed while the local attachment did not.
func (r *initRepo) prunePrimaryOnOrigin(t *testing.T) {
	t.Helper()
	runGit(t, r.writer, "checkout", "-q", "main")
	runGit(t, r.writer, "rm", "-rq", "docs/changes/active", "docs/changes/BOARD.md", "docs/changes/README.md")
	runGit(t, r.writer, "commit", "-q", "-m", "prune legacy planning surface")
	runGit(t, r.writer, "push", "-q", "origin", "main")
}

// advanceIntegration adds one file at rel on the origin integration branch and
// returns nothing — used to move the pinned source between a seed and a prune.
func (r *initRepo) advanceIntegration(t *testing.T, rel, content string) {
	t.Helper()
	runGit(t, r.writer, "checkout", "-q", "main")
	writeRepoFile(t, r.writer, rel, content)
	runGit(t, r.writer, "add", "--", rel)
	runGit(t, r.writer, "commit", "-q", "-m", "advance integration: "+rel)
	runGit(t, r.writer, "push", "-q", "origin", "main")
}

// discoverRepo resolves the invocation clone's canonical repository for a direct
// call into an unexported service helper.
func (r *initRepo) discoverRepo(t *testing.T, client *gitcli.Client) gitcli.Repository {
	t.Helper()
	repo, err := client.Discover(context.Background(), gitcli.DiscoverOptions{InvocationPath: r.invocation})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	return repo
}

// ownedTempWorktrees lists the BASE NAMES of the invocation clone's registered
// worktrees that carry the owned transient prefix. Base names are compared rather
// than full paths because git reports a worktree path canonicalized against the
// cwd it runs from (on macOS /var vs /private/var), so a full-path compare
// against a testsupport.TempDir(t) spelling is unreliable; the base names are invocation-
// unique here.
func (r *initRepo) ownedTempWorktrees(t *testing.T) []string {
	t.Helper()
	out := runGit(t, r.invocation, "worktree", "list", "--porcelain")
	var found []string
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, "worktree ") {
			continue
		}
		base := filepath.Base(strings.TrimPrefix(line, "worktree "))
		if strings.HasPrefix(base, setupTmpWorktreePrefix) {
			found = append(found, base)
		}
	}
	return found
}

// assertNoOwnedTempRefs fails when any ref under the owned transient namespace
// survives on the invocation clone.
func (r *initRepo) assertNoOwnedTempRefs(t *testing.T) {
	t.Helper()
	out, _ := tryGit(r.invocation, "for-each-ref", "--format=%(refname)", setupTmpRefNamespace)
	if strings.TrimSpace(out) != "" {
		t.Errorf("owned transient refs survived: %q", out)
	}
}

// --- boundary 1: before metadata publication ---------------------------------

// TestIntegrationRepoRecoveryBeforeSeedPushLeavesNoRemote proves a run
// interrupted before the seed push leaves no remote effect and no owned transient
// debris, and a retry replans from fresh authority and succeeds.
func TestIntegrationRepoRecoveryBeforeSeedPushLeavesNoRemote(t *testing.T) {
	r := newInitRepo(t, legacyDocketYML, cleanLegacyFiles())

	crash := errors.New("injected crash before the seed push")
	res := r.runMigrateWithHooks(t, MigrateOptions{Authorized: true}, setupHooks{
		beforeSeedPush: func() error { return crash },
	})
	if res.Result == ResultApplied {
		t.Fatalf("a run interrupted before the seed push must not report applied: %s", res.HumanText())
	}
	if r.remoteBranchExists(t, "docket") {
		t.Error("a run interrupted before the seed push created the remote docket branch")
	}
	if wts := r.ownedTempWorktrees(t); len(wts) != 0 {
		t.Errorf("owned transient worktrees survived an interrupted run: %v", wts)
	}
	r.assertNoOwnedTempRefs(t)

	retry := r.runMigrate(t, MigrateOptions{Authorized: true})
	if retry.Result != ResultApplied {
		t.Fatalf("retry = %q (%s), want applied", retry.Result, retry.HumanText())
	}
	if !r.remoteBranchExists(t, "docket") {
		t.Error("retry did not publish the metadata branch")
	}
	if contains(r.originTreePaths(t, "main"), "docs/changes/active/0001-first-change.md") {
		t.Error("retry did not prune the active surface")
	}
}

// --- boundary 2: metadata push response lost ---------------------------------

// TestIntegrationRepoRecoverySeedPushResponseLost proves that when the seed push
// lands but its response is lost, a re-run re-reads the remote docket branch,
// accepts the exact expected shape, and continues; and that a tampered published
// seed is refused as a conflict with nothing destroyed.
func TestIntegrationRepoRecoverySeedPushResponseLost(t *testing.T) {
	t.Run("re-run adopts the durable seed and continues", func(t *testing.T) {
		r := newInitRepo(t, legacyDocketYML, cleanLegacyFiles())

		lost := errors.New("injected response loss after the seed push")
		res := r.runMigrateWithHooks(t, MigrateOptions{Authorized: true}, setupHooks{
			afterSeedPush: func() error { return lost },
		})
		if res.Result == ResultApplied {
			t.Fatalf("a lost seed-push response must not report applied: %s", res.HumanText())
		}
		if !r.remoteBranchExists(t, "docket") {
			t.Fatal("the seed push landed but the docket branch is absent")
		}
		if !contains(r.originTreePaths(t, "main"), "docs/changes/active/0001-first-change.md") {
			t.Error("integration was pruned even though the run aborted before the prune")
		}
		seedTip := r.originTip(t, "docket")

		rerun := r.runMigrate(t, MigrateOptions{Authorized: true})
		if rerun.Result != ResultApplied {
			t.Fatalf("re-run = %q (%s), want applied", rerun.Result, rerun.HumanText())
		}
		if got := r.originTip(t, "docket"); got != seedTip {
			t.Errorf("re-run re-seeded the docket branch (%s -> %s); it must adopt the durable seed", seedTip, got)
		}
		if contains(r.originTreePaths(t, "main"), "docs/changes/active/0001-first-change.md") {
			t.Error("re-run did not prune the active surface")
		}
	})

	t.Run("a tampered seed is a conflict, nothing destroyed", func(t *testing.T) {
		r := newInitRepo(t, legacyDocketYML, cleanLegacyFiles())

		lost := errors.New("injected response loss after the seed push")
		r.runMigrateWithHooks(t, MigrateOptions{Authorized: true}, setupHooks{
			afterSeedPush: func() error { return lost },
		})
		if !r.remoteBranchExists(t, "docket") {
			t.Fatal("the seed push landed but the docket branch is absent")
		}

		// Tamper the published seed: amend one byte into it, KEEPING its receipt
		// (its Docket-Source-Revision still names the current, unchanged source), so
		// only the tree diverges.
		runGit(t, r.writer, "fetch", "-q", "origin", "docket")
		runGit(t, r.writer, "checkout", "-q", "-B", "docket", "origin/docket")
		writeRepoFile(t, r.writer, "docs/changes/tampered.txt", "tampered byte\n")
		runGit(t, r.writer, "add", "--", "docs/changes/tampered.txt")
		runGit(t, r.writer, "commit", "-q", "--amend", "--no-edit")
		runGit(t, r.writer, "push", "-q", "--force", "origin", "docket")
		tamperedTip := r.originTip(t, "docket")

		res := r.runMigrate(t, MigrateOptions{Authorized: true})
		if res.Result != ResultInvalidState || res.RepositoryState != "conflict" {
			t.Fatalf("re-run over a tampered seed = %q (%s), want invalid-state conflict", res.Result, res.HumanText())
		}
		if got := r.originTip(t, "docket"); got != tamperedTip {
			t.Errorf("the tampered docket branch was overwritten (%s -> %s); nothing must be destroyed", tamperedTip, got)
		}
		if !contains(r.originTreePaths(t, "main"), "docs/changes/active/0001-first-change.md") {
			t.Error("a conflict pruned the integration surface; nothing must be destroyed")
		}
	})
}

// --- boundary 3: bash-shaped partial seed adoption ---------------------------

// TestIntegrationRepoRecoveryBashShapedPartialSeedAdopted proves a legacy
// Bash-shaped seed (parentless, exact copy-set tree, no receipt) is adopted: with
// integration still live the migration resumes prune-only; with integration
// already pruned but .docket unattached, only the local steps run.
func TestIntegrationRepoRecoveryBashShapedPartialSeedAdopted(t *testing.T) {
	t.Run("integration live: resume prune-only", func(t *testing.T) {
		r := newInitRepo(t, legacyDocketYML, cleanLegacyFiles())
		seedTip := r.seedBashDocket(t)

		res := r.runMigrate(t, MigrateOptions{Authorized: true})
		if res.Result != ResultApplied {
			t.Fatalf("migrate over a bash seed = %q (%s), want applied", res.Result, res.HumanText())
		}
		if got := r.originTip(t, "docket"); got != seedTip {
			t.Errorf("the bash seed was re-created (%s -> %s); an exact legacy-equivalent seed must be adopted", seedTip, got)
		}
		if contains(r.originTreePaths(t, "main"), "docs/changes/active/0001-first-change.md") {
			t.Error("the resume did not prune the active surface")
		}
		if !contains(r.originTreePaths(t, "main"), "docs/changes/archive/2026-01-02-0003-archived-change.md") {
			t.Error("the resume dropped a retained archived record")
		}
	})

	t.Run("integration pruned, .docket unattached: local steps only", func(t *testing.T) {
		r := newInitRepo(t, legacyDocketYML, cleanLegacyFiles())
		seedTip := r.seedBashDocket(t)
		r.prunePrimaryOnOrigin(t)
		metaTip := r.originTip(t, "docket")
		intTip := r.originTip(t, "main")

		res := r.runMigrate(t, MigrateOptions{Authorized: true})
		if res.Result != ResultApplied {
			t.Fatalf("local-only resume = %q (%s), want applied", res.Result, res.HumanText())
		}
		// No remote write: both branches are byte-untouched.
		if got := r.originTip(t, "docket"); got != metaTip || metaTip != seedTip {
			t.Errorf("local-only resume moved the docket branch: seed=%s meta=%s now=%s", seedTip, metaTip, got)
		}
		if got := r.originTip(t, "main"); got != intTip {
			t.Errorf("local-only resume moved the integration branch from %s to %s", intTip, got)
		}
		// The local .docket worktree is now attached on the metadata branch.
		dotDocket := filepath.Join(r.invocation, ".docket")
		if branch := runGit(t, dotDocket, "rev-parse", "--abbrev-ref", "HEAD"); branch != "docket" {
			t.Errorf(".docket HEAD branch = %q, want docket", branch)
		}
	})
}

// --- boundary 4: integration advanced before prune ---------------------------

// TestIntegrationRepoRecoveryIntegrationAdvancedBeforePrune proves the three
// integration-advance recoveries: a non-planning advance rebuilds the prune atop
// the fresh tip; a planning advance updates docket under its owned lease with the
// re-validated seed and then prunes; a foreign metadata advance refuses.
func TestIntegrationRepoRecoveryIntegrationAdvancedBeforePrune(t *testing.T) {
	t.Run("non-planning advance: prune rebuilt atop the fresh tip", func(t *testing.T) {
		r := newInitRepo(t, legacyDocketYML, cleanLegacyFiles())
		r.runMigrateWithHooks(t, MigrateOptions{Authorized: true}, setupHooks{
			afterSeedPush: func() error { return errors.New("crash after seed") },
		})
		seedTip := r.originTip(t, "docket")
		// Advance integration with bytes OUTSIDE the copy and removal sets.
		r.advanceIntegration(t, "docs/results/late.md", "# late results\n")
		freshTip := r.originTip(t, "main")

		res := r.runMigrate(t, MigrateOptions{Authorized: true})
		if res.Result != ResultApplied {
			t.Fatalf("non-planning advance = %q (%s), want applied", res.Result, res.HumanText())
		}
		if got := r.originTip(t, "docket"); got != seedTip {
			t.Errorf("a non-planning advance re-seeded docket (%s -> %s); the seed bytes were unchanged", seedTip, got)
		}
		// The prune is a descendant of the FRESH tip: the late file survives and the
		// active surface is gone.
		if anc, err := tryGit(r.invocation, "merge-base", "--is-ancestor", freshTip, r.originTip(t, "main")); err != nil {
			t.Errorf("the prune was not built atop the fresh integration tip (%s): %v", freshTip, anc)
		}
		if !contains(r.originTreePaths(t, "main"), "docs/results/late.md") {
			t.Error("the non-planning advance was dropped by the rebuilt prune")
		}
		if contains(r.originTreePaths(t, "main"), "docs/changes/active/0001-first-change.md") {
			t.Error("the rebuilt prune did not remove the active surface")
		}
	})

	t.Run("planning advance: docket updated under owned lease, then prune", func(t *testing.T) {
		r := newInitRepo(t, legacyDocketYML, cleanLegacyFiles())
		r.runMigrateWithHooks(t, MigrateOptions{Authorized: true}, setupHooks{
			afterSeedPush: func() error { return errors.New("crash after seed") },
		})
		seedTip := r.originTip(t, "docket")
		// Advance integration with a NEW ADR — copy-set bytes that must land on the
		// updated seed.
		r.advanceIntegration(t, "docs/adrs/0002-second-decision.md",
			"---\nid: 2\nslug: second-decision\nstatus: Accepted\ntitle: Second decision\n---\nContext.\n")

		res := r.runMigrate(t, MigrateOptions{Authorized: true})
		if res.Result != ResultApplied {
			t.Fatalf("planning advance = %q (%s), want applied", res.Result, res.HumanText())
		}
		if got := r.originTip(t, "docket"); got == seedTip {
			t.Error("a planning advance did not update the docket seed; the new copy-set bytes would be missing")
		}
		if _, ok := r.originBlob(t, "docket", "docs/adrs/0002-second-decision.md"); !ok {
			t.Error("the updated seed is missing the newly added ADR")
		}
		if contains(r.originTreePaths(t, "main"), "docs/changes/active/0001-first-change.md") {
			t.Error("the planning advance did not prune the active surface")
		}
	})

	t.Run("foreign metadata advance: refusal", func(t *testing.T) {
		r := newInitRepo(t, legacyDocketYML, cleanLegacyFiles())
		r.runMigrateWithHooks(t, MigrateOptions{Authorized: true}, setupHooks{
			afterSeedPush: func() error { return errors.New("crash after seed") },
		})
		// A foreign actor advances docket to a NON-orphan tip (a child commit).
		runGit(t, r.writer, "fetch", "-q", "origin", "docket")
		runGit(t, r.writer, "checkout", "-q", "-B", "docket", "origin/docket")
		runGit(t, r.writer, "commit", "-q", "--allow-empty", "-m", "foreign advance")
		runGit(t, r.writer, "push", "-q", "origin", "docket")
		foreignTip := r.originTip(t, "docket")

		res := r.runMigrate(t, MigrateOptions{Authorized: true})
		if res.Result != ResultInvalidState || res.RepositoryState != "conflict" {
			t.Fatalf("foreign advance = %q (%s), want invalid-state conflict", res.Result, res.HumanText())
		}
		if got := r.originTip(t, "docket"); got != foreignTip {
			t.Errorf("the foreign docket advance was overwritten (%s -> %s)", foreignTip, got)
		}
		if !contains(r.originTreePaths(t, "main"), "docs/changes/active/0001-first-change.md") {
			t.Error("a conflict pruned the integration surface")
		}
	})
}

// --- boundary 5: integration push response lost ------------------------------

// TestIntegrationRepoRecoveryPrunePushResponseLost proves that a lost prune-push
// response re-reads the remote and finishes locally on the exact postcondition,
// and that a prune whose lease is lost to a moved integration tip is contention,
// never an overwrite.
func TestIntegrationRepoRecoveryPrunePushResponseLost(t *testing.T) {
	t.Run("exact postcondition: re-run finishes locally", func(t *testing.T) {
		r := newInitRepo(t, legacyDocketYML, cleanLegacyFiles())
		res := r.runMigrateWithHooks(t, MigrateOptions{Authorized: true}, setupHooks{
			afterPrunePush: func() error { return errors.New("prune response lost") },
		})
		if res.Result == ResultApplied {
			t.Fatalf("a lost prune response must not report applied: %s", res.HumanText())
		}
		// Both remote postconditions are durable.
		if !r.remoteBranchExists(t, "docket") {
			t.Fatal("the seed is absent after the prune push")
		}
		if contains(r.originTreePaths(t, "main"), "docs/changes/active/0001-first-change.md") {
			t.Fatal("the prune did not land before the lost response")
		}
		metaTip := r.originTip(t, "docket")
		intTip := r.originTip(t, "main")

		rerun := r.runMigrate(t, MigrateOptions{Authorized: true})
		if rerun.Result != ResultApplied {
			t.Fatalf("re-run = %q (%s), want applied (local finish)", rerun.Result, rerun.HumanText())
		}
		if got := r.originTip(t, "docket"); got != metaTip {
			t.Errorf("the local finish moved the docket branch from %s to %s", metaTip, got)
		}
		if got := r.originTip(t, "main"); got != intTip {
			t.Errorf("the local finish moved the integration branch from %s to %s", intTip, got)
		}
		if branch := runGit(t, filepath.Join(r.invocation, ".docket"), "rev-parse", "--abbrev-ref", "HEAD"); branch != "docket" {
			t.Errorf(".docket HEAD branch = %q, want docket", branch)
		}
	})

	t.Run("moved tip: contention, no overwrite", func(t *testing.T) {
		r := newInitRepo(t, legacyDocketYML, cleanLegacyFiles())
		var movedTip string
		res := r.runMigrateWithHooks(t, MigrateOptions{Authorized: true}, setupHooks{
			beforePrunePush: func() error {
				r.advanceIntegration(t, "docs/results/racer.md", "# a concurrent writer\n")
				movedTip = r.originTip(t, "main")
				return nil
			},
		})
		if res.Result != ResultContended {
			t.Fatalf("a prune against a moved integration tip = %q (%s), want contended", res.Result, res.HumanText())
		}
		if got := r.originTip(t, "main"); got != movedTip {
			t.Errorf("the moved integration tip was overwritten (%s -> %s)", movedTip, got)
		}
	})
}

// --- boundary 6: abrupt-death debris cleanup ---------------------------------

// erroringDebrisProber forces the worktree-enumeration probe to fail, so the
// sweep must RETAIN any debris rather than treat the errored probe as a clean
// absence (learning probe-error-is-not-clean-absence).
type erroringDebrisProber struct{}

func (erroringDebrisProber) ListWorktrees(context.Context, gitcli.Repository) ([]gitcli.WorktreeInfo, error) {
	return nil, errors.New("injected worktree enumeration failure")
}
func (erroringDebrisProber) RemoveWorktree(context.Context, gitcli.Repository, string) error {
	panic("sweep must not remove after an enumeration probe error")
}
func (erroringDebrisProber) ResolveRef(context.Context, gitcli.Repository, gitcli.RefName) (gitcli.ObjectID, error) {
	panic("sweep must not resolve refs after an enumeration probe error")
}
func (erroringDebrisProber) DeleteOwnedRef(context.Context, gitcli.Repository, gitcli.RefName) error {
	panic("sweep must not delete refs after an enumeration probe error")
}

// TestIntegrationRepoRecoveryAbruptDeathDebrisCleanup proves the next invocation
// removes exactly the owned transient worktree/ref left by an abrupt death,
// preserves and reports a user worktree and an ambiguous registration, and — when
// the debris probe itself errors — retains the debris with a warning.
func TestIntegrationRepoRecoveryAbruptDeathDebrisCleanup(t *testing.T) {
	r := newInitRepo(t, defaultSetupYML, nil)
	client := newGitClient(t)
	repo := r.discoverRepo(t, client)
	head := runGit(t, r.invocation, "rev-parse", "HEAD")

	// (1) An exact owned transient: a detached worktree at an owned-prefixed
	// sibling path plus its paired owned ref.
	ownedWT := filepath.Join(r.root, setupTmpWorktreePrefix+"abc")
	runGit(t, r.invocation, "worktree", "add", "--detach", "--", ownedWT, head)
	runGit(t, r.invocation, "update-ref", setupTmpRefNamespace+"abc", head)
	// (2) A user worktree: an unrelated name on its own branch.
	userWT := filepath.Join(r.root, "userfeature")
	runGit(t, r.invocation, "worktree", "add", "-b", "userfeature", "--", userWT, head)
	// (3) An ambiguous registration: the owned prefix but NOT the exact removable
	// shape (attached to a branch, not detached).
	ambWT := filepath.Join(r.root, setupTmpWorktreePrefix+"xyz")
	runGit(t, r.invocation, "worktree", "add", "-b", "ambbranch", "--", ambWT, head)

	report := sweepSetupDebris(context.Background(), client, repo)

	// The exact owned transient worktree and its paired ref are gone.
	if _, err := tryGit(r.invocation, "worktree", "list", "--porcelain"); err != nil {
		t.Fatalf("worktree list: %v", err)
	}
	remaining := r.ownedTempWorktrees(t)
	if contains(remaining, setupTmpWorktreePrefix+"abc") {
		t.Errorf("the exact owned transient worktree survived the sweep: %v", remaining)
	}
	if _, err := tryGit(r.invocation, "rev-parse", "--verify", "--quiet", setupTmpRefNamespace+"abc"); err == nil {
		t.Error("the paired owned transient ref survived the sweep")
	}
	if len(report.removedWorktrees) != 1 || filepath.Base(report.removedWorktrees[0]) != setupTmpWorktreePrefix+"abc" {
		t.Errorf("removed worktrees = %v, want exactly the owned transient", report.removedWorktrees)
	}

	// The user worktree and the ambiguous registration both survive (compared by
	// base name — see ownedTempWorktrees).
	wtList := runGit(t, r.invocation, "worktree", "list", "--porcelain")
	if !strings.Contains(wtList, filepath.Base(userWT)) {
		t.Error("the user worktree was removed; only exact owned transients may be swept")
	}
	if !strings.Contains(wtList, filepath.Base(ambWT)) {
		t.Error("the ambiguous registration was removed; it must be preserved")
	}
	// The ambiguous registration is reported, not silently left.
	reportedAmbiguous := false
	for _, p := range report.preserved {
		if filepath.Base(p) == setupTmpWorktreePrefix+"xyz" {
			reportedAmbiguous = true
		}
	}
	if !reportedAmbiguous {
		t.Errorf("the ambiguous registration was not reported: preserved=%v", report.preserved)
	}

	// Probe error: the sweep retains everything and warns.
	r2 := newInitRepo(t, defaultSetupYML, nil)
	client2 := newGitClient(t)
	repo2 := r2.discoverRepo(t, client2)
	head2 := runGit(t, r2.invocation, "rev-parse", "HEAD")
	debrisWT := filepath.Join(r2.root, setupTmpWorktreePrefix+"probe")
	runGit(t, r2.invocation, "worktree", "add", "--detach", "--", debrisWT, head2)

	errReport := sweepSetupDebris(context.Background(), erroringDebrisProber{}, repo2)
	if len(errReport.warnings) == 0 {
		t.Error("an errored enumeration probe must produce a warning")
	}
	if len(errReport.removedWorktrees) != 0 {
		t.Errorf("an errored enumeration probe removed debris: %v", errReport.removedWorktrees)
	}
	_ = debrisWT
	if got := r2.ownedTempWorktrees(t); !contains(got, setupTmpWorktreePrefix+"probe") {
		t.Errorf("the debris was not retained after a probe error: %v", got)
	}
}
