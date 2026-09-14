package gatedrive

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// copyLegacyFixture installs testdata/legacy-v2/<name>.json as drive <id>'s
// record in store s (creating the 0700 dir), returning the id.
func copyLegacyFixture(t *testing.T, s *Store, name string) string {
	t.Helper()
	id := "0428aaaaaaaaaaaaaaaaaaaaaaaaaa" + map[string]string{
		"passed": "01", "failed": "02", "halted": "03", "waiting": "04",
		"schema1": "05", "missing-worktree": "06", "corrupt": "07"}[name]
	dir := filepath.Join(s.root, id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	buf, err := os.ReadFile(filepath.Join("testdata", "legacy-v2", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, recordFileName), buf, 0o600); err != nil {
		t.Fatal(err)
	}
	return id
}

// rewriteRecordField reads drive id's record.json, replaces exactly one
// occurrence of old with new, and writes it back.
func rewriteRecordField(t *testing.T, s *Store, id, old, newv string) {
	t.Helper()
	p := filepath.Join(s.root, id, recordFileName)
	buf, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(buf), old); n != 1 {
		t.Fatalf("want exactly one occurrence of %q, got %d", old, n)
	}
	out := strings.Replace(string(buf), old, newv, 1)
	if err := os.WriteFile(p, []byte(out), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadHistoricalDriveReadsSchema2(t *testing.T) {
	s := OpenStore(t.TempDir())
	id := copyLegacyFixture(t, s, "passed")
	h, err := s.loadHistoricalDrive(id)
	if err != nil {
		t.Fatalf("schema-2 must load for assessment: %v", err)
	}
	if h.SchemaVersion != 2 || h.LastOutcome != PASSED || h.WorktreePath == "" || h.RepoIdentity == "" {
		t.Fatalf("bad view: %+v", h)
	}
	// The EXECUTABLE reader still refuses it — no new compatibility granted.
	if _, err := s.Load(id); !isStoreKind(err, ErrUnknownSchema) {
		t.Fatalf("execution reader must still refuse v2, got %v", err)
	}
}

func TestLoadHistoricalDriveFailsClosed(t *testing.T) {
	s := OpenStore(t.TempDir())
	for name, kind := range map[string]StoreErrorKind{
		"schema1": ErrUnknownSchema,
		"corrupt": ErrCorruptRecord,
	} {
		id := copyLegacyFixture(t, s, name)
		if _, err := s.loadHistoricalDrive(id); !isStoreKind(err, kind) {
			t.Fatalf("%s: want %s, got %v", name, kind, err)
		}
	}
}

// A v2 document with a required identity/outcome field missing must be refused
// as corrupt — never zero-value-decoded into a trustworthy record.
func TestLoadHistoricalDriveValidatesRequiredFields(t *testing.T) {
	s := OpenStore(t.TempDir())
	id := copyLegacyFixture(t, s, "passed")
	rewriteRecordField(t, s, id, `"worktree_path": "/repo/.worktrees/old-feature"`, `"worktree_path": ""`)
	if _, err := s.loadHistoricalDrive(id); !isStoreKind(err, ErrCorruptRecord) {
		t.Fatalf("empty worktree_path must fail closed, got %v", err)
	}
	id2 := copyLegacyFixture(t, s, "halted")
	rewriteRecordField(t, s, id2, `"last_outcome": "HALTED"`, `"last_outcome": "EXPLODED"`)
	if _, err := s.loadHistoricalDrive(id2); !isStoreKind(err, ErrCorruptRecord) {
		t.Fatalf("unknown outcome vocabulary must fail closed, got %v", err)
	}
}
