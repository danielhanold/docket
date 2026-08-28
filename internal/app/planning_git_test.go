package app

import (
	"context"
	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/repository/transaction"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// These are the Git-backed planning-operation integration tests: they drive the
// real operations (change/adr/learning) through a real transaction.Engine over
// real bare-remote temporary repositories, exercising the concurrency,
// idempotency, refusal, and atomicity properties the unit tests can only fake.
// Every operation resolves its own authoritative context from repoDir through
// the production StatusReader, so a test only builds a repository topology, reads
// the current entity versions with an independent git oracle, and calls the
// operation with repoDir pointed at an invocation clone.
//
// The matrix runs on the one supported topology: records live on the orphan
// `docket` metadata branch (change 0363 removed the retired main-mode row).
// The bare origin is the shared authority every concurrent operation
// contends against through the engine's exact-lease push; each operation gets its
// own invocation clone (its own common dir, its own transactions root) so the
// only contention point is the origin ref — a faithful concurrent-repository
// model. The topology builders and the git oracle helpers are reused from
// status_git_test.go (same package); the record and request fixtures are reused
// from the operation unit-test files.

// --- mode matrix + real-git harness ---------------------------------------

// planRepoMode names one repository topology and how to build a bare-remote
// fixture whose corpus records live on its metadata branch.
type planRepoMode struct {
	name   string
	branch string // the metadata branch the corpus and every commit land on
	build  func(t *testing.T, records map[string]string) *gitRepo
}

// planRepoModes is the topology matrix every integration test iterates. Change
// 0363 collapsed it to its docket row — the one supported topology — keeping
// every mode-independent assertion (exact-lease CAS, private transaction
// worktrees, ref isolation, retries, interruption recovery, finalization,
// cleanup, link repair) running against the docket metadata branch. The
// retired main-mode row's topology survives only as newLegacyRepo, whose sole
// subjects are the operational refusal and the migration path.
func planRepoModes() []planRepoMode {
	return []planRepoMode{
		{
			name:   "docket",
			branch: "docket",
			build: func(t *testing.T, records map[string]string) *gitRepo {
				return newDocketModeRepo(t, nil, records)
			},
		},
	}
}

// realNode is one invocation clone paired with the production planning
// dependencies that operate it. Every concurrent participant holds its own node
// so no reader/engine/client state is shared across goroutines.
type realNode struct {
	dir  string
	deps PlanningDeps
}

// planningDepsFor builds the production planning seams over dir: a real gitcli
// client (with an isolated empty global-config layer), a transaction engine on
// the shared test clock, and the Git-backed status reader. It must be called on
// the test goroutine (newGitClient uses t.Setenv), so every concurrent node is
// constructed up front, before any goroutine launches.
func planningDepsFor(t *testing.T, dir string) realNode {
	t.Helper()
	client := newGitClient(t)
	engine, err := transaction.NewEngine(client, testClock())
	if err != nil {
		t.Fatalf("transaction.NewEngine: %v", err)
	}
	return realNode{
		dir: dir,
		deps: PlanningDeps{
			Client: client,
			Engine: engine,
			Reader: NewGitStatusReader(client),
			Clock:  testClock(),
		},
	}
}

// cloneOrigin makes a fresh invocation clone of the bare origin with a pinned
// identity, so each concurrent operation works against its own common dir.
func cloneOrigin(t *testing.T, origin string) string {
	t.Helper()
	parent := t.TempDir()
	dir := filepath.Join(parent, "clone")
	runGit(t, parent, "clone", "-q", origin, dir)
	gitIdentity(t, dir)
	return dir
}

// blobVersionAt reads the blob object id of a repo-relative path at branch on the
// bare origin — the independent oracle an exact-version request submits.
func blobVersionAt(t *testing.T, origin, branch, p string) string {
	t.Helper()
	return runGit(t, origin, "rev-parse", branch+":"+p)
}

// originFile returns the raw bytes of a path at branch on the bare origin and
// whether it exists. The bytes are untrimmed so a byte-exact derived-view
// comparison is honest.
func originFile(t *testing.T, origin, branch, p string) (string, bool) {
	t.Helper()
	out, err := tryGit(origin, "show", branch+":"+p)
	if err != nil {
		return "", false
	}
	return out, true
}

// originTip returns the current commit id of branch on the bare origin.
func originTip(t *testing.T, origin, branch string) string {
	t.Helper()
	return runGit(t, origin, "rev-parse", branch)
}

// originCommitPaths returns the sorted changed-path set of commit against its
// first parent on the bare origin.
func originCommitPaths(t *testing.T, origin, commit string) []string {
	t.Helper()
	out := runGit(t, origin, "diff-tree", "--no-commit-id", "--name-only", "-r", commit)
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			paths = append(paths, line)
		}
	}
	sort.Strings(paths)
	return paths
}

// originFeatureBranches returns every non-metadata local branch on the bare
// origin — anything the kill end-to-end proof must show untouched.
func originFeatureBranches(t *testing.T, origin string) []string {
	t.Helper()
	out := runGit(t, origin, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	var branches []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "main" || line == "docket" {
			continue
		}
		branches = append(branches, line)
	}
	return branches
}

// transactionsRootEmpty reports whether the engine left no candidate worktrees
// behind under repoDir's transactions root (the registry.lock mutex file does
// not count). It mirrors transactionsEmpty in the transaction package's harness,
// resolving the root the same way (CommonDir/docket/transactions) without
// importing that package's unexported helpers.
func transactionsRootEmpty(t *testing.T, client *gitcli.Client, repoDir string) bool {
	t.Helper()
	repo, err := client.Discover(context.Background(), gitcli.DiscoverOptions{InvocationPath: repoDir})
	if err != nil {
		t.Fatalf("discover for transactions root: %v", err)
	}
	root := filepath.Join(repo.CommonDir, "docket", "transactions")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return true
		}
		t.Fatalf("read transactions root: %v", err)
	}
	for _, e := range entries {
		if e.Name() == "registry.lock" {
			continue
		}
		return false
	}
	return true
}

// committedCorpusSnapshot re-reads the committed corpus from repoDir through the
// production reader and builds the domain snapshot the derived-view renderers
// consume. It is the oracle that proves a committed derived view never trails the
// records it was rendered from: rendering it here must reproduce the committed
// board / index byte-for-byte.
func committedCorpusSnapshot(t *testing.T, repoDir string) domain.Snapshot {
	t.Helper()
	ctx := context.Background()
	reader := NewGitStatusReader(newGitClient(t))
	pin, err := reader.PinContext(ctx, repoDir)
	if err != nil {
		t.Fatalf("re-pin committed context: %v", err)
	}
	blobs, err := reader.ReadCorpus(ctx, pin)
	if err != nil {
		t.Fatalf("re-read committed corpus: %v", err)
	}
	inputs := make([]repository.InputDocument, 0, len(blobs))
	for _, b := range blobs {
		doc, perr := document.Parse(b.Data)
		if perr != nil {
			t.Fatalf("parse committed record %q: %v", b.Path, perr)
		}
		inputs = append(inputs, repository.InputDocument{
			Kind: b.Kind, Location: b.Location, Path: b.Path, Document: doc,
		})
	}
	build, err := repository.BuildSnapshot(repository.BuildInput{
		Config:    pin.Config.Effective,
		Documents: inputs,
	})
	if err != nil {
		t.Fatalf("build committed snapshot: %v", err)
	}
	if build.Report.HasErrors() {
		t.Fatalf("committed corpus carries error findings: %v", build.Report.Findings())
	}
	return build.Snapshot
}

// assertBoardMatchesCommitted proves the committed inline board equals a fresh
// render of the committed corpus.
func assertBoardMatchesCommitted(t *testing.T, origin, branch, repoDir string) {
	t.Helper()
	committed, ok := originFile(t, origin, branch, "docs/changes/BOARD.md")
	if !ok {
		t.Fatalf("BOARD.md absent on %s after a change operation", branch)
	}
	want, err := render.Board(render.BoardInput{Snapshot: committedCorpusSnapshot(t, repoDir)})
	if err != nil {
		t.Fatalf("render board oracle: %v", err)
	}
	if string(want) != committed {
		t.Errorf("committed board trails its sources:\n--fresh render--\n%s\n--committed--\n%s", want, committed)
	}
}

// assertIndexMatchesCommitted proves the committed ADR index equals a fresh
// render of the committed corpus.
func assertIndexMatchesCommitted(t *testing.T, origin, branch, repoDir string) {
	t.Helper()
	committed, ok := originFile(t, origin, branch, "docs/adrs/README.md")
	if !ok {
		t.Fatalf("docs/adrs/README.md absent on %s after an adr operation", branch)
	}
	want, err := render.ADRIndex(committedCorpusSnapshot(t, repoDir))
	if err != nil {
		t.Fatalf("render adr index oracle: %v", err)
	}
	if string(want) != committed {
		t.Errorf("committed adr index trails its sources:\n--fresh render--\n%s\n--committed--\n%s", want, committed)
	}
}

// killableChangeWithSpec renders a proposed change record carrying a spec pointer
// at specPath, so a kill exercises the metadata-resident-spec backlink retarget.
func killableChangeWithSpec(id int, slug, specPath string) string {
	src := groomableChange(id, slug)
	return strings.Replace(src, "spec:\n", "spec: "+specPath+"\n", 1)
}

// specWithBacklink renders a minimal spec file: a docket:backlink managed block
// (targeting the change's active path) followed by an authored body, exactly the
// shape change groom writes. Kill retargets the block to the archive path.
func specWithBacklink(activePath string) string {
	return "<!-- docket:backlink:start (generated — do not hand-edit) -->\n" +
		"> ↩ **Change 0003 — A change** — `" + activePath + "`\n" +
		"<!-- docket:backlink:end -->\n\n" +
		"# Design\n\nThe widget design.\n"
}

// --- bullet 1: unrelated concurrent mutations both land --------------------

// --- bullet 2: same submitted version → one applied, one contended ---------

// --- bullet 3: concurrent allocation never duplicates an id ----------------

// --- bullet 5: a validation refusal pushes nothing -------------------------

// --- bullet 4 + 6: one commit, explicit paths, clean root, board current ---

// --- bullet 7: idempotent replay end-to-end --------------------------------

// --- bullet 8: kill end-to-end (archive move, backlink, board, no branch) --
