package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

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

// gateSh runs one git subcommand in dir through the real git binary, failing on
// any error. It is the built-binary package's local git helper (there is no
// shared statusGit here).
func gateSh(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// gateDriveConfiguredRepo builds a full main-mode docket topology — a bare file
// origin, an invocation clone, and an orphan `docket` metadata branch — whose
// committed `.docket.yml` carries configBody, and returns the invocation clone for
// use as --repo-dir. The drive resolves its suite command from this pinned config
// (never operator argv), which the built binary reads over origin. It isolates the
// global config layer to an empty XDG dir so a developer's own config cannot steer
// resolution, and skips when git is absent.
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

	if err := os.MkdirAll(writer, 0o755); err != nil {
		t.Fatal(err)
	}
	gateSh(t, root, "init", "--bare", "-b", "main", origin)
	gateSh(t, root, "init", "-b", "main", writer)
	gateSh(t, writer, "config", "user.name", "t")
	gateSh(t, writer, "config", "user.email", "t@t")
	gateSh(t, writer, "config", "commit.gpgsign", "false")

	if err := os.WriteFile(filepath.Join(writer, ".docket.yml"), []byte(configBody), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(writer, "README.md"), []byte("readme\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gateSh(t, writer, "add", "-A")
	gateSh(t, writer, "commit", "-q", "-m", "main content")
	gateSh(t, writer, "remote", "add", "origin", origin)
	gateSh(t, writer, "push", "-q", "-u", "origin", "main")

	gateSh(t, writer, "checkout", "--orphan", "docket")
	gateSh(t, writer, "rm", "-rf", ".")
	if err := os.MkdirAll(filepath.Join(writer, "docs/changes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(writer, "docs/changes/BOARD.md"), []byte("# Board\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gateSh(t, writer, "add", "-A")
	gateSh(t, writer, "commit", "-q", "-m", "docket: initialize metadata branch")
	gateSh(t, writer, "push", "-q", "-u", "origin", "docket")
	gateSh(t, writer, "checkout", "-q", "main")

	gateSh(t, root, "clone", "-q", origin, invocation)
	return invocation
}

// TestGateDriveEndToEndThroughBuiltBinary drives `gate drive start --owner build`
// through the true production re-exec: the built binary resolves build.test_command
// from authoritative config, starts a detached supervisor, and advances one slice,
// returning the shared protocol document with a drive id and the PASSED outcome.
func TestGateDriveEndToEndThroughBuiltBinary(t *testing.T) {
	wt := gateDriveConfiguredRepo(t, "metadata_branch: main\nbuild:\n  gate: local\n  test_command: /bin/echo hi\n")
	root := gateTempDir(t)
	out, errS, code := run(t, "--json", "gate", "drive", "start", "--repo-dir", wt, "--run-root", root, "--owner", "build")
	if code != 0 || errS != "" {
		t.Fatalf("start: out=%q err=%q code=%d", out, errS, code)
	}
	doc := assertOneJSONDocument(t, out)
	if doc["operation"] != "gate.drive.start" || doc["result"] != "applied" {
		t.Fatalf("start envelope: %v", doc)
	}
	drive, ok := doc["drive"].(map[string]any)
	if !ok {
		t.Fatalf("start carried no drive document: %v", doc)
	}
	if id, _ := drive["drive_id"].(string); id == "" {
		t.Fatalf("start produced no drive id: %v", drive)
	}
	if drive["outcome"] != "PASSED" {
		t.Fatalf("start outcome=%v, want PASSED", drive["outcome"])
	}
}

// TestGateEndToEndThroughBuiltBinary is the only test that exercises the true
// production re-exec: the built docket binary launches a supervised /bin/echo
// under a fresh root, then the same binary re-executes itself as the durable
// supervisor. It proves the whole cli -> app -> process path plus the one
// os.Exit site, not the test-binary seam the package tests use.
func TestGateEndToEndThroughBuiltBinary(t *testing.T) {
	root := gateTempDir(t)
	cwd := gateTempDir(t)

	out, errS, code := run(t, "--json", "gate", "launch", "--root", root, "--cwd", cwd, "--", "/bin/echo", "hi")
	if code != 0 || errS != "" {
		t.Fatalf("launch: out=%q err=%q code=%d", out, errS, code)
	}
	doc := assertOneJSONDocument(t, out)
	if doc["operation"] != "gate.launch" || doc["result"] != "applied" {
		t.Fatalf("launch doc=%v", doc)
	}
	runDir, _ := doc["run_dir"].(string)
	if runDir == "" {
		t.Fatalf("launch produced no run_dir: %v", doc)
	}

	// Poll observe through the built binary until the run is terminal.
	var obs map[string]any
	for i := 0; ; i++ {
		out, errS, code = run(t, "--json", "gate", "observe", runDir)
		if errS != "" {
			t.Fatalf("observe stderr=%q", errS)
		}
		obs = assertOneJSONDocument(t, out)
		if obs["state"] != "running" {
			break
		}
		if i > 300 {
			t.Fatal("run never became terminal")
		}
		time.Sleep(100 * time.Millisecond)
	}

	if obs["operation"] != "gate.observe" || obs["state"] != "passed" {
		t.Fatalf("observe doc=%v", obs)
	}
	if code != 0 {
		t.Fatalf("observe exit code = %d, want 0", code)
	}
	ec, ok := obs["exit_code"].(float64)
	if !ok || ec != 0 {
		t.Fatalf("observe exit_code=%v (%T), want exact 0", obs["exit_code"], obs["exit_code"])
	}
}
