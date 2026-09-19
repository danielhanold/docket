package gatedrive

import (
	"errors"
	"sync"
	"testing"

	"github.com/danielhanold/docket/internal/process"
)

// ---------------------------------------------------------------------------
// StartAdmitted revalidation, per-drive claim across launch, delayed-ticket
// refusal (change 0437 Task 2). Between Admit and StartAdmitted a cancellation
// fence can revoke the epoch, or a rotated/foreign reservation can take the
// slot. StartAdmitted re-reads the EXACT durable reservation the ticket minted
// under the epoch gate and holds the drive's claimant flock across launch/attach
// so a concurrent cancellation observes pending work rather than a free slot.
// ---------------------------------------------------------------------------

// flippableGate is a fake EpochLaunchGate whose verdict can be flipped between
// Admit and StartAdmitted, simulating a cancellation fence that lands in the
// window between the two phases. Permissive by default (it runs reserve); once
// refusing, it returns its sentinel WITHOUT running reserve — the EpochLaunchGate
// contract: a validation failure NEVER calls reserve.
type flippableGate struct {
	mu     sync.Mutex
	refuse bool
	err    error
	calls  int
}

func (g *flippableGate) setRefuse(v bool) {
	g.mu.Lock()
	g.refuse = v
	g.mu.Unlock()
}

func (g *flippableGate) callCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.calls
}

func (g *flippableGate) gate() EpochLaunchGate {
	return func(_, _ string, reserve func() error) error {
		g.mu.Lock()
		g.calls++
		refuse := g.refuse
		g.mu.Unlock()
		if refuse {
			return g.err
		}
		return reserve()
	}
}

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

// TestStartAdmittedRevalidatesEpoch proves a fence landing between Admit and
// StartAdmitted refuses the delayed launch: the gate is flipped to refuse after a
// permissive Admit, so StartAdmitted returns the gate's typed error, launches
// nothing, fail-closed settles the reserved drive record HALTED "run-cancelled",
// and releases the freshly reserved worktree slot (never-launched by
// construction — no Launch call happened).
func TestStartAdmittedRevalidatesEpoch(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())
	sentinel := errors.New("gatedrive-test: epoch fence between admit and start")
	g := &flippableGate{err: sentinel}
	d.SetEpochLaunchGate(g.gate())

	req := sampleStart()
	req.RunEpochID = "e1"
	ticket, err := d.Admit(req)
	if err != nil {
		t.Fatalf("Admit: %v", err)
	}

	// A cancellation fence lands between the two phases.
	g.setRefuse(true)

	if _, serr := d.StartAdmitted(ticket); !errors.Is(serr, sentinel) {
		t.Fatalf("StartAdmitted error = %v, want the gate's sentinel", serr)
	}
	if proc.launchN != 0 {
		t.Fatalf("a revoked epoch must launch nothing, proc.Launch called %d times", proc.launchN)
	}

	// The delayed ticket's reserved drive record is fail-closed settled HALTED.
	rec, lerr := store.Load(ticket.id)
	if lerr != nil {
		t.Fatalf("Load drive record: %v", lerr)
	}
	if rec.LastOutcome != HALTED || rec.LastCause != "run-cancelled" {
		t.Fatalf("drive record settled (%v,%q), want (HALTED,%q)", rec.LastOutcome, rec.LastCause, "run-cancelled")
	}

	// The freshly reserved worktree slot is released (provably idle).
	slot, _, werr := store.LoadWorktreeExecution(req.Worktree)
	if werr != nil {
		t.Fatalf("LoadWorktreeExecution: %v", werr)
	}
	if slot.State != admissionReleased {
		t.Fatalf("worktree slot state = %q, want %q (released)", slot.State, admissionReleased)
	}
}

// TestStartAdmittedRefusesForeignReservation proves the revalidation re-reads the
// EXACT durable reservation the ticket minted: after Admit, the slot is released
// and reserved anew (a rotated/foreign reservation under the same epoch now owns
// the slot with a different token). StartAdmitted refuses
// ErrUnresolvedLaunchTransition without launching.
func TestStartAdmittedRefusesForeignReservation(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())
	// A permissive gate: the epoch stays live, so the refusal must come from the
	// exact-reservation revalidation, never the epoch check.
	g := &flippableGate{}
	d.SetEpochLaunchGate(g.gate())

	req := sampleStart()
	req.RunEpochID = "e1"
	ticket, err := d.Admit(req)
	if err != nil {
		t.Fatalf("Admit: %v", err)
	}

	// Release this ticket's slot and reserve the worktree anew under the same
	// epoch: a rotated/foreign reservation now owns the slot with a different token.
	if rerr := store.ReleaseWorktreeExecution(req.Worktree, ticket.token); rerr != nil {
		t.Fatalf("ReleaseWorktreeExecution: %v", rerr)
	}
	newToken, _, rerr := store.reserveWorktreeExecution(admissionRecord{
		RepoIdentity: req.RepoDir,
		WorktreeRoot: req.Worktree,
		RunEpochID:   "e1",
		Kind:         "scopeless",
	}, proc)
	if rerr != nil {
		t.Fatalf("reserve foreign slot: %v", rerr)
	}
	if newToken == ticket.token {
		t.Fatal("the foreign reservation must mint a different token")
	}

	_, serr := d.StartAdmitted(ticket)
	oe, ok := AsOwnershipError(serr)
	if !ok || oe.Kind != ErrUnresolvedLaunchTransition {
		t.Fatalf("StartAdmitted error = %v, want ErrUnresolvedLaunchTransition", serr)
	}
	if proc.launchN != 0 {
		t.Fatalf("a foreign reservation must launch nothing, proc.Launch called %d times", proc.launchN)
	}
}

// TestStartAdmittedRefusesBusyClaim proves a held claimant flock refuses the
// launch without launching and without waiting: the test holds the drive's claim
// (a concurrent launch/attach is in flight), so StartAdmitted returns promptly
// with a typed refusal. The done channel — not a timer — is the promptness oracle:
// tryRelaunchClaim is nonblocking, so StartAdmitted must return without the test
// ever releasing the claim.
func TestStartAdmittedRefusesBusyClaim(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())
	g := &flippableGate{}
	d.SetEpochLaunchGate(g.gate())

	req := sampleStart()
	req.RunEpochID = "e1"
	ticket, err := d.Admit(req)
	if err != nil {
		t.Fatalf("Admit: %v", err)
	}

	held, busy, cerr := store.tryRelaunchClaim(ticket.id)
	if cerr != nil {
		t.Fatalf("tryRelaunchClaim: %v", cerr)
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
}

// TestStartAdmittedHoldsClaimAcrossLaunch proves the drive's claimant flock is
// HELD across the launch: a launch parked on a barrier makes a concurrent
// tryRelaunchClaim report busy (pending work, never a crashed caller); once
// launch+attach complete the claim is free again so the drive slice can reserve
// its own relaunch.
func TestStartAdmittedHoldsClaimAcrossLaunch(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())
	g := &flippableGate{}
	d.SetEpochLaunchGate(g.gate())

	req := sampleStart()
	req.RunEpochID = "e1"
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
	c, busy, cerr := store.tryRelaunchClaim(ticket.id)
	if cerr != nil {
		t.Fatalf("tryRelaunchClaim during launch: %v", cerr)
	}
	if !busy {
		c.close()
		t.Fatal("the claim must be HELD (busy) while launch is in flight")
	}

	close(release)
	if serr := <-done; serr != nil {
		t.Fatalf("StartAdmitted: %v", serr)
	}

	free, busyAfter, cerr := store.tryRelaunchClaim(ticket.id)
	if cerr != nil {
		t.Fatalf("tryRelaunchClaim after launch: %v", cerr)
	}
	if busyAfter {
		t.Fatal("the claim must be FREE again after launch+attach return")
	}
	free.close()
}

// TestStartAdmittedEpochlessUnchanged proves the epoch-less standalone path is
// preserved: an empty RunEpochID consults no gate and launches exactly as today —
// but STILL under the claim (the claim discipline is unconditional; only the
// epoch validation is conditional).
func TestStartAdmittedEpochlessUnchanged(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())
	g := &flippableGate{}
	d.SetEpochLaunchGate(g.gate())

	req := sampleStart() // RunEpochID == ""
	ticket, err := d.Admit(req)
	if err != nil {
		t.Fatalf("Admit: %v", err)
	}

	entered := make(chan struct{})
	release := make(chan struct{})
	proc.launch = blockingLaunch(entered, release)

	done := make(chan struct {
		doc DriveDoc
		err error
	}, 1)
	go func() {
		doc, serr := d.StartAdmitted(ticket)
		done <- struct {
			doc DriveDoc
			err error
		}{doc, serr}
	}()

	<-entered // even epoch-less, the claim is held across launch
	c, busy, cerr := store.tryRelaunchClaim(ticket.id)
	if cerr != nil {
		t.Fatalf("tryRelaunchClaim during epoch-less launch: %v", cerr)
	}
	if !busy {
		c.close()
		t.Fatal("the claim discipline is unconditional: it must be held even without an epoch")
	}

	close(release)
	res := <-done
	if res.err != nil {
		t.Fatalf("StartAdmitted (epoch-less): %v", res.err)
	}
	if g.callCount() != 0 {
		t.Fatalf("an empty-epoch StartAdmitted must NOT consult the gate, called %d times", g.callCount())
	}
	if proc.launchN != 1 {
		t.Fatalf("an epoch-less start must launch exactly once, proc.Launch called %d times", proc.launchN)
	}
	if res.doc.Outcome != WAITING {
		t.Fatalf("a live epoch-less start returns WAITING, got %v", res.doc.Outcome)
	}
}
