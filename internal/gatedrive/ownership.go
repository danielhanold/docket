// Ownership generations and the single-use handoff receipt for the gate driver.
//
// The store layer (store.go) owns a lower-level compare-and-swap: a physical
// generation token, rotated on every accepted write, that lets one writer prove
// it is mutating the state it last read. This file builds the OWNERSHIP layer on
// top of it. Ownership is a separate, logical notion from that physical token:
//
//   - An owner generation (driveRecord.OwnerGeneration) is the opaque claim an
//     agent holds. It is stable across ordinary advances — a plain WAITING slice
//     leaves it valid so the same owner can advance again — and changes only at
//     an ownership boundary (handoff, claim). It is NOT the store generation,
//     which rotates on every write.
//   - A handoff generation (driveRecord.HandoffGeneration) is a single-use
//     transfer token. Creating it invalidates the old owner; consuming it, once,
//     mints a fresh owner. Exactly one of the two fields is meaningfully set at a
//     time: an owned drive has an owner and no handoff; an offered drive has a
//     handoff and no owner.
//
// The primitives here (verifyOwner, invalidateOwner, writeHandoffReceipt,
// consumeHandoffCAS) are the CAS building blocks the Task-6 Driver.Handoff and
// Driver.Claim state-machine wiring composes; the full Driver is not built here.
//
// Every transition that mutates disk runs through ownerCAS, which performs the
// logical transition under the store's flock and physical CAS. The transition
// closure is the sole authority: it re-checks the ownership/handoff token and
// the repository fingerprint on every locked attempt and returns a typed
// OwnershipError to abort. A caller that lost the race, presented a stale token,
// or no longer fingerprint-matches acquires NO partial authority — the abort
// leaves the persisted record exactly as it was. Physical store contention is
// retried transparently and never surfaces as a logical rejection (spec
// "Explicit handoff and nearest-owner continuation").
package gatedrive

import (
	"errors"
	"fmt"
)

// OwnershipErrorKind is the typed category of an OwnershipError. Callers (and
// the Task-6 driver mapping HALT causes) key on it to tell a lost claim from a
// stale owner from a drifted worktree — each of which fails closed, never red.
type OwnershipErrorKind string

const (
	// ErrNotOwner: the presented generation is not the drive's current owner —
	// an empty token, a stale token, or the token of an owner already
	// invalidated by a handoff. It confers no authority to advance or hand off.
	ErrNotOwner OwnershipErrorKind = "not-owner"
	// ErrHandoffOutstanding: a handoff was requested on a drive that already has
	// an unclaimed handoff. The offer is single-use end to end; a second offer
	// is refused rather than overwriting the first.
	ErrHandoffOutstanding OwnershipErrorKind = "handoff-outstanding"
	// ErrNoHandoffOffered: a claim was attempted on a drive with no outstanding
	// handoff — a plain owned/WAITING drive, or a receipt already consumed.
	// Seeing a running suite is not authority to take it over.
	ErrNoHandoffOffered OwnershipErrorKind = "no-handoff-offered"
	// ErrHandoffMismatch: a claim presented a handoff token that is not the
	// drive's current outstanding one. The chain is unambiguous: only the exact
	// token consumes the receipt.
	ErrHandoffMismatch OwnershipErrorKind = "handoff-mismatch"
	// ErrFingerprintMismatch: the repository execution identity no longer matches
	// the drive-start fingerprint at an ownership boundary. A pass could no
	// longer certify the original bytes, so the boundary is refused (the Task-6
	// driver maps this to a stop-if-owned HALT, never red).
	ErrFingerprintMismatch OwnershipErrorKind = "fingerprint-mismatch"
	// ErrScopeCapabilityMismatch: a scope transition presented a capability whose
	// hash does not match the scope's stored child (or parent) capability hash —
	// an empty, wrong, or role-swapped token. It confers no scope authority.
	ErrScopeCapabilityMismatch OwnershipErrorKind = "scope-capability-mismatch"
	// ErrScopeSecondDrive: a bind was attempted on a scope that already binds a
	// different live drive. One scope binds at most one live drive; a second is
	// refused rather than overwriting the first (an idempotent re-bind of the
	// same drive id is a no-op, not this error).
	ErrScopeSecondDrive OwnershipErrorKind = "scope-second-live-drive"
	// ErrScopeClosed: a transition was attempted on a scope already closed by a
	// normal claim or an event-authorized takeover. A closed scope is terminal.
	ErrScopeClosed OwnershipErrorKind = "scope-closed"
	// ErrScopeIdentityMismatch: a scope's identity (its bound change, or an
	// identity field a takeover re-verifies) no longer matches what the caller
	// presented — e.g. rebinding a scope to a different change. Fail closed.
	ErrScopeIdentityMismatch OwnershipErrorKind = "scope-identity-mismatch"
)

// OwnershipError is the ownership layer's typed failure. Like StoreError it
// carries a stable kind and stage and never embeds record content, an owner
// credential, or repository bytes.
type OwnershipError struct {
	Kind OwnershipErrorKind
	Op   string
}

func (e *OwnershipError) Error() string {
	return fmt.Sprintf("gatedrive ownership %s: %s", e.Op, e.Kind)
}

func ownershipErr(kind OwnershipErrorKind, op string) *OwnershipError {
	return &OwnershipError{Kind: kind, Op: op}
}

// AsOwnershipError unwraps err to an *OwnershipError when one is in the chain,
// so a caller can branch on its Kind.
func AsOwnershipError(err error) (*OwnershipError, bool) {
	var e *OwnershipError
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// handoffReceipt is the single-use transfer offer created when the current owner
// hands off a drive. It names one unambiguous change/task/phase/drive/generation
// chain: the immutable work identity, the owner generation the handoff
// supersedes, and the fresh single-use handoff generation a claimant swaps (via
// consumeHandoffCAS) for a new owner generation. It carries the drive's
// execution fingerprint so a claimant knows the exact identity it must still
// match; the CAS re-verifies that match under the lock before granting any
// authority. Like DriveDoc it is a diagnostic surface — it carries no launch
// argv, environment values, or worktree content.
type handoffReceipt struct {
	// Change/task/phase identity — the work the drive certifies.
	ChangeID string
	TaskID   string
	Phase    string
	// DriveID is the opaque drive the receipt transfers.
	DriveID string
	// SupersededOwner is the owner generation the handoff invalidated — the tail
	// of the chain that can no longer act.
	SupersededOwner string
	// HandoffGeneration is the single-use token a fresh claimant presents. It is
	// distinct from every owner generation in the chain.
	HandoffGeneration string
	// Fingerprint is the drive-start execution identity a claimant must match
	// exactly at claim time.
	Fingerprint Fingerprint
}

// verifyOwner confirms ownerGen is the drive record's current owner generation.
// An empty presented token, an unowned record (owner invalidated by a handoff),
// or any mismatch is ErrNotOwner. It is a pure check on a loaded record, so the
// Task-6 Advance path and the write transitions below both key on it.
func verifyOwner(rec *driveRecord, ownerGen string) error {
	if ownerGen == "" || rec.OwnerGeneration == "" || rec.OwnerGeneration != ownerGen {
		return ownershipErr(ErrNotOwner, "verify-owner")
	}
	return nil
}

// invalidateOwner clears the current owner generation so no token verifies until
// a new owner is installed. It is the first half of a handoff: after it, the old
// owner can neither advance nor hand off again.
func invalidateOwner(rec *driveRecord) {
	rec.OwnerGeneration = ""
}

// writeHandoffReceipt, called by the current owner, atomically invalidates that
// owner and records a fresh single-use handoff generation, returning the receipt
// that names the full chain. It is an ownership boundary: it verifies the
// presented owner generation AND that the current repository fingerprint still
// matches the drive-start identity under the lock, so a stale owner cannot offer
// a handoff and a drifted worktree is refused rather than transferred. A drive
// that already has an outstanding unclaimed handoff is refused (single-use end
// to end). On any rejection the persisted record is untouched.
func (s *Store) writeHandoffReceipt(id, ownerGen string, current Fingerprint) (handoffReceipt, error) {
	handoffGen, err := randomToken(genNBytes)
	if err != nil {
		return handoffReceipt{}, storeErr(ErrIO, "write-handoff", err)
	}
	var receipt handoffReceipt
	err = s.ownerCAS(id, func(rec *driveRecord) error {
		if err := verifyOwner(rec, ownerGen); err != nil {
			return err
		}
		if rec.HandoffGeneration != "" {
			return ownershipErr(ErrHandoffOutstanding, "write-handoff")
		}
		if !rec.Fingerprint.Equal(current) {
			return ownershipErr(ErrFingerprintMismatch, "write-handoff")
		}
		receipt = handoffReceipt{
			ChangeID:          rec.ChangeID,
			TaskID:            rec.TaskID,
			Phase:             rec.Phase,
			DriveID:           id,
			SupersededOwner:   rec.OwnerGeneration,
			HandoffGeneration: handoffGen,
			Fingerprint:       rec.Fingerprint,
		}
		invalidateOwner(rec)
		rec.HandoffGeneration = handoffGen
		return nil
	})
	if err != nil {
		return handoffReceipt{}, err
	}
	return receipt, nil
}

// consumeHandoffCAS, called by a fresh claimant, atomically consumes the drive's
// single-use handoff receipt and returns a new owner generation. The transition
// is the sole authority under the lock: it refuses a drive with no outstanding
// handoff (ErrNoHandoffOffered), a handoff token that is not the current one
// (ErrHandoffMismatch), or a claimant whose recomputed fingerprint no longer
// matches the drive-start identity (ErrFingerprintMismatch). Only when all three
// agree does it install the new owner and clear the handoff, so the receipt is
// consumed exactly once and a claimant that lost the race or no longer matches
// acquires NO partial authority — a rejected claim leaves the receipt intact for
// a correct claimant.
func (s *Store) consumeHandoffCAS(id, handoffID string, current Fingerprint) (string, error) {
	newOwner, err := randomToken(genNBytes)
	if err != nil {
		return "", storeErr(ErrIO, "consume-handoff", err)
	}
	err = s.ownerCAS(id, func(rec *driveRecord) error {
		if rec.HandoffGeneration == "" {
			return ownershipErr(ErrNoHandoffOffered, "consume-handoff")
		}
		if rec.HandoffGeneration != handoffID {
			return ownershipErr(ErrHandoffMismatch, "consume-handoff")
		}
		if !rec.Fingerprint.Equal(current) {
			return ownershipErr(ErrFingerprintMismatch, "consume-handoff")
		}
		rec.OwnerGeneration = newOwner
		rec.HandoffGeneration = ""
		return nil
	})
	if err != nil {
		return "", err
	}
	return newOwner, nil
}

// ownerCASMaxAttempts bounds the physical-contention retry so a pathological
// storm of concurrent writers fails closed with a typed IO error rather than
// spinning forever. Ordinary contention (a handful of racing claimants)
// resolves in one or two attempts.
const ownerCASMaxAttempts = 64

// ownerCAS runs a logical ownership transition under the store's physical
// compare-and-swap. It reads the current physical generation, then performs
// Store.CAS with mutate. A physical generation mismatch means a concurrent
// writer rotated the record first; ownerCAS re-reads and retries so physical
// contention never surfaces as a logical failure. Any error mutate itself
// returns is a deliberate logical rejection (or a real IO fault) and propagates
// immediately with no retry, so a rejected transition writes nothing. mutate
// runs on the store's own locked read, so its ownership/handoff/fingerprint
// checks see authoritative state.
func (s *Store) ownerCAS(id string, mutate func(*driveRecord) error) error {
	var lastErr error
	for attempt := 0; attempt < ownerCASMaxAttempts; attempt++ {
		dir, err := s.driveDir(id)
		if err != nil {
			return err
		}
		stored, err := s.readStored(dir)
		if err != nil {
			return err
		}
		if _, err := s.CAS(id, stored.Generation, mutate); err != nil {
			if se, ok := AsStoreError(err); ok && se.Kind == ErrGenerationMismatch {
				lastErr = err
				continue // physical contention only: re-read and retry
			}
			return err
		}
		return nil
	}
	return storeErr(ErrIO, "owner-cas", fmt.Errorf("exceeded %d attempts under contention: %w", ownerCASMaxAttempts, lastErr))
}
