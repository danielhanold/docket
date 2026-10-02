// The gate-driver state machine: Start and Advance advance one logical suite
// execution over the raw process supervisor in short, slice-bounded synchronous
// calls behind one persisted deadline and execution identity.
//
// The driver is deliberately thin over the composed seams and never
// re-implements process liveness (spec Constraints 1, 9): the injected
// ProcessSeam (a faithful facade over internal/process's Launch/Observe/Stop)
// is the sole authority for process identity and terminal status; the Clock
// governs the fixed-once deadline and per-slice bound; the GitSeam recomputes
// the execution fingerprint at every ownership boundary and before a pass; the
// Store persists an atomic record at each transition and the ownership layer
// arbitrates the single owner.
//
// Every successful Start/Advance returns exactly one typed Outcome document
// (WAITING/PASSED/FAILED/HALTED). The mapping from a raw observation to an
// outcome is the whole point of the file, and it fails closed: only an exact
// native running state is retryable; a death earns at most one relaunch under
// five conjoined conditions; every other uncertainty is HALTED, never coerced
// into a red suite result (spec "State transitions and recovery", "Typed
// outcomes").
package gatedrive

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/danielhanold/docket/internal/process"
)

// ProcessSeam is the driver's faithful facade over the raw process supervisor.
// Its method set and types mirror internal/process.Service exactly, so the real
// service is a drop-in seam (Task 7 wires it directly) while tests inject a
// deterministic double. The driver depends on the native run-state vocabulary
// (process.State) unchanged — it never invents or reinterprets a state.
type ProcessSeam interface {
	// Launch starts one detached gate-supervisor run and returns its handle,
	// including the run's state at the moment Launch returned.
	Launch(process.LaunchRequest) (*process.LaunchOutcome, error)
	// Observe reads one read-only snapshot of a raw run's durable state without
	// waiting or changing policy.
	Observe(runDir string) (*process.Observation, error)
	// Stop performs an ownership-proven bounded termination, or reports the
	// already-terminal no-op that preserves the child's own verdict.
	Stop(runDir, reason string) (*process.StopOutcome, error)
	// ResolveReservation answers what became of a launch handed the caller
	// reservation token but whose response was lost — never-launched, identified,
	// or unresolved (Task 2). The relaunch's launch-error leg and crash-window
	// recovery consult it to attach an identified replacement, and the launch
	// census consults it to resolve a first launch whose run was never attached.
	ResolveReservation(root, token string) (*process.ReservationResolution, error)
}

// productionSlice is the slice target: the maximum a single synchronous driver
// call observes a live run before returning WAITING. It is materially below the
// harness foreground-call ceiling and is plumbing, not a user knob (spec
// "Deadline semantics"). Tests shrink Driver.slice directly.
const productionSlice = 30 * time.Second

// productionPollInterval bounds how often a slice re-observes a live run.
const productionPollInterval = 250 * time.Millisecond

// StartRequest is the validated input to a new drive. It carries the repository
// and worktree identity, the change/task/phase the drive certifies, the
// authoritative resolved command + cwd + budget (never agent input), the launch
// environment/config provenance the record needs, and whether the gate is an
// idempotent suite gate eligible for the single relaunch. The application seam
// (Task 9) resolves these from authoritative config before calling Start.
type StartRequest struct {
	// Worktree is the working tree that is fingerprinted (Start computes the
	// fingerprint over it, and every later boundary re-fingerprints the recorded
	// WorktreePath), and is the canonical worktree path recorded on the drive.
	// RepoDir is the repository identity recorded on the drive (stored as
	// RepoIdentity); it does not drive the fingerprint. In a linked-worktree
	// layout they are the same directory.
	RepoDir  string
	Worktree string

	// Change/task/phase identity — the work the drive certifies.
	ChangeID string
	TaskID   string
	Phase    string

	// Branch/ref recorded alongside the fingerprint's HEAD object id.
	Branch string
	Ref    string

	// Resolved command + working directory for the raw launch (authoritative
	// config, never agent input).
	Command []string
	Cwd     string

	// Config provenance + observation budget (authoritative config).
	ConfigProvenance string
	Budget           time.Duration

	// EnvHash is a canonical hash of the launch environment; the values are
	// never persisted (spec "Persisted execution identity").
	EnvHash string

	// RunRoot is the native process-supervisor allocation root (LaunchRequest.Root).
	RunRoot string

	// IdempotentSuiteGate marks a gate the application contract designates
	// idempotent; ONLY such a gate may earn the single relaunch.
	IdempotentSuiteGate bool

	// RunContext is the RAW outer child-context token linking this drive to the
	// dispatched run whose outer recovery scope run.start prepared; it is stored
	// only as its sha256 hash (RunContextHash), which the run tracker's outer
	// takeover matches. Empty for a drive outside any dispatched run. (change 0359)
	RunContext string

	// Owner names the policy that owns this drive ("build", "finalize", or ""),
	// recorded only in the worktree lock's diagnostic holder note so a busy
	// refusal can name the right remedy. It is never persisted on the drive and
	// decides nothing. (change 0490)
	Owner string
}

// Driver is the gate-drive state machine. It holds no mutable per-drive state:
// the durable record in the Store is the sole source of truth, so a fresh
// Driver over the same store resumes any drive from disk. The slice/pollInterval/
// sleep fields are production plumbing tests override to run deterministically
// without sleeping for production durations.
type Driver struct {
	store *Store
	clock Clock
	proc  ProcessSeam
	git   GitSeam

	slice        time.Duration
	pollInterval time.Duration
	sleep        func(time.Duration)
}

// NewDriver builds a Driver over the composed seams with production slice bounds
// and a real sleep. Tests set the unexported slice/pollInterval/sleep fields to
// inject a deterministic clock/slice.
func NewDriver(store *Store, clock Clock, proc ProcessSeam, git GitSeam) *Driver {
	return &Driver{
		store:        store,
		clock:        clock,
		proc:         proc,
		git:          git,
		slice:        productionSlice,
		pollInterval: productionPollInterval,
		sleep:        time.Sleep,
	}
}

// lockWorktree takes the worktree lock (worktree_lock.go) for the canonical
// worktree containing cwd — the drive's launch working directory, resolved
// through GitSeam.WorktreeRoot so every spelling of one worktree maps to one
// lock. It is only ever TRIED: a held lock is a typed ErrWorktreeBusy naming a
// live holder when the process seam confirms one, and a cwd that resolves to no
// git worktree is a typed ErrWorktreeUnresolved refusal naming that cwd — never
// read as free, and never an internal fault (the cwd is the caller's input).
// Every drive launch site takes it before launching (change 0490).
func (d *Driver) lockWorktree(cwd string) (*WorktreeLock, error) {
	root, err := d.git.WorktreeRoot(cwd)
	if err != nil {
		oe := ownershipErr(ErrWorktreeUnresolved, opWorktreeAdmission)
		oe.Cwd = cwd
		return nil, oe
	}
	return d.store.TryWorktreeLock(root, d.proc)
}

// isWorktreeBusy reports whether err is the worktree lock's typed busy refusal.
func isWorktreeBusy(err error) bool {
	oe, ok := AsOwnershipError(err)
	return ok && oe.Kind == ErrWorktreeBusy
}

// relaunchLockTries bounds how many times the single relaunch tries the
// worktree lock, and relaunchLockPause spaces the tries (through the
// injectable sleep): about two seconds for the relaunch's own just-dead
// supervisor to finish closing its copy (driveSlice). Admission never re-tries:
// a busy worktree there is refused at once.
const (
	relaunchLockTries = 40
	relaunchLockPause = 50 * time.Millisecond
)

// NewSystemDriver builds a production Driver over the real monotonic clock and
// the real git seam, composing the given store and process seam. The application
// service seam (internal/app) uses it so an in-process caller composes the same
// state machine the CLI drives, without shelling out to docket's own CLI.
func NewSystemDriver(store *Store, proc ProcessSeam) *Driver {
	return NewDriver(store, systemClock{}, proc, realGit{})
}

// AdmissionTicket is the opaque result of Admit: the canonical worktree's lock
// held in-process and a RESERVED drive record persisted, but with NO process
// launched yet. StartAdmitted consumes the ticket to launch the raw run (handing
// the lock to its supervisor) and drive one slice; AbandonAdmission releases it
// when the caller decides between admission and launch not to proceed.
//
// The two-phase split exists so a caller can interpose exactly one accounting
// decision BETWEEN admission and launch: the BUILD owner reserves one full-suite
// attempt only after Admit succeeds, so a refused admission (worktree-busy)
// charges nothing, while a launch/persistence failure in StartAdmitted keeps the
// charge — admission precedes charging, and once charged there are no refunds
// (change 0375 Task 8, ADR-0116). Every other caller composes the two through
// the thin Start.
type AdmissionTicket struct {
	id       string
	ownerGen string
	rec      driveRecord
	// lock is the worktree lock Admit took (change 0490). It travels in-process
	// to StartAdmitted, which hands its file to the supervisor; every path that
	// abandons the admission before launching closes it, and a crashed caller's
	// lock is released by the kernel.
	lock *WorktreeLock
	// owner is StartRequest.Owner, carried to the holder note the launch writes.
	owner string
}

// Start creates a drive, validates and fingerprints the execution context,
// launches the first raw run through the process seam, persists the drive
// identity, and advances through at most one slice — returning the same typed
// outcome document Advance returns. A malformed request or a launch failure is a
// command failure (an error), not a drive outcome.
//
// Start is the thin composition of Admit (the pre-launch admission half) and
// StartAdmitted (the launch half) for callers that charge nothing between the two
// — finalize's local gate and the commandless resumption service. The BUILD owner composes the two halves itself so it can
// reserve one full-suite attempt between them (gate_drive.go); its behavior is
// otherwise identical to this composition.
func (d *Driver) Start(req StartRequest) (DriveDoc, error) {
	ticket, err := d.Admit(req)
	if err != nil {
		return DriveDoc{}, err
	}
	return d.StartAdmitted(ticket)
}

// Admit performs the pre-launch half of a start: it validates the request,
// fingerprints the execution context, takes the canonical worktree's lock, and
// persists a RESERVED drive record — but launches NO process. Every refusal here
// (a malformed request, a worktree-busy lock, an I/O failure resolving or taking
// the lock) returns a typed error and leaves nothing behind the caller must later
// account for: no process was launched, no drive was created, and a lock taken
// before a later failure is closed before returning. Admission never waits for,
// queues behind, or stops a holder. On success it returns the ticket
// StartAdmitted (or AbandonAdmission) consumes.
func (d *Driver) Admit(req StartRequest) (*AdmissionTicket, error) {
	if len(req.Command) == 0 || req.Command[0] == "" {
		return nil, fmt.Errorf("gatedrive: start requires a non-empty command")
	}
	if req.Budget < 0 {
		return nil, fmt.Errorf("gatedrive: start requires a non-negative budget")
	}

	fp, err := ComputeFingerprint(req.Worktree, d.git)
	if err != nil {
		return nil, fmt.Errorf("gatedrive: start fingerprint: %w", err)
	}

	now := d.clock.Now()
	ownerGen, err := randomToken(genNBytes)
	if err != nil {
		return nil, storeErr(ErrIO, "start", err)
	}

	rec := driveRecord{
		RepoIdentity:        req.RepoDir,
		WorktreePath:        req.Worktree,
		ChangeID:            req.ChangeID,
		TaskID:              req.TaskID,
		Phase:               req.Phase,
		Branch:              req.Branch,
		Ref:                 req.Ref,
		HeadOID:             fp.Head,
		Fingerprint:         fp,
		Command:             append([]string(nil), req.Command...),
		Cwd:                 req.Cwd,
		ConfigProvenance:    req.ConfigProvenance,
		Budget:              req.Budget,
		EnvHash:             req.EnvHash,
		RunRoot:             req.RunRoot,
		IdempotentSuiteGate: req.IdempotentSuiteGate,
		StartedAt:           now,
		UpdatedAt:           now,
		Deadline:            computeDeadline(now, req.Budget),
		LastClock:           now,
		ProtocolVersion:     ProtocolVersion,
		Attempt:             1,
		OwnerGeneration:     ownerGen,
	}
	// RunContextHash links a drive started inside a dispatched run to that run's
	// outer recovery scope (empty for a drive outside any dispatched run).
	if req.RunContext != "" {
		rec.RunContextHash = capHash(req.RunContext)
	}

	// Admission takes the worktree lock and persists the reserved drive. No run is
	// checked (change 0491): a gate start is refused only by the worktree lock
	// (worktree-busy) and the drive protocol's own checks.
	return d.admitScopeless(rec, ownerGen, req.Owner)
}

// StartAdmitted performs the launch half of a start: it launches the admitted
// drive's raw run (handing the ticket's worktree lock to its supervisor),
// persists the launch handle, records the holder note, and advances through at
// most one slice — returning the same typed outcome document Advance returns. A
// launch or persistence failure is a command failure (an error), not a drive
// outcome, and — because the caller may already have charged a suite attempt
// against this ticket — it never refunds. The worktree frees itself on every
// failure path: process.Launch closes the handed lock on every path, and a
// spawned supervisor's exit releases its copy.
func (d *Driver) StartAdmitted(t *AdmissionTicket) (DriveDoc, error) {
	if t == nil {
		return DriveDoc{}, fmt.Errorf("gatedrive: StartAdmitted requires an admission ticket")
	}
	// Safety net: a ticket whose lock was never handed to Launch is released on
	// every return (a no-op once launchScopeless took the file).
	defer t.lock.Release()
	if t.lock == nil || t.lock.file == nil {
		// A ticket launches at most once, and only while it still holds its lock: a
		// ticket already consumed or abandoned admits nothing.
		return DriveDoc{}, ownershipErr(ErrUnresolvedLaunchTransition, "start-admitted")
	}
	// Revalidate the ticket's RESERVED drive record, and acquire the drive's
	// claimant flock, before any launch (change 0437 Task 2). A busy claim or a
	// settled record refuses with a typed error and launches nothing. On success
	// the returned claim is HELD across launch/attach so a concurrent census
	// observes pending work; the launch half releases it once the launch is
	// attached.
	claim, err := d.revalidateAdmittedLaunch(t)
	if err != nil {
		return DriveDoc{}, err
	}
	return d.launchScopeless(t, claim)
}

// revalidateAdmittedLaunch re-reads the RESERVED drive record this ticket minted
// — it must still exist under the ticket's owner generation and stay nonterminal
// — and acquires the drive's claimant flock NONBLOCKING. A busy claim, a lost
// owner, or a settled record (run.cancel or the keyed run.verdict settled a
// proven never-launched launch, change 0491) refuses with a typed error, closes
// the ticket's worktree lock, and launches nothing.
//
// The claim is taken as the nonblocking per-drive launch claimant (the SAME lock
// file tryRelaunchClaim/reserveRelaunch use), so a concurrent census that probes
// the claim reports busy — pending work, never proof of a crashed caller. The
// returned claim is retained by the caller across the launch.
func (d *Driver) revalidateAdmittedLaunch(t *AdmissionTicket) (*relaunchClaim, error) {
	refuse := func(c *relaunchClaim, err error) (*relaunchClaim, error) {
		if c != nil {
			c.close()
		}
		t.lock.Release()
		return nil, err
	}
	c, busy, cerr := d.store.tryRelaunchClaim(t.id)
	if cerr != nil {
		return refuse(nil, cerr)
	}
	if busy {
		return refuse(nil, ownershipErr(ErrUnresolvedLaunchTransition, "start-admitted"))
	}
	cur, lerr := d.store.Load(t.id)
	if lerr != nil {
		return refuse(c, lerr)
	}
	if verr := verifyOwner(&cur, t.ownerGen); verr != nil {
		return refuse(c, verr)
	}
	if isTerminalOutcome(cur.LastOutcome) {
		return refuse(c, ownershipErr(ErrUnresolvedLaunchTransition, "start-admitted"))
	}
	return c, nil
}

// AbandonAdmission releases an admission the caller decided, between Admit and
// StartAdmitted, not to launch (change 0375 Task 8: a build-owned start whose
// full-suite attempt could not be reserved after admission). Because no process
// was launched, the reserved drive record is a pure orphan (removed so it is not a
// spurious recovery candidate), and closing the ticket's worktree lock frees the
// worktree. It is deliberately best-effort, never a refund path.
func (d *Driver) AbandonAdmission(t *AdmissionTicket) error {
	if t == nil {
		return nil
	}
	_ = d.store.removeReservedDrive(t.id)
	t.lock.Release()
	return nil
}

// admitScopeless runs the pre-launch admission half of a start:
//
//	lockWorktree -> mint the launch token -> NewReservedDrive.
//
// The lock is keyed on the canonical worktree containing the launch working
// directory (rec.Cwd), never on RunRoot (which only scopes process-supervisor
// allocation) or a caller's spelling, so independent callers cannot use distinct
// private run roots or path aliases to launch concurrently against one worktree.
// It launches no process; launchScopeless does. A failure after the lock was
// taken closes it before returning, so a refused admission leaks nothing.
func (d *Driver) admitScopeless(rec driveRecord, ownerGen, owner string) (*AdmissionTicket, error) {
	lock, err := d.lockWorktree(rec.Cwd)
	if err != nil {
		return nil, err
	}
	token, err := randomToken(genNBytes)
	if err != nil {
		lock.Release()
		return nil, storeErr(ErrIO, "start", err)
	}
	// The drive-minted launch token: threaded into the raw launch as the
	// manifest's reservation token, so a lost launch response resolves to this
	// exact run (ResolveReservation).
	rec.AdmissionToken = token

	// Persist a drive with no run handle before Launch. This makes a post-launch
	// persist error recoverable and gives the launch census a record to resolve
	// the launch token from.
	id, _, err := d.store.NewReservedDrive(rec)
	if err != nil {
		lock.Release()
		return nil, err
	}
	return &AdmissionTicket{id: id, ownerGen: ownerGen, rec: rec, lock: lock, owner: owner}, nil
}

// launchScopeless runs the launch half of a start:
//
//	Launch (handing over the worktree lock) -> attachLaunch -> holder note ->
//	driveAndPersist.
//
// process.Launch takes ownership of the worktree lock on every path, so a launch
// that fails frees the worktree in-process, and one that spawned leaves the
// supervisor as the only holder; a post-launch attach failure stops the fresh
// run, whose supervisor's exit frees the worktree. There is never an automatic
// second launch. The claimant flock revalidateAdmittedLaunch acquired is HELD
// across Launch and attach (so a concurrent cancellation observes pending work),
// then released before the drive slice so a first-slice relaunch can reserve its
// own claim; the deferred close is an idempotent safety net for every failure
// leg.
func (d *Driver) launchScopeless(t *AdmissionTicket, claim *relaunchClaim) (DriveDoc, error) {
	defer claim.close()
	rec := t.rec
	id := t.id
	ownerGen := t.ownerGen

	out, lerr := d.proc.Launch(rec.launchRequest(rec.AdmissionToken, t.lock.TakeFile()))
	if lerr != nil {
		// Launch closed the handed lock on every path. Keep the attempted drive as
		// durable evidence (HALTED "launch-failed").
		_ = d.store.ownerCAS(id, func(r *driveRecord) error {
			if err := verifyOwner(r, ownerGen); err != nil {
				return err
			}
			if isTerminalOutcome(r.LastOutcome) {
				return errAlreadyTerminal
			}
			r.LastOutcome = HALTED
			r.LastCause = "launch-failed"
			return nil
		})
		return DriveDoc{}, fmt.Errorf("gatedrive: start launch: %w", lerr)
	}

	if err := d.store.attachLaunch(id, ownerGen, out.RunDir, out.RunID); err != nil {
		d.stopIfOwned(out.RunDir) // the supervisor's exit frees the worktree
		return DriveDoc{}, err
	}
	t.lock.WriteHolder(HolderNote{Kind: "drive", DriveID: id, RunDir: out.RunDir, ChangeID: rec.ChangeID, Owner: t.owner}, d.proc)

	// Launch and attach are confirmed: release the launch claim so the drive slice
	// can reserve its own single relaunch (the claim is the SAME lock file
	// reserveRelaunch takes). close is idempotent with the deferred safety net.
	claim.close()

	rec.RawRunDir = out.RunDir
	rec.RawOwnership = out.RunID
	return d.driveAndPersist(id, ownerGen, rec)
}

// Advance resumes a drive through at most one slice. It loads the durable record
// (the only source of truth), verifies the presented owner generation, and — for
// a still-live drive — runs one slice and persists the transition. A record that
// cannot be read at all is a command failure; a recognized-but-unusable record
// (unknown schema, corrupt) or a stale owner fails closed to HALTED.
func (d *Driver) Advance(id, ownerGen string) (DriveDoc, error) {
	rec, err := d.store.Load(id)
	if err != nil {
		// A missing or malformed id is a command failure — there is no drive to
		// report on. A recognized drive whose schema/content is unusable is a
		// fail-closed HALT (spec "unknown schema ... => HALTED").
		if se, ok := AsStoreError(err); ok {
			switch se.Kind {
			case ErrUnknownSchema, ErrCorruptRecord:
				return d.haltDoc(id, ownerGen, driveRecord{}, CauseSchemaMismatch), nil
			default:
				return DriveDoc{}, err
			}
		}
		return DriveDoc{}, err
	}

	// A stale/wrong owner generation is an identity disagreement: HALT, never a
	// silent continuation, and mutate nothing.
	if err := verifyOwner(&rec, ownerGen); err != nil {
		return d.haltDoc(id, ownerGen, rec, "owner-superseded"), nil
	}

	// A terminal drive is idempotent: return the recorded verdict without
	// re-driving the (already consumed or torn-down) run.
	if isTerminalOutcome(rec.LastOutcome) {
		return d.recordedDoc(id, ownerGen, rec), nil
	}
	var claim *relaunchClaim
	if rec.RelaunchReserved {
		// A crash-window reservation: resolve it before driving. A never-launched
		// replacement returns the held claim, and driveSlice launches it only after
		// taking the worktree lock again (change 0490).
		var resolved *DriveDoc
		rec, claim, resolved, err = d.recoverReservedRelaunch(id, ownerGen, rec)
		if err != nil {
			return DriveDoc{}, err
		}
		if resolved != nil {
			return *resolved, nil
		}
	}

	return d.driveAndPersistClaim(id, ownerGen, rec, claim)
}

// Handoff transfers a live drive to a fresh owner through the single-use handoff
// receipt. It loads the durable record, recomputes the current repository
// fingerprint through the injected git seam, and — under the ownership CAS —
// invalidates the presented owner and writes the receipt only when that owner is
// current and the worktree still matches the drive-start identity (spec
// "Explicit handoff and nearest-owner continuation"). The returned document
// carries, in Generation, the single-use handoff token a claimant presents to
// Claim. A record that cannot be read at all is a command failure; a
// recognized-but-unusable record, a stale owner, an outstanding handoff, or a
// drifted worktree fails closed to HALTED — never a silent transfer.
func (d *Driver) Handoff(id, ownerGen string) (DriveDoc, error) {
	rec, err := d.store.Load(id)
	if err != nil {
		return d.loadHalt(id, ownerGen, err)
	}
	current, ferr := ComputeFingerprint(rec.WorktreePath, d.git)
	if ferr != nil {
		return d.haltDoc(id, ownerGen, rec, "fingerprint-error"), nil
	}
	receipt, herr := d.store.writeHandoffReceipt(id, ownerGen, current)
	if herr != nil {
		if oe, ok := AsOwnershipError(herr); ok {
			return d.haltDoc(id, ownerGen, rec, string(oe.Kind)), nil
		}
		return DriveDoc{}, herr
	}
	cur, err := d.store.Load(id)
	if err != nil {
		return DriveDoc{}, err
	}
	return d.transferDoc(id, receipt.HandoffGeneration, cur), nil
}

// Claim consumes an outstanding single-use handoff receipt for a fresh owner. It
// loads the durable record, recomputes the current repository fingerprint through
// the injected git seam, and — under the ownership CAS — installs a new owner only
// when the presented handoff token is the drive's current outstanding one and the
// recomputed identity still matches the drive-start fingerprint. The returned
// document carries, in Generation, the fresh owner generation the claimant
// advances with. A record that cannot be read at all is a command failure; no
// outstanding handoff, a mismatched token, or a drifted worktree fails closed to
// HALTED, so a claimant that lost the race or no longer matches acquires no
// authority.
func (d *Driver) Claim(id, handoffID string) (DriveDoc, error) {
	rec, err := d.store.Load(id)
	if err != nil {
		return d.loadHalt(id, "", err)
	}
	current, ferr := ComputeFingerprint(rec.WorktreePath, d.git)
	if ferr != nil {
		return d.haltDoc(id, "", rec, "fingerprint-error"), nil
	}
	newOwner, cerr := d.store.consumeHandoffCAS(id, handoffID, current)
	if cerr != nil {
		if oe, ok := AsOwnershipError(cerr); ok {
			return d.haltDoc(id, "", rec, string(oe.Kind)), nil
		}
		return DriveDoc{}, cerr
	}
	cur, err := d.store.Load(id)
	if err != nil {
		return DriveDoc{}, err
	}
	return d.transferDoc(id, newOwner, cur), nil
}

// loadHalt maps a store Load error at an ownership boundary the same way Advance
// does: a recognized-but-unusable record (unknown schema, corrupt) fails closed
// to a HALTED document, while a missing or malformed id is a command failure
// (there is no drive to report on).
func (d *Driver) loadHalt(id, gen string, err error) (DriveDoc, error) {
	if se, ok := AsStoreError(err); ok {
		switch se.Kind {
		case ErrUnknownSchema, ErrCorruptRecord:
			return d.haltDoc(id, gen, driveRecord{}, CauseSchemaMismatch), nil
		}
	}
	return DriveDoc{}, err
}

// transferDoc builds the document a successful ownership transfer returns:
// Generation carries the credential the caller presents next — the single-use
// handoff token after Handoff, or the fresh owner generation after Claim — while
// Outcome/Cause report the drive's last recorded verdict and only a PASSED drive
// exposes its raw run dir.
func (d *Driver) transferDoc(id, generation string, rec driveRecord) DriveDoc {
	doc := DriveDoc{
		ProtocolVersion: ProtocolVersion,
		DriveID:         id,
		Generation:      generation,
		Attempt:         rec.Attempt,
		Deadline:        rec.Deadline,
		Outcome:         rec.LastOutcome,
		Cause:           rec.LastCause,
	}
	if rec.LastOutcome == PASSED {
		doc.RawRunDir = rec.RawRunDir
	}
	return doc
}

// driveAndPersist runs one slice over rec, persists the resulting transition
// atomically under the owner CAS, and returns the outcome document built from
// the authoritative post-transition record.
func (d *Driver) driveAndPersist(id, ownerGen string, rec driveRecord) (DriveDoc, error) {
	return d.driveAndPersistClaim(id, ownerGen, rec, nil)
}

func (d *Driver) driveAndPersistClaim(id, ownerGen string, rec driveRecord, claim *relaunchClaim) (DriveDoc, error) {
	if claim != nil {
		defer claim.close()
	}
	res := d.driveSlice(id, ownerGen, rec, claim)
	if res.err != nil {
		return DriveDoc{}, res.err
	}
	if res.relaunchRaceLost {
		cur, err := d.store.Load(id)
		if err != nil {
			return DriveDoc{}, err
		}
		return d.recordedDoc(id, ownerGen, cur), nil
	}

	err := d.store.ownerCAS(id, func(r *driveRecord) error {
		if err := verifyOwner(r, ownerGen); err != nil {
			return err
		}
		// A concurrent writer may already have driven this drive to a terminal
		// state; never clobber a recorded verdict.
		if isTerminalOutcome(r.LastOutcome) {
			return errAlreadyTerminal
		}
		r.UpdatedAt = res.lastClock
		r.LastClock = res.lastClock
		r.LastOutcome = res.outcome
		r.LastCause = res.cause
		return nil
	})
	if err != nil {
		if errors.Is(err, errAlreadyTerminal) || errors.Is(err, errRelaunchRaceLost) {
			// Relaunch attachment is committed before its observation slice, so a
			// stale outcome writer owns no orphan to stop here. Return the current
			// authoritative verdict.
			cur, lerr := d.store.Load(id)
			if lerr != nil {
				return DriveDoc{}, lerr
			}
			return d.recordedDoc(id, ownerGen, cur), nil
		}
		return DriveDoc{}, err
	}

	cur, err := d.store.Load(id)
	if err != nil {
		return DriveDoc{}, err
	}
	return d.recordedDoc(id, ownerGen, cur), nil
}

// errAlreadyTerminal is a sentinel raised inside an owner CAS (the persist CAS
// and reserveRelaunch's reservation CAS) to abort a write over a drive a
// concurrent same-owner writer already finished; it never escapes as a workflow
// error. Every caller — the persist path, the attach-race branch, and the
// reserveRelaunch branch — treats it as "reload the authoritative recorded
// state", so a loser that reaches its reservation CAS only after the winner
// settled the drive terminally returns that verdict rather than the raw sentinel.
var errAlreadyTerminal = errors.New("gatedrive: drive already terminal")

// errRelaunchRaceLost reports that another same-owner advance has already
// reserved or consumed the single automatic replacement. The loser reloads the
// authoritative drive state and never issues a second backend launch.
var errRelaunchRaceLost = errors.New("gatedrive: relaunch already consumed by a concurrent advance")

// reserveRelaunch acquires the short-lived claimant fence before durably
// consuming the drive's one automatic replacement. The unique process token is
// written in the same CAS. A competitor cannot mistake the interval between
// this transition and Launch for a crashed caller because it cannot acquire the
// claimant flock; a crashed caller releases the flock while leaving the durable
// token available for exact ResolveReservation recovery.
func (s *Store) reserveRelaunch(id, ownerGen string) (*relaunchClaim, error) {
	claim, busy, err := s.tryRelaunchClaim(id)
	if err != nil {
		return nil, err
	}
	if busy {
		return nil, errRelaunchRaceLost
	}
	token, err := randomToken(genNBytes)
	if err != nil {
		claim.close()
		return nil, storeErr(ErrIO, "reserve-relaunch-token", err)
	}
	claim.token = token
	err = s.ownerCAS(id, func(rec *driveRecord) error {
		if err := verifyOwner(rec, ownerGen); err != nil {
			return err
		}
		if isTerminalOutcome(rec.LastOutcome) {
			return errAlreadyTerminal
		}
		if rec.RelaunchCount > 0 || rec.RelaunchReserved || rec.RelaunchToken != "" {
			return errRelaunchRaceLost
		}
		rec.RelaunchReserved = true
		rec.RelaunchToken = token
		return nil
	})
	if err != nil {
		claim.close()
		return nil, err
	}
	return claim, nil
}

// recoverReservedRelaunch resolves a crash window after reserveRelaunch and
// before the replacement was attached. The claimant flock is checked before
// the census: contention proves the original reserving call is still within
// launch/attach, so this caller returns authoritative state without resolving
// or launching. After a crash, the new claimant resolves the replacement's own
// token, never the launch token used by the original run. A proven
// never-launched replacement returns the held claim; driveSlice launches it only
// after taking the worktree lock again, and HALTs worktree-busy instead when
// another gate holds it (change 0490).
func (d *Driver) recoverReservedRelaunch(id, ownerGen string, rec driveRecord) (driveRecord, *relaunchClaim, *DriveDoc, error) {
	claim, busy, err := d.store.tryRelaunchClaim(id)
	if err != nil {
		return driveRecord{}, nil, nil, err
	}
	if busy {
		doc := d.recordedDoc(id, ownerGen, rec)
		return rec, nil, &doc, nil
	}
	cur, err := d.store.Load(id)
	if err != nil {
		claim.close()
		return driveRecord{}, nil, nil, err
	}
	if err := verifyOwner(&cur, ownerGen); err != nil {
		claim.close()
		return driveRecord{}, nil, nil, err
	}
	if isTerminalOutcome(cur.LastOutcome) || !cur.RelaunchReserved {
		claim.close()
		doc := d.recordedDoc(id, ownerGen, cur)
		return cur, nil, &doc, nil
	}
	if cur.RelaunchToken == "" {
		claim.close()
		return d.haltReservedRelaunch(id, ownerGen, cur)
	}
	claim.token = cur.RelaunchToken
	resolution, err := d.proc.ResolveReservation(cur.RunRoot, cur.RelaunchToken)
	if err != nil || resolution == nil || resolution.Disposition == "unresolved" {
		claim.close()
		return d.haltReservedRelaunch(id, ownerGen, cur)
	}
	switch resolution.Disposition {
	case "never-launched":
		return cur, claim, nil, nil
	case "identified":
		if resolution.RunID == "" || resolution.RunDir == "" {
			claim.close()
			return d.haltReservedRelaunch(id, ownerGen, cur)
		}
		err := d.attachReservedRelaunch(id, ownerGen, claim, resolution.RunDir, resolution.RunID)
		claim.close()
		if err != nil {
			if !errors.Is(err, errRelaunchRaceLost) {
				return driveRecord{}, nil, nil, err
			}
			cur, lerr := d.store.Load(id)
			if lerr != nil {
				return driveRecord{}, nil, nil, lerr
			}
			doc := d.recordedDoc(id, ownerGen, cur)
			return cur, nil, &doc, nil
		}
		cur, err := d.store.Load(id)
		if err != nil {
			return driveRecord{}, nil, nil, err
		}
		return cur, nil, nil, nil
	default:
		claim.close()
		return d.haltReservedRelaunch(id, ownerGen, cur)
	}
}

func (d *Driver) haltReservedRelaunch(id, ownerGen string, rec driveRecord) (driveRecord, *relaunchClaim, *DriveDoc, error) {
	return d.haltReservedRelaunchCause(id, ownerGen, rec, "launch-unconfirmed")
}

// haltReservedRelaunchCause settles a reserved-but-unattached relaunch HALTED with
// the given cause, preserving the consumed reservation (the CAS never clears
// RelaunchReserved, so the sole relaunch is never refunded). "launch-unconfirmed"
// is the crash-window uncertainty cause: a drive HALT cause, never an admission
// refusal (change 0490).
func (d *Driver) haltReservedRelaunchCause(id, ownerGen string, rec driveRecord, cause string) (driveRecord, *relaunchClaim, *DriveDoc, error) {
	err := d.store.ownerCAS(id, func(r *driveRecord) error {
		if err := verifyOwner(r, ownerGen); err != nil {
			return err
		}
		if isTerminalOutcome(r.LastOutcome) {
			return errAlreadyTerminal
		}
		if !r.RelaunchReserved {
			return errRelaunchRaceLost
		}
		r.LastOutcome = HALTED
		r.LastCause = cause
		return nil
	})
	if err != nil && !errors.Is(err, errAlreadyTerminal) && !errors.Is(err, errRelaunchRaceLost) {
		return driveRecord{}, nil, nil, err
	}
	cur, lerr := d.store.Load(id)
	if lerr != nil {
		return driveRecord{}, nil, nil, lerr
	}
	doc := d.recordedDoc(id, ownerGen, cur)
	return cur, nil, &doc, nil
}

// attachReservedRelaunch makes the replacement authoritative before any
// observation slice begins. The claimant token prevents a stale launcher from
// attaching over a recovered claimant, and clearing RelaunchReserved lets later
// advances observe the replacement normally after the liveness flock closes.
func (d *Driver) attachReservedRelaunch(id, ownerGen string, claim *relaunchClaim, runDir, runID string) error {
	return d.store.ownerCAS(id, func(r *driveRecord) error {
		if err := verifyOwner(r, ownerGen); err != nil {
			return err
		}
		if r.RelaunchCount > 0 || !r.RelaunchReserved || claim == nil || r.RelaunchToken != claim.token {
			return errRelaunchRaceLost
		}
		r.PriorRawRunDir = r.RawRunDir
		r.RawRunDir = runDir
		r.RawOwnership = runID
		r.Attempt++
		r.RelaunchCount++
		r.RelaunchReserved = false
		return nil
	})
}

// sliceResult is one slice's decision: the outcome/cause to persist and, on the
// single admitted relaunch, the new raw run identity. lastClock is the freshly
// accepted clock value bound to the record.
type sliceResult struct {
	outcome   Outcome
	cause     string
	rawRunDir string // PASSED only

	// relaunchRaceLost reports that a same-owner competitor already owns this
	// drive's single automatic replacement: it either reserved/consumed the
	// relaunch (errRelaunchRaceLost) or settled the drive terminally before this
	// caller's reservation CAS (errAlreadyTerminal). The loser reloads the
	// authoritative record and returns it, issuing no backend launch and mutating
	// no attempt/relaunch state.
	relaunchRaceLost bool
	err              error

	lastClock time.Time
}

// driveSlice observes the drive's live raw run for at most one slice and maps
// the native observation to a typed outcome. The mapping fails closed: only an
// exact running state is retryable, a pass is accepted only after the
// fingerprint revalidates, a death earns at most one relaunch under the five
// conjoined conditions, and every other uncertainty HALTs (never red). rec is
// read-only; the persisted mutations travel back in the sliceResult.
func (d *Driver) driveSlice(id, ownerGen string, rec driveRecord, claim *relaunchClaim) sliceResult {
	sliceStart := d.clock.Now()
	runDir := rec.RawRunDir
	relaunchUsed := rec.RelaunchCount > 0
	res := sliceResult{lastClock: sliceStart}

	for {
		now := d.clock.Now()
		res.lastClock = now

		observation, err := d.proc.Observe(runDir)
		if err != nil {
			// An unreadable observation is fail-closed: HALT, never a guessed state.
			return halt(&res, CauseObservationUnreadable)
		}

		switch observation.State {
		case process.StateRunning:
			// Time governs a live run. Terminal states above never reach here, so
			// only a live run consults the clock and the slice bound.
			expired, backward := rec.deadlineState(now)
			if backward {
				// A backward clock jump could lengthen the budget; distrust it and
				// stop the tree we can still prove we own.
				d.stopIfOwned(runDir)
				return halt(&res, "clock-backward")
			}
			if expired {
				// Deadline expired with a live run: stop and HALT. No relaunch is
				// earned. Stop's own outcome IS the re-observed result (it re-reads
				// the durable terminal record), so no separate observation is taken
				// — a zero budget therefore takes exactly one observation before the
				// stop (deadline == start expires the first observation). A stop
				// that cannot prove ownership says so in the cause.
				if _, serr := d.proc.Stop(runDir, "gatedrive-halt"); serr != nil {
					return halt(&res, "deadline-expired-stop-unproven")
				}
				return halt(&res, CauseDeadlineExpired)
			}
			if d.clock.Since(sliceStart) >= d.slice {
				// The slice ended with the run still live: record the observation
				// and WAIT. No shell monitor, sleep loop, or notification.
				res.outcome = WAITING
				return res
			}
			d.sleep(d.pollInterval)
			continue

		case process.StatePassed:
			// A pass certifies the exact drive-start bytes: revalidate the
			// fingerprint before accepting it; any drift stops-if-owned and HALTs,
			// never red.
			cur, ferr := ComputeFingerprint(rec.WorktreePath, d.git)
			if ferr != nil {
				d.stopIfOwned(runDir)
				return halt(&res, "fingerprint-error")
			}
			if !cur.Equal(rec.Fingerprint) {
				d.stopIfOwned(runDir)
				return halt(&res, "worktree-changed")
			}
			res.outcome = PASSED
			res.rawRunDir = observation.RunDir
			return res

		case process.StateFailed:
			// The suite itself completed red — the only path to FAILED.
			res.outcome = FAILED
			return res

		case process.StateStopped:
			// A stop this drive did not initiate as a continuing transition is a
			// fail-closed HALT, never red.
			return halt(&res, "stopped-not-initiated")

		case process.StateSignaled, process.StateVanished:
			// A death without a verdict. Prove no owned tree survives, then admit
			// the single relaunch only if every condition holds.
			gone, derr := d.proveNoTreeSurvives(runDir, observation)
			if derr != nil || !gone {
				return halt(&res, "uncertain-ownership")
			}
			if relaunchUsed {
				return halt(&res, "relaunch-exhausted")
			}
			if refusal := d.relaunchRefusal(&rec, now); refusal != "" {
				return halt(&res, refusal)
			}
			if claim == nil {
				// Reserve the single automatic relaunch under the per-drive claim. The
				// relaunch checks no run (no gate start does since change 0491),
				// so the worktree lock below is its admission (change 0490).
				var err error
				claim, err = d.store.reserveRelaunch(id, ownerGen)
				if err != nil {
					// A same-owner competitor may have consumed the relaunch
					// (errRelaunchRaceLost) or already settled the drive
					// terminally before this reservation CAS (errAlreadyTerminal).
					// Either way the loser reloads and returns the authoritative
					// recorded state; it never launches, spends no attempt, and
					// surfaces no error — mirroring the attach-race branch below.
					if errors.Is(err, errRelaunchRaceLost) || errors.Is(err, errAlreadyTerminal) {
						res.relaunchRaceLost = true
						return res
					}
					res.err = err
					return res
				}
				rec.RelaunchReserved = true
				rec.RelaunchToken = claim.token
			}
			// The replacement takes the worktree lock again before it launches —
			// this covers both a fresh relaunch and a never-launched crash-window
			// replacement recoverReservedRelaunch handed back. If another gate took
			// the worktree in between, HALT rather than relaunch over it.
			//
			// The first run's terminal record becomes visible a few durable writes
			// before its dying supervisor closes its copy of the lock (live.lock,
			// then the worktree lock, last), so this relaunch can briefly find its
			// OWN prior run still holding it. A bounded re-try lets that exit
			// finish; it never blocks in flock, and a lock still held after the
			// bound belongs to another gate.
			lock, kerr := d.lockWorktree(rec.Cwd)
			for try := 1; try < relaunchLockTries && isWorktreeBusy(kerr); try++ {
				d.sleep(relaunchLockPause)
				lock, kerr = d.lockWorktree(rec.Cwd)
			}
			if kerr != nil {
				claim.close()
				if isWorktreeBusy(kerr) {
					return halt(&res, CauseWorktreeBusy)
				}
				return halt(&res, "relaunch-failed")
			}
			prior, _ := lock.PriorHolder()
			out, lerr := d.proc.Launch(rec.launchRequest(claim.token, lock.TakeFile()))
			if lerr != nil {
				// Launch closed the handed lock on every path; an identified
				// replacement's supervisor holds its own copy.
				resolution, rerr := d.proc.ResolveReservation(rec.RunRoot, claim.token)
				if rerr != nil || resolution == nil || resolution.Disposition == "unresolved" {
					claim.close()
					return halt(&res, "launch-unconfirmed")
				}
				if resolution.Disposition != "identified" || resolution.RunID == "" || resolution.RunDir == "" {
					claim.close()
					return halt(&res, "relaunch-failed")
				}
				out = &process.LaunchOutcome{RunID: resolution.RunID, RunDir: resolution.RunDir, State: resolution.State}
			}
			if err := d.attachReservedRelaunch(id, ownerGen, claim, out.RunDir, out.RunID); err != nil {
				claim.close()
				d.stopIfOwned(out.RunDir) // the replacement's supervisor exit frees the worktree
				if errors.Is(err, errRelaunchRaceLost) || errors.Is(err, errAlreadyTerminal) {
					res.relaunchRaceLost = true
					return res
				}
				res.err = err
				return res
			}
			lock.WriteHolder(HolderNote{Kind: "drive", DriveID: id, RunDir: out.RunDir, ChangeID: rec.ChangeID, Owner: ownerIf(prior, id)}, d.proc)
			claim.close()
			claim = nil
			cur, err := d.store.Load(id)
			if err != nil {
				res.err = err
				return res
			}
			rec = cur
			// The second raw run belongs to the same drive and deadline. It was
			// never started alongside the first (proven gone above). Continue the
			// same slice observing the new run.
			runDir = out.RunDir
			relaunchUsed = true
			continue

		default:
			// An unrecognized native state fails closed.
			return halt(&res, CauseUnknownObservation)
		}
	}
}

// halt stamps a HALTED outcome and cause onto res and returns it.
func halt(res *sliceResult, cause string) sliceResult {
	res.outcome = HALTED
	res.cause = cause
	return *res
}

// proveNoTreeSurvives establishes that no owned process tree survives a death
// before any relaunch is considered (spec "Death and the single relaunch"). A
// vanished observation already proves the supervisor is gone with no terminal to
// consume. A signaled run is already terminal: an already-terminal stop no-op
// confirms it, and a re-observe consumes that terminal state before deciding. A
// stop that cannot prove ownership (an error) leaves the outcome uncertain.
func (d *Driver) proveNoTreeSurvives(runDir string, observation *process.Observation) (bool, error) {
	if observation.State == process.StateVanished {
		return true, nil
	}
	if _, err := d.proc.Stop(runDir, "gatedrive-death-probe"); err != nil {
		return false, err
	}
	if _, err := d.proc.Observe(runDir); err != nil {
		return false, err
	}
	return true, nil
}

// relaunchRefusal returns the typed reason a second launch is refused, or "" when
// all admittable conditions hold. It checks conditions 1 (idempotent gate), 5
// (deadline remains), and 4 (worktree identity still matches, recomputed here).
// Condition 2 (former tree proven gone) is established by proveNoTreeSurvives and
// condition 3 (no prior relaunch) by the caller's relaunchUsed. Command,
// configuration, and environment identity are intrinsic to the immutable record
// and unchanged within a drive; a live environment re-hash belongs to the
// application seam that resolves config/env (Task 9).
func (d *Driver) relaunchRefusal(rec *driveRecord, now time.Time) string {
	if rec.RelaunchCount > 0 {
		return "relaunch-exhausted"
	}
	if !rec.IdempotentSuiteGate {
		return "not-idempotent"
	}
	if expired, _ := rec.deadlineState(now); expired {
		return CauseDeadlineExpired
	}
	cur, err := ComputeFingerprint(rec.WorktreePath, d.git)
	if err != nil {
		return "fingerprint-error"
	}
	if !cur.Equal(rec.Fingerprint) {
		return "worktree-changed"
	}
	return ""
}

// stopIfOwned issues a best-effort stop of a run this drive owns and reports
// whether the stop was performed or the run was already terminal (an ownership-
// proven no-op). It is used at fail-closed boundaries — a drifted pass, a
// backward clock, a deadline expiry — where a live owned tree must not leak. A
// stop it cannot prove ownership for returns false so the caller can say so.
func (d *Driver) stopIfOwned(runDir string) bool {
	out, err := d.proc.Stop(runDir, "gatedrive-halt")
	return err == nil && out != nil
}

// recordedDoc builds the outcome document from an authoritative persisted
// record. Only PASSED exposes the raw run dir. Every terminal document
// (PASSED/FAILED/HALTED) exposes the private run root so the owning caller that
// minted it removes it at the terminal; WAITING retains it — a relaunch may still
// replay under it. Under the worktree lock (change 0490) nothing about a terminal
// drive needs its root kept as release evidence: a HALTED drive whose supervisor
// still runs holds the lock itself, and the launch census proves its teardown
// from the run dirs the drive record names. A HALTED verdict alone does not prove
// its supervisor exited, so the owning caller keeps a HALTED root that still
// holds an unexited run (the finalize seam's haltedRunRootHoldsUnexitedRun)
// rather than deleting a live gate's manifest and logs. haltDoc paths (empty or stale-owner
// records) never reach here, so a superseded owner never deletes a live drive's
// root.
func (d *Driver) recordedDoc(id, ownerGen string, rec driveRecord) DriveDoc {
	doc := DriveDoc{
		ProtocolVersion: ProtocolVersion,
		DriveID:         id,
		Generation:      ownerGen,
		Attempt:         rec.Attempt,
		Deadline:        rec.Deadline,
		Outcome:         rec.LastOutcome,
		Cause:           rec.LastCause,
	}
	if rec.LastOutcome == PASSED {
		doc.RawRunDir = rec.RawRunDir
	}
	if isTerminalOutcome(rec.LastOutcome) {
		doc.RunRoot = rec.RunRoot
	}
	return doc
}

// haltDoc builds a HALTED document for a boundary reached before (or without) a
// persisted transition — a stale owner or an unusable record. It carries the
// identity it can prove and never exposes a raw run dir.
func (d *Driver) haltDoc(id, ownerGen string, rec driveRecord, cause string) DriveDoc {
	return DriveDoc{
		ProtocolVersion: ProtocolVersion,
		DriveID:         id,
		Generation:      ownerGen,
		Attempt:         rec.Attempt,
		Deadline:        rec.Deadline,
		Outcome:         HALTED,
		Cause:           cause,
	}
}

// isTerminalOutcome reports whether an outcome is a settled verdict. WAITING is
// the sole nonterminal outcome; PASSED/FAILED/HALTED are terminal and make a
// re-advance idempotent.
func isTerminalOutcome(o Outcome) bool {
	return o == PASSED || o == FAILED || o == HALTED
}

// launchRequest builds the deterministic raw launch input for both drive launch
// sites — the first launch and the single relaunch replay the same allocation
// root, working directory, and argv. token is the drive-minted launch token
// (AdmissionToken for the first launch, RelaunchToken for the replacement),
// carried as the manifest's reservation token so a lost launch response is
// resolvable to this exact run (ResolveReservation). worktreeLock is the held
// worktree lock handed to the supervisor (change 0490): process.Launch takes
// ownership of it on every path. This body is the only process.LaunchRequest
// literal in the package, and it always sets WorktreeLock.
func (rec *driveRecord) launchRequest(token string, worktreeLock *os.File) process.LaunchRequest {
	return process.LaunchRequest{
		Root:             rec.RunRoot,
		Cwd:              rec.Cwd,
		Argv:             rec.Command,
		ReservationToken: token,
		WorktreeLock:     worktreeLock,
	}
}

// ownerIf keeps the prior holder note's owner across a relaunch: the
// replacement belongs to the same drive, so when the note the drive's first
// launch wrote still names this drive its owner carries over; any other note
// (another gate's, or none) yields "" (unknown). Diagnostic only.
func ownerIf(prior HolderNote, driveID string) string {
	if prior.DriveID == driveID {
		return prior.Owner
	}
	return ""
}
