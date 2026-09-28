//go:build integration

package workspace

import (
	"context"
	"sync"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
)

// TestRaceIntegrationWorkspacePrepareConcurrentSameTarget proves two Prepares of the SAME target
// serialize on the operation lock and yield exactly one created plus one
// existing, with exactly one branch and one registration.
// Race shard (change 0466): concurrent Prepare calls race for one target.
func TestRaceIntegrationWorkspacePrepareConcurrentSameTarget(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)

	var wg sync.WaitGroup
	results := make([]Workspace, 2)
	errs := make([]error, 2)
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i], errs[i] = svc.Prepare(context.Background(), PrepareRequest{Repository: repo, Remote: "origin", Target: tgt})
		}(i)
	}
	close(start)
	wg.Wait()

	created, existing := 0, 0
	for i := 0; i < 2; i++ {
		if errs[i] != nil {
			t.Fatalf("goroutine %d Prepare: %v", i, errs[i])
		}
		switch results[i].Disposition {
		case PrepareCreated:
			created++
		case PrepareExisting, PrepareResumed:
			existing++
		default:
			t.Errorf("goroutine %d disposition = %q; want created/existing/resumed", i, results[i].Disposition)
		}
	}
	if created != 1 || existing != 1 {
		t.Errorf("dispositions: created=%d existing/resumed=%d; want 1 and 1", created, existing)
	}

	// Exactly one branch and one registration for the target.
	wl := gitOut(t, r.Primary, "worktree", "list", "--porcelain")
	if n := countPathOccurrences(wl, wsPathOf(repo)); n != 1 {
		t.Errorf("registrations at target path = %d; want 1", n)
	}
	if !branchExists(r.Primary, "feat/"+prepSlug) {
		t.Errorf("feat branch missing after concurrent prepare")
	}
}

// TestRaceIntegrationWorkspacePrepareConcurrentDistinctTargets proves two Prepares of DIFFERENT targets
// proceed concurrently (distinct operation locks) and both create successfully.
// Race shard (change 0466): concurrent Prepare calls for distinct targets share one primary clone.
func TestRaceIntegrationWorkspacePrepareConcurrentDistinctTargets(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)

	baseA := resolveBase(t, []domain.ChangeSpec{{ID: 7, Status: domain.StatusProposed}}, nil, 7)
	tgtA, err := NewTarget(7, "alpha-slug", baseA, "feat/alpha-slug")
	if err != nil {
		t.Fatalf("NewTarget A: %v", err)
	}
	baseB := resolveBase(t, []domain.ChangeSpec{{ID: 8, Status: domain.StatusProposed}}, nil, 8)
	tgtB, err := NewTarget(8, "beta-slug", baseB, "feat/beta-slug")
	if err != nil {
		t.Fatalf("NewTarget B: %v", err)
	}

	var wg sync.WaitGroup
	var outA, outB Workspace
	var errA, errB error
	start := make(chan struct{})
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		outA, errA = svc.Prepare(context.Background(), PrepareRequest{Repository: repo, Remote: "origin", Target: tgtA})
	}()
	go func() {
		defer wg.Done()
		<-start
		outB, errB = svc.Prepare(context.Background(), PrepareRequest{Repository: repo, Remote: "origin", Target: tgtB})
	}()
	close(start)
	wg.Wait()

	if errA != nil || errB != nil {
		t.Fatalf("distinct-target Prepare errors: A=%v B=%v", errA, errB)
	}
	if outA.Disposition != PrepareCreated || outB.Disposition != PrepareCreated {
		t.Errorf("dispositions A=%q B=%q; want both created", outA.Disposition, outB.Disposition)
	}
}
