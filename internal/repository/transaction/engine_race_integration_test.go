//go:build integration

package transaction

import (
	"context"
	"sync"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
)

// TestRaceIntegrationTxnEngineConcurrentExecuteIsRaceFree runs two Execute calls on one shared Engine
// against two independent repositories, coordinated by a barrier, to catch shared
// state races under -race.
// Race shard (change 0466): concurrent Execute calls share one engine.
func TestRaceIntegrationTxnEngineConcurrentExecuteIsRaceFree(t *testing.T) {
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
