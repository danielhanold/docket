// Durable per-phase full-suite attempt budget: one counted reservation ledger
// per (repository, change, phase) owning build scope.
//
// The build gate spends a bounded number of logical full-suite runs per owned
// build phase — an initial run plus a fixed number of repair-and-rerun cycles
// (change 0421). That bound must survive re-observation, recovery, ownership
// takeover, and continuation without ever double-charging or resetting, and no
// repair worker may bypass it. This store makes the count durable and atomic: a
// reservation is committed to disk before any suite launches, and there are no
// refunds, so a lost launch can never overrun the configured bound.
//
// Budget records live beside the drive and scope records under the repository's
// Git common directory, sharing every durability and privacy discipline the
// drive store established (store.go):
//
//	<git-common-dir>/docket/gate-suite-budgets/v1/<id>/record.json
//
// where <id> is the lowercase-hex sha256 of RepoIdentity, ChangeID, and Phase
// joined by NUL — path-safe by construction, mirroring capHash. The directory is
// owner-only (0700) and its record is private (0600). Writes go through
// writeAtomicJSON, and each reservation runs under a per-record blocking flock
// plus a persisted physical-generation compare-and-swap, mirroring scopeCAS so
// concurrent reservations serialize and physical contention never surfaces as a
// logical failure. Unknown schema versions and corrupt records fail closed with
// a typed StoreError, exactly as the drive and scope stores do.
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

const (
	// ErrSuiteBudgetExhausted: a reservation was refused because the phase's
	// snapshotted full-suite attempt budget is spent (used == limit). It is a
	// deliberate logical refusal that changes no state and takes no refund, not a
	// fault — the caller HALTs with the build.max_attempts used/limit.
	ErrSuiteBudgetExhausted StoreErrorKind = "suite-budget-exhausted"
	// ErrInvalidRequest: a reservation asked to create a budget with a limit
	// below the floor of 1. Config validation makes this unreachable in normal
	// operation; the store fails closed anyway rather than persist a budget that
	// could never grant even the initial attempt.
	ErrInvalidRequest StoreErrorKind = "invalid-request"
)

// suiteBudgetSchemaVersion is the persisted suiteBudgetRecord schema generation.
// Like the drive and scope stores, an unknown version fails closed rather than
// being migrated.
const suiteBudgetSchemaVersion = 1

// SuiteBudgetKey identifies one owning build phase's full-suite attempt budget.
// The phase is the literal owning-scope phase (e.g. "build"), not a task-owned
// focused start, so every build-owned rerun of the same phase shares one budget.
type SuiteBudgetKey struct {
	RepoIdentity string
	ChangeID     string
	Phase        string
}

// suiteBudgetRecord is the durable schema of one phase's attempt budget. Limit
// is the value snapshotted when the record was created (the first reservation of
// the phase); every later reservation enforces it and ignores the caller's
// parameter, so a config edit mid-phase cannot rewrite an owned budget. Used is
// the number of logical attempts reserved so far.
type suiteBudgetRecord struct {
	SchemaVersion int    `json:"schema_version"`
	RepoIdentity  string `json:"repo_identity"`
	ChangeID      string `json:"change_id"`
	Phase         string `json:"phase"`
	Limit         int    `json:"limit"`
	Used          int    `json:"used"`
}

// storedSuiteBudget is the on-disk envelope: the store-owned physical generation
// token beside the suiteBudgetRecord it guards, mirroring storedRecord for
// drives and storedScope for scopes.
type storedSuiteBudget struct {
	Generation string            `json:"generation"`
	Record     suiteBudgetRecord `json:"record"`
}

// suiteBudgetID derives the path-safe record id for a key: the lowercase-hex
// sha256 of RepoIdentity, ChangeID, and Phase joined by NUL. NUL is not a legal
// byte in any of the three fields' real values, so distinct keys never collide
// on the joined preimage. It mirrors capHash: the derivation is the single
// boundary at which caller-supplied identity becomes a path-safe token, so no
// key value ever reaches the filesystem verbatim.
func suiteBudgetID(key SuiteBudgetKey) string {
	sum := sha256.Sum256([]byte(key.RepoIdentity + "\x00" + key.ChangeID + "\x00" + key.Phase))
	return hex.EncodeToString(sum[:])
}

// suiteBudgetDir returns the record directory for a key. The id is our own
// sha256 hex token, so — unlike a user-supplied drive/scope id — it needs no
// validation and cannot traverse out of the private root.
func (s *Store) suiteBudgetDir(key SuiteBudgetKey) string {
	return filepath.Join(s.suiteBudgetRoot, suiteBudgetID(key))
}

// peekSuiteBudget reads the current budget envelope in dir without taking a lock.
// It returns the physical generation and record, or the empty generation ("")
// when no record exists yet — a present record always carries a non-empty
// generation, so an empty generation reliably means absent. It fails closed on a
// corrupt document or an unknown schema version.
func (s *Store) peekSuiteBudget(dir string) (gen string, rec suiteBudgetRecord, err error) {
	buf, err := os.ReadFile(filepath.Join(dir, recordFileName))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", suiteBudgetRecord{}, nil // absent: the create path
		}
		return "", suiteBudgetRecord{}, storeErr(ErrIO, "read-suite-budget", err)
	}
	var stored storedSuiteBudget
	if err := json.Unmarshal(buf, &stored); err != nil {
		return "", suiteBudgetRecord{}, storeErr(ErrCorruptRecord, "read-suite-budget", err)
	}
	if stored.Record.SchemaVersion != suiteBudgetSchemaVersion {
		return "", suiteBudgetRecord{}, storeErr(ErrUnknownSchema, "read-suite-budget",
			fmt.Errorf("suite-budget schema version %d, want %d", stored.Record.SchemaVersion, suiteBudgetSchemaVersion))
	}
	return stored.Generation, stored.Record, nil
}

// SuiteBudgetUsage reports (used, limit) for a key without reserving. A key with
// no record yet returns (0, 0, nil). It fails closed on a corrupt or
// unknown-schema record.
func (s *Store) SuiteBudgetUsage(key SuiteBudgetKey) (used, limit int, err error) {
	gen, rec, err := s.peekSuiteBudget(s.suiteBudgetDir(key))
	if err != nil {
		return 0, 0, err
	}
	if gen == "" {
		return 0, 0, nil // no record yet
	}
	return rec.Used, rec.Limit, nil
}

// ReserveSuiteAttempt durably reserves one logical full-suite attempt for key,
// creating the budget record with the snapshotted limit on the first reservation
// of the phase. It returns the 1-based attempt number reserved and the snapshot
// actually in force (which equals the stored limit, never the caller's parameter
// after creation). A spent budget returns ErrSuiteBudgetExhausted with no state
// change and no refund, so a lost launch can never overrun the configured bound.
// A creation with limit below 1 returns ErrInvalidRequest and persists nothing.
//
// It mirrors scopeCAS: an outer unlocked read establishes the expected physical
// generation, then a locked reservation re-reads and proceeds only when the
// generation still matches. A concurrent writer that rotated the record first
// yields ErrGenerationMismatch, which is retried (never surfaced), so racing
// reservations serialize onto distinct attempt numbers up to the bound.
func (s *Store) ReserveSuiteAttempt(key SuiteBudgetKey, limit int) (attempt, snappedLimit int, err error) {
	dir := s.suiteBudgetDir(key)
	var lastErr error
	for i := 0; i < ownerCASMaxAttempts; i++ {
		expectGen, _, err := s.peekSuiteBudget(dir)
		if err != nil {
			return 0, 0, err
		}
		a, snap, err := s.reserveSuiteAttemptOnce(dir, key, limit, expectGen)
		if err != nil {
			if se, ok := AsStoreError(err); ok && se.Kind == ErrGenerationMismatch {
				lastErr = err
				continue // physical contention only: re-read and retry
			}
			return 0, 0, err
		}
		return a, snap, nil
	}
	return 0, 0, storeErr(ErrIO, "reserve-suite-attempt",
		fmt.Errorf("exceeded %d attempts under contention: %w", ownerCASMaxAttempts, lastErr))
}

// reserveSuiteAttemptOnce performs a single flock-serialized reservation. It
// ensures the private directory exists, takes the blocking exclusive lock, and
// re-reads under the lock; a physical generation that no longer equals expectGen
// (including an absent→present transition) returns ErrGenerationMismatch so the
// caller retries. When no record exists it creates one at the snapshotted limit
// (refusing a sub-1 limit); otherwise it enforces the stored limit, refusing with
// ErrSuiteBudgetExhausted when used == limit and writing nothing on that refusal.
func (s *Store) reserveSuiteAttemptOnce(dir string, key SuiteBudgetKey, limit int, expectGen string) (attempt, snappedLimit int, err error) {
	if err := ensurePrivateDir(dir); err != nil {
		return 0, 0, storeErr(ErrIO, "reserve-suite-attempt", err)
	}
	lock, err := acquireExclusiveLock(filepath.Join(dir, lockFileName))
	if err != nil {
		return 0, 0, err
	}
	defer lock.Close()

	gotGen, rec, err := s.peekSuiteBudget(dir)
	if err != nil {
		return 0, 0, err
	}
	if gotGen != expectGen {
		return 0, 0, storeErr(ErrGenerationMismatch, "reserve-suite-attempt", nil)
	}

	if gotGen == "" {
		// First reservation of the phase: snapshot the limit into the record.
		if limit < 1 {
			return 0, 0, storeErr(ErrInvalidRequest, "reserve-suite-attempt", nil)
		}
		rec = suiteBudgetRecord{
			SchemaVersion: suiteBudgetSchemaVersion,
			RepoIdentity:  key.RepoIdentity,
			ChangeID:      key.ChangeID,
			Phase:         key.Phase,
			Limit:         limit,
			Used:          1,
		}
		if err := s.writeSuiteBudget(dir, rec); err != nil {
			return 0, 0, err
		}
		return rec.Used, rec.Limit, nil
	}

	// A later reservation enforces the stored limit and ignores the parameter.
	if rec.Used >= rec.Limit {
		return 0, 0, storeErr(ErrSuiteBudgetExhausted, "reserve-suite-attempt", nil)
	}
	rec.Used++
	rec.SchemaVersion = suiteBudgetSchemaVersion
	if err := s.writeSuiteBudget(dir, rec); err != nil {
		return 0, 0, err
	}
	return rec.Used, rec.Limit, nil
}

// writeSuiteBudget atomically persists rec in dir under a freshly rotated
// physical generation, so a stale concurrent writer's compare-and-swap fails.
func (s *Store) writeSuiteBudget(dir string, rec suiteBudgetRecord) error {
	gen, err := randomToken(genNBytes)
	if err != nil {
		return storeErr(ErrIO, "reserve-suite-attempt", err)
	}
	if err := writeAtomicJSON(filepath.Join(dir, recordFileName), storedSuiteBudget{Generation: gen, Record: rec}); err != nil {
		return storeErr(ErrIO, "reserve-suite-attempt", err)
	}
	return nil
}
