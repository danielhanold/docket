package install

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/assets"
)

// ---------------------------------------------------------------------------
// Bundle fixture
// ---------------------------------------------------------------------------

// samplePayload is the bundle every test in this file extracts: a nested skill
// tree and a flat agent source, so both a created intermediate directory and a
// top-level file are exercised.
func samplePayload() map[string][]byte {
	return map[string][]byte{
		"agents/docket-build-standard.md":   []byte("---\nname: docket-build-standard\n---\nbody\n"),
		"skills/docket-build/SKILL.md":      []byte("skill body\n"),
		"skills/docket-build/refs/notes.md": []byte("notes\n"),
	}
}

func sampleRole(p string) assets.Role {
	if strings.HasPrefix(p, "agents/") {
		return assets.RoleAgentSource
	}
	return assets.RoleSkill
}

// sampleManifest builds a valid manifest over payload, digest and all, so a
// test never hand-maintains a hash.
func sampleManifest(t *testing.T, payload map[string][]byte) assets.Manifest {
	t.Helper()

	paths := make([]string, 0, len(payload))
	for p := range payload {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	m := assets.Manifest{
		FormatVersion: assets.ManifestFormatVersion,
		AssetProtocol: assets.AssetProtocol,
	}
	for _, p := range paths {
		body := payload[p]
		m.Entries = append(m.Entries, assets.Entry{
			Path:   p,
			Role:   sampleRole(p),
			Mode:   0o644,
			Size:   int64(len(body)),
			SHA256: sha256Hex(body),
		})
	}
	id, err := assets.ComputeAssetSetID(m)
	if err != nil {
		t.Fatalf("ComputeAssetSetID: %v", err)
	}
	m.AssetSetID = id
	if err := assets.ValidateManifest(m); err != nil {
		t.Fatalf("the fixture manifest is invalid: %v", err)
	}
	return m
}

func sha256Hex(b []byte) string { return hashBytes(b) }

// openFrom serves payload bytes and refuses anything else, which is how a test
// injects an extraction failure at a chosen entry.
func openFrom(payload map[string][]byte) func(string) ([]byte, error) {
	return func(p string) ([]byte, error) {
		body, ok := payload[p]
		if !ok {
			return nil, fmt.Errorf("no payload for %q", p)
		}
		return append([]byte(nil), body...), nil
	}
}

// poisonedOpen fails on every call. Passing it to a call that must reuse an
// existing tree is the proof that reuse never re-reads the bundle.
func poisonedOpen(t *testing.T) func(string) ([]byte, error) {
	t.Helper()
	return func(p string) ([]byte, error) {
		return nil, fmt.Errorf("open must not be called, but was, for %q", p)
	}
}

// versionRoots builds roots under a temp home and arranges for the read-only
// version tree to be made writable again before the temp dir is removed —
// otherwise the extraction's own 0o555 directories defeat the test cleanup.
func versionRoots(t *testing.T) UserRoots {
	t.Helper()
	home := t.TempDir()
	roots, err := ResolveRoots(
		func() (string, error) { return home, nil },
		func(string) string { return "" },
	)
	if err != nil {
		t.Fatalf("ResolveRoots: %v", err)
	}
	t.Cleanup(func() { makeWritable(roots.DataRoot) })
	return roots
}

func makeWritable(root string) {
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			_ = os.Chmod(p, 0o700)
		} else {
			_ = os.Chmod(p, 0o600)
		}
		return nil
	})
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestEnsureVersionTreeExtractsAndVerifies(t *testing.T) {
	roots := versionRoots(t)
	payload := samplePayload()
	m := sampleManifest(t, payload)

	dir, reused, err := EnsureVersionTree(roots, m, openFrom(payload))
	if err != nil {
		t.Fatalf("EnsureVersionTree: %v", err)
	}
	if reused {
		t.Fatal("a fresh data root reported a reused version tree")
	}
	if want := roots.VersionDir(m.AssetSetID); dir != want {
		t.Fatalf("extracted to %s, want %s", dir, want)
	}

	for p, want := range payload {
		full := filepath.Join(dir, filepath.FromSlash(p))
		got, err := os.ReadFile(full)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", full, err)
		}
		if string(got) != string(want) {
			t.Errorf("%s holds %q, want %q", p, got, want)
		}
		info, err := os.Lstat(full)
		if err != nil {
			t.Fatalf("Lstat(%s): %v", full, err)
		}
		if perm := info.Mode().Perm(); perm != 0o444 {
			t.Errorf("%s has mode %o, want 444 — the version tree is immutable", p, perm)
		}
	}

	// Every directory the extraction created, including the one holding a
	// nested skill file, must be read-only too.
	for _, d := range []string{dir, filepath.Join(dir, "skills", "docket-build", "refs")} {
		info, err := os.Lstat(d)
		if err != nil {
			t.Fatalf("Lstat(%s): %v", d, err)
		}
		if perm := info.Mode().Perm(); perm != 0o555 {
			t.Errorf("directory %s has mode %o, want 555", d, perm)
		}
	}

	// A second call over the same bundle reuses what is already there. The
	// poisoned open proves the bundle is never re-read: reuse is decided by
	// verifying the tree on disk against the manifest, not by trusting its name.
	reusedDir, reused, err := EnsureVersionTree(roots, m, poisonedOpen(t))
	if err != nil {
		t.Fatalf("second EnsureVersionTree: %v", err)
	}
	if !reused {
		t.Error("an identical existing tree was re-extracted instead of reused")
	}
	if reusedDir != dir {
		t.Errorf("reuse returned %s, want %s", reusedDir, dir)
	}
}

func TestEnsureVersionTreeRejectsPartial(t *testing.T) {
	roots := versionRoots(t)
	payload := samplePayload()
	m := sampleManifest(t, payload)

	dir, _, err := EnsureVersionTree(roots, m, openFrom(payload))
	if err != nil {
		t.Fatalf("EnsureVersionTree: %v", err)
	}
	removeFromTree(t, dir, "skills/docket-build/SKILL.md")

	_, _, err = EnsureVersionTree(roots, m, openFrom(payload))
	if !errors.Is(err, ErrVersionTreeInvalid) {
		t.Fatalf("a tree missing a file returned %v, want ErrVersionTreeInvalid", err)
	}
	// Never silently replaced: the incomplete tree is still exactly as found.
	if _, err := os.Lstat(filepath.Join(dir, "skills", "docket-build", "SKILL.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the rejected tree was rewritten: Lstat returned %v", err)
	}
}

func TestEnsureVersionTreeRejectsMutated(t *testing.T) {
	roots := versionRoots(t)
	payload := samplePayload()
	m := sampleManifest(t, payload)

	dir, _, err := EnsureVersionTree(roots, m, openFrom(payload))
	if err != nil {
		t.Fatalf("EnsureVersionTree: %v", err)
	}
	writeIntoTree(t, dir, "skills/docket-build/SKILL.md", "skill body!\n")

	if _, _, err := EnsureVersionTree(roots, m, openFrom(payload)); !errors.Is(err, ErrVersionTreeInvalid) {
		t.Fatalf("a mutated tree returned %v, want ErrVersionTreeInvalid", err)
	}
}

func TestEnsureVersionTreeRejectsExtraFile(t *testing.T) {
	roots := versionRoots(t)
	payload := samplePayload()
	m := sampleManifest(t, payload)

	dir, _, err := EnsureVersionTree(roots, m, openFrom(payload))
	if err != nil {
		t.Fatalf("EnsureVersionTree: %v", err)
	}
	writeIntoTree(t, dir, "skills/docket-build/EXTRA.md", "not in the bundle\n")

	if _, _, err := EnsureVersionTree(roots, m, openFrom(payload)); !errors.Is(err, ErrVersionTreeInvalid) {
		t.Fatalf("a tree carrying an unmanifested file returned %v, want ErrVersionTreeInvalid", err)
	}
}

func TestStagingNeverVisible(t *testing.T) {
	roots := versionRoots(t)
	payload := samplePayload()
	m := sampleManifest(t, payload)

	// Drop one entry's bytes so the extraction fails partway through, after at
	// least one file has already been staged.
	partial := samplePayload()
	delete(partial, "skills/docket-build/refs/notes.md")

	if _, _, err := EnsureVersionTree(roots, m, openFrom(partial)); err == nil {
		t.Fatal("an extraction missing a payload succeeded")
	}

	if _, err := os.Lstat(filepath.Dir(roots.VersionDir(m.AssetSetID))); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a failed extraction published a version directory: Lstat returned %v", err)
	}
	entries, err := os.ReadDir(roots.VersionsDir())
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", roots.VersionsDir(), err)
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("a failed extraction left %v behind under versions/", names)
	}
}

func TestEnsureVersionTreeRejectsInvalidManifest(t *testing.T) {
	roots := versionRoots(t)
	payload := samplePayload()
	m := sampleManifest(t, payload)
	m.Entries[0].SHA256 = "not-a-digest"

	if _, _, err := EnsureVersionTree(roots, m, openFrom(payload)); !errors.Is(err, assets.ErrManifestInvalid) {
		t.Fatalf("an invalid manifest returned %v, want ErrManifestInvalid", err)
	}
	if _, err := os.Lstat(roots.VersionsDir()); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a refused manifest created %s: Lstat returned %v", roots.VersionsDir(), err)
	}
}

// ---------------------------------------------------------------------------
// Tree mutation helpers — the version tree is read-only by design, so a test
// that tampers with it has to open the permissions itself.
// ---------------------------------------------------------------------------

func writeIntoTree(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	parent := filepath.Dir(full)
	if err := os.Chmod(parent, 0o700); err != nil {
		t.Fatalf("Chmod(%s): %v", parent, err)
	}
	_ = os.Remove(full)
	if err := os.WriteFile(full, []byte(content), 0o444); err != nil {
		t.Fatalf("WriteFile(%s): %v", full, err)
	}
	if err := os.Chmod(parent, 0o555); err != nil {
		t.Fatalf("Chmod(%s): %v", parent, err)
	}
}

func removeFromTree(t *testing.T, dir, rel string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	parent := filepath.Dir(full)
	if err := os.Chmod(parent, 0o700); err != nil {
		t.Fatalf("Chmod(%s): %v", parent, err)
	}
	if err := os.Remove(full); err != nil {
		t.Fatalf("Remove(%s): %v", full, err)
	}
	if err := os.Chmod(parent, 0o555); err != nil {
		t.Fatalf("Chmod(%s): %v", parent, err)
	}
}
