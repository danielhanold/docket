// The durable run-epoch registry (change 0375 Task 9). A run epoch is the
// coordinator-level fence for one workflow implementation run: it is bound to the
// arming gate record (rungate_store.go) by living beside it under the SAME key
// directory, keyed by the gate key, so the gate key that already locates a
// dispatch's attribution/retry state also locates its epoch.
//
// WHERE: <git-common-dir>/docket/rungate/<gate-key>/epoch.json — the epoch record
// sits next to the gate record.json the arming gate minted. Rooting under the git
// COMMON dir (via gateKeyDir, the shared preamble the claim-binding primitives
// use) keeps it outside every worktree yet reachable from any linked worktree, is
// never tracked, and never leaks into a commit.
//
// WHAT IT IS FOR: the epoch is the durable authority a later human cancellation
// (run.cancel, Task 10) flips active→cancelling→cancelled, and the fence a resume
// (Task 12) supersedes. It records the participants a coordinator/worker/task
// registered and the mutation admissions journaled at the shared mutation
// boundaries (Task 11). This task establishes the record, its participant
// registration, and the state-gated compare-and-swap; the transitions and the
// mutation journal reconciliation are wired by later tasks.
//
// IDENTITY vs. AUTHORITY: EpochID is a random, PUBLIC locator — it authorizes
// nothing (the dispatch context's child capability continues to carry authority,
// per ADR-0111) and travels onto a scoped start's worktree execution slot so an
// omitted or stale epoch cannot detach a workflow-owned worktree (the gatedrive
// stale-run-epoch fence). It is safe to print.
//
// DURABILITY + CAS: writes go through the same atomic temp-file + rename discipline
// as writeGateRecordAtomic (0600), and every read-modify-write serializes on a
// per-key epoch.lock flock plus a persisted physical generation the loader returns,
// so a concurrent participant registration never loses an update and physical
// contention never surfaces as a logical failure. Unknown schema versions and
// corrupt records fail closed with a typed EpochError — a record the store cannot
// read is never treated as a live epoch or a free fence.
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

// epochSchemaVersion is the on-disk EpochRecord schema this store understands. A
// record carrying any other version fails closed as a corrupt record — never a
// best-effort migration.
const epochSchemaVersion = 1

// epochRecordFileName is the atomic record within a gate-key directory;
// epochLockFileName is the per-key flock the compare-and-swap serializes on.
const (
	epochRecordFileName = "epoch.json"
	epochLockFileName   = "epoch.lock"
)

// epochState is the lifecycle state of one run epoch. Only an active epoch admits
// a new participant registration; the cancelling/cancelled/superseded states are
// the durable fence a later human cancellation or resume drives it into, and the
// completing/completed states are the durable fence a verified successful keyed-
// verdict closeout drives it into (change 0441). Success is never encoded as
// cancellation.
type epochState string

const (
	// EpochActive: the epoch owns the run; new participants may register, and the
	// worktree admission fence links its slots to this epoch.
	EpochActive epochState = "active"
	// EpochCancelling: an explicit cancellation (run.cancel, Task 10) has fenced the
	// epoch and is tearing the run down; no new participant, start, or mutation admits.
	EpochCancelling epochState = "cancelling"
	// EpochCancelled: cancellation completed with full accounting; the epoch is
	// terminal and admits nothing.
	EpochCancelled epochState = "cancelled"
	// EpochSuperseded: a resume (Task 12) atomically superseded a confirmed-cancelled
	// epoch, reserving exactly one replacement dispatch. Terminal for this epoch.
	EpochSuperseded epochState = "superseded"
)

// EpochCompleting / EpochCompleted: the successful-run closeout lifecycle
// (change 0441). Completing is the durable success fence — RunGateVerdict
// verified run-complete but ownership accounting/retirement is unfinished, so
// the epoch still owns its worktree and admits no NEW registration, start,
// mutation, takeover, or relaunch. Completed means retirement finished:
// terminal, excluded from ambient worktree-owner lookup, revoked for explicit
// references. Success is never encoded as cancellation.
const (
	EpochCompleting epochState = "completing"
	EpochCompleted  epochState = "completed"
)

// EpochParticipant is one registered coordinator/worker/task/raw-run boundary the
// epoch tracks so a cancellation knows what to stop. NativeHandle is the adapter's
// own opaque task handle (a thread/turn id, a drive id) — a locator, never a
// credential. RegisteredAt is an RFC3339 UTC stamp set at registration.
type EpochParticipant struct {
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
// terminal-observation statuses RecordEpochParticipantTerminal will store — a
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

// EpochRecord is the durable run-epoch state. GateKey binds it to the arming gate
// record it lives beside; ChangeID and Worktree are bound once the claim confirms
// the change instance (bindEpochChange) and a scope claims the feature worktree.
// EpochID is the random public locator. Participants and AdmittedMutations are the
// accounting a cancellation reconciles.
type EpochRecord struct {
	SchemaVersion     int                `json:"schema_version"`
	GateKey           string             `json:"gate_key"`
	ChangeID          string             `json:"change_id,omitempty"`
	Worktree          string             `json:"worktree,omitempty"`
	State             epochState         `json:"state"`
	EpochID           string             `json:"epoch_id"`
	Participants      []EpochParticipant `json:"participants,omitempty"`
	AdmittedMutations []AdmittedMutation `json:"admitted_mutations,omitempty"`
	// ReplacementReserved carries the resume winner's replacement reservation (the
	// new gate key) once a confirmed-cancelled epoch is superseded (Task 12). Empty
	// for an active epoch.
	ReplacementReserved string    `json:"replacement_reserved,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// storedEpoch is the on-disk envelope: the store-owned physical generation token
// beside the EpochRecord it guards, mirroring storedScope/storedAdmission for the
// gatedrive stores. The generation rotates on every accepted write so a caller can
// prove it is mutating the state it last read.
type storedEpoch struct {
	Generation string      `json:"generation"`
	Record     EpochRecord `json:"record"`
}

// EpochErrorKind is the typed category of an EpochError, so a caller can branch on
// a not-found / corrupt / not-active / mismatch condition rather than string prose.
type EpochErrorKind string

const (
	// ErrEpochNotFound: no epoch record exists for the gate key (never minted, or
	// pruned with its gate record).
	ErrEpochNotFound EpochErrorKind = "epoch-not-found"
	// ErrEpochCorrupt: the record could not be decoded, or its schema version is not
	// the one this store understands. Fail closed.
	ErrEpochCorrupt EpochErrorKind = "epoch-corrupt"
	// ErrEpochExists: a mint found an epoch already minted for the gate key. Bind-once:
	// a second mint can never overwrite the first.
	ErrEpochExists EpochErrorKind = "epoch-exists"
	// ErrEpochNotActive: a participant registration (or another active-only transition)
	// was attempted on an epoch whose state is not active — a fenced or terminal epoch
	// admits no new participant.
	ErrEpochNotActive EpochErrorKind = "epoch-not-active"
	// ErrEpochMismatch: a caller presented an expected epoch id that is not this
	// record's EpochID — a stale locator. It confers no registration authority.
	ErrEpochMismatch EpochErrorKind = "epoch-mismatch"
	// ErrEpochNotCancelled: a resume attempted to supersede an epoch whose state is
	// not confirmed-cancelled — resume reserves a replacement ONLY after confirmed
	// cancellation, never over an active or still-cancelling run (change 0375 Task 12).
	ErrEpochNotCancelled EpochErrorKind = "epoch-not-cancelled"
	// ErrEpochAmbiguous: more than one non-superseded epoch matches one change id, so
	// the run a resume targets cannot be resolved to a single epoch. Fail closed.
	ErrEpochAmbiguous EpochErrorKind = "epoch-ambiguous"
	// ErrEpochOwnerAmbiguous: two or more ACTIVE (or completing) epochs are bound to
	// one canonical worktree, so ambient owner lookup (findEpochByWorktree) cannot name
	// a single current owner. It is a contradiction, never resolved by directory order
	// or timestamp: the path fence refuses locally (change 0446 spec §5).
	ErrEpochOwnerAmbiguous EpochErrorKind = "epoch-owner-ambiguous"
	// ErrEpochOwnerUnresolved: the worktree's execution slot names a RunEpochID that no
	// readable epoch record carries, so the worktree's current owner is unresolved. The
	// path fence refuses locally with that locator rather than admitting unfenced
	// (change 0446 spec §1).
	ErrEpochOwnerUnresolved EpochErrorKind = "epoch-owner-unresolved"
	// ErrEpochIO: an underlying filesystem, lock, or randomness operation failed.
	ErrEpochIO EpochErrorKind = "epoch-io"
	// ErrEpochParticipantUnknown: a terminal-observation record named a native
	// handle that no registered participant carries (change 0441) — evidence for a
	// participant this epoch never registered is never stored.
	ErrEpochParticipantUnknown EpochErrorKind = "epoch-participant-unknown"
)

// EpochError is the epoch store's typed failure carrying a stable kind and stage.
// It never embeds record content or any credential.
type EpochError struct {
	Kind EpochErrorKind
	Op   string
	err  error
}

func (e *EpochError) Error() string {
	if e.err != nil {
		return fmt.Sprintf("run epoch %s: %s: %v", e.Op, e.Kind, e.err)
	}
	return fmt.Sprintf("run epoch %s: %s", e.Op, e.Kind)
}

func (e *EpochError) Unwrap() error { return e.err }

func epochErr(kind EpochErrorKind, op string, err error) *EpochError {
	return &EpochError{Kind: kind, Op: op, err: err}
}

// AsEpochError unwraps err to an *EpochError when one is in the chain so a caller
// can branch on its Kind.
func AsEpochError(err error) (*EpochError, bool) {
	var e *EpochError
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// MintEpochRecord mints a fresh run epoch beside the gate record for gateKey,
// keyed by the gate key. changeID may be "" (a fresh non-resume arm binds the
// change later, at claim confirmation, via bindEpochChange). The gate-key
// directory must already exist (minted by MintGateRecord). It mints under the
// per-key flock and refuses ErrEpochExists if an epoch was already minted for the
// key — bind-once, a second arm can never clobber the first. On success it returns
// the persisted active record (EpochID + Generation stamped).
func MintEpochRecord(repoDir, gateKey, changeID string) (EpochRecord, error) {
	dir, err := gateKeyDir(repoDir, gateKey, "mint-epoch")
	if err != nil {
		return EpochRecord{}, err
	}
	lock, err := acquireEpochLock(dir)
	if err != nil {
		return EpochRecord{}, err
	}
	defer lock.Close()

	if _, serr := os.Stat(filepath.Join(dir, epochRecordFileName)); serr == nil {
		return EpochRecord{}, epochErr(ErrEpochExists, "mint-epoch", nil)
	} else if !errors.Is(serr, fs.ErrNotExist) {
		return EpochRecord{}, epochErr(ErrEpochIO, "mint-epoch", serr)
	}

	epochID, err := epochToken()
	if err != nil {
		return EpochRecord{}, epochErr(ErrEpochIO, "mint-epoch", err)
	}
	gen, err := epochToken()
	if err != nil {
		return EpochRecord{}, epochErr(ErrEpochIO, "mint-epoch", err)
	}
	now := time.Now().UTC()
	rec := EpochRecord{
		SchemaVersion: epochSchemaVersion,
		GateKey:       gateKey,
		ChangeID:      changeID,
		State:         EpochActive,
		EpochID:       epochID,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := writeEpochAtomic(dir, storedEpoch{Generation: gen, Record: rec}); err != nil {
		return EpochRecord{}, err
	}
	return rec, nil
}

// LoadEpochRecord reads the epoch record for gateKey and returns it with its
// physical generation. It fails closed on a missing record (ErrEpochNotFound), an
// unparseable document or an unknown schema version (ErrEpochCorrupt) — a record
// the store cannot read is never a live epoch.
func LoadEpochRecord(repoDir, gateKey string) (EpochRecord, string, error) {
	dir, err := gateKeyDir(repoDir, gateKey, "load-epoch")
	if err != nil {
		return EpochRecord{}, "", err
	}
	return readStoredEpoch(dir, "load-epoch")
}

// readStoredEpoch decodes the epoch envelope in dir and fails closed on a missing,
// corrupt, or unknown-schema record. It takes no lock: the atomic rename in every
// write guarantees a reader observes a whole document.
func readStoredEpoch(dir, op string) (EpochRecord, string, error) {
	buf, err := os.ReadFile(filepath.Join(dir, epochRecordFileName))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return EpochRecord{}, "", epochErr(ErrEpochNotFound, op, err)
		}
		return EpochRecord{}, "", epochErr(ErrEpochIO, op, err)
	}
	var stored storedEpoch
	if err := json.Unmarshal(buf, &stored); err != nil {
		return EpochRecord{}, "", epochErr(ErrEpochCorrupt, op, err)
	}
	if stored.Record.SchemaVersion != epochSchemaVersion {
		return EpochRecord{}, "", epochErr(ErrEpochCorrupt, op,
			fmt.Errorf("epoch schema version %d, want %d", stored.Record.SchemaVersion, epochSchemaVersion))
	}
	return stored.Record, stored.Generation, nil
}

// RegisterEpochParticipant appends p to the epoch's participants under the CAS. It
// verifies expectEpoch matches the record's EpochID when expectEpoch is non-empty
// (a stale locator is ErrEpochMismatch) and REJECTS when the state is not active
// (ErrEpochNotActive) — a fenced or terminal epoch admits no new participant. It
// stamps RegisteredAt (RFC3339 UTC) when the caller left it empty.
func RegisterEpochParticipant(repoDir, gateKey, expectEpoch string, p EpochParticipant) error {
	return epochCAS(repoDir, gateKey, func(rec *EpochRecord) error {
		if expectEpoch != "" && rec.EpochID != expectEpoch {
			return epochErr(ErrEpochMismatch, "register-participant", nil)
		}
		if rec.State != EpochActive {
			return epochErr(ErrEpochNotActive, "register-participant", nil)
		}
		if p.RegisteredAt == "" {
			p.RegisteredAt = time.Now().UTC().Format(time.RFC3339)
		}
		rec.Participants = append(rec.Participants, p)
		return nil
	})
}

// RecordEpochParticipantTerminal stamps the adapter's exact terminal observation
// (change 0441) onto the participant whose NativeHandle equals handle. Completing
// an ALREADY-registered participant's record is observation of fact, so — unlike
// RegisterEpochParticipant — it is allowed in ANY epoch state (mirroring the
// mutation journal's completion callback, which has no state gate); registering
// NEW work stays active-only. It fails closed on malformed evidence: an empty
// handle/turn or a status outside {completed, failed} is ErrEpochMismatch, and a
// handle no participant carries is ErrEpochParticipantUnknown. It is idempotent on
// identical evidence; a DIFFERENT already-recorded status or turn is ErrEpochMismatch
// — recorded terminal evidence is never silently overwritten. A non-empty
// expectEpoch mismatching EpochID is ErrEpochMismatch (a stale locator).
func RecordEpochParticipantTerminal(repoDir, gateKey, expectEpoch, handle, turn, status string) error {
	if handle == "" || turn == "" ||
		(status != ParticipantTerminalCompleted && status != ParticipantTerminalFailed) {
		return epochErr(ErrEpochMismatch, "record-participant-terminal", nil)
	}
	err := epochCAS(repoDir, gateKey, func(rec *EpochRecord) error {
		if expectEpoch != "" && rec.EpochID != expectEpoch {
			return epochErr(ErrEpochMismatch, "record-participant-terminal", nil)
		}
		for i := range rec.Participants {
			p := &rec.Participants[i]
			if p.NativeHandle != handle {
				continue
			}
			if p.TerminalStatus != "" {
				if p.TerminalStatus == status && p.TerminalTurn == turn {
					return errEpochFenceNoWrite // idempotent replay of identical evidence
				}
				return epochErr(ErrEpochMismatch, "record-participant-terminal", nil)
			}
			p.TerminalStatus = status
			p.TerminalTurn = turn
			p.TerminalObservedAt = time.Now().UTC().Format(time.RFC3339)
			return nil
		}
		return epochErr(ErrEpochParticipantUnknown, "record-participant-terminal", nil)
	})
	if errors.Is(err, errEpochFenceNoWrite) {
		return nil
	}
	return err
}

// bindEpochChange binds the epoch's ChangeID once, at claim confirmation, so the
// epoch records which change instance it owns (a locator for a later resume/cancel
// lookup, per ADR-0111 proofs — the committed claim receipt remains authority). It
// is a NO-OP when no epoch exists for the key (a standalone gate arms none): an
// ErrEpochNotFound is swallowed so a claim over a keyless or epoch-less dispatch is
// unaffected. Binding is idempotent: an already-bound identical id is a no-op; a
// bind over a different id fails closed (ErrEpochMismatch) so a confirmed claim can
// never silently re-point an epoch's change. Callers treat it best-effort.
func bindEpochChange(repoDir, gateKey, changeID string) error {
	err := epochCAS(repoDir, gateKey, func(rec *EpochRecord) error {
		if rec.ChangeID != "" {
			if rec.ChangeID == changeID {
				return nil // idempotent
			}
			return epochErr(ErrEpochMismatch, "bind-epoch-change", nil)
		}
		rec.ChangeID = changeID
		return nil
	})
	if ee, ok := AsEpochError(err); ok && ee.Kind == ErrEpochNotFound {
		return nil // no epoch (standalone gate): nothing to bind
	}
	return err
}

// bindEpochWorktree binds the epoch's Worktree once, at claim confirmation, so a
// FRESH (non-resume) run's epoch is locatable by the mutation fence
// (findEpochByWorktree) and actionable by run.cancel's worktree teardown
// (reconcileWorktreeSlot) — the same job armResumeReplacement does for the resume path
// (change 0375). Without it a fresh run's epoch keeps Worktree == "", which every
// worktree-keyed consumer skips, so the fence and the teardown are inert for the common
// first-dispatch case. The bound value is the LOGICAL feature worktree path (it need
// not exist yet at bind time): the fence canonicalizes the stored value at COMPARE time
// (epochOwnsWorktree), once the workspace exists, so a logical spelling resolves to the
// same canonical worktree the mutations run in. It is a NO-OP on an empty worktree
// (nothing to bind) and a NO-OP when no epoch exists for the key (a standalone gate
// arms none): ErrEpochNotFound is swallowed so a claim over a keyless or epoch-less
// dispatch is unaffected. Binding is idempotent: an already-bound identical worktree is
// a no-op; a bind over a different worktree fails closed (ErrEpochMismatch) so a
// confirmed claim can never silently re-point an epoch's worktree (mirroring
// bindEpochChange). Callers treat it best-effort.
func bindEpochWorktree(repoDir, gateKey, worktree string) error {
	if worktree == "" {
		return nil // nothing to bind (a keyless/standalone confirm, or an unknown path)
	}
	err := epochCAS(repoDir, gateKey, func(rec *EpochRecord) error {
		if rec.Worktree != "" {
			if rec.Worktree == worktree {
				return nil // idempotent
			}
			return epochErr(ErrEpochMismatch, "bind-epoch-worktree", nil)
		}
		rec.Worktree = worktree
		return nil
	})
	if ee, ok := AsEpochError(err); ok && ee.Kind == ErrEpochNotFound {
		return nil // no epoch (standalone gate): nothing to bind
	}
	return err
}

// epochCAS runs a logical epoch transition under a flock-serialized physical
// compare-and-swap: it acquires the per-key epoch.lock, reads the current record,
// applies mutate to a copy, and atomically writes it back under a freshly rotated
// generation. Any error mutate returns is a deliberate logical rejection (or a
// real IO fault) and aborts with NO write, so a rejected transition leaves the
// persisted record untouched. The whole read-modify-write is under the lock, so a
// concurrent registration serializes rather than losing an update.
func epochCAS(repoDir, gateKey string, mutate func(*EpochRecord) error) error {
	dir, err := gateKeyDir(repoDir, gateKey, "epoch-cas")
	if err != nil {
		return err
	}
	lock, err := acquireEpochLock(dir)
	if err != nil {
		return err
	}
	defer lock.Close()

	rec, _, err := readStoredEpoch(dir, "epoch-cas")
	if err != nil {
		return err
	}
	if err := mutate(&rec); err != nil {
		return err
	}
	rec.SchemaVersion = epochSchemaVersion
	rec.UpdatedAt = time.Now().UTC()
	gen, err := epochToken()
	if err != nil {
		return epochErr(ErrEpochIO, "epoch-cas", err)
	}
	return writeEpochAtomic(dir, storedEpoch{Generation: gen, Record: rec})
}

// writeEpochAtomic marshals stored and writes it at dir/epoch.json through a
// same-directory temp file followed by os.Rename — the atomic-adjacent replacement
// rule, matching writeGateRecordAtomic. The temp file is created 0600 so the
// private record's mode survives the rename.
func writeEpochAtomic(dir string, stored storedEpoch) error {
	buf, err := json.Marshal(stored)
	if err != nil {
		return epochErr(ErrEpochIO, "write-epoch", err)
	}
	tmp, err := os.CreateTemp(dir, "."+epochRecordFileName+".tmp-*")
	if err != nil {
		return epochErr(ErrEpochIO, "write-epoch", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful rename
	if _, err := tmp.Write(buf); err != nil {
		tmp.Close()
		return epochErr(ErrEpochIO, "write-epoch", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return epochErr(ErrEpochIO, "write-epoch", err)
	}
	if err := tmp.Close(); err != nil {
		return epochErr(ErrEpochIO, "write-epoch", err)
	}
	if err := os.Rename(tmpName, filepath.Join(dir, epochRecordFileName)); err != nil {
		return epochErr(ErrEpochIO, "write-epoch", err)
	}
	return nil
}

// acquireEpochLock opens (creating if needed) the per-key epoch.lock and takes an
// exclusive flock, mirroring gatedrive's acquireExclusiveLock: the flock is the
// critical-section primitive for one read-modify-write, never the lifetime
// guarantee (the persisted state is the authority).
func acquireEpochLock(dir string) (*os.File, error) {
	f, err := os.OpenFile(filepath.Join(dir, epochLockFileName), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, epochErr(ErrEpochIO, "lock-open", err)
	}
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return nil, epochErr(ErrEpochIO, "lock-chmod", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, epochErr(ErrEpochIO, "lock", err)
	}
	return f, nil
}

// errEpochAlreadySuperseded is the sentinel SupersedeCancelledEpoch's CAS returns
// when a concurrent resume already superseded the epoch — the loser observes the
// winner's reservation rather than reserving a second replacement. It is internal
// to the supersede one-winner race and never surfaces as a typed EpochError.
var errEpochAlreadySuperseded = errors.New("run epoch already superseded")

// errEpochFenceNoWrite aborts a completion-lifecycle CAS with no write when the
// observed state already satisfies the transition (idempotent replay).
var errEpochFenceNoWrite = errors.New("run epoch completion state already satisfied")

// FenceEpochCompleting compare-and-swaps an active epoch active→completing, the
// durable success fence a verified keyed run-complete drives (change 0441). It
// returns the state OBSERVED under the lock. active→fenced (EpochCompleting, nil);
// already completing→idempotent replay (EpochCompleting, nil); completed→
// (EpochCompleted, nil) (replay of a finished closeout); any cancelling/cancelled/
// superseded/unknown state is returned as-is plus ErrEpochNotActive and is NEVER
// relabelled successful. A non-empty expectEpoch mismatching EpochID is
// ErrEpochMismatch — a stale locator confers no completion authority.
func FenceEpochCompleting(repoDir, gateKey, expectEpoch string) (epochState, error) {
	var observed epochState
	err := epochCAS(repoDir, gateKey, func(rec *EpochRecord) error {
		if expectEpoch != "" && rec.EpochID != expectEpoch {
			return epochErr(ErrEpochMismatch, "fence-completing", nil)
		}
		observed = rec.State
		switch rec.State {
		case EpochActive:
			rec.State = EpochCompleting
			observed = EpochCompleting
			return nil
		case EpochCompleting, EpochCompleted:
			return errEpochFenceNoWrite // idempotent observation, no write
		default:
			return epochErr(ErrEpochNotActive, "fence-completing", nil)
		}
	})
	if errors.Is(err, errEpochFenceNoWrite) {
		return observed, nil
	}
	return observed, err
}

// CompleteEpoch compare-and-swaps a fenced epoch completing→completed, retiring a
// successful closeout (change 0441). already completed→idempotent nil (a completed
// receipt replay is safe); ANY other state is ErrEpochNotActive — a concurrent
// cancellation that won from completing makes completion lose, and there is no
// shortcut past the fence from active.
func CompleteEpoch(repoDir, gateKey string) error {
	err := epochCAS(repoDir, gateKey, func(rec *EpochRecord) error {
		switch rec.State {
		case EpochCompleting:
			rec.State = EpochCompleted
			return nil
		case EpochCompleted:
			return errEpochFenceNoWrite
		default:
			return epochErr(ErrEpochNotActive, "complete-epoch", nil)
		}
	})
	if errors.Is(err, errEpochFenceNoWrite) {
		return nil
	}
	return err
}

// SupersedeCancelledEpoch atomically transitions a CONFIRMED-CANCELLED epoch to
// superseded and records replacementKey as its one reserved replacement dispatch
// (change 0375 Task 12, spec "After confirmed cancellation, atomically supersede
// the old epoch and reserve one replacement dispatch. Two concurrent resumes
// produce one winner"). The whole read-check-write runs under the epoch CAS, so two
// concurrent resumes serialize: the WINNER sees the cancelled state, sets superseded
// + ReplacementReserved, and returns nil; the LOSER sees the already-superseded
// state and returns errEpochAlreadySuperseded (its caller re-reads and reports the
// winner's reservation). Any non-cancelled state (active/cancelling) is
// ErrEpochNotCancelled — resume never supersedes a run that has not confirmed
// cancellation. A superseded epoch owns no worktree for the mutation fence, so its
// Worktree is cleared as the same atomic transition.
func SupersedeCancelledEpoch(repoDir, gateKey, replacementKey string) error {
	return epochCAS(repoDir, gateKey, func(rec *EpochRecord) error {
		switch rec.State {
		case EpochCancelled:
			rec.State = EpochSuperseded
			rec.ReplacementReserved = replacementKey
			rec.Worktree = "" // a superseded epoch no longer owns the worktree fence
			return nil
		case EpochSuperseded:
			return errEpochAlreadySuperseded
		default:
			return epochErr(ErrEpochNotCancelled, "supersede-epoch", nil)
		}
	})
}

// FindEpochByChange resolves the run epoch a resume of changeID targets by scanning
// the repository's rungate root (each gate-key directory may hold one epoch.json).
// It returns the matching epoch's gate key and record, found=false when no epoch
// names the change, and a typed error for an enumeration fault or an unresolvable
// ambiguity.
//
// Disambiguation across a resume chain (E1 superseded → E2 …): a matched epoch that
// is NOT superseded is the current run and wins — exactly one such epoch must exist
// (more is ErrEpochAmbiguous). When every match is superseded (the replacement has
// not yet bound its own change at claim time), the unique superseded match — or, in
// a longer chain, the tail whose ReplacementReserved points outside the matched set —
// is returned, so a repeat resume still recovers the reservation. A missing rungate
// root or no match is (found=false, nil); a corrupt/unreadable sibling epoch is
// skipped, mirroring findEpochByWorktree.
func FindEpochByChange(repoDir, changeID string) (gateKey string, rec EpochRecord, found bool, err error) {
	if changeID == "" {
		return "", EpochRecord{}, false, nil
	}
	root, rerr := gateRoot(repoDir)
	if rerr != nil {
		return "", EpochRecord{}, false, rerr
	}
	entries, derr := os.ReadDir(root)
	if derr != nil {
		if errors.Is(derr, fs.ErrNotExist) {
			return "", EpochRecord{}, false, nil // no rungate root: no epochs
		}
		return "", EpochRecord{}, false, epochErr(ErrEpochIO, "find-by-change", derr)
	}
	type matchEntry struct {
		key string
		rec EpochRecord
	}
	var matches []matchEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		key := e.Name()
		r, _, lerr := readStoredEpoch(filepath.Join(root, key), "find-by-change")
		if lerr != nil {
			continue // no epoch.json here, or a corrupt/unreadable sibling: cannot match
		}
		if r.ChangeID == changeID {
			matches = append(matches, matchEntry{key: key, rec: r})
		}
	}
	if len(matches) == 0 {
		return "", EpochRecord{}, false, nil
	}
	// Prefer a non-superseded match: it is the current run for the change.
	var live []matchEntry
	for _, m := range matches {
		if m.rec.State != EpochSuperseded {
			live = append(live, m)
		}
	}
	switch {
	case len(live) == 1:
		return live[0].key, live[0].rec, true, nil
	case len(live) > 1:
		return "", EpochRecord{}, false, epochErr(ErrEpochAmbiguous, "find-by-change", nil)
	}
	// Every match is superseded: resolve to the chain tail whose reserved replacement
	// is not itself one of the matched (superseded) epochs.
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
	return "", EpochRecord{}, false, epochErr(ErrEpochAmbiguous, "find-by-change", nil)
}

// epochDirMatch is one gate-key directory whose epoch.json records the sought
// EpochID: the shared shape scanEpochsByID yields to both findEpochByID (first
// match) and findEpochDirByID (unique match).
type epochDirMatch struct {
	dir string
	rec EpochRecord
}

// scanEpochsByID is the single walker under findEpochByID and findEpochDirByID (one
// walker, two shapes): it enumerates rungateRoot and returns every gate-key
// directory whose epoch.json records EpochID == epochID. An empty id or a missing
// root is (nil, nil); an enumeration fault is a typed ErrEpochIO; a corrupt or
// unreadable sibling is SKIPPED for matching (it cannot prove it holds the sought
// id), mirroring findEpochByWorktree's conservative skip.
func scanEpochsByID(rungateRoot, epochID string) ([]epochDirMatch, error) {
	if epochID == "" {
		return nil, nil
	}
	entries, derr := os.ReadDir(rungateRoot)
	if derr != nil {
		if errors.Is(derr, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, epochErr(ErrEpochIO, "find-by-id", derr)
	}
	var matches []epochDirMatch
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(rungateRoot, e.Name())
		r, _, lerr := readStoredEpoch(dir, "find-by-id")
		if lerr != nil {
			continue
		}
		if r.EpochID == epochID {
			matches = append(matches, epochDirMatch{dir: dir, rec: r})
		}
	}
	return matches, nil
}

// findEpochByID locates the epoch whose public EpochID equals epochID by scanning
// rungateRoot (each gate-key directory may hold one epoch.json). It returns the
// record and found=true on a match, (found=false, nil) for a clean absence, and a
// typed error only for an enumeration fault. A corrupt/unreadable sibling is
// skipped. It underlies the Takeover revocation resolver, which keys on a scope's
// RunEpochID (the public locator, not the gate key).
func findEpochByID(rungateRoot, epochID string) (EpochRecord, bool, error) {
	matches, err := scanEpochsByID(rungateRoot, epochID)
	if err != nil {
		return EpochRecord{}, false, err
	}
	if len(matches) == 0 {
		return EpochRecord{}, false, nil
	}
	return matches[0].rec, true, nil
}

// findEpochDirByID resolves the UNIQUE gate-key directory holding the epoch whose
// public EpochID is epochID (change 0437 Task 5 — the epoch launch gate locates the
// key directory it must lock and re-read under). Zero matches → ErrEpochNotFound;
// more than one → ErrEpochAmbiguous; corrupt/unreadable siblings are skipped for
// matching but the enumeration-fault contract mirrors findEpochByID. It shares the
// one walker (scanEpochsByID) with findEpochByID.
func findEpochDirByID(rungateRoot, epochID string) (dir string, rec EpochRecord, err error) {
	matches, serr := scanEpochsByID(rungateRoot, epochID)
	if serr != nil {
		return "", EpochRecord{}, serr
	}
	switch len(matches) {
	case 0:
		return "", EpochRecord{}, epochErr(ErrEpochNotFound, "find-dir-by-id", nil)
	case 1:
		return matches[0].dir, matches[0].rec, nil
	default:
		return "", EpochRecord{}, epochErr(ErrEpochAmbiguous, "find-dir-by-id", nil)
	}
}

// epochRevokedResolver builds the gatedrive.EpochRevokedFunc the Takeover path
// consults (change 0375 Task 12). It reads the app-owned run-epoch registry under
// gitCommonDir and reports revoked=true when the named epoch is cancelled,
// superseded, or completing/completed (a successful closeout — change 0441; a
// takeover of a completing/completed run refuses, and explicit references to a
// completed epoch remain revoked) — the states a takeover must refuse. A clean
// "no such epoch" is
// (false, nil): a locator that resolves to nothing cannot prove a run was cancelled,
// and the takeover's other guards still protect it. An enumeration/IO fault is
// returned so the takeover fails closed (HALT epoch-unreadable).
func epochRevokedResolver(gitCommonDir string) func(string) (bool, error) {
	rungateRoot := filepath.Join(gitCommonDir, "docket", "rungate")
	return func(epochID string) (bool, error) {
		rec, ok, err := findEpochByID(rungateRoot, epochID)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
		return rec.State == EpochCancelled || rec.State == EpochSuperseded ||
			rec.State == EpochCompleting || rec.State == EpochCompleted, nil
	}
}

// epochSettledResolver builds the gatedrive.EpochSettledFunc the worktree admission
// fence consults when a RELEASED slot still names another run epoch (change 0446
// spec §§2, 5). It reads the app-owned run-epoch registry under gitCommonDir and
// reports settled=true only for an epoch whose readable record is terminal with
// its accounting done: completed (successful closeout retired — a completed run is
// never asked to be cancelled), cancelled (cancellation completed with full
// accounting), or superseded (a resume superseded an already confirmed-cancelled
// epoch). Active, cancelling, and completing epochs are NOT settled: they still own
// the worktree between drives until their own closeout completes. A clean "no such
// epoch" is (false, gatedrive.ErrEpochUnresolved) — a slot-named epoch with no
// readable record is an unresolved owner, never settlement — and an ambiguous id or
// an enumeration/IO fault is returned as an error; the fence fails closed on all.
func epochSettledResolver(gitCommonDir string) func(string) (bool, error) {
	rungateRoot := filepath.Join(gitCommonDir, "docket", "rungate")
	return func(epochID string) (bool, error) {
		_, rec, err := findEpochDirByID(rungateRoot, epochID)
		if err != nil {
			if ee, ok := AsEpochError(err); ok && ee.Kind == ErrEpochNotFound {
				// Unsettled, and typed so the admission refusal's remedy never points
				// at a run.cancel that cannot resolve this epoch.
				return false, gatedrive.ErrEpochUnresolved
			}
			return false, err
		}
		switch rec.State {
		case EpochCompleted, EpochCancelled, EpochSuperseded:
			return true, nil
		default:
			return false, nil
		}
	}
}

// epochToken mints a random 32-hex-char token (16 crypto-random bytes) for the
// public EpochID locator and for the physical generation. Both are opaque lookup
// tokens, never encoded state.
func epochToken() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
