package cli

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
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

// gitCmd runs one git subcommand in dir, failing the test on any error.
func gitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// gateDriveRepo initializes a real, committed git worktree the gate driver can
// fingerprint and root its durable store beside. A drive needs a discoverable,
// non-bare repository with a resolvable HEAD.
func gateDriveRepo(t *testing.T) string {
	t.Helper()
	dir := gateTempDir(t)
	gitCmd(t, dir, "init", "-q", "-b", "main")
	gitCmd(t, dir, "config", "user.email", "t@t")
	gitCmd(t, dir, "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(dir, "seed"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, dir, "add", "seed")
	gitCmd(t, dir, "commit", "-q", "-m", "seed")
	return dir
}

// driveDoc pulls the shared `drive` sub-document out of a decoded protocol doc,
// failing when a successful operation carried none.
func driveDoc(t *testing.T, doc map[string]any) map[string]any {
	t.Helper()
	d, ok := doc["drive"].(map[string]any)
	if !ok {
		t.Fatalf("no drive sub-document: %v", doc)
	}
	return d
}

// TestGateDriveStartRunsToPassed proves `gate drive start -- <argv>` composes the
// same state machine as the app seam through the CLI: the argv after `--` is the
// suite command, and a fast green command returns a doc carrying a drive id and
// the PASSED outcome at exit 0.
func TestGateDriveStartRunsToPassed(t *testing.T) {
	wt := gateDriveRepo(t)
	root := gateTempDir(t)
	out, errS, code := runCLI(t, "--json", "gate", "drive", "start", "--repo-dir", wt, "--run-root", root, "--", "/bin/echo", "hi")
	if code != 0 || errS != "" {
		t.Fatalf("start: out=%q err=%q code=%d", out, errS, code)
	}
	doc := decodeOneJSON(t, out)
	if doc["operation"] != "gate.drive.start" || doc["result"] != "applied" {
		t.Fatalf("start envelope: %v", doc)
	}
	d := driveDoc(t, doc)
	if id, _ := d["drive_id"].(string); id == "" {
		t.Fatalf("start produced no drive id: %v", d)
	}
	if d["outcome"] != "PASSED" {
		t.Fatalf("start outcome=%v, want PASSED", d["outcome"])
	}
}

// TestGateDriveStartFailedIsNonZeroExit is the exit-code guard for the Task 9
// residual risk: the shared envelope result is `applied` for a FAILED verdict, so
// a CLI that keyed its exit on ExitCode(result) would report a red suite as
// success (exit 0). The process exit MUST derive from the typed outcome instead.
func TestGateDriveStartFailedIsNonZeroExit(t *testing.T) {
	wt := gateDriveRepo(t)
	root := gateTempDir(t)
	out, errS, code := runCLI(t, "--json", "gate", "drive", "start", "--repo-dir", wt, "--run-root", root, "--", "/usr/bin/false")
	if errS != "" {
		t.Fatalf("start: err=%q", errS)
	}
	doc := decodeOneJSON(t, out)
	// The shared protocol document is unchanged: a FAILED verdict still rides in
	// an `applied` envelope, and the JSON consumer keys on drive.outcome.
	if doc["result"] != "applied" {
		t.Fatalf("shared doc envelope changed: %v", doc)
	}
	if driveDoc(t, doc)["outcome"] != "FAILED" {
		t.Fatalf("outcome=%v, want FAILED", driveDoc(t, doc)["outcome"])
	}
	if code == 0 {
		t.Fatalf("a FAILED verdict must not exit 0 (blind ExitCode(applied) success)")
	}
}

// TestGateDriveAdvanceHandoffClaim proves advance resumes the same durable drive
// and that handoff then claim transfer ownership: handoff mints a fresh single-use
// token, and claim consumes it for a fresh owner generation, all keyed on opaque
// drive/claim identifiers across separate short-lived CLI invocations.
func TestGateDriveAdvanceHandoffClaim(t *testing.T) {
	wt := gateDriveRepo(t)
	root := gateTempDir(t)
	out, _, code := runCLI(t, "--json", "gate", "drive", "start", "--repo-dir", wt, "--run-root", root, "--", "/bin/echo", "hi")
	if code != 0 {
		t.Fatalf("start failed: %q", out)
	}
	start := driveDoc(t, decodeOneJSON(t, out))
	id, _ := start["drive_id"].(string)
	gen, _ := start["generation"].(string)
	if id == "" || gen == "" {
		t.Fatalf("start doc missing id/generation: %v", start)
	}

	// advance resumes the same drive; a terminal drive is idempotent.
	out, _, code = runCLI(t, "--json", "gate", "drive", "advance", "--repo-dir", wt, "--drive-id", id, "--owner-gen", gen)
	if code != 0 {
		t.Fatalf("advance failed: %q", out)
	}
	adv := driveDoc(t, decodeOneJSON(t, out))
	if adv["drive_id"] != id || adv["outcome"] != "PASSED" {
		t.Fatalf("advance did not resume the same drive: %v", adv)
	}

	// handoff invalidates the owner and mints a single-use transfer token.
	out, _, code = runCLI(t, "--json", "gate", "drive", "handoff", "--repo-dir", wt, "--drive-id", id, "--owner-gen", gen)
	if code != 0 {
		t.Fatalf("handoff failed: %q", out)
	}
	token, _ := driveDoc(t, decodeOneJSON(t, out))["generation"].(string)
	if token == "" || token == gen {
		t.Fatalf("handoff did not mint a fresh token: %q (owner %q)", token, gen)
	}

	// claim consumes the token for a fresh owner generation.
	out, _, code = runCLI(t, "--json", "gate", "drive", "claim", "--repo-dir", wt, "--drive-id", id, "--handoff-id", token)
	if code != 0 {
		t.Fatalf("claim failed: %q", out)
	}
	claimed := driveDoc(t, decodeOneJSON(t, out))
	newOwner, _ := claimed["generation"].(string)
	if newOwner == "" || newOwner == token {
		t.Fatalf("claim did not mint a fresh owner: %q (token %q)", newOwner, token)
	}
	if claimed["drive_id"] != id {
		t.Fatalf("claim reported a different drive: %v", claimed)
	}
}

// TestGateDriveStartRequiresDashBoundary proves an argument-parse failure is a
// command failure — invalid-input, no workflow document — never a drive outcome,
// and that no drive is launched before the boundary is enforced.
func TestGateDriveStartRequiresDashBoundary(t *testing.T) {
	wt := gateDriveRepo(t)
	root := gateTempDir(t)
	_, _, code := runCLI(t, "gate", "drive", "start", "--repo-dir", wt, "--run-root", root)
	if code != 2 {
		t.Fatalf("no dash: code=%d, want 2", code)
	}
	out, errS, code := runCLI(t, "--json", "gate", "drive", "start", "--repo-dir", wt, "--run-root", root)
	if code != 2 || errS != "" {
		t.Fatalf("no dash json: err=%q code=%d", errS, code)
	}
	doc := decodeOneJSON(t, out)
	if doc["result"] != "invalid-input" {
		t.Fatalf("parse failure result=%v, want invalid-input", doc["result"])
	}
	if _, hasDrive := doc["drive"]; hasDrive {
		t.Fatalf("a parse failure must not carry a workflow drive document: %v", doc)
	}
	// A positional word before `--` is rejected too.
	_, _, code = runCLI(t, "gate", "drive", "start", "--repo-dir", wt, "--run-root", root, "before", "--", "/bin/echo")
	if code != 2 {
		t.Fatalf("positional-before-dash: code=%d, want 2", code)
	}
}

// TestGateDriveStartNoArgvLeak proves the protocol output never leaks the suite
// argv (redaction): the drive document carries identity and outcome, never the
// command words, in either JSON or human mode.
func TestGateDriveStartNoArgvLeak(t *testing.T) {
	wt := gateDriveRepo(t)
	root := gateTempDir(t)
	const secret = "SENTINEL_no_leak_TOKEN_98217"
	out, _, code := runCLI(t, "--json", "gate", "drive", "start", "--repo-dir", wt, "--run-root", root, "--", "/bin/echo", secret)
	if code != 0 {
		t.Fatalf("start failed: %q", out)
	}
	if strings.Contains(out, secret) {
		t.Fatalf("suite argv leaked into protocol JSON: %q", out)
	}
	out, _, code = runCLI(t, "gate", "drive", "start", "--repo-dir", wt, "--run-root", root, "--", "/bin/echo", secret)
	if code != 0 {
		t.Fatalf("start (human) failed: %q", out)
	}
	if strings.Contains(out, secret) {
		t.Fatalf("suite argv leaked into human output: %q", out)
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
