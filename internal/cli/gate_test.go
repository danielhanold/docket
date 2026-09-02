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
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	root := gateTempDir(t)
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
	root := gateTempDir(t)
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
	markers := t.TempDir()
	buildMarker := filepath.Join(markers, "build-ran")
	finalizeMarker := filepath.Join(markers, "finalize-ran")
	cfg := "metadata_branch: main\n" +
		"build:\n  gate: local\n  test_command: touch " + buildMarker + "\n" +
		"finalize:\n  test_command: touch " + finalizeMarker + "\n"
	wt := gateDriveConfiguredRepo(t, cfg)
	root := gateTempDir(t)

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
	root := gateTempDir(t)
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
	root := gateTempDir(t)
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
	root := gateTempDir(t)
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
	root := gateTempDir(t)
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

// TestGateDrivePrepareScopeGrantAndRedaction proves `gate drive prepare-scope`
// composes the commandless store seam and returns the full grant in JSON — a
// scope id and BOTH opaque capabilities — while its human text names ONLY the
// scope id. The capabilities are authority: they travel in the protocol document
// and never in diagnostic prose.
func TestGateDrivePrepareScopeGrantAndRedaction(t *testing.T) {
	wt := gateDriveRepo(t)
	out, errS, code := runCLI(t, "--json", "gate", "drive", "prepare-scope",
		"--repo-dir", wt, "--change-id", "359", "--task-id", "task-4",
		"--phase", "build", "--branch", "fix/x", "--worktree", wt)
	if code != 0 || errS != "" {
		t.Fatalf("prepare-scope: out=%q err=%q code=%d", out, errS, code)
	}
	doc := decodeOneJSON(t, out)
	if doc["operation"] != "gate.drive.prepare-scope" || doc["result"] != "applied" {
		t.Fatalf("prepare-scope envelope: %v", doc)
	}
	scopeID, _ := doc["scope_id"].(string)
	childCap, _ := doc["child_capability"].(string)
	parentCap, _ := doc["parent_capability"].(string)
	if scopeID == "" || childCap == "" || parentCap == "" {
		t.Fatalf("prepare-scope JSON missing a grant field: %v", doc)
	}
	if childCap == parentCap {
		t.Fatalf("child and parent capabilities must be distinct: %q", childCap)
	}

	// Human mode: the scope id appears, neither capability does.
	human, errS, code := runCLI(t, "gate", "drive", "prepare-scope",
		"--repo-dir", wt, "--change-id", "359", "--task-id", "task-4",
		"--phase", "build", "--branch", "fix/x", "--worktree", wt)
	if code != 0 || errS != "" {
		t.Fatalf("prepare-scope human: out=%q err=%q code=%d", human, errS, code)
	}
	if !strings.Contains(human, "scope_id") {
		t.Fatalf("human text must name the scope id: %q", human)
	}
	if strings.Contains(human, childCap) || strings.Contains(human, parentCap) {
		t.Fatalf("human text leaked a capability: %q", human)
	}
}

// TestGateDriveScopeBoundStartRoundTrips proves the task-intent scope→start flow
// round-trips in a worktree: `gate drive prepare-scope` then `gate drive start
// --owner task --scope-id … --child-cap …` binds and runs a real drive, rather
// than failing scope-identity-mismatch → invalid-request. It is the regression
// guard for the RepoIdentity dimension the two sides compare: prepare-scope pins
// the REPOSITORY identity (the Git common dir, shared across worktrees, as the
// rest of docket records it) and the scope-bound start MUST resolve that same
// dimension — never the worktree path — so a legitimate prepare→start round-trips.
// The change/task/phase/branch/worktree dimensions are supplied to match the
// scope, so RepoIdentity is the dimension actually under test. A genuine
// cross-repo/cross-worktree start still fails closed (proved in gatedrive's
// scopeIdentityMatch table and TestStartBindsScope), so this never weakens the
// fail-closed check.
func TestGateDriveScopeBoundStartRoundTrips(t *testing.T) {
	wt := gateDriveConfiguredRepo(t, "metadata_branch: main\n")
	root := gateTempDir(t)

	// Prepare a task recovery scope for this worktree.
	out, errS, code := runCLI(t, "--json", "gate", "drive", "prepare-scope",
		"--repo-dir", wt, "--change-id", "359", "--task-id", "task-12",
		"--phase", "build", "--branch", "fix/x", "--worktree", wt)
	if code != 0 || errS != "" {
		t.Fatalf("prepare-scope: out=%q err=%q code=%d", out, errS, code)
	}
	grant := decodeOneJSON(t, out)
	scopeID, _ := grant["scope_id"].(string)
	childCap, _ := grant["child_capability"].(string)
	if scopeID == "" || childCap == "" {
		t.Fatalf("prepare-scope missing a grant field: %v", grant)
	}

	// A scope-bound task start over the SAME worktree, with every pinned identity
	// dimension supplied to match the scope, must BIND — an APPLIED result carrying
	// a drive id — rather than fail scope-identity-mismatch → invalid-request. The
	// BIND is the exact property the RepoIdentity fix delivers: the start cleared
	// scopeIdentityMatch, launched, and got a drive id. The drive's terminal
	// OUTCOME is deliberately NOT asserted here — this regression keys on the bind
	// (an APPLIED result carrying a drive id), which fails hard on the old
	// no-drive/invalid-request state regardless of whether the fast child reaches
	// PASSED within the first slice. (The task-intent zero-budget defect that once
	// forced a HALT here is fixed: the task owner now resolves the configured
	// observation budget, so a still-running child WAITS rather than HALTing.)
	out, errS, code = runCLI(t, "--json", "gate", "drive", "start",
		"--repo-dir", wt, "--run-root", root, "--owner", "task",
		"--scope-id", scopeID, "--child-cap", childCap,
		"--change-id", "359", "--task-id", "task-12", "--phase", "build",
		"--branch", "fix/x", "--", "/bin/echo", "hi")
	if errS != "" {
		t.Fatalf("scope-bound task start wrote stderr: out=%q err=%q code=%d", out, errS, code)
	}
	doc := decodeOneJSON(t, out)
	if doc["operation"] != "gate.drive.start" {
		t.Fatalf("scope-bound start operation=%v: %v", doc["operation"], doc)
	}
	if doc["result"] != "applied" {
		t.Fatalf("scope-bound start did not apply — a RepoIdentity-dimension mismatch surfaces as result=invalid-input reason=invalid-request with no drive: %v", doc)
	}
	d := driveDoc(t, doc)
	if id, _ := d["drive_id"].(string); id == "" {
		t.Fatalf("scope-bound start bound no drive id (the scope bind did not take): %v", d)
	}
}

// TestGateDriveTakeoverWired proves the `gate drive takeover` leaf is registered
// and reaches the app seam: it composes the commandless service and emits exactly
// one gate.drive.takeover protocol document (its workflow outcome — a HALTED
// refusal for a bogus scope — is the driver's concern, proved in gatedrive).
func TestGateDriveTakeoverWired(t *testing.T) {
	wt := gateDriveRepo(t)
	out, errS, code := runCLI(t, "--json", "gate", "drive", "takeover",
		"--repo-dir", wt, "--scope-id", "0123456789abcdef0123456789abcdef", "--parent-cap", "deadbeefdeadbeefdeadbeefdeadbeef")
	if errS != "" {
		t.Fatalf("takeover: err=%q code=%d", errS, code)
	}
	doc := decodeOneJSON(t, out)
	if doc["operation"] != "gate.drive.takeover" {
		t.Fatalf("takeover operation=%v, want gate.drive.takeover: %v", doc["operation"], doc)
	}
}

// TestGateDriveTakeoverRequiresFlags proves --scope-id and --parent-cap are
// required (cobra's required-flag failure, exit 2, before RunE).
func TestGateDriveTakeoverRequiresFlags(t *testing.T) {
	wt := gateDriveRepo(t)
	_, _, code := runCLI(t, "gate", "drive", "takeover", "--repo-dir", wt, "--scope-id", "x")
	if code != 2 {
		t.Fatalf("missing --parent-cap: code=%d, want 2", code)
	}
	_, _, code = runCLI(t, "gate", "drive", "takeover", "--repo-dir", wt, "--parent-cap", "x")
	if code != 2 {
		t.Fatalf("missing --scope-id: code=%d, want 2", code)
	}
}

// TestGateDriveStartOwnerTaskRunsArgv proves `--owner task` runs the agent-supplied
// argv verbatim (the COMMAND is not resolved from config), while the observation
// BUDGET IS resolved from authoritative config (the configured repo below carries
// the default gate_observation_budget): a fast green command returns a drive doc
// carrying a drive id and PASSED at exit 0. It is deliberately NOT load-fragile:
// with the resolved non-zero budget the slice polls the still-running child until
// it exits (PASSED) instead of the pre-fix zero budget, which fixed the deadline
// at start and HALTed deadline-expired the first time the child was observed
// running under load (the Task-12 defect).
func TestGateDriveStartOwnerTaskRunsArgv(t *testing.T) {
	wt := gateDriveConfiguredRepo(t, "metadata_branch: main\n")
	root := gateTempDir(t)
	out, errS, code := runCLI(t, "--json", "gate", "drive", "start",
		"--repo-dir", wt, "--run-root", root, "--owner", "task", "--", "/bin/echo", "hi")
	if code != 0 || errS != "" {
		t.Fatalf("task start: out=%q err=%q code=%d", out, errS, code)
	}
	d := driveDoc(t, decodeOneJSON(t, out))
	if id, _ := d["drive_id"].(string); id == "" {
		t.Fatalf("task start produced no drive id: %v", d)
	}
	if d["outcome"] != "PASSED" {
		t.Fatalf("task start outcome=%v, want PASSED", d["outcome"])
	}
}

// TestGateDriveStartOwnerTaskRequiresArgv proves `--owner task` without a `--`
// argv boundary is an invalid-input command failure (exit 2) naming the `--`
// contract, and never launches a drive.
func TestGateDriveStartOwnerTaskRequiresArgv(t *testing.T) {
	wt := gateDriveRepo(t)
	root := gateTempDir(t)
	_, errS, code := runCLI(t, "gate", "drive", "start", "--repo-dir", wt, "--run-root", root, "--owner", "task")
	if code != 2 {
		t.Fatalf("task without argv: code=%d, want 2", code)
	}
	if !strings.Contains(errS, "--") {
		t.Fatalf("task without argv: message does not name the -- contract: %q", errS)
	}
}

// TestGateDriveStartBuildRejectsArgv proves `--owner build|finalize` still refuses
// a `-- <argv>` boundary: the config owners run their resolved suite command, never
// operator argv. Exit 2, no drive launched.
func TestGateDriveStartBuildRejectsArgv(t *testing.T) {
	wt := gateDriveRepo(t)
	root := gateTempDir(t)
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
// `--` separator, or before one, is rejected (exit 2) — the argv only ever follows
// a bare `--`.
func TestGateDriveStartRejectsPositionalBeforeDash(t *testing.T) {
	wt := gateDriveRepo(t)
	root := gateTempDir(t)
	_, _, code := runCLI(t, "gate", "drive", "start", "--repo-dir", wt, "--run-root", root, "--owner", "task", "/bin/echo")
	if code != 2 {
		t.Fatalf("positional without dash: code=%d, want 2", code)
	}
	_, _, code = runCLI(t, "gate", "drive", "start", "--repo-dir", wt, "--run-root", root, "--owner", "task", "before", "--", "/bin/echo")
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
