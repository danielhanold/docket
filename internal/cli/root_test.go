package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/buildinfo"
)

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
