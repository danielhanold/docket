package reposetup

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"strings"
)

// DiscoveryKind names the four discovery outcomes. The vocabulary is fixed by
// the change-0374 spec and must not gain a fifth value without a spec change.
type DiscoveryKind string

const (
	DiscoveryConfigured DiscoveryKind = "configured"
	DiscoveryDetected   DiscoveryKind = "detected"
	DiscoveryNone       DiscoveryKind = "none"
	DiscoveryAmbiguous  DiscoveryKind = "ambiguous"
)

// TestTree is the pinned-tree read seam. Init reads the primary worktree;
// migrate reads the pinned git tree; tests use a map. Every method returns an
// error ONLY for a probe failure — absence is (false/nil, nil), never an error
// (learning probe-error-is-not-clean-absence: a probe error aborts discovery
// as unknown; it is never folded into "none" and never into "off").
type TestTree interface {
	Exists(path string) (bool, error)
	ReadFile(path string) ([]byte, error)  // fs.ErrNotExist for absent
	Glob(pattern string) ([]string, error) // path.Match semantics, repo-root-relative
}

// DetectedSuite is one family's certification: a stable family token, the exact
// command that family runs, and a one-sentence human account of what matched.
type DetectedSuite struct {
	Family   string // stable token: makefile|go|node|pytest|rust|shell
	Command  string // the exact command this family certifies
	Evidence string // one human sentence naming the files that matched
}

// DiscoveryOutcome is the pure result of DiscoverTests. Command is set only for
// detected; Candidates holds the single detected suite (len 1) or every match
// in family-registry order for ambiguous.
type DiscoveryOutcome struct {
	Kind       DiscoveryKind
	Command    string          // set for detected; the single suite command
	Candidates []DetectedSuite // detected: len 1; ambiguous: all matches, family order
}

// DiscoverTests: declared build/finalize commands that are explicit and
// non-legacy ("" and "auto" are unconfigured) yield configured without
// probing — either side being explicit short-circuits, because a detected
// outcome writes BOTH keys and probing over an already-set command would
// clobber it. Otherwise every registered detector runs; one match is detected,
// zero is none, two or more is ambiguous (no priority list guesses). A probe
// error aborts discovery as unknown: it returns (DiscoveryOutcome{}, err) and
// is never folded into none.
func DiscoverTests(tree TestTree, declaredBuildCommand, declaredFinalizeCommand string) (DiscoveryOutcome, error) {
	if isConfiguredCommand(declaredBuildCommand) || isConfiguredCommand(declaredFinalizeCommand) {
		return DiscoveryOutcome{Kind: DiscoveryConfigured}, nil
	}
	var matches []DetectedSuite
	for _, d := range detectorRegistry {
		suite, err := d.detect(tree)
		if err != nil {
			return DiscoveryOutcome{}, fmt.Errorf("reposetup: test discovery probe failed for family %q: %w", d.family, err)
		}
		if suite != nil {
			matches = append(matches, *suite)
		}
	}
	switch len(matches) {
	case 0:
		return DiscoveryOutcome{Kind: DiscoveryNone}, nil
	case 1:
		return DiscoveryOutcome{Kind: DiscoveryDetected, Command: matches[0].Command, Candidates: matches}, nil
	default:
		return DiscoveryOutcome{Kind: DiscoveryAmbiguous, Candidates: matches}, nil
	}
}

// isConfiguredCommand reports whether a declared command is a real, explicit
// suite command. "" is unconfigured and "auto" is the legacy migration
// spelling of the same unconfigured state (spec: `auto` never survives
// resolution as a valid command).
func isConfiguredCommand(cmd string) bool {
	return cmd != "" && cmd != "auto"
}

// AmbiguousTestDiscoveryError reports that setup-time test discovery matched more
// than one suite family and cannot deterministically choose one. It carries the
// matched candidates and renders a remedy naming the exact setup command, so a
// caller (migrate) can surface it BEFORE any repository mutation rather than
// guessing a command. Init tolerates ambiguity (it reports the candidates and
// writes nothing); only migrate treats it as a typed refusal.
type AmbiguousTestDiscoveryError struct {
	Candidates []DetectedSuite
}

// Error names the ambiguous families and the exact remedy command.
func (e *AmbiguousTestDiscoveryError) Error() string {
	fams := make([]string, 0, len(e.Candidates))
	for _, c := range e.Candidates {
		fams = append(fams, c.Family)
	}
	return fmt.Sprintf("test discovery is ambiguous: multiple suite families match (%s); set build.test_command / finalize.test_command explicitly, then run `docket repository configure-tests`",
		strings.Join(fams, ", "))
}

// detector pairs a family token with its pure detection function. The registry
// slice below is the SINGLE owning place for the supported shapes and their
// order; DiscoverTests iterates it in order, so family order in an ambiguous
// outcome is exactly this slice order (no map iteration reaches output).
type detector struct {
	family string
	detect func(TestTree) (*DetectedSuite, error)
}

// detectorRegistry is the closed, ordered set of recognized test-suite shapes.
// A detect func returns a non-nil suite for a match, nil for no match, and a
// non-nil error ONLY for a probe fault (which aborts the whole discovery).
var detectorRegistry = []detector{
	{family: "makefile", detect: detectMakefile},
	{family: "go", detect: detectGo},
	{family: "node", detect: detectNode},
	{family: "pytest", detect: detectPytest},
	{family: "rust", detect: detectRust},
	{family: "shell", detect: detectShell},
}

// makeTestTarget matches a column-0 `test:` target line (GNU make tolerates
// whitespace between the target name and its colon).
var makeTestTarget = regexp.MustCompile(`(?m)^test[ \t]*:`)

// detectMakefile matches a `Makefile` (or lowercase `makefile`) that declares a
// column-0 `test:` target.
func detectMakefile(tree TestTree) (*DetectedSuite, error) {
	var content []byte
	found := false
	for _, name := range []string{"Makefile", "makefile"} {
		data, err := tree.ReadFile(name)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, err
		}
		content = data
		found = true
		break
	}
	if !found || !makeTestTarget.Match(content) {
		return nil, nil
	}
	return &DetectedSuite{
		Family:   "makefile",
		Command:  "make test",
		Evidence: "a Makefile declares a column-0 test: target",
	}, nil
}

// detectGo matches a repo-root Go module with test files at the root or one
// directory deep. It probes exactly two levels: path.Match has no `**`
// recursion, so a module whose tests live 2+ directories deep is NOT matched —
// a deliberate, documented bound rather than a silent miss.
func detectGo(tree TestTree) (*DetectedSuite, error) {
	hasMod, err := tree.Exists("go.mod")
	if err != nil {
		return nil, err
	}
	if !hasMod {
		return nil, nil
	}
	hasTests, err := anyGlobMatches(tree, "*_test.go", "*/*_test.go")
	if err != nil {
		return nil, err
	}
	if !hasTests {
		return nil, nil
	}
	return &DetectedSuite{
		Family:   "go",
		Command:  "go test ./...",
		Evidence: "a root go.mod with _test.go files at or one level below the root",
	}, nil
}

// detectNode matches a package.json declaring a real `scripts.test` (not the
// npm "no test specified" placeholder) alongside EXACTLY ONE recognized
// lockfile. Zero or two-plus lockfiles is an unrecognizable launcher and is not
// a detected suite.
func detectNode(tree TestTree) (*DetectedSuite, error) {
	data, err := tree.ReadFile("package.json")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var pkg struct {
		Scripts struct {
			Test string `json:"test"`
		} `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		// Unparseable manifest: a launcher we cannot recognize, not a probe
		// fault of the tree. No match.
		return nil, nil
	}
	script := strings.TrimSpace(pkg.Scripts.Test)
	if script == "" || strings.Contains(script, "no test specified") {
		return nil, nil
	}
	lockfiles := []struct{ file, command string }{
		{"package-lock.json", "npm test"},
		{"yarn.lock", "yarn test"},
		{"pnpm-lock.yaml", "pnpm test"},
	}
	var matchedFile, matchedCommand string
	count := 0
	for _, lf := range lockfiles {
		ok, err := tree.Exists(lf.file)
		if err != nil {
			return nil, err
		}
		if ok {
			count++
			matchedFile, matchedCommand = lf.file, lf.command
		}
	}
	if count != 1 {
		return nil, nil
	}
	return &DetectedSuite{
		Family:   "node",
		Command:  matchedCommand,
		Evidence: fmt.Sprintf("a package.json scripts.test with a single %s lockfile", matchedFile),
	}, nil
}

// detectPytest matches a pytest configuration (pytest.ini, or the pytest table
// in pyproject.toml / setup.cfg) together with at least one test file at the
// root, in tests/, or one directory deep.
func detectPytest(tree TestTree) (*DetectedSuite, error) {
	hasConfig, err := tree.Exists("pytest.ini")
	if err != nil {
		return nil, err
	}
	if !hasConfig {
		hasConfig, err = fileContains(tree, "pyproject.toml", "[tool.pytest.ini_options]")
		if err != nil {
			return nil, err
		}
	}
	if !hasConfig {
		hasConfig, err = fileContains(tree, "setup.cfg", "[tool:pytest]")
		if err != nil {
			return nil, err
		}
	}
	if !hasConfig {
		return nil, nil
	}
	hasTests, err := anyGlobMatches(tree, "test_*.py", "tests/test_*.py", "*/test_*.py")
	if err != nil {
		return nil, err
	}
	if !hasTests {
		return nil, nil
	}
	return &DetectedSuite{
		Family:   "pytest",
		Command:  "pytest",
		Evidence: "a pytest configuration with test_*.py files",
	}, nil
}

// detectRust matches a repo-root Cargo.toml.
func detectRust(tree TestTree) (*DetectedSuite, error) {
	ok, err := tree.Exists("Cargo.toml")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	return &DetectedSuite{
		Family:   "rust",
		Command:  "cargo test",
		Evidence: "a root Cargo.toml",
	}, nil
}

// detectShell matches a tests/ directory of test_*.sh scripts.
func detectShell(tree TestTree) (*DetectedSuite, error) {
	hasTests, err := anyGlobMatches(tree, "tests/test_*.sh")
	if err != nil {
		return nil, err
	}
	if !hasTests {
		return nil, nil
	}
	return &DetectedSuite{
		Family:   "shell",
		Command:  `bash -c 'set -e; for t in tests/test_*.sh; do bash "$t"; done'`,
		Evidence: "tests/test_*.sh scripts",
	}, nil
}

// anyGlobMatches reports whether any of the patterns matches at least one path.
// A probe fault on any pattern aborts (returns the error).
func anyGlobMatches(tree TestTree, patterns ...string) (bool, error) {
	for _, p := range patterns {
		matches, err := tree.Glob(p)
		if err != nil {
			return false, err
		}
		if len(matches) > 0 {
			return true, nil
		}
	}
	return false, nil
}

// fileContains reports whether the file at path exists and contains needle. An
// absent file is (false, nil) — never an error; only a real probe fault errors.
func fileContains(tree TestTree, path, needle string) (bool, error) {
	data, err := tree.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return strings.Contains(string(data), needle), nil
}
