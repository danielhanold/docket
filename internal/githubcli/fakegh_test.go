package githubcli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestMain routes a re-exec of this test binary into fakeGHMain when
// GO_WANT_FAKE_GH is set, so the protocol-faithful GitHub tests can spawn a
// deterministic fake "gh" without a shell script and without a live network.
func TestMain(m *testing.M) {
	if os.Getenv("GO_WANT_FAKE_GH") != "" {
		fakeGHMain() // never returns; os.Exit inside
	}
	os.Exit(m.Run())
}

// fakeExit codes distinguish the fake's own protocol failures from a scripted
// non-zero gh exit. An unmatched invocation is a hard test-authoring error, not
// a GitHub condition, so it never collides with gh's own 0/1 space.
const (
	fakeExitUnmatched = 64 // no scenario arm matched the invocation
	fakeExitInternal  = 65 // the fake could not read its own scenario/log
)

// fakeScenario is the on-disk contract a test hands the fake gh: an ordered list
// of invocation arms. The FIRST arm whose ArgvPrefix is a prefix of the child's
// argument vector wins; a call matching no arm exits fakeExitUnmatched with a
// diagnostic, so an unexpected gh call can never silently succeed (a catch-all
// exit 0 is forbidden by the spec).
type fakeScenario struct {
	Invocations []fakeArm `json:"invocations"`
}

// fakeArm is one scripted response. Stdout carries the EXACT nested JSON field
// shapes real `gh --json` documents — never a flattened fake-only shape.
type fakeArm struct {
	ArgvPrefix []string `json:"argvPrefix"`
	Stdout     string   `json:"stdout"`
	Stderr     string   `json:"stderr"`
	Exit       int      `json:"exit"`
	DelayMs    int      `json:"delayMs"`
}

// invocationRecord is one entry the fake appends to the NUL-delimited witness
// log: the full argument vector, the working directory the child ran in, the
// complete stdin bytes it received, and the GitHub-relevant environment keys it
// observed. Recording invocation side effects SEPARATELY from stdout is what
// lets a test prove a create/edit actually happened rather than inferring it
// from decoded output.
type invocationRecord struct {
	Argv  []string          `json:"argv"`
	Cwd   string            `json:"cwd"`
	Stdin string            `json:"stdin"`
	Env   map[string]string `json:"env"`
}

// fakeGHMain is the fake gh process. It records the invocation to the witness
// log FIRST (so a test sees the call even when the process is later reaped mid
// delay), then honors the scripted delay, then writes an optional completion
// marker, then emits the scripted stdout/stderr and exits with the scripted
// code. It always exits the process itself and never returns to the runner.
func fakeGHMain() {
	scenarioPath := os.Getenv("FAKE_GH_SCENARIO")
	raw, err := os.ReadFile(scenarioPath)
	if err != nil {
		os.Exit(fakeExitInternal)
	}
	var scenario fakeScenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		os.Exit(fakeExitInternal)
	}

	argv := os.Args[1:]
	// Child stdin is /dev/null when the parent set none, so this returns EOF
	// immediately and never blocks.
	stdin, _ := io.ReadAll(os.Stdin)
	cwd, _ := os.Getwd()

	if logPath := os.Getenv("FAKE_GH_LOG"); logPath != "" {
		rec := invocationRecord{Argv: argv, Cwd: cwd, Stdin: string(stdin), Env: fakeCapturedEnv()}
		if err := appendInvocation(logPath, rec); err != nil {
			os.Exit(fakeExitInternal)
		}
	}

	arm, ok := matchArm(scenario, argv)
	if !ok {
		os.Stderr.WriteString("fake gh: no scenario arm matched invocation: " + strings.Join(argv, " ") + "\n")
		os.Exit(fakeExitUnmatched)
	}

	if arm.DelayMs > 0 {
		time.Sleep(time.Duration(arm.DelayMs) * time.Millisecond)
	}
	// The completion marker is written only after the full delay: a test that
	// drives the deadline past DelayMs proves the child was reaped by observing
	// this file's ABSENCE.
	if donePath := os.Getenv("FAKE_GH_DONE"); donePath != "" {
		_ = os.WriteFile(donePath, []byte("done"), 0o600)
	}
	if arm.Stderr != "" {
		os.Stderr.WriteString(arm.Stderr)
	}
	os.Stdout.WriteString(arm.Stdout)
	os.Exit(arm.Exit)
}

// fakeCapturedEnv records every GitHub-relevant environment key the child
// observed: the GH_*/GITHUB_* families. This is how the env-hygiene test proves
// GH_REPO/GH_HOST were stripped while an auth token survived.
func fakeCapturedEnv() map[string]string {
	out := map[string]string{}
	for _, kv := range os.Environ() {
		i := strings.IndexByte(kv, '=')
		if i < 0 {
			continue
		}
		name := kv[:i]
		if strings.HasPrefix(name, "GH_") || strings.HasPrefix(name, "GITHUB_") {
			out[name] = kv[i+1:]
		}
	}
	return out
}

// matchArm returns the first arm whose ArgvPrefix is a leading prefix of argv.
// An arm with an empty prefix would match everything (a catch-all), so it is
// rejected — the fake must never answer an unexpected shape.
func matchArm(s fakeScenario, argv []string) (fakeArm, bool) {
	for _, arm := range s.Invocations {
		if len(arm.ArgvPrefix) == 0 {
			continue
		}
		if hasArgvPrefix(argv, arm.ArgvPrefix) {
			return arm, true
		}
	}
	return fakeArm{}, false
}

func hasArgvPrefix(argv, prefix []string) bool {
	if len(prefix) > len(argv) {
		return false
	}
	for i := range prefix {
		if argv[i] != prefix[i] {
			return false
		}
	}
	return true
}

// appendInvocation writes one NUL-delimited JSON record. NUL delimiting keeps
// records unambiguous even though the payload is JSON (which never contains a
// raw NUL byte).
func appendInvocation(path string, rec invocationRecord) error {
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	b = append(b, 0)
	_, err = f.Write(b)
	return err
}

// witnessLog reads the fake's NUL-delimited invocation log back into records.
type witnessLog struct {
	path string
}

// records parses the witness log; a missing file (no invocation yet) is an empty
// slice, not an error.
func (w *witnessLog) records(t *testing.T) []invocationRecord {
	t.Helper()
	raw, err := os.ReadFile(w.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatalf("reading witness log: %v", err)
	}
	var out []invocationRecord
	for _, field := range bytes.Split(raw, []byte{0}) {
		if len(field) == 0 {
			continue
		}
		var rec invocationRecord
		if err := json.Unmarshal(field, &rec); err != nil {
			t.Fatalf("decoding witness record %q: %v", field, err)
		}
		out = append(out, rec)
	}
	return out
}

// newFakeClient builds a Client whose executable is this test binary re-exec'd
// as the fake gh, driven by the given scenario, with a fully pinned base
// environment. It returns the client and a witnessLog over the invocation
// record. extraEnv is appended to the pinned base (used to plant GH_REPO/GH_HOST
// for the hygiene test); pass timeouts via opts to override the test defaults.
func newFakeClient(t *testing.T, scenario fakeScenario, opts ...clientOverride) (*Client, *witnessLog) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	scenarioPath := filepath.Join(dir, "scenario.json")
	raw, err := json.Marshal(scenario)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(scenarioPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, "invocations.log")

	cfg := fakeClientConfig{
		extraEnv:       nil,
		localTimeout:   2 * time.Second,
		networkTimeout: 2 * time.Second,
		donePath:       "",
	}
	for _, o := range opts {
		o(&cfg)
	}

	env := []string{
		"GO_WANT_FAKE_GH=1",
		"FAKE_GH_SCENARIO=" + scenarioPath,
		"FAKE_GH_LOG=" + logPath,
		"PATH=" + os.Getenv("PATH"),
	}
	if cfg.donePath != "" {
		env = append(env, "FAKE_GH_DONE="+cfg.donePath)
	}
	env = append(env, cfg.extraEnv...)

	c, err := NewClient(
		WithExecutable(exe),
		WithBaseEnvironment(env),
		WithLocalTimeout(cfg.localTimeout),
		WithNetworkTimeout(cfg.networkTimeout),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c, &witnessLog{path: logPath}
}

type fakeClientConfig struct {
	extraEnv       []string
	localTimeout   time.Duration
	networkTimeout time.Duration
	donePath       string
}

type clientOverride func(*fakeClientConfig)

func withExtraEnv(env ...string) clientOverride {
	return func(c *fakeClientConfig) { c.extraEnv = append(c.extraEnv, env...) }
}

func withNetworkTimeout(d time.Duration) clientOverride {
	return func(c *fakeClientConfig) { c.networkTimeout = d }
}

func withDonePath(p string) clientOverride {
	return func(c *fakeClientConfig) { c.donePath = p }
}

// prViewArm builds a scenario arm for `gh pr view` that emits the exact nested
// JSON gh documents for the given PR fields.
func prViewArm(j string) fakeArm {
	return fakeArm{ArgvPrefix: []string{"pr", "view"}, Stdout: j, Exit: 0}
}

// --- Witness tests on the fake itself (Task 9 step 1 a–d) ---

// samplePRJSON is the canonical nested shape `gh pr view --json ...` documents.
const samplePRJSON = `{
  "number": 7,
  "url": "https://github.com/acme/widget/pull/7",
  "state": "OPEN",
  "isDraft": false,
  "headRefName": "feat/x",
  "headRefOid": "1111111111111111111111111111111111111111",
  "baseRefName": "main",
  "title": "Add widget",
  "body": "Body text"
}`

// TestFakePassThrough (a): a scripted pr view invoked directly through Client
// yields the scripted stdout, decoded into the typed PullRequest.
func TestFakePassThrough(t *testing.T) {
	c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{prViewArm(samplePRJSON)}})
	res, f := c.run(context.Background(), runRequest{
		op:      "probe",
		dir:     t.TempDir(),
		args:    []string{"pr", "view", "7", "--repo", "github.com/acme/widget", "--json", "number,url,state,isDraft,headRefName,headRefOid,baseRefName,title,body"},
		network: true,
	})
	if f != nil {
		t.Fatalf("run failed: %v", f)
	}
	if res.exitCode != 0 {
		t.Fatalf("exit = %d, want 0", res.exitCode)
	}
	pr, err := decodePullRequest("probe", res.stdout)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if pr.Number != 7 || pr.State != StateOpen || pr.HeadBranch != "feat/x" || pr.Title != "Add widget" {
		t.Fatalf("decoded PR mismatch: %+v", pr)
	}
}

// TestFakeInvocationWitness (b): the log records exact argv, cwd, and stdin.
func TestFakeInvocationWitness(t *testing.T) {
	c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
		{ArgvPrefix: []string{"pr", "create"}, Stdout: samplePRJSON, Exit: 0},
	}})
	dir := t.TempDir()
	body := "line one\nline two\n"
	args := []string{"pr", "create", "--repo", "github.com/acme/widget", "--head", "feat/x", "--base", "main", "--title", "Add widget", "--body-file", "-"}
	res, f := c.run(context.Background(), runRequest{op: "create", dir: dir, args: args, stdin: []byte(body), network: true})
	if f != nil {
		t.Fatalf("run failed: %v", f)
	}
	if res.exitCode != 0 {
		t.Fatalf("exit = %d", res.exitCode)
	}
	recs := log.records(t)
	if len(recs) != 1 {
		t.Fatalf("witness records = %d, want 1", len(recs))
	}
	rec := recs[0]
	if strings.Join(rec.Argv, " ") != strings.Join(args, " ") {
		t.Fatalf("argv = %v, want %v", rec.Argv, args)
	}
	if rec.Stdin != body {
		t.Fatalf("stdin = %q, want %q", rec.Stdin, body)
	}
	// cwd is canonical; compare after resolving symlinks (macOS /tmp).
	wantCwd, _ := filepath.EvalSymlinks(dir)
	gotCwd, _ := filepath.EvalSymlinks(rec.Cwd)
	if gotCwd != wantCwd {
		t.Fatalf("cwd = %q, want %q", gotCwd, wantCwd)
	}
	// The authored body must reach gh via stdin, NEVER argv.
	for _, a := range rec.Argv {
		if strings.Contains(a, "line one") {
			t.Fatal("body leaked into argv")
		}
	}
}

// TestFakeUnmatchedInvocationFails (c): deleting a scenario's dispatch arm (an
// invocation no arm matches) makes the fake exit fakeExitUnmatched with a
// diagnostic — an unexpected call can never silently succeed.
func TestFakeUnmatchedInvocationFails(t *testing.T) {
	// Scenario only knows `pr view`; we invoke `pr edit`.
	c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{prViewArm(samplePRJSON)}})
	res, f := c.run(context.Background(), runRequest{
		op:      "edit",
		dir:     t.TempDir(),
		args:    []string{"pr", "edit", "7", "--repo", "github.com/acme/widget"},
		network: true,
	})
	if f != nil {
		t.Fatalf("run failed unexpectedly: %v", f)
	}
	if res.exitCode != fakeExitUnmatched {
		t.Fatalf("exit = %d, want %d (unmatched)", res.exitCode, fakeExitUnmatched)
	}
	if !bytes.Contains(res.stderr, []byte("no scenario arm matched")) {
		t.Fatalf("missing diagnostic; stderr = %q", res.stderr)
	}
}

// TestFakeCatchAllArmRejected proves the fake refuses a catch-all: an arm with
// an empty ArgvPrefix never matches, so it cannot become a silent exit-0
// answer for arbitrary calls.
func TestFakeCatchAllArmRejected(t *testing.T) {
	c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
		{ArgvPrefix: []string{}, Stdout: "should never be served", Exit: 0},
	}})
	res, f := c.run(context.Background(), runRequest{
		op:      "probe",
		dir:     t.TempDir(),
		args:    []string{"pr", "view", "7"},
		network: true,
	})
	if f != nil {
		t.Fatalf("run failed: %v", f)
	}
	if res.exitCode != fakeExitUnmatched {
		t.Fatalf("catch-all arm matched: exit = %d, want %d", res.exitCode, fakeExitUnmatched)
	}
}

// TestFakeDelayReapedOnTimeout (d): a DelayMs beyond WithNetworkTimeout produces
// a timed-out Failure and the child is reaped — proven by the completion marker
// being absent (the fake writes it only AFTER the full delay).
func TestFakeDelayReapedOnTimeout(t *testing.T) {
	donePath := filepath.Join(t.TempDir(), "done.marker")
	c, log := newFakeClient(t,
		fakeScenario{Invocations: []fakeArm{
			{ArgvPrefix: []string{"pr", "view"}, Stdout: samplePRJSON, Exit: 0, DelayMs: 4000},
		}},
		withNetworkTimeout(200*time.Millisecond),
		withDonePath(donePath),
	)
	start := time.Now()
	_, f := c.run(context.Background(), runRequest{
		op:      "probe",
		dir:     t.TempDir(),
		args:    []string{"pr", "view", "7"},
		network: true,
	})
	elapsed := time.Since(start)
	if f == nil {
		t.Fatal("expected timed-out failure, got nil")
	}
	if f.Kind != KindTimedOut {
		t.Fatalf("kind = %q, want %q", f.Kind, KindTimedOut)
	}
	if elapsed >= 3*time.Second {
		t.Fatalf("process not reaped promptly: elapsed %v", elapsed)
	}
	// The invocation was recorded (fake logs before the delay)...
	if len(log.records(t)) != 1 {
		t.Fatalf("expected 1 witnessed invocation, got %d", len(log.records(t)))
	}
	// ...but the completion marker must be absent: the child never finished the
	// delay, proving it was killed rather than left running.
	if _, err := os.Stat(donePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("completion marker present (child not reaped): stat err = %v", err)
	}
}
