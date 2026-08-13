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

// cleanTempDir is t.TempDir() for the tests that compare a fake home against a
// path the code under test built. t.TempDir() hands back $TMPDIR's spelling
// verbatim (os.TempDir strips trailing slashes and nothing else), so a $TMPDIR
// carrying an interior "//" — which is exactly what scripts/run-tests.sh
// produces, since macOS's default TMPDIR ends in "/" and the runner appends
// "/run-tests.XXXXXX" to it — yields a home that is not lexically clean.
// ResolveRoots normalizes its home ("home = filepath.Clean(home)") and
// filepath.Join cleans everything derived from it, so a test that compares a
// resolved or joined path against the RAW spelling fails on the slashes alone.
// Cleaning here puts the fixture on the same footing production is always on:
// UserRoots is only ever constructed by ResolveRoots, whose Home is clean.
//
// Clean, deliberately NOT filepath.EvalSymlinks: on macOS /var is a symlink to
// /private/var, so resolving would move the fixture to a spelling the code under
// test never produces and trade this mismatch for a worse one.
func cleanTempDir(t *testing.T) string {
	t.Helper()
	return filepath.Clean(t.TempDir())
}

func TestResolveRootsXDG(t *testing.T) {
	home := cleanTempDir(t)
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

// TestResolveRootsNormalizesHome pins the normalization every other root
// consumer leans on: whatever spelling the environment hands over, the Home
// that leaves ResolveRoots is lexically clean, and so is each root derived from
// it. Nothing asserted this before — the XDG table fed ResolveRoots an
// already-clean path and so could not tell normalization from a passthrough —
// and the property is not cosmetic. Callers compare paths as STRINGS (a
// planned target against the root it must fall under, a state entry against
// the root that owns it), and every path the planners produce comes out of
// filepath.Join, which cleans. A Home that skipped normalization would compare
// unequal to its own children and quietly fail containment.
func TestResolveRootsNormalizesHome(t *testing.T) {
	want := cleanTempDir(t)
	// The shapes a shell hands over in practice: a trailing slash, and the
	// doubled interior separator "${TMPDIR%/}/x" leaves behind when TMPDIR
	// already ends in one.
	cases := map[string]string{
		"trailing separator":   want + "/",
		"trailing dot":         want + "/./",
		"doubled separator":    filepath.Dir(want) + "//" + filepath.Base(want),
		"interior dot segment": filepath.Dir(want) + "/./" + filepath.Base(want),
	}
	for name, messy := range cases {
		t.Run(name, func(t *testing.T) {
			if messy == want {
				t.Fatalf("input %q is already clean; the case asserts nothing", messy)
			}
			roots, err := ResolveRoots(fixedHome(messy), fakeEnv(nil))
			if err != nil {
				t.Fatalf("ResolveRoots(%q): %v", messy, err)
			}
			if roots.Home != want {
				t.Errorf("Home = %q, want the cleaned %q", roots.Home, want)
			}
			for rootName, got := range map[string]string{
				"Home":       roots.Home,
				"DataRoot":   roots.DataRoot,
				"ConfigHome": roots.ConfigHome,
				"BinDir":     roots.BinDir,
				"StatePath":  roots.StatePath(),
			} {
				if got != filepath.Clean(got) {
					t.Errorf("%s = %q is not lexically clean", rootName, got)
				}
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
