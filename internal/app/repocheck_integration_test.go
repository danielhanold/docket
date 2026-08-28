//go:build integration

package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/reposetup"
)

// --- repository check scenarios (its own TestIntegrationRepoCheck shard) ---------
//
// These exercise the read-only `repository check` service end to end against the
// same bare-upstream + clone fixture. A repository reaches the healthy state only
// once init has run AND the pending managed .gitignore edit has been committed and
// pushed, so the committed-ignore guarantee is proven from the integration COMMIT
// tree (learning gitignore-guarantee-must-be-committed). The healthy fixture's
// .docket.yml deliberately omits the metadata_branch key (a legacy artifact the
// migration removes; a healthy docket repository resolves the default), so the
// CheckHealthyFullPostcondition sub-case can re-add it to break healthy.

// healthySetupYML is docket-mode config WITHOUT the legacy metadata_branch key:
// the metadata branch resolves to its default (docket). A healthy repository
// carries no metadata_branch key.
const healthySetupYML = "integration_branch: main\n"

// runCheck runs RunRepositoryCheck against the invocation clone with a fresh
// isolated client.
func (r *initRepo) runCheck(t *testing.T) RepositoryCheckResult {
	t.Helper()
	client := newGitClient(t)
	return RunRepositoryCheck(context.Background(), SetupDeps{Git: client, RepoDir: r.invocation})
}

// commitAndPushMain stages the given paths in the invocation clone, commits, and
// pushes main to the origin so the authoritative integration tip advances.
func (r *initRepo) commitAndPushMain(t *testing.T, message string, paths ...string) {
	t.Helper()
	args := append([]string{"add", "--"}, paths...)
	runGit(t, r.invocation, args...)
	runGit(t, r.invocation, "commit", "-q", "-m", message)
	runGit(t, r.invocation, "push", "-q", "origin", "main")
}

// newHealthyRepo builds a repository in the healthy state: init, then the pending
// .gitignore edit committed and pushed. It asserts the base state classifies
// healthy with exit 0 before returning, so a sub-case that breaks one conjunct is
// measured against a proven-healthy baseline.
func newHealthyRepo(t *testing.T) *initRepo {
	t.Helper()
	r := newInitRepo(t, healthySetupYML, nil)
	if res := r.runInit(t); res.Result != ResultApplied {
		t.Fatalf("init for healthy fixture = %q (%s), want applied", res.Result, res.HumanText())
	}
	r.commitAndPushMain(t, "commit managed .gitignore", ".gitignore")

	res := r.runCheck(t)
	if res.RepositoryState != string(reposetup.StateHealthy) {
		t.Fatalf("healthy fixture classified %q (%s), want healthy", res.RepositoryState, res.HumanText())
	}
	if code := res.CheckExitCode(); code != 0 {
		t.Fatalf("healthy fixture exit = %d (%s), want 0", code, res.HumanText())
	}
	return r
}

// TestIntegrationRepoCheckFresh proves a fresh repository (no metadata
// branch, no live surface) checks as fresh, exits 1 (diagnosed action required,
// not a crash), and its remedy names init.
func TestIntegrationRepoCheckFresh(t *testing.T) {
	r := newInitRepo(t, healthySetupYML, nil)
	res := r.runCheck(t)

	if res.RepositoryState != string(reposetup.StateFresh) {
		t.Errorf("RepositoryState = %q, want fresh", res.RepositoryState)
	}
	if code := res.CheckExitCode(); code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(res.HumanText(), "docket repository init") {
		t.Errorf("remedy %q must name `docket repository init`", res.HumanText())
	}
}

// TestIntegrationRepoCheckNeedsReviewAfterInit proves the needs-review →
// healthy transition: after init the pending .gitignore edit makes check exit 1
// naming the pending path; committing and pushing it makes check exit 0.
func TestIntegrationRepoCheckNeedsReviewAfterInit(t *testing.T) {
	r := newInitRepo(t, healthySetupYML, nil)
	if res := r.runInit(t); res.Result != ResultApplied {
		t.Fatalf("init = %q (%s), want applied", res.Result, res.HumanText())
	}

	afterInit := r.runCheck(t)
	if afterInit.RepositoryState != string(reposetup.StateNeedsReview) {
		t.Errorf("post-init state = %q, want needs-review", afterInit.RepositoryState)
	}
	if code := afterInit.CheckExitCode(); code != 1 {
		t.Errorf("post-init exit = %d, want 1", code)
	}
	namesGitignore := false
	for _, f := range afterInit.Findings {
		if strings.Contains(f.Ref, ".gitignore") || strings.Contains(f.Remedy, ".gitignore") {
			namesGitignore = true
		}
	}
	if !namesGitignore {
		t.Errorf("post-init findings %+v must name the pending .gitignore path", afterInit.Findings)
	}

	r.commitAndPushMain(t, "commit managed .gitignore", ".gitignore")
	afterCommit := r.runCheck(t)
	if afterCommit.RepositoryState != string(reposetup.StateHealthy) {
		t.Errorf("post-commit state = %q (%s), want healthy", afterCommit.RepositoryState, afterCommit.HumanText())
	}
	if code := afterCommit.CheckExitCode(); code != 0 {
		t.Errorf("post-commit exit = %d (%s), want 0", code, afterCommit.HumanText())
	}
}

// TestIntegrationRepoCheckHealthyFullPostcondition proves the healthy
// baseline and that breaking any single healthy conjunct flips the exit to 1.
func TestIntegrationRepoCheckHealthyFullPostcondition(t *testing.T) {
	// The baseline itself is proven healthy inside newHealthyRepo.
	newHealthyRepo(t)

	t.Run("live surface reintroduced on integration", func(t *testing.T) {
		r := newHealthyRepo(t)
		writeRepoFile(t, r.invocation, "docs/changes/active/0001-example.md", "---\nid: 1\n---\n")
		r.commitAndPushMain(t, "reintroduce live surface", "docs/changes/active/0001-example.md")
		if code := r.runCheck(t).CheckExitCode(); code != 1 {
			t.Errorf("exit = %d, want 1 (a live surface is not healthy)", code)
		}
	})

	t.Run("metadata_branch key re-added", func(t *testing.T) {
		r := newHealthyRepo(t)
		writeRepoFile(t, r.invocation, ".docket.yml", "metadata_branch: docket\nintegration_branch: main\n")
		r.commitAndPushMain(t, "re-add legacy metadata_branch key", ".docket.yml")
		res := r.runCheck(t)
		if code := res.CheckExitCode(); code != 1 {
			t.Errorf("exit = %d (%s), want 1 (a legacy metadata_branch key is not healthy)", code, res.HumanText())
		}
	})

	t.Run("committed ignore block stripped from the commit, working tree intact", func(t *testing.T) {
		r := newHealthyRepo(t)
		// Commit and push a .gitignore WITHOUT the managed block, then restore the
		// managed block in the working tree only. A probe that reads the working
		// tree would see the block (Present) and misreport healthy; the correct
		// probe reads the integration COMMIT tree and reports it stripped.
		writeRepoFile(t, r.invocation, ".gitignore", "node_modules/\n")
		r.commitAndPushMain(t, "strip managed .gitignore block", ".gitignore")
		gitignorePath := filepath.Join(r.invocation, ".gitignore")
		if err := os.WriteFile(gitignorePath, reposetup.GitignoreBlock(), 0o644); err != nil {
			t.Fatalf("restore working-tree block: %v", err)
		}
		if !reposetup.ValidGitignoreBlock(mustReadFile(t, gitignorePath)) {
			t.Fatal("working-tree .gitignore must carry the block so the mutation probe can observe the difference")
		}
		if code := r.runCheck(t).CheckExitCode(); code != 1 {
			t.Errorf("exit = %d, want 1 (the committed ignore block is stripped)", code)
		}
	})

	t.Run("metadata worktree dirtied", func(t *testing.T) {
		r := newHealthyRepo(t)
		if err := os.WriteFile(filepath.Join(r.invocation, ".docket", "scratch.txt"), []byte("dirt\n"), 0o644); err != nil {
			t.Fatalf("dirty .docket worktree: %v", err)
		}
		res := r.runCheck(t)
		if code := res.CheckExitCode(); code != 1 {
			t.Errorf("exit = %d (%s), want 1 (a dirty metadata worktree is not healthy)", code, res.HumanText())
		}
		// A dirty metadata worktree remedy never proposes discarding the work.
		for _, f := range res.Findings {
			if strings.Contains(strings.ToLower(f.Remedy), "discard") || strings.Contains(f.Remedy, "reset --hard") {
				t.Errorf("dirty-worktree remedy %q must not propose a destructive discard", f.Remedy)
			}
		}
	})

	t.Run("docket dir tracked in the integration tree", func(t *testing.T) {
		r := newHealthyRepo(t)
		// Advance the integration branch on the origin so it tracks a .docket/ path
		// (which the managed .gitignore is supposed to keep out of the tree). The
		// writer clone syncs to the current healthy tip first so its push is a
		// fast-forward.
		runGit(t, r.writer, "fetch", "-q", "origin", "main")
		runGit(t, r.writer, "checkout", "-q", "-B", "main", "origin/main")
		writeRepoFile(t, r.writer, ".docket/tracked.txt", "should not be tracked\n")
		// Force past the managed .gitignore (which ignores .docket/) so the path is
		// actually tracked in the integration commit.
		runGit(t, r.writer, "add", "-f", "--", ".docket/tracked.txt")
		runGit(t, r.writer, "commit", "-q", "-m", "track .docket in integration")
		runGit(t, r.writer, "push", "-q", "origin", "main")
		if code := r.runCheck(t).CheckExitCode(); code != 1 {
			t.Errorf("exit = %d, want 1 (.docket tracked in the integration tree is not healthy)", code)
		}
	})
}

// TestIntegrationRepoCheckIsReadOnly proves check performs no write: across
// several fixture states the local refs (excluding refs/remotes/*, which a bounded
// fetch may advance), the primary HEAD, and both worktrees' porcelain status are
// byte-identical before and after a check.
func TestIntegrationRepoCheckIsReadOnly(t *testing.T) {
	fresh := newInitRepo(t, healthySetupYML, nil)
	healthy := newHealthyRepo(t)

	dirty := newHealthyRepo(t)
	if err := os.WriteFile(filepath.Join(dirty.invocation, ".docket", "scratch.txt"), []byte("dirt\n"), 0o644); err != nil {
		t.Fatalf("dirty .docket worktree: %v", err)
	}

	for _, r := range []*initRepo{fresh, healthy, dirty} {
		before := r.readOnlySnapshot(t)
		r.runCheck(t)
		after := r.readOnlySnapshot(t)
		if before != after {
			t.Errorf("check wrote observable state:\nbefore:\n%s\nafter:\n%s", before, after)
		}
	}
}

// readOnlySnapshot captures everything a read-only check must leave untouched:
// the local (non-remote) refs, the primary HEAD, and the porcelain status of the
// primary and (when present) the .docket worktree.
func (r *initRepo) readOnlySnapshot(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	for _, line := range strings.Split(runGit(t, r.invocation, "for-each-ref", "--format=%(refname) %(objectname)"), "\n") {
		if line == "" || strings.HasPrefix(line, "refs/remotes/") {
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString("HEAD ")
	b.WriteString(runGit(t, r.invocation, "rev-parse", "HEAD"))
	b.WriteString("\nprimary-status\n")
	b.WriteString(runGit(t, r.invocation, "status", "--porcelain=v2", "--untracked-files=all"))
	dotDocket := filepath.Join(r.invocation, ".docket")
	if _, err := os.Stat(dotDocket); err == nil {
		b.WriteString("\ndocket-status\n")
		b.WriteString(runGit(t, dotDocket, "status", "--porcelain=v2", "--untracked-files=all"))
	}
	return b.String()
}

// TestIntegrationRepoCheckUnknownAuthority proves an unreachable remote (an
// origin URL pointing at a nonexistent path) yields an undeterminable authority:
// state unknown, exit 2 — never 0 or 1.
func TestIntegrationRepoCheckUnknownAuthority(t *testing.T) {
	r := newInitRepo(t, healthySetupYML, nil)
	runGit(t, r.invocation, "remote", "set-url", "origin", filepath.Join(r.root, "does-not-exist.git"))

	res := r.runCheck(t)
	if res.RepositoryState != string(reposetup.StateUnknown) {
		t.Errorf("RepositoryState = %q, want unknown", res.RepositoryState)
	}
	if code := res.CheckExitCode(); code != 2 {
		t.Errorf("exit = %d, want 2 (undeterminable authority)", code)
	}
}
