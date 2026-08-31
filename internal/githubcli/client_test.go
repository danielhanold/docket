package githubcli

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// Network sites gated by the read/write budget split (mechanical enumeration,
// `grep -rn "network: *true" internal/githubcli/ --include='*.go' | grep -v _test`):
//
//	merge.go        MergePullRequest    pr merge          WRITE
//	merge.go        probeMergeSnapshot  pr view           READ  (verifyMerge/ProbeMerged reprobe)
//	ensure.go       createRequest       pr create         WRITE
//	ensure.go       editRequest         pr edit           WRITE
//	ensure.go       probeByHead         pr list --all     READ
//	ensure.go       verifyListOpen      pr list --open    READ
//	ensure.go       verifyViewByNumber  pr view           READ
//	retarget.go     RetargetPullRequest pr edit --base    WRITE
//	retarget.go     viewPullRequest     pr view           READ  (verifyRetarget reprobe)
//	comment.go      EnsureComment       pr comment        WRITE
//	comment.go      FindComment         pr view --comments READ
//	mergemethod.go  merge-method probes repo/api reads    READ
//	probe.go        probe reads         pr/repo reads     READ
//	repo.go         repo discovery      repo view         READ
//
// Writes are the gh invocations that MUTATE GitHub state: the merge in
// MergePullRequest, the create/edit in EnsurePullRequest/mutateAndVerify, the
// edit --base in RetargetPullRequest, and the comment post in EnsureComment.
// Their verification/reprobe queries (verifyMerge, verifyPostMutation,
// viewPullRequest, FindComment) and every discovery probe are reads.

// TestNetworkReadWriteTimeoutOptions asserts the two new options resolve to
// their explicit values on the accessors.
func TestNetworkReadWriteTimeoutOptions(t *testing.T) {
	c, err := NewClient(WithExecutable(mustSelfExe(t)),
		WithNetworkReadTimeout(30*time.Second), WithNetworkWriteTimeout(60*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if got := c.NetworkReadTimeout(); got != 30*time.Second {
		t.Fatalf("read timeout = %v", got)
	}
	if got := c.NetworkWriteTimeout(); got != 60*time.Second {
		t.Fatalf("write timeout = %v", got)
	}
}

// TestNetworkTimeoutDefaultsInheritBase proves that without the new options both
// budgets are the existing network default, so every standalone client is
// behaviorally unchanged.
func TestNetworkTimeoutDefaultsInheritBase(t *testing.T) {
	c, err := NewClient(WithExecutable(mustSelfExe(t)))
	if err != nil {
		t.Fatal(err)
	}
	if c.NetworkReadTimeout() != defaultNetworkTimeout || c.NetworkWriteTimeout() != defaultNetworkTimeout {
		t.Fatalf("defaults changed: read=%v write=%v", c.NetworkReadTimeout(), c.NetworkWriteTimeout())
	}
}

// TestNetworkReadWriteTimeoutInheritExplicitBase proves the budgets inherit the
// explicitly-set WithNetworkTimeout base when the read/write options are absent,
// so a client tuned only through the legacy option keeps one shared budget.
func TestNetworkReadWriteTimeoutInheritExplicitBase(t *testing.T) {
	c, err := NewClient(WithExecutable(mustSelfExe(t)), WithNetworkTimeout(90*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if c.NetworkReadTimeout() != 90*time.Second || c.NetworkWriteTimeout() != 90*time.Second {
		t.Fatalf("did not inherit base: read=%v write=%v", c.NetworkReadTimeout(), c.NetworkWriteTimeout())
	}
}

// TestNonPositiveReadWriteTimeoutRejected asserts an explicitly non-positive read
// or write budget is rejected at construction — the zero default only inherits
// when the option is absent, never when it is passed explicitly.
func TestNonPositiveReadWriteTimeoutRejected(t *testing.T) {
	exe := mustSelfExe(t)
	if _, err := NewClient(WithExecutable(exe), WithNetworkReadTimeout(0)); err == nil {
		t.Fatal("zero read timeout accepted")
	}
	if _, err := NewClient(WithExecutable(exe), WithNetworkWriteTimeout(-time.Second)); err == nil {
		t.Fatal("negative write timeout accepted")
	}
}

// TestRunSelectsReadVsWriteNetworkBudget proves runRequest.write chooses the
// write budget and its absence chooses the read budget. With a short read budget
// and a long write budget against a fake gh that sleeps a fixed interval longer
// than the read budget but shorter than the write budget, a read request must
// time out while a write request survives. No wall-clock equality: the assert is
// only which request timed out; no test sleeps 30/60s.
func TestRunSelectsReadVsWriteNetworkBudget(t *testing.T) {
	script := writeSleepingGH(t, 200*time.Millisecond)
	c, err := NewClient(WithExecutable(script),
		WithLocalTimeout(5*time.Second),
		WithNetworkReadTimeout(50*time.Millisecond),
		WithNetworkWriteTimeout(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}

	// A read request under the 50ms read budget cannot outlast the 200ms sleep.
	_, f := c.run(t.Context(), runRequest{op: "test-read", args: []string{"read"}, network: true})
	if f == nil {
		t.Fatal("read request under the short read budget did not time out")
	}
	if f.Kind != KindTimedOut {
		t.Fatalf("read request failure kind = %q, want %q", f.Kind, KindTimedOut)
	}

	// A write request under the 5s write budget survives the same 200ms sleep.
	res, f := c.run(t.Context(), runRequest{op: "test-write", args: []string{"write"}, network: true, write: true})
	if f != nil {
		t.Fatalf("write request under the long write budget failed: %v", f)
	}
	if res.exitCode != 0 {
		t.Fatalf("write request exit code = %d, want 0", res.exitCode)
	}
}

// mustSelfExe returns this test binary's path — a real executable that
// exec.LookPath resolves — for construction tests that only inspect accessors and
// never invoke gh.
func mustSelfExe(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return exe
}

// writeSleepingGH writes an executable shell script that sleeps for d then exits
// 0, standing in for a slow gh so the read/write budget selection is observable
// without a live network or a 30/60s wall-clock wait.
func writeSleepingGH(t *testing.T, d time.Duration) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-gh")
	secs := d.Seconds()
	script := "#!/bin/sh\nsleep " + strconv.FormatFloat(secs, 'f', 3, 64) + "\nexit 0\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestNewClientResolvesOrFails (e): NewClient with no executable and an empty
// PATH fails; an injected fake executable succeeds.
func TestNewClientResolvesOrFails(t *testing.T) {
	t.Setenv("PATH", "")
	if _, err := NewClient(); err == nil {
		t.Fatal("expected error resolving gh with empty PATH, got nil")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewClient(WithExecutable(exe)); err != nil {
		t.Fatalf("injected executable should succeed: %v", err)
	}
}

// TestNewClientRejectsNonPositiveTimeouts asserts a non-positive local or
// network timeout is an invalid-input construction failure.
func TestNewClientRejectsNonPositiveTimeouts(t *testing.T) {
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
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewClient(tc.opts...)
			f, ok := AsFailure(err)
			if !ok || f.Kind != KindInvalidInput {
				t.Fatalf("got %v, want invalid-input", err)
			}
		})
	}
}

// TestRedactSecretsStripsTokensAndURLs proves diagnostic redaction keys on token
// and URL SHAPE, not an enumerated host/spelling list: every gh token family, an
// Authorization header, and a credentialed transport URL collapse to markers,
// while prose that merely contains a colon survives.
func TestRedactSecretsStripsTokensAndURLs(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		leaked string
	}{
		{"oauth token", "boom gho_0123456789abcdefABCDEF here", "gho_0123456789abcdefABCDEF"},
		{"personal token", "ghp_ABCdef0123456789ABCdef0123456789 fail", "ghp_ABCdef0123456789ABCdef0123456789"},
		{"fine-grained pat", "github_pat_11ABCDE0123_aBcDeF0123456789 x", "github_pat_11ABCDE0123_aBcDeF0123456789"},
		{"authorization header", "Authorization: Bearer s0m3-t0ken-value", "s0m3-t0ken-value"},
		{"credentialed url", "unable to access 'https://x:s3cr3t@github.example/r.git'", "s3cr3t"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := redactSecrets(tc.in)
			if strings.Contains(got, tc.leaked) {
				t.Fatalf("secret survived redaction: %q -> %q", tc.in, got)
			}
		})
	}
	if redactSecrets("fatal: could not find ref refs/heads/x") != "fatal: could not find ref refs/heads/x" {
		t.Fatal("benign prose was mangled by redaction")
	}
}

// TestStderrExcerptBounded asserts a large stderr is redacted then bounded to the
// 512-byte excerpt policy with an explicit truncation marker.
func TestStderrExcerptBounded(t *testing.T) {
	big := strings.Repeat("x", 4096)
	ex := stderrExcerpt([]byte(big))
	if !strings.HasSuffix(ex, " [truncated]") {
		t.Fatalf("excerpt not marked truncated: %q", ex)
	}
	body := strings.TrimSuffix(ex, " [truncated]")
	if len(body) > stderrExcerptLimit {
		t.Fatalf("excerpt body = %d bytes, want <= %d", len(body), stderrExcerptLimit)
	}
}

// TestStderrExcerptRuneBoundary asserts a byte-limit cut landing mid-rune yields
// valid UTF-8, and that whole-rune input is otherwise unaffected.
func TestStderrExcerptRuneBoundary(t *testing.T) {
	// "世" is 3 bytes; pad so the cut at stderrExcerptLimit lands inside it.
	padded := strings.Repeat("a", stderrExcerptLimit-1) + "世" + strings.Repeat("a", stderrExcerptLimit)
	ex := stderrExcerpt([]byte(padded))
	if !utf8.ValidString(ex) {
		t.Fatalf("mid-rune cut produced invalid UTF-8: %q", ex)
	}
	// A whole-rune boundary at the limit must not lose a valid rune.
	aligned := strings.Repeat("a", stderrExcerptLimit) + strings.Repeat("世", 10)
	ex2 := stderrExcerpt([]byte(aligned))
	if !utf8.ValidString(ex2) {
		t.Fatalf("whole-rune input produced invalid UTF-8: %q", ex2)
	}
	body := strings.TrimSuffix(ex2, " [truncated]")
	if body != strings.Repeat("a", stderrExcerptLimit) {
		t.Fatalf("whole-rune prefix altered: %q", body)
	}
}
