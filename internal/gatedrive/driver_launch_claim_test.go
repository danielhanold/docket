package gatedrive

import (
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/process"
	"github.com/danielhanold/docket/internal/testsupport"
)

// ---------------------------------------------------------------------------
// The per-drive launch claim (change 0437 Task 2). StartAdmitted re-reads the
// ticket's reserved drive and holds the drive's claimant flock across
// launch/attach, so a concurrent census observes pending work rather than a
// crashed caller. The worktree itself is held by the ticket's worktree lock
// (change 0490), so no other gate can take it between the two phases. No run is
// checked at either phase (change 0491).
// ---------------------------------------------------------------------------

// blockingLaunch returns a proc.Launch closure that closes entered, blocks until
// release is closed, then returns a fresh running launch outcome. It is the
// deterministic barrier the claim-across-launch tests park a launch on.
func blockingLaunch(entered, release chan struct{}) func(process.LaunchRequest) (*process.LaunchOutcome, error) {
	return func(process.LaunchRequest) (*process.LaunchOutcome, error) {
		close(entered)
		<-release
		return &process.LaunchOutcome{RunID: "run1", RunDir: "/runs/run1", State: process.StateRunning}, nil
	}
}

// TestStartAdmittedRefusesBusyClaim proves a held claimant flock refuses the
// launch without launching and without waiting: the test holds the drive's claim
// (a concurrent launch/attach is in flight), so StartAdmitted returns promptly
// with a typed refusal and frees the ticket's worktree. The done channel — not a timer — is the promptness oracle:
// tryDriveClaim is nonblocking, so StartAdmitted must return without the test
// ever releasing the claim.
func TestStartAdmittedRefusesBusyClaim(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())

	req := sampleStart()
	ticket, err := d.Admit(req)
	if err != nil {
		t.Fatalf("Admit: %v", err)
	}

	held, busy, cerr := store.tryDriveClaim(ticket.id)
	if cerr != nil {
		t.Fatalf("tryDriveClaim: %v", cerr)
	}
	if busy {
		t.Fatal("the drive's claim must be free before the test holds it")
	}
	defer held.close()

	done := make(chan error, 1)
	go func() {
		_, serr := d.StartAdmitted(ticket)
		done <- serr
	}()

	serr := <-done // no timer: a nonblocking claim probe must return without the release
	oe, ok := AsOwnershipError(serr)
	if !ok || oe.Kind != ErrUnresolvedLaunchTransition {
		t.Fatalf("StartAdmitted error = %v, want ErrUnresolvedLaunchTransition", serr)
	}
	if proc.launchN != 0 {
		t.Fatalf("a busy claim must launch nothing, proc.Launch called %d times", proc.launchN)
	}
	if !worktreeFree(t, store, req.Cwd) {
		t.Fatalf("a refused launch must close the ticket's worktree lock")
	}
}

// TestStartAdmittedHoldsClaimAcrossLaunch proves the drive's claimant flock is
// HELD across the launch: a launch parked on a barrier makes a concurrent
// tryDriveClaim report busy (pending work, never a crashed caller); once
// launch+attach complete the claim is free again before the drive slice.
func TestStartAdmittedHoldsClaimAcrossLaunch(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())

	req := sampleStart()
	ticket, err := d.Admit(req)
	if err != nil {
		t.Fatalf("Admit: %v", err)
	}

	entered := make(chan struct{})
	release := make(chan struct{})
	proc.launch = blockingLaunch(entered, release)

	done := make(chan error, 1)
	go func() {
		_, serr := d.StartAdmitted(ticket)
		done <- serr
	}()

	<-entered // launch is parked: the claim is held across it
	c, busy, cerr := store.tryDriveClaim(ticket.id)
	if cerr != nil {
		t.Fatalf("tryDriveClaim during launch: %v", cerr)
	}
	if !busy {
		c.close()
		t.Fatal("the claim must be HELD (busy) while launch is in flight")
	}

	close(release)
	if serr := <-done; serr != nil {
		t.Fatalf("StartAdmitted: %v", serr)
	}

	free, busyAfter, cerr := store.tryDriveClaim(ticket.id)
	if cerr != nil {
		t.Fatalf("tryDriveClaim after launch: %v", cerr)
	}
	if busyAfter {
		t.Fatal("the claim must be FREE again after launch+attach return")
	}
	free.close()
}

// ---------------------------------------------------------------------------
// Contention proofs (change 0437 Task 7). The nonblocking per-drive claim bounds
// every contender — all return, and exactly one launch happens — proved with
// channel/done oracles, never a timing sleep.
// ---------------------------------------------------------------------------

// TestClaimContentionBounded proves the nonblocking per-drive claim bounds
// contention with no deadlock: StartAdmitted parks inside Launch holding the
// drive's claim while several reconcilers probe the drive. Every reconciler
// returns promptly with claim-busy (done-channel oracles, never a timing sleep
// as the ordering fact), and the launch completes after release with exactly one
// backend launch. A drive never relaunches (change 0493), so StartAdmitted is the
// claim's only launcher.
func TestClaimContentionBounded(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())
	req := sampleStart()
	req.RunContext = "ctx-e1"
	ticket, err := d.Admit(req)
	if err != nil {
		t.Fatalf("Admit: %v", err)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	proc.launch = blockingLaunch(entered, release)
	launched := make(chan error, 1)
	go func() {
		_, serr := d.StartAdmitted(ticket)
		launched <- serr
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("StartAdmitted did not enter Launch")
	}

	const reconcilers = 4
	reconcileDone := make(chan RunLaunchReport, reconcilers)
	for i := 0; i < reconcilers; i++ {
		go func() {
			dr := NewDriver(reopenStore(store), &fakeClock{now: startRun()}, &fakeProc{}, stableGit())
			r, _ := dr.ReconcileRunLaunches(capHash(req.RunContext))
			reconcileDone <- r
		}()
	}
	for i := 0; i < reconcilers; i++ {
		select {
		case report := <-reconcileDone:
			if report.Accounted || !reconcileFindingPresent(report.Findings, "claim-busy:"+ticket.id) {
				t.Errorf("a held claim must read claim-busy:%s and stay unaccounted, got %+v", ticket.id, report)
			}
		case <-time.After(5 * time.Second):
			close(release)
			t.Fatal("a reconcile blocked on the held claim; it must probe nonblocking")
		}
	}

	close(release)
	select {
	case serr := <-launched:
		if serr != nil {
			t.Fatalf("StartAdmitted: %v", serr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("StartAdmitted did not return after release")
	}
	if proc.launchN != 1 {
		t.Fatalf("exactly one backend launch under contention, got %d", proc.launchN)
	}
}

// TestStartAdmittedRefusesAfterVerdictSettle (change 0491, spec T4): a launcher
// delayed between Admit and StartAdmitted, still holding its in-memory ticket,
// finds its reserved record settled HALTED launch-abandoned by the keyed verdict.
// StartAdmitted re-reads the record under the claim, refuses, launches nothing,
// and frees the worktree — exactly as after cancel's settle.
func TestStartAdmittedRefusesAfterVerdictSettle(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{} // a nil resolve answers never-launched
	d, store := newTestDriver(t, clk, proc, stableGit())
	req := sampleStart()
	req.RunContext = "ctx-0491"
	req.RunRoot = testsupport.TempDir(t) // exists, holds no reservation: never launched
	ticket, err := d.Admit(req)
	if err != nil {
		t.Fatalf("Admit: %v", err)
	}

	report, err := d.VerdictRunLaunches(capHash(req.RunContext))
	if err != nil {
		t.Fatalf("VerdictRunLaunches: %v", err)
	}
	if !report.Accounted {
		t.Fatalf("a proven never-launched admission must be accounted, got %+v", report)
	}
	if proc.resolveN != 1 {
		t.Fatalf("the verdict must resolve the existing root's launch token, ResolveReservation called %d times", proc.resolveN)
	}
	rec, err := store.Load(ticket.id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if rec.LastOutcome != HALTED || rec.LastCause != "launch-abandoned" {
		t.Fatalf("want HALTED launch-abandoned, got %v/%q", rec.LastOutcome, rec.LastCause)
	}

	_, serr := d.StartAdmitted(ticket)
	if oe, ok := AsOwnershipError(serr); !ok || oe.Kind != ErrUnresolvedLaunchTransition {
		t.Fatalf("StartAdmitted over a settled record must refuse ErrUnresolvedLaunchTransition, got %v", serr)
	}
	if proc.launchN != 0 {
		t.Fatalf("a settled admission must launch nothing, proc.Launch called %d times", proc.launchN)
	}
	if !worktreeFree(t, store, req.Cwd) {
		t.Fatal("a refused launch must free the worktree lock")
	}
}
