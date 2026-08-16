package githubcli

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
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

// TestEnvHygieneStripsRetargeting (f): GH_REPO/GH_HOST set in the parent are NOT
// visible to the fake, while an auth token (GH_TOKEN) survives so normal GitHub
// authentication channels remain available.
func TestEnvHygieneStripsRetargeting(t *testing.T) {
	c, log := newFakeClient(t,
		fakeScenario{Invocations: []fakeArm{prViewArm(samplePRJSON)}},
		withExtraEnv(
			"GH_REPO=evil/owner-repo",
			"GH_HOST=evil.example.invalid",
			"GH_TOKEN=gho_survivingtoken",
		),
	)
	_, f := c.run(context.Background(), runRequest{
		op:      "probe",
		dir:     t.TempDir(),
		args:    []string{"pr", "view", "7"},
		network: true,
	})
	if f != nil {
		t.Fatalf("run failed: %v", f)
	}
	recs := log.records(t)
	if len(recs) != 1 {
		t.Fatalf("witness records = %d, want 1", len(recs))
	}
	env := recs[0].Env
	if _, ok := env["GH_REPO"]; ok {
		t.Errorf("GH_REPO leaked to child: %q", env["GH_REPO"])
	}
	if _, ok := env["GH_HOST"]; ok {
		t.Errorf("GH_HOST leaked to child: %q", env["GH_HOST"])
	}
	if env["GH_TOKEN"] != "gho_survivingtoken" {
		t.Errorf("GH_TOKEN not preserved for auth: %q", env["GH_TOKEN"])
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
