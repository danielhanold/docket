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
	const op = "reserve-worktree-execution"
	canonical, key, err := s.admissionKeyFor(rec.WorktreeRoot, op)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(s.admissionRoot, key)
	if err := ensurePrivateDir(dir); err != nil {
		return "", storeErr(ErrIO, op, err)
	}
	lock, err := acquireExclusiveLock(filepath.Join(dir, lockFileName))
	if err != nil {
		return "", err
	}
	defer lock.Close()

	prevGen := 0
	stored, rerr := s.readStoredAdmission(dir)
	switch {
	case rerr == nil:
		switch stored.Record.State {
		case admissionReleased:
			prevGen = stored.Record.ExecutionGen // readmit over a released slot
		case admissionUnresolved:
			return "", ownershipErr(ErrUnresolvedExecution, op)
		default:
			// reserved, executing, stopping, or any unrecognized non-released
			// state: fail closed as busy — never a free slot.
			return "", ownershipErr(ErrWorktreeBusy, op)
		}
	case storeErrIs(rerr, ErrNotFound):
		prevGen = 0 // a fresh slot: absence proven under the lock
	default:
		return "", rerr // unknown schema / corrupt / IO — fail closed
	}

	token, err = randomToken(genNBytes)
	if err != nil {
		return "", storeErr(ErrIO, op, err)
	}
	now := time.Now().UTC()
	rec.SchemaVersion = admissionSchemaVersion
	rec.WorktreeRoot = canonical
	rec.State = admissionReserved
	rec.ExecutionGen = prevGen + 1
	rec.ReservationToken = token
	rec.RawRunID = ""
	rec.RawRunDir = ""
	rec.ReservedAt = now
	rec.UpdatedAt = now

	newGen, err := randomToken(genNBytes)
	if err != nil {
		return "", storeErr(ErrIO, op, err)
	}
	if err := writeAtomicJSON(filepath.Join(dir, recordFileName), storedAdmission{Generation: newGen, Record: rec}); err != nil {
		return "", storeErr(ErrIO, op, err)
	}
	return token, nil
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
