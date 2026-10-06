// Durable outer recovery scopes: one per run.
//
// A recovery scope is the durable record that lets the run tracker recover a
// still-running suite gate implement-next was dispatched under, after
// implement-next returns without handing it off (takeover.go). run.start prepares
// exactly one outer scope per run (PrepareScope), run.verdict binds its change id
// once attribution resolves (bindScopeChange) and takes over the run's live drive
// through it (Takeover), and run.cancel reads its recorded worktree (LoadScope).
// Takeover closes it once (claimScopeForTakeover); there is no per-test lifecycle.
// Each scope carries two SEPARATELY-minted opaque capabilities — a child
// capability that becomes the run context handed to implement-next, and a parent
// capability retained by the run tracker and never exposed to the child — so a
// takeover is authorized by the parent capability alone, distinct from the
// child's own authority.
//
// Scope records live beside the drive records under the repository's Git common
// directory, sharing every durability and privacy discipline the drive store
// established (store.go):
//
//	<per-repo state folder>/gate-scopes/v2/<opaque-scope-id>/record.json
//
// The directory is owner-only (0700) and its record is private (0600). Writes go
// through the same writeAtomicJSON helper, and every mutating transition runs
// under a per-scope blocking flock and a persisted physical-generation
// conflict-checked write (scopeCAS), mirroring Store.CAS/ownerCAS so physical
// contention never surfaces as a logical failure. Capabilities are persisted only
// as sha256 hashes (capHash) — never the raw tokens — so a leaked record file
// discloses no authority. Unknown schema versions and corrupt records fail closed
// with a typed StoreError, exactly as the drive store does.
//
// Records a pre-0489 binary left on disk (change 0489): an outer scope's on-disk
// shape is unchanged (the kept fields are a subset of what it wrote, and the
// dropped ones are ignored on decode), so a run in flight across the upgrade keeps
// its continuation path. A task scope is never opened again: LoadScope is only ever
// called with a scope id from a run-tracker record, and nothing lists the scope
// directory.
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
// drive store, an unknown version fails closed rather than being migrated. Change
// 0405 bumped it to 2 and change 0375 to 3; change 0489 dropped the task and slot
// fields without a bump (the kept fields are a subset of v3) and stopped
// tolerating v2, which the gate-scopes/v2 root created by change 0471's reset has
// never held.
const scopeSchemaVersion = 3

// scopeRecord is the durable, owner-private schema of one outer recovery scope.
// It stores only hashes of the two capabilities — never their raw values — plus
// the dispatch identity the takeover path re-verifies. Every field carries an
// explicit snake_case json tag so the store round-trips it canonically.
type scopeRecord struct {
	SchemaVersion int    `json:"schema_version"`
	RepoIdentity  string `json:"repo_identity"`
	ChangeID      string `json:"change_id"`
	Branch        string `json:"branch"`
	Worktree      string `json:"worktree"`
	ChildCapHash  string `json:"child_cap_hash"`
	ParentCapHash string `json:"parent_cap_hash"`
	Closed        bool   `json:"closed"`
}

// storedScope is the on-disk envelope: the store-owned physical generation token
// beside the scopeRecord it guards, mirroring storedRecord for drives.
type storedScope struct {
	Generation string      `json:"generation"`
	Record     scopeRecord `json:"record"`
}

// ScopeRequest identifies the run whose outer recovery scope run.start prepares:
// the repository, the feature branch and worktree, and the change id when it is
// already known (a resume pre-binds it; a fresh run binds it once later).
type ScopeRequest struct {
	RepoIdentity string
	ChangeID     string // may be "" for a fresh outer scope; binds once later
	Branch       string
	Worktree     string
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
// persists the record carrying only their hashes, and returns the grant. It
// creates a fresh owner-only directory and atomically writes the record — the
// same discipline as NewDrive.
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
		Branch:        req.Branch,
		Worktree:      req.Worktree,
		ChildCapHash:  capHash(childCap),
		ParentCapHash: capHash(parentCap),
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
	// Only v3 is read; every other version — v1's retired bound_drive_id shape and
	// v2 included — fails closed, never migrated.
	if stored.Record.SchemaVersion != scopeSchemaVersion {
		return storedScope{}, storeErr(ErrUnknownSchema, "read-scope",
			fmt.Errorf("scope schema version %d, want %d", stored.Record.SchemaVersion, scopeSchemaVersion))
	}
	return stored, nil
}

// scopeCASOnce performs a single flock-serialized conflict-checked write on a scope
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

// scopeCAS runs a logical scope transition under the physical conflict-checked write,
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
