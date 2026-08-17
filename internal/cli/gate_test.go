package cli

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/app"
)

// TestMain routes the supervisor re-exec role of the cli test binary: a real
// gate launch re-executes this binary with the private supervisor env var set,
// and it must become the durable supervisor rather than re-running the test
// suite. Ordinary `go test` runs set neither var and fall through to m.Run.
// This uses app.MaybeRunGateSupervisor so the test never imports
// internal/process — the boundary this task guards.
func TestMain(m *testing.M) {
	if code, ok := app.MaybeRunGateSupervisor(); ok {
		os.Exit(code)
	}
	os.Exit(m.Run())
}

// gateTempDir is a temp dir whose cleanup tolerates the external supervisor's
// brief exit window. Observe reports "passed" the instant the terminal record
// lands, which can precede the supervisor's final same-directory atomic write
// and lock release, so a single-shot RemoveAll (as t.TempDir does) races it and
// fails "directory not empty". The retry loop lets the supervisor finish.
func gateTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "docket-gate-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for i := 0; i < 40; i++ {
			if err := os.RemoveAll(dir); err == nil {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
		_ = os.RemoveAll(dir)
	})
	return dir
}

// decodeOneJSON proves stdout is exactly one newline-terminated JSON document
// and returns it decoded, mirroring the cmd/docket harness.
func decodeOneJSON(t *testing.T, stdout string) map[string]any {
	t.Helper()
	if !strings.HasSuffix(stdout, "\n") || strings.Count(stdout, "\n") != 1 {
		t.Fatalf("want exactly one newline-terminated document, got %q", stdout)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSuffix(stdout, "\n")), &doc); err != nil {
		t.Fatalf("decoding %q: %v", stdout, err)
	}
	return doc
}

// TestGateGroupMissingCommand: `docket gate` behaves like `docket diagnostic` —
// invalid input, error on stderr in human mode, one JSON document in JSON mode.
func TestGateGroupMissingCommand(t *testing.T) {
	_, errS, code := runCLI(t, "gate")
	if code != 2 || !strings.Contains(errS, "missing command") {
		t.Fatalf("human bare group: err=%q code=%d", errS, code)
	}

	out, errS, code := runCLI(t, "--json", "gate")
	if code != 2 || errS != "" {
		t.Fatalf("json bare group: err=%q code=%d", errS, code)
	}
	doc := decodeOneJSON(t, out)
	if doc["result"] != "invalid-input" || doc["operation"] != "cli" {
		t.Fatalf("json bare group doc=%v", doc)
	}

	// An unknown subcommand names the offending token, never "missing command".
	out, errS, code = runCLI(t, "--json", "gate", "bogus")
	if code != 2 || errS != "" {
		t.Fatalf("json unknown sub: err=%q code=%d", errS, code)
	}
	if !strings.Contains(out, "bogus") || strings.Contains(out, "missing command") {
		t.Fatalf("json unknown sub: misdirecting doc=%q", out)
	}
}

// TestGateLaunchRequiresDashBoundary: the `--` argv boundary is mandatory, and
// no positional words may precede it.
func TestGateLaunchRequiresDashBoundary(t *testing.T) {
	root := gateTempDir(t)
	// No `--` at all: invalid input naming the requirement.
	out, errS, code := runCLI(t, "gate", "launch", "--root", root, "--cwd", root)
	if code != 2 || out != "" {
		t.Fatalf("no dash human: out=%q err=%q code=%d", out, errS, code)
	}
	if !strings.Contains(errS, "--") {
		t.Fatalf("no dash human: message does not name the -- contract: %q", errS)
	}
	// A positional word before `--` is rejected too. `/bin/echo` sits before the
	// separator, so under the correct guard this is exit 2; a mutation that
	// treats all args as argv would instead launch /bin/echo and exit 0.
	_, _, code = runCLI(t, "gate", "launch", "--root", root, "--cwd", root, "/bin/echo", "--", "hi")
	if code != 2 {
		t.Fatalf("positional-before-dash: code=%d, want 2", code)
	}
	out, errS, code = runCLI(t, "--json", "gate", "launch", "--root", root, "--cwd", root, "/bin/echo", "--", "hi")
	if code != 2 || errS != "" {
		t.Fatalf("positional-before-dash json: err=%q code=%d", errS, code)
	}
	doc := decodeOneJSON(t, out)
	if doc["result"] != "invalid-input" {
		t.Fatalf("positional-before-dash json: doc=%v", doc)
	}
}

// pollObserveJSON drives `gate observe --json <runDir>` until the run reaches a
// terminal state or the generous deadline elapses, returning the last document.
func pollObserveJSON(t *testing.T, runDir string) map[string]any {
	t.Helper()
	for i := 0; i < 300; i++ {
		out, errS, code := runCLI(t, "--json", "gate", "observe", runDir)
		if errS != "" {
			t.Fatalf("observe stderr=%q", errS)
		}
		doc := decodeOneJSON(t, out)
		state, _ := doc["state"].(string)
		if state != "running" {
			// terminal (passed/failed/…) — code carries the mapped exit.
			_ = code
			return doc
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("run never became terminal")
	return nil
}

// TestGateLaunchJSONOneDocument drives a real supervised /bin/echo through the
// CLI and proves the launch + observe protocol documents.
func TestGateLaunchJSONOneDocument(t *testing.T) {
	root := gateTempDir(t)
	cwd := gateTempDir(t)
	out, errS, code := runCLI(t, "--json", "gate", "launch", "--root", root, "--cwd", cwd, "--", "/bin/echo", "hi")
	if code != 0 || errS != "" {
		t.Fatalf("launch: out=%q err=%q code=%d", out, errS, code)
	}
	doc := decodeOneJSON(t, out)
	if doc["operation"] != "gate.launch" || doc["result"] != "applied" {
		t.Fatalf("launch doc=%v", doc)
	}
	runDir, _ := doc["run_dir"].(string)
	if runDir == "" {
		t.Fatalf("launch produced no run_dir: %v", doc)
	}

	obs := pollObserveJSON(t, runDir)
	if obs["operation"] != "gate.observe" || obs["state"] != "passed" {
		t.Fatalf("observe doc=%v", obs)
	}
	// exit_code decodes as a float64 through map[string]any; it must be exactly 0.
	ec, ok := obs["exit_code"].(float64)
	if !ok || ec != 0 {
		t.Fatalf("observe exit_code=%v (%T), want 0", obs["exit_code"], obs["exit_code"])
	}
}

// TestGateStopAndRecoverWiring proves stop and recover reach the app layer and
// carry their protocol documents.
func TestGateStopAndRecoverWiring(t *testing.T) {
	root := gateTempDir(t)
	cwd := gateTempDir(t)
	out, errS, code := runCLI(t, "--json", "gate", "launch", "--root", root, "--cwd", cwd, "--", "/bin/echo", "hi")
	if code != 0 || errS != "" {
		t.Fatalf("launch: out=%q err=%q code=%d", out, errS, code)
	}
	runDir, _ := decodeOneJSON(t, out)["run_dir"].(string)
	if runDir == "" {
		t.Fatalf("launch produced no run_dir")
	}
	// Drive to terminal so stop is an already-terminal no-op.
	passed := pollObserveJSON(t, runDir)
	if passed["state"] != "passed" {
		t.Fatalf("precondition: run not passed: %v", passed)
	}

	// stop on a passed run -> no-op, state preserved (0 exit).
	out, errS, code = runCLI(t, "--json", "gate", "stop", runDir, "--reason", "test")
	if code != 0 || errS != "" {
		t.Fatalf("stop: out=%q err=%q code=%d", out, errS, code)
	}
	stopDoc := decodeOneJSON(t, out)
	if stopDoc["operation"] != "gate.stop" || stopDoc["result"] != "no-op" {
		t.Fatalf("stop doc=%v", stopDoc)
	}
	if stopDoc["state"] != "passed" {
		t.Fatalf("stop did not preserve state: %v", stopDoc)
	}

	// recover --root <launch-root>: the passed run is terminal, so it is
	// retained (marked 0) and reported as a recovery entry — no-op, and the
	// recovery field is a populated JSON array.
	out, errS, code = runCLI(t, "--json", "gate", "recover", "--root", root)
	if code != 0 || errS != "" {
		t.Fatalf("recover: out=%q err=%q code=%d", out, errS, code)
	}
	recDoc := decodeOneJSON(t, out)
	if recDoc["operation"] != "gate.recover" || recDoc["result"] != "no-op" {
		t.Fatalf("recover doc=%v", recDoc)
	}
	if _, ok := recDoc["recovery"].([]any); !ok {
		t.Fatalf("recover: recovery is not a JSON array: %v", recDoc["recovery"])
	}

	// recover --root <empty>: the nil-collection convention marshals an empty
	// scan as "recovery":[], never an absent field.
	out, errS, code = runCLI(t, "--json", "gate", "recover", "--root", gateTempDir(t))
	if code != 0 || errS != "" {
		t.Fatalf("recover empty: out=%q err=%q code=%d", out, errS, code)
	}
	if !strings.Contains(out, `"recovery":[]`) {
		t.Fatalf("recover empty: missing empty recovery array: %q", out)
	}
}

// TestCLIDoesNotImportProcess is the second half of the import-boundary check
// (Task 1 owns the first): no internal/cli production file may import
// internal/process. Same go/parser shape as the process-side guard, with a
// population floor.
func TestCLIDoesNotImportProcess(t *testing.T) {
	const forbidden = "github.com/danielhanold/docket/internal/process"
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	checked := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(".", name), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		checked++
		for _, imp := range f.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if path == forbidden {
				t.Errorf("%s imports %q — internal/cli must never import internal/process", name, path)
			}
		}
	}
	if checked < 5 {
		t.Fatalf("population floor: only %d production files checked — the guard is scanning the wrong directory", checked)
	}
}
