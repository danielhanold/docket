//go:build integration

package app

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// race shard (change 0333): 16 goroutines contend on the on-disk gate record; -race guards the O_EXCL single-grant CAS in ConsumeGateRetry.
func TestRaceIntegrationAppConcurrencyGateRetryConcurrentExactlyOne(t *testing.T) {
	repo := newGateRepo(t)
	key, err := MintGateRecord(repo, sampleGateRecord())
	if err != nil {
		t.Fatalf("MintGateRecord: %v", err)
	}

	const n = 16
	var wg sync.WaitGroup
	results := make(chan bool, n)
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			ok, cerr := ConsumeGateRetry(repo, key)
			if cerr != nil {
				t.Errorf("ConsumeGateRetry: %v", cerr)
			}
			results <- ok
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	trues := 0
	for ok := range results {
		if ok {
			trues++
		}
	}
	if trues != 1 {
		t.Fatalf("concurrent ConsumeGateRetry granted %d permits, want exactly 1", trues)
	}
}

// race shard (change 0333): two goroutines allocate ADR IDs concurrently; -race guards the shared allocation path.
func TestRaceIntegrationAppConcurrencyPlanningConcurrentADRRecordsAllocateDistinctIDs(t *testing.T) {
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

// race shard (change 0333): two goroutines allocate change IDs concurrently; -race guards the shared allocation path.
func TestRaceIntegrationAppConcurrencyPlanningConcurrentCreatesAllocateDistinctIDs(t *testing.T) {
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

// race shard (change 0333): two goroutines drive concurrent block/defer through separate clones onto a shared origin; -race guards the shared adapter/transaction paths.
func TestRaceIntegrationAppConcurrencyPlanningConcurrentUnrelatedMutationsBothLand(t *testing.T) {
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

// race shard (change 0333): two goroutines mutate the same entity concurrently; -race guards the shared adapter/transaction paths.
func TestRaceIntegrationAppConcurrencyPlanningSameEntityVersionOneAppliesOneContends(t *testing.T) {
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

// race shard (change 0333): two concurrent RunGateVerdict calls contend on the on-disk gate record; -race guards the single-grant CAS.
// TestRunGateVerdictConcurrentRetryGrantsOnce is the mutation target: two
// concurrent verdict calls on one not-implemented run must grant EXACTLY ONE
// gate-retry-once (the O_EXCL CAS in ConsumeGateRetry, consumed before the report
// is chosen). Reversing the consume-then-emit order — deciding from the record's
// stale Retry mirror and consuming afterward — double-grants and reddens here.
func TestRaceIntegrationAppConcurrencyRunGateVerdictConcurrentRetryGrantsOnce(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	ev := string(prEvidenceBytes(t, f.head))
	key := gateMintArmed(t, f.repo.invocation, nil, 1)

	// Each goroutine gets its OWN deps triple: in production the two concurrent
	// verdict calls are separate processes, each with its own reader/workspace/
	// GitHub adapters. The in-memory fakes record their calls without locks, so
	// sharing one triple across both goroutines races under -race on that
	// bookkeeping — a test-double artifact, not the behavior under test. The only
	// resource the two calls genuinely contend on is the on-disk gate record under
	// f.repo.invocation, whose single-grant guarantee is the O_EXCL CAS in
	// ConsumeGateRetry — that contention is preserved.
	var wg sync.WaitGroup
	results := make([]RunGateVerdictResult, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		deps, wdeps, gdeps := f.deps(
			rvInProgressRecord(rvPlanPath, rvResultsPath, "feat/"+rvSlug),
			rvPR(f.head, ev),
		)
		go func(idx int, deps PlanningDeps, wdeps WorkspaceDeps, gdeps GitHubDeps) {
			defer wg.Done()
			results[idx] = RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
		}(i, deps, wdeps, gdeps)
	}
	wg.Wait()

	retryOnce, stop := 0, 0
	for _, r := range results {
		switch r.Decision {
		case GateDecisionRetryOnce:
			retryOnce++
		case GateDecisionStop:
			stop++
		default:
			t.Fatalf("unexpected decision %q (%q)", r.Decision, r.HumanText())
		}
	}
	if retryOnce != 1 || stop != 1 {
		t.Fatalf("gate-retry-once=%d gate-stop=%d, want exactly 1 and 1 (single retry permit)", retryOnce, stop)
	}
}
