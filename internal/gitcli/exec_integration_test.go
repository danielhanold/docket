//go:build integration

package gitcli

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

// TestSanitizeRemovesRedirectionClassesKeepsAuthSentinel plants one variable
// from each removed class plus a benign auth sentinel and asserts every planted
// redirection/config/trace var is absent from the child's observed environment,
// the sentinel survives, and the fixed controls are present exactly.
func TestIntegrationProcessSanitizeRemovesRedirectionClassesKeepsAuthSentinel(t *testing.T) {
	planted := []string{
		"GIT_DIR=/evil", "GIT_COMMON_DIR=/evil", "GIT_WORK_TREE=/evil",
		"GIT_INDEX_FILE=/evil", "GIT_OBJECT_DIRECTORY=/evil",
		"GIT_ALTERNATE_OBJECT_DIRECTORIES=/evil", "GIT_NAMESPACE=evil",
		"GIT_CONFIG_GLOBAL=/evil", "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=a.b",
		"GIT_TRACE=1", "GIT_TRACE2_EVENT=/evil", "GIT_CEILING_DIRECTORIES=/evil",
		"GIT_DISCOVERY_ACROSS_FILESYSTEM=1", "GIT_ALTERNATE_FOO=/evil",
		// GIT_EXEC_PATH redirects where git resolves its own helper binaries
		// (git-remote-https, git-fetch-pack): a redirection-class variable.
		"GIT_EXEC_PATH=/evil",
		// The pathspec-magic family: scrubbed by the _PATHSPECS suffix shape so
		// the appended GIT_LITERAL_PATHSPECS=1 stands unopposed (git rejects a
		// literal setting combined with any other global pathspec setting).
		"GIT_ICASE_PATHSPECS=1", "GIT_GLOB_PATHSPECS=1", "GIT_NOGLOB_PATHSPECS=1",
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
	// The child's effective pathspec behavior is literal: the only *_PATHSPECS
	// variable present is the pinned GIT_LITERAL_PATHSPECS=1 control.
	for _, added := range []string{"GIT_TERMINAL_PROMPT=0", "LC_ALL=C", "LANG=C", "GIT_OPTIONAL_LOCKS=0", "GIT_LITERAL_PATHSPECS=1"} {
		if !hasExactEntry(entries, added) {
			t.Errorf("missing added control %s", added)
		}
	}
}

// TestRunTimeoutKillsProcess drives the "block" helper past the 2s local
// timeout and asserts a timed-out Failure with the process actually reaped
// (elapsed well under the 30s helper sleep).
func TestIntegrationProcessRunTimeoutKillsProcess(t *testing.T) {
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
func TestIntegrationProcessRunCancelledKind(t *testing.T) {
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
func TestIntegrationProcessStderrExcerptBounded(t *testing.T) {
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
func TestIntegrationProcessNoEnvironmentInDiagnostics(t *testing.T) {
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

// TestNoRemoteURLInDiagnostics is the spec's planted-secret probe for the remote
// URL channel: a real command-failed diagnostic built exactly as production
// builds it must disclose neither the URL nor the credential inside it.
func TestIntegrationProcessNoRemoteURLInDiagnostics(t *testing.T) {
	const secret = "urlsecret-should-never-leak"
	stderrText := "fatal: unable to access 'https://x:" + secret + "@example.invalid/r.git/': 403 "
	c := helperClient(t, "stderr",
		"GITCLI_HELPER_STDERR_BYTES=65536",
		"GITCLI_HELPER_STDERR_TEXT="+stderrText)
	res, f := c.run(context.Background(), runRequest{op: "fetch-branch", dir: t.TempDir(), args: []string{"fetch"}})
	if f != nil {
		t.Fatalf("unexpected run failure: %v", f)
	}
	if !bytes.Contains(res.stderr, []byte(secret)) {
		t.Fatal("fixture stderr does not carry the planted secret")
	}
	cf := newFailure("fetch-branch", KindCommandFailed, "git fetch failed: "+stderrExcerpt(res.stderr), nil)
	if strings.Contains(cf.Error(), secret) || strings.Contains(cf.Detail, secret) {
		t.Fatal("remote-URL credential leaked into diagnostic")
	}
	if strings.Contains(cf.Detail, "example.invalid") {
		t.Fatal("remote host leaked into diagnostic")
	}
	if !strings.Contains(cf.Detail, redactedURL) {
		t.Fatal("redaction marker absent: the URL was not recognized at all")
	}
}

// TestFailureCarriesChildExitCode proves a failure classified from a non-zero
// child exit records the status it was classified from, rather than leaving the
// declared ExitCode field permanently zero.
func TestIntegrationProcessFailureCarriesChildExitCode(t *testing.T) {
	c := helperClient(t, "exit", "GITCLI_HELPER_EXIT=7")
	_, err := c.ResolveRef(context.Background(), Repository{PrimaryWorktree: t.TempDir()}, "refs/heads/main")
	f, ok := AsFailure(err)
	if !ok {
		t.Fatalf("expected a *Failure, got %v", err)
	}
	if f.Kind != KindRefUnavailable {
		t.Fatalf("kind = %q, want %q", f.Kind, KindRefUnavailable)
	}
	if f.ExitCode != 7 {
		t.Fatalf("ExitCode = %d, want 7", f.ExitCode)
	}
}

// TestRunWaitDelayBoundsPipeHoldingGrandchild drives the "orphan" helper, whose
// grandchild inherits the capture pipe and outlives the killed child. Without
// cmd.WaitDelay, cmd.Run blocks on that pipe until the grandchild exits (20s)
// despite the 2s deadline having fired long before.
func TestIntegrationProcessRunWaitDelayBoundsPipeHoldingGrandchild(t *testing.T) {
	c := helperClient(t, "orphan", "GITCLI_HELPER_HOLD_MS=20000")
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
		t.Fatalf("Wait blocked on the grandchild's pipe: elapsed %v", elapsed)
	}
}
