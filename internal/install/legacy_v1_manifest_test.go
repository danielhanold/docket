package install

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/danielhanold/docket/internal/assets"
)

// legacyV1Fixture is deliberately built from the v1 protocol roots. Its future
// directory is not an allowed fixture convenience: a reconstruction must reject
// it even if a later live generator starts producing such a root.
func legacyV1Fixture(t *testing.T) (assets.Manifest, string) {
	t.Helper()
	payload := map[string][]byte{
		"skills/docket-build/SKILL.md":     []byte("skill\n"),
		"agents/docket-build.md":           []byte("agent\n"),
		"agents/harness-defaults.yml":      []byte("defaults\n"),
		"cursor-rules/docket-dispatch.mdc": []byte("dispatch\n"),
		".docket.example.yml":              []byte("example\n"),
	}
	roles := map[string]assets.Role{
		"skills/docket-build/SKILL.md":     assets.RoleSkill,
		"agents/docket-build.md":           assets.RoleAgentSource,
		"agents/harness-defaults.yml":      assets.RoleHarnessDefaults,
		"cursor-rules/docket-dispatch.mdc": assets.RoleDispatch,
		".docket.example.yml":              assets.RoleConfigSchema,
	}
	base := t.TempDir()
	assetsDir := filepath.Join(base, "pending", versionAssetsDir)
	paths := make([]string, 0, len(payload))
	for p := range payload {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	m := assets.Manifest{FormatVersion: assets.ManifestFormatVersion, AssetProtocol: assets.AssetProtocol}
	for _, p := range paths {
		body := payload[p]
		full := filepath.Join(assetsDir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), versionDirMode); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, body, versionFileMode); err != nil {
			t.Fatal(err)
		}
		m.Entries = append(m.Entries, assets.Entry{Path: p, Role: roles[p], Mode: 0o644, Size: int64(len(body)), SHA256: sha256Hex(body)})
	}
	id, err := assets.ComputeAssetSetID(m)
	if err != nil {
		t.Fatal(err)
	}
	m.AssetSetID = id
	if err := assets.ValidateManifest(m); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, sanitizeSegment(id))
	if err := os.Rename(filepath.Dir(assetsDir), root); err != nil {
		t.Fatal(err)
	}
	return m, filepath.Join(root, versionAssetsDir)
}

func TestReconstructLegacyV1Manifest(t *testing.T) {
	want, assetsDir := legacyV1Fixture(t)
	got, err := reconstructLegacyV1Manifest(assetsDir)
	if err != nil {
		t.Fatalf("reconstructLegacyV1Manifest: %v", err)
	}
	if got.AssetSetID != want.AssetSetID || len(got.Entries) != len(want.Entries) {
		t.Fatalf("reconstructed manifest = %#v, want %#v", got, want)
	}
	for i := range want.Entries {
		if got.Entries[i] != want.Entries[i] {
			t.Errorf("entry %d = %#v, want %#v", i, got.Entries[i], want.Entries[i])
		}
	}
	if proof, err := ProveVersionTree(filepath.Dir(assetsDir)); err != nil {
		t.Fatalf("ProveVersionTree(legacy) = %v", err)
	} else if !proof.Legacy || proof.Manifest.AssetSetID != want.AssetSetID {
		t.Errorf("legacy proof = %#v, want reconstructed legacy manifest", proof)
	}
}

func TestReconstructLegacyV1ManifestRejectsOutsideFrozenBoundary(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(t *testing.T, assetsDir string)
	}{
		{"unknown root", func(t *testing.T, assetsDir string) { writeIntoTree(t, assetsDir, "unknown/file", "foreign\n") }},
		{"future live generator root", func(t *testing.T, assetsDir string) {
			writeIntoTree(t, assetsDir, "future-generator-root/file", "future\n")
		}},
		{"empty directory", func(t *testing.T, assetsDir string) {
			if err := os.Mkdir(filepath.Join(assetsDir, "skills", "empty"), versionDirMode); err != nil {
				t.Fatal(err)
			}
		}},
		{"changed mode", func(t *testing.T, assetsDir string) {
			if err := os.Chmod(filepath.Join(assetsDir, "skills", "docket-build", "SKILL.md"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{"changed digest", func(t *testing.T, assetsDir string) {
			writeIntoTree(t, assetsDir, "skills/docket-build/SKILL.md", "changed\n")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, assetsDir := legacyV1Fixture(t)
			tc.mutate(t, assetsDir)
			if _, err := reconstructLegacyV1Manifest(assetsDir); !errors.Is(err, ErrVersionTreeInvalid) {
				t.Fatalf("reconstructLegacyV1Manifest = %v, want ErrVersionTreeInvalid", err)
			}
		})
	}
}

func TestEnsureVersionTreeReusesProvenLegacyV1Tree(t *testing.T) {
	m, assetsDir := legacyV1Fixture(t)
	root := filepath.Dir(assetsDir)
	dataRoot := t.TempDir()
	versions := filepath.Join(dataRoot, "versions")
	if err := os.MkdirAll(versions, versionDirMode); err != nil {
		t.Fatal(err)
	}
	movedRoot := filepath.Join(versions, filepath.Base(root))
	if err := os.Rename(root, movedRoot); err != nil {
		t.Fatal(err)
	}
	assetsDir = filepath.Join(movedRoot, versionAssetsDir)
	root = movedRoot
	roots := UserRoots{DataRoot: dataRoot}
	got, reused, err := EnsureVersionTree(roots, m, poisonedOpen(t))
	if err != nil {
		t.Fatalf("EnsureVersionTree(legacy) = %v", err)
	}
	if !reused || got != assetsDir {
		t.Errorf("EnsureVersionTree legacy = (%q, reused=%v), want (%q, true)", got, reused, assetsDir)
	}
	if _, err := os.Lstat(filepath.Join(root, "manifest.json")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("legacy root was rewritten with manifest: %v", err)
	}
}
