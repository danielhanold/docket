package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var binPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "docket-bin-*")
	if err != nil {
		panic(err)
	}
	binPath = filepath.Join(dir, "docket")
	out, err := exec.Command("go", "build", "-o", binPath, ".").CombinedOutput()
	if err != nil {
		panic("building test binary: " + err.Error() + "\n" + string(out))
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func run(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errBuf bytes.Buffer
	cmd := exec.Command(binPath, args...)
	cmd.Stdout, cmd.Stderr = &out, &errBuf
	err := cmd.Run()
	code = 0
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("running %v: %v", args, err)
		}
		code = ee.ExitCode()
	}
	return out.String(), errBuf.String(), code
}

// assertOneJSONDocument decodes stdout and proves it is exactly one complete
// JSON value with one trailing newline, returning the decoded object.
func assertOneJSONDocument(t *testing.T, stdout string) map[string]any {
	t.Helper()
	if !strings.HasSuffix(stdout, "\n") || strings.Count(stdout, "\n") != 1 {
		t.Fatalf("want exactly one newline-terminated document, got %q", stdout)
	}
	dec := json.NewDecoder(strings.NewReader(stdout))
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		t.Fatalf("decoding %q: %v", stdout, err)
	}
	var second any
	if err := dec.Decode(&second); err != io.EOF {
		t.Fatalf("stdout carries a second JSON value: %q", stdout)
	}
	return doc
}

func TestVersionTextGolden(t *testing.T) {
	out, errS, code := run(t, "version")
	if code != 0 || errS != "" {
		t.Fatalf("err=%q code=%d", errS, code)
	}
	if out != "docket development (commit unknown, built unknown)\n" {
		t.Fatalf("stdout = %q", out)
	}
}

func TestVersionJSONGoldenBytes(t *testing.T) {
	out, errS, code := run(t, "version", "--json")
	if code != 0 || errS != "" {
		t.Fatalf("err=%q code=%d", errS, code)
	}
	want := `{"protocol_version":1,"operation":"version","result":"applied","version":"development","commit":"unknown","build_date":"unknown"}` + "\n"
	if out != want {
		t.Fatalf("stdout = %q, want %q", out, want)
	}
	assertOneJSONDocument(t, out)
}

// pflag's Bool accepts every strconv.ParseBool spelling, so the built binary
// must honor --json=1 and --json=TRUE as protocol mode — the pre-scan's
// narrower grammar only stands in when parsing never reaches the flag.
func TestBoundJSONFlagSpellingsSubprocess(t *testing.T) {
	for _, args := range [][]string{
		{"--json=1", "version"},
		{"--json=TRUE", "version"},
		{"version", "--json=t"},
	} {
		out, errS, code := run(t, args...)
		if code != 0 || errS != "" {
			t.Fatalf("args=%v err=%q code=%d", args, errS, code)
		}
		doc := assertOneJSONDocument(t, out)
		if doc["operation"] != "version" || doc["result"] != "applied" {
			t.Fatalf("args=%v doc=%v", args, doc)
		}
	}
	for _, args := range [][]string{
		{"--json=0", "version"},
		{"version", "--json=F"},
	} {
		out, errS, code := run(t, args...)
		if code != 0 || errS != "" {
			t.Fatalf("args=%v err=%q code=%d", args, errS, code)
		}
		if out != "docket development (commit unknown, built unknown)\n" {
			t.Fatalf("args=%v stdout = %q", args, out)
		}
	}
}

func TestDiagnosticRuntimeReflectsHost(t *testing.T) {
	out, errS, code := run(t, "diagnostic", "runtime", "--json")
	if code != 0 || errS != "" {
		t.Fatalf("err=%q code=%d", errS, code)
	}
	doc := assertOneJSONDocument(t, out)
	// The subprocess runs on this same host, so its facts equal ours.
	if doc["go_version"] != runtime.Version() || doc["go_os"] != runtime.GOOS || doc["go_arch"] != runtime.GOARCH {
		t.Fatalf("runtime facts diverge from host: %v", doc)
	}
	if doc["protocol_version"] != float64(1) || doc["operation"] != "diagnostic.runtime" || doc["result"] != "applied" {
		t.Fatalf("envelope wrong: %v", doc)
	}
}

func TestInjectedBuildIdentity(t *testing.T) {
	injected := filepath.Join(t.TempDir(), "docket-injected")
	ldflags := "-X github.com/danielhanold/docket/internal/buildinfo.Version=1.2.3" +
		" -X github.com/danielhanold/docket/internal/buildinfo.Commit=abc1234" +
		" -X github.com/danielhanold/docket/internal/buildinfo.BuildDate=2026-08-13"
	if out, err := exec.Command("go", "build", "-ldflags", ldflags, "-o", injected, ".").CombinedOutput(); err != nil {
		t.Fatalf("injected build failed: %v\n%s", err, out)
	}
	var stdout bytes.Buffer
	cmd := exec.Command(injected, "version")
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "docket 1.2.3 (commit abc1234, built 2026-08-13)\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestJSONErrorCasesOneDocumentEmptyStderr(t *testing.T) {
	cases := [][]string{
		{"--json", "bogus"},              // unknown command
		{"--json", "version", "--bogus"}, // unknown flag after the mode flag
		{"version", "--bogus", "--json"}, // unknown flag with --json AFTER the failing token
		{"--json", "version", "extra"},   // extra argument
		{"--json"},                       // missing command
	}
	for _, args := range cases {
		out, errS, code := run(t, args...)
		if code != 2 {
			t.Fatalf("args=%v code=%d, want 2", args, code)
		}
		if errS != "" {
			t.Fatalf("args=%v stderr=%q, want empty", args, errS)
		}
		doc := assertOneJSONDocument(t, out)
		if doc["result"] != "invalid-input" || doc["operation"] != "cli" {
			t.Fatalf("args=%v doc=%v", args, doc)
		}
	}
}

// Cobra's hidden completion commands are registered before it honors
// CompletionOptions.DisableDefaultCmd, so they survive disabling the visible
// `completion` command and would otherwise write shell-completion text to
// stdout and a directive line to stderr, outside the presenter.
func TestHiddenCompletionCommandsRejectedSubprocess(t *testing.T) {
	for _, name := range []string{"__complete", "__completeNoDesc"} {
		out, errS, code := run(t, "--json", name, "")
		if code != 2 {
			t.Fatalf("json %s: code = %d, want 2", name, code)
		}
		if errS != "" {
			t.Fatalf("json %s: stderr = %q, want empty", name, errS)
		}
		doc := assertOneJSONDocument(t, out)
		if doc["result"] != "invalid-input" || doc["operation"] != "cli" || doc["reason"] != "invalid-arguments" {
			t.Fatalf("json %s: doc = %v", name, doc)
		}

		out, errS, code = run(t, name, "")
		if code != 2 || out != "" {
			t.Fatalf("human %s: out=%q code=%d", name, out, code)
		}
		if !strings.HasPrefix(errS, "docket: ") || !strings.Contains(errS, name) {
			t.Fatalf("human %s: stderr = %q", name, errS)
		}
	}
}

func TestHumanParseErrorStderrOnly(t *testing.T) {
	out, errS, code := run(t, "version", "--bogus")
	if code != 2 || out != "" {
		t.Fatalf("out=%q code=%d", out, code)
	}
	if !strings.HasPrefix(errS, "docket: ") {
		t.Fatalf("stderr = %q", errS)
	}
}

func TestHelpConflictAndHumanHelp(t *testing.T) {
	for _, args := range [][]string{
		{"--json", "--help"},
		{"--json", "-h"},
		{"--json", "help"},
		{"--json", "help", "bogus"},      // unresolvable topic
		{"--json", "help", "completion"}, // disabled built-in, so also unresolvable
		{"--json", "help", "version"},    // resolvable topic
		{"--json=1", "--help"},           // Bool spelling outside the pre-scan grammar
		{"--json=TRUE", "help"},
	} {
		out, errS, code := run(t, args...)
		if code != 2 || errS != "" {
			t.Fatalf("args=%v err=%q code=%d", args, errS, code)
		}
		doc := assertOneJSONDocument(t, out)
		if doc["reason"] != "json-help-conflict" {
			t.Fatalf("args=%v doc=%v", args, doc)
		}
	}
	out, errS, code := run(t, "--help")
	if code != 0 || errS != "" || !strings.Contains(out, "Usage") {
		t.Fatalf("human help: err=%q code=%d out=%q", errS, code, out)
	}
	if strings.Contains(out, "completion") {
		t.Fatalf("help advertises completion: %q", out)
	}
}

func TestCrossCompileApprovedTargets(t *testing.T) {
	// Buildability gate only: the four tuples must compile with CGO off.
	// Foreign binaries are never executed (change 0317 owns on-target runs).
	tuples := [][2]string{{"darwin", "amd64"}, {"darwin", "arm64"}, {"linux", "amd64"}, {"linux", "arm64"}}
	dir := t.TempDir()
	for _, tp := range tuples {
		out := filepath.Join(dir, "docket-"+tp[0]+"-"+tp[1])
		cmd := exec.Command("go", "build", "-o", out, ".")
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+tp[0], "GOARCH="+tp[1])
		if msg, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cross-build %s/%s failed: %v\n%s", tp[0], tp[1], err, msg)
		}
		if fi, err := os.Stat(out); err != nil || fi.Size() == 0 {
			t.Fatalf("cross-build %s/%s produced no binary", tp[0], tp[1])
		}
	}
}
