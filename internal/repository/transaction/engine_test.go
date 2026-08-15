package transaction

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
)

// engineClock is the pinned instant every engine test commits and stamps with.
var engineClock = fakeClock{t: time.Date(2026, 8, 15, 9, 0, 0, 0, time.UTC)}

// scriptedOp is a configurable SemanticOperation for the engine tests. Each field
// shapes one facet of an attempt's plan; beforePlan runs a side effect (advancing
// origin) before a chosen attempt's plan is returned. It is used single-threaded
// within one Execute, so the calls counter needs no synchronization.
type scriptedOp struct {
	files      []FileMutation
	subject    string
	receipt    []byte
	refuse     bool
	findings   []domain.Finding
	planErr    error
	calls      int
	beforePlan func(call int)
}

func (o *scriptedOp) Key() OperationKey { return "test.op" }

func (o *scriptedOp) Plan(_ context.Context, _ AttemptState) (MutationPlan, OperationResult, error) {
	o.calls++
	if o.beforePlan != nil {
		o.beforePlan(o.calls)
	}
	if o.planErr != nil {
		return MutationPlan{}, OperationResult{}, o.planErr
	}
	if o.refuse {
		return MutationPlan{}, OperationResult{Refused: true, Findings: o.findings}, nil
	}
	subject := o.subject
	if subject == "" {
		subject = "test: apply"
	}
	receipt := o.receipt
	if receipt == nil {
		receipt = validReceipt()
	}
	return MutationPlan{Files: o.files, CommitSubject: subject, Receipt: receipt}, OperationResult{}, nil
}

// createOp returns an operation that creates one record at path with content.
func createOp(path, content string) *scriptedOp {
	return &scriptedOp{files: []FileMutation{
		{Path: gitcli.RepoPath(path), Kind: MutationCreate, Bytes: []byte(content)},
	}}
}

// newEngine builds an Engine over a fresh client and the pinned test clock.
func newEngine(t *testing.T, client *gitcli.Client) *Engine {
	t.Helper()
	eng, err := NewEngine(client, engineClock)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	return eng
}

// thirdChangePath / thirdChange is the standard record the happy-path operations
// create: a valid change whose filename encodes its id and slug.
const thirdChangePath = "docs/changes/active/0003-third-change.md"

func thirdChange() string { return corpusChange(3, "third-change", "proposed") }

func TestNewEngineRejectsNilDependencies(t *testing.T) {
	if _, err := NewEngine(nil, engineClock); err == nil {
		t.Error("NewEngine(nil client): want error")
	}
	client, err := gitcli.NewClient()
	if err != nil {
		t.Skipf("NewClient: %v", err)
	}
	if _, err := NewEngine(client, nil); err == nil {
		t.Error("NewEngine(nil clock): want error")
	}
}

// TestEngineAppliesHappyPath runs one full apply on both topologies and proves the
// commit landed on origin with exactly the planned path, the engine trailer block,
// a populated Result, and a fully cleaned transactions root.
func TestEngineAppliesHappyPath(t *testing.T) {
	for _, topo := range topologies() {
		t.Run(topo.name, func(t *testing.T) {
			r := topo.build(t)
			client, repo := r.discover(t)
			eng := newEngine(t, client)
			base := r.originTip(t)

			op := createOp(thirdChangePath, thirdChange())
			res, err := eng.Execute(context.Background(), Request{
				Repository: repo, Remote: "origin", TargetRef: r.Target,
				Loader: testLoader{}, Operation: op,
			})
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if res.Disposition != DispositionApplied {
				t.Fatalf("disposition = %q, want applied (findings %v)", res.Disposition, res.Findings)
			}
			if res.Attempts != 1 {
				t.Errorf("attempts = %d, want 1", res.Attempts)
			}
			if res.Operation != "test.op" {
				t.Errorf("operation = %q, want test.op", res.Operation)
			}
			if res.BaseCommit != base {
				t.Errorf("base commit = %q, want %q", res.BaseCommit, base)
			}
			tip := r.originTip(t)
			if res.AppliedCommit != tip {
				t.Errorf("applied commit = %q, origin tip = %q", res.AppliedCommit, tip)
			}
			if res.RemoteCommit != tip {
				t.Errorf("remote commit = %q, want %q", res.RemoteCommit, tip)
			}
			if string(res.Receipt) != string(validReceipt()) {
				t.Errorf("receipt = %q, want %q", res.Receipt, validReceipt())
			}
			if len(res.CleanupWarnings) != 0 {
				t.Errorf("cleanup warnings = %v, want none", res.CleanupWarnings)
			}

			paths := diffTreePaths(t, r.Origin, res.AppliedCommit)
			if len(paths) != 1 || paths[0] != thirdChangePath {
				t.Errorf("committed paths = %v, want [%s]", paths, thirdChangePath)
			}
			trailers := hgitOut(t, r.Origin, "log", "-1", "--format=%(trailers:only,unfold)", string(res.AppliedCommit))
			for _, want := range []string{"Docket-Transaction-ID: ", "Docket-Operation: test.op", "Docket-Result: "} {
				if !strings.Contains(trailers, want) {
					t.Errorf("trailer block missing %q:\n%s", want, trailers)
				}
			}
			if !transactionsEmpty(t, repo) {
				t.Error("transactions root not empty after apply")
			}
		})
	}
}

// TestEngineNoOpOnEmptyPlan proves an empty-Files plan is a no-op: no commit lands
// and the disposition is no-op.
func TestEngineNoOpOnEmptyPlan(t *testing.T) {
	r := newMainModeRepos(t)
	client, repo := r.discover(t)
	eng := newEngine(t, client)
	base := r.originTip(t)

	op := &scriptedOp{files: nil} // empty plan (valid subject/receipt supplied by defaults)
	res, err := eng.Execute(context.Background(), Request{
		Repository: repo, Remote: "origin", TargetRef: r.Target,
		Loader: testLoader{}, Operation: op,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Disposition != DispositionNoOp {
		t.Fatalf("disposition = %q, want no-op", res.Disposition)
	}
	if r.originTip(t) != base {
		t.Error("origin advanced on a no-op")
	}
	if !transactionsEmpty(t, repo) {
		t.Error("transactions root not empty after no-op")
	}
}

// TestEngineRefusalFromOperation proves an operation refusal produces a refused
// disposition carrying its findings, with no commit.
func TestEngineRefusalFromOperation(t *testing.T) {
	r := newMainModeRepos(t)
	client, repo := r.discover(t)
	eng := newEngine(t, client)
	base := r.originTip(t)

	op := &scriptedOp{refuse: true, findings: []domain.Finding{
		{Code: "op-refused", Severity: domain.SeverityError, Entity: domain.EntityRef{Kind: domain.EntityRepo}},
	}}
	res, err := eng.Execute(context.Background(), Request{
		Repository: repo, Remote: "origin", TargetRef: r.Target,
		Loader: testLoader{}, Operation: op,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Disposition != DispositionRefused {
		t.Fatalf("disposition = %q, want refused", res.Disposition)
	}
	if len(res.Findings) == 0 {
		t.Error("refusal carried no findings")
	}
	if r.originTip(t) != base {
		t.Error("origin advanced on a refusal")
	}
	if !transactionsEmpty(t, repo) {
		t.Error("transactions root not empty after refusal")
	}
}

// TestEngineBeforeGateRefusesInvalidBase proves the before-gate refuses when the
// loaded base has error findings — before the operation is ever consulted.
func TestEngineBeforeGateRefusesInvalidBase(t *testing.T) {
	r := newMainModeRepos(t)
	client, repo := r.discover(t)
	eng := newEngine(t, client)

	// A record whose filename disagrees with its frontmatter slug is a base error.
	r.advanceOrigin(t, "docs/changes/active/0007-mismatch.md", corpusChange(7, "different-slug", "proposed"))
	base := r.originTip(t)

	op := createOp(thirdChangePath, thirdChange())
	res, err := eng.Execute(context.Background(), Request{
		Repository: repo, Remote: "origin", TargetRef: r.Target,
		Loader: testLoader{}, Operation: op,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Disposition != DispositionRefused {
		t.Fatalf("disposition = %q, want refused", res.Disposition)
	}
	if op.calls != 0 {
		t.Errorf("operation was consulted %d times before the before-gate refused", op.calls)
	}
	if len(res.Findings) == 0 {
		t.Error("before-gate refusal carried no findings")
	}
	if r.originTip(t) != base {
		t.Error("origin advanced on a before-gate refusal")
	}
}

// TestEngineAfterGateRefusesInvalidPlan proves the after-gate refuses when the
// operation plans a domain-invalid record; nothing is pushed.
func TestEngineAfterGateRefusesInvalidPlan(t *testing.T) {
	r := newMainModeRepos(t)
	client, repo := r.discover(t)
	eng := newEngine(t, client)
	base := r.originTip(t)

	// Filename slug "bad" disagrees with frontmatter slug "wrong-slug": an error.
	op := createOp("docs/changes/active/0004-bad.md", corpusChange(4, "wrong-slug", "proposed"))
	res, err := eng.Execute(context.Background(), Request{
		Repository: repo, Remote: "origin", TargetRef: r.Target,
		Loader: testLoader{}, Operation: op,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Disposition != DispositionRefused {
		t.Fatalf("disposition = %q, want refused", res.Disposition)
	}
	if len(res.Findings) == 0 {
		t.Error("after-gate refusal carried no findings")
	}
	if r.originTip(t) != base {
		t.Error("origin advanced on an after-gate refusal")
	}
}

// TestEngineEvolutionGateBlocksFrozenADRRewrite proves the evolution gate blocks a
// plan that rewrites an already-published ADR's frozen body.
func TestEngineEvolutionGateBlocksFrozenADRRewrite(t *testing.T) {
	r := newMainModeRepos(t)
	client, repo := r.discover(t)
	eng := newEngine(t, client)
	base := r.originTip(t)

	rewritten := strings.Replace(corpusADR(1, "first-decision"), "Body.", "Rewritten body.", 1)
	op := &scriptedOp{files: []FileMutation{
		{Path: "docs/adrs/0001-first-decision.md", Kind: MutationReplace, Bytes: []byte(rewritten)},
	}}
	res, err := eng.Execute(context.Background(), Request{
		Repository: repo, Remote: "origin", TargetRef: r.Target,
		Loader: testLoader{}, Operation: op,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Disposition != DispositionRefused {
		t.Fatalf("disposition = %q, want refused (findings %v)", res.Disposition, res.Findings)
	}
	if r.originTip(t) != base {
		t.Error("origin advanced on an evolution-gate refusal")
	}
}

// TestEngineExpectationMatrix proves matching/stale blob and present/absent
// expectations gate the transaction correctly.
func TestEngineExpectationMatrix(t *testing.T) {
	firstPath := "docs/changes/active/0001-first-change.md"

	t.Run("matching blob applies", func(t *testing.T) {
		r := newMainModeRepos(t)
		client, repo := r.discover(t)
		eng := newEngine(t, client)
		exp := []EntityExpectation{{Path: gitcli.RepoPath(firstPath),
			Version: ExpectedVersion{Kind: VersionBlob, ObjectID: r.blobID(t, firstPath)}}}
		res := mustExecute(t, eng, r, repo, exp, createOp(thirdChangePath, thirdChange()))
		if res.Disposition != DispositionApplied {
			t.Fatalf("disposition = %q, want applied", res.Disposition)
		}
	})

	t.Run("stale blob contends first attempt", func(t *testing.T) {
		r := newMainModeRepos(t)
		client, repo := r.discover(t)
		eng := newEngine(t, client)
		base := r.originTip(t)
		exp := []EntityExpectation{{Path: gitcli.RepoPath(firstPath),
			Version: ExpectedVersion{Kind: VersionBlob, ObjectID: "0000000000000000000000000000000000000000"}}}
		op := createOp(thirdChangePath, thirdChange())
		res := mustExecute(t, eng, r, repo, exp, op)
		if res.Disposition != DispositionContended {
			t.Fatalf("disposition = %q, want contended", res.Disposition)
		}
		if len(res.ContendedPaths) != 1 || res.ContendedPaths[0] != gitcli.RepoPath(firstPath) {
			t.Errorf("contended paths = %v, want [%s]", res.ContendedPaths, firstPath)
		}
		if res.Attempts != 1 {
			t.Errorf("attempts = %d, want 1", res.Attempts)
		}
		if op.calls != 0 {
			t.Errorf("operation consulted %d times despite a first-attempt mismatch", op.calls)
		}
		if r.originTip(t) != base {
			t.Error("origin advanced on a contended expectation")
		}
		if !transactionsEmpty(t, repo) {
			t.Error("transactions root not empty after contention")
		}
	})

	t.Run("absent expectation passes when path is absent", func(t *testing.T) {
		r := newMainModeRepos(t)
		client, repo := r.discover(t)
		eng := newEngine(t, client)
		exp := []EntityExpectation{{Path: "docs/changes/active/9999-none.md",
			Version: ExpectedVersion{Kind: VersionAbsent}}}
		res := mustExecute(t, eng, r, repo, exp, createOp(thirdChangePath, thirdChange()))
		if res.Disposition != DispositionApplied {
			t.Fatalf("disposition = %q, want applied", res.Disposition)
		}
	})

	t.Run("absent expectation contends when path is present", func(t *testing.T) {
		r := newMainModeRepos(t)
		client, repo := r.discover(t)
		eng := newEngine(t, client)
		exp := []EntityExpectation{{Path: gitcli.RepoPath(firstPath),
			Version: ExpectedVersion{Kind: VersionAbsent}}}
		res := mustExecute(t, eng, r, repo, exp, createOp(thirdChangePath, thirdChange()))
		if res.Disposition != DispositionContended {
			t.Fatalf("disposition = %q, want contended", res.Disposition)
		}
	})
}

// TestEngineRejectsNonBranchTargetRef proves a tag ref or a short name is a
// call-shape failure: a Go *Failure of kind invalid-input, before any Git work.
func TestEngineRejectsNonBranchTargetRef(t *testing.T) {
	r := newMainModeRepos(t)
	client, repo := r.discover(t)
	eng := newEngine(t, client)

	for _, ref := range []gitcli.RefName{"refs/tags/v1", "main", "refs/remotes/origin/main"} {
		_, err := eng.Execute(context.Background(), Request{
			Repository: repo, Remote: "origin", TargetRef: ref,
			Loader: testLoader{}, Operation: createOp(thirdChangePath, thirdChange()),
		})
		if err == nil {
			t.Fatalf("target ref %q: want error", ref)
		}
		assertFailureKind(t, err, KindInvalidInput)
	}
}

// TestEngineFailsOnMissingTargetBranch proves a missing target branch is a typed
// failure and never triggers branch creation.
func TestEngineFailsOnMissingTargetBranch(t *testing.T) {
	r := newMainModeRepos(t)
	client, repo := r.discover(t)
	eng := newEngine(t, client)

	res, err := eng.Execute(context.Background(), Request{
		Repository: repo, Remote: "origin", TargetRef: "refs/heads/nope",
		Loader: testLoader{}, Operation: createOp(thirdChangePath, thirdChange()),
	})
	if err == nil {
		t.Fatal("missing target branch: want error")
	}
	f, ok := AsFailure(err)
	if !ok {
		t.Fatalf("error is not *Failure: %v", err)
	}
	if f.Stage != StageFetch {
		t.Errorf("failure stage = %q, want fetch", f.Stage)
	}
	if res.Disposition != DispositionFailed {
		t.Errorf("disposition = %q, want failed", res.Disposition)
	}
	if _, gerr := hgitTry(r.Origin, "rev-parse", "--verify", "--quiet", "refs/heads/nope"); gerr == nil {
		t.Error("engine created the missing target branch on origin")
	}
}

// TestEngineRetriesLeaseLoss proves a structurally proven lease loss retries from
// a fresh fetch and a fresh plan, applying on the next attempt — the reused first
// plan never appears in the winning commit.
func TestEngineRetriesLeaseLoss(t *testing.T) {
	r := newMainModeRepos(t)
	client, repo := r.discover(t)
	eng := newEngine(t, client)

	op := createOp(thirdChangePath, thirdChange())
	op.beforePlan = func(call int) {
		if call == 1 {
			// Advance origin between fetch and push so attempt 1 loses the lease.
			r.advanceOrigin(t, "docs/changes/active/0005-writer-change.md",
				corpusChange(5, "writer-change", "proposed"))
		}
	}
	res, err := eng.Execute(context.Background(), Request{
		Repository: repo, Remote: "origin", TargetRef: r.Target,
		Loader: testLoader{}, Operation: op,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Disposition != DispositionApplied {
		t.Fatalf("disposition = %q, want applied", res.Disposition)
	}
	if res.Attempts != 2 {
		t.Errorf("attempts = %d, want 2 (one lease loss then apply)", res.Attempts)
	}
	if op.calls != 2 {
		t.Errorf("operation re-planned %d times, want 2", op.calls)
	}
	// Final origin carries both the writer's change and the engine's.
	names := hgitOut(t, r.Origin, "ls-tree", "-r", "--name-only", string(r.Target))
	for _, want := range []string{thirdChangePath, "docs/changes/active/0005-writer-change.md"} {
		if !strings.Contains(names, want) {
			t.Errorf("final origin tree missing %q:\n%s", want, names)
		}
	}
	if !transactionsEmpty(t, repo) {
		t.Error("transactions root not empty after a retried apply")
	}
}

// TestEngineConcurrentExecuteIsRaceFree runs two Execute calls on one shared Engine
// against two independent repositories, coordinated by a barrier, to catch shared
// state races under -race.
func TestEngineConcurrentExecuteIsRaceFree(t *testing.T) {
	requireGit(t)
	client, err := gitcli.NewClient()
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	eng := newEngine(t, client)

	r1 := newMainModeRepos(t)
	r2 := newDocketModeRepos(t)
	repo1, err := client.Discover(context.Background(), gitcli.DiscoverOptions{InvocationPath: r1.Invocation})
	if err != nil {
		t.Fatalf("Discover r1: %v", err)
	}
	repo2, err := client.Discover(context.Background(), gitcli.DiscoverOptions{InvocationPath: r2.Invocation})
	if err != nil {
		t.Fatalf("Discover r2: %v", err)
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	run := func(repo gitcli.Repository, ref gitcli.RefName, out *Result, rerr *error) {
		defer wg.Done()
		<-start
		res, e := eng.Execute(context.Background(), Request{
			Repository: repo, Remote: "origin", TargetRef: ref,
			Loader: testLoader{}, Operation: createOp(thirdChangePath, thirdChange()),
		})
		*out, *rerr = res, e
	}
	var res1, res2 Result
	var err1, err2 error
	wg.Add(2)
	go run(repo1, r1.Target, &res1, &err1)
	go run(repo2, r2.Target, &res2, &err2)
	close(start)
	wg.Wait()

	if err1 != nil || res1.Disposition != DispositionApplied {
		t.Errorf("goroutine 1: disposition %q err %v", res1.Disposition, err1)
	}
	if err2 != nil || res2.Disposition != DispositionApplied {
		t.Errorf("goroutine 2: disposition %q err %v", res2.Disposition, err2)
	}
}

// topology names a harness builder so a test can run against both metadata shapes.
type topology struct {
	name  string
	build func(*testing.T) *testRepos
}

func topologies() []topology {
	return []topology{
		{"main", newMainModeRepos},
		{"docket", newDocketModeRepos},
	}
}

// mustExecute runs a standard single-operation transaction and fails on a Go error.
func mustExecute(t *testing.T, eng *Engine, r *testRepos, repo gitcli.Repository,
	exp []EntityExpectation, op SemanticOperation) Result {
	t.Helper()
	res, err := eng.Execute(context.Background(), Request{
		Repository: repo, Remote: "origin", TargetRef: r.Target,
		Expected: exp, Loader: testLoader{}, Operation: op,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	return res
}
