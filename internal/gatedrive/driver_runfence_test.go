package gatedrive

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/process"
	"github.com/danielhanold/docket/internal/testsupport"
)

// ---------------------------------------------------------------------------
// StartAdmitted revalidation, per-drive claim across launch, delayed-ticket
// refusal (change 0437 Task 2). Between Admit and StartAdmitted a cancellation
// fence can revoke the run. StartAdmitted re-reads the ticket's reserved drive
// under the run launch gate (kept for change 0491) and holds the drive's
// claimant flock across launch/attach so a concurrent cancellation observes
// pending work. The worktree itself is held by the ticket's worktree lock
// (change 0490), so no other gate can take it between the two phases.
// ---------------------------------------------------------------------------

// flippableGate is a fake RunLaunchGate whose verdict can be flipped between
// Admit and StartAdmitted, simulating a cancellation fence that lands in the
// window between the two phases. Permissive by default (it runs reserve); once
// refusing, it returns its sentinel WITHOUT running reserve — the RunLaunchGate
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

func (g *flippableGate) gate() RunLaunchGate {
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

// TestStartAdmittedRevalidatesRun proves a fence landing between Admit and
// StartAdmitted refuses the delayed launch: the gate is flipped to refuse after a
// permissive Admit, so StartAdmitted returns the gate's typed error, launches
// nothing, and fail-closed settles the reserved drive record HALTED
// "run-cancelled" (never-launched by construction — no Launch call happened).
// TestStartAdmittedRunRefusalFreesWorktree pins that the worktree frees too.
func TestStartAdmittedRevalidatesRun(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())
	sentinel := errors.New("gatedrive-test: run fence between admit and start")
	g := &flippableGate{err: sentinel}
	d.SetRunLaunchGate(g.gate())

	req := sampleStart()
	req.RunID = "e1"
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
		t.Fatalf("a revoked run must launch nothing, proc.Launch called %d times", proc.launchN)
	}

	// The delayed ticket's reserved drive record is fail-closed settled HALTED.
	rec, lerr := store.Load(ticket.id)
	if lerr != nil {
		t.Fatalf("Load drive record: %v", lerr)
	}
	if rec.LastOutcome != HALTED || rec.LastCause != "run-cancelled" {
		t.Fatalf("drive record settled (%v,%q), want (HALTED,%q)", rec.LastOutcome, rec.LastCause, "run-cancelled")
	}
}

// TestStartAdmittedRefusesBusyClaim proves a held claimant flock refuses the
// launch without launching and without waiting: the test holds the drive's claim
// (a concurrent launch/attach is in flight), so StartAdmitted returns promptly
// with a typed refusal and frees the ticket's worktree. The done channel — not a timer — is the promptness oracle:
// tryRelaunchClaim is nonblocking, so StartAdmitted must return without the test
// ever releasing the claim.
func TestStartAdmittedRefusesBusyClaim(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())
	g := &flippableGate{}
	d.SetRunLaunchGate(g.gate())

	req := sampleStart()
	req.RunID = "e1"
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
	if !worktreeFree(t, store, req.Cwd) {
		t.Fatalf("a refused launch must close the ticket's worktree lock")
	}
}

// TestStartAdmittedHoldsClaimAcrossLaunch proves the drive's claimant flock is
// HELD across the launch: a launch parked on a barrier makes a concurrent
// tryRelaunchClaim report busy (pending work, never a crashed caller); once
// launch+attach complete the claim is free again so the drive slice can reserve
// its own relaunch.
func TestStartAdmittedHoldsClaimAcrossLaunch(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())
	g := &flippableGate{}
	d.SetRunLaunchGate(g.gate())

	req := sampleStart()
	req.RunID = "e1"
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

// TestStartAdmittedNoRunRecordUnchanged proves the no-run-record standalone path is
// preserved: an empty RunID consults no gate and launches exactly as today —
// but STILL under the claim (the claim discipline is unconditional; only the
// run validation is conditional).
func TestStartAdmittedNoRunRecordUnchanged(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())
	g := &flippableGate{}
	d.SetRunLaunchGate(g.gate())

	req := sampleStart() // RunID == ""
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

	<-entered // even no-run-record, the claim is held across launch
	c, busy, cerr := store.tryRelaunchClaim(ticket.id)
	if cerr != nil {
		t.Fatalf("tryRelaunchClaim during no-run-record launch: %v", cerr)
	}
	if !busy {
		c.close()
		t.Fatal("the claim discipline is unconditional: it must be held even without a run")
	}

	close(release)
	res := <-done
	if res.err != nil {
		t.Fatalf("StartAdmitted (no-run-record): %v", res.err)
	}
	if g.callCount() != 0 {
		t.Fatalf("an empty-run StartAdmitted must NOT consult the gate, called %d times", g.callCount())
	}
	if proc.launchN != 1 {
		t.Fatalf("a no-run-record start must launch exactly once, proc.Launch called %d times", proc.launchN)
	}
	if res.doc.Outcome != WAITING {
		t.Fatalf("a live no-run-record start returns WAITING, got %v", res.doc.Outcome)
	}
}

// ---------------------------------------------------------------------------
// The single automatic relaunch crosses no run launch gate (change 0490). No
// production drive both carries a run and can relaunch (only finalize's run-less
// local gate is idempotent), so the relaunch's admission is the worktree lock it
// re-takes, never a run check — whatever run the drive was started under.
// ---------------------------------------------------------------------------

// TestRelaunchCrossesNoRunGate proves a death's single automatic relaunch never
// consults the run launch gate: a drive started under a live run, and a legacy
// drive with no launch token, each earn their replacement with a gate that
// fails the test if consulted.
func TestRelaunchCrossesNoRunGate(t *testing.T) {
	tripGate := func(t *testing.T) RunLaunchGate {
		return func(_, _ string, _ func() error) error {
			t.Fatalf("the single relaunch must NOT consult the run launch gate")
			return nil
		}
	}

	t.Run("drive started under a run", func(t *testing.T) {
		clk := &fakeClock{now: startRun()}
		dead := false
		proc := &fakeProc{observe: func(runDir string) (*process.Observation, error) {
			if dead && strings.HasSuffix(runDir, "run1") {
				return obs(process.StateSignaled, runDir), nil
			}
			return obs(process.StateRunning, runDir), nil
		}}
		proc.stop = func(runDir, _ string) (*process.StopOutcome, error) {
			return &process.StopOutcome{State: process.StateSignaled, RunDir: runDir, Performed: false,
				Terminal: &process.Terminal{Kind: "signal", Signal: 9}}, nil
		}
		d, _ := newTestDriver(t, clk, proc, stableGit())
		d.SetRunLaunchGate((&flippableGate{}).gate())
		req := sampleStart()
		req.RunID = "e1"
		started, err := d.Start(req)
		if err != nil || started.Outcome != WAITING {
			t.Fatalf("run-backed first slice = %s (%v), want WAITING", started.Outcome, err)
		}

		d.SetRunLaunchGate(tripGate(t))
		dead = true
		doc, err := d.Advance(started.DriveID, started.Generation)
		if err != nil {
			t.Fatalf("Advance: %v", err)
		}
		if doc.Outcome != WAITING || doc.Attempt != 2 {
			t.Fatalf("relaunch = %s/%q attempt %d, want WAITING attempt 2", doc.Outcome, doc.Cause, doc.Attempt)
		}
		if proc.launchN != 2 {
			t.Fatalf("exactly one relaunch (two launches) must occur, got %d", proc.launchN)
		}
	})

	t.Run("legacy drive with no launch token", func(t *testing.T) {
		store := OpenStore(testsupport.TempDir(t))
		proc := &fakeProc{}
		// The seeded drive already owns /runs/run1; the single relaunch must mint a
		// DISTINCT run dir, else the replacement would collide with the dead original.
		proc.launch = func(process.LaunchRequest) (*process.LaunchOutcome, error) {
			return &process.LaunchOutcome{RunID: "run2", RunDir: "/runs/run2", State: process.StateRunning}, nil
		}
		proc.observe = func(runDir string) (*process.Observation, error) {
			if strings.HasSuffix(runDir, "run1") {
				return obs(process.StateSignaled, runDir), nil
			}
			return obs(process.StateRunning, runDir), nil
		}
		proc.stop = func(runDir, _ string) (*process.StopOutcome, error) {
			return &process.StopOutcome{State: process.StateSignaled, RunDir: runDir, Performed: false,
				Terminal: &process.Terminal{Kind: "signal", Signal: 9}}, nil
		}
		id, ownerGen := seedDrive(t, store, seedRecord(t)) // AdmissionToken == ""

		clk := &fakeClock{now: startRun().Add(time.Second)}
		d := NewDriver(reopenStore(store), clk, proc, stableGit())
		d.slice = 4 * pollTick
		d.pollInterval = pollTick
		d.sleep = func(dur time.Duration) { clk.advance(dur) }
		d.SetRunLaunchGate(tripGate(t))

		doc, err := d.Advance(id, ownerGen)
		if err != nil {
			t.Fatalf("Advance: %v", err)
		}
		if doc.Outcome != WAITING || doc.Attempt != 2 {
			t.Fatalf("legacy relaunch = %s/%q attempt %d, want WAITING attempt 2", doc.Outcome, doc.Cause, doc.Attempt)
		}
		if proc.launchN != 1 {
			t.Fatalf("legacy relaunch must launch the replacement exactly once, got %d", proc.launchN)
		}
	})
}

// ---------------------------------------------------------------------------
// Lock-order and no-deadlock proofs (change 0437 Task 7). The mandated order is
// run lock → per-drive launch claim; no path acquires the run while holding
// the claim. These prove it deterministically: a probing gate asserts the claim is
// still free at every gate ENTER, and a contention race proves the nonblocking
// claim bounds every contender (all return; exactly one launch) with channel/done
// oracles, never a timing sleep. Since change 0490 only the first launch crosses
// the run gate; the relaunch reserves under its claim alone.
// ---------------------------------------------------------------------------

// TestNoRunAcquisitionWhileClaimHeld proves the run launch gate is entered only while
// the per-drive claim is still free for the delayed StartAdmitted launch. A
// probing gate acquires the claim at entry: if it is ever already held by this
// caller when the gate is entered, the lock order (run before claim) is violated.
func TestNoRunAcquisitionWhileClaimHeld(t *testing.T) {
	// probeGate is a permissive gate that, at each ENTER, probes the drive's claim.
	// The lock order requires it FREE at every entry; the gate records enters and
	// whether any entry saw it already held.
	probeGate := func(store *Store, id string, enters *int, sawHeld *bool) RunLaunchGate {
		return func(_, _ string, reserve func() error) error {
			*enters++
			c, busy, cerr := store.tryRelaunchClaim(id)
			if cerr == nil {
				if busy {
					*sawHeld = true
				} else {
					c.close()
				}
			}
			return reserve()
		}
	}

	t.Run("StartAdmitted enters the gate before taking the claim", func(t *testing.T) {
		clk := &fakeClock{now: startRun()}
		proc := &fakeProc{}
		d, store := newTestDriver(t, clk, proc, stableGit())
		// Admit under a plain permissive gate so the drive id exists to key the probe.
		d.SetRunLaunchGate((&flippableGate{}).gate())
		req := sampleStart()
		req.RunID = "e1"
		ticket, err := d.Admit(req)
		if err != nil {
			t.Fatalf("Admit: %v", err)
		}
		enters, sawHeld := 0, false
		d.SetRunLaunchGate(probeGate(store, ticket.id, &enters, &sawHeld))

		doc, serr := d.StartAdmitted(ticket)
		if serr != nil {
			t.Fatalf("StartAdmitted: %v", serr)
		}
		if doc.Outcome != WAITING {
			t.Fatalf("a live launch WAITs, got %s/%s", doc.Outcome, doc.Cause)
		}
		if enters == 0 {
			t.Fatalf("StartAdmitted must consult the run launch gate")
		}
		if sawHeld {
			t.Fatalf("the run launch gate must be entered while the per-drive claim is still FREE (run before claim)")
		}
	})
}

// TestClaimContentionBounded proves the nonblocking per-drive claim bounds
// contention with no deadlock: N advancers race one dead drive's single relaunch
// while M reconcilers probe it, with the reservation winner parked inside Launch.
// Every contender returns (done-channel oracles, never a timing sleep as the
// ordering fact): the losing advancers return authoritative state promptly, the
// reconcilers report claim-busy promptly, and the winner returns after release —
// with EXACTLY ONE backend launch, one relaunch, and never a crash-recovery
// resolution of the live holder.
func TestClaimContentionBounded(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	proc := newClaimWindowProc()
	// The drive's run context attributes it to e1's census.
	rec := seedRecord(t)
	rec.AdmissionToken = "aaaaaaaaaaaaaaaa"
	rec.RunContextHash = capHash("ctx-e1")
	id, ownerGen := seedDrive(t, store, rec)

	permissive := &flippableGate{}
	mkDriver := func(seam ProcessSeam) *Driver {
		clk := &fakeClock{now: startRun().Add(time.Second)}
		d := NewDriver(reopenStore(store), clk, seam, stableGit())
		d.slice = 4 * pollTick
		d.pollInterval = pollTick
		d.sleep = func(dur time.Duration) { clk.advance(dur) }
		d.SetRunLaunchGate(permissive.gate())
		return d
	}

	const advancers = 4
	const reconcilers = 2

	advanceDone := make(chan error, advancers)
	for i := 0; i < advancers; i++ {
		go func() {
			_, err := mkDriver(proc).Advance(id, ownerGen)
			advanceDone <- err
		}()
	}

	// The reservation winner parks inside Launch holding the claim.
	select {
	case <-proc.launchEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("the reservation winner did not enter Launch")
	}

	// Reconcilers race the held claim: each reports claim-busy pending and returns
	// promptly (nonblocking), without the winner ever being released.
	reconcileDone := make(chan RunLaunchReport, reconcilers)
	for i := 0; i < reconcilers; i++ {
		go func() {
			r, _ := mkDriver(&fakeProc{}).ReconcileRunLaunches(capHash("ctx-e1"))
			reconcileDone <- r
		}()
	}
	for i := 0; i < reconcilers; i++ {
		select {
		case report := <-reconcileDone:
			if report.Accounted {
				t.Errorf("a held claim must not be accounted, got %+v", report)
			}
			if !reconcileFindingPresent(report.Findings, "claim-busy:"+id) {
				t.Errorf("findings = %v, want claim-busy:%s", report.Findings, id)
			}
		case <-time.After(5 * time.Second):
			close(proc.releaseLaunch)
			t.Fatal("a reconcile blocked on the held claim; it must probe nonblocking")
		}
	}

	// The losing advancers return promptly — before the winner is released — proving
	// the nonblocking claim bounds contention.
	for i := 0; i < advancers-1; i++ {
		select {
		case err := <-advanceDone:
			if err != nil {
				t.Errorf("a losing advancer must return authoritative state, got %v", err)
			}
		case <-time.After(5 * time.Second):
			close(proc.releaseLaunch)
			t.Fatal("a losing advancer blocked; the nonblocking claim must bound contention")
		}
	}

	// Release the reservation winner's launch; it returns too.
	close(proc.releaseLaunch)
	select {
	case err := <-advanceDone:
		if err != nil {
			t.Errorf("the reservation winner must return without error, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the reservation winner did not return after release")
	}

	launches, resolutions := proc.counts()
	if launches != 1 {
		t.Fatalf("exactly one backend launch under contention, got %d", launches)
	}
	if resolutions != 0 {
		t.Fatalf("the live reservation holder must never be treated as crash recovery, got %d resolutions", resolutions)
	}
	final, err := store.Load(id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if final.RelaunchCount != 1 {
		t.Fatalf("contention must yield EXACTLY ONE relaunch, got RelaunchCount=%d", final.RelaunchCount)
	}
}
