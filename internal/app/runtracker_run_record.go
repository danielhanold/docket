// The durable run registry (change 0375 Task 9). A run is the
// coordinator-level fence for one workflow implementation run: it is bound to the
// starting gate record (runtracker_store.go) by living beside it under the SAME key
// directory, keyed by the run key, so the run key that already locates a
// dispatch's attribution/retry state also locates its run.
//
// WHERE: <git-common-dir>/docket/run-tracker/<run-key>/run.json — the run record
// sits next to the run-tracker record.json the run start minted. Rooting under the git
// COMMON dir (via runKeyDir, the shared preamble the claim-binding primitives
// use) keeps it outside every worktree yet reachable from any linked worktree, is
// never tracked, and never leaks into a commit.
//
// WHAT IT IS FOR: the run is the durable authority a later human cancellation
// (run.cancel, Task 10) flips active→cancelling→cancelled, and the fence a resume
// (Task 12) supersedes. It records the participants a coordinator/worker/task
// registered and the mutation admissions journaled at the shared mutation
// boundaries (Task 11). This task establishes the record, its participant
// registration, and the state-gated conflict-checked write; the transitions and the
// mutation journal reconciliation are wired by later tasks.
//
// IDENTITY vs. AUTHORITY: RunID is a random, PUBLIC locator — it authorizes
// nothing (the run context's child capability continues to carry authority,
// per ADR-0111) and is what a build-owned start presents to the run launch gate
// (runLaunchGate), so a cancelled or superseded run cannot start a gate. It is safe
// to print.
//
// DURABILITY + CAS: writes go through the same atomic temp-file + rename discipline
// as writeRunTrackerRecordAtomic (0600), and every read-modify-write serializes on a
// per-key run.lock flock plus a persisted physical generation the loader returns,
// so a concurrent participant registration never loses an update and physical
// contention never surfaces as a logical failure. Unknown schema versions and
// corrupt records fail closed with a typed RunError — a record the store cannot
// read is never treated as a live run or a free fence.
package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/danielhanold/docket/internal/gatedrive"
)

// runSchemaVersion is the on-disk RunRecord schema this store understands. A
// record carrying any other version fails closed as a corrupt record — never a
// best-effort migration.
const runSchemaVersion = 1

// runRecordFileName is the atomic record within a run-key directory;
// runLockFileName is the per-key flock the conflict-checked write serializes on.
const (
	runRecordFileName = "run.json"
	runLockFileName   = "run.lock"
)

// runState is the lifecycle state of one run. Only an active run admits
// a new participant registration; the cancelling/cancelled/superseded states are
// the durable fence a later human cancellation or resume drives it into, and the
// completing/completed states are the durable fence a verified successful keyed-
// verdict closeout drives it into (change 0441). Success is never encoded as
// cancellation.
type runState string

const (
	// RunActive: the run owns the run; new participants may register, and the run
	// launch gate admits the run's gate starts.
	RunActive runState = "active"
	// RunCancelling: an explicit cancellation (run.cancel, Task 10) has fenced the
	// run and is tearing the run down; no new participant, start, or mutation admits.
	RunCancelling runState = "cancelling"
	// RunCancelled: cancellation completed with full accounting; the run is
	// terminal and admits nothing.
	RunCancelled runState = "cancelled"
	// RunSuperseded: a resume (Task 12) atomically superseded a confirmed-cancelled
	// run, reserving exactly one replacement dispatch. Terminal for this run.
	RunSuperseded runState = "superseded"
)

// RunCompleting / RunCompleted: the successful-run closeout lifecycle
// (change 0441). Completing is the durable success fence — RunVerdict
// verified run-complete but ownership accounting/retirement is unfinished, so
// the run still owns its worktree and admits no NEW registration, start,
// mutation, takeover, or relaunch. Completed means retirement finished:
// terminal, excluded from ambient worktree-owner lookup, revoked for explicit
// references. Success is never encoded as cancellation.
const (
	RunCompleting runState = "completing"
	RunCompleted  runState = "completed"
)

// RunParticipant is one registered coordinator/worker/task/raw-run boundary the
// run tracks so a cancellation knows what to stop. NativeHandle is the adapter's
// own opaque task handle (a thread/turn id, a drive id) — a locator, never a
// credential. RegisteredAt is an RFC3339 UTC stamp set at registration.
type RunParticipant struct {
	Kind         string `json:"kind"` // coordinator|task|gate-scope|raw-run
	NativeHandle string `json:"native_handle,omitempty"`
	RegisteredAt string `json:"registered_at"`
	// Terminal* persist the adapter's exact terminal observation of this native
	// task — change 0441. Absent evidence means UNPROVEN, never implicitly
	// complete; a terminal failure is termination evidence too (RunVerify
	// independently decides implementation success). TerminalStatus is one of
	// ParticipantTerminalCompleted / ParticipantTerminalFailed; TerminalObservedAt
	// is an RFC3339 UTC stamp set when the evidence is first recorded.
	TerminalStatus     string `json:"terminal_status,omitempty"`
	TerminalTurn       string `json:"terminal_turn,omitempty"`
	TerminalObservedAt string `json:"terminal_observed_at,omitempty"`
}

// ParticipantTerminalCompleted / ParticipantTerminalFailed are the only two
// terminal-observation statuses RecordRunParticipantTerminal will store — a
// terminal failure is termination evidence, not an implementation verdict
// (change 0441). Any other value is malformed evidence and is refused. They are
// exported as the single canonical set for the codexentry/CLI adapter boundary
// (change 0441 Task 9); the adapter passes literal strings matching these values
// and the recorder validates, so no other package need import them.
const (
	ParticipantTerminalCompleted = "completed"
	ParticipantTerminalFailed    = "failed"
)

// AdmittedMutation is one journaled workflow-mutation admission at a shared
// mutation boundary (transaction engine, PR publish, workspace publish). Status is
// admitted|completed|uncertain; a cancellation stays pending until every admitted
// entry is completed or uncertain-reconciled (Task 11 wires the journal writes,
// Task 10 reads them). OpKey is the bounded operation key, never argv/env/content.
// Publication is the optional immutable publication identity captured at admission
// (change 0444) — additive schema-v1 field; nil on legacy and non-publication
// entries. Reconciliation trusts it only when validPublication accepts it.
// Verified records whether the completion OBSERVED the operation's postcondition
// (the boundary resolved applied or no-op) — distinct from Status completed, which
// also covers pushed-nothing outcomes (contended, a local refusal, an internal
// error) where no postcondition was verified. Only a Verified completed entry is
// settling evidence for an uncertain identical publication. Additive schema-v1
// field: absent (legacy) decodes false — never verified, fail-safe.
type AdmittedMutation struct {
	OpKey       string               `json:"op_key"`
	Status      string               `json:"status"` // admitted|completed|uncertain
	Publication *MutationPublication `json:"publication,omitempty"`
	Verified    bool                 `json:"verified,omitempty"`
}

// RunRecord is the durable run state. RunKey binds it to the starting gate
// record it lives beside; ChangeID and Worktree are bound once the claim confirms
// the change instance (bindRunChange) and a scope claims the feature worktree.
// RunID is the random public locator. Participants and AdmittedMutations are the
// accounting a cancellation reconciles.
type RunRecord struct {
	SchemaVersion     int                `json:"schema_version"`
	RunKey            string             `json:"run_key"`
	ChangeID          string             `json:"change_id,omitempty"`
	Worktree          string             `json:"worktree,omitempty"`
	State             runState           `json:"state"`
	RunID             string             `json:"run_id"`
	Participants      []RunParticipant   `json:"participants,omitempty"`
	AdmittedMutations []AdmittedMutation `json:"admitted_mutations,omitempty"`
	// ReplacementReserved carries the resume winner's replacement reservation (the
	// new run key) once a confirmed-cancelled run is superseded (Task 12). Empty
	// for an active run.
	ReplacementReserved string    `json:"replacement_reserved,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// storedRun is the on-disk envelope: the store-owned physical generation token
// beside the RunRecord it guards, mirroring storedScope/storedAdmission for the
// gatedrive stores. The generation rotates on every accepted write so a caller can
// prove it is mutating the state it last read.
type storedRun struct {
	Generation string    `json:"generation"`
	Record     RunRecord `json:"record"`
}

// RunErrorKind is the typed category of a RunError, so a caller can branch on
// a not-found / corrupt / not-active / mismatch condition rather than string prose.
type RunErrorKind string

const (
	// ErrRunNotFound: no run record exists for the run key (never minted, or
	// pruned with its gate record).
	ErrRunNotFound RunErrorKind = "run-not-found"
	// ErrRunRecordCorrupt: the record could not be decoded, or its schema version is not
	// the one this store understands. Fail closed.
	ErrRunRecordCorrupt RunErrorKind = "run-record-corrupt"
	// ErrRunExists: a mint found a run already minted for the run key. Bind-once:
	// a second mint can never overwrite the first.
	ErrRunExists RunErrorKind = "run-exists"
	// ErrRunNotActive: a participant registration (or another active-only transition)
	// was attempted on a run whose state is not active — a fenced or terminal run
	// admits no new participant.
	ErrRunNotActive RunErrorKind = "run-not-active"
	// ErrRunIDMismatch: a caller presented an expected run id that is not this
	// record's RunID — a stale locator. It confers no registration authority.
	ErrRunIDMismatch RunErrorKind = "run-id-mismatch"
	// ErrRunNotCancelled: a resume attempted to supersede a run whose state is
	// not confirmed-cancelled — resume reserves a replacement ONLY after confirmed
	// cancellation, never over an active or still-cancelling run (change 0375 Task 12).
	ErrRunNotCancelled RunErrorKind = "run-not-cancelled"
	// ErrRunAmbiguous: more than one non-superseded run matches one change id, so
	// the run a resume targets cannot be resolved to a single run. Fail closed.
	ErrRunAmbiguous RunErrorKind = "run-ambiguous"
	// ErrRunOwnerAmbiguous: two or more ACTIVE (or completing) runs are bound to
	// one canonical worktree, so ambient owner lookup (findRunByWorktree) cannot name
	// a single current owner. It is a contradiction, never resolved by directory order
	// or timestamp: the path fence refuses locally (change 0446 spec §5).
	ErrRunOwnerAmbiguous RunErrorKind = "run-owner-ambiguous"
	// ErrRunRecordIO: an underlying filesystem, lock, or randomness operation failed.
	ErrRunRecordIO RunErrorKind = "run-record-io"
	// ErrRunParticipantUnknown: a terminal-observation record named a native
	// handle that no registered participant carries (change 0441) — evidence for a
	// participant this run never registered is never stored.
	ErrRunParticipantUnknown RunErrorKind = "run-participant-unknown"
)

// RunError is the run store's typed failure carrying a stable kind and stage.
// It never embeds record content or any credential.
type RunError struct {
	Kind RunErrorKind
	Op   string
	err  error
}

func (e *RunError) Error() string {
	if e.err != nil {
		return fmt.Sprintf("run %s: %s: %v", e.Op, e.Kind, e.err)
	}
	return fmt.Sprintf("run %s: %s", e.Op, e.Kind)
}

func (e *RunError) Unwrap() error { return e.err }

func runErr(kind RunErrorKind, op string, err error) *RunError {
	return &RunError{Kind: kind, Op: op, err: err}
}

// AsRunError unwraps err to a *RunError when one is in the chain so a caller
// can branch on its Kind.
func AsRunError(err error) (*RunError, bool) {
	var e *RunError
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// MintRunRecord mints a fresh run beside the run-tracker record for runKey,
// keyed by the run key. changeID may be "" (a fresh non-resume start binds the
// change later, at claim confirmation, via bindRunChange). The run-key
// directory must already exist (minted by MintRunTrackerRecord). It mints under the
// per-key flock and refuses ErrRunExists if a run was already minted for the
// key — bind-once, a second start can never clobber the first. On success it returns
// the persisted active record (RunID + Generation stamped).
func MintRunRecord(repoDir, runKey, changeID string) (RunRecord, error) {
	dir, err := runKeyDir(repoDir, runKey, "mint-run")
	if err != nil {
		return RunRecord{}, err
	}
	lock, err := acquireRunLock(dir)
	if err != nil {
		return RunRecord{}, err
	}
	defer lock.Close()

	if _, serr := os.Stat(filepath.Join(dir, runRecordFileName)); serr == nil {
		return RunRecord{}, runErr(ErrRunExists, "mint-run", nil)
	} else if !errors.Is(serr, fs.ErrNotExist) {
		return RunRecord{}, runErr(ErrRunRecordIO, "mint-run", serr)
	}

	runID, err := runToken()
	if err != nil {
		return RunRecord{}, runErr(ErrRunRecordIO, "mint-run", err)
	}
	gen, err := runToken()
	if err != nil {
		return RunRecord{}, runErr(ErrRunRecordIO, "mint-run", err)
	}
	now := time.Now().UTC()
	rec := RunRecord{
		SchemaVersion: runSchemaVersion,
		RunKey:        runKey,
		ChangeID:      changeID,
		State:         RunActive,
		RunID:         runID,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := writeRunAtomic(dir, storedRun{Generation: gen, Record: rec}); err != nil {
		return RunRecord{}, err
	}
	return rec, nil
}

// LoadRunRecord reads the run record for runKey and returns it with its
// physical generation. It fails closed on a missing record (ErrRunNotFound), an
// unparseable document or an unknown schema version (ErrRunRecordCorrupt) — a record
// the store cannot read is never a live run.
func LoadRunRecord(repoDir, runKey string) (RunRecord, string, error) {
	dir, err := runKeyDir(repoDir, runKey, "load-run-record")
	if err != nil {
		return RunRecord{}, "", err
	}
	return readStoredRun(dir, "load-run-record")
}

// readStoredRun decodes the run envelope in dir and fails closed on a missing,
// corrupt, or unknown-schema record. It takes no lock: the atomic rename in every
// write guarantees a reader observes a whole document.
func readStoredRun(dir, op string) (RunRecord, string, error) {
	buf, err := os.ReadFile(filepath.Join(dir, runRecordFileName))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return RunRecord{}, "", runErr(ErrRunNotFound, op, err)
		}
		return RunRecord{}, "", runErr(ErrRunRecordIO, op, err)
	}
	var stored storedRun
	if err := json.Unmarshal(buf, &stored); err != nil {
		return RunRecord{}, "", runErr(ErrRunRecordCorrupt, op, err)
	}
	if stored.Record.SchemaVersion != runSchemaVersion {
		return RunRecord{}, "", runErr(ErrRunRecordCorrupt, op,
			fmt.Errorf("run schema version %d, want %d", stored.Record.SchemaVersion, runSchemaVersion))
	}
	return stored.Record, stored.Generation, nil
}

// RegisterRunParticipant appends p to the run's participants under the CAS. It
// verifies expectRunID matches the record's RunID when expectRunID is non-empty
// (a stale locator is ErrRunIDMismatch) and REJECTS when the state is not active
// (ErrRunNotActive) — a fenced or terminal run admits no new participant. It
// stamps RegisteredAt (RFC3339 UTC) when the caller left it empty.
func RegisterRunParticipant(repoDir, runKey, expectRunID string, p RunParticipant) error {
	return runRecordCAS(repoDir, runKey, func(rec *RunRecord) error {
		if expectRunID != "" && rec.RunID != expectRunID {
			return runErr(ErrRunIDMismatch, "register-participant", nil)
		}
		if rec.State != RunActive {
			return runErr(ErrRunNotActive, "register-participant", nil)
		}
		if p.RegisteredAt == "" {
			p.RegisteredAt = time.Now().UTC().Format(time.RFC3339)
		}
		rec.Participants = append(rec.Participants, p)
		return nil
	})
}

// RecordRunParticipantTerminal stamps the adapter's exact terminal observation
// (change 0441) onto the participant whose NativeHandle equals handle. Completing
// an ALREADY-registered participant's record is observation of fact, so — unlike
// RegisterRunParticipant — it is allowed in ANY run state (mirroring the
// mutation journal's completion callback, which has no state gate); registering
// NEW work stays active-only. It fails closed on malformed evidence: an empty
// handle/turn or a status outside {completed, failed} is ErrRunIDMismatch, and a
// handle no participant carries is ErrRunParticipantUnknown. It is idempotent on
// identical evidence; a DIFFERENT already-recorded status or turn is ErrRunIDMismatch
// — recorded terminal evidence is never silently overwritten. A non-empty
// expectRunID mismatching RunID is ErrRunIDMismatch (a stale locator).
func RecordRunParticipantTerminal(repoDir, runKey, expectRunID, handle, turn, status string) error {
	if handle == "" || turn == "" ||
		(status != ParticipantTerminalCompleted && status != ParticipantTerminalFailed) {
		return runErr(ErrRunIDMismatch, "record-participant-terminal", nil)
	}
	err := runRecordCAS(repoDir, runKey, func(rec *RunRecord) error {
		if expectRunID != "" && rec.RunID != expectRunID {
			return runErr(ErrRunIDMismatch, "record-participant-terminal", nil)
		}
		for i := range rec.Participants {
			p := &rec.Participants[i]
			if p.NativeHandle != handle {
				continue
			}
			if p.TerminalStatus != "" {
				if p.TerminalStatus == status && p.TerminalTurn == turn {
					return errRunFenceNoWrite // idempotent replay of identical evidence
				}
				return runErr(ErrRunIDMismatch, "record-participant-terminal", nil)
			}
			p.TerminalStatus = status
			p.TerminalTurn = turn
			p.TerminalObservedAt = time.Now().UTC().Format(time.RFC3339)
			return nil
		}
		return runErr(ErrRunParticipantUnknown, "record-participant-terminal", nil)
	})
	if errors.Is(err, errRunFenceNoWrite) {
		return nil
	}
	return err
}

// bindRunChange binds the run's ChangeID once, at claim confirmation, so the
// run records which change instance it owns (a locator for a later resume/cancel
// lookup, per ADR-0111 proofs — the committed claim receipt remains authority). It
// is a NO-OP when no run exists for the key (a standalone run-tracker record starts none): an
// ErrRunNotFound is swallowed so a claim over a keyless or no-run-record dispatch is
// unaffected. Binding is idempotent: an already-bound identical id is a no-op; a
// bind over a different id fails closed (ErrRunIDMismatch) so a confirmed claim can
// never silently re-point a run's change. Callers treat it best-effort.
func bindRunChange(repoDir, runKey, changeID string) error {
	err := runRecordCAS(repoDir, runKey, func(rec *RunRecord) error {
		if rec.ChangeID != "" {
			if rec.ChangeID == changeID {
				return nil // idempotent
			}
			return runErr(ErrRunIDMismatch, "bind-run-change", nil)
		}
		rec.ChangeID = changeID
		return nil
	})
	if ee, ok := AsRunError(err); ok && ee.Kind == ErrRunNotFound {
		return nil // no run (standalone gate): nothing to bind
	}
	return err
}

// bindRunWorktree binds the run's Worktree once, at claim confirmation, so a
// FRESH (non-resume) run's run record is locatable by the mutation fence
// (findRunByWorktree) and admitted by the run launch gate (runLaunchGate refuses an
// active run with no Worktree) — the same job armResumeReplacement does for the resume path
// (change 0375). Without it a fresh run's run record keeps Worktree == "", which every
// worktree-keyed consumer skips, so the fence and the teardown are inert for the common
// first-dispatch case. The bound value is the LOGICAL feature worktree path (it need
// not exist yet at bind time): the fence canonicalizes the stored value at COMPARE time
// (runOwnsWorktree), once the workspace exists, so a logical spelling resolves to the
// same canonical worktree the mutations run in. It is a NO-OP on an empty worktree
// (nothing to bind) and a NO-OP when no run exists for the key (a standalone gate
// starts none): ErrRunNotFound is swallowed so a claim over a keyless or no-run-record
// dispatch is unaffected. Binding is idempotent: an already-bound identical worktree is
// a no-op; a bind over a different worktree fails closed (ErrRunIDMismatch) so a
// confirmed claim can never silently re-point a run's worktree (mirroring
// bindRunChange). Callers treat it best-effort.
func bindRunWorktree(repoDir, runKey, worktree string) error {
	if worktree == "" {
		return nil // nothing to bind (a keyless/standalone confirm, or an unknown path)
	}
	err := runRecordCAS(repoDir, runKey, func(rec *RunRecord) error {
		if rec.Worktree != "" {
			if rec.Worktree == worktree {
				return nil // idempotent
			}
			return runErr(ErrRunIDMismatch, "bind-run-worktree", nil)
		}
		rec.Worktree = worktree
		return nil
	})
	if ee, ok := AsRunError(err); ok && ee.Kind == ErrRunNotFound {
		return nil // no run (standalone gate): nothing to bind
	}
	return err
}

// runRecordCAS runs a logical run transition under a flock-serialized physical
// conflict-checked write: it acquires the per-key run.lock, reads the current record,
// applies mutate to a copy, and atomically writes it back under a freshly rotated
// generation. Any error mutate returns is a deliberate logical rejection (or a
// real IO fault) and aborts with NO write, so a rejected transition leaves the
// persisted record untouched. The whole read-modify-write is under the lock, so a
// concurrent registration serializes rather than losing an update.
func runRecordCAS(repoDir, runKey string, mutate func(*RunRecord) error) error {
	dir, err := runKeyDir(repoDir, runKey, "run-record-cas")
	if err != nil {
		return err
	}
	lock, err := acquireRunLock(dir)
	if err != nil {
		return err
	}
	defer lock.Close()

	rec, _, err := readStoredRun(dir, "run-record-cas")
	if err != nil {
		return err
	}
	if err := mutate(&rec); err != nil {
		return err
	}
	rec.SchemaVersion = runSchemaVersion
	rec.UpdatedAt = time.Now().UTC()
	gen, err := runToken()
	if err != nil {
		return runErr(ErrRunRecordIO, "run-record-cas", err)
	}
	return writeRunAtomic(dir, storedRun{Generation: gen, Record: rec})
}

// writeRunAtomic marshals stored and writes it at dir/run.json through a
// same-directory temp file followed by os.Rename — the atomic-adjacent replacement
// rule, matching writeRunTrackerRecordAtomic. The temp file is created 0600 so the
// private record's mode survives the rename.
func writeRunAtomic(dir string, stored storedRun) error {
	buf, err := json.Marshal(stored)
	if err != nil {
		return runErr(ErrRunRecordIO, "write-run-record", err)
	}
	tmp, err := os.CreateTemp(dir, "."+runRecordFileName+".tmp-*")
	if err != nil {
		return runErr(ErrRunRecordIO, "write-run-record", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful rename
	if _, err := tmp.Write(buf); err != nil {
		tmp.Close()
		return runErr(ErrRunRecordIO, "write-run-record", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return runErr(ErrRunRecordIO, "write-run-record", err)
	}
	if err := tmp.Close(); err != nil {
		return runErr(ErrRunRecordIO, "write-run-record", err)
	}
	if err := os.Rename(tmpName, filepath.Join(dir, runRecordFileName)); err != nil {
		return runErr(ErrRunRecordIO, "write-run-record", err)
	}
	return nil
}

// acquireRunLock opens (creating if needed) the per-key run.lock and takes an
// exclusive flock, mirroring gatedrive's acquireExclusiveLock: the flock is the
// critical-section primitive for one read-modify-write, never the lifetime
// guarantee (the persisted state is the authority).
func acquireRunLock(dir string) (*os.File, error) {
	f, err := os.OpenFile(filepath.Join(dir, runLockFileName), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, runErr(ErrRunRecordIO, "lock-open", err)
	}
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return nil, runErr(ErrRunRecordIO, "lock-chmod", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, runErr(ErrRunRecordIO, "lock", err)
	}
	return f, nil
}

// errRunAlreadySuperseded is the sentinel SupersedeCancelledRun's CAS returns
// when a concurrent resume already superseded the run — the loser observes the
// winner's reservation rather than reserving a second replacement. It is internal
// to the supersede one-winner race and never surfaces as a typed RunError.
var errRunAlreadySuperseded = errors.New("run already superseded")

// errRunFenceNoWrite aborts a completion-lifecycle CAS with no write when the
// observed state already satisfies the transition (idempotent replay).
var errRunFenceNoWrite = errors.New("run completion state already satisfied")

// FenceRunCompleting moves an active run active→completing through a conflict-checked write, the
// durable success fence a verified keyed run-complete drives (change 0441). It
// returns the state OBSERVED under the lock. active→fenced (RunCompleting, nil);
// already completing→idempotent replay (RunCompleting, nil); completed→
// (RunCompleted, nil) (replay of a finished closeout); any cancelling/cancelled/
// superseded/unknown state is returned as-is plus ErrRunNotActive and is NEVER
// relabelled successful. A non-empty expectRunID mismatching RunID is
// ErrRunIDMismatch — a stale locator confers no completion authority.
func FenceRunCompleting(repoDir, runKey, expectRunID string) (runState, error) {
	var observed runState
	err := runRecordCAS(repoDir, runKey, func(rec *RunRecord) error {
		if expectRunID != "" && rec.RunID != expectRunID {
			return runErr(ErrRunIDMismatch, "fence-completing", nil)
		}
		observed = rec.State
		switch rec.State {
		case RunActive:
			rec.State = RunCompleting
			observed = RunCompleting
			return nil
		case RunCompleting, RunCompleted:
			return errRunFenceNoWrite // idempotent observation, no write
		default:
			return runErr(ErrRunNotActive, "fence-completing", nil)
		}
	})
	if errors.Is(err, errRunFenceNoWrite) {
		return observed, nil
	}
	return observed, err
}

// CompleteRun moves a fenced run completing→completed through a conflict-checked write, retiring a
// successful closeout (change 0441). already completed→idempotent nil (a completed
// receipt replay is safe); ANY other state is ErrRunNotActive — a concurrent
// cancellation that won from completing makes completion lose, and there is no
// shortcut past the fence from active.
func CompleteRun(repoDir, runKey string) error {
	err := runRecordCAS(repoDir, runKey, func(rec *RunRecord) error {
		switch rec.State {
		case RunCompleting:
			rec.State = RunCompleted
			return nil
		case RunCompleted:
			return errRunFenceNoWrite
		default:
			return runErr(ErrRunNotActive, "complete-run", nil)
		}
	})
	if errors.Is(err, errRunFenceNoWrite) {
		return nil
	}
	return err
}

// SupersedeCancelledRun atomically transitions a CONFIRMED-CANCELLED run to
// superseded and records replacementKey as its one reserved replacement dispatch
// (change 0375 Task 12, spec "After confirmed cancellation, atomically supersede
// the old run and reserve one replacement dispatch. Two concurrent resumes
// produce one winner"). The whole read-check-write runs under the run CAS, so two
// concurrent resumes serialize: the WINNER sees the cancelled state, sets superseded
// + ReplacementReserved, and returns nil; the LOSER sees the already-superseded
// state and returns errRunAlreadySuperseded (its caller re-reads and reports the
// winner's reservation). Any non-cancelled state (active/cancelling) is
// ErrRunNotCancelled — resume never supersedes a run that has not confirmed
// cancellation. A superseded run owns no worktree for the mutation fence, so its
// Worktree is cleared as the same atomic transition.
func SupersedeCancelledRun(repoDir, runKey, replacementKey string) error {
	return runRecordCAS(repoDir, runKey, func(rec *RunRecord) error {
		switch rec.State {
		case RunCancelled:
			rec.State = RunSuperseded
			rec.ReplacementReserved = replacementKey
			rec.Worktree = "" // a superseded run no longer owns the worktree fence
			return nil
		case RunSuperseded:
			return errRunAlreadySuperseded
		default:
			return runErr(ErrRunNotCancelled, "supersede-run", nil)
		}
	})
}

// FindRunByChange resolves the run a resume of changeID targets by scanning
// the repository's run-tracker root (each run-key directory may hold one run.json).
// It returns the matching run's run key and record, found=false when no run
// names the change, and a typed error for an enumeration fault or an unresolvable
// ambiguity.
//
// Disambiguation across a resume chain (E1 superseded → E2 …): a matched run that
// is NOT superseded is the current run and wins — exactly one such run must exist
// (more is ErrRunAmbiguous). When every match is superseded (the replacement has
// not yet bound its own change at claim time), the unique superseded match — or, in
// a longer chain, the tail whose ReplacementReserved points outside the matched set —
// is returned, so a repeat resume still recovers the reservation. A missing run-tracker
// root or no match is (found=false, nil); a corrupt/unreadable sibling run is
// skipped, mirroring findRunByWorktree.
func FindRunByChange(repoDir, changeID string) (runKey string, rec RunRecord, found bool, err error) {
	if changeID == "" {
		return "", RunRecord{}, false, nil
	}
	root, rerr := runTrackerRoot(repoDir)
	if rerr != nil {
		return "", RunRecord{}, false, rerr
	}
	entries, derr := os.ReadDir(root)
	if derr != nil {
		if errors.Is(derr, fs.ErrNotExist) {
			return "", RunRecord{}, false, nil // no run-tracker root: no runs
		}
		return "", RunRecord{}, false, runErr(ErrRunRecordIO, "find-by-change", derr)
	}
	type matchEntry struct {
		key string
		rec RunRecord
	}
	var matches []matchEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		key := e.Name()
		r, _, lerr := readStoredRun(filepath.Join(root, key), "find-by-change")
		if lerr != nil {
			continue // no run.json here, or a corrupt/unreadable sibling: cannot match
		}
		if r.ChangeID == changeID {
			matches = append(matches, matchEntry{key: key, rec: r})
		}
	}
	if len(matches) == 0 {
		return "", RunRecord{}, false, nil
	}
	// Prefer a non-superseded match: it is the current run for the change.
	var live []matchEntry
	for _, m := range matches {
		if m.rec.State != RunSuperseded {
			live = append(live, m)
		}
	}
	switch {
	case len(live) == 1:
		return live[0].key, live[0].rec, true, nil
	case len(live) > 1:
		return "", RunRecord{}, false, runErr(ErrRunAmbiguous, "find-by-change", nil)
	}
	// Every match is superseded: resolve to the chain tail whose reserved replacement
	// is not itself one of the matched (superseded) runs.
	if len(matches) == 1 {
		return matches[0].key, matches[0].rec, true, nil
	}
	inSet := make(map[string]bool, len(matches))
	for _, m := range matches {
		inSet[m.key] = true
	}
	var tail []matchEntry
	for _, m := range matches {
		if !inSet[m.rec.ReplacementReserved] {
			tail = append(tail, m)
		}
	}
	if len(tail) == 1 {
		return tail[0].key, tail[0].rec, true, nil
	}
	return "", RunRecord{}, false, runErr(ErrRunAmbiguous, "find-by-change", nil)
}

// runDirMatch is one run-key directory whose run.json records the sought
// RunID: the shape scanRunsByID yields to findRunDirByID (unique match).
type runDirMatch struct {
	dir string
	rec RunRecord
}

// scanRunsByID is the walker under findRunDirByID: it enumerates runTrackerRoot
// and returns every run-key directory whose run.json records RunID == runID. An
// empty id or a missing
// root is (nil, nil); an enumeration fault is a typed ErrRunRecordIO; a corrupt or
// unreadable sibling is SKIPPED for matching (it cannot prove it holds the sought
// id), mirroring findRunByWorktree's conservative skip.
func scanRunsByID(runTrackerRoot, runID string) ([]runDirMatch, error) {
	if runID == "" {
		return nil, nil
	}
	entries, derr := os.ReadDir(runTrackerRoot)
	if derr != nil {
		if errors.Is(derr, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, runErr(ErrRunRecordIO, "find-by-id", derr)
	}
	var matches []runDirMatch
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(runTrackerRoot, e.Name())
		r, _, lerr := readStoredRun(dir, "find-by-id")
		if lerr != nil {
			continue
		}
		if r.RunID == runID {
			matches = append(matches, runDirMatch{dir: dir, rec: r})
		}
	}
	return matches, nil
}

// findRunDirByID resolves the UNIQUE run-key directory holding the run whose
// public RunID is runID (change 0437 Task 5 — the run launch gate locates the
// key directory it must lock and re-read under). Zero matches → ErrRunNotFound;
// more than one → ErrRunAmbiguous; corrupt/unreadable siblings are skipped for
// matching, and an enumeration fault is a typed ErrRunRecordIO (scanRunsByID).
func findRunDirByID(runTrackerRoot, runID string) (dir string, rec RunRecord, err error) {
	matches, serr := scanRunsByID(runTrackerRoot, runID)
	if serr != nil {
		return "", RunRecord{}, serr
	}
	switch len(matches) {
	case 0:
		return "", RunRecord{}, runErr(ErrRunNotFound, "find-dir-by-id", nil)
	case 1:
		return matches[0].dir, matches[0].rec, nil
	default:
		return "", RunRecord{}, runErr(ErrRunAmbiguous, "find-dir-by-id", nil)
	}
}

// runSettledResolver builds the gatedrive.RunSettledFunc the worktree admission
// fence consults when a RELEASED slot still names another run (change 0446
// spec §§2, 5). It reads the app-owned run registry under gitCommonDir and
// reports settled=true only for a run whose readable record is terminal with
// its accounting done: completed (successful closeout retired — a completed run is
// never asked to be cancelled), cancelled (cancellation completed with full
// accounting), or superseded (a resume superseded an already confirmed-cancelled
// run). Active, cancelling, and completing runs are NOT settled: they still own
// the worktree between drives until their own closeout completes. A clean "no such
// run" is (false, gatedrive.ErrRunRecordUnresolved) — a slot-named run with no
// readable record is an unresolved owner, never settlement — and an ambiguous id or
// an enumeration/IO fault is returned as an error; the fence fails closed on all.
func runSettledResolver(gitCommonDir string) func(string) (bool, error) {
	runTrackerRoot := filepath.Join(gitCommonDir, "docket", runTrackerDirName)
	return func(runID string) (bool, error) {
		_, rec, err := findRunDirByID(runTrackerRoot, runID)
		if err != nil {
			if ee, ok := AsRunError(err); ok && ee.Kind == ErrRunNotFound {
				// Unsettled, and typed so the admission refusal's remedy never points
				// at a run.cancel that cannot resolve this run.
				return false, gatedrive.ErrRunRecordUnresolved
			}
			return false, err
		}
		switch rec.State {
		case RunCompleted, RunCancelled, RunSuperseded:
			return true, nil
		default:
			return false, nil
		}
	}
}

// runToken mints a random 32-hex-char token (16 crypto-random bytes) for the
// public RunID locator and for the physical generation. Both are opaque lookup
// tokens, never encoded state.
func runToken() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
