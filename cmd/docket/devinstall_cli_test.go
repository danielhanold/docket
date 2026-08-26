package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// witness is the name-independent core prohibition harness.RecursionGuard
// injects into every generated wrapper. It exists in the source goldens and is
// exactly what the live development-install defect dropped from the wrappers a
// stale renderer re-emitted. It is a single source literal in guard.go, so it is
// compiled contiguously into any real docket binary too — which is what lets the
// same clause prove a binary is a genuine docket build rather than a placeholder.
// (RecursionGuard splits its full sentence across several `+` literals, so a
// longer phrase spanning a split is not contiguous in the binary; this clause is
// deliberately within one literal.)
const witness = "Do not dispatch another"

// TestDevelopmentInstallFreshRenderHandoff is the spec's fresh-render acceptance
// test. One `development install` invocation must produce a binary and wrappers
// that BOTH carry the witness — proving the installation was rendered by the
// candidate the command built from the source tree, not by the older executable
// that started the command.
//
// The parent binary run here is the TestMain-built one (the source tree's
// renderer). It validates the checkout, builds a candidate from it, and hands
// the whole installation to that candidate; the candidate plans and applies. A
// renderer-only change carries an unchanged asset-tree digest, so only a run
// that renders through the freshly built candidate updates the affected wrappers
// on this first invocation.
func TestDevelopmentInstallFreshRenderHandoff(t *testing.T) {
	source := moduleRoot(t)
	home := t.TempDir()

	// The "old installed binary": a stub whose renderer omits the witness. It is
	// the absence control that makes the witness a real discriminator — a
	// non-docket binary genuinely lacks the compiled recursion-guard literal. It
	// is not placed at the bin destination: an unrecorded foreign binary there
	// is an ownership conflict the installer correctly refuses, and a black-box
	// subprocess cannot pre-authorize one. Its role is to prove the witness is
	// not something every executable happens to contain.
	stub := buildWitnesslessStub(t)
	if bytes.Contains(readBytes(t, stub), []byte(witness)) {
		t.Fatal("the stub carries the witness; it cannot serve as the absence control")
	}

	stdout, stderr, code := runDevelopmentInstall(t, home, source, "claude")
	if code != 0 {
		t.Fatalf("development install exited %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}

	// The installed binary is the candidate: its bytes carry the witness the
	// stub lacked, so a real docket build — not a placeholder — landed at the
	// destination.
	installed := filepath.Join(home, ".local", "bin", "docket")
	if !bytes.Contains(readBytes(t, installed), []byte(witness)) {
		t.Errorf("installed binary at %s does not carry the witness: it is not the freshly built candidate", installed)
	}

	// The rendered wrappers carry the witness: the candidate's renderer produced
	// them on this single invocation. Absent the fresh-binary handoff, an older
	// renderer would have re-emitted wrappers without it.
	assertAWrapperCarriesWitness(t, filepath.Join(home, ".claude", "agents"))
}

// moduleRoot is the checkout the candidate is built from: the directory at or
// above the test's working directory that carries go.mod.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no go.mod found above %s", dir)
		}
		dir = parent
	}
}

// buildWitnesslessStub compiles a tiny standalone program that prints a canned
// line and exits 0, returning its path.
func buildWitnesslessStub(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	const prog = "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"old installed binary: nothing to see here\") }\n"
	if err := os.WriteFile(src, []byte(prog), 0o644); err != nil {
		t.Fatalf("writing stub source: %v", err)
	}
	out := filepath.Join(dir, "docket-old")
	cmd := exec.Command("go", "build", "-o", out, src)
	cmd.Env = os.Environ()
	if msg, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building stub: %v\n%s", err, msg)
	}
	return out
}

// runDevelopmentInstall runs the TestMain-built binary as `development install`
// with HOME and the XDG roots pinned under a temp home, preserving the real Go
// build caches so the candidate build the command triggers is warm rather than
// cold under the pinned home.
func runDevelopmentInstall(t *testing.T, home, source, harness string) (string, string, int) {
	t.Helper()
	drop := map[string]bool{
		"HOME": true, "XDG_CONFIG_HOME": true, "XDG_DATA_HOME": true, "XDG_BIN_HOME": true,
		"GOCACHE": true, "GOMODCACHE": true, "GOPATH": true,
	}
	var env []string
	for _, kv := range os.Environ() {
		if i := strings.IndexByte(kv, '='); i >= 0 && drop[kv[:i]] {
			continue
		}
		env = append(env, kv)
	}
	env = append(env, "HOME="+home)
	for _, k := range []string{"GOCACHE", "GOMODCACHE", "GOPATH"} {
		env = append(env, k+"="+goEnv(t, k))
	}

	var out, errBuf bytes.Buffer
	cmd := exec.Command(binPath, "development", "install", "--source", source, "--harness", harness)
	cmd.Env = env
	cmd.Stdout, cmd.Stderr = &out, &errBuf
	err := cmd.Run()
	code := 0
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("running development install: %v", err)
		}
		code = ee.ExitCode()
	}
	return out.String(), errBuf.String(), code
}

func goEnv(t *testing.T, key string) string {
	t.Helper()
	out, err := exec.Command("go", "env", key).Output()
	if err != nil {
		t.Fatalf("go env %s: %v", key, err)
	}
	return strings.TrimSpace(string(out))
}

func readBytes(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return b
}

// assertAWrapperCarriesWitness proves at least one installed agent wrapper
// exists under dir and carries the witness. An empty directory is a failure: the
// candidate rendered nothing.
func assertAWrapperCarriesWitness(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("no rendered wrappers under %s: %v", dir, err)
	}
	var wrappers int
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		wrappers++
		if bytes.Contains(readBytes(t, filepath.Join(dir, e.Name())), []byte(witness)) {
			return
		}
	}
	if wrappers == 0 {
		t.Fatalf("the candidate rendered no agent wrappers under %s", dir)
	}
	t.Fatalf("no wrapper under %s carries the witness; a stale renderer produced them", dir)
}
