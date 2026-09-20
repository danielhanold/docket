// Durable worktree execution slot: one canonical worktree admits at most one
// reserved-or-running top-level gate execution.
//
// A worktree execution slot is the outermost admission authority of the gate
// driver. Before any gate process is launched — a scoped start, a scopeless
// start, an automatic relaunch, or a raw app.GateLaunch inside a registered
// worktree — the launcher reserves the slot for the launch's CANONICAL worktree
// root. A second reservation for the same worktree, across any scope, run root,
// owner, task, or phase, is REFUSED with a typed ErrWorktreeBusy (or
// ErrUnresolvedExecution) and a safe locator — never a get-or-create, never a
// second launch, never stopping the incumbent (spec "at most one
// reserved-or-running top-level Docket gate execution per worktree").
//
// Storage mirrors the drive and scope stores (store.go, scope.go): the record
// lives below the repository's Git common directory, outside every worktree yet
// reachable from any linked worktree of the same repository:
//
//	<git-common-dir>/docket/gate-admission/v1/<admission-key>/record.json
//
// Unlike a drive or scope id — a freshly minted opaque token — an admission key
// is DETERMINISTIC: the sha256 of the canonical (every-symlink-hop-resolved)
// worktree root (admissionKey). Two spellings of one worktree (/tmp and
// /private/tmp on macOS) resolve to one key, so a path alias cannot buy a second
// slot. The directory is owner-only (0700) and its record private (0600); writes
// go through writeAtomicJSON and every read-modify-write serializes on a per-slot
// blocking flock plus a persisted physical generation (admissionCAS), the same
// compare-and-swap discipline the drive store established. Unknown schema
// versions and corrupt records fail closed with a typed StoreError, never a free
// slot: a record the store cannot read is never proof the worktree is idle.
//
// Lock and authority order. When a single logical start needs the worktree, its
// scope, and a drive, acquire them in the order WORKTREE ADMISSION → SCOPE →
// DRIVE; no path acquires the outer admission authority while holding an inner
// scope or drive lock. The slot outlives the CLI process that reserved it: the
// flock is a critical-section primitive here, never the lifetime guarantee — the
// persisted state (reserved/executing/stopping/unresolved/released), not lock
// possession, is the authority on whether the worktree is busy.
//
// State lifecycle. A reserve mints a fresh reservation token and lands the slot
// in "reserved"; the launcher confirms it to "executing" once the process is
// persisted; a proven-terminal run with proven teardown releases it to
// "released"; an ambiguous outcome (a lost launch response, an interrupted
// release) marks it "unresolved" so it fails closed until recovery; explicit
// cancellation marks it "stopping". Every transition after the reserve verifies
// the reservation token, so a stale caller can never release, confirm, or stop
// the successor's slot. A released record is overwritten in place by the next
// reserve; releasing preserves the historical DriveID/RawRunID, so a released
// record remains historical evidence until the next reservation replaces it.
// Completed cancellation may additionally retire a released slot's RunEpochID
// through RetireWorktreeExecutionEpoch (change 0435), leaving the historical
// evidence intact while detaching the cancelled epoch's ownership.
package gatedrive

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// admissionSchemaVersion is the persisted admissionRecord schema generation.
// Like the drive and scope stores, an unknown version fails closed with a typed
// ErrUnknownSchema rather than being best-effort migrated, so this is bumped
// only on a real schema change.
const admissionSchemaVersion = 1

// admissionState is the lifecycle state of one worktree execution slot. Only a
// released or absent slot admits a new reservation; every other state fails a
// reserve closed (reserved/executing/stopping busy, unresolved blocked).
type admissionState string

const (
	// admissionReserved: a reservation is persisted but its process has not yet
	// been launch-confirmed — the recoverable window between reserve and confirm.
	admissionReserved admissionState = "reserved"
	// admissionExecuting: the reserved launch has been confirmed; a live gate
	// process (or its proven-durable record) occupies the worktree.
	admissionExecuting admissionState = "executing"
	// admissionStopping: explicit cancellation (run.cancel, Task 10) has begun
	// tearing the execution down; the slot is not yet proven released.
	admissionStopping admissionState = "stopping"
	// admissionUnresolved: a launch or release outcome could not be established
	// (a lost launch response, an interrupted release). The slot fails closed:
	// it blocks a new admission until recovery resolves it.
	admissionUnresolved admissionState = "unresolved"
	// admissionReleased: a proven-terminal execution with proven teardown vacated
	// the slot. It is admittable again; its facts survive as historical evidence
	// until the next reservation overwrites the record.
	admissionReleased admissionState = "released"
)

// admissionRecord is the durable, owner-private schema of one worktree execution
// slot. Unlike driveRecord/scopeRecord it carries NO json tags: the store
// round-trips it through Go's default field-name marshalling, so the persisted
// keys are the exported field names verbatim. It carries no launch argv,
// environment values, or worktree content — only bounded identity and the
// reservation token, which is the slot's own authority (never a child
// capability).
type admissionRecord struct {
	SchemaVersion int
	RepoIdentity  string // canonical git common dir
	WorktreeRoot  string // canonical worktree root (or "" for a raw non-worktree root — Task 7 never stores those)
	State         admissionState
	ExecutionGen  int // logical execution counter, monotonic per worktree

	ReservationToken string // random token; also handed to process.Launch (Task 2)

	DriveID    string // "" for raw launches
	ScopeID    string // "" when scopeless/raw
	RunEpochID string // "" for standalone gates (Task 9 links workflow gates)

	RawRunID  string // attached at confirm
	RawRunDir string // attached at confirm

	Kind string // "scoped"|"scopeless"|"raw"

	// LegacyInventoried records that the pre-admission drive registry was
	// examined before this first slot reservation. No legacy credential or owner
	// fact is copied into this record.
	LegacyInventoried bool
	LegacyInventoryAt time.Time

	ReservedAt time.Time
	UpdatedAt  time.Time
}

// storedAdmission is the on-disk envelope: the store-owned physical generation
// token beside the admissionRecord it guards, mirroring storedRecord for drives
// and storedScope for scopes. Like admissionRecord it carries no json tags.
type storedAdmission struct {
	Generation string
	Record     admissionRecord
}

// admissionKey returns the deterministic slot key for a worktree root: the
// sha256 of the CANONICAL (already symlink-resolved) absolute root, as lowercase
// hex. The 64-hex result is exactly the shape validateID accepts, so the key
// reuses the store's traversal/symlink directory guards unchanged. It refuses a
// non-absolute input by returning "" — the callers below only ever feed it a
// filepath.EvalSymlinks result, so a non-canonical spelling can never mint a
// slot (an empty key fails validateID downstream).
func admissionKey(worktreeRoot string) string {
	if !filepath.IsAbs(worktreeRoot) {
		return ""
	}
	sum := sha256.Sum256([]byte(worktreeRoot))
	return hex.EncodeToString(sum[:])
}

// admissionKeyFor canonicalises worktreeRoot (resolving every symlink hop) and
// returns the resolved absolute root alongside its admission key. It fails
// closed: a root that is not absolute, or that cannot be symlink-resolved (it
// must exist to be a worktree), is a typed ErrInvalidID rather than a guessed
// key — an unresolvable worktree is never admitted, never silently keyed on an
// unresolved spelling.
func (s *Store) admissionKeyFor(worktreeRoot, op string) (canonical, key string, err error) {
	if !filepath.IsAbs(worktreeRoot) {
		return "", "", storeErr(ErrInvalidID, op, nil)
	}
	canonical, e := filepath.EvalSymlinks(worktreeRoot)
	if e != nil {
		return "", "", storeErr(ErrInvalidID, op, e)
	}
	key = admissionKey(canonical)
	if key == "" {
		return "", "", storeErr(ErrInvalidID, op, nil)
	}
	return canonical, key, nil
}

// ReserveWorktreeExecution admits a new top-level gate execution for a worktree,
// or refuses it. It serializes on the slot's flock, then reads the current
// record: an absent or released slot is admitted; a reserved, executing, or
// stopping slot is refused ErrWorktreeBusy; an unresolved slot is refused
// ErrUnresolvedExecution; an unreadable record (unknown schema, corrupt JSON, IO
// fault) fails closed with its typed StoreError, never a free slot. On success it
// bumps ExecutionGen (monotonic across the worktree's whole history), mints a
// fresh ReservationToken, clears the launch identity, and persists the slot as
// reserved. The returned token is the sole authority for every later transition.
func (s *Store) ReserveWorktreeExecution(rec admissionRecord) (token string, err error) {
	token, _, err = s.reserveWorktreeExecution(rec, nil)
	return token, err
}

// ReserveRawWorktreeExecution reserves the worktree execution slot for a raw
// app.GateLaunch (change 0375 Task 7). It is the sole reserve entry point callable
// from OUTSIDE this package, where the unexported admissionRecord literal is
// unreachable: it composes a Kind "raw" record (no drive id, no scope id, no run
// epoch) and delegates to the same reserveWorktreeExecution the driver uses, so a
// raw launch admits through exactly one authority and one lock/CAS discipline as
// every scoped and scopeless start. proc is the caller's process-recovery seam,
// used only for the first-admission legacy inventory; a nil proc fails a HALTED
// legacy drive closed rather than assessing it. The raw launch's result document
// does not surface the recovery summary (the spec carries it on gate.drive.start
// only), so the summary the inventory returns is deliberately dropped here.
func (s *Store) ReserveRawWorktreeExecution(repoIdentity, worktreeRoot string, proc recoverySeam) (token string, err error) {
	token, _, err = s.reserveWorktreeExecution(admissionRecord{
		RepoIdentity: repoIdentity,
		WorktreeRoot: worktreeRoot,
		Kind:         "raw",
	}, proc)
	return token, err
}

// ReserveWorktreeExecutionForEpoch reserves the worktree execution slot for a
// top-level execution a workflow RUN EPOCH owns (change 0435). It is the exported
// epoch-carrying sibling of ReserveRawWorktreeExecution for callers outside this
// package, where the unexported admissionRecord literal is unreachable — today the
// app layer's cancellation fixtures, which must exercise the owning-epoch
// retirement case (see RetireWorktreeExecutionEpoch) against a slot that genuinely
// records its epoch. It composes a Kind "scopeless" record carrying runEpochID (no
// drive id, no scope id) and delegates to the same reserveWorktreeExecution every
// scoped, scopeless, and raw start admits through — one authority, one lock/CAS
// discipline. An empty runEpochID is refused ErrInvalidID: the raw (epoch-less)
// entry is ReserveRawWorktreeExecution, and the two must not blur.
func (s *Store) ReserveWorktreeExecutionForEpoch(repoIdentity, worktreeRoot, runEpochID string, proc recoverySeam) (token string, err error) {
	if runEpochID == "" {
		return "", storeErr(ErrInvalidID, "reserve-worktree-execution-epoch", nil)
	}
	token, _, err = s.reserveWorktreeExecution(admissionRecord{
		RepoIdentity: repoIdentity,
		WorktreeRoot: worktreeRoot,
		Kind:         "scopeless",
		RunEpochID:   runEpochID,
	}, proc)
	return token, err
}

// reserveWorktreeExecution is ReserveWorktreeExecution's driver-aware form. proc
// is the exact process-recovery seam (the driver's ProcessSeam, or a raw caller's
// *process.Service), supplied for the first-admission legacy inventory; it is
// deliberately not persisted on Store. It returns the legacy history summary the
// inventory produced (nil when no legacy history was relevant) so a caller can
// surface it on a successful start; on an inventory refusal the same summary rides
// the returned OwnershipError.Legacy.
func (s *Store) reserveWorktreeExecution(rec admissionRecord, proc recoverySeam) (token string, legacy *LegacyHistorySummary, err error) {
	const op = "reserve-worktree-execution"
	canonical, key, err := s.admissionKeyFor(rec.WorktreeRoot, op)
	if err != nil {
		return "", nil, err
	}
	dir := filepath.Join(s.admissionRoot, key)
	if err := ensurePrivateDir(dir); err != nil {
		return "", nil, storeErr(ErrIO, op, err)
	}
	lock, err := acquireExclusiveLock(filepath.Join(dir, lockFileName))
	if err != nil {
		return "", nil, err
	}
	defer lock.Close()

	prevGen := 0
	firstAdmission := false
	stored, rerr := s.readStoredAdmission(dir)
	switch {
	case rerr == nil:
		// Run-epoch fence (change 0375 Task 9). A slot a workflow epoch owns admits
		// only that epoch's own sequential drives: an incoming reservation carrying a
		// different (or empty) epoch cannot detach the worktree from its owning epoch.
		// The check precedes the state switch, so it governs an executing incumbent AND
		// a released (between-drives) slot the epoch still owns — the exact detach
		// window. A slot with no recorded epoch (a standalone gate) fences nothing, and
		// a same-epoch reservation falls through to the normal state machine (a released
		// slot readmits; a busy slot returns ErrWorktreeBusy so a same-scope successor
		// can reuse it).
		if stored.Record.RunEpochID != "" && stored.Record.RunEpochID != rec.RunEpochID {
			oe := ownershipErr(ErrStaleRunEpoch, op)
			oe.Incumbent = incumbentSnapshot(stored.Record)
			return "", nil, oe
		}
		switch stored.Record.State {
		case admissionReleased:
			prevGen = stored.Record.ExecutionGen // readmit over a released slot
		case admissionUnresolved:
			oe := ownershipErr(ErrUnresolvedExecution, op)
			oe.Incumbent = incumbentSnapshot(stored.Record)
			return "", nil, oe
		default:
			// reserved, executing, stopping, or any unrecognized non-released
			// state: fail closed as busy — never a free slot.
			oe := ownershipErr(ErrWorktreeBusy, op)
			oe.Incumbent = incumbentSnapshot(stored.Record)
			return "", nil, oe
		}
	case storeErrIs(rerr, ErrNotFound):
		// Inventory is inside this slot's lock so no concurrent reservation can
		// slip a new execution between the legacy census and this reservation. The
		// census is first-admission-only: it runs solely on this ErrNotFound (no
		// slot record yet), so a later readmit over a released slot never re-runs
		// it. On an inventory refusal the summary rides err.Legacy.
		sum, ierr := s.inventoryLegacyDrives(canonical, proc)
		if ierr != nil {
			return "", sum, ierr
		}
		legacy = sum
		prevGen = 0
		firstAdmission = true
	default:
		return "", nil, rerr // unknown schema / corrupt / IO — fail closed
	}

	token, err = randomToken(genNBytes)
	if err != nil {
		return "", nil, storeErr(ErrIO, op, err)
	}
	now := time.Now().UTC()
	rec.SchemaVersion = admissionSchemaVersion
	rec.WorktreeRoot = canonical
	rec.State = admissionReserved
	rec.ExecutionGen = prevGen + 1
	rec.ReservationToken = token
	rec.RawRunID = ""
	rec.RawRunDir = ""
	rec.LegacyInventoried = firstAdmission
	if firstAdmission {
		rec.LegacyInventoryAt = now
	}
	rec.ReservedAt = now
	rec.UpdatedAt = now

	newGen, err := randomToken(genNBytes)
	if err != nil {
		return "", nil, storeErr(ErrIO, op, err)
	}
	if err := writeAtomicJSON(filepath.Join(dir, recordFileName), storedAdmission{Generation: newGen, Record: rec}); err != nil {
		return "", nil, storeErr(ErrIO, op, err)
	}
	return token, legacy, nil
}

// incumbentSnapshot projects the refusing slot's record into the bounded,
// credential-free facts a diagnostic may carry. The reservation token, owner
// generations, and the epoch id itself are deliberately excluded.
func incumbentSnapshot(rec admissionRecord) *IncumbentSnapshot {
	return &IncumbentSnapshot{
		Kind:       rec.Kind,
		State:      string(rec.State),
		DriveID:    rec.DriveID,
		RawRunID:   rec.RawRunID,
		RawRunDir:  rec.RawRunDir,
		EpochOwned: rec.RunEpochID != "",
	}
}

// inventoryLegacyDrives assesses EVERY pre-admission drive record through the
// shared classifier (classifyLegacyDrive), then refuses ONCE if any record is
// unassessable — the first blocker's locator in the returned OwnershipError.Op,
// with all findings gathered in the summary. Records are walked in deterministic
// id order so the first-blocker locator is stable across runs. A completed
// (PASSED/FAILED) drive, a drive bound to a different worktree, and a HALTED drive
// the seam proves torn down are nonblocking; an unreadable record, a nonterminal
// state, or a HALTED drive not provably dead retains and blocks. It fails closed:
// an unreadable record is uncertainty, never a free slot. Every locator is a safe
// recovery token — a validated drive id, or the raw-name-free inventory-level
// "inventory-legacy-drives" — and carries no command, environment, credential, or
// arbitrary directory-name material. It returns a nil summary when no legacy
// history was relevant (nothing was assessable), so a normal start narrates
// nothing.
func (s *Store) inventoryLegacyDrives(worktreeRoot string, proc recoverySeam) (*LegacyHistorySummary, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, ownershipErr(ErrUnresolvedExecution, "inventory-legacy-drives")
	}
	sum := &LegacyHistorySummary{}
	firstLocator := ""
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		id := entry.Name()
		if !entry.IsDir() || validateID(id) != nil {
			// A non-directory or invalid-name entry is not a readable drive. Its
			// arbitrary name never enters a diagnostic: use the safe inventory-level
			// locator and record a finding that carries no drive id.
			sum.Retained = append(sum.Retained, LegacyFinding{DriveID: "", Class: LegacyRetained, Reason: "unrecognized entry in the drive registry"})
			if firstLocator == "" {
				firstLocator = "inventory-legacy-drives"
			}
			continue
		}
		h, lerr := s.loadHistoricalDrive(id)
		if lerr != nil {
			// A drive directory that carries no record file yet is NOT evidence of an
			// occupying execution: writeNewDrive creates the directory before it
			// atomically writes the record, so a concurrent FIRST admission for a
			// DIFFERENT worktree can observe this in-flight (or crashed-mid-creation)
			// directory during its global census. A record-less directory has no
			// worktree binding and has launched no process, so it is skipped rather
			// than failing an unrelated worktree's admission closed. Same-worktree
			// creations serialize on the worktree slot lock, so they never reach a
			// concurrent census here. Every other load fault (a corrupt record, an
			// unknown/legacy-unsupported schema, an IO error) is a genuine unreadable
			// drive and retains.
			if storeErrIs(lerr, ErrNotFound) {
				continue
			}
			sum.Checked++
			reason := "unreadable record"
			if storeErrIs(lerr, ErrUnknownSchema) {
				reason = "unknown schema"
			}
			sum.Retained = append(sum.Retained, LegacyFinding{DriveID: id, Class: LegacyRetained, Reason: reason})
			if firstLocator == "" {
				firstLocator = "inventory-legacy-drive-" + id
			}
			continue
		}
		sum.Checked++
		f := s.classifyLegacyDrive(h, worktreeRoot, proc, true)
		switch f.Class {
		case LegacyRecovered:
			sum.Recovered = append(sum.Recovered, id)
		case LegacyRetained:
			sum.Retained = append(sum.Retained, f)
			if firstLocator == "" {
				firstLocator = "inventory-legacy-drive-" + id
			}
		}
	}
	if firstLocator != "" {
		oe := ownershipErr(ErrUnresolvedExecution, firstLocator)
		oe.Legacy = sum
		return sum, oe
	}
	if sum.Checked == 0 {
		return nil, nil // no relevant legacy history: no summary narration
	}
	return sum, nil
}

// ConfirmWorktreeExecution moves a reserved slot to executing once its launch is
// persisted, attaching the raw run identity. It verifies the reservation token,
// then acts only on the matching current reservation: a reserved slot advances to
// executing; an already-executing slot carrying the same raw run id is an
// idempotent no-op (a replayed confirm after a lost response); any other state is
// a fail-closed ErrUnresolvedLaunchTransition. On any rejection the persisted
// record is untouched.
func (s *Store) ConfirmWorktreeExecution(worktreeRoot, token, rawRunID, rawRunDir string) error {
	const op = "confirm-worktree-execution"
	return s.admissionCAS(worktreeRoot, func(rec *admissionRecord) error {
		if err := verifyAdmissionToken(rec, token, op); err != nil {
			return err
		}
		switch rec.State {
		case admissionReserved:
			rec.State = admissionExecuting
			rec.RawRunID = rawRunID
			rec.RawRunDir = rawRunDir
			rec.UpdatedAt = time.Now().UTC()
			return nil
		case admissionExecuting:
			if rec.RawRunID == rawRunID && rec.RawRunDir == rawRunDir {
				return nil // idempotent replay
			}
			return ownershipErr(ErrUnresolvedLaunchTransition, op)
		default:
			return ownershipErr(ErrUnresolvedLaunchTransition, op)
		}
	})
}

// ReleaseWorktreeExecution vacates the slot to released. It verifies the
// reservation token, then flips the state to released while PRESERVING the
// historical DriveID/RawRunID/RawRunDir so the record remains evidence of what
// last ran (spec "Released records remain historical evidence"). A release of an
// already-released slot under the same token is an idempotent no-op. The caller
// supplies the teardown proof; Task 6 gates which terminal states may release. On
// any rejection the persisted record is untouched.
func (s *Store) ReleaseWorktreeExecution(worktreeRoot, token string) error {
	const op = "release-worktree-execution"
	return s.admissionCAS(worktreeRoot, func(rec *admissionRecord) error {
		if err := verifyAdmissionToken(rec, token, op); err != nil {
			return err
		}
		rec.State = admissionReleased
		rec.UpdatedAt = time.Now().UTC()
		return nil
	})
}

// rotateWorktreeExecutionForSuccessor transitions an EXECUTING slot the same
// scope+epoch still owns to a FRESH reservation for the sequence's next drive: it
// verifies oldToken, requires state executing, bumps ExecutionGen, mints a new
// ReservationToken, clears RawRunID/RawRunDir (the predecessor's raw-run identity
// never rides the successor's reservation), preserves RepoIdentity/WorktreeRoot/
// ScopeID/RunEpochID/Kind, and lands in "reserved". Any other state, a token
// mismatch, or an unreadable record refuses typed and writes nothing. After
// rotation the predecessor's oldToken has NO authority: its late release/unresolve/
// stopping calls fail ErrNotOwner (verifyAdmissionToken), so a stale predecessor
// cleanup can never free or poison the successor's slot.
//
// The fresh token is minted OUTSIDE the CAS body so a physical-generation retry
// never re-mints it; admissionCAS commits exactly once. It performs no epoch write
// (the caller has already validated liveness), leaving every field the mutate does
// not name byte-for-byte intact.
func (s *Store) rotateWorktreeExecutionForSuccessor(worktreeRoot, oldToken string) (newToken string, err error) {
	const op = "rotate-worktree-execution-successor"
	newToken, err = randomToken(genNBytes)
	if err != nil {
		return "", storeErr(ErrIO, op, err)
	}
	if cerr := s.admissionCAS(worktreeRoot, func(rec *admissionRecord) error {
		if verr := verifyAdmissionToken(rec, oldToken, op); verr != nil {
			return verr
		}
		if rec.State != admissionExecuting {
			return ownershipErr(ErrUnresolvedLaunchTransition, op)
		}
		rec.State = admissionReserved
		rec.ExecutionGen++
		rec.ReservationToken = newToken
		rec.RawRunID = ""
		rec.RawRunDir = ""
		rec.UpdatedAt = time.Now().UTC()
		return nil
	}); cerr != nil {
		return "", cerr
	}
	return newToken, nil
}

// MarkWorktreeExecutionUnresolved marks the slot unresolved after an ambiguous
// launch or release outcome. It verifies the reservation token, then flips the
// state to unresolved, where it fails a future reserve closed until recovery
// resolves it. On any rejection the persisted record is untouched.
func (s *Store) MarkWorktreeExecutionUnresolved(worktreeRoot, token string) error {
	const op = "mark-worktree-execution-unresolved"
	return s.admissionCAS(worktreeRoot, func(rec *admissionRecord) error {
		if err := verifyAdmissionToken(rec, token, op); err != nil {
			return err
		}
		rec.State = admissionUnresolved
		rec.UpdatedAt = time.Now().UTC()
		return nil
	})
}

// MarkWorktreeExecutionStopping marks the slot stopping as explicit cancellation
// (run.cancel, Task 10) begins tearing the execution down. It verifies the
// reservation token, then flips the state to stopping; only a later proven
// teardown releases it. On any rejection the persisted record is untouched.
func (s *Store) MarkWorktreeExecutionStopping(worktreeRoot, token string) error {
	const op = "mark-worktree-execution-stopping"
	return s.admissionCAS(worktreeRoot, func(rec *admissionRecord) error {
		if err := verifyAdmissionToken(rec, token, op); err != nil {
			return err
		}
		rec.State = admissionStopping
		rec.UpdatedAt = time.Now().UTC()
		return nil
	})
}

// LoadWorktreeExecution reads and returns the current slot record for a worktree
// and its physical generation. It validates the derived key and refuses a
// symlinked slot directory before touching the record, and fails closed on a
// corrupt document or an unknown schema version. It takes no lock: the atomic
// rename in every write guarantees a reader observes a whole document.
func (s *Store) LoadWorktreeExecution(worktreeRoot string) (admissionRecord, string, error) {
	const op = "load-worktree-execution"
	_, key, err := s.admissionKeyFor(worktreeRoot, op)
	if err != nil {
		return admissionRecord{}, "", err
	}
	dir, err := s.admissionDir(key)
	if err != nil {
		return admissionRecord{}, "", err
	}
	stored, err := s.readStoredAdmission(dir)
	if err != nil {
		return admissionRecord{}, "", err
	}
	return stored.Record, stored.Generation, nil
}

// WorktreeAdmissionRefusal reports the ADVISORY worktree-admission refusal for a
// worktree, or nil when the slot is admissible or its state cannot be read. It is
// the read-only half of the admission authority the application layer consults
// BEFORE charging a full-suite attempt (change 0375 Task 8): a plainly busy slot
// (reserved/executing/stopping) returns ErrWorktreeBusy and an unresolved slot
// returns ErrUnresolvedExecution, so a plainly inadmissible start short-circuits
// with no charge. It is deliberately ADVISORY — the authoritative admission is
// ReserveWorktreeExecution under the slot lock — so anything it cannot determine
// (an absent record, a worktree it cannot yet resolve, a corrupt or unknown-schema
// record) returns nil and defers to that authority, which re-checks and fails
// closed. It never mutates and takes no lock, mirroring LoadWorktreeExecution.
func (s *Store) WorktreeAdmissionRefusal(worktreeRoot string) error {
	const op = "worktree-admission-refusal"
	slot, _, err := s.LoadWorktreeExecution(worktreeRoot)
	if err != nil {
		return nil // absent / unresolvable / unreadable: defer to the authoritative reserve
	}
	switch slot.State {
	case admissionReserved, admissionExecuting, admissionStopping:
		return ownershipErr(ErrWorktreeBusy, op)
	case admissionUnresolved:
		return ownershipErr(ErrUnresolvedExecution, op)
	default:
		return nil
	}
}

// verifyAdmissionToken confirms token is the slot's current reservation token.
// An empty presented token, an unset record token, or any mismatch is
// ErrNotOwner — a stale caller holding a superseded reservation acquires no
// authority to confirm, release, or stop the successor's slot.
func verifyAdmissionToken(rec *admissionRecord, token, op string) error {
	if token == "" || rec.ReservationToken == "" || rec.ReservationToken != token {
		return ownershipErr(ErrNotOwner, op)
	}
	return nil
}

// admissionCAS runs a logical slot transition under the store's physical
// compare-and-swap, mirroring scopeCAS/ownerCAS: it re-reads the current physical
// generation and retries on a physical generation mismatch so concurrent writers
// serialize and physical contention never surfaces as a logical failure. Any
// error mutate itself returns is a deliberate logical rejection (or a real IO
// fault) and propagates immediately with no retry, so a rejected transition
// writes nothing. The slot must already exist (a reserve created it); a
// transition against an absent slot is a typed ErrNotFound.
func (s *Store) admissionCAS(worktreeRoot string, mutate func(*admissionRecord) error) error {
	const op = "admission-cas"
	_, key, err := s.admissionKeyFor(worktreeRoot, op)
	if err != nil {
		return err
	}
	var lastErr error
	for attempt := 0; attempt < ownerCASMaxAttempts; attempt++ {
		dir, err := s.admissionDir(key)
		if err != nil {
			return err
		}
		stored, err := s.readStoredAdmission(dir)
		if err != nil {
			return err
		}
		if _, err := s.admissionCASOnce(key, stored.Generation, mutate); err != nil {
			if se, ok := AsStoreError(err); ok && se.Kind == ErrGenerationMismatch {
				lastErr = err
				continue // physical contention only: re-read and retry
			}
			return err
		}
		return nil
	}
	return storeErr(ErrIO, op, fmt.Errorf("exceeded %d attempts under contention: %w", ownerCASMaxAttempts, lastErr))
}

// admissionCASOnce performs a single flock-serialized compare-and-swap on a slot
// record keyed by its admission key, mirroring scopeCASOnce for scopes. A stale
// physical generation returns ErrGenerationMismatch and writes nothing; a mutate
// error aborts the transition with no write.
func (s *Store) admissionCASOnce(key, expectGen string, mutate func(*admissionRecord) error) (string, error) {
	const op = "admission-cas"
	dir, err := s.admissionDir(key)
	if err != nil {
		return "", err
	}
	lock, err := acquireExclusiveLock(filepath.Join(dir, lockFileName))
	if err != nil {
		return "", err
	}
	defer lock.Close()

	stored, err := s.readStoredAdmission(dir)
	if err != nil {
		return "", err
	}
	if stored.Generation != expectGen {
		return "", storeErr(ErrGenerationMismatch, op, nil)
	}

	rec := stored.Record // a copy; mutate never touches the persisted bytes
	if err := mutate(&rec); err != nil {
		return "", err
	}
	rec.SchemaVersion = admissionSchemaVersion

	newGen, err := randomToken(genNBytes)
	if err != nil {
		return "", storeErr(ErrIO, op, err)
	}
	if err := writeAtomicJSON(filepath.Join(dir, recordFileName), storedAdmission{Generation: newGen, Record: rec}); err != nil {
		return "", storeErr(ErrIO, op, err)
	}
	return newGen, nil
}

// admissionDir validates a slot key, then confirms its directory exists and is a
// real directory (not a symlink) before any caller reads or writes inside it —
// the same traversal/symlink guarantees as driveDir/scopeDir.
func (s *Store) admissionDir(key string) (string, error) {
	if err := validateID(key); err != nil {
		return "", err
	}
	dir := filepath.Join(s.admissionRoot, key)
	fi, err := os.Lstat(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", storeErr(ErrNotFound, "resolve-admission", err)
		}
		return "", storeErr(ErrIO, "resolve-admission", err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return "", storeErr(ErrInvalidID, "resolve-admission", nil)
	}
	return dir, nil
}

// readStoredAdmission decodes the slot envelope in dir and fails closed on a
// corrupt document or an unknown schema version, exactly as the drive and scope
// stores do — a record the store cannot read is never treated as a free slot.
func (s *Store) readStoredAdmission(dir string) (storedAdmission, error) {
	buf, err := os.ReadFile(filepath.Join(dir, recordFileName))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return storedAdmission{}, storeErr(ErrNotFound, "read-admission", err)
		}
		return storedAdmission{}, storeErr(ErrIO, "read-admission", err)
	}
	var stored storedAdmission
	if err := json.Unmarshal(buf, &stored); err != nil {
		return storedAdmission{}, storeErr(ErrCorruptRecord, "read-admission", err)
	}
	if stored.Record.SchemaVersion != admissionSchemaVersion {
		return storedAdmission{}, storeErr(ErrUnknownSchema, "read-admission",
			fmt.Errorf("admission schema version %d, want %d", stored.Record.SchemaVersion, admissionSchemaVersion))
	}
	return stored, nil
}

// storeErrIs reports whether err carries a *StoreError of the given kind. It is
// the production sibling of the test-only isStoreKind helper; production code
// keeps its own so a non-test build resolves it.
func storeErrIs(err error, kind StoreErrorKind) bool {
	se, ok := AsStoreError(err)
	return ok && se.Kind == kind
}
