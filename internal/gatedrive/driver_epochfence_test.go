package gatedrive

import (
	"errors"
	"fmt"
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

// ---------------------------------------------------------------------------
// Epoch linkage resolution; fence automatic relaunch and reserved-relaunch
// recovery (change 0437 Task 3). A death earns at most one automatic relaunch,
// and a crash between reserving that relaunch and attaching it earns recovery —
// both now flow through the epoch gate. resolveDriveEpoch answers, from durable
// records only, which run epoch a drive is linked to; authorizeRelaunch reserves
// the automatic replacement while the gate is held; and the recovery entry
// validates the epoch (read-only) BEFORE taking the per-drive claim.
// ---------------------------------------------------------------------------

// startScopedWaitingWithEpoch prepares a scope carrying runEpoch, Starts a
// scope-bound drive under a permissive launch gate, and asserts the first slice
// WAITs. It returns the scoped StartRequest, the WAITING doc, and the launch gate
// the caller can flip to refuse a later relaunch.
func startScopedWaitingWithEpoch(t *testing.T, d *Driver, store *Store, runEpoch string) (StartRequest, DriveDoc) {
	t.Helper()
	req := sampleStart()
	sreq := scopeReqFor(req, "")
	sreq.RunEpochID = runEpoch
	grant, err := store.PrepareScope(sreq)
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	req.ScopeID = grant.ScopeID
	req.ChildCapability = grant.ChildCapability
	req.RunEpochID = runEpoch
	started, err := d.Start(req)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if started.Outcome != WAITING {
		t.Fatalf("scope-bound first slice must WAIT, got %s (%s)", started.Outcome, started.Cause)
	}
	return req, started
}

// TestRelaunchRefusedWhenEpochRevoked proves a death's single automatic relaunch
// is fenced on epoch liveness: a scoped drive whose scope carries a run epoch
// dies while the gate refuses (a cancellation fence landed), so the relaunch leg
// HALTs "run-cancelled", the ORIGINAL launch is the only one (no replacement),
// and no relaunch was reserved.
func TestRelaunchRefusedWhenEpochRevoked(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	dead := false
	proc := &fakeProc{
		observe: func(runDir string) (*process.Observation, error) {
			if dead && strings.HasSuffix(runDir, "run1") {
				return obs(process.StateSignaled, runDir), nil
			}
			return obs(process.StateRunning, runDir), nil
		},
	}
	d, store := newTestDriver(t, clk, proc, stableGit())
	g := &flippableGate{err: errors.New("gatedrive-test: relaunch epoch fence")}
	d.SetEpochLaunchGate(g.gate())

	req, started := startScopedWaitingWithEpoch(t, d, store, "e1")

	// A cancellation fence revokes the epoch; the run dies on the next slice.
	dead = true
	g.setRefuse(true)

	doc, err := d.Advance(started.DriveID, started.Generation)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if doc.Outcome != HALTED || doc.Cause != "run-cancelled" {
		t.Fatalf("relaunch under a revoked epoch = %s/%q, want HALTED/run-cancelled", doc.Outcome, doc.Cause)
	}
	if proc.launchN != 1 {
		t.Fatalf("a revoked epoch must not relaunch: proc.Launch called %d times, want 1", proc.launchN)
	}
	rec, lerr := store.Load(started.DriveID)
	if lerr != nil {
		t.Fatalf("Load: %v", lerr)
	}
	if rec.RelaunchReserved || rec.RelaunchToken != "" || rec.RelaunchCount != 0 {
		t.Fatalf("a refused relaunch must reserve nothing, got reserved=%v token=%q count=%d", rec.RelaunchReserved, rec.RelaunchToken, rec.RelaunchCount)
	}
	_ = req
}

// TestRelaunchAuthorizedUnderGateThenLaunchedOutside proves the durable relaunch
// reservation commits WHILE the epoch gate is held, and the replacement process
// launches OUTSIDE it. A permissive recording gate marks its held window; the
// replacement launch asserts the gate is not held when it runs, and the gate
// wrapper asserts reserveRelaunch's CAS committed (RelaunchReserved set) before it
// released.
func TestRelaunchAuthorizedUnderGateThenLaunchedOutside(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	dead := false
	var (
		mu              sync.Mutex
		gateHeld        bool
		committedInside bool
		driveID         string
	)
	proc := &fakeProc{
		observe: func(runDir string) (*process.Observation, error) {
			if dead && strings.HasSuffix(runDir, "run1") {
				return obs(process.StateSignaled, runDir), nil
			}
			return obs(process.StateRunning, runDir), nil
		},
	}
	proc.launch = func(process.LaunchRequest) (*process.LaunchOutcome, error) {
		id := fmt.Sprintf("run%d", proc.launchN)
		if proc.launchN == 2 { // the replacement launch
			mu.Lock()
			held := gateHeld
			mu.Unlock()
			if held {
				t.Errorf("the replacement launch must run OUTSIDE the epoch gate")
			}
		}
		return &process.LaunchOutcome{RunID: id, RunDir: "/runs/" + id, State: process.StateRunning}, nil
	}
	d, store := newTestDriver(t, clk, proc, stableGit())
	gate := func(_, _ string, reserve func() error) error {
		mu.Lock()
		gateHeld = true
		mu.Unlock()
		rerr := reserve()
		if rerr == nil && driveID != "" {
			if rec, lerr := store.Load(driveID); lerr == nil && rec.RelaunchReserved {
				mu.Lock()
				committedInside = true
				mu.Unlock()
			}
		}
		mu.Lock()
		gateHeld = false
		mu.Unlock()
		return rerr
	}
	d.SetEpochLaunchGate(gate)

	// Permissive Start (RelaunchReserved never set during Start's admission
	// reservations, so committedInside stays false until the relaunch reserve).
	req := sampleStart()
	sreq := scopeReqFor(req, "")
	sreq.RunEpochID = "e1"
	grant, err := store.PrepareScope(sreq)
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	req.ScopeID = grant.ScopeID
	req.ChildCapability = grant.ChildCapability
	req.RunEpochID = "e1"
	started, err := d.Start(req)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if started.Outcome != WAITING {
		t.Fatalf("first slice must WAIT, got %s/%s", started.Outcome, started.Cause)
	}
	driveID = started.DriveID

	dead = true
	doc, err := d.Advance(started.DriveID, started.Generation)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if doc.Outcome != WAITING {
		t.Fatalf("an authorized relaunch's healthy new run must WAIT, got %s/%s", doc.Outcome, doc.Cause)
	}
	if proc.launchN != 2 {
		t.Fatalf("exactly one relaunch (two launches) must occur, got %d", proc.launchN)
	}
	mu.Lock()
	committed := committedInside
	mu.Unlock()
	if !committed {
		t.Fatalf("reserveRelaunch's CAS must commit WHILE the epoch gate is held")
	}
}

// TestRecoveredRelaunchValidatesEpochBeforeClaim proves the reserved-relaunch
// recovery entry validates the epoch (read-only) BEFORE taking the per-drive
// claim — the lock order that forbids acquiring the epoch while holding the claim.
// A revoked epoch settles a proven never-launched replacement HALTED
// "run-cancelled" with no launch, while an identified replacement still attaches
// and is observed normally (reconcile is teardown, not permission).
func TestRecoveredRelaunchValidatesEpochBeforeClaim(t *testing.T) {
	// seedReservedRelaunchDrive persists a scoped drive whose scope carries epoch
	// e1 and whose record has a reserved-but-unattached relaunch (the crash window
	// recoverReservedRelaunch resolves).
	seed := func(t *testing.T, store *Store) (id, ownerGen string) {
		t.Helper()
		req := sampleStart()
		sreq := scopeReqFor(req, "")
		sreq.RunEpochID = "e1"
		grant, err := store.PrepareScope(sreq)
		if err != nil {
			t.Fatalf("PrepareScope: %v", err)
		}
		rec := seedRecord(t)
		rec.ScopeID = grant.ScopeID
		rec.AdmissionToken = "reservation-token"
		rec.RelaunchToken = "bbbbbbbbbbbbbbbb"
		id, ownerGen = seedDrive(t, store, rec)
		if err := store.ownerCAS(id, func(r *driveRecord) error {
			r.RelaunchReserved = true
			return nil
		}); err != nil {
			t.Fatalf("reserve relaunch: %v", err)
		}
		return id, ownerGen
	}

	// revokingProbeGate refuses (the epoch is revoked) AND probes that the drive's
	// per-drive claim is FREE when the gate is entered — proving the epoch is
	// acquired before the claim.
	revokingProbeGate := func(store *Store, id string, sawFreeClaim *bool) EpochLaunchGate {
		return func(_, _ string, _ func() error) error {
			c, busy, cerr := store.tryRelaunchClaim(id)
			if cerr == nil && !busy {
				*sawFreeClaim = true
				c.close()
			}
			return errors.New("gatedrive-test: recovery epoch fence")
		}
	}

	t.Run("never-launched under a revoked epoch halts run-cancelled", func(t *testing.T) {
		store := OpenStore(testsupport.TempDir(t))
		id, ownerGen := seed(t, store)
		proc := &fakeProc{
			resolve: func(root, token string) (*process.ReservationResolution, error) {
				return &process.ReservationResolution{Disposition: "never-launched"}, nil
			},
		}
		sawFreeClaim := false
		clk := &fakeClock{now: startEpoch().Add(time.Second)}
		d := NewDriver(reopenStore(store), clk, proc, stableGit())
		d.slice = 4 * pollTick
		d.pollInterval = pollTick
		d.sleep = func(dur time.Duration) { clk.advance(dur) }
		d.SetEpochLaunchGate(revokingProbeGate(store, id, &sawFreeClaim))

		doc, err := d.Advance(id, ownerGen)
		if err != nil {
			t.Fatalf("Advance: %v", err)
		}
		if doc.Outcome != HALTED || doc.Cause != "run-cancelled" {
			t.Fatalf("a never-launched replacement under a revoked epoch = %s/%q, want HALTED/run-cancelled", doc.Outcome, doc.Cause)
		}
		if proc.launchN != 0 {
			t.Fatalf("a revoked recovery must not launch, proc.Launch called %d times", proc.launchN)
		}
		if !sawFreeClaim {
			t.Fatalf("the epoch gate must be consulted while the per-drive claim is still free (epoch before claim)")
		}
	})

	t.Run("identified replacement still attaches under a revoked epoch", func(t *testing.T) {
		store := OpenStore(testsupport.TempDir(t))
		id, ownerGen := seed(t, store)
		proc := &fakeProc{
			resolve: func(root, token string) (*process.ReservationResolution, error) {
				return &process.ReservationResolution{Disposition: "identified", RunID: "run2", RunDir: "/runs/run2", State: process.StateRunning}, nil
			},
			observe: func(runDir string) (*process.Observation, error) {
				return obs(process.StateRunning, runDir), nil
			},
		}
		sawFreeClaim := false
		clk := &fakeClock{now: startEpoch().Add(time.Second)}
		d := NewDriver(reopenStore(store), clk, proc, stableGit())
		d.slice = 4 * pollTick
		d.pollInterval = pollTick
		d.sleep = func(dur time.Duration) { clk.advance(dur) }
		d.SetEpochLaunchGate(revokingProbeGate(store, id, &sawFreeClaim))

		doc, err := d.Advance(id, ownerGen)
		if err != nil {
			t.Fatalf("Advance: %v", err)
		}
		if doc.Outcome != WAITING {
			t.Fatalf("an identified replacement is reconciled (attached+observed), got %s/%q", doc.Outcome, doc.Cause)
		}
		if proc.launchN != 0 {
			t.Fatalf("an identified replacement attaches without a new launch, proc.Launch called %d times", proc.launchN)
		}
		rec, lerr := store.Load(id)
		if lerr != nil {
			t.Fatalf("Load: %v", lerr)
		}
		if rec.RawRunDir != "/runs/run2" || rec.RelaunchCount != 1 {
			t.Fatalf("the identified replacement must be attached, got RawRunDir=%q count=%d", rec.RawRunDir, rec.RelaunchCount)
		}
	})
}

// TestRelaunchLostLinkageRefuses proves a drive whose epoch linkage is LOST never
// demotes to a standalone relaunch: a scopeless drive with an AdmissionToken whose
// worktree slot now carries a DIFFERENT reservation token can no longer prove
// whether it is epoch-backed, so its death-relaunch leg HALTs "unresolved-execution"
// without launching and without consulting the epoch gate.
func TestRelaunchLostLinkageRefuses(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	wt := sampleWorktree()
	proc := &fakeProc{
		observe: func(runDir string) (*process.Observation, error) {
			return obs(process.StateSignaled, runDir), nil
		},
	}
	// Mint a live worktree slot with its own reservation token.
	slotToken, _, rerr := store.reserveWorktreeExecution(admissionRecord{
		RepoIdentity: "/repo",
		WorktreeRoot: wt,
		Kind:         "scopeless",
	}, proc)
	if rerr != nil {
		t.Fatalf("reserve worktree slot: %v", rerr)
	}

	rec := seedRecord(t)
	rec.WorktreePath = wt
	rec.AdmissionToken = "stale-admission-token" // NOT the slot's current token
	if rec.AdmissionToken == slotToken {
		t.Fatal("the drive's stale token must differ from the slot's live token")
	}
	id, ownerGen := seedDrive(t, store, rec)

	d := NewDriver(reopenStore(store), &fakeClock{now: startEpoch().Add(time.Second)}, proc, stableGit())
	d.slice = pollTick
	d.pollInterval = pollTick
	d.sleep = func(dur time.Duration) {}
	// A gate that fails the test if consulted: lost linkage must refuse BEFORE the gate.
	d.SetEpochLaunchGate(func(_, _ string, _ func() error) error {
		t.Fatalf("lost linkage must refuse before the epoch gate is consulted")
		return nil
	})

	doc, err := d.Advance(id, ownerGen)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if doc.Outcome != HALTED || doc.Cause != "unresolved-execution" {
		t.Fatalf("lost linkage = %s/%q, want HALTED/unresolved-execution", doc.Outcome, doc.Cause)
	}
	if proc.launchN != 0 {
		t.Fatalf("lost linkage must launch nothing, proc.Launch called %d times", proc.launchN)
	}
}

// TestRelaunchStandaloneUnchanged proves a genuinely epoch-less drive relaunches
// exactly as before this change, with the launch gate wired but never consulted:
// a scopeless drive whose worktree slot records an empty RunEpochID, and a legacy
// drive with no admission token and no scope, each earn their single automatic
// relaunch through the epoch-less path.
func TestRelaunchStandaloneUnchanged(t *testing.T) {
	relaunchProc := func() *fakeProc {
		p := &fakeProc{}
		// The seeded drive already owns /runs/run1; the single relaunch must mint a
		// DISTINCT run dir (the first launch through this proc becomes run2), else the
		// replacement would collide with the dead original and relaunch-exhaust.
		p.launch = func(process.LaunchRequest) (*process.LaunchOutcome, error) {
			return &process.LaunchOutcome{RunID: "run2", RunDir: "/runs/run2", State: process.StateRunning}, nil
		}
		p.observe = func(runDir string) (*process.Observation, error) {
			if strings.HasSuffix(runDir, "run1") {
				return obs(process.StateSignaled, runDir), nil
			}
			return obs(process.StateRunning, runDir), nil
		}
		p.stop = func(runDir, reason string) (*process.StopOutcome, error) {
			return &process.StopOutcome{State: process.StateSignaled, RunDir: runDir, Performed: false,
				Terminal: &process.Terminal{Kind: "signal", Signal: 9}}, nil
		}
		return p
	}
	tripGate := func(t *testing.T) EpochLaunchGate {
		return func(_, _ string, _ func() error) error {
			t.Fatalf("an epoch-less drive must NOT consult the launch gate")
			return nil
		}
	}

	t.Run("scopeless slot with empty epoch", func(t *testing.T) {
		store := OpenStore(testsupport.TempDir(t))
		wt := sampleWorktree()
		proc := relaunchProc()
		slotToken, _, rerr := store.reserveWorktreeExecution(admissionRecord{
			RepoIdentity: "/repo",
			WorktreeRoot: wt,
			RunEpochID:   "", // epoch-less slot
			Kind:         "scopeless",
		}, proc)
		if rerr != nil {
			t.Fatalf("reserve worktree slot: %v", rerr)
		}
		rec := seedRecord(t)
		rec.WorktreePath = wt
		rec.AdmissionToken = slotToken // matches: linkage resolves to an empty epoch
		id, ownerGen := seedDrive(t, store, rec)

		clk := &fakeClock{now: startEpoch().Add(time.Second)}
		d := NewDriver(reopenStore(store), clk, proc, stableGit())
		d.slice = 4 * pollTick
		d.pollInterval = pollTick
		d.sleep = func(dur time.Duration) { clk.advance(dur) }
		d.SetEpochLaunchGate(tripGate(t))

		doc, err := d.Advance(id, ownerGen)
		if err != nil {
			t.Fatalf("Advance: %v", err)
		}
		if doc.Outcome != WAITING || doc.Attempt != 2 {
			t.Fatalf("epoch-less relaunch = %s/%q attempt=%d, want WAITING attempt 2", doc.Outcome, doc.Cause, doc.Attempt)
		}
		if proc.launchN != 1 {
			t.Fatalf("epoch-less relaunch must launch the replacement exactly once, got %d", proc.launchN)
		}
	})

	t.Run("legacy drive with no admission token or scope", func(t *testing.T) {
		store := OpenStore(testsupport.TempDir(t))
		proc := relaunchProc()
		rec := seedRecord(t) // AdmissionToken == "", ScopeID == ""
		id, ownerGen := seedDrive(t, store, rec)

		clk := &fakeClock{now: startEpoch().Add(time.Second)}
		d := NewDriver(reopenStore(store), clk, proc, stableGit())
		d.slice = 4 * pollTick
		d.pollInterval = pollTick
		d.sleep = func(dur time.Duration) { clk.advance(dur) }
		d.SetEpochLaunchGate(tripGate(t))

		doc, err := d.Advance(id, ownerGen)
		if err != nil {
			t.Fatalf("Advance: %v", err)
		}
		if doc.Outcome != WAITING || doc.Attempt != 2 {
			t.Fatalf("legacy relaunch = %s/%q attempt=%d, want WAITING attempt 2", doc.Outcome, doc.Cause, doc.Attempt)
		}
		if proc.launchN != 1 {
			t.Fatalf("legacy relaunch must launch the replacement exactly once, got %d", proc.launchN)
		}
	})
}

// ---------------------------------------------------------------------------
// Lock-order and no-deadlock proofs (change 0437 Task 7). The mandated order is
// epoch lock → per-drive launch claim; no path acquires the epoch while holding
// the claim. These prove it deterministically: a probing gate asserts the claim is
// still free at every gate ENTER, and a contention race proves the nonblocking
// claim bounds every contender (all return; exactly one launch) with channel/done
// oracles, never a timing sleep.
// ---------------------------------------------------------------------------

// TestNoEpochAcquisitionWhileClaimHeld proves the epoch gate is entered only while
// the per-drive claim is still free — for both the delayed StartAdmitted launch
// and the reserved-relaunch recovery entry. A probing gate acquires the claim at
// entry: if it is ever already held by this caller when the gate is entered, the
// lock order (epoch before claim) is violated.
func TestNoEpochAcquisitionWhileClaimHeld(t *testing.T) {
	// probeGate is a permissive gate that, at each ENTER, probes the drive's claim.
	// The lock order requires it FREE at every entry; the gate records enters and
	// whether any entry saw it already held.
	probeGate := func(store *Store, id string, enters *int, sawHeld *bool) EpochLaunchGate {
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
		clk := &fakeClock{now: startEpoch()}
		proc := &fakeProc{}
		d, store := newTestDriver(t, clk, proc, stableGit())
		// Admit under a plain permissive gate so the drive id exists to key the probe.
		d.SetEpochLaunchGate((&flippableGate{}).gate())
		req := sampleStart()
		req.RunEpochID = "e1"
		ticket, err := d.Admit(req)
		if err != nil {
			t.Fatalf("Admit: %v", err)
		}
		enters, sawHeld := 0, false
		d.SetEpochLaunchGate(probeGate(store, ticket.id, &enters, &sawHeld))

		doc, serr := d.StartAdmitted(ticket)
		if serr != nil {
			t.Fatalf("StartAdmitted: %v", serr)
		}
		if doc.Outcome != WAITING {
			t.Fatalf("a live launch WAITs, got %s/%s", doc.Outcome, doc.Cause)
		}
		if enters == 0 {
			t.Fatalf("StartAdmitted must consult the epoch gate")
		}
		if sawHeld {
			t.Fatalf("the epoch gate must be entered while the per-drive claim is still FREE (epoch before claim)")
		}
	})

	t.Run("reserved-relaunch recovery validates the epoch before the claim", func(t *testing.T) {
		store := OpenStore(testsupport.TempDir(t))
		req := sampleStart()
		sreq := scopeReqFor(req, "")
		sreq.RunEpochID = "e1"
		grant, err := store.PrepareScope(sreq)
		if err != nil {
			t.Fatalf("PrepareScope: %v", err)
		}
		rec := seedRecord(t)
		rec.ScopeID = grant.ScopeID
		rec.AdmissionToken = "reservation-token"
		rec.RelaunchToken = "bbbbbbbbbbbbbbbb"
		id, ownerGen := seedDrive(t, store, rec)
		if err := store.ownerCAS(id, func(r *driveRecord) error {
			r.RelaunchReserved = true
			return nil
		}); err != nil {
			t.Fatalf("reserve relaunch: %v", err)
		}
		proc := &fakeProc{
			resolve: func(root, token string) (*process.ReservationResolution, error) {
				return &process.ReservationResolution{Disposition: "identified", RunID: "run2", RunDir: "/runs/run2", State: process.StateRunning}, nil
			},
			observe: func(runDir string) (*process.Observation, error) {
				return obs(process.StateRunning, runDir), nil
			},
		}
		enters, sawHeld := 0, false
		clk := &fakeClock{now: startEpoch().Add(time.Second)}
		d := NewDriver(reopenStore(store), clk, proc, stableGit())
		d.slice = 4 * pollTick
		d.pollInterval = pollTick
		d.sleep = func(dur time.Duration) { clk.advance(dur) }
		d.SetEpochLaunchGate(probeGate(store, id, &enters, &sawHeld))

		doc, err := d.Advance(id, ownerGen)
		if err != nil {
			t.Fatalf("Advance: %v", err)
		}
		if doc.Outcome != WAITING {
			t.Fatalf("an identified recovered replacement is attached+observed, got %s/%s", doc.Outcome, doc.Cause)
		}
		if enters == 0 {
			t.Fatalf("the recovery entry must consult the epoch gate")
		}
		if sawHeld {
			t.Fatalf("the recovery epoch pass must run while the per-drive claim is still FREE (epoch before claim)")
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
	wt := mkWorktree(t)
	proc := newClaimWindowProc()
	// A live worktree slot recording epoch e1 backs the scopeless drive's admission
	// token, so resolveDriveEpoch attributes the drive to e1 and reconcile accounts it.
	token, _, terr := store.reserveWorktreeExecution(admissionRecord{
		RepoIdentity: "/repo",
		WorktreeRoot: wt,
		RunEpochID:   "e1",
		Kind:         "scopeless",
	}, proc)
	if terr != nil {
		t.Fatalf("reserve worktree slot: %v", terr)
	}
	rec := seedRecord(t)
	rec.WorktreePath = wt
	rec.AdmissionToken = token
	id, ownerGen := seedDrive(t, store, rec)

	permissive := &flippableGate{}
	mkDriver := func(seam ProcessSeam) *Driver {
		clk := &fakeClock{now: startEpoch().Add(time.Second)}
		d := NewDriver(reopenStore(store), clk, seam, stableGit())
		d.slice = 4 * pollTick
		d.pollInterval = pollTick
		d.sleep = func(dur time.Duration) { clk.advance(dur) }
		d.SetEpochLaunchGate(permissive.gate())
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
	reconcileDone := make(chan EpochLaunchReport, reconcilers)
	for i := 0; i < reconcilers; i++ {
		go func() {
			r, _ := mkDriver(&fakeProc{}).ReconcileEpochLaunches(wt, "e1")
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
