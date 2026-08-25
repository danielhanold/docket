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
	"time"

	"github.com/danielhanold/docket/internal/process"
)

// ProcessSeam is the driver's faithful facade over the raw process supervisor.
// Its method set and types mirror internal/process.Service exactly, so the real
// service is a drop-in seam (Task 7 wires it directly) while tests inject a
// deterministic double. The driver depends on the native run-state vocabulary
// (process.State) unchanged — it never invents or reinterprets a state.
type ProcessSeam interface {
	// Launch starts one detached native-supervisor run and returns its handle,
	// including the run's state at the moment Launch returned.
	Launch(process.LaunchRequest) (*process.LaunchOutcome, error)
	// Observe reads one read-only snapshot of a raw run's durable state without
	// waiting or changing policy.
	Observe(runDir string) (*process.Observation, error)
	// Stop performs an ownership-proven bounded termination, or reports the
	// already-terminal no-op that preserves the child's own verdict.
	Stop(runDir, reason string) (*process.StopOutcome, error)
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
	// RepoDir is where the git reads for the fingerprint run (the working tree),
	// and Worktree is the canonical worktree path recorded on the drive. In a
	// linked-worktree layout they are the same directory.
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

// NewSystemDriver builds a production Driver over the real monotonic clock and
// the real git seam, composing the given store and process seam. The application
// service seam (internal/app) uses it so an in-process caller composes the same
// state machine the CLI drives, without shelling out to docket's own CLI.
func NewSystemDriver(store *Store, proc ProcessSeam) *Driver {
	return NewDriver(store, systemClock{}, proc, realGit{})
}

// Start creates a drive, validates and fingerprints the execution context,
// launches the first raw run through the process seam, persists the drive
// identity, and advances through at most one slice — returning the same typed
// outcome document Advance returns. A malformed request or a launch failure is a
// command failure (an error), not a drive outcome.
func (d *Driver) Start(req StartRequest) (DriveDoc, error) {
	if len(req.Command) == 0 || req.Command[0] == "" {
		return DriveDoc{}, fmt.Errorf("gatedrive: start requires a non-empty command")
	}
	if req.Budget < 0 {
		return DriveDoc{}, fmt.Errorf("gatedrive: start requires a non-negative budget")
	}

	fp, err := ComputeFingerprint(req.Worktree, d.git)
	if err != nil {
		return DriveDoc{}, fmt.Errorf("gatedrive: start fingerprint: %w", err)
	}

	now := d.clock.Now()
	ownerGen, err := randomToken(genNBytes)
	if err != nil {
		return DriveDoc{}, storeErr(ErrIO, "start", err)
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

	// Launch the first raw run, then persist the raw run identity. On a persist
	// failure the freshly launched run would be orphaned, so stop it best-effort
	// before surfacing the command failure — the drive never existed.
	out, err := d.proc.Launch(rec.launchRequest())
	if err != nil {
		return DriveDoc{}, fmt.Errorf("gatedrive: start launch: %w", err)
	}
	rec.RawRunDir = out.RunDir
	rec.RawOwnership = out.RunID

	id, _, err := d.store.NewDrive(rec)
	if err != nil {
		d.stopIfOwned(out.RunDir)
		return DriveDoc{}, err
	}

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
				return d.haltDoc(id, ownerGen, driveRecord{}, "schema-mismatch"), nil
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

	return d.driveAndPersist(id, ownerGen, rec)
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
			return d.haltDoc(id, gen, driveRecord{}, "schema-mismatch"), nil
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
	res := d.driveSlice(rec)

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
		if res.relaunched {
			r.PriorRawRunDir = r.RawRunDir
			r.RawRunDir = res.newRawRunDir
			r.RawOwnership = res.newRawOwnership
			r.Attempt++
			r.RelaunchCount++
		}
		r.LastOutcome = res.outcome
		r.LastCause = res.cause
		return nil
	})
	if err != nil {
		if errors.Is(err, errAlreadyTerminal) {
			// The drive reached a terminal verdict under a concurrent writer;
			// return that authoritative verdict.
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

// errAlreadyTerminal is a sentinel used inside the persist CAS to abort a write
// over a drive a concurrent writer already finished; it never escapes as a
// workflow error.
var errAlreadyTerminal = errors.New("gatedrive: drive already terminal")

// sliceResult is one slice's decision: the outcome/cause to persist and, on the
// single admitted relaunch, the new raw run identity. lastClock is the freshly
// accepted clock value bound to the record.
type sliceResult struct {
	outcome   Outcome
	cause     string
	rawRunDir string // PASSED only

	relaunched      bool
	newRawRunDir    string
	newRawOwnership string

	lastClock time.Time
}

// driveSlice observes the drive's live raw run for at most one slice and maps
// the native observation to a typed outcome. The mapping fails closed: only an
// exact running state is retryable, a pass is accepted only after the
// fingerprint revalidates, a death earns at most one relaunch under the five
// conjoined conditions, and every other uncertainty HALTs (never red). rec is
// read-only; the persisted mutations travel back in the sliceResult.
func (d *Driver) driveSlice(rec driveRecord) sliceResult {
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
			return halt(&res, "observation-unreadable")
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
				return halt(&res, "deadline-expired")
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
				return halt(&res, "identity-mismatch")
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
			out, lerr := d.proc.Launch(rec.launchRequest())
			if lerr != nil {
				return halt(&res, "relaunch-failed")
			}
			// The second raw run belongs to the same drive and deadline. It was
			// never started alongside the first (proven gone above). Continue the
			// same slice observing the new run.
			res.relaunched = true
			res.newRawRunDir = out.RunDir
			res.newRawOwnership = out.RunID
			runDir = out.RunDir
			relaunchUsed = true
			continue

		default:
			// An unrecognized native state fails closed.
			return halt(&res, "unknown-observation")
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
	if !rec.IdempotentSuiteGate {
		return "not-idempotent"
	}
	if expired, _ := rec.deadlineState(now); expired {
		return "deadline-expired"
	}
	cur, err := ComputeFingerprint(rec.WorktreePath, d.git)
	if err != nil {
		return "fingerprint-error"
	}
	if !cur.Equal(rec.Fingerprint) {
		return "identity-mismatch"
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

// recordedDoc builds the outcome document from an authoritative persisted record
// (a terminal re-advance, a concurrent-writer verdict, or a just-persisted
// transition). Only PASSED exposes the raw run dir.
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

// launchRequest builds the deterministic raw launch input the drive replays on
// the single relaunch: the same allocation root, working directory, and argv.
func (rec *driveRecord) launchRequest() process.LaunchRequest {
	return process.LaunchRequest{
		Root: rec.RunRoot,
		Cwd:  rec.Cwd,
		Argv: rec.Command,
	}
}
