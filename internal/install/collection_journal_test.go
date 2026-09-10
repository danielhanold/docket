package install

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/testsupport"
)

type collectionFailFS struct {
	RealFS
	rename func(string, string) error
	remove func(string) error
}

func (f collectionFailFS) Rename(old, new string) error {
	if f.rename != nil {
		return f.rename(old, new)
	}
	return f.RealFS.Rename(old, new)
}

func (f collectionFailFS) Remove(path string) error {
	if f.remove != nil {
		return f.remove(path)
	}
	return f.RealFS.Remove(path)
}

func collectionFixture(t *testing.T) (UserRoots, assets.Manifest, collectionJournal) {
	t.Helper()
	home := filepath.Clean(testsupport.TempDir(t))
	roots, err := ResolveRoots(fixedHome(home), fakeEnv(nil))
	if err != nil {
		t.Fatalf("ResolveRoots: %v", err)
	}
	bodies := map[string][]byte{
		"agents/docket-test.md": []byte("agent\n"),
		"skills/test/SKILL.md":  []byte("skill\n"),
	}
	m := assets.Manifest{FormatVersion: assets.ManifestFormatVersion, AssetProtocol: assets.AssetProtocol}
	for _, path := range []string{"agents/docket-test.md", "skills/test/SKILL.md"} {
		body := bodies[path]
		role := assets.RoleAgentSource
		if strings.HasPrefix(path, "skills/") {
			role = assets.RoleSkill
		}
		m.Entries = append(m.Entries, assets.Entry{Path: path, Role: role, Mode: 0o644, Size: int64(len(body)), SHA256: hashBytes(body)})
	}
	m.AssetSetID, err = assets.ComputeAssetSetID(m)
	if err != nil {
		t.Fatalf("ComputeAssetSetID: %v", err)
	}
	if _, _, err := EnsureVersionTree(roots, m, func(path string) ([]byte, error) { return bodies[path], nil }); err != nil {
		t.Fatalf("EnsureVersionTree: %v", err)
	}
	j := collectionJournal{
		FormatVersion:      collectionJournalFormatVersion,
		OriginalAssetSetID: m.AssetSetID,
		Manifest:           m,
		SourcePath:         filepath.Dir(roots.VersionDir(m.AssetSetID)),
		QuarantinePath:     roots.CollectionQuarantineDir(),
		Phase:              collectionPhasePrepared,
	}
	return roots, m, j
}

func TestCollectionJournalStrictCodec(t *testing.T) {
	t.Run("canonical round trip", func(t *testing.T) {
		roots, _, journal := collectionFixture(t)
		if err := writeCollectionJournal(RealFS{}, roots, &journal); err != nil {
			t.Fatalf("writeCollectionJournal: %v", err)
		}
		got, err := loadCollectionJournal(roots)
		if err != nil {
			t.Fatalf("loadCollectionJournal: %v", err)
		}
		if got == nil || got.OriginalAssetSetID != journal.OriginalAssetSetID || got.Phase != journal.Phase {
			t.Fatalf("loaded journal = %#v, want identity %q phase %q", got, journal.OriginalAssetSetID, journal.Phase)
		}
		want, err := encodeCollectionJournal(roots, &journal)
		if err != nil {
			t.Fatalf("encodeCollectionJournal: %v", err)
		}
		raw, err := os.ReadFile(roots.CollectionJournalPath())
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		if !bytes.Equal(raw, want) {
			t.Errorf("journal bytes are not canonical:\n%s\nwant:\n%s", raw, want)
		}
	})

	for _, tc := range []struct {
		name   string
		mutate func(UserRoots, collectionJournal) []byte
	}{
		{"unknown field", func(roots UserRoots, j collectionJournal) []byte {
			b, _ := encodeCollectionJournal(roots, &j)
			return bytes.Replace(b, []byte("\n}"), []byte(",\n  \"surprise\": true\n}"), 1)
		}},
		{"unsupported version", func(_ UserRoots, j collectionJournal) []byte {
			j.FormatVersion = 99
			b, _ := jsonCollectionJournal(j)
			return b
		}},
		{"unknown phase", func(_ UserRoots, j collectionJournal) []byte {
			j.Phase = "paused"
			b, _ := jsonCollectionJournal(j)
			return b
		}},
		{"relative source", func(_ UserRoots, j collectionJournal) []byte {
			j.SourcePath = "versions/tree"
			b, _ := jsonCollectionJournal(j)
			return b
		}},
		{"wrong source", func(roots UserRoots, j collectionJournal) []byte {
			j.SourcePath = filepath.Join(roots.DataRoot, "elsewhere")
			b, _ := jsonCollectionJournal(j)
			return b
		}},
		{"wrong quarantine", func(roots UserRoots, j collectionJournal) []byte {
			j.QuarantinePath = filepath.Join(roots.CollectionDir(), "other")
			b, _ := jsonCollectionJournal(j)
			return b
		}},
		{"identity mismatch", func(_ UserRoots, j collectionJournal) []byte {
			j.OriginalAssetSetID = "sha256:other"
			b, _ := jsonCollectionJournal(j)
			return b
		}},
		{"noncanonical", func(roots UserRoots, j collectionJournal) []byte {
			b, _ := encodeCollectionJournal(roots, &j)
			return bytes.Replace(b, []byte("  \"format_version\""), []byte("    \"format_version\""), 1)
		}},
		{"trailing document", func(roots UserRoots, j collectionJournal) []byte {
			b, _ := encodeCollectionJournal(roots, &j)
			return append(b, []byte("{}\n")...)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			roots, _, journal := collectionFixture(t)
			if err := os.MkdirAll(roots.CollectionDir(), 0o700); err != nil {
				t.Fatalf("MkdirAll: %v", err)
			}
			if err := os.WriteFile(roots.CollectionJournalPath(), tc.mutate(roots, journal), 0o600); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
			if got, err := loadCollectionJournal(roots); err == nil || got != nil {
				t.Fatalf("loadCollectionJournal = %#v, %v; want strict refusal", got, err)
			}
		})
	}
}

// jsonCollectionJournal intentionally bypasses validation so strict-load tests
// can put structurally invalid but syntactically canonical-looking bytes on disk.
func jsonCollectionJournal(j collectionJournal) ([]byte, error) {
	return marshalCollectionJournal(j)
}

func TestCollectionJournalAtomicRenameFailureLeavesPriorRecord(t *testing.T) {
	roots, _, journal := collectionFixture(t)
	if err := writeCollectionJournal(RealFS{}, roots, &journal); err != nil {
		t.Fatalf("initial write: %v", err)
	}
	before, err := os.ReadFile(roots.CollectionJournalPath())
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	journal.Phase = collectionPhaseQuarantined
	boom := errors.New("rename refused")
	fs := collectionFailFS{rename: func(_, _ string) error { return boom }}
	if err := writeCollectionJournal(fs, roots, &journal); !errors.Is(err, boom) {
		t.Fatalf("writeCollectionJournal error = %v, want %v", err, boom)
	}
	after, err := os.ReadFile(roots.CollectionJournalPath())
	if err != nil {
		t.Fatalf("ReadFile after failed publish: %v", err)
	}
	if !bytes.Equal(after, before) {
		t.Errorf("failed publish changed journal:\n%s\nwas:\n%s", after, before)
	}
	entries, err := os.ReadDir(roots.CollectionDir())
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(roots.CollectionJournalPath()) {
		t.Errorf("collection directory after failure = %v, want journal only", entries)
	}
}

func TestReconcileCollectionJournalInterruptionMatrix(t *testing.T) {
	t.Run("interruption before quarantine rename", func(t *testing.T) {
		roots, _, journal := collectionFixture(t)
		if err := writeCollectionJournal(RealFS{}, roots, &journal); err != nil {
			t.Fatalf("write journal: %v", err)
		}
		boom := errors.New("rename interrupted")
		fs := collectionFailFS{rename: func(old, new string) error {
			if old == journal.SourcePath && new == journal.QuarantinePath {
				return boom
			}
			return os.Rename(old, new)
		}}
		if err := reconcileCollectionJournal(fs, roots); !errors.Is(err, ErrCollectionPending) {
			t.Fatalf("reconcile error = %v, want ErrCollectionPending", err)
		}
		assertExists(t, journal.SourcePath)
		assertCollectionAbsent(t, journal.QuarantinePath)
		loaded, err := loadCollectionJournal(roots)
		if err != nil || loaded.Phase != collectionPhasePrepared {
			t.Fatalf("journal after interruption = %#v, %v", loaded, err)
		}
	})

	t.Run("rename response loss resumes", func(t *testing.T) {
		roots, _, journal := collectionFixture(t)
		if err := writeCollectionJournal(RealFS{}, roots, &journal); err != nil {
			t.Fatalf("write journal: %v", err)
		}
		lost := errors.New("rename response lost")
		fs := collectionFailFS{rename: func(old, new string) error {
			if old == journal.SourcePath && new == journal.QuarantinePath {
				if err := os.Rename(old, new); err != nil {
					return err
				}
				return lost
			}
			return os.Rename(old, new)
		}}
		if err := reconcileCollectionJournal(fs, roots); !errors.Is(err, ErrCollectionPending) {
			t.Fatalf("first reconcile error = %v, want ErrCollectionPending", err)
		}
		assertCollectionAbsent(t, journal.SourcePath)
		assertExists(t, journal.QuarantinePath)
		if err := reconcileCollectionJournal(RealFS{}, roots); err != nil {
			t.Fatalf("retry reconcile: %v", err)
		}
		assertCollectionComplete(t, roots)
	})

	t.Run("partial verified entry deletion resumes", func(t *testing.T) {
		roots, _, journal := quarantinedCollectionFixture(t)
		calls := 0
		boom := errors.New("remove interrupted")
		fs := collectionFailFS{remove: func(path string) error {
			if strings.HasSuffix(path, ".md") {
				calls++
				if calls == 2 {
					return boom
				}
			}
			return os.Remove(path)
		}}
		if err := reconcileCollectionJournal(fs, roots); !errors.Is(err, ErrCollectionPending) {
			t.Fatalf("first reconcile error = %v, want ErrCollectionPending", err)
		}
		assertExists(t, roots.CollectionJournalPath())
		if err := reconcileCollectionJournal(RealFS{}, roots); err != nil {
			t.Fatalf("retry reconcile: %v", err)
		}
		assertCollectionComplete(t, roots)
		_ = journal
	})

	t.Run("already absent recorded entry is accepted", func(t *testing.T) {
		roots, manifest, _ := quarantinedCollectionFixture(t)
		missing := filepath.Join(roots.CollectionQuarantineDir(), versionAssetsDir, filepath.FromSlash(manifest.Entries[0].Path))
		if err := os.Remove(missing); err != nil {
			t.Fatalf("Remove fixture entry: %v", err)
		}
		if err := reconcileCollectionJournal(RealFS{}, roots); err != nil {
			t.Fatalf("reconcile: %v", err)
		}
		assertCollectionComplete(t, roots)
	})

	for _, tc := range []struct {
		name  string
		plant func(t *testing.T, roots UserRoots, m assets.Manifest) string
	}{
		{"changed bytes", func(t *testing.T, roots UserRoots, m assets.Manifest) string {
			p := filepath.Join(roots.CollectionQuarantineDir(), versionAssetsDir, filepath.FromSlash(m.Entries[0].Path))
			if err := os.Chmod(p, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte("other\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(p, versionFileMode); err != nil {
				t.Fatal(err)
			}
			return p
		}},
		{"foreign entry", func(t *testing.T, roots UserRoots, _ assets.Manifest) string {
			p := filepath.Join(roots.CollectionQuarantineDir(), versionAssetsDir, "foreign.txt")
			if err := os.WriteFile(p, []byte("foreign\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			return p
		}},
		{"symlink entry", func(t *testing.T, roots UserRoots, m assets.Manifest) string {
			p := filepath.Join(roots.CollectionQuarantineDir(), versionAssetsDir, filepath.FromSlash(m.Entries[0].Path))
			if err := os.Remove(p); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(testsupport.TempDir(t), "foreign.txt")
			if err := os.WriteFile(target, []byte("agent\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, p); err != nil {
				t.Fatal(err)
			}
			return p
		}},
	} {
		t.Run(tc.name+" is retained", func(t *testing.T) {
			roots, manifest, _ := quarantinedCollectionFixture(t)
			foreign := tc.plant(t, roots, manifest)
			ownedSurvivor := filepath.Join(roots.CollectionQuarantineDir(), versionAssetsDir, filepath.FromSlash(manifest.Entries[1].Path))
			before, err := os.ReadFile(foreign)
			if err != nil {
				t.Fatal(err)
			}
			if err := reconcileCollectionJournal(RealFS{}, roots); !errors.Is(err, ErrCollectionPending) {
				t.Fatalf("reconcile error = %v, want ErrCollectionPending", err)
			}
			after, err := os.ReadFile(foreign)
			if err != nil {
				t.Fatalf("foreign content was removed: %v", err)
			}
			if !bytes.Equal(after, before) {
				t.Errorf("foreign content changed")
			}
			assertExists(t, ownedSurvivor)
			assertExists(t, roots.CollectionJournalPath())
		})
	}

	t.Run("journal remains until owned directories are gone", func(t *testing.T) {
		roots, _, _ := quarantinedCollectionFixture(t)
		boom := errors.New("directory remove interrupted")
		failed := false
		fs := collectionFailFS{remove: func(path string) error {
			if path == filepath.Join(roots.CollectionQuarantineDir(), versionAssetsDir) && !failed {
				failed = true
				return boom
			}
			return os.Remove(path)
		}}
		if err := reconcileCollectionJournal(fs, roots); !errors.Is(err, ErrCollectionPending) {
			t.Fatalf("first reconcile error = %v, want ErrCollectionPending", err)
		}
		assertExists(t, roots.CollectionJournalPath())
		assertExists(t, filepath.Join(roots.CollectionQuarantineDir(), versionAssetsDir))
		if err := reconcileCollectionJournal(RealFS{}, roots); err != nil {
			t.Fatalf("retry reconcile: %v", err)
		}
		assertCollectionComplete(t, roots)
	})

	t.Run("unexpected source and quarantine combination is retained", func(t *testing.T) {
		roots, _, journal := collectionFixture(t)
		if err := writeCollectionJournal(RealFS{}, roots, &journal); err != nil {
			t.Fatalf("write journal: %v", err)
		}
		if err := os.MkdirAll(journal.QuarantinePath, versionDirMode); err != nil {
			t.Fatalf("MkdirAll quarantine: %v", err)
		}
		if err := reconcileCollectionJournal(RealFS{}, roots); !errors.Is(err, ErrCollectionPending) {
			t.Fatalf("reconcile error = %v, want ErrCollectionPending", err)
		}
		assertExists(t, journal.SourcePath)
		assertExists(t, journal.QuarantinePath)
		assertExists(t, roots.CollectionJournalPath())
	})
}

func quarantinedCollectionFixture(t *testing.T) (UserRoots, assets.Manifest, collectionJournal) {
	t.Helper()
	roots, manifest, journal := collectionFixture(t)
	if err := os.MkdirAll(roots.CollectionDir(), 0o700); err != nil {
		t.Fatalf("MkdirAll collection: %v", err)
	}
	if err := os.Rename(journal.SourcePath, journal.QuarantinePath); err != nil {
		t.Fatalf("quarantine fixture: %v", err)
	}
	journal.Phase = collectionPhaseQuarantined
	if err := writeCollectionJournal(RealFS{}, roots, &journal); err != nil {
		t.Fatalf("write journal: %v", err)
	}
	return roots, manifest, journal
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}

func assertCollectionAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected %s to be absent, Lstat error = %v", path, err)
	}
}

func assertCollectionComplete(t *testing.T, roots UserRoots) {
	t.Helper()
	assertCollectionAbsent(t, roots.CollectionJournalPath())
	assertCollectionAbsent(t, roots.CollectionQuarantineDir())
}
