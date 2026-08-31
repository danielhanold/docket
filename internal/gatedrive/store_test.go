package gatedrive

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// sampleRecord builds a driveRecord carrying at least one value in every field
// group so a round-trip proves the store persists the whole schema, not a subset.
func sampleRecord() driveRecord {
	return driveRecord{
		RepoIdentity:     "repo-x",
		WorktreePath:     "/wt/x",
		ChangeID:         "0342",
		TaskID:           "task-4",
		Phase:            "build",
		Branch:           "feat/x",
		Ref:              "refs/heads/feat/x",
		HeadOID:          "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		Fingerprint:      Fingerprint{Head: "deadbeef", Index: "i", Status: "s", Worktree: "w", Entries: 3},
		Command:          []string{"go test ./..."},
		Cwd:              "/wt/x",
		ConfigProvenance: "config:finalize.test_command",
		Budget:           30 * time.Minute,
		EnvHash:          "envhash",
		StartedAt:        time.Unix(1000, 0).UTC(),
		UpdatedAt:        time.Unix(1000, 0).UTC(),
		Deadline:         time.Unix(2800, 0).UTC(),
		LastClock:        time.Unix(1000, 0).UTC(),
		ProtocolVersion:  ProtocolVersion,
		RawRunDir:        "/runs/abc",
		RawOwnership:     "own-1",
		Attempt:          1,
		RelaunchCount:    0,
		OwnerGeneration:  "owner-g0",
	}
}

// TestNewDriveRoundTrip persists a record and loads it back unchanged: the store
// is a faithful, lossless round trip and stamps the current schema version.
func TestNewDriveRoundTrip(t *testing.T) {
	s := OpenStore(t.TempDir())
	rec := sampleRecord()
	id, gen, err := s.NewDrive(rec)
	if err != nil {
		t.Fatalf("NewDrive: %v", err)
	}
	if id == "" || gen == "" {
		t.Fatalf("NewDrive must return a non-empty id and generation, got id=%q gen=%q", id, gen)
	}

	got, err := s.Load(id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := rec
	want.SchemaVersion = driveSchemaVersion // the store stamps it
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", got, want)
	}
}

// TestNewDriveIDsAreHighEntropyAndDistinct proves ids are opaque high-entropy
// tokens, not a predictable sequence, and never collide across drives.
func TestNewDriveIDsAreHighEntropyAndDistinct(t *testing.T) {
	s := OpenStore(t.TempDir())
	seen := map[string]bool{}
	for i := 0; i < 64; i++ {
		id, _, err := s.NewDrive(sampleRecord())
		if err != nil {
			t.Fatalf("NewDrive: %v", err)
		}
		if len(id) < 32 {
			t.Fatalf("id %q is too short to be high-entropy", id)
		}
		if seen[id] {
			t.Fatalf("id %q collided", id)
		}
		seen[id] = true
	}
}

// TestCASAdvancesGenerationOnMatch proves CAS applies the mutation and rotates
// the generation only when the caller presents the current generation.
func TestCASAdvancesGenerationOnMatch(t *testing.T) {
	s := OpenStore(t.TempDir())
	id, g0, err := s.NewDrive(sampleRecord())
	if err != nil {
		t.Fatalf("NewDrive: %v", err)
	}

	g1, err := s.CAS(id, g0, func(r *driveRecord) error {
		r.Attempt = 5
		return nil
	})
	if err != nil {
		t.Fatalf("CAS with current generation: %v", err)
	}
	if g1 == g0 {
		t.Fatalf("CAS must advance the generation, got same %q", g1)
	}

	got, err := s.Load(id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Attempt != 5 {
		t.Fatalf("mutation not persisted: Attempt=%d", got.Attempt)
	}

	// The chain continues from the new generation.
	if _, err := s.CAS(id, g1, func(r *driveRecord) error { return nil }); err != nil {
		t.Fatalf("CAS with advanced generation: %v", err)
	}
}

// TestCASRejectsStaleGeneration proves a writer holding an outdated generation
// is refused with the typed generation-mismatch error and changes nothing.
func TestCASRejectsStaleGeneration(t *testing.T) {
	s := OpenStore(t.TempDir())
	id, g0, err := s.NewDrive(sampleRecord())
	if err != nil {
		t.Fatalf("NewDrive: %v", err)
	}
	g1, err := s.CAS(id, g0, func(r *driveRecord) error { r.Attempt = 2; return nil })
	if err != nil {
		t.Fatalf("first CAS: %v", err)
	}

	_, err = s.CAS(id, g0, func(r *driveRecord) error { r.Attempt = 99; return nil })
	se, ok := AsStoreError(err)
	if !ok || se.Kind != ErrGenerationMismatch {
		t.Fatalf("stale CAS must return ErrGenerationMismatch, got %v", err)
	}
	got, err := s.Load(id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Attempt != 2 {
		t.Fatalf("rejected CAS must not mutate: Attempt=%d", got.Attempt)
	}
	// The current generation still writes.
	if _, err := s.CAS(id, g1, func(r *driveRecord) error { return nil }); err != nil {
		t.Fatalf("current-generation CAS after a rejected one: %v", err)
	}
}

// TestCASMutateErrorAborts proves a mutate that returns an error aborts the
// transition (an impossible transition halts, never a best-effort write): the
// record and the generation are both unchanged and the error propagates.
func TestCASMutateErrorAborts(t *testing.T) {
	s := OpenStore(t.TempDir())
	id, g0, err := s.NewDrive(sampleRecord())
	if err != nil {
		t.Fatalf("NewDrive: %v", err)
	}
	sentinel := errors.New("impossible transition")
	_, err = s.CAS(id, g0, func(r *driveRecord) error {
		r.Attempt = 42
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("CAS must propagate the mutate error, got %v", err)
	}
	got, err := s.Load(id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Attempt != 1 {
		t.Fatalf("aborted CAS must not persist: Attempt=%d", got.Attempt)
	}
	// The generation did not rotate: g0 still writes.
	if _, err := s.CAS(id, g0, func(r *driveRecord) error { return nil }); err != nil {
		t.Fatalf("generation must be unchanged after an aborted CAS: %v", err)
	}
}

// TestConcurrentCASSingleWinner races two writers off the same starting
// generation and proves exactly one wins and the mutation applies exactly once.
// Run under -race; the store shares no mutable Go state, and the flock plus the
// persisted generation serialize the read-modify-write on disk.
func TestConcurrentCASSingleWinner(t *testing.T) {
	s := OpenStore(t.TempDir())
	id, g0, err := s.NewDrive(sampleRecord())
	if err != nil {
		t.Fatalf("NewDrive: %v", err)
	}

	var wg sync.WaitGroup
	results := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, e := s.CAS(id, g0, func(r *driveRecord) error {
				r.Attempt++
				return nil
			})
			results[idx] = e
		}(i)
	}
	wg.Wait()

	winners := 0
	for _, e := range results {
		if e == nil {
			winners++
			continue
		}
		if se, ok := AsStoreError(e); !ok || se.Kind != ErrGenerationMismatch {
			t.Fatalf("loser must fail with ErrGenerationMismatch, got %v", e)
		}
	}
	if winners != 1 {
		t.Fatalf("exactly one writer must win, got %d", winners)
	}
	got, err := s.Load(id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Attempt != 2 { // sample starts at 1; exactly one increment applied
		t.Fatalf("mutation must apply exactly once: Attempt=%d", got.Attempt)
	}
}

// TestLoadUnknownSchemaVersionErrors proves a persisted record whose schema
// version the store does not recognize fails closed with the typed unknown-schema
// error rather than being best-effort migrated.
func TestLoadUnknownSchemaVersionErrors(t *testing.T) {
	s := OpenStore(t.TempDir())
	id, _, err := s.NewDrive(sampleRecord())
	if err != nil {
		t.Fatalf("NewDrive: %v", err)
	}
	// Overwrite the persisted record with a future/unknown schema version.
	rec := sampleRecord()
	rec.SchemaVersion = driveSchemaVersion + 999
	buf, err := json.Marshal(storedRecord{Generation: "x", Record: rec})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	path := filepath.Join(s.root, id, recordFileName)
	if err := os.WriteFile(path, buf, 0o600); err != nil {
		t.Fatalf("overwrite: %v", err)
	}

	_, err = s.Load(id)
	se, ok := AsStoreError(err)
	if !ok || se.Kind != ErrUnknownSchema {
		t.Fatalf("unknown schema must return ErrUnknownSchema, got %v", err)
	}
	// A CAS over the same record is equally refused — never a silent migration.
	_, err = s.CAS(id, "x", func(r *driveRecord) error { return nil })
	if se, ok := AsStoreError(err); !ok || se.Kind != ErrUnknownSchema {
		t.Fatalf("unknown schema must refuse CAS, got %v", err)
	}
}

// TestTraversalIDRejected proves a user-supplied id carrying path traversal or
// separators is rejected by validation before any path is constructed or touched.
func TestTraversalIDRejected(t *testing.T) {
	s := OpenStore(t.TempDir())
	for _, bad := range []string{
		"../../etc/passwd",
		"..",
		".",
		"a/b",
		`a\b`,
		"",
		"foo.json",
		"ABCDEF0123456789ABCDEF0123456789", // uppercase is outside the safe charset
		"/abs/path",
	} {
		if _, err := s.Load(bad); !isInvalidID(err) {
			t.Fatalf("Load(%q) must reject with ErrInvalidID, got %v", bad, err)
		}
		if _, err := s.CAS(bad, "g", func(r *driveRecord) error { return nil }); !isInvalidID(err) {
			t.Fatalf("CAS(%q) must reject with ErrInvalidID, got %v", bad, err)
		}
	}
	// The store root must hold no stray file or directory created by a rejected id.
	entries, err := os.ReadDir(s.root)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("ReadDir root: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("a rejected id must touch no path; root has %d entries", len(entries))
	}
}

// TestSymlinkDriveDirRejected proves a symlinked drive directory cannot be used
// to escape the private root: the store refuses it rather than following it to
// the decoy record it points at.
func TestSymlinkDriveDirRejected(t *testing.T) {
	s := OpenStore(t.TempDir())
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	// A decoy drive dir OUTSIDE the root with a readable record. If the guard
	// failed, Load would succeed against this instead of refusing.
	decoy := t.TempDir()
	buf, err := json.Marshal(storedRecord{Generation: "g", Record: sampleRecord()})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(decoy, recordFileName), buf, 0o600); err != nil {
		t.Fatalf("write decoy record: %v", err)
	}
	id := strings.Repeat("a", 32) // a valid-shaped id whose dir we make a symlink
	if err := os.Symlink(decoy, filepath.Join(s.root, id)); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	if _, err := s.Load(id); !isInvalidID(err) {
		t.Fatalf("Load of a symlinked drive dir must reject with ErrInvalidID, got %v", err)
	}
	if _, err := s.CAS(id, "g", func(r *driveRecord) error { return nil }); !isInvalidID(err) {
		t.Fatalf("CAS of a symlinked drive dir must reject with ErrInvalidID, got %v", err)
	}
}

// TestOwnerPrivatePermissions proves the drive directory is 0700 and its record
// and lock files are 0600 — owner-only, so the private runtime state cannot be
// read by another user on a shared host.
func TestOwnerPrivatePermissions(t *testing.T) {
	s := OpenStore(t.TempDir())
	id, g0, err := s.NewDrive(sampleRecord())
	if err != nil {
		t.Fatalf("NewDrive: %v", err)
	}
	dir := filepath.Join(s.root, id)
	if fi, err := os.Stat(dir); err != nil {
		t.Fatalf("stat dir: %v", err)
	} else if fi.Mode().Perm() != 0o700 {
		t.Fatalf("drive dir perms = %o, want 0700", fi.Mode().Perm())
	}
	if fi, err := os.Stat(filepath.Join(dir, recordFileName)); err != nil {
		t.Fatalf("stat record: %v", err)
	} else if fi.Mode().Perm() != 0o600 {
		t.Fatalf("record perms = %o, want 0600", fi.Mode().Perm())
	}
	// The lock file is created by CAS; drive it once, then check its perms.
	if _, err := s.CAS(id, g0, func(r *driveRecord) error { return nil }); err != nil {
		t.Fatalf("CAS: %v", err)
	}
	if fi, err := os.Stat(filepath.Join(dir, lockFileName)); err != nil {
		t.Fatalf("stat lock: %v", err)
	} else if fi.Mode().Perm() != 0o600 {
		t.Fatalf("lock perms = %o, want 0600", fi.Mode().Perm())
	}
}

// TestLoadMissingDriveErrors proves an unknown id is a typed not-found, distinct
// from an invalid id or a corrupt record.
func TestLoadMissingDriveErrors(t *testing.T) {
	s := OpenStore(t.TempDir())
	_, err := s.Load(strings.Repeat("b", 32))
	if se, ok := AsStoreError(err); !ok || se.Kind != ErrNotFound {
		t.Fatalf("missing drive must return ErrNotFound, got %v", err)
	}
}

// isInvalidID reports whether err is the typed invalid-id store error.
func isInvalidID(err error) bool {
	se, ok := AsStoreError(err)
	return ok && se.Kind == ErrInvalidID
}
