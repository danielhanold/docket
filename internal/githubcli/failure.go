package githubcli

import (
	"errors"
	"fmt"
	"regexp"
)

// Stage names the phase a Failure arose in, drawn from the closed set below.
// It mirrors workspace.Failure's Op/Stage/Kind shape without importing it —
// githubcli imports no other docket package.
//
//	"validate" — input/identity validation before any process ran
//	"launch"   — the gh executable could not be started or resolved
//	"invoke"   — gh ran but exited non-zero, timed out, or was cancelled
//	"decode"   — gh's response could not be decoded into a typed value
type Stage string

const (
	StageValidate Stage = "validate"
	StageLaunch   Stage = "launch"
	StageInvoke   Stage = "invoke"
	StageDecode   Stage = "decode"
)

// Kind is the stable, redaction-safe category of a Failure. The six-kind set
// matches workspace.Failure exactly (invalid-input, invalid-state, external,
// cancelled, timed-out, invalid-output).
type Kind string

const (
	KindInvalidInput  Kind = "invalid-input"
	KindInvalidState  Kind = "invalid-state"
	KindExternal      Kind = "external"
	KindCancelled     Kind = "cancelled"
	KindTimedOut      Kind = "timed-out"
	KindInvalidOutput Kind = "invalid-output"
)

// Failure is the adapter's typed error. Detail carries bounded, redacted prose
// only — never token values, Authorization headers, credentialed URLs, PR body
// bytes, environment values, or unbounded stderr.
type Failure struct {
	Op     string // the adapter operation, e.g. "discover-repository"
	Stage  Stage
	Kind   Kind
	Detail string // bounded, redacted
	Err    error  // wrapped cause, may be nil
}

// Error renders the operation, stage, kind, and bounded detail.
func (f *Failure) Error() string {
	if f.Detail == "" {
		return fmt.Sprintf("githubcli %s/%s: %s", f.Op, f.Stage, f.Kind)
	}
	return fmt.Sprintf("githubcli %s/%s: %s: %s", f.Op, f.Stage, f.Kind, f.Detail)
}

// Unwrap exposes the wrapped cause for errors.Is/As.
func (f *Failure) Unwrap() error { return f.Err }

// newFailure constructs a *Failure with a redacted Detail. Redaction happens
// here, at the single construction boundary, so no call site can accidentally
// embed a secret — a per-site scrub is only as complete as the list of sites
// someone remembered to change.
func newFailure(op string, stage Stage, kind Kind, detail string, err error) *Failure {
	return &Failure{Op: op, Stage: stage, Kind: kind, Detail: redactSecrets(detail), Err: err}
}

// AsFailure is an errors.As convenience recovering a *Failure from an error.
func AsFailure(err error) (*Failure, bool) {
	var f *Failure
	if errors.As(err, &f) {
		return f, true
	}
	return nil, false
}

// stderrExcerptLimit bounds how many redacted stderr bytes may appear in a
// diagnostic. It is the 0308 excerpt-length policy: the first 512 bytes, with an
// explicit truncation marker for anything longer.
const stderrExcerptLimit = 512

// redacted markers replace each secret class removed from a diagnostic.
const (
	redactedToken = "[redacted-token]"
	redactedAuth  = "[redacted-authorization]"
	redactedURL   = "[redacted-url]"
)

// ghTokenPattern matches the GitHub token families by shape, not by an
// enumerated spelling: the ghX_ personal/oauth/user/server/refresh prefixes and
// the github_pat_ fine-grained prefix, each followed by its token body. Keying
// on shape means the family member added tomorrow is redacted too.
var ghTokenPattern = regexp.MustCompile(`\b(gh[pousr]_[A-Za-z0-9]+|github_pat_[A-Za-z0-9_]+)\b`)

// authorizationHeaderPattern matches an Authorization header and its credential
// value up to end of line, case-insensitively — the credential is the whole
// point of the header, so the value is stripped, not just the scheme.
var authorizationHeaderPattern = regexp.MustCompile(`(?i)authorization:\s*\S.*`)

// transportURLPattern matches an absolute transport URL — any scheme, with or
// without userinfo — up to the first whitespace or quote. gh's stderr quotes the
// URL it failed on, so the terminator set stops at the closing quote.
var transportURLPattern = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.\-]*://[^\s'"]*`)

// scpLikeRemotePattern matches git's scp-like user@host:path remote spelling. It
// runs after transportURLPattern, whose marker carries no '@' and cannot
// re-match.
var scpLikeRemotePattern = regexp.MustCompile(`[A-Za-z0-9._\-]+@[A-Za-z0-9._\-]+:[^\s'"]*`)

// redactSecrets removes token values, Authorization headers, and credentialed or
// bare transport URLs from text bound for a diagnostic. It keys on shape rather
// than an enumerated host or scheme list, because the transport or token that
// leaks is the one an enumeration missed. Authorization headers are stripped
// before URLs so a header carrying a URL-shaped token still loses its whole
// value.
func redactSecrets(s string) string {
	s = authorizationHeaderPattern.ReplaceAllString(s, redactedAuth)
	s = transportURLPattern.ReplaceAllString(s, redactedURL)
	s = scpLikeRemotePattern.ReplaceAllString(s, redactedURL)
	s = ghTokenPattern.ReplaceAllString(s, redactedToken)
	return s
}

// stderrExcerpt returns a redacted, bounded, explicitly-truncated view of
// captured stderr: secrets removed, then at most stderrExcerptLimit bytes, with
// " [truncated]" appended when the redacted text was longer. Redaction precedes
// truncation deliberately: bounding first could sever a token mid-value and
// leave the surviving prefix inside the window.
func stderrExcerpt(stderr []byte) string {
	safe := redactSecrets(string(stderr))
	if len(safe) <= stderrExcerptLimit {
		return safe
	}
	return safe[:stderrExcerptLimit] + " [truncated]"
}
