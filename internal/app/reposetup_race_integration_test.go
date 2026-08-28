//go:build integration

package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/reposetup"
)

// This is the real-concurrency repository-setup shard (prefix
// TestRaceIntegrationRepoSetup), run under -race. Two goroutines contend on ONE
// bare upstream through their own invocation clones and their own gitcli clients —
// the only genuinely shared resource is the remote, so the race detector guards
// that the create-only / exact-lease publication path and the read-only check path
// carry no shared-memory data race in production code. Each goroutine's client is
// built BEFORE the goroutines start (newGitClient calls t.Setenv, which is not safe
// to call from a goroutine), and each drives a whole service against its own clone.

// TestRaceIntegrationRepoSetupConcurrentInitRace runs two RunRepositoryInit calls
// against one upstream from separate clones. Exactly one create-push wins the
// orphan metadata root; the loser adopts the already-published exact empty orphan
// and reports cleanly. The postcondition is identical to a single run: origin has
// exactly one parentless empty-tree docket root and each clone attached its own
// .docket worktree on it.
func TestRaceIntegrationRepoSetupConcurrentInitRace(t *testing.T) {
	r := newInitRepo(t, defaultSetupYML, nil)
	cloneB := cloneOrigin(t, r.origin)
	dirs := []string{r.invocation, cloneB}

	// Clients are constructed sequentially here, off the goroutines' path.
	clients := []*gitcli.Client{newGitClient(t), newGitClient(t)}

	var wg sync.WaitGroup
	results := make([]RepositoryOpResult, 2)
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results[i] = RunRepositoryInit(context.Background(), SetupDeps{Git: clients[i], RepoDir: dirs[i]})
		}()
	}
	close(start)
	wg.Wait()

	// Both runs end cleanly (one created the remote, the other adopted it); neither
	// refuses or errors, and both land the needs-review topology.
	for i, res := range results {
		if res.Result != ResultApplied && res.Result != ResultNoOp {
			t.Fatalf("concurrent init %d = %q (%s), want applied or no-op", i, res.Result, res.HumanText())
		}
		if res.RepositoryState != string(reposetup.StateNeedsReview) {
			t.Errorf("concurrent init %d state = %q, want needs-review", i, res.RepositoryState)
		}
	}

	// Postcondition identical to a single run: EXACTLY ONE parentless empty-tree
	// root on origin/docket — never two roots from two winning create-pushes.
	roots := strings.Fields(runGit(t, r.origin, "rev-list", "--max-parents=0", "docket"))
	if len(roots) != 1 {
		t.Fatalf("origin/docket roots = %v, want exactly one (a single create-push won)", roots)
	}
	tree := runGit(t, r.origin, "rev-parse", "docket^{tree}")
	emptyTree := runGit(t, r.origin, "hash-object", "-t", "tree", "/dev/null")
	if tree != emptyTree {
		t.Errorf("origin/docket tree = %s, want the empty tree %s", tree, emptyTree)
	}
	// Each clone independently attached its own .docket worktree on the docket branch.
	for _, dir := range dirs {
		dotDocket := filepath.Join(dir, ".docket")
		if branch := runGit(t, dotDocket, "rev-parse", "--abbrev-ref", "HEAD"); branch != "docket" {
			t.Errorf("%s/.docket HEAD = %q, want docket", dir, branch)
		}
	}
}

// TestRaceIntegrationRepoSetupConcurrentMigrateAndCheck races an authorized
// migration on one clone against repeated read-only checks on a separate clone of
// the same upstream. Every check observes a well-formed, resumable classification —
// legacy before the seed lands, then the resumable partial through the seed/prune
// window and the remote-migrated-but-locally-unattached window — never a torn
// conflict/unknown, and never writes a local branch or worktree.
func TestRaceIntegrationRepoSetupConcurrentMigrateAndCheck(t *testing.T) {
	r := newInitRepo(t, legacyDocketYML, cleanLegacyFiles())
	checkClone := cloneOrigin(t, r.origin)

	// Both clients built off the goroutines' path (t.Setenv is not goroutine-safe).
	migrateClient := newGitClient(t)
	checkClient := newGitClient(t)

	headBefore := runGit(t, checkClone, "rev-parse", "HEAD")

	const checks = 24
	var wg sync.WaitGroup
	var migrateRes RepositoryMigrateResult
	observed := make([]string, checks)
	exits := make([]int, checks)
	start := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		migrateRes = RunRepositoryMigrate(context.Background(),
			SetupDeps{Git: migrateClient, RepoDir: r.invocation}, MigrateOptions{Authorized: true})
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < checks; i++ {
			res := RunRepositoryCheck(context.Background(), SetupDeps{Git: checkClient, RepoDir: checkClone})
			observed[i] = res.RepositoryState
			exits[i] = res.CheckExitCode()
		}
	}()
	close(start)
	wg.Wait()

	if migrateRes.Result != ResultApplied {
		t.Fatalf("concurrent migrate = %q (%s), want applied", migrateRes.Result, migrateRes.HumanText())
	}

	// Every observed check state is a resumable, well-formed classification: the
	// only non-terminal states a racing check may see are legacy and partial. A
	// conflict, unknown, fresh, or healthy reading would be a torn observation.
	allowed := map[string]bool{
		string(reposetup.StateLegacy):  true,
		string(reposetup.StatePartial): true,
	}
	for i, st := range observed {
		if !allowed[st] {
			t.Errorf("check %d observed a torn state %q; only legacy/partial are permitted mid-migration", i, st)
		}
		if exits[i] != 1 {
			t.Errorf("check %d exit = %d, want 1 (diagnosed, resumable)", i, exits[i])
		}
	}

	// The read-only checks never wrote: the check clone grew no local docket branch,
	// no .docket worktree, and its HEAD is unmoved (fetched remote-tracking refs are
	// excluded — the established read-only contract).
	if _, err := os.Stat(filepath.Join(checkClone, ".docket")); !os.IsNotExist(err) {
		t.Errorf("the racing checks attached a .docket worktree (err=%v); check must not write", err)
	}
	if _, err := tryGit(checkClone, "rev-parse", "--verify", "--quiet", "refs/heads/docket"); err == nil {
		t.Error("the racing checks created a local docket branch; check must not write")
	}
	if headAfter := runGit(t, checkClone, "rev-parse", "HEAD"); headAfter != headBefore {
		t.Errorf("the racing checks moved the check clone HEAD (%s -> %s)", headBefore, headAfter)
	}

	// The migration itself landed the correct, single-rooted metadata topology.
	migRoots := strings.Fields(runGit(t, r.origin, "rev-list", "--max-parents=0", "docket"))
	if len(migRoots) != 1 {
		t.Errorf("origin/docket roots = %v, want exactly one parentless seed", migRoots)
	}
}
