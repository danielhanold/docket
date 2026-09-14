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
// the durable fence a later human cancellation or resume drives it into.
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

// EpochParticipant is one registered coordinator/worker/task/raw-run boundary the
// epoch tracks so a cancellation knows what to stop. NativeHandle is the adapter's
// own opaque task handle (a thread/turn id, a drive id) — a locator, never a
// credential. RegisteredAt is an RFC3339 UTC stamp set at registration.
type EpochParticipant struct {
	Kind         string `json:"kind"` // coordinator|task|gate-scope|raw-run
	NativeHandle string `json:"native_handle,omitempty"`
	RegisteredAt string `json:"registered_at"`
}

// AdmittedMutation is one journaled workflow-mutation admission at a shared
// mutation boundary (transaction engine, PR publish, workspace publish). Status is
// admitted|completed|uncertain; a cancellation stays pending until every admitted
// entry is completed or uncertain-reconciled (Task 11 wires the journal writes,
// Task 10 reads them). OpKey is the bounded operation key, never argv/env/content.
type AdmittedMutation struct {
	OpKey  string `json:"op_key"`
	Status string `json:"status"` // admitted|completed|uncertain
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
	// ErrEpochIO: an underlying filesystem, lock, or randomness operation failed.
	ErrEpochIO EpochErrorKind = "epoch-io"
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
