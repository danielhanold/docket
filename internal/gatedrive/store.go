// Durable owner-private drive store with generation compare-and-swap.
//
// A drive's state lives below the repository's Git common directory so it sits
// outside every worktree yet stays reachable from any linked worktree of the
// same repository (spec "Durable drive record → Location and privacy"):
//
//	<git-common-dir>/docket/gate-drives/v1/<opaque-drive-id>/record.json
//
// The directory is owner-only (0700) and its files are private (0600). Writes go
// through a sibling temp file, fsync, atomic rename, and a directory fsync, so a
// concurrent reader sees a complete old or new document, never a partial one —
// the same durability pattern the process layer uses in
// internal/process/atomic.go (writeAtomicJSON, ensurePrivateDir), reimplemented
// here because those helpers are unexported to that package.
//
// Concurrency safety rests on two mechanisms together, as the spec requires ("A
// lock plus a persisted generation provides compare-and-swap semantics"): a
// blocking exclusive flock serializes the read-modify-write of one drive, and a
// persisted opaque generation lets a writer prove it is mutating the state it
// last read. A writer holding a stale generation is refused rather than
// clobbering a concurrent winner.
//
// Unknown schema versions and impossible transitions fail closed with a typed
// StoreError so the caller HALTs; the store never best-effort migrates a record
// it does not recognize. User-supplied ids are validated before any path is
// constructed, and a symlinked drive directory is refused, so neither path
// traversal nor a symlink can escape the private root.
package gatedrive

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
)

const (
	// recordFileName is the atomic drive record within a drive directory.
	recordFileName = "record.json"
	// lockFileName is the advisory-lock file CAS serializes on. It is separate
	// from the record so acquiring the lock never races the record's rename.
	lockFileName = "lock"
	// idNBytes is the entropy of an opaque drive id: 128 bits, encoded as 32
	// lowercase hex characters — enough that an agent cannot collide with or
	// guess a different drive (spec "Location and privacy").
	idNBytes = 16
	// genNBytes is the entropy of an opaque generation token, rotated on every
	// accepted write so a stale writer's compare-and-swap fails.
	genNBytes = 16
)

// StoreErrorKind is the typed category of a StoreError. Callers key on it to
// separate a fail-closed HALT (unknown schema, corrupt record, invalid id) from
// an ordinary lost compare-and-swap (generation mismatch) and from a missing
// drive (not found).
type StoreErrorKind string

const (
	// ErrInvalidID: a user-supplied id failed validation before path
	// construction, or its drive directory is a symlink. Never a real drive.
	ErrInvalidID StoreErrorKind = "invalid-id"
	// ErrNotFound: no drive record exists for a well-formed id.
	ErrNotFound StoreErrorKind = "not-found"
	// ErrUnknownSchema: a persisted record's schema version is not the one this
	// store understands. Fail closed — never a best-effort migration.
	ErrUnknownSchema StoreErrorKind = "unknown-schema"
	// ErrCorruptRecord: a persisted record could not be decoded.
	ErrCorruptRecord StoreErrorKind = "corrupt-record"
	// ErrGenerationMismatch: a CAS presented a generation that is no longer
	// current. The caller lost the race and wrote nothing.
	ErrGenerationMismatch StoreErrorKind = "generation-mismatch"
	// ErrIO: an underlying filesystem or randomness operation failed.
	ErrIO StoreErrorKind = "io"
)

// StoreError is the store's typed failure. It carries a stable kind and stage
// and, where useful, a wrapped cause. It is internal store state, not the
// redaction-sensitive DriveDoc, but it still never embeds record content.
type StoreError struct {
	Kind StoreErrorKind
	Op   string
	err  error
}

func (e *StoreError) Error() string {
	if e.err != nil {
		return fmt.Sprintf("gatedrive store %s: %s: %v", e.Op, e.Kind, e.err)
	}
	return fmt.Sprintf("gatedrive store %s: %s", e.Op, e.Kind)
}

func (e *StoreError) Unwrap() error { return e.err }

func storeErr(kind StoreErrorKind, op string, err error) *StoreError {
	return &StoreError{Kind: kind, Op: op, err: err}
}

// AsStoreError unwraps err to a *StoreError when one is in the chain, so a
// caller can branch on its Kind.
func AsStoreError(err error) (*StoreError, bool) {
	var e *StoreError
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// storedRecord is the on-disk envelope: the store-owned generation token beside
// the driveRecord it guards. Keeping the generation out of driveRecord leaves
// the record's own OwnerGeneration/HandoffGeneration fields entirely to the
// ownership layer (Task 5); the store's compare-and-swap token is a separate,
// lower-level concern.
type storedRecord struct {
	Generation string      `json:"generation"`
	Record     driveRecord `json:"record"`
}

// Store is a durable, owner-private collection of drive records rooted at the
// v1 gate-drives directory under a repository's Git common dir. It also owns a
// sibling gate-scopes root for recovery-scope records (scope.go). It holds no
// mutable state, so one Store is safe for concurrent use across goroutines.
type Store struct {
	root      string
	scopeRoot string
}

// OpenStore returns a Store rooted at <gitCommonDir>/docket/gate-drives/v1 with
// a sibling recovery-scope root at <gitCommonDir>/docket/gate-scopes/v1. It
// creates nothing; directories are minted lazily by NewDrive/PrepareScope so an
// unused store leaves no trace.
func OpenStore(gitCommonDir string) *Store {
	return &Store{
		root:      filepath.Join(gitCommonDir, "docket", "gate-drives", "v1"),
		scopeRoot: filepath.Join(gitCommonDir, "docket", "gate-scopes", "v1"),
	}
}

// NewDrive allocates an opaque high-entropy id and initial generation, stamps
// the current schema version, and atomically persists the record in a fresh
// owner-only directory. It returns the id and the generation a first CAS must
// present.
func (s *Store) NewDrive(rec driveRecord) (id string, gen string, err error) {
	id, err = randomToken(idNBytes)
	if err != nil {
		return "", "", storeErr(ErrIO, "new-drive", err)
	}
	gen, err = randomToken(genNBytes)
	if err != nil {
		return "", "", storeErr(ErrIO, "new-drive", err)
	}
	rec.SchemaVersion = driveSchemaVersion

	dir := filepath.Join(s.root, id) // id is our own hex token: no validation needed
	if err := ensurePrivateDir(dir); err != nil {
		return "", "", storeErr(ErrIO, "new-drive", err)
	}
	if err := writeAtomicJSON(filepath.Join(dir, recordFileName), storedRecord{Generation: gen, Record: rec}); err != nil {
		return "", "", storeErr(ErrIO, "new-drive", err)
	}
	return id, gen, nil
}

// Load reads and returns the current record for id. It validates the id and
// refuses a symlinked drive directory before touching the record, decodes the
// envelope, and fails closed on an unknown schema version. Load takes no lock:
// the atomic rename in every write guarantees a reader observes a whole
// document.
func (s *Store) Load(id string) (driveRecord, error) {
	dir, err := s.driveDir(id)
	if err != nil {
		return driveRecord{}, err
	}
	stored, err := s.readStored(dir)
	if err != nil {
		return driveRecord{}, err
	}
	return stored.Record, nil
}

// CAS performs a compare-and-swap on the record for id. It serializes on the
// drive's flock, reads the current record, and — only when the stored
// generation equals expectGen — applies mutate to a copy and atomically writes
// it back under a freshly rotated generation, which it returns. A stale
// expectGen returns ErrGenerationMismatch and writes nothing; a mutate that
// returns an error aborts the transition and propagates that error, so an
// impossible transition halts rather than half-writing.
func (s *Store) CAS(id string, expectGen string, mutate func(*driveRecord) error) (newGen string, err error) {
	dir, err := s.driveDir(id)
	if err != nil {
		return "", err
	}
	// Existence and non-symlink are proven by driveDir before we create the lock
	// file, so a rejected id never mints a lock either.
	lock, err := acquireExclusiveLock(filepath.Join(dir, lockFileName))
	if err != nil {
		return "", err
	}
	defer lock.Close()

	stored, err := s.readStored(dir)
	if err != nil {
		return "", err
	}
	if stored.Generation != expectGen {
		return "", storeErr(ErrGenerationMismatch, "cas", nil)
	}

	rec := stored.Record // a copy; mutate never touches the persisted bytes
	if err := mutate(&rec); err != nil {
		return "", err
	}
	rec.SchemaVersion = driveSchemaVersion

	newGen, err = randomToken(genNBytes)
	if err != nil {
		return "", storeErr(ErrIO, "cas", err)
	}
	if err := writeAtomicJSON(filepath.Join(dir, recordFileName), storedRecord{Generation: newGen, Record: rec}); err != nil {
		return "", storeErr(ErrIO, "cas", err)
	}
	return newGen, nil
}

// driveDir validates a user-supplied id, then confirms its directory exists and
// is a real directory (not a symlink) before any caller reads or writes inside
// it. Both checks precede every record path, so neither traversal nor a
// symlinked drive dir can escape the private root.
func (s *Store) driveDir(id string) (string, error) {
	if err := validateID(id); err != nil {
		return "", err
	}
	dir := filepath.Join(s.root, id)
	fi, err := os.Lstat(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", storeErr(ErrNotFound, "resolve", err)
		}
		return "", storeErr(ErrIO, "resolve", err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		// A symlinked drive dir is an escape attempt, not a drive; refuse it
		// rather than following it out of the private root.
		return "", storeErr(ErrInvalidID, "resolve", nil)
	}
	return dir, nil
}

// readStored decodes the record envelope in dir and fails closed on a corrupt
// document or an unknown schema version.
func (s *Store) readStored(dir string) (storedRecord, error) {
	buf, err := os.ReadFile(filepath.Join(dir, recordFileName))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return storedRecord{}, storeErr(ErrNotFound, "read", err)
		}
		return storedRecord{}, storeErr(ErrIO, "read", err)
	}
	var stored storedRecord
	if err := json.Unmarshal(buf, &stored); err != nil {
		return storedRecord{}, storeErr(ErrCorruptRecord, "read", err)
	}
	if stored.Record.SchemaVersion != driveSchemaVersion {
		return storedRecord{}, storeErr(ErrUnknownSchema, "read",
			fmt.Errorf("schema version %d, want %d", stored.Record.SchemaVersion, driveSchemaVersion))
	}
	return stored, nil
}

// validateID accepts only a non-empty lowercase-hex token within a sane length
// bound — the exact shape NewDrive mints. Because the safe charset excludes '/',
// '\\', and '.', every path-traversal or absolute-path form (".." , "a/b",
// "/abs", "foo.json") is rejected as a pure string check, before any path is
// constructed. This is the "validated BEFORE path construction" guarantee.
func validateID(id string) error {
	if len(id) < 16 || len(id) > 128 {
		return storeErr(ErrInvalidID, "validate-id", nil)
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return storeErr(ErrInvalidID, "validate-id", nil)
		}
	}
	return nil
}

// randomToken returns nbytes of cryptographic randomness as lowercase hex.
func randomToken(nbytes int) (string, error) {
	buf := make([]byte, nbytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// ensurePrivateDir creates path (and parents) and enforces 0700 with an explicit
// chmod, because a create-time mode is only a request the umask can mask. It
// mirrors internal/process/atomic.go's ensurePrivateDir, reimplemented here
// because that copy is unexported to the process package.
func ensurePrivateDir(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	return os.Chmod(path, 0o700)
}

// writeAtomicJSON writes v as JSON at path via a same-directory temp file,
// fsync, chmod 0600, atomic rename, and directory fsync, so a reader sees a
// complete old or new document, never a partial one. It mirrors
// internal/process/atomic.go's writeAtomicJSON, reimplemented here because that
// copy is unexported to the process package.
func writeAtomicJSON(path string, v any) error {
	buf, err := json.Marshal(v)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful rename
	if _, err := tmp.Write(buf); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

// acquireExclusiveLock opens (creating if needed) path and takes a BLOCKING
// exclusive advisory flock on it, enforcing 0600 with an explicit chmod because
// the create-time mode is umask-masked. Unlike internal/process/lock.go's
// acquireFlock — which takes the lock non-blocking as a liveness probe and
// reports contention — this one blocks so two racing CAS writers serialize and
// the persisted generation, not lock contention, arbitrates the compare-and-swap
// (spec "A lock plus a persisted generation provides compare-and-swap
// semantics"). Closing the returned file releases the lock.
func acquireExclusiveLock(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, storeErr(ErrIO, "lock-open", err)
	}
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return nil, storeErr(ErrIO, "lock-chmod", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, storeErr(ErrIO, "lock", err)
	}
	return f, nil
}
