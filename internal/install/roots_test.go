package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeEnv builds a getenv seam over a fixed map, so no test ever reads the
// developer's real environment.
func fakeEnv(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

func fixedHome(dir string) func() (string, error) {
	return func() (string, error) { return dir, nil }
}

func TestResolveRootsXDG(t *testing.T) {
	home := t.TempDir()
	xdgData := filepath.Join(t.TempDir(), "data")
	xdgConfig := filepath.Join(t.TempDir(), "config")
	xdgBin := filepath.Join(t.TempDir(), "bin")

	cases := []struct {
		name           string
		env            map[string]string
		wantData       string
		wantConfigHome string
		wantBinDir     string
	}{
		{
			name:           "unset falls back to home defaults",
			env:            map[string]string{},
			wantData:       filepath.Join(home, ".local", "share", "docket"),
			wantConfigHome: filepath.Join(home, ".config"),
			wantBinDir:     filepath.Join(home, ".local", "bin"),
		},
		{
			name: "absolute XDG values are honored",
			env: map[string]string{
				"XDG_DATA_HOME":   xdgData,
				"XDG_CONFIG_HOME": xdgConfig,
				"XDG_BIN_HOME":    xdgBin,
			},
			wantData:       filepath.Join(xdgData, "docket"),
			wantConfigHome: xdgConfig,
			wantBinDir:     xdgBin,
		},
		{
			name: "relative XDG values are ignored",
			env: map[string]string{
				"XDG_DATA_HOME":   "relative/data",
				"XDG_CONFIG_HOME": "relative/config",
				"XDG_BIN_HOME":    "relative/bin",
			},
			wantData:       filepath.Join(home, ".local", "share", "docket"),
			wantConfigHome: filepath.Join(home, ".config"),
			wantBinDir:     filepath.Join(home, ".local", "bin"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			roots, err := ResolveRoots(fixedHome(home), fakeEnv(tc.env))
			if err != nil {
				t.Fatalf("ResolveRoots: unexpected error: %v", err)
			}
			if roots.Home != home {
				t.Errorf("Home = %q, want %q", roots.Home, home)
			}
			if roots.DataRoot != tc.wantData {
				t.Errorf("DataRoot = %q, want %q", roots.DataRoot, tc.wantData)
			}
			if roots.ConfigHome != tc.wantConfigHome {
				t.Errorf("ConfigHome = %q, want %q", roots.ConfigHome, tc.wantConfigHome)
			}
			if roots.BinDir != tc.wantBinDir {
				t.Errorf("BinDir = %q, want %q", roots.BinDir, tc.wantBinDir)
			}
		})
	}
}

func TestResolveRootsNoHome(t *testing.T) {
	cases := []struct {
		name   string
		homeFn func() (string, error)
	}{
		{"homeFn error", func() (string, error) { return "", os.ErrNotExist }},
		{"empty home", func() (string, error) { return "", nil }},
		{"relative home", func() (string, error) { return "home/u", nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ResolveRoots(tc.homeFn, fakeEnv(nil)); err == nil {
				t.Fatal("ResolveRoots: want error, got nil")
			}
		})
	}
}

func TestResolveRootsExistingNonDirRoot(t *testing.T) {
	home := t.TempDir()
	// Plant a regular file exactly where DataRoot would live.
	if err := os.MkdirAll(filepath.Join(home, ".local", "share"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	dataRoot := filepath.Join(home, ".local", "share", "docket")
	if err := os.WriteFile(dataRoot, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := ResolveRoots(fixedHome(home), fakeEnv(nil))
	if err == nil {
		t.Fatal("ResolveRoots: want error for non-directory data root, got nil")
	}
	if !strings.Contains(err.Error(), dataRoot) {
		t.Errorf("error %q does not name the offending path %q", err, dataRoot)
	}
}

func TestResolveRootsMissingRootIsFine(t *testing.T) {
	home := t.TempDir() // nothing under it yet
	roots, err := ResolveRoots(fixedHome(home), fakeEnv(nil))
	if err != nil {
		t.Fatalf("ResolveRoots: unexpected error for absent roots: %v", err)
	}
	if _, err := os.Stat(roots.DataRoot); !os.IsNotExist(err) {
		t.Fatalf("ResolveRoots must not create %q (stat err = %v)", roots.DataRoot, err)
	}
}

func TestRootsDerivedPaths(t *testing.T) {
	home := t.TempDir()
	roots, err := ResolveRoots(fixedHome(home), fakeEnv(nil))
	if err != nil {
		t.Fatalf("ResolveRoots: %v", err)
	}
	data := roots.DataRoot

	if got, want := roots.VersionsDir(), filepath.Join(data, "versions"); got != want {
		t.Errorf("VersionsDir = %q, want %q", got, want)
	}
	if got, want := roots.TransactionsDir(), filepath.Join(data, "transactions"); got != want {
		t.Errorf("TransactionsDir = %q, want %q", got, want)
	}
	if got, want := roots.StatePath(), filepath.Join(data, "state", "install.json"); got != want {
		t.Errorf("StatePath = %q, want %q", got, want)
	}
	if got, want := roots.VersionDir("sha256:abc123"), filepath.Join(data, "versions", "sha256-abc123", "assets"); got != want {
		t.Errorf("VersionDir = %q, want %q", got, want)
	}
}

// A version directory name is derived from an asset-set id, which is
// attacker-irrelevant but still untrusted shape: sanitisation must keep the
// result a single path segment no matter what arrives.
func TestVersionDirSanitizesID(t *testing.T) {
	home := t.TempDir()
	roots, err := ResolveRoots(fixedHome(home), fakeEnv(nil))
	if err != nil {
		t.Fatalf("ResolveRoots: %v", err)
	}
	for _, id := range []string{"../escape", "sha256:../x", "a/b", `a\b`, "..", "", "sha256:ok_ID-1.2"} {
		dir := roots.VersionDir(id)
		rel, err := filepath.Rel(roots.VersionsDir(), dir)
		if err != nil {
			t.Fatalf("Rel(%q): %v", dir, err)
		}
		segs := strings.Split(rel, string(filepath.Separator))
		if len(segs) != 2 || segs[1] != "assets" {
			t.Errorf("VersionDir(%q) = %q: want exactly <one-segment>/assets under %q", id, dir, roots.VersionsDir())
			continue
		}
		if segs[0] == "" || segs[0] == "." || segs[0] == ".." {
			t.Errorf("VersionDir(%q) produced unsafe segment %q", id, segs[0])
		}
	}
}
