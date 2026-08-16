package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/repository/transaction"
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
// The whole matrix runs in BOTH repository modes: `main` (planning records live
// on the default branch) and `docket` (records live on an orphan metadata
// branch). The bare origin is the shared authority every concurrent operation
// contends against through the engine's exact-lease push; each operation gets its
// own invocation clone (its own common dir, its own transactions root) so the
// only contention point is the origin ref — a faithful concurrent-repository
// model. The topology builders and the git oracle helpers are reused from
// status_git_test.go (same package); the record and request fixtures are reused
// from the operation unit-test files.

// --- mode matrix + real-git harness ---------------------------------------

// planRepoMode names one repository mode and how to build a bare-remote topology
// whose corpus records live on that mode's metadata branch.
type planRepoMode struct {
	name   string
	branch string // the metadata branch the corpus and every commit land on
	build  func(t *testing.T, records map[string]string) *gitRepo
}

// planRepoModes is the both-mode matrix every integration test iterates.
func planRepoModes() []planRepoMode {
	return []planRepoMode{
		{
			name:   "main",
			branch: "main",
			build: func(t *testing.T, records map[string]string) *gitRepo {
				return newMainModeRepo(t, records)
			},
		},
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

func TestPlanningConcurrentUnrelatedMutationsBothLand(t *testing.T) {
	requireRealGit(t)
	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			widgetPath := groomPath(3, "widget")
			gadgetPath := groomPath(4, "gadget")
			repo := m.build(t, map[string]string{
				widgetPath: lifecycleChange(3, "widget", "in-progress"),
				gadgetPath: lifecycleChange(4, "gadget", "proposed"),
			})
			widgetVer := blobVersionAt(t, repo.origin, m.branch, widgetPath)
			gadgetVer := blobVersionAt(t, repo.origin, m.branch, gadgetPath)

			// Two independent clones: block on A ∥ defer on B.
			nodeA := planningDepsFor(t, cloneOrigin(t, repo.origin))
			nodeB := planningDepsFor(t, cloneOrigin(t, repo.origin))

			var (
				wg    sync.WaitGroup
				start = make(chan struct{})
				resA  ChangeLifecycleResult
				resB  ChangeLifecycleResult
			)
			wg.Add(2)
			go func() {
				defer wg.Done()
				<-start
				resA = ChangeBlock(context.Background(), nodeA.deps, nodeA.dir, ChangeBlockRequest{
					ChangeID: 3, Path: widgetPath, Version: widgetVer, Reason: "waiting on upstream",
				})
			}()
			go func() {
				defer wg.Done()
				<-start
				resB = ChangeDefer(context.Background(), nodeB.deps, nodeB.dir, ChangeDeferRequest{
					ChangeID: 4, Path: gadgetPath, Version: gadgetVer, WhyDeferred: "Parked pending a decision.\n",
				})
			}()
			close(start)
			wg.Wait()

			if resA.Result != ResultApplied {
				t.Fatalf("block A did not land: %q (findings %v)", resA.Result, resA.Findings)
			}
			if resB.Result != ResultApplied {
				t.Fatalf("defer B did not land: %q (findings %v)", resB.Result, resB.Findings)
			}

			// Both authored decisions survive on the final tree.
			widgetFinal, ok := originFile(t, repo.origin, m.branch, widgetPath)
			if !ok {
				t.Fatalf("widget record missing after block")
			}
			if !strings.Contains(widgetFinal, "status: 'blocked'") {
				t.Errorf("widget not blocked:\n%s", widgetFinal)
			}
			if !strings.Contains(widgetFinal, "blocked_by: 'waiting on upstream'") {
				t.Errorf("block reason lost:\n%s", widgetFinal)
			}
			gadgetFinal, ok := originFile(t, repo.origin, m.branch, gadgetPath)
			if !ok {
				t.Fatalf("gadget record missing after defer")
			}
			if !strings.Contains(gadgetFinal, "status: 'deferred'") {
				t.Errorf("gadget not deferred:\n%s", gadgetFinal)
			}
			if !strings.Contains(gadgetFinal, "## Why deferred\n\nParked pending a decision.\n") {
				t.Errorf("defer rationale lost:\n%s", gadgetFinal)
			}

			// The board reflects the final winner's candidate snapshot: re-rendering
			// the committed corpus reproduces the committed board.
			assertBoardMatchesCommitted(t, repo.origin, m.branch, nodeA.dir)
		})
	}
}

// --- bullet 2: same submitted version → one applied, one contended ---------

func TestPlanningSameEntityVersionOneAppliesOneContends(t *testing.T) {
	requireRealGit(t)
	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			recPath := groomPath(3, "widget")
			repo := m.build(t, map[string]string{
				recPath: lifecycleChange(3, "widget", "in-progress"),
			})
			ver := blobVersionAt(t, repo.origin, m.branch, recPath)

			nodeA := planningDepsFor(t, cloneOrigin(t, repo.origin))
			nodeB := planningDepsFor(t, cloneOrigin(t, repo.origin))

			var (
				wg      sync.WaitGroup
				start   = make(chan struct{})
				results [2]ChangeLifecycleResult
			)
			for i, node := range []realNode{nodeA, nodeB} {
				i, node := i, node
				reason := fmt.Sprintf("contender %d", i)
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					results[i] = ChangeBlock(context.Background(), node.deps, node.dir, ChangeBlockRequest{
						ChangeID: 3, Path: recPath, Version: ver, Reason: reason,
					})
				}()
			}
			close(start)
			wg.Wait()

			applied, contended := 0, 0
			for _, r := range results {
				switch r.Result {
				case ResultApplied:
					applied++
				case ResultContended:
					contended++
				default:
					t.Errorf("unexpected outcome %q (findings %v)", r.Result, r.Findings)
				}
			}
			if applied != 1 || contended != 1 {
				t.Fatalf("same-version race: applied=%d contended=%d, want exactly one of each", applied, contended)
			}
		})
	}
}

// --- bullet 3: concurrent allocation never duplicates an id ----------------

func TestPlanningConcurrentCreatesAllocateDistinctIDs(t *testing.T) {
	requireRealGit(t)
	const n = 4
	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			repo := m.build(t, map[string]string{
				"docs/changes/active/0001-first.md": fixtureChange(1, "first"),
			})

			nodes := make([]realNode, n)
			for i := range nodes {
				nodes[i] = planningDepsFor(t, cloneOrigin(t, repo.origin))
			}

			var (
				wg      sync.WaitGroup
				start   = make(chan struct{})
				results = make([]ChangeCreateResult, n)
			)
			for i := range nodes {
				i := i
				req := validChangeCreateRequest()
				req.RequestID = fmt.Sprintf("req-%08d", i+1)
				req.Title = fmt.Sprintf("Concurrent change number %d", i+1)
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					results[i] = ChangeCreate(context.Background(), nodes[i].deps, nodes[i].dir, req)
				}()
			}
			close(start)
			wg.Wait()

			seen := map[int]bool{}
			for i, r := range results {
				if r.Result != ResultApplied {
					t.Fatalf("create %d did not apply: %q (findings %v)", i, r.Result, r.Findings)
				}
				if r.ID <= 1 {
					t.Errorf("create %d allocated id %d, want > 1 (never gap-fill or reuse)", i, r.ID)
				}
				if seen[r.ID] {
					t.Errorf("duplicate allocated id %d", r.ID)
				}
				seen[r.ID] = true
			}

			// The final corpus is valid and carries every allocation.
			snap := committedCorpusSnapshot(t, repo.invocation)
			if got := len(snap.Changes()); got != n+1 {
				t.Errorf("final corpus has %d changes, want %d", got, n+1)
			}
		})
	}
}

func TestPlanningConcurrentADRRecordsAllocateDistinctIDs(t *testing.T) {
	requireRealGit(t)
	const n = 4
	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			repo := m.build(t, map[string]string{
				adrPath("0001", "first"): fixtureADR(1, "first"),
			})

			nodes := make([]realNode, n)
			for i := range nodes {
				nodes[i] = planningDepsFor(t, cloneOrigin(t, repo.origin))
			}

			var (
				wg      sync.WaitGroup
				start   = make(chan struct{})
				results = make([]ADRResult, n)
			)
			for i := range nodes {
				i := i
				req := validADRRecordRequest()
				req.RequestID = fmt.Sprintf("adr-%08d", i+1)
				req.Title = fmt.Sprintf("Concurrent decision number %d", i+1)
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					results[i] = ADRRecordOp(context.Background(), nodes[i].deps, nodes[i].dir, req)
				}()
			}
			close(start)
			wg.Wait()

			seen := map[int]bool{}
			for i, r := range results {
				if r.Result != ResultApplied {
					t.Fatalf("adr record %d did not apply: %q (findings %v)", i, r.Result, r.Findings)
				}
				if r.ID <= 1 {
					t.Errorf("adr record %d allocated id %d, want > 1", i, r.ID)
				}
				if seen[r.ID] {
					t.Errorf("duplicate allocated adr id %d", r.ID)
				}
				seen[r.ID] = true
			}

			// The committed ADR index reflects the final ADR set, byte-for-byte.
			assertIndexMatchesCommitted(t, repo.origin, m.branch, repo.invocation)
		})
	}
}

// --- bullet 5: a validation refusal pushes nothing -------------------------

func TestPlanningRefusalPushesNothing(t *testing.T) {
	requireRealGit(t)
	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			repo := m.build(t, map[string]string{
				"docs/changes/active/0001-first.md": fixtureChange(1, "first"),
			})
			before := originTip(t, repo.origin, m.branch)

			node := planningDepsFor(t, repo.invocation)
			req := validChangeCreateRequest()
			// A depends_on reference that resolves against nothing: the whole-repository
			// validation inside the plan closure refuses, so the transaction commits
			// nothing.
			req.DependsOn = []int{999}
			res := ChangeCreate(context.Background(), node.deps, node.dir, req)

			if res.Result != ResultInvalidInput {
				t.Fatalf("dangling-reference create mapped to %q, want invalid-input (findings %v)", res.Result, res.Findings)
			}
			if after := originTip(t, repo.origin, m.branch); after != before {
				t.Errorf("remote ref moved on a refusal: %q -> %q", before, after)
			}
		})
	}
}

// --- bullet 4 + 6: one commit, explicit paths, clean root, board current ---

func TestPlanningSuccessIsOneCommitWithExplicitPathsCleanRoot(t *testing.T) {
	requireRealGit(t)
	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			repo := m.build(t, map[string]string{
				"docs/changes/active/0001-first.md": fixtureChange(1, "first"),
			})
			node := planningDepsFor(t, repo.invocation)

			res := ChangeCreate(context.Background(), node.deps, node.dir, validChangeCreateRequest())
			if res.Result != ResultApplied {
				t.Fatalf("create did not apply: %q (findings %v)", res.Result, res.Findings)
			}
			if res.Revision == "" {
				t.Fatalf("applied result carried no committed revision")
			}

			newRecord := "docs/changes/active/0002-add-a-widget.md"
			want := []string{"docs/changes/BOARD.md", newRecord}
			sort.Strings(want)
			got := originCommitPaths(t, repo.origin, res.Revision)
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Errorf("winning commit changed %v, want exactly %v", got, want)
			}

			// The candidate never leaks: the transactions root is clean.
			if !transactionsRootEmpty(t, node.deps.Client, node.dir) {
				t.Errorf("transactions root left a candidate worktree behind")
			}

			// The committed board is a fresh render of the committed corpus.
			assertBoardMatchesCommitted(t, repo.origin, m.branch, repo.invocation)
		})
	}
}

// --- bullet 7: idempotent replay end-to-end --------------------------------

func TestPlanningIdempotentReplayEndToEnd(t *testing.T) {
	requireRealGit(t)
	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			repo := m.build(t, map[string]string{
				"docs/changes/active/0001-first.md": fixtureChange(1, "first"),
			})
			node := planningDepsFor(t, repo.invocation)
			req := validChangeCreateRequest()

			first := ChangeCreate(context.Background(), node.deps, node.dir, req)
			if first.Result != ResultApplied || first.Replayed {
				t.Fatalf("first create = (%q, replayed=%v), want applied/false", first.Result, first.Replayed)
			}
			if first.ID != 2 {
				t.Fatalf("first create allocated id %d, want 2", first.ID)
			}
			tipAfterFirst := originTip(t, repo.origin, m.branch)

			// Sever the response and re-run the identical request: the replay returns
			// the original receipt and commits nothing new.
			replay := ChangeCreate(context.Background(), node.deps, node.dir, req)
			if replay.Result != ResultApplied || !replay.Replayed {
				t.Fatalf("replay = (%q, replayed=%v), want applied/true (findings %v)", replay.Result, replay.Replayed, replay.Findings)
			}
			if replay.ID != first.ID {
				t.Errorf("replay allocated a different id: %d != %d", replay.ID, first.ID)
			}
			if tip := originTip(t, repo.origin, m.branch); tip != tipAfterFirst {
				t.Errorf("replay produced a second commit: %q -> %q", tipAfterFirst, tip)
			}

			// The same request id with a different digest is a conflicting reuse.
			conflict := req
			conflict.Title = "A completely different title"
			res := ChangeCreate(context.Background(), node.deps, node.dir, conflict)
			if res.Result != ResultInvalidInput {
				t.Errorf("request-id reuse with a changed digest mapped to %q, want invalid-input (findings %v)", res.Result, res.Findings)
			}
			if tip := originTip(t, repo.origin, m.branch); tip != tipAfterFirst {
				t.Errorf("a rejected digest-conflict moved the remote ref: %q -> %q", tipAfterFirst, tip)
			}
		})
	}
}

// --- bullet 8: kill end-to-end (archive move, backlink, board, no branch) --

func TestPlanningKillEndToEnd(t *testing.T) {
	requireRealGit(t)
	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			widgetPath := groomPath(3, "widget")
			specPath := "docs/superpowers/specs/2026-08-16-widget-design.md"
			repo := m.build(t, map[string]string{
				widgetPath: killableChangeWithSpec(3, "widget", specPath),
				specPath:   specWithBacklink(widgetPath),
			})
			ver := blobVersionAt(t, repo.origin, m.branch, widgetPath)
			node := planningDepsFor(t, repo.invocation)

			res := ChangeKill(context.Background(), node.deps, node.dir, ChangeKillRequest{
				ChangeID: 3, Path: widgetPath, Version: ver, WhyKilled: "Superseded by a better plan.\n",
			})
			if res.Result != ResultApplied {
				t.Fatalf("kill did not apply: %q (findings %v)", res.Result, res.Findings)
			}
			archivePath := "docs/changes/archive/2026-08-16-0003-widget.md"
			if res.ArchivePath != archivePath {
				t.Fatalf("archive path = %q, want %q", res.ArchivePath, archivePath)
			}

			// The active record is gone; the archived record carries the killed status
			// and the authored rationale.
			if _, ok := originFile(t, repo.origin, m.branch, widgetPath); ok {
				t.Errorf("active record still present after kill (presence-encoded state)")
			}
			archived, ok := originFile(t, repo.origin, m.branch, archivePath)
			if !ok {
				t.Fatalf("archived record absent at %q", archivePath)
			}
			if !strings.Contains(archived, "status: 'killed'") {
				t.Errorf("archived record not killed:\n%s", archived)
			}
			if !strings.Contains(archived, "## Why killed\n\nSuperseded by a better plan.\n") {
				t.Errorf("kill rationale not spliced:\n%s", archived)
			}

			// The linked spec's backlink is retargeted to the archive path.
			specFinal, ok := originFile(t, repo.origin, m.branch, specPath)
			if !ok {
				t.Fatalf("spec file vanished")
			}
			if !strings.Contains(specFinal, archivePath) {
				t.Errorf("spec backlink not retargeted to the archive path:\n%s", specFinal)
			}
			if strings.Contains(specFinal, "`"+widgetPath+"`") {
				t.Errorf("spec backlink still points at the vacated active path:\n%s", specFinal)
			}

			// The board is refreshed and current; no feature-branch state is touched.
			assertBoardMatchesCommitted(t, repo.origin, m.branch, repo.invocation)
			if branches := originFeatureBranches(t, repo.origin); len(branches) != 0 {
				t.Errorf("kill touched feature-branch state: %v", branches)
			}
		})
	}
}
