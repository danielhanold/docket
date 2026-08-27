package githubcli

import (
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

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
