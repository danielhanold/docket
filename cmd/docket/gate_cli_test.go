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

// gateDriveRepo initializes a committed git worktree the driver can fingerprint,
// mirroring the package-level helper for the built-binary path.
func gateDriveRepo(t *testing.T) string {
	t.Helper()
	dir := gateTempDir(t)
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "t@t"},
		{"config", "user.name", "t"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "seed"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "seed"}, {"commit", "-q", "-m", "seed"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

// TestGateDriveEndToEndThroughBuiltBinary drives `gate drive start -- <argv>`
// through the true production re-exec: the built binary starts a detached
// supervisor and advances one slice, returning the shared protocol document with
// a drive id and the PASSED outcome.
func TestGateDriveEndToEndThroughBuiltBinary(t *testing.T) {
	wt := gateDriveRepo(t)
	root := gateTempDir(t)
	out, errS, code := run(t, "--json", "gate", "drive", "start", "--repo-dir", wt, "--run-root", root, "--", "/bin/echo", "hi")
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
