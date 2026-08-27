package gitcli

import (
	"bytes"
	"os"
	"os/exec"
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
//	"script": serve canned stdout, exit 0. When GITCLI_HELPER_LSTREE_FILE /
//	          GITCLI_HELPER_CATFILE_FILE is set AND the argument vector names
//	          that subcommand (ls-tree / cat-file), that file's raw bytes are
//	          served — this lets a single fake git answer the two-process
//	          ReadBlobs pipeline (ls-tree resolve, then cat-file batch) with
//	          independent payloads. Otherwise GITCLI_HELPER_STDOUT_FILE's raw
//	          bytes (if set — the only way to deliver NUL-delimited output an
//	          env var cannot hold), else GITCLI_HELPER_STDOUT verbatim.
//
// Spawn logging is orthogonal to the mode: when GITCLI_HELPER_SPAWNLOG names a
// file, every helper invocation appends its argument vector as one line before
// dispatching, so a test can count how many git processes an operation spawned.
func helperMain() {
	if logPath := os.Getenv("GITCLI_HELPER_SPAWNLOG"); logPath != "" {
		f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			os.Exit(5)
		}
		_, _ = f.WriteString(strings.Join(os.Args[1:], " ") + "\n")
		_ = f.Close()
	}
	switch os.Getenv("GITCLI_HELPER_MODE") {
	case "script":
		args := os.Args[1:]
		if argsContain(args, "ls-tree") {
			if path := os.Getenv("GITCLI_HELPER_LSTREE_FILE"); path != "" {
				serveFileOrExit(path)
			}
		}
		if argsContain(args, "cat-file") {
			if path := os.Getenv("GITCLI_HELPER_CATFILE_FILE"); path != "" {
				serveFileOrExit(path)
			}
		}
		if path := os.Getenv("GITCLI_HELPER_STDOUT_FILE"); path != "" {
			serveFileOrExit(path)
		}
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
	case "hold":
		// A grandchild that inherited its parent's stdout/stderr and outlives
		// it, keeping the write end of the capture pipe open.
		ms, _ := strconv.Atoi(os.Getenv("GITCLI_HELPER_HOLD_MS"))
		if ms <= 0 {
			ms = 20000
		}
		time.Sleep(time.Duration(ms) * time.Millisecond)
		os.Exit(0)
	case "orphan":
		// Spawn a pipe-holding grandchild, then block until killed. The
		// grandchild inherits this process's stdout/stderr, so after the
		// deadline kills this process the capture pipe stays open and only
		// cmd.WaitDelay can unblock the parent's Wait.
		sub := exec.Command(os.Args[0])
		sub.Env = append(os.Environ(), "GITCLI_HELPER_MODE=hold")
		sub.Stdout = os.Stdout
		sub.Stderr = os.Stderr
		if err := sub.Start(); err != nil {
			os.Exit(6)
		}
		time.Sleep(30 * time.Second)
		os.Exit(0)
	case "fetchslowfail":
		// Fake a remote that answers `fetch` slowly and non-zero, then hangs on
		// the ls-remote classification probe — the shape that makes an
		// unshared budget cost two network timeouts instead of one.
		args := os.Args[1:]
		switch {
		case argsContain(args, "fetch"):
			ms, _ := strconv.Atoi(os.Getenv("GITCLI_HELPER_FETCH_SLEEP_MS"))
			time.Sleep(time.Duration(ms) * time.Millisecond)
			os.Stderr.WriteString("fatal: could not read from remote repository\n")
			os.Exit(128)
		case argsContain(args, "ls-remote"):
			time.Sleep(30 * time.Second)
			os.Exit(0)
		default: // `remote get-url` and anything else: succeed cheaply.
			os.Stdout.WriteString("https://example.invalid/repo.git\n")
			os.Exit(0)
		}
	case "exit":
		code, _ := strconv.Atoi(os.Getenv("GITCLI_HELPER_EXIT"))
		os.Exit(code)
	default:
		os.Exit(0)
	}
}

// serveFileOrExit writes a canned payload file to stdout and exits the fake git
// process; a read failure exits non-zero so the caller sees a spawn error rather
// than silent empty output.
func serveFileOrExit(path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		os.Exit(4)
	}
	os.Stdout.Write(b)
	os.Exit(0)
}

// argsContain reports whether the argument vector names sub as one of its
// tokens (used to route ls-tree vs cat-file to distinct canned payloads).
func argsContain(args []string, sub string) bool {
	for _, a := range args {
		if a == sub {
			return true
		}
	}
	return false
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

// TestSanitizeDropsInboundControlCopies proves the removeGitEnv dedup case earns
// its place: an inbound copy of every re-appended control is dropped, so
// sanitizeEnvironment emits EXACTLY ONE entry for each control name carrying the
// pinned value. This asserts on sanitizeEnvironment's own output rather than a
// child-process env dump on purpose — os/exec dedups cmd.Env keeping the LAST
// occurrence, and the appended controls are always last, so a child dump would
// mask a surviving inbound duplicate and stay green even with the dedup case
// deleted. Deleting the "LC_ALL"/"GIT_TERMINAL_PROMPT"/… case leaves the inbound
// tr_TR/1 copies in the output, so a count > 1 reddens here.
func TestSanitizeDropsInboundControlCopies(t *testing.T) {
	pinned := map[string]string{
		"LC_ALL":                "C",
		"LANG":                  "C",
		"GIT_TERMINAL_PROMPT":   "0",
		"GIT_OPTIONAL_LOCKS":    "0",
		"GIT_LITERAL_PATHSPECS": "1",
	}
	// Plant a CONFLICTING inbound copy of each control so a missing dedup would
	// leave two entries (the inbound value plus the appended pinned value).
	base := []string{
		"LC_ALL=tr_TR",
		"LANG=tr_TR",
		"GIT_TERMINAL_PROMPT=1",
		"GIT_OPTIONAL_LOCKS=1",
		"GIT_LITERAL_PATHSPECS=0",
		"HOME=/home/keep", // a benign survivor, to prove scrubbing is scoped
	}
	out := sanitizeEnvironment(base)

	for name, wantVal := range pinned {
		count := 0
		var sawVal string
		for _, kv := range out {
			if i := strings.IndexByte(kv, '='); i >= 0 && kv[:i] == name {
				count++
				sawVal = kv[i+1:]
			}
		}
		if count != 1 {
			t.Errorf("%s appears %d times in sanitized env, want exactly 1 (inbound copy not dropped)", name, count)
		}
		if sawVal != wantVal {
			t.Errorf("%s = %q, want the pinned %q", name, sawVal, wantVal)
		}
	}
	if !hasExactEntry(out, "HOME=/home/keep") {
		t.Error("benign HOME was scrubbed")
	}
}

// TestRedactRemoteLocationsStripsEveryRemoteSpelling proves the redaction keys
// on URL shape rather than a scheme list: an https URL carrying credentials in
// its userinfo, a bare https URL, an unlisted scheme, and git's scp-like
// user@host:path spelling all collapse to the marker, while prose that merely
// contains a colon or a slash survives untouched.
func TestRedactRemoteLocationsStripsEveryRemoteSpelling(t *testing.T) {
	const secret = "s3cr3t-token-should-never-leak"
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			"https with credentials in userinfo",
			"fatal: unable to access 'https://user:" + secret + "@example.invalid/r.git/': 403",
			"fatal: unable to access '" + redactedURL + "': 403",
		},
		{"bare https", "fetching https://example.invalid/r.git now", "fetching " + redactedURL + " now"},
		{"unlisted scheme", "via ftps://example.invalid/r.git", "via " + redactedURL},
		{"scp-like", "fatal: git@example.invalid:org/r.git denied", "fatal: " + redactedURL + " denied"},
		{"no remote location", "fatal: couldn't find remote ref refs/heads/x", "fatal: couldn't find remote ref refs/heads/x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := redactRemoteLocations(tc.in)
			if got != tc.want {
				t.Fatalf("redacted = %q, want %q", got, tc.want)
			}
			if strings.Contains(got, secret) {
				t.Fatal("credential survived redaction")
			}
		})
	}
}

// TestStderrExcerptRedactsBeforeTruncating pins the ordering: a URL straddling
// the truncation boundary must be redacted first, because bounding first would
// sever the token and leave its scheme-and-credential prefix inside the window.
func TestStderrExcerptRedactsBeforeTruncating(t *testing.T) {
	const secret = "boundary-token-should-never-leak"
	url := "https://user:" + secret + "@example.invalid/r.git"
	// Place the URL so it starts just inside the limit and runs past it.
	// The trailing filler is space-separated: an unbroken run of non-space bytes
	// is part of the URL token and would be redacted along with it.
	prefix := strings.Repeat("x", stderrExcerptLimit-len(url)/2)
	ex := stderrExcerpt([]byte(prefix + url + " " + strings.Repeat("y", 4096)))
	if strings.Contains(ex, secret) {
		t.Fatal("credential disclosed inside the bounded excerpt")
	}
	if strings.Contains(ex, "https://") {
		t.Fatal("URL prefix survived truncation")
	}
	if !strings.HasSuffix(ex, " [truncated]") {
		t.Fatal("excerpt not marked truncated")
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
