// Durable recovery scopes: one per parent/child dispatch boundary.
//
// A recovery scope is the durable record that lets a parent recover a drive its
// direct child was dispatched under, after that child returns without handing
// off (takeover.go, Task 2). Each scope carries two SEPARATELY-minted opaque
// capabilities — a child capability handed to the dispatched child and a parent
// capability retained by the preparing parent and never exposed to the child —
// so a takeover can be authorized by the parent capability alone, distinct from
// the child's own authority.
//
// Scope records live beside the drive records under the repository's Git common
// directory, sharing every durability and privacy discipline the drive store
// established (store.go):
//
//	<git-common-dir>/docket/gate-scopes/v1/<opaque-scope-id>/record.json
//
// The directory is owner-only (0700) and its record is private (0600). Writes go
// through the same writeAtomicJSON helper, and every mutating transition runs
// under a per-scope blocking flock and a persisted physical-generation
// compare-and-swap (scopeCAS), mirroring Store.CAS/ownerCAS so physical
// contention never surfaces as a logical failure. Capabilities and GateContext
// are persisted only as sha256 hashes (capHash) — never the raw tokens — so a
// leaked record file discloses no authority. Unknown schema versions and corrupt
// records fail closed with a typed StoreError, exactly as the drive store does.
//
// Slot lifecycle and lock order (schema v2, change 0405). A scope carries a
// single drive slot that a SEQUENCE of drives passes through — baseline, RED,
// GREEN, verification — at most one current execution or launch reservation at a
// time. A start reserves the slot durably (reserveScopeDrive) BEFORE any process
// launch, then confirms it (confirmScopeLaunch) once the launch is persisted; a
// successor additionally journals the predecessor it must retire (the pending-ack
// journal), which is cleared (clearPendingAck) as the second half of that one
// logical transition. The current scope state — never a newest timestamp — is the
// authority on which drive is current.
//
// When a single logical transition needs both the scope authority and a drive's
// ownership authority, acquire the SCOPE LOCK BEFORE THE DRIVE LOCK; elsewhere the
// acquisitions are sequential (release between). Every authority holder revalidates
// its target after acquiring authority: Start, the final acknowledgement, and
// Handoff/Claim/Takeover re-check the scope's current drive under the lock so a
// stale reader cannot act on a drive a concurrent transition has already moved. A
// reservation is never a PASSED, FAILED, or safely quiescent result; a slot that
// is reserved or carries a pending-ack journal is an unresolved transition that
// fails closed rather than admitting a second launch.
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
)

// scopeSchemaVersion is the persisted scopeRecord schema generation. Like the
// drive store, an unknown version fails closed rather than being migrated.
// Bumped to 2 by change 0405, which replaces the single bound_drive_id with the
// slot lifecycle below (CurrentDriveID/CurrentDriveState/PriorDriveID/DriveCount
// and the pending-ack journal). A v1 record read by a v2 store fails closed as
// ErrUnknownSchema: an in-flight one-drive record is never silently reinterpreted
// as a reusable sequential scope (spec "Version changed persistent formats").
const scopeSchemaVersion = 2

// scopeStateReserved marks a slot whose successor drive record and scope
// reservation are persisted but whose process has not yet been launch-confirmed —
// the recoverable window between reservation and confirmScopeLaunch.
const scopeStateReserved = "reserved"

// scopeStateLaunched marks a slot whose current drive's process launch has been
// confirmed. Only a launched (and durably terminal) predecessor authorizes a
// successor reservation.
const scopeStateLaunched = "launched"

// predecessorReceipt is the explicit acknowledgement a successor start presents:
// the previous drive's id and its current owner generation. Both fields are set
// for a successor, or both empty for a scope's first start. A half-filled receipt
// is a fail-closed ErrStalePredecessor (spec "Subsequent tests").
type predecessorReceipt struct {
	DriveID  string
	OwnerGen string
}

// empty reports whether the receipt carries no predecessor (a first start).
func (r predecessorReceipt) empty() bool { return r.DriveID == "" && r.OwnerGen == "" }

// halfFilled reports whether exactly one of the receipt's two fields is set — a
// malformed receipt that names neither a complete predecessor nor a first start.
func (r predecessorReceipt) halfFilled() bool {
	return (r.DriveID == "") != (r.OwnerGen == "")
}

// scopeRecord is the durable, owner-private schema of one recovery scope. It
// stores only hashes of the two capabilities and of the outer gate-context
// token — never their raw values — plus the immutable dispatch identity the
// takeover path re-verifies. Every field carries an explicit snake_case json tag
// so the store round-trips it canonically.
type scopeRecord struct {
	SchemaVersion   int    `json:"schema_version"`
	RepoIdentity    string `json:"repo_identity"`
	ChangeID        string `json:"change_id"`
	TaskID          string `json:"task_id"`
	Phase           string `json:"phase"`
	Branch          string `json:"branch"`
	Worktree        string `json:"worktree"`
	GateContextHash string `json:"gate_context_hash,omitempty"`
	ChildCapHash    string `json:"child_cap_hash"`
	ParentCapHash   string `json:"parent_cap_hash"`

	// The single-slot lifecycle (schema v2). At most one current drive occupies
	// the slot at a time; a sequence of drives passes through it, each successor
	// acknowledging its predecessor. CurrentDriveID/CurrentDriveState name the slot
	// occupant and whether its launch is reserved or confirmed; PriorDriveID is the
	// immediately preceding drive (history); DriveCount is the total number of
	// drives the scope has admitted.
	CurrentDriveID    string `json:"current_drive_id,omitempty"`
	CurrentDriveState string `json:"current_drive_state,omitempty"` // "" | scopeStateReserved | scopeStateLaunched
	PriorDriveID      string `json:"prior_drive_id,omitempty"`
	DriveCount        int    `json:"drive_count"`

	// The pending-ack journal is the recoverable second-phase record of a successor
	// transition: after a successor reservation wins the slot, these name the
	// predecessor whose recovery authority must still be retired. They are cleared
	// (clearPendingAck) as the journaled second half of the one logical transition.
	PendingAckDriveID  string `json:"pending_ack_drive_id,omitempty"`
	PendingAckOwnerGen string `json:"pending_ack_owner_gen,omitempty"`

	// FinalAcked records that the scope's last result was consumed by the terminal
	// acknowledgement (Task 5) rather than by a claim or takeover.
	FinalAcked bool `json:"final_acked,omitempty"`

	Closed bool `json:"closed"`
}

// storedScope is the on-disk envelope: the store-owned physical generation token
// beside the scopeRecord it guards, mirroring storedRecord for drives.
type storedScope struct {
	Generation string      `json:"generation"`
	Record     scopeRecord `json:"record"`
}

// ScopeRequest identifies one parent/child dispatch boundary. GateContext is the
// RAW outer child-context token linking nested drives to the outer gate (may be
// empty for the outer scope itself); it is stored only as a sha256 hash.
type ScopeRequest struct {
	RepoIdentity string
	ChangeID     string // may be "" for a fresh outer scope; binds once later
	TaskID       string
	Phase        string
	Branch       string
	Worktree     string
	GateContext  string
}

// ScopeGrant returns the scope locator and the two SEPARATE opaque capabilities.
// ChildCapability goes to the dispatched child; ParentCapability is retained by
// the preparing parent and never exposed to the child.
type ScopeGrant struct {
	ScopeID          string
	ChildCapability  string
	ParentCapability string
}

// capHash returns the sha256 of a capability (or any raw token) as lowercase
// hex. It is the single boundary at which a raw token becomes a stored hash, so
// the persisted record never carries authority.
func capHash(capability string) string {
	sum := sha256.Sum256([]byte(capability))
	return hex.EncodeToString(sum[:])
}

// PrepareScope mints an opaque scope id and two separate opaque capabilities,
// persists the record carrying only their hashes (and the hash of GateContext
// when non-empty), and returns the grant. It creates a fresh owner-only
// directory and atomically writes the record — the same discipline as NewDrive.
func (s *Store) PrepareScope(req ScopeRequest) (ScopeGrant, error) {
	childCap, err := randomToken(idNBytes)
	if err != nil {
		return ScopeGrant{}, storeErr(ErrIO, "prepare-scope", err)
	}
	parentCap, err := randomToken(idNBytes)
	if err != nil {
		return ScopeGrant{}, storeErr(ErrIO, "prepare-scope", err)
	}
	id, err := randomToken(idNBytes)
	if err != nil {
		return ScopeGrant{}, storeErr(ErrIO, "prepare-scope", err)
	}
	gen, err := randomToken(genNBytes)
	if err != nil {
		return ScopeGrant{}, storeErr(ErrIO, "prepare-scope", err)
	}

	rec := scopeRecord{
		SchemaVersion: scopeSchemaVersion,
		RepoIdentity:  req.RepoIdentity,
		ChangeID:      req.ChangeID,
		TaskID:        req.TaskID,
		Phase:         req.Phase,
		Branch:        req.Branch,
		Worktree:      req.Worktree,
		ChildCapHash:  capHash(childCap),
		ParentCapHash: capHash(parentCap),
	}
	if req.GateContext != "" {
		rec.GateContextHash = capHash(req.GateContext)
	}

	dir := filepath.Join(s.scopeRoot, id) // id is our own hex token: no validation needed
	if err := ensurePrivateDir(dir); err != nil {
		return ScopeGrant{}, storeErr(ErrIO, "prepare-scope", err)
	}
	if err := writeAtomicJSON(filepath.Join(dir, recordFileName), storedScope{Generation: gen, Record: rec}); err != nil {
		return ScopeGrant{}, storeErr(ErrIO, "prepare-scope", err)
	}
	return ScopeGrant{ScopeID: id, ChildCapability: childCap, ParentCapability: parentCap}, nil
}

// LoadScope reads and returns the current record for a scope id. It validates
// the id and refuses a symlinked scope directory before touching the record, and
// fails closed on a corrupt document or an unknown schema version.
func (s *Store) LoadScope(id string) (scopeRecord, error) {
	dir, err := s.scopeDir(id)
	if err != nil {
		return scopeRecord{}, err
	}
	stored, err := s.readStoredScope(dir)
	if err != nil {
		return scopeRecord{}, err
	}
	return stored.Record, nil
}

// reserveScopeDrive persists a newDriveID into the scope's single slot under the
// child capability, as the durable FIRST half of a start transition (the launch
// itself follows, then confirmScopeLaunch). It is the serialization authority for
// the slot: the scopeCAS mutate re-checks, in order — capability, closed, receipt
// shape, then slot state — and mutates only on full agreement, so a rejected
// reserve writes nothing.
//
// An EMPTY slot accepts only an empty receipt (a first start): it fills the slot
// as reserved with drive count 1. A non-empty receipt on an empty scope is
// ErrStalePredecessor.
//
// An OCCUPIED slot accepts only a successor whose receipt names the current
// LAUNCHED drive: a reserved (unconfirmed) slot is ErrScopeBusy, a journaled
// pending ack is ErrUnresolvedLaunchTransition, an empty receipt is
// ErrScopeSecondDrive, and a receipt naming a non-current drive is
// ErrStalePredecessor. On success it advances the slot to the successor
// (reserved), records the predecessor as PriorDriveID, journals the pending ack,
// and increments DriveCount. Retiring the predecessor's recovery authority is the
// caller's journaled second half (Task 4 retirePredecessor + clearPendingAck).
func (s *Store) reserveScopeDrive(scopeID, childCapability, newDriveID string, receipt predecessorReceipt) error {
	return s.scopeCAS(scopeID, func(rec *scopeRecord) error {
		if childCapability == "" || rec.ChildCapHash != capHash(childCapability) {
			return ownershipErr(ErrScopeCapabilityMismatch, "reserve-scope-drive")
		}
		if rec.Closed {
			return ownershipErr(ErrScopeClosed, "reserve-scope-drive")
		}
		if receipt.halfFilled() {
			return ownershipErr(ErrStalePredecessor, "reserve-scope-drive")
		}
		if rec.CurrentDriveID == "" {
			// Empty slot: only a first start (empty receipt) may fill it.
			if !receipt.empty() {
				return ownershipErr(ErrStalePredecessor, "reserve-scope-drive")
			}
			rec.CurrentDriveID = newDriveID
			rec.CurrentDriveState = scopeStateReserved
			rec.DriveCount++
			return nil
		}
		// Occupied slot: a mid-transition state fails closed before the receipt is
		// even considered, so a reservation in flight or an unretired predecessor is
		// never overwritten.
		if rec.CurrentDriveState == scopeStateReserved {
			return ownershipErr(ErrScopeBusy, "reserve-scope-drive")
		}
		if rec.PendingAckDriveID != "" {
			return ownershipErr(ErrUnresolvedLaunchTransition, "reserve-scope-drive")
		}
		if receipt.empty() {
			return ownershipErr(ErrScopeSecondDrive, "reserve-scope-drive")
		}
		if receipt.DriveID != rec.CurrentDriveID {
			return ownershipErr(ErrStalePredecessor, "reserve-scope-drive")
		}
		rec.PriorDriveID = rec.CurrentDriveID
		rec.CurrentDriveID = newDriveID
		rec.CurrentDriveState = scopeStateReserved
		rec.PendingAckDriveID = receipt.DriveID
		rec.PendingAckOwnerGen = receipt.OwnerGen
		rec.DriveCount++
		return nil
	})
}

// confirmScopeLaunch flips the slot's current drive from reserved to launched
// once its process launch has been persisted, completing the visible half of a
// start. It acts only on the matching current drive id: a mismatched id is a
// fail-closed ErrUnresolvedLaunchTransition, and an already-launched matching id
// is an idempotent no-op. On any rejection the persisted record is untouched.
func (s *Store) confirmScopeLaunch(scopeID, driveID string) error {
	return s.scopeCAS(scopeID, func(rec *scopeRecord) error {
		if rec.CurrentDriveID != driveID {
			return ownershipErr(ErrUnresolvedLaunchTransition, "confirm-scope-launch")
		}
		if rec.CurrentDriveState == scopeStateLaunched {
			return nil // idempotent: the launch is already confirmed
		}
		if rec.CurrentDriveState != scopeStateReserved {
			return ownershipErr(ErrUnresolvedLaunchTransition, "confirm-scope-launch")
		}
		rec.CurrentDriveState = scopeStateLaunched
		return nil
	})
}

// clearPendingAck clears the pending-ack journal entry as the second half of a
// successor transition, once the predecessor's recovery authority has been
// retired. It clears only a journal entry naming predecessorID; a mismatch (a
// different id, or an already-cleared journal) is a fail-closed
// ErrUnresolvedLaunchTransition. On any rejection the persisted record is
// untouched.
func (s *Store) clearPendingAck(scopeID, predecessorID string) error {
	return s.scopeCAS(scopeID, func(rec *scopeRecord) error {
		if rec.PendingAckDriveID != predecessorID {
			return ownershipErr(ErrUnresolvedLaunchTransition, "clear-pending-ack")
		}
		rec.PendingAckDriveID = ""
		rec.PendingAckOwnerGen = ""
		return nil
	})
}

// bindScopeChange binds the outer scope's change id exactly once. An empty
// ChangeID is set; rebinding the same id is a no-op; rebinding a different id
// fails closed; a closed scope refuses the bind. On any rejection the persisted
// record is untouched.
func (s *Store) bindScopeChange(scopeID, changeID string) error {
	return s.scopeCAS(scopeID, func(rec *scopeRecord) error {
		if rec.Closed {
			return ownershipErr(ErrScopeClosed, "bind-scope-change")
		}
		if rec.ChangeID != "" {
			if rec.ChangeID == changeID {
				return nil // idempotent
			}
			return ownershipErr(ErrScopeIdentityMismatch, "bind-scope-change")
		}
		rec.ChangeID = changeID
		return nil
	})
}

// closeScope marks a scope closed. It is idempotent — closing an already-closed
// scope is not an error. The nearest-parent chain closes a scope on both the
// normal claim path and the event-authorized takeover path (Task 2).
func (s *Store) closeScope(scopeID string) error {
	return s.scopeCAS(scopeID, func(rec *scopeRecord) error {
		rec.Closed = true
		return nil
	})
}

// scopeDir validates a user-supplied scope id, then confirms its directory
// exists and is a real directory (not a symlink) before any record path is
// constructed — the same traversal/symlink guarantees as driveDir.
func (s *Store) scopeDir(id string) (string, error) {
	if err := validateID(id); err != nil {
		return "", err
	}
	dir := filepath.Join(s.scopeRoot, id)
	fi, err := os.Lstat(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", storeErr(ErrNotFound, "resolve-scope", err)
		}
		return "", storeErr(ErrIO, "resolve-scope", err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return "", storeErr(ErrInvalidID, "resolve-scope", nil)
	}
	return dir, nil
}

// readStoredScope decodes the scope envelope in dir and fails closed on a
// corrupt document or an unknown schema version.
func (s *Store) readStoredScope(dir string) (storedScope, error) {
	buf, err := os.ReadFile(filepath.Join(dir, recordFileName))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return storedScope{}, storeErr(ErrNotFound, "read-scope", err)
		}
		return storedScope{}, storeErr(ErrIO, "read-scope", err)
	}
	var stored storedScope
	if err := json.Unmarshal(buf, &stored); err != nil {
		return storedScope{}, storeErr(ErrCorruptRecord, "read-scope", err)
	}
	if stored.Record.SchemaVersion != scopeSchemaVersion {
		return storedScope{}, storeErr(ErrUnknownSchema, "read-scope",
			fmt.Errorf("scope schema version %d, want %d", stored.Record.SchemaVersion, scopeSchemaVersion))
	}
	return stored, nil
}

// scopeCASOnce performs a single flock-serialized compare-and-swap on a scope
// record, mirroring Store.CAS for drives: it reads the current record, and only
// when the stored physical generation equals expectGen applies mutate to a copy
// and atomically writes it back under a freshly rotated generation. A stale
// generation returns ErrGenerationMismatch and writes nothing; a mutate error
// aborts the transition with no write.
func (s *Store) scopeCASOnce(id, expectGen string, mutate func(*scopeRecord) error) (string, error) {
	dir, err := s.scopeDir(id)
	if err != nil {
		return "", err
	}
	lock, err := acquireExclusiveLock(filepath.Join(dir, lockFileName))
	if err != nil {
		return "", err
	}
	defer lock.Close()

	stored, err := s.readStoredScope(dir)
	if err != nil {
		return "", err
	}
	if stored.Generation != expectGen {
		return "", storeErr(ErrGenerationMismatch, "scope-cas", nil)
	}

	rec := stored.Record // a copy; mutate never touches the persisted bytes
	if err := mutate(&rec); err != nil {
		return "", err
	}
	rec.SchemaVersion = scopeSchemaVersion

	newGen, err := randomToken(genNBytes)
	if err != nil {
		return "", storeErr(ErrIO, "scope-cas", err)
	}
	if err := writeAtomicJSON(filepath.Join(dir, recordFileName), storedScope{Generation: newGen, Record: rec}); err != nil {
		return "", storeErr(ErrIO, "scope-cas", err)
	}
	return newGen, nil
}

// scopeCAS runs a logical scope transition under the physical compare-and-swap,
// mirroring ownerCAS: it re-reads the current physical generation and retries on
// a physical generation mismatch so concurrent writers serialize and physical
// contention never surfaces as a logical failure. Any error mutate itself
// returns is a deliberate logical rejection (or a real IO fault) and propagates
// immediately with no retry, so a rejected transition writes nothing.
func (s *Store) scopeCAS(id string, mutate func(*scopeRecord) error) error {
	var lastErr error
	for attempt := 0; attempt < ownerCASMaxAttempts; attempt++ {
		dir, err := s.scopeDir(id)
		if err != nil {
			return err
		}
		stored, err := s.readStoredScope(dir)
		if err != nil {
			return err
		}
		if _, err := s.scopeCASOnce(id, stored.Generation, mutate); err != nil {
			if se, ok := AsStoreError(err); ok && se.Kind == ErrGenerationMismatch {
				lastErr = err
				continue // physical contention only: re-read and retry
			}
			return err
		}
		return nil
	}
	return storeErr(ErrIO, "scope-cas", fmt.Errorf("exceeded %d attempts under contention: %w", ownerCASMaxAttempts, lastErr))
}
