package transaction

import (
	"context"
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
