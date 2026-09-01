package reposetup

import (
	"errors"
	"io/fs"
	"path"
	"sort"
	"testing"
)

// mapTree is a TestTree test double backed by a map of repo-root-relative path
// to file content. Absence is (false/nil, nil) — never an error — matching the
// probe-error-is-not-clean-absence contract the real trees honor.
type mapTree map[string]string

func (m mapTree) Exists(p string) (bool, error) {
	_, ok := m[p]
	return ok, nil
}

func (m mapTree) ReadFile(p string) ([]byte, error) {
	content, ok := m[p]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return []byte(content), nil
}

func (m mapTree) Glob(pattern string) ([]string, error) {
	var out []string
	for p := range m {
		ok, err := path.Match(pattern, p)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out, nil
}

// errTree fails every probe: it stands in for an I/O fault (unreadable tree,
// broken pin) that must abort discovery as UNKNOWN, never be folded into none.
type errTree struct{}

var errProbe = errors.New("reposetup_test: probe fault")

func (errTree) Exists(string) (bool, error)     { return false, errProbe }
func (errTree) ReadFile(string) ([]byte, error) { return nil, errProbe }
func (errTree) Glob(string) ([]string, error)   { return nil, errProbe }

// panicTree panics on ANY method call. Passing it through DiscoverTests proves
// the configured short-circuit never touches the tree.
type panicTree struct{}

func (panicTree) Exists(string) (bool, error)     { panic("configured path must not probe the tree (Exists)") }
func (panicTree) ReadFile(string) ([]byte, error) { panic("configured path must not probe the tree (ReadFile)") }
func (panicTree) Glob(string) ([]string, error)   { panic("configured path must not probe the tree (Glob)") }

func TestDiscoverConfiguredShortCircuitsWithoutProbing(t *testing.T) {
	// Explicit, non-legacy commands on BOTH pairs: configured, and the tree is
	// never consulted (panicTree would blow up if any detector ran).
	out, err := DiscoverTests(panicTree{}, "go test ./...", "make check")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Kind != DiscoveryConfigured {
		t.Errorf("kind = %q, want %q", out.Kind, DiscoveryConfigured)
	}
	if out.Command != "" || out.Candidates != nil {
		t.Errorf("configured must carry no command/candidates, got %q/%+v", out.Command, out.Candidates)
	}
}

func TestDiscoverConfiguredWhenEitherCommandExplicit(t *testing.T) {
	// A single explicit command is enough to be configured: discovery writes
	// BOTH keys from one outcome, so probing over an already-set command would
	// clobber it. Either side being explicit short-circuits.
	if out, err := DiscoverTests(panicTree{}, "go test ./...", ""); err != nil || out.Kind != DiscoveryConfigured {
		t.Errorf("build-only explicit: kind=%q err=%v, want configured", out.Kind, err)
	}
	if out, err := DiscoverTests(panicTree{}, "", "make check"); err != nil || out.Kind != DiscoveryConfigured {
		t.Errorf("finalize-only explicit: kind=%q err=%v, want configured", out.Kind, err)
	}
}

func TestDiscoverLegacyAutoIsUnconfiguredSoDiscoveryRuns(t *testing.T) {
	// `auto` and "" are the unconfigured spellings: discovery must run over the
	// tree, not short-circuit as configured.
	tree := mapTree{"go.mod": "module x", "x_test.go": ""}
	out, err := DiscoverTests(tree, "auto", "auto")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Kind != DiscoveryDetected || out.Command != "go test ./..." {
		t.Errorf("kind/command = %q/%q, want detected/go test ./...", out.Kind, out.Command)
	}
}

func TestDiscoverEachFamilyExactCommand(t *testing.T) {
	cases := []struct {
		name    string
		tree    mapTree
		family  string
		command string
	}{
		{
			name:    "makefile",
			tree:    mapTree{"Makefile": "build:\n\tgo build\ntest:\n\tgo test ./...\n"},
			family:  "makefile",
			command: "make test",
		},
		{
			name:    "makefile lowercase name",
			tree:    mapTree{"makefile": "test: deps\n\tpytest\n"},
			family:  "makefile",
			command: "make test",
		},
		{
			name:    "go top-level tests",
			tree:    mapTree{"go.mod": "module x", "main_test.go": ""},
			family:  "go",
			command: "go test ./...",
		},
		{
			name:    "go one-level-deep tests",
			tree:    mapTree{"go.mod": "module x", "pkg/thing_test.go": ""},
			family:  "go",
			command: "go test ./...",
		},
		{
			name:    "node npm",
			tree:    mapTree{"package.json": `{"scripts":{"test":"jest"}}`, "package-lock.json": ""},
			family:  "node",
			command: "npm test",
		},
		{
			name:    "node yarn",
			tree:    mapTree{"package.json": `{"scripts":{"test":"jest"}}`, "yarn.lock": ""},
			family:  "node",
			command: "yarn test",
		},
		{
			name:    "node pnpm",
			tree:    mapTree{"package.json": `{"scripts":{"test":"jest"}}`, "pnpm-lock.yaml": ""},
			family:  "node",
			command: "pnpm test",
		},
		{
			name:    "pytest ini",
			tree:    mapTree{"pytest.ini": "[pytest]\n", "test_thing.py": ""},
			family:  "pytest",
			command: "pytest",
		},
		{
			name:    "pytest pyproject",
			tree:    mapTree{"pyproject.toml": "[tool.pytest.ini_options]\n", "tests/test_thing.py": ""},
			family:  "pytest",
			command: "pytest",
		},
		{
			name:    "pytest setup.cfg",
			tree:    mapTree{"setup.cfg": "[tool:pytest]\n", "pkg/test_thing.py": ""},
			family:  "pytest",
			command: "pytest",
		},
		{
			name:    "rust",
			tree:    mapTree{"Cargo.toml": "[package]\nname = \"x\"\n"},
			family:  "rust",
			command: "cargo test",
		},
		{
			name:    "shell",
			tree:    mapTree{"tests/test_smoke.sh": ""},
			family:  "shell",
			command: `bash -c 'set -e; for t in tests/test_*.sh; do bash "$t"; done'`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, err := DiscoverTests(c.tree, "", "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out.Kind != DiscoveryDetected {
				t.Fatalf("kind = %q, want detected", out.Kind)
			}
			if out.Command != c.command {
				t.Errorf("command = %q, want %q", out.Command, c.command)
			}
			if len(out.Candidates) != 1 {
				t.Fatalf("detected must carry exactly one candidate, got %+v", out.Candidates)
			}
			if out.Candidates[0].Family != c.family {
				t.Errorf("family = %q, want %q", out.Candidates[0].Family, c.family)
			}
			if out.Candidates[0].Command != c.command {
				t.Errorf("candidate command = %q, want %q", out.Candidates[0].Command, c.command)
			}
			if out.Candidates[0].Evidence == "" {
				t.Errorf("detected candidate must carry human evidence, got empty")
			}
		})
	}
}

func TestDiscoverNoFamiliesIsNone(t *testing.T) {
	tree := mapTree{"README.md": "# hi", "LICENSE": ""}
	out, err := DiscoverTests(tree, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Kind != DiscoveryNone {
		t.Errorf("kind = %q, want none", out.Kind)
	}
	if out.Command != "" || out.Candidates != nil {
		t.Errorf("none carries nothing, got %q/%+v", out.Command, out.Candidates)
	}
}

func TestDiscoverAmbiguousListsAllCandidates(t *testing.T) {
	tree := mapTree{"go.mod": "module x", "x_test.go": "", "Cargo.toml": "[package]"}
	out, err := DiscoverTests(tree, "", "")
	if err != nil || out.Kind != DiscoveryAmbiguous {
		t.Fatalf("kind = %v err %v, want ambiguous", out.Kind, err)
	}
	if len(out.Candidates) != 2 || out.Candidates[0].Family != "go" || out.Candidates[1].Family != "rust" {
		t.Errorf("candidates = %+v, want [go rust]", out.Candidates)
	}
	if out.Command != "" {
		t.Errorf("ambiguous must carry no command, got %q", out.Command)
	}
}

func TestDiscoverNodeTwoLockfilesIsSilent(t *testing.T) {
	// Two recognized lockfiles is an unrecognizable launcher: the node detector
	// reports NO match. With nothing else in the tree, the outcome is none.
	tree := mapTree{
		"package.json":      `{"scripts":{"test":"jest"}}`,
		"package-lock.json": "",
		"yarn.lock":         "",
	}
	out, err := DiscoverTests(tree, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Kind != DiscoveryNone {
		t.Errorf("kind = %q, want none (two lockfiles are not a detected suite)", out.Kind)
	}
}

func TestDiscoverNodeZeroLockfilesIsSilent(t *testing.T) {
	tree := mapTree{"package.json": `{"scripts":{"test":"jest"}}`}
	out, err := DiscoverTests(tree, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Kind != DiscoveryNone {
		t.Errorf("kind = %q, want none (no lockfile is not a detected suite)", out.Kind)
	}
}

func TestDiscoverNodePlaceholderScriptIsSilent(t *testing.T) {
	// The npm default placeholder script is not a real suite.
	tree := mapTree{
		"package.json":      `{"scripts":{"test":"echo \"Error: no test specified\" && exit 1"}}`,
		"package-lock.json": "",
	}
	out, err := DiscoverTests(tree, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Kind != DiscoveryNone {
		t.Errorf("kind = %q, want none (placeholder test script)", out.Kind)
	}
}

func TestDiscoverPytestConfigWithoutTestFilesIsSilent(t *testing.T) {
	tree := mapTree{"pytest.ini": "[pytest]\n"}
	out, err := DiscoverTests(tree, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Kind != DiscoveryNone {
		t.Errorf("kind = %q, want none (pytest config but no test files)", out.Kind)
	}
}

func TestDiscoverMakefileWithoutTestTargetIsSilent(t *testing.T) {
	tree := mapTree{"Makefile": "build:\n\tgo build\ninstall:\n\tcp x /usr/bin\n"}
	out, err := DiscoverTests(tree, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Kind != DiscoveryNone {
		t.Errorf("kind = %q, want none (no column-0 test: target)", out.Kind)
	}
}

func TestDiscoverGoOnlyDeepTestsIsUnmatched(t *testing.T) {
	// The go detector probes two levels only (path.Match has no ** recursion);
	// a module whose tests live 2+ dirs deep is not matched. Documented bound.
	tree := mapTree{"go.mod": "module x", "a/b/deep_test.go": ""}
	out, err := DiscoverTests(tree, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Kind != DiscoveryNone {
		t.Errorf("kind = %q, want none (tests deeper than two levels are out of the glob bound)", out.Kind)
	}
}

func TestProbeErrorIsUnknownNeverNone(t *testing.T) {
	out, err := DiscoverTests(errTree{}, "", "")
	if err == nil {
		t.Fatal("a probe error must surface as an error, never a clean none")
	}
	if out.Kind != "" {
		t.Errorf("a probe error yields the zero outcome, got kind %q", out.Kind)
	}
}
