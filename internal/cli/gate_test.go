package cli

import (
	"crypto/sha256"
	"encoding/hex"
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
	"github.com/danielhanold/docket/internal/testsupport"
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
	root := testsupport.TempDir(t)
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
	root := testsupport.TempDir(t)
	cwd := testsupport.TempDir(t)
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
	root := testsupport.TempDir(t)
	cwd := testsupport.TempDir(t)
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
	out, errS, code = runCLI(t, "--json", "gate", "recover", "--root", testsupport.TempDir(t))
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
	// Point this directly-spawned git at testsupport.GitEnv's background-off
	// GIT_CONFIG_GLOBAL so a detached gc/maintenance/fsmonitor child cannot
	// outlive the test and race the fixture's RemoveAll into "directory not
	// empty" under parallel load (change 0373).
	cmd.Env = append(os.Environ(), testsupport.GitEnv(t)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// gateDriveRepo initializes a real, committed git worktree the gate driver can
// fingerprint and root its durable store beside. A drive needs a discoverable,
// non-bare repository with a resolvable HEAD.
func gateDriveRepo(t *testing.T) string {
	t.Helper()
	dir := testsupport.TempDir(t)
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

// gateDriveConfiguredRepo builds a full main-mode docket topology — a bare file
// origin, an invocation clone, and an orphan `docket` metadata branch — whose
// committed `.docket.yml` carries configBody, and returns the invocation clone
// path for use as --repo-dir. A drive resolves its suite command from this pinned
// config (the default-branch blob, never operator argv), so a test proves owner
// routing by the SIDE EFFECT of whichever command actually runs. It isolates the
// global config layer to an empty XDG dir so a developer's own config cannot steer
// resolution, and skips when git is absent. It mirrors root_test.go's
// newStatusFixtureRepo topology.
func gateDriveConfiguredRepo(t *testing.T, configBody string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found on PATH")
	}
	t.Setenv("XDG_CONFIG_HOME", testsupport.TempDir(t))

	root := testsupport.TempDir(t)
	origin := filepath.Join(root, "origin.git")
	writer := filepath.Join(root, "writer")
	invocation := filepath.Join(root, "invocation")

	statusGit(t, root, "init", "--bare", "-b", "main", origin)
	statusGit(t, root, "init", "-b", "main", writer)
	statusGit(t, writer, "config", "user.name", "t")
	statusGit(t, writer, "config", "user.email", "t@t")
	statusGit(t, writer, "config", "commit.gpgsign", "false")

	statusWriteFile(t, writer, ".docket.yml", configBody)
	statusWriteFile(t, writer, "README.md", "readme\n")
	statusGit(t, writer, "add", "-A")
	statusGit(t, writer, "commit", "-q", "-m", "main content")
	statusGit(t, writer, "remote", "add", "origin", origin)
	statusGit(t, writer, "push", "-q", "-u", "origin", "main")

	// Orphan `docket` metadata branch so PinContext's fixed metadata pin resolves
	// (0363: the metadata branch is fixed at refs/heads/docket).
	statusGit(t, writer, "checkout", "--orphan", "docket")
	statusGit(t, writer, "rm", "-rf", ".")
	statusWriteFile(t, writer, "docs/changes/BOARD.md", "# Board\n")
	statusGit(t, writer, "add", "-A")
	statusGit(t, writer, "commit", "-q", "-m", "docket: initialize metadata branch")
	statusGit(t, writer, "push", "-q", "-u", "origin", "docket")
	statusGit(t, writer, "checkout", "-q", "main")

	statusGit(t, root, "clone", "-q", origin, invocation)
	return invocation
}

// TestGateDriveStartRunsToPassed proves `gate drive start --owner build` composes
// the same state machine as the app seam through the CLI: the suite command is the
// resolved build.test_command from authoritative config (never operator argv), and
// a fast green command returns a doc carrying a drive id and the PASSED outcome at
// exit 0.
func TestGateDriveStartRunsToPassed(t *testing.T) {
	wt := gateDriveConfiguredRepo(t, "metadata_branch: main\nbuild:\n  gate: local\n  test_command: /bin/echo hi\n")
	root := testsupport.TempDir(t)
	out, errS, code := runCLI(t, "--json", "gate", "drive", "start", "--repo-dir", wt, "--run-root", root, "--owner", "build")
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

// TestGateDriveStartOwnerRoutesToOwnCommand is the DIVERGENT-COMMAND CLI test: the
// repo's build and finalize test commands touch DIFFERENT marker files, so a
// service that read the wrong owner's command cannot pass. `--owner build` must
// launch build.test_command (its marker appears) and never finalize's (its marker
// stays absent). Swapping the "build" branch of buildOwnedGateDriveService to the
// finalize constructor reddens this test.
func TestGateDriveStartOwnerRoutesToOwnCommand(t *testing.T) {
	markers := testsupport.TempDir(t)
	buildMarker := filepath.Join(markers, "build-ran")
	finalizeMarker := filepath.Join(markers, "finalize-ran")
	cfg := "metadata_branch: main\n" +
		"build:\n  gate: local\n  test_command: touch " + buildMarker + "\n" +
		"finalize:\n  test_command: touch " + finalizeMarker + "\n"
	wt := gateDriveConfiguredRepo(t, cfg)
	root := testsupport.TempDir(t)

	out, errS, code := runCLI(t, "--json", "gate", "drive", "start", "--repo-dir", wt, "--run-root", root, "--owner", "build")
	if errS != "" {
		t.Fatalf("start: err=%q code=%d", errS, code)
	}
	doc := decodeOneJSON(t, out)
	if got := driveDoc(t, doc)["outcome"]; got != "PASSED" {
		t.Fatalf("start outcome=%v, want PASSED (build command must run green): %v", got, doc)
	}
	if _, err := os.Stat(buildMarker); err != nil {
		t.Fatalf("--owner build must launch build.test_command; marker %q missing: %v", buildMarker, err)
	}
	if _, err := os.Stat(finalizeMarker); err == nil {
		t.Fatalf("--owner build must NOT launch finalize.test_command; marker %q was created", finalizeMarker)
	}
}

// TestGateDriveStartFailedIsNonZeroExit is the exit-code guard for the Task 9
// residual risk: the shared envelope result is `applied` for a FAILED verdict, so
// a CLI that keyed its exit on ExitCode(result) would report a red suite as
// success (exit 0). The process exit MUST derive from the typed outcome instead.
func TestGateDriveStartFailedIsNonZeroExit(t *testing.T) {
	wt := gateDriveConfiguredRepo(t, "metadata_branch: main\nbuild:\n  gate: local\n  test_command: /usr/bin/false\n")
	root := testsupport.TempDir(t)
	out, errS, code := runCLI(t, "--json", "gate", "drive", "start", "--repo-dir", wt, "--run-root", root, "--owner", "build")
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
// drive/claim identifiers across separate short-lived CLI invocations. Advance,
// handoff, and claim are commandless — they never resolve config.
func TestGateDriveAdvanceHandoffClaim(t *testing.T) {
	wt := gateDriveConfiguredRepo(t, "metadata_branch: main\nbuild:\n  gate: local\n  test_command: /bin/echo hi\n")
	root := testsupport.TempDir(t)
	out, _, code := runCLI(t, "--json", "gate", "drive", "start", "--repo-dir", wt, "--run-root", root, "--owner", "build")
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

// TestGateDriveStartRequiresOwner proves `--owner` is required and closed:
// omitting it is cobra's required-flag failure (invalid input, exit 2), and any
// value other than build|finalize is a command failure — invalid-input, no
// workflow document — with no drive launched, because the value is rejected before
// any config resolution or engine call. The `-- <argv>` suite-command surface is
// gone: no drive start ever accepts an operator command.
func TestGateDriveStartRequiresOwner(t *testing.T) {
	wt := gateDriveRepo(t)
	root := testsupport.TempDir(t)
	// Omitting --owner: cobra's required-flag check fails before RunE, exit 2.
	_, _, code := runCLI(t, "gate", "drive", "start", "--repo-dir", wt, "--run-root", root)
	if code != 2 {
		t.Fatalf("missing --owner: code=%d, want 2", code)
	}
	// An unknown owner value is a command failure: invalid-input, no drive doc.
	out, errS, code := runCLI(t, "--json", "gate", "drive", "start", "--repo-dir", wt, "--run-root", root, "--owner", "bogus")
	if code != 2 || errS != "" {
		t.Fatalf("bogus owner json: err=%q code=%d", errS, code)
	}
	doc := decodeOneJSON(t, out)
	if doc["result"] != "invalid-input" {
		t.Fatalf("bogus owner result=%v, want invalid-input", doc["result"])
	}
	if _, hasDrive := doc["drive"]; hasDrive {
		t.Fatalf("a rejected owner must not carry a workflow drive document: %v", doc)
	}
}

// TestGateDriveStartNoCommandLeak proves the protocol output never leaks the
// resolved suite command (redaction): the drive document carries identity and
// outcome, never the command words, in either JSON or human mode — even though the
// command now comes from authoritative config rather than operator argv.
func TestGateDriveStartNoCommandLeak(t *testing.T) {
	const secret = "SENTINEL_no_leak_TOKEN_98217"
	wt := gateDriveConfiguredRepo(t, "metadata_branch: main\nbuild:\n  gate: local\n  test_command: /bin/echo "+secret+"\n")
	root := testsupport.TempDir(t)
	out, _, code := runCLI(t, "--json", "gate", "drive", "start", "--repo-dir", wt, "--run-root", root, "--owner", "build")
	if code != 0 {
		t.Fatalf("start failed: %q", out)
	}
	if strings.Contains(out, secret) {
		t.Fatalf("suite command leaked into protocol JSON: %q", out)
	}
	out, _, code = runCLI(t, "gate", "drive", "start", "--repo-dir", wt, "--run-root", root, "--owner", "build")
	if code != 0 {
		t.Fatalf("start (human) failed: %q", out)
	}
	if strings.Contains(out, secret) {
		t.Fatalf("suite command leaked into human output: %q", out)
	}
}

// TestGateDriveStartBuildRejectsArgv proves `--owner build|finalize` still refuses
// a `-- <argv>` boundary: the config owners run their resolved suite command, never
// operator argv. Exit 2, no drive launched.
func TestGateDriveStartBuildRejectsArgv(t *testing.T) {
	wt := gateDriveRepo(t)
	root := testsupport.TempDir(t)
	_, _, code := runCLI(t, "gate", "drive", "start", "--repo-dir", wt, "--run-root", root, "--owner", "build", "--", "/bin/echo")
	if code != 2 {
		t.Fatalf("build with argv: code=%d, want 2", code)
	}
	_, _, code = runCLI(t, "gate", "drive", "start", "--repo-dir", wt, "--run-root", root, "--owner", "finalize", "--", "/bin/echo")
	if code != 2 {
		t.Fatalf("finalize with argv: code=%d, want 2", code)
	}
}

// TestGateDriveStartRejectsPositionalBeforeDash proves a positional word with no
// `--` separator, or before one, is rejected (exit 2) — no owner takes an argv.
func TestGateDriveStartRejectsPositionalBeforeDash(t *testing.T) {
	wt := gateDriveRepo(t)
	root := testsupport.TempDir(t)
	_, _, code := runCLI(t, "gate", "drive", "start", "--repo-dir", wt, "--run-root", root, "--owner", "build", "/bin/echo")
	if code != 2 {
		t.Fatalf("positional without dash: code=%d, want 2", code)
	}
	_, _, code = runCLI(t, "gate", "drive", "start", "--repo-dir", wt, "--run-root", root, "--owner", "build", "before", "--", "/bin/echo")
	if code != 2 {
		t.Fatalf("positional before dash: code=%d, want 2", code)
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

// TestGateLaunchInsideWorktreeSecondRefused proves the CLI `gate launch` leaf
// carries the worktree-admission refusal shape (change 0490): a first launch whose
// --cwd sits inside a registered worktree hands the worktree lock to its live
// supervisor, and a second launch into the same worktree from a DISTINCT --root is
// refused with result "blocked", reason "worktree-busy", exit 1, and no run_dir —
// exactly one launched. Once `gate stop` ends the first run, a third launch is
// admitted with no other step: the supervisor's exit freed the lock.
func TestGateLaunchInsideWorktreeSecondRefused(t *testing.T) {
	wt := gateDriveConfiguredRepo(t, "metadata_branch: main\n")

	out1, err1, code1 := runCLI(t, "--json", "gate", "launch", "--root", testsupport.TempDir(t), "--cwd", wt, "--", "/bin/sleep", "60")
	if code1 != 0 || err1 != "" {
		t.Fatalf("first launch: out=%q err=%q code=%d", out1, err1, code1)
	}
	doc1 := decodeOneJSON(t, out1)
	runDir, _ := doc1["run_dir"].(string)
	if runDir == "" {
		t.Fatalf("first launch produced no run_dir: %v", doc1)
	}
	// The reason is a flag: a positional reason is rejected as invalid input and
	// would silently leave the live first run (and its supervisor) running.
	t.Cleanup(func() { runCLI(t, "gate", "stop", runDir, "--reason", "test cleanup") })

	out2, err2, code2 := runCLI(t, "--json", "gate", "launch", "--root", testsupport.TempDir(t), "--cwd", wt, "--", "/bin/echo", "hi")
	if err2 != "" {
		t.Fatalf("second launch stderr=%q", err2)
	}
	doc2 := decodeOneJSON(t, out2)
	if rd, _ := doc2["run_dir"].(string); rd != "" {
		// Never expected; stop it so a wrongly admitted run does not leak.
		t.Cleanup(func() { runCLI(t, "gate", "stop", rd, "--reason", "test cleanup") })
	}
	if doc2["operation"] != "gate.launch" || doc2["result"] != "blocked" {
		t.Fatalf("second launch doc=%v, want blocked", doc2)
	}
	if doc2["reason"] != "worktree-busy" {
		t.Fatalf("second launch reason=%v, want worktree-busy", doc2["reason"])
	}
	if code2 != 1 {
		t.Fatalf("second launch exit code=%d, want 1", code2)
	}
	if rd, _ := doc2["run_dir"].(string); rd != "" {
		t.Fatalf("refused launch produced a run_dir %q", rd)
	}

	// Stop the first run; the worktree frees itself when its supervisor exits. The
	// stop's terminal state can land a moment before the supervisor is gone, so the
	// third launch is retried only while it is refused worktree-busy.
	if _, serr, _ := runCLI(t, "--json", "gate", "stop", runDir, "--reason", "test stop"); serr != "" {
		t.Fatalf("gate stop stderr=%q", serr)
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		out3, err3, code3 := runCLI(t, "--json", "gate", "launch", "--root", testsupport.TempDir(t), "--cwd", wt, "--", "/bin/echo", "hi")
		if err3 != "" {
			t.Fatalf("third launch stderr=%q", err3)
		}
		doc3 := decodeOneJSON(t, out3)
		if rd, _ := doc3["run_dir"].(string); rd != "" {
			t.Cleanup(func() { runCLI(t, "gate", "stop", rd, "--reason", "test cleanup") })
			if code3 != 0 {
				t.Fatalf("third launch exit code=%d doc=%v", code3, doc3)
			}
			return
		}
		if doc3["reason"] != "worktree-busy" || time.Now().After(deadline) {
			t.Fatalf("third launch after stop must be admitted, got %v", doc3)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// TestGateDriveRetiredScopeCommandsAreUnknown (change 0489): the recovery-scope
// operations are gone from the CLI. The leaf is not registered, and a stale caller
// invoking one gets a usage error and no gate.drive.* protocol document — never a
// silently different operation.
func TestGateDriveRetiredScopeCommandsAreUnknown(t *testing.T) {
	root := captureTree(t)
	for _, sub := range []string{"prepare-scope", "acknowledge", "takeover"} {
		if cmd, _, err := root.Find([]string{"gate", "drive", sub}); err == nil && cmd.Name() == sub {
			t.Errorf("docket gate drive %s is still registered", sub)
		}
		out, _, code := runCLI(t, "--json", "gate", "drive", sub)
		if code != 2 {
			t.Errorf("docket gate drive %s exited %d, want 2 (unknown command)", sub, code)
		}
		if strings.Contains(out, `"operation":"gate.drive.`) {
			t.Errorf("docket gate drive %s emitted a protocol document: %s", sub, out)
		}
	}
}

// TestGateDriveStartRetiredTaskSurfaceIsUsageError (change 0489): a stale caller of
// the retired task-owned surface gets a usage error and launches nothing — never a
// silent build-owned run of the configured suite.
func TestGateDriveStartRetiredTaskSurfaceIsUsageError(t *testing.T) {
	marker := filepath.Join(testsupport.TempDir(t), "suite-ran")
	wt := gateDriveConfiguredRepo(t, "metadata_branch: main\nbuild:\n  gate: local\n  test_command: touch "+marker+"\n")
	root := testsupport.TempDir(t)
	base := []string{"--json", "gate", "drive", "start", "--repo-dir", wt, "--run-root", root}
	cases := map[string][]string{
		"owner task":            {"--owner", "task", "--", "/bin/echo", "hi"},
		"argv after dash":       {"--owner", "build", "--", "/bin/echo", "hi"},
		"scope id":              {"--owner", "build", "--scope-id", "s"},
		"child cap":             {"--owner", "build", "--child-cap", "c"},
		"predecessor drive id":  {"--owner", "build", "--predecessor-drive-id", "d"},
		"predecessor owner gen": {"--owner", "build", "--predecessor-owner-gen", "g"},
	}
	for name, extra := range cases {
		out, _, code := runCLI(t, append(append([]string{}, base...), extra...)...)
		if code != 2 {
			t.Errorf("%s: exited %d, want 2 (usage error): %s", name, code, out)
		}
		if strings.Contains(out, `"drive"`) {
			t.Errorf("%s: emitted a drive document: %s", name, out)
		}
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatalf("a refused start launched the configured suite")
	}
}

// gitCommonDirForTest resolves dir's absolute Git common directory, the root the
// durable drive store lives under.
func gitCommonDirForTest(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	cmd.Env = append(os.Environ(), testsupport.GitEnv(t)...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse --git-common-dir: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// TestGateDriveStartBuildOwnedStoresRunContextHash (change 0489): with the task
// owner gone, this proves a build-owned start's --run-context reaches the drive
// record as the hash run.verdict's outer scan matches a run's drives on.
func TestGateDriveStartBuildOwnedStoresRunContextHash(t *testing.T) {
	wt := gateDriveConfiguredRepo(t, "metadata_branch: main\nbuild:\n  gate: local\n  test_command: /bin/echo hi\n")
	root := testsupport.TempDir(t)
	out, errS, code := runCLI(t, "--json", "gate", "drive", "start", "--repo-dir", wt, "--run-root", root,
		"--owner", "build", "--change-id", "489", "--run-context", "ctx-0489")
	if code != 0 || errS != "" {
		t.Fatalf("start: out=%q err=%q code=%d", out, errS, code)
	}
	id, _ := driveDoc(t, decodeOneJSON(t, out))["drive_id"].(string)
	if id == "" {
		t.Fatalf("start produced no drive id: %s", out)
	}
	common := gitCommonDirForTest(t, wt)
	buf, err := os.ReadFile(filepath.Join(common, "docket", "gate-drives", "v2", id, "record.json"))
	if err != nil {
		t.Fatalf("read drive record: %v", err)
	}
	var env struct {
		Record struct {
			RunContextHash string `json:"run_context_hash"`
		} `json:"record"`
	}
	if err := json.Unmarshal(buf, &env); err != nil {
		t.Fatalf("decode drive record: %v", err)
	}
	sum := sha256.Sum256([]byte("ctx-0489"))
	if want := hex.EncodeToString(sum[:]); env.Record.RunContextHash != want {
		t.Fatalf("drive run_context_hash = %q, want sha256(--run-context) %q", env.Record.RunContextHash, want)
	}
}

// TestGateDriveStartRejectsRunIDFlag (change 0491): gate drive start no longer
// takes --run-id. The run launch check it fed is gone, so a caller still passing
// it fails on an unknown flag (exit 2) instead of being silently accepted.
func TestGateDriveStartRejectsRunIDFlag(t *testing.T) {
	_, errS, code := runCLI(t, "gate", "drive", "start", "--owner", "build",
		"--run-root", "/tmp/docket-0491-run-root", "--run-id", "0790b760e26444866ef2e156ba383326")
	if code != 2 || !strings.Contains(errS, "unknown flag: --run-id") {
		t.Fatalf("exit %d stderr %q, want exit 2 naming the unknown --run-id flag", code, errS)
	}
}

// TestGateDriveStartRejectsIdempotentSuiteGateFlag (change 0493): gate drive
// start no longer takes --idempotent-suite-gate — no drive relaunches — so a
// hand-typed use fails on an unknown flag (exit 2) instead of being accepted.
func TestGateDriveStartRejectsIdempotentSuiteGateFlag(t *testing.T) {
	_, errS, code := runCLI(t, "gate", "drive", "start", "--owner", "finalize",
		"--run-root", "/tmp/docket-0493-run-root", "--idempotent-suite-gate")
	if code != 2 || !strings.Contains(errS, "unknown flag: --idempotent-suite-gate") {
		t.Fatalf("exit %d stderr %q, want exit 2 naming the unknown --idempotent-suite-gate flag", code, errS)
	}
}
