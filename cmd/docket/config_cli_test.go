package main

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These tests drive the BUILT binary, so they are the only place the whole
// stack — Cobra wiring, filesystem adapter, resolver, presenter, exit mapping —
// is observed the way a machine consumer observes it.
//
// Every one of them runs the child with XDG_CONFIG_HOME and HOME pinned into a
// temp directory. A configuration reader that consulted the developer's real
// global file would otherwise make these tests depend on the machine they run
// on, and would make a green run on a clean machine no evidence at all.

// hermeticEnv returns a child environment whose global-configuration lookup can
// only reach the returned xdgDir, plus the home it falls back to. It never
// mutates this process's environment, so parallel tests cannot interfere.
func hermeticEnv(t *testing.T) (xdgDir string, env []string) {
	t.Helper()
	base := t.TempDir()
	xdgDir = filepath.Join(base, "xdg")
	homeDir := filepath.Join(base, "home")
	for _, d := range []string{xdgDir, homeDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("creating %s: %v", d, err)
		}
	}
	// exec.Cmd keeps the LAST occurrence of a duplicated key, so appending
	// overrides whatever the developer's environment carries.
	return xdgDir, append(os.Environ(), "XDG_CONFIG_HOME="+xdgDir, "HOME="+homeDir)
}

// runEnv is run() with an explicit child environment. main_test.go's run uses
// the inherited environment, which no configuration test may do.
func runEnv(t *testing.T, env []string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errBuf strings.Builder
	cmd := exec.Command(binPath, args...)
	cmd.Env = env
	cmd.Stdout, cmd.Stderr = &out, &errBuf
	err := cmd.Run()
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("running %v: %v", args, err)
		}
		code = ee.ExitCode()
	}
	return out.String(), errBuf.String(), code
}

// sparseRepo is a repository directory with no configuration files at all.
func sparseRepo(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating %s: %v", dir, err)
	}
	return dir
}

// repoWithFile is a repository directory carrying one configuration file.
func repoWithFile(t *testing.T, name, body string) string {
	t.Helper()
	dir := sparseRepo(t)
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
	return dir
}

// copyFixtureRepo copies a frozen fixture's repo/ tree into a fresh temp
// directory. The frozen tree is an immutable input (testdata/README.md): tests
// copy before they point anything writable at it.
func copyFixtureRepo(t *testing.T, fixture string) string {
	t.Helper()
	src := filepath.Join("..", "..", "testdata", "repositories", "v0.9.2", fixture, "repo")
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("fixture %s: %v", fixture, err)
	}
	dst := filepath.Join(t.TempDir(), "repo")
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copying fixture %s: %v", fixture, err)
	}
	return dst
}

func TestConfigHumanInspection(t *testing.T) {
	_, env := hermeticEnv(t)
	out, errS, code := runEnv(t, env, "diagnostic", "config", "--repo-dir", sparseRepo(t), "--default-branch", "main")
	if code != 0 || errS != "" {
		t.Fatalf("code=%d stderr=%q", code, errS)
	}
	for _, want := range []string{"configuration: valid", "mutation: allowed", "integration_branch = "} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q:\n%s", want, out)
		}
	}
	// metadata_branch is no longer effective configuration (change 0363): Go v1
	// supports one metadata topology, so the effective inspection carries no
	// metadata_branch row.
	if strings.Contains(out, "metadata_branch = ") {
		t.Errorf("stdout still carries a metadata_branch effective row:\n%s", out)
	}
}

func TestConfigJSONInspection(t *testing.T) {
	_, env := hermeticEnv(t)
	out, errS, code := runEnv(t, env, "diagnostic", "config", "--repo-dir", sparseRepo(t), "--default-branch", "main", "--json")
	if code != 0 {
		t.Fatalf("code = %d, want 0 (stdout=%q stderr=%q)", code, out, errS)
	}
	if errS != "" {
		t.Fatalf("stderr = %q, want empty", errS)
	}
	doc := assertOneJSONDocument(t, out)

	// A repository that declares nothing classifies nothing, and capabilities
	// is omitempty — so the key set here is the seven-key shape, never a null.
	assertKeySet(t, doc, "protocol_version", "operation", "result", "source_mode",
		"mutation_allowed", "effective", "diagnostics")

	if doc["protocol_version"] != float64(1) || doc["operation"] != "diagnostic.config" ||
		doc["result"] != "applied" || doc["source_mode"] != "filesystem" || doc["mutation_allowed"] != true {
		t.Fatalf("envelope wrong: %v", doc)
	}
	if _, ok := doc["effective"].(map[string]any); !ok {
		t.Fatalf("effective is not an object: %v", doc["effective"])
	}
}

// The spec's rule: inspection reports a blocked configuration as data under an
// applied result. Only the preflight mode turns the block into a failure.
func TestConfigJSONBlockedInspectionStillApplied(t *testing.T) {
	_, env := hermeticEnv(t)
	repo := copyFixtureRepo(t, "deferred-active")
	out, errS, code := runEnv(t, env, "diagnostic", "config", "--repo-dir", repo, "--default-branch", "main", "--json")
	if code != 0 || errS != "" {
		t.Fatalf("code=%d stderr=%q stdout=%q", code, errS, out)
	}
	doc := assertOneJSONDocument(t, out)
	if doc["operation"] != "diagnostic.config" || doc["result"] != "applied" {
		t.Fatalf("want an applied diagnostic.config, got %v/%v", doc["operation"], doc["result"])
	}
	if doc["mutation_allowed"] != false {
		t.Fatalf("mutation_allowed = %v, want false", doc["mutation_allowed"])
	}
	if len(blockingCapabilities(t, doc)) == 0 {
		t.Fatalf("blocked snapshot reports no mutation-blocking capability: %v", doc["capabilities"])
	}
}

func TestConfigPreflightUnsupported(t *testing.T) {
	_, env := hermeticEnv(t)
	repo := copyFixtureRepo(t, "deferred-active")
	out, errS, code := runEnv(t, env, "diagnostic", "config", "--repo-dir", repo,
		"--default-branch", "main", "--for-mutation", "--json")
	if code != 1 {
		t.Fatalf("code = %d, want 1 (stdout=%q stderr=%q)", code, out, errS)
	}
	if errS != "" {
		t.Fatalf("stderr = %q, want empty", errS)
	}
	doc := assertOneJSONDocument(t, out)
	if doc["operation"] != "config.preflight" || doc["result"] != "unsupported-config" ||
		doc["reason"] != "deferred-capability-requested" {
		t.Fatalf("doc = %v", doc)
	}

	// The refusal must name EVERY blocker, not the first. The expected count
	// is derived from the document's own capability view rather than
	// transcribed from the fixture, so a deferred setting added to the
	// registry cannot silently shrink this assertion.
	wantPaths := blockingCapabilities(t, doc)
	if len(wantPaths) == 0 {
		t.Fatalf("no mutation-blocking capabilities in %v", doc["capabilities"])
	}
	gotPaths := diagnosticPaths(t, doc, "deferred-capability-requested")
	if strings.Join(gotPaths, ",") != strings.Join(wantPaths, ",") {
		t.Fatalf("blocker diagnostics = %v, want %v", gotPaths, wantPaths)
	}
}

func TestConfigPreflightAllowed(t *testing.T) {
	_, env := hermeticEnv(t)
	out, errS, code := runEnv(t, env, "diagnostic", "config", "--repo-dir", sparseRepo(t),
		"--default-branch", "main", "--for-mutation", "--json")
	if code != 0 || errS != "" {
		t.Fatalf("code=%d stderr=%q stdout=%q", code, errS, out)
	}
	doc := assertOneJSONDocument(t, out)
	if doc["operation"] != "config.preflight" || doc["result"] != "applied" || doc["mutation_allowed"] != true {
		t.Fatalf("doc = %v", doc)
	}
}

func TestConfigInvalidInput(t *testing.T) {
	_, env := hermeticEnv(t)
	repo := repoWithFile(t, ".docket.yml", "a: [unclosed\n")
	out, errS, code := runEnv(t, env, "diagnostic", "config", "--repo-dir", repo,
		"--default-branch", "main", "--json")
	if code != 2 {
		t.Fatalf("code = %d, want 2 (stdout=%q stderr=%q)", code, out, errS)
	}
	if errS != "" {
		t.Fatalf("stderr = %q, want empty", errS)
	}
	doc := assertOneJSONDocument(t, out)
	if doc["result"] != "invalid-input" || doc["reason"] != "invalid-config" {
		t.Fatalf("doc = %v", doc)
	}
	// There is no snapshot, so neither view of one may appear.
	for _, absent := range []string{"effective", "capabilities"} {
		if _, ok := doc[absent]; ok {
			t.Errorf("failure document carries %q: %v", absent, doc)
		}
	}
	if _, ok := doc["diagnostics"].([]any); !ok {
		t.Fatalf("diagnostics missing or not an array: %v", doc["diagnostics"])
	}
}

func TestConfigMissingContext(t *testing.T) {
	_, env := hermeticEnv(t)
	out, errS, code := runEnv(t, env, "diagnostic", "config", "--repo-dir", sparseRepo(t), "--json")
	if code != 2 || errS != "" {
		t.Fatalf("code=%d stderr=%q stdout=%q", code, errS, out)
	}
	doc := assertOneJSONDocument(t, out)
	if doc["result"] != "invalid-input" || doc["reason"] != "missing-resolution-context" {
		t.Fatalf("doc = %v", doc)
	}
}

// A missing required flag is a CLI argument error, presented by 0304's
// contract: stderr in human mode, empty stdout, exit 2.
func TestConfigMissingRepoDirFlag(t *testing.T) {
	_, env := hermeticEnv(t)
	out, errS, code := runEnv(t, env, "diagnostic", "config")
	if code != 2 || out != "" {
		t.Fatalf("out=%q code=%d", out, code)
	}
	if !strings.HasPrefix(errS, "docket: ") || !strings.Contains(errS, "repo-dir") {
		t.Fatalf("stderr = %q", errS)
	}
}

// A --repo-dir naming a directory that does not exist is an argument problem,
// not a configuration verdict: without this the mutation gate would certify a
// repository that is not there.
func TestConfigNonexistentRepoDir(t *testing.T) {
	_, env := hermeticEnv(t)
	missing := filepath.Join(t.TempDir(), "no-such-repo")
	out, errS, code := runEnv(t, env, "diagnostic", "config", "--repo-dir", missing,
		"--default-branch", "main", "--for-mutation")
	if code != 2 || out != "" {
		t.Fatalf("out=%q code=%d stderr=%q", out, code, errS)
	}
	if !strings.HasPrefix(errS, "docket: ") || !strings.Contains(errS, missing) {
		t.Fatalf("stderr = %q", errS)
	}
}

// The global layer must be read from the pinned environment: a machine-global
// file placed under the test's own XDG_CONFIG_HOME has to reach the resolver,
// which is simultaneously the proof that the developer's real one never does.
func TestConfigGlobalConfigIsHermetic(t *testing.T) {
	xdgDir, env := hermeticEnv(t)
	globalDir := filepath.Join(xdgDir, "docket")
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		t.Fatalf("creating %s: %v", globalDir, err)
	}
	if err := os.WriteFile(filepath.Join(globalDir, "config.yml"),
		[]byte("auto_capture:\n  enabled: true\n"), 0o644); err != nil {
		t.Fatalf("writing global config: %v", err)
	}

	repo := sparseRepo(t)
	out, errS, code := runEnv(t, env, "diagnostic", "config", "--repo-dir", repo,
		"--default-branch", "main", "--for-mutation", "--json")
	if code != 1 || errS != "" {
		t.Fatalf("code=%d stderr=%q stdout=%q", code, errS, out)
	}
	doc := assertOneJSONDocument(t, out)
	if doc["result"] != "unsupported-config" || doc["mutation_allowed"] != false {
		t.Fatalf("global layer did not reach the resolver: %v", doc)
	}
	if paths := diagnosticPaths(t, doc, "deferred-capability-requested"); len(paths) != 1 || paths[0] != "auto_capture.enabled" {
		t.Fatalf("blockers = %v, want exactly auto_capture.enabled", paths)
	}

	// Same repository, same binary, without the global file: allowed. That
	// difference is what proves the block came from the pinned global layer
	// and not from anything the repository or the built-in defaults carry.
	_, cleanEnv := hermeticEnv(t)
	out, errS, code = runEnv(t, cleanEnv, "diagnostic", "config", "--repo-dir", repo,
		"--default-branch", "main", "--for-mutation", "--json")
	if code != 0 || errS != "" {
		t.Fatalf("without a global file: code=%d stderr=%q stdout=%q", code, errS, out)
	}
}

// assertKeySet proves the document's top-level keys are exactly the wanted set.
func assertKeySet(t *testing.T, doc map[string]any, want ...string) {
	t.Helper()
	wanted := make(map[string]bool, len(want))
	for _, k := range want {
		wanted[k] = true
		if _, ok := doc[k]; !ok {
			t.Errorf("document missing key %q", k)
		}
	}
	for k := range doc {
		if !wanted[k] {
			t.Errorf("document carries unexpected key %q", k)
		}
	}
}

// blockingCapabilities lists, in document order, the paths of every capability
// entry the document itself marks as an active mutation-blocking deferral.
func blockingCapabilities(t *testing.T, doc map[string]any) []string {
	t.Helper()
	entries, ok := doc["capabilities"].([]any)
	if !ok {
		t.Fatalf("capabilities missing or not an array: %v", doc["capabilities"])
	}
	var out []string
	for _, e := range entries {
		c, ok := e.(map[string]any)
		if !ok {
			t.Fatalf("capability entry is not an object: %v", e)
		}
		if c["active"] == true && c["mutation_block"] == true && c["classification"] == "deferred" {
			out = append(out, c["path"].(string))
		}
	}
	return out
}

// diagnosticPaths lists, in document order, the paths of every diagnostic
// carrying the given code.
func diagnosticPaths(t *testing.T, doc map[string]any, code string) []string {
	t.Helper()
	entries, ok := doc["diagnostics"].([]any)
	if !ok {
		t.Fatalf("diagnostics missing or not an array: %v", doc["diagnostics"])
	}
	var out []string
	for _, e := range entries {
		d, ok := e.(map[string]any)
		if !ok {
			t.Fatalf("diagnostic entry is not an object: %v", e)
		}
		if d["code"] == code {
			out = append(out, d["path"].(string))
		}
	}
	return out
}
