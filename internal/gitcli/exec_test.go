package gitcli

import (
	"bytes"
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestMain routes a re-exec of this test binary into helperMain when
// GITCLI_HELPER_MODE is set, so the controlled-execution tests can spawn a
// deterministic fake "git" without a shell script.
func TestMain(m *testing.M) {
	if os.Getenv("GITCLI_HELPER_MODE") != "" {
		helperMain() // never returns; os.Exit inside
	}
	os.Exit(m.Run())
}

// helperMain acts as a fake git process driven by GITCLI_HELPER_MODE. It always
// exits the process itself and never returns to the test runner.
//
//	"dump":   write os.Args[1:] then os.Environ() as NUL-joined records, exit 0
//	"stderr": write GITCLI_HELPER_STDERR_BYTES bytes of GITCLI_HELPER_STDERR_TEXT
//	          (default "x") to stderr, exit 3
//	"block":  ignore args, sleep 30s (killed by timeout/cancel)
//	"exit":   exit with code GITCLI_HELPER_EXIT
//	"script": write GITCLI_HELPER_STDOUT verbatim to stdout, exit 0
func helperMain() {
	switch os.Getenv("GITCLI_HELPER_MODE") {
	case "script":
		os.Stdout.WriteString(os.Getenv("GITCLI_HELPER_STDOUT"))
		os.Exit(0)
	case "dump":
		var buf bytes.Buffer
		for _, a := range os.Args[1:] {
			buf.WriteString(a)
			buf.WriteByte(0)
		}
		for _, e := range os.Environ() {
			buf.WriteString(e)
			buf.WriteByte(0)
		}
		os.Stdout.Write(buf.Bytes())
		os.Exit(0)
	case "stderr":
		n, _ := strconv.Atoi(os.Getenv("GITCLI_HELPER_STDERR_BYTES"))
		if n < 0 {
			n = 0
		}
		text := os.Getenv("GITCLI_HELPER_STDERR_TEXT")
		if text == "" {
			text = "x"
		}
		out := make([]byte, 0, n)
		for len(out) < n {
			out = append(out, text...)
		}
		os.Stderr.Write(out[:n])
		os.Exit(3)
	case "block":
		time.Sleep(30 * time.Second)
		os.Exit(0)
	case "exit":
		code, _ := strconv.Atoi(os.Getenv("GITCLI_HELPER_EXIT"))
		os.Exit(code)
	default:
		os.Exit(0)
	}
}

// helperClient builds a Client whose executable is this test binary re-exec'd in
// the given helper mode, with a fully pinned base environment and short
// timeouts so timeout/cancel tests stay fast.
func helperClient(t *testing.T, mode string, extraEnv ...string) *Client {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(), "GITCLI_HELPER_MODE="+mode)
	env = append(env, extraEnv...)
	c, err := NewClient(WithExecutable(exe), WithBaseEnvironment(env),
		WithLocalTimeout(2*time.Second), WithNetworkTimeout(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// envEntries splits a NUL-joined environment dump into individual NAME=VALUE
// records.
func envEntries(dump string) []string {
	parts := strings.Split(dump, "\x00")
	out := parts[:0]
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func hasPrefixEntry(entries []string, prefix string) bool {
	for _, e := range entries {
		if strings.HasPrefix(e, prefix) {
			return true
		}
	}
	return false
}

func hasExactEntry(entries []string, want string) bool {
	for _, e := range entries {
		if e == want {
			return true
		}
	}
	return false
}

// TestSanitizeRemovesRedirectionClassesKeepsAuthSentinel plants one variable
// from each removed class plus a benign auth sentinel and asserts every planted
// redirection/config/trace var is absent from the child's observed environment,
// the sentinel survives, and the fixed controls are present exactly.
func TestSanitizeRemovesRedirectionClassesKeepsAuthSentinel(t *testing.T) {
	planted := []string{
		"GIT_DIR=/evil", "GIT_COMMON_DIR=/evil", "GIT_WORK_TREE=/evil",
		"GIT_INDEX_FILE=/evil", "GIT_OBJECT_DIRECTORY=/evil",
		"GIT_ALTERNATE_OBJECT_DIRECTORIES=/evil", "GIT_NAMESPACE=evil",
		"GIT_CONFIG_GLOBAL=/evil", "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=a.b",
		"GIT_TRACE=1", "GIT_TRACE2_EVENT=/evil", "GIT_CEILING_DIRECTORIES=/evil",
		"GIT_DISCOVERY_ACROSS_FILESYSTEM=1", "GIT_ALTERNATE_FOO=/evil",
	}
	sentinel := "GIT_SSH_COMMAND=ssh -o BatchMode=yes"
	c := helperClient(t, "dump", append(append([]string{}, planted...), sentinel)...)
	res, f := c.run(context.Background(), runRequest{op: "discover", dir: t.TempDir(), args: []string{"status"}})
	if f != nil {
		t.Fatal(f)
	}
	entries := envEntries(string(res.stdout))
	for _, p := range planted {
		name := p[:strings.IndexByte(p, '=')]
		if hasPrefixEntry(entries, name+"=") {
			t.Errorf("%s leaked into child environment", name)
		}
	}
	if !hasExactEntry(entries, sentinel) {
		t.Error("benign auth sentinel was scrubbed")
	}
	for _, added := range []string{"GIT_TERMINAL_PROMPT=0", "LC_ALL=C", "LANG=C", "GIT_OPTIONAL_LOCKS=0"} {
		if !hasExactEntry(entries, added) {
			t.Errorf("missing added control %s", added)
		}
	}
}

// TestRunTimeoutKillsProcess drives the "block" helper past the 2s local
// timeout and asserts a timed-out Failure with the process actually reaped
// (elapsed well under the 30s helper sleep).
func TestRunTimeoutKillsProcess(t *testing.T) {
	c := helperClient(t, "block")
	start := time.Now()
	_, f := c.run(context.Background(), runRequest{op: "fetch-branch", dir: t.TempDir(), args: []string{"fetch"}})
	elapsed := time.Since(start)
	if f == nil {
		t.Fatal("expected timeout failure, got nil")
	}
	if f.Kind != KindTimedOut {
		t.Fatalf("kind = %q, want %q", f.Kind, KindTimedOut)
	}
	if elapsed >= 10*time.Second {
		t.Fatalf("process not reaped promptly: elapsed %v", elapsed)
	}
}

// TestRunCancelledKind cancels the caller context after 100ms and asserts a
// cancelled (not timed-out) Failure well before the 2s timeout.
func TestRunCancelledKind(t *testing.T) {
	c := helperClient(t, "block")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	time.AfterFunc(100*time.Millisecond, cancel)
	start := time.Now()
	_, f := c.run(ctx, runRequest{op: "fetch-branch", dir: t.TempDir(), args: []string{"fetch"}})
	elapsed := time.Since(start)
	if f == nil {
		t.Fatal("expected cancellation failure, got nil")
	}
	if f.Kind != KindCancelled {
		t.Fatalf("kind = %q, want %q", f.Kind, KindCancelled)
	}
	if elapsed >= 2*time.Second {
		t.Fatalf("cancellation not honored promptly: elapsed %v", elapsed)
	}
}

// TestStderrExcerptBounded asserts a 64 KiB stderr is bounded to <= 1024 bytes
// with an explicit truncation marker, while the raw stderr stays fully
// available in the runResult.
func TestStderrExcerptBounded(t *testing.T) {
	c := helperClient(t, "stderr", "GITCLI_HELPER_STDERR_BYTES=65536")
	res, f := c.run(context.Background(), runRequest{op: "read-blobs", dir: t.TempDir(), args: []string{"cat-file"}})
	if f != nil {
		t.Fatalf("unexpected failure: %v", f)
	}
	if res.exitCode != 3 {
		t.Fatalf("exit code = %d, want 3", res.exitCode)
	}
	if len(res.stderr) != 65536 {
		t.Fatalf("raw stderr len = %d, want 65536", len(res.stderr))
	}
	ex := stderrExcerpt(res.stderr)
	if !strings.HasSuffix(ex, " [truncated]") {
		t.Fatal("excerpt not marked truncated")
	}
	body := strings.TrimSuffix(ex, " [truncated]")
	if len(body) > 1024 {
		t.Fatalf("excerpt body = %d bytes, want <= 1024", len(body))
	}
}

// TestNoEnvironmentInDiagnostics proves the environment never enters a Failure
// diagnostic: a distinct env secret is absent from both a caller-built
// command-failed diagnostic and run's own timeout diagnostic, while the only
// stderr-derived content is the bounded excerpt (so a 64 KiB secret-shaped
// stderr is disclosed only within the <= 1024-byte window).
func TestNoEnvironmentInDiagnostics(t *testing.T) {
	const envSecret = "envsecret-should-never-leak"
	c := helperClient(t, "stderr",
		"SECRET_TOKEN="+envSecret,
		"GITCLI_HELPER_STDERR_BYTES=65536",
		"GITCLI_HELPER_STDERR_TEXT=hunter2")
	res, f := c.run(context.Background(), runRequest{op: "read-blobs", dir: t.TempDir(), args: []string{"cat-file"}})
	if f != nil {
		t.Fatalf("unexpected run failure: %v", f)
	}
	if !bytes.Contains(res.stderr, []byte("hunter2")) || len(res.stderr) != 65536 {
		t.Fatalf("helper stderr not as expected: len=%d", len(res.stderr))
	}
	// Caller builds the command-failed diagnostic exactly as production does:
	// the bounded stderr excerpt is the only stderr-derived content.
	cf := newFailure("read-blobs", KindCommandFailed, stderrExcerpt(res.stderr), nil)
	if strings.Contains(cf.Error(), envSecret) || strings.Contains(cf.Detail, envSecret) {
		t.Fatal("environment value leaked into diagnostic")
	}
	if cf.Detail != stderrExcerpt(res.stderr) {
		t.Fatal("Detail is not exactly the bounded stderr excerpt")
	}
	body := strings.TrimSuffix(cf.Detail, " [truncated]")
	if len(body) > 1024 {
		t.Fatalf("stderr disclosed beyond bounded excerpt: %d bytes", len(body))
	}
	// run's own failure path (timeout) must also never embed the environment.
	bc := helperClient(t, "block", "SECRET_TOKEN="+envSecret)
	_, tf := bc.run(context.Background(), runRequest{op: "fetch-branch", dir: t.TempDir(), args: []string{"fetch"}})
	if tf == nil {
		t.Fatal("expected timeout failure")
	}
	if strings.Contains(tf.Error(), envSecret) {
		t.Fatal("environment value leaked into timeout diagnostic")
	}
}

// TestClientConstructionRejections asserts non-positive timeouts fail
// construction with invalid-request and a missing executable with
// executable-unavailable.
func TestClientConstructionRejections(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		opts []Option
	}{
		{"zero local", []Option{WithExecutable(exe), WithLocalTimeout(0)}},
		{"negative local", []Option{WithExecutable(exe), WithLocalTimeout(-time.Second)}},
		{"zero network", []Option{WithExecutable(exe), WithNetworkTimeout(0)}},
		{"negative network", []Option{WithExecutable(exe), WithNetworkTimeout(-time.Second)}},
	}
	for _, tc := range cases {
		_, err := NewClient(tc.opts...)
		if err == nil {
			t.Fatalf("%s: expected error, got nil", tc.name)
		}
		fail, ok := AsFailure(err)
		if !ok || fail.Kind != KindInvalidRequest {
			t.Fatalf("%s: got %v, want invalid-request", tc.name, err)
		}
	}
	_, err = NewClient(WithExecutable("/nonexistent/path/to/git-binary"))
	if err == nil {
		t.Fatal("expected executable-unavailable, got nil")
	}
	fail, ok := AsFailure(err)
	if !ok || fail.Kind != KindExecutableUnavailable {
		t.Fatalf("got %v, want executable-unavailable", err)
	}
}
