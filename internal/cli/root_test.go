package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/buildinfo"
	"github.com/danielhanold/docket/internal/install"
)

// treeWalkCommand is the scratch command captureTree registers. It is
// asset-independent for the duration of that run only.
const treeWalkCommand = "treewalk"

func devInfo() buildinfo.Info {
	return buildinfo.Info{Version: "development", Commit: "unknown", BuildDate: "unknown"}
}

func hostFacts() buildinfo.RuntimeFacts {
	return buildinfo.RuntimeFacts{GoVersion: "go1.26.5", GOOS: "darwin", GOARCH: "arm64"}
}

func runCLI(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errBuf bytes.Buffer
	code = Run(args, strings.NewReader(""), &out, &errBuf, devInfo(), hostFacts())
	return out.String(), errBuf.String(), code
}

func TestVersionHuman(t *testing.T) {
	out, errS, code := runCLI(t, "version")
	if code != 0 || errS != "" || out != "docket development (commit unknown, built unknown)\n" {
		t.Fatalf("out=%q err=%q code=%d", out, errS, code)
	}
}

func TestVersionJSONFlagPositions(t *testing.T) {
	want := `{"protocol_version":1,"operation":"version","result":"applied","version":"development","commit":"unknown","build_date":"unknown"}` + "\n"
	for _, args := range [][]string{{"--json", "version"}, {"version", "--json"}} {
		out, errS, code := runCLI(t, args...)
		if code != 0 || errS != "" || out != want {
			t.Fatalf("args=%v out=%q err=%q code=%d", args, out, errS, code)
		}
	}
}

func TestVersionJSONFalseIsHuman(t *testing.T) {
	out, errS, code := runCLI(t, "version", "--json=false")
	if code != 0 || errS != "" || out != "docket development (commit unknown, built unknown)\n" {
		t.Fatalf("out=%q err=%q code=%d", out, errS, code)
	}
}

// pflag parses --json with strconv.ParseBool, so every spelling that Bool
// accepts selects the mode once Cobra has parsed the flag successfully. The
// pre-scan's three-spelling grammar is the fallback for the parse-failure
// path, not the mode input on a clean parse.
func TestBoundJSONFlagSpellings(t *testing.T) {
	jsonWant := `{"protocol_version":1,"operation":"version","result":"applied","version":"development","commit":"unknown","build_date":"unknown"}` + "\n"
	humanWant := "docket development (commit unknown, built unknown)\n"
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"--json=1", "version"}, jsonWant},
		{[]string{"--json=TRUE", "version"}, jsonWant},
		{[]string{"--json=t", "version"}, jsonWant},
		{[]string{"version", "--json=True"}, jsonWant},
		{[]string{"--json=0", "version"}, humanWant},
		{[]string{"--json=F", "version"}, humanWant},
		{[]string{"--json=1", "version", "--json=0"}, humanWant},
	} {
		out, errS, code := runCLI(t, c.args...)
		if code != 0 || errS != "" || out != c.want {
			t.Fatalf("args=%v out=%q err=%q code=%d, want out=%q", c.args, out, errS, code, c.want)
		}
	}
}

// The conflict rule keys on the mode actually selected, so a Bool spelling the
// pre-scan does not recognize still conflicts with help.
func TestJSONHelpConflictBoundSpelling(t *testing.T) {
	for _, args := range [][]string{
		{"--json=1", "--help"},
		{"--json=TRUE", "-h"},
		{"--json=1", "help"},
		{"--json=1", "help", "version"},
	} {
		out, errS, code := runCLI(t, args...)
		if code != 2 {
			t.Fatalf("args=%v code=%d, want 2", args, code)
		}
		if errS != "" {
			t.Fatalf("args=%v stderr=%q, want empty", args, errS)
		}
		if !strings.Contains(out, `"reason":"json-help-conflict"`) {
			t.Fatalf("args=%v stdout=%q", args, out)
		}
		if strings.Contains(out, "Usage") {
			t.Fatalf("args=%v help text leaked into protocol stream: %q", args, out)
		}
	}
	// A Bool-false spelling is not JSON mode, so help renders as usual.
	out, errS, code := runCLI(t, "--json=0", "--help")
	if code != 0 || errS != "" || !strings.Contains(out, "Usage") {
		t.Fatalf("--json=0 --help: out=%q err=%q code=%d", out, errS, code)
	}
}

// The fallback's boundary, pinned deliberately: when parsing dies before
// reaching the flag there is no bound value, so only the pre-scan's three
// spellings can still select JSON mode. --json=1 after a failing token is
// therefore a human-mode error.
func TestBoundSpellingAfterParseErrorFallsBackToHuman(t *testing.T) {
	for _, args := range [][]string{
		// Parsing stops at the unknown token before reaching --json=1.
		{"version", "--bogus", "--json=1"},
		// Command resolution fails before any flag is parsed.
		{"--json=1", "bogus"},
	} {
		out, errS, code := runCLI(t, args...)
		if code != 2 || out != "" {
			t.Fatalf("args=%v out=%q code=%d", args, out, code)
		}
		if !strings.HasPrefix(errS, "docket: ") {
			t.Fatalf("args=%v stderr = %q", args, errS)
		}
	}
	// A bound spelling that pflag DID reach still selects JSON mode for the
	// error document, whether the failure is a later flag or argument checking.
	for _, args := range [][]string{
		{"--json=1", "version", "--bogus"},
		{"--json=TRUE", "version", "extra"},
	} {
		out, errS, code := runCLI(t, args...)
		if code != 2 || errS != "" {
			t.Fatalf("args=%v err=%q code=%d", args, errS, code)
		}
		if !strings.Contains(out, `"result":"invalid-input"`) {
			t.Fatalf("args=%v stdout=%q", args, out)
		}
	}
}

func TestDiagnosticRuntimeJSON(t *testing.T) {
	out, errS, code := runCLI(t, "diagnostic", "runtime", "--json")
	want := `{"protocol_version":1,"operation":"diagnostic.runtime","result":"applied","go_version":"go1.26.5","go_os":"darwin","go_arch":"arm64","supported_target":true}` + "\n"
	if code != 0 || errS != "" || out != want {
		t.Fatalf("out=%q err=%q code=%d", out, errS, code)
	}
}

func TestHostileParseOrderStillJSON(t *testing.T) {
	// --json AFTER the failing token: the transport scan, not Cobra, must
	// select JSON mode, or this emits a human error.
	out, errS, code := runCLI(t, "version", "--bogus", "--json")
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if errS != "" {
		t.Fatalf("stderr must be empty in JSON mode, got %q", errS)
	}
	if !strings.HasPrefix(out, `{"protocol_version":1,"operation":"cli","result":"invalid-input","reason":"invalid-arguments",`) {
		t.Fatalf("stdout = %q", out)
	}
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("must be exactly one newline-terminated document, got %q", out)
	}
}

func TestHumanParseErrorStdoutEmpty(t *testing.T) {
	out, errS, code := runCLI(t, "version", "--bogus")
	if code != 2 || out != "" {
		t.Fatalf("out=%q code=%d", out, code)
	}
	if !strings.HasPrefix(errS, "docket: ") {
		t.Fatalf("stderr = %q", errS)
	}
}

func TestJSONHelpConflictThreeForms(t *testing.T) {
	for _, args := range [][]string{
		{"--json", "--help"},
		{"--json", "-h"},
		{"--json", "help"},
	} {
		out, errS, code := runCLI(t, args...)
		if code != 2 {
			t.Fatalf("args=%v code=%d, want 2", args, code)
		}
		if errS != "" {
			t.Fatalf("args=%v stderr=%q, want empty", args, errS)
		}
		if !strings.Contains(out, `"reason":"json-help-conflict"`) {
			t.Fatalf("args=%v stdout=%q", args, out)
		}
		if strings.Contains(out, "Usage") {
			t.Fatalf("args=%v help text leaked into protocol stream: %q", args, out)
		}
	}
}

// An unresolvable help topic must not escape the conflict policy: Cobra's own
// help command answers an unknown topic with prose plus usage on stdout, which
// would put non-protocol bytes into the JSON stream and exit 0.
func TestJSONHelpConflictUnknownTopic(t *testing.T) {
	for _, args := range [][]string{
		{"--json", "help", "bogus"},
		{"--json", "help", "completion"},
		{"--json", "help", "version"},
		{"--json", "help", "diagnostic", "runtime"},
	} {
		out, errS, code := runCLI(t, args...)
		if code != 2 {
			t.Fatalf("args=%v code=%d, want 2", args, code)
		}
		if errS != "" {
			t.Fatalf("args=%v stderr=%q, want empty", args, errS)
		}
		if !strings.Contains(out, `"reason":"json-help-conflict"`) {
			t.Fatalf("args=%v stdout=%q", args, out)
		}
		if strings.Contains(out, "Usage") || strings.Contains(out, "Unknown help topic") {
			t.Fatalf("args=%v help text leaked into protocol stream: %q", args, out)
		}
		if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
			t.Fatalf("args=%v must be one newline-terminated document, got %q", args, out)
		}
	}
}

// Human mode: a resolvable topic renders Cobra help on stdout at exit 0; an
// unresolvable one is invalid input like any other unknown command, so it
// leaves stdout empty rather than printing usage prose and exiting 0.
func TestHumanHelpTopics(t *testing.T) {
	out, errS, code := runCLI(t, "help", "version")
	if code != 0 || errS != "" || !strings.Contains(out, "Usage") {
		t.Fatalf("resolvable topic: out=%q err=%q code=%d", out, errS, code)
	}
	out, errS, code = runCLI(t, "help", "bogus")
	if code != 2 || out != "" {
		t.Fatalf("unknown topic: out=%q code=%d", out, code)
	}
	if !strings.HasPrefix(errS, "docket: ") {
		t.Fatalf("unknown topic stderr = %q", errS)
	}
}

func TestHumanHelpIsolation(t *testing.T) {
	out, errS, code := runCLI(t, "--help")
	if code != 0 || errS != "" {
		t.Fatalf("err=%q code=%d", errS, code)
	}
	if !strings.Contains(out, "Usage") {
		t.Fatalf("human help must render Cobra text, got %q", out)
	}
}

func TestMissingCommand(t *testing.T) {
	out, errS, code := runCLI(t, "--json")
	if code != 2 || errS != "" {
		t.Fatalf("err=%q code=%d", errS, code)
	}
	if !strings.Contains(out, `"result":"invalid-input"`) {
		t.Fatalf("stdout = %q", out)
	}
}

func TestExtraArgumentRejected(t *testing.T) {
	_, _, code := runCLI(t, "version", "extra")
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	out, errS, code := runCLI(t, "version", "extra", "--json")
	if code != 2 || errS != "" || !strings.Contains(out, `"result":"invalid-input"`) {
		t.Fatalf("out=%q err=%q code=%d", out, errS, code)
	}

	// A command GROUP must reject an unknown positional the same way, and
	// name the offending token — reporting "missing command" for
	// `diagnostic runtimee` would misdirect, since a word was supplied.
	out, errS, code = runCLI(t, "diagnostic", "runtimee")
	if code != 2 || out != "" {
		t.Fatalf("group human: out=%q err=%q code=%d", out, errS, code)
	}
	if !strings.HasPrefix(errS, "docket: ") || !strings.Contains(errS, "runtimee") {
		t.Fatalf("group human: stderr = %q", errS)
	}
	if strings.Contains(errS, "missing command") {
		t.Fatalf("group human: misdirecting error for a supplied token: %q", errS)
	}

	out, errS, code = runCLI(t, "--json", "diagnostic", "runtimee")
	if code != 2 || errS != "" {
		t.Fatalf("group json: err=%q code=%d", errS, code)
	}
	if !strings.Contains(out, `"result":"invalid-input"`) || !strings.Contains(out, "runtimee") {
		t.Fatalf("group json: stdout = %q", out)
	}
	if strings.Contains(out, "missing command") {
		t.Fatalf("group json: misdirecting error for a supplied token: %q", out)
	}

	// The genuinely bare group still reports the missing command.
	_, errS, code = runCLI(t, "diagnostic")
	if code != 2 || !strings.Contains(errS, "missing command") {
		t.Fatalf("bare group: stderr=%q code=%d", errS, code)
	}
}

func TestUnknownCommandJSON(t *testing.T) {
	out, errS, code := runCLI(t, "--json", "bogus")
	if code != 2 || errS != "" || !strings.Contains(out, `"result":"invalid-input"`) {
		t.Fatalf("out=%q err=%q code=%d", out, errS, code)
	}
}

func TestNoCompletionCommand(t *testing.T) {
	_, _, code := runCLI(t, "completion")
	if code == 0 {
		t.Fatal("default completion command must be disabled")
	}
	out, _, _ := runCLI(t, "--help")
	if strings.Contains(out, "completion") {
		t.Fatalf("help must not advertise a completion command: %q", out)
	}
}

// Cobra registers __complete and __completeNoDesc unconditionally — before it
// consults CompletionOptions.DisableDefaultCmd — so disabling the visible
// `completion` command leaves both hidden spellings live. Unhandled, they write
// completion candidates to stdout and a directive line to stderr, bypassing the
// presenter entirely. Both must be ordinary unknown commands in both modes.
func TestHiddenCompletionCommandsRejected(t *testing.T) {
	for _, name := range []string{"__complete", "__completeNoDesc"} {
		out, errS, code := runCLI(t, name, "")
		if code != 2 || out != "" {
			t.Fatalf("human %s: out=%q err=%q code=%d", name, out, errS, code)
		}
		if !strings.HasPrefix(errS, "docket: ") || !strings.Contains(errS, name) {
			t.Fatalf("human %s: stderr = %q", name, errS)
		}

		out, errS, code = runCLI(t, "--json", name, "")
		if code != 2 {
			t.Fatalf("json %s: code = %d, want 2", name, code)
		}
		if errS != "" {
			t.Fatalf("json %s: stderr = %q, want empty", name, errS)
		}
		if !strings.Contains(out, `"result":"invalid-input"`) || !strings.Contains(out, `"reason":"invalid-arguments"`) {
			t.Fatalf("json %s: stdout = %q", name, out)
		}
		if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
			t.Fatalf("json %s: must be one newline-terminated document, got %q", name, out)
		}
	}
}

// pinGlobalConfig points the configuration reader's global-layer lookup at an
// empty temp directory. These tests run Run in this process, so without the pin
// they would consult the developer's own ~/.config/docket/config.yml.
func pinGlobalConfig(t *testing.T) {
	t.Helper()
	base := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(base, "xdg"))
	t.Setenv("HOME", filepath.Join(base, "home"))
}

// --repo-dir is required, so omitting it must fail as an argument error before
// any resolution is attempted.
func TestDiagnosticConfigRequiresRepoDir(t *testing.T) {
	pinGlobalConfig(t)
	out, errS, code := runCLI(t, "diagnostic", "config")
	if code != 2 || out != "" {
		t.Fatalf("out=%q err=%q code=%d", out, errS, code)
	}
	if !strings.HasPrefix(errS, "docket: ") || !strings.Contains(errS, "repo-dir") {
		t.Fatalf("stderr = %q", errS)
	}
}

// With the flag supplied, the subcommand reaches the operation and the
// presenter renders its document — the wiring assertion this package owns.
func TestDiagnosticConfigReachesOperation(t *testing.T) {
	pinGlobalConfig(t)
	out, errS, code := runCLI(t, "diagnostic", "config", "--repo-dir", t.TempDir(), "--default-branch", "main", "--json")
	if code != 0 || errS != "" {
		t.Fatalf("out=%q err=%q code=%d", out, errS, code)
	}
	if !strings.Contains(out, `"operation":"diagnostic.config"`) || !strings.Contains(out, `"result":"applied"`) {
		t.Fatalf("stdout = %q", out)
	}

	// --for-mutation selects the other operation over the same repository.
	out, errS, code = runCLI(t, "diagnostic", "config", "--repo-dir", t.TempDir(), "--default-branch", "main", "--for-mutation", "--json")
	if code != 0 || errS != "" {
		t.Fatalf("for-mutation: out=%q err=%q code=%d", out, errS, code)
	}
	if !strings.Contains(out, `"operation":"config.preflight"`) {
		t.Fatalf("for-mutation: stdout = %q", out)
	}
}

// pinInstallEnv points every root docket resolves — home, XDG config, XDG data
// — at a scratch directory, so an installation test can never read or write
// the developer's real home.
func pinInstallEnv(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	home := filepath.Join(base, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("XDG_BIN_HOME", filepath.Join(home, ".local", "bin"))
	return home
}

// TestInstallCommandsRegistered proves the three operations are reachable and
// that adding them left the help/--json conflict rule alone.
func TestInstallCommandsRegistered(t *testing.T) {
	out, errS, code := runCLI(t, "--help")
	if code != 0 || errS != "" {
		t.Fatalf("err=%q code=%d", errS, code)
	}
	for _, want := range []string{"install", "development"} {
		if !strings.Contains(out, want) {
			t.Errorf("root help does not list %q:\n%s", want, out)
		}
	}

	out, errS, code = runCLI(t, "install", "--help")
	if code != 0 || errS != "" {
		t.Fatalf("install help: err=%q code=%d", errS, code)
	}
	for _, want := range []string{"check", "--harness"} {
		if !strings.Contains(out, want) {
			t.Errorf("install help does not mention %q:\n%s", want, out)
		}
	}

	out, errS, code = runCLI(t, "development", "install", "--help")
	if code != 0 || errS != "" {
		t.Fatalf("development install help: err=%q code=%d", errS, code)
	}
	for _, want := range []string{"--source", "--bin-dir"} {
		if !strings.Contains(out, want) {
			t.Errorf("development install help does not mention %q:\n%s", want, out)
		}
	}

	// The conflict rule is unchanged for the new commands.
	out, errS, code = runCLI(t, "--json", "help", "install")
	if code != 2 || errS != "" {
		t.Fatalf("json help conflict: out=%q err=%q code=%d", out, errS, code)
	}
	if !strings.Contains(out, `"reason":"json-help-conflict"`) {
		t.Fatalf("json help conflict: stdout = %q", out)
	}
}

// TestInstallCheckWithoutInstallation is the wiring assertion: the command
// reaches the operation, and an unwritten machine answers invalid-state with
// the installation-required reason rather than an argument error.
func TestInstallCheckWithoutInstallation(t *testing.T) {
	pinInstallEnv(t)
	out, errS, code := runCLI(t, "install", "check", "--json")
	if code != 1 || errS != "" {
		t.Fatalf("out=%q err=%q code=%d", out, errS, code)
	}
	if !strings.Contains(out, `"operation":"install.check"`) ||
		!strings.Contains(out, `"result":"invalid-state"`) ||
		!strings.Contains(out, `"reason":"installation-required"`) {
		t.Fatalf("stdout = %q", out)
	}
}

// TestInstallIgnoresRepositoryLayer holds the spec's rule that installing is a
// user-level operation: a .docket.yml in the current directory is not a layer
// these commands have. The planted file is invalid enough to fail resolution,
// so loading it would turn this into an invalid-input document.
func TestInstallIgnoresRepositoryLayer(t *testing.T) {
	pinInstallEnv(t)
	cwd := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwd, ".docket.yml"), []byte("metadata_branch: [not, a, string]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(cwd)

	out, errS, code := runCLI(t, "install", "check", "--json")
	if code != 1 || errS != "" {
		t.Fatalf("out=%q err=%q code=%d", out, errS, code)
	}
	if !strings.Contains(out, `"reason":"installation-required"`) {
		t.Fatalf("a repository layer reached the operation: %q", out)
	}
}

func TestInstallRejectsUnknownHarness(t *testing.T) {
	pinInstallEnv(t)
	out, errS, code := runCLI(t, "install", "--harness", "emacs", "--json")
	if code != 2 || errS != "" {
		t.Fatalf("out=%q err=%q code=%d", out, errS, code)
	}
	if !strings.Contains(out, `"result":"invalid-input"`) || !strings.Contains(out, `"reason":"unknown-harness"`) {
		t.Fatalf("stdout = %q", out)
	}
}

func TestDevelopmentInstallRequiresSource(t *testing.T) {
	pinInstallEnv(t)
	_, errS, code := runCLI(t, "development", "install")
	if code != 2 || !strings.Contains(errS, "source") {
		t.Fatalf("err=%q code=%d", errS, code)
	}
}

func TestDevelopmentInstallWiresBothRunners(t *testing.T) {
	pinInstallEnv(t)
	bogus := filepath.Join(t.TempDir(), "not-a-checkout")
	out, errS, code := runCLI(t, "development", "install", "--source", bogus, "--json")
	// invalid-input results exit 2 (the convention every other install/flag
	// error test in this file asserts, e.g. TestVersionExtraArgsJSON).
	if code != 2 || errS != "" {
		t.Fatalf("out=%q err=%q code=%d", out, errS, code)
	}
	// invalid-source-root proves execution got PAST the runner nil-checks:
	// an unwired GoRunner or GitRunner would have refused as invalid-options.
	if !strings.Contains(out, `"reason":"invalid-source-root"`) {
		t.Fatalf("stdout = %q, want an invalid-source-root refusal (a runner is unwired if this reads invalid-options)", out)
	}
}

// captureTree runs a scratch command whose only job is to hand this test the
// Cobra tree the production wiring built.
func captureTree(t *testing.T) *cobra.Command {
	t.Helper()
	assetIndependent[treeWalkCommand] = true
	defer delete(assetIndependent, treeWalkCommand)

	var root *cobra.Command
	scratch := &cobra.Command{
		Use: treeWalkCommand,
		RunE: func(c *cobra.Command, _ []string) error {
			root = c.Root()
			return nil
		},
	}
	var out, errBuf bytes.Buffer
	run([]string{treeWalkCommand}, strings.NewReader(""), &out, &errBuf, devInfo(), hostFacts(), scratch)
	if root == nil {
		t.Fatalf("the scratch command never ran: out=%q err=%q", out.String(), errBuf.String())
	}
	return root
}

// TestAssetIndependentSetExact runs the correspondence both ways: every
// command in the tree is registered as asset-independent (nothing ships a
// gated command yet), and every registered name is a command that exists. A
// one-way check would let a stale entry hide a command that quietly became
// gated, or let a new command be gated by forgetfulness rather than by choice.
func TestAssetIndependentSetExact(t *testing.T) {
	root := captureTree(t)

	inTree := map[string]bool{}
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		key := commandKey(c)
		if key != treeWalkCommand {
			inTree[key] = true
		}
		// The asset-dependence guard rides on the ROOT's PersistentPreRunE,
		// and Cobra runs only the CLOSEST PersistentPreRun/PersistentPreRunE
		// in the chain — a subcommand that defines one of its own silently
		// replaces the guard rather than adding to it, disabling it for that
		// whole subtree with no test failing. Keeping the hook exclusive to
		// the root is what makes the guard tree-wide. (Deliberately not
		// solved with cobra.EnableTraverseRunHooks: that is a process-global
		// toggle, a broader behavior change than this guard needs.)
		if c != root {
			if c.PersistentPreRunE != nil || c.PersistentPreRun != nil {
				t.Errorf("command %q defines its own PersistentPreRun(E), which shadows the root's asset-dependence guard", key)
			}
		}
		for _, child := range c.Commands() {
			walk(child)
		}
	}
	walk(root)

	for key := range inTree {
		if !assetIndependent[key] {
			t.Errorf("command %q is not in the asset-independent set; register it or gate it deliberately", key)
		}
	}
	for key := range assetIndependent {
		if !inTree[key] {
			t.Errorf("asset-independent set names %q, which is no command in the tree", key)
		}
	}
}

// TestAssetDependentRefusal is the guard's mutation evidence: a command that is
// NOT in the independent set is refused before its body runs, with the
// installation-required reason, on a machine with no installation.
func TestAssetDependentRefusal(t *testing.T) {
	pinInstallEnv(t)
	ran := false
	gated := &cobra.Command{
		Use:  "gated",
		RunE: func(*cobra.Command, []string) error { ran = true; return nil },
	}
	var out, errBuf bytes.Buffer
	code := run([]string{"gated", "--json"}, strings.NewReader(""), &out, &errBuf, devInfo(), hostFacts(), gated)
	if ran {
		t.Fatal("the gated command's body ran without an installation")
	}
	if code != 1 || errBuf.String() != "" {
		t.Fatalf("out=%q err=%q code=%d", out.String(), errBuf.String(), code)
	}
	if !strings.Contains(out.String(), `"result":"invalid-state"`) ||
		!strings.Contains(out.String(), `"reason":"installation-required"`) ||
		!strings.Contains(out.String(), `"operation":"gated"`) {
		t.Fatalf("stdout = %q", out.String())
	}
}

// statusGit runs real git with -C <dir> and fails the test on a non-zero exit.
func statusGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		t.Fatalf("git -C %s %s: %v: %s", dir, strings.Join(args, " "), err, errBuf.String())
	}
}

// statusWriteFile writes content (creating parents) at a repo-relative path.
func statusWriteFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newStatusFixtureRepo builds a minimal main-mode topology — a bare file
// origin plus an invocation clone — carrying a .docket.yml and one active
// change, and returns the invocation clone path for use as --repo-dir. It also
// isolates the global configuration layer to an empty XDG dir so a developer's
// own config cannot steer resolution. It skips when git is absent.
func newStatusFixtureRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found on PATH")
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	writer := filepath.Join(root, "writer")
	invocation := filepath.Join(root, "invocation")

	statusGit(t, root, "init", "--bare", "-b", "main", origin)
	statusGit(t, root, "init", "-b", "main", writer)
	statusGit(t, writer, "config", "user.name", "t")
	statusGit(t, writer, "config", "user.email", "t@t")
	statusGit(t, writer, "config", "commit.gpgsign", "false")

	statusWriteFile(t, writer, ".docket.yml", "metadata_branch: main\n")
	statusWriteFile(t, writer, "README.md", "readme\n")
	statusWriteFile(t, writer, "docs/changes/active/0001-alpha.md",
		"---\nid: 1\nslug: alpha\ntitle: Alpha\nstatus: proposed\npriority: high\ntype: feat\ncreated: 2026-01-02\n---\n\nBody of alpha.\n")
	statusGit(t, writer, "add", "-A")
	statusGit(t, writer, "commit", "-q", "-m", "main content")
	statusGit(t, writer, "remote", "add", "origin", origin)
	statusGit(t, writer, "push", "-q", "-u", "origin", "main")

	statusGit(t, root, "clone", "-q", origin, invocation)
	return invocation
}

// TestStatusRejectsInvalidPriority is a wiring assertion: the CLI hands the
// flag through to app.Status, whose closed-value check against the resolved
// configuration rejects a bogus priority as invalid input — exit 2, one JSON
// document naming the operation.
func TestStatusRejectsInvalidPriority(t *testing.T) {
	repo := newStatusFixtureRepo(t)
	out, errS, code := runCLI(t, "status", "--priority", "bogus", "--repo-dir", repo, "--json")
	if code != 2 || errS != "" {
		t.Fatalf("out=%q err=%q code=%d", out, errS, code)
	}
	if !strings.Contains(out, `"operation":"status"`) || !strings.Contains(out, `"result":"invalid-input"`) {
		t.Fatalf("stdout = %q", out)
	}
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("must be one newline-terminated document, got %q", out)
	}
}

// TestStatusOutsideRepository is a wiring assertion: pointed at a directory that
// is not a Git repository, the operation returns invalid-input. In JSON mode the
// one document lands on stdout with stderr empty; in human mode the document
// still lands on stdout and stderr stays empty, per the presenter contract for a
// failing operation the CLI reached.
func TestStatusOutsideRepository(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found on PATH")
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	bare := t.TempDir()

	out, errS, code := runCLI(t, "status", "--repo-dir", bare, "--json")
	if code != 2 || errS != "" {
		t.Fatalf("json: out=%q err=%q code=%d", out, errS, code)
	}
	if !strings.Contains(out, `"operation":"status"`) || !strings.Contains(out, `"result":"invalid-input"`) {
		t.Fatalf("json: stdout = %q", out)
	}

	out, errS, code = runCLI(t, "status", "--repo-dir", bare)
	if code != 2 || errS != "" {
		t.Fatalf("human: out=%q err=%q code=%d", out, errS, code)
	}
	if out == "" {
		t.Fatalf("human: the failing operation's document must land on stdout, got empty")
	}
}

// TestStatusReachesOperationJSON is the wiring assertion for the success path:
// pointed at a minimal fixture repository, the command reaches the operation and
// the presenter renders exactly one protocol-v1 status document with the
// contract's arrays present.
func TestStatusReachesOperationJSON(t *testing.T) {
	repo := newStatusFixtureRepo(t)
	out, errS, code := runCLI(t, "status", "--repo-dir", repo, "--json")
	if code != 0 || errS != "" {
		t.Fatalf("out=%q err=%q code=%d", out, errS, code)
	}
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("must be exactly one newline-terminated document, got %q", out)
	}
	for _, want := range []string{
		`"operation":"status"`,
		`"protocol_version":1`,
		`"result":"applied"`,
		`"changes":`,
		`"ready":`,
		`"records":`,
		`"findings":`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("status document missing %s: %s", want, out)
		}
	}
}

// writeInstallState publishes a minimal installation record for the pinned
// roots, so a guard test can drive the half of RequireCompatibleInstallation
// that only a PRESENT installation reaches.
func writeInstallState(t *testing.T, protocol int) {
	t.Helper()
	roots, err := install.ResolveRoots(os.UserHomeDir, os.Getenv)
	if err != nil {
		t.Fatalf("ResolveRoots: %v", err)
	}
	err = install.WriteStateAtomic(roots.StatePath(), &install.State{
		FormatVersion:  install.StateFormatVersion,
		ProductVersion: "0.1.0-dev",
		AssetProtocol:  protocol,
		AssetSetID:     "sha256:pinned",
		Mode:           install.ModeRelease,
		Harnesses:      []string{"claude"},
		AgentDigest:    "sha256:agents",
	})
	if err != nil {
		t.Fatalf("WriteStateAtomic: %v", err)
	}
}

// TestAssetDependentProtocolMismatch is the guard's second half: an
// installation that EXISTS but speaks another asset protocol refuses an
// asset-dependent command with its own reason, distinct from the
// nothing-installed refusal. The matching-protocol case runs the body, which
// is what proves the protocol comparison — not the mere presence of a state
// file — is what decides.
func TestAssetDependentProtocolMismatch(t *testing.T) {
	t.Run("mismatch refuses", func(t *testing.T) {
		pinInstallEnv(t)
		writeInstallState(t, assets.AssetProtocol+1)

		ran := false
		gated := &cobra.Command{
			Use:  "gated",
			RunE: func(*cobra.Command, []string) error { ran = true; return nil },
		}
		var out, errBuf bytes.Buffer
		code := run([]string{"gated", "--json"}, strings.NewReader(""), &out, &errBuf, devInfo(), hostFacts(), gated)
		if ran {
			t.Fatal("the gated command's body ran against an incompatible installation")
		}
		if code != 1 || errBuf.String() != "" {
			t.Fatalf("out=%q err=%q code=%d", out.String(), errBuf.String(), code)
		}
		if !strings.Contains(out.String(), `"result":"invalid-state"`) ||
			!strings.Contains(out.String(), `"reason":"asset-protocol-mismatch"`) ||
			!strings.Contains(out.String(), `"operation":"gated"`) {
			t.Fatalf("stdout = %q", out.String())
		}
	})

	t.Run("matching protocol admits", func(t *testing.T) {
		pinInstallEnv(t)
		writeInstallState(t, assets.AssetProtocol)

		ran := false
		gated := &cobra.Command{
			Use:  "gated",
			RunE: func(*cobra.Command, []string) error { ran = true; return nil },
		}
		var out, errBuf bytes.Buffer
		code := run([]string{"gated"}, strings.NewReader(""), &out, &errBuf, devInfo(), hostFacts(), gated)
		if !ran {
			t.Fatalf("a compatible installation still refused the command: out=%q err=%q code=%d",
				out.String(), errBuf.String(), code)
		}
	})
}
