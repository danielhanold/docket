package gitcli

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// stderrExcerptLimit bounds how many raw stderr bytes may appear in a
// diagnostic; anything longer is truncated with an explicit marker.
const stderrExcerptLimit = 1024

// waitDelay bounds how long cmd.Wait keeps draining inherited output pipes once
// the context has fired and the process itself has been killed. Without it a
// grandchild that inherited stdout/stderr — a credential helper, an ssh
// multiplexer — holds the pipe open and cmd.Run blocks past the deadline
// indefinitely, so the timeout/cancel guarantee would hold for the child only.
const waitDelay = 2 * time.Second

// runRequest is the package-private execution seam every operation uses.
type runRequest struct {
	op      Operation // operation label carried into any Failure
	dir     string    // working directory; callers set it after discovery
	args    []string  // git argument vector, no leading "git"
	stdin   []byte    // nil = no stdin
	network bool      // selects the network vs local default timeout
	// env is appended to the client's sanitized base environment for this one
	// command; a duplicate name overrides the base value (last wins). It is the
	// only per-command environment channel — used for the engine-clock commit
	// dates (GIT_AUTHOR_DATE / GIT_COMMITTER_DATE) — and never carries repository
	// redirection, config injection, or credentials.
	env []string
}

// runResult carries the captured output of one git invocation. A non-zero exit
// is returned as data, not a Failure; classification is the caller's job.
type runResult struct {
	stdout   []byte
	stderr   []byte // raw; excerpted only at diagnostic time
	exitCode int
}

// run executes one git command under the client's sanitized environment and
// timeout policy. It never invokes a shell: exec.CommandContext runs the
// absolute executable with the argument vector as-is. On context
// cancellation/timeout the process is killed and reaped (cmd.Run completes Wait)
// before returning, and the kind is chosen from ctx.Err(). A start failure is
// executable-unavailable; a non-zero process exit is returned in runResult.
func (c *Client) run(ctx context.Context, req runRequest) (runResult, *Failure) {
	// A caller that has already opened a budget covering several processes keeps
	// it: this per-process timeout can only ever shorten the deadline in force,
	// never extend past the caller's.
	timeout := c.localTimeout
	if req.network {
		timeout = c.networkTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.executable, req.args...)
	cmd.Dir = req.dir
	// The sanitized base wins by default; a per-command env entry is appended so a
	// duplicate name (a commit date) overrides it, leaving the base intact for
	// every other variable.
	if len(req.env) > 0 {
		cmd.Env = append(append([]string(nil), c.env...), req.env...)
	} else {
		cmd.Env = c.env
	}
	// Bound the post-kill pipe drain so a pipe-holding grandchild cannot outlive
	// the deadline that already killed its parent.
	cmd.WaitDelay = waitDelay
	if req.stdin != nil {
		cmd.Stdin = bytes.NewReader(req.stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// A killed process reports an exit error too, so consult ctx.Err()
		// first: a fired deadline or a cancelled caller context wins over the
		// generic exit classification.
		if ctxErr := ctx.Err(); ctxErr != nil {
			if errors.Is(ctxErr, context.DeadlineExceeded) {
				return runResult{}, newFailure(req.op, KindTimedOut, "git execution exceeded its deadline", ctxErr)
			}
			return runResult{}, newFailure(req.op, KindCancelled, "git execution was cancelled", ctxErr)
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return runResult{stdout: stdout.Bytes(), stderr: stderr.Bytes(), exitCode: exitErr.ExitCode()}, nil
		}
		return runResult{}, newFailure(req.op, KindExecutableUnavailable, "failed to start git", err)
	}
	return runResult{stdout: stdout.Bytes(), stderr: stderr.Bytes(), exitCode: 0}, nil
}

// gitEnvRemovePrefixes are the environment-variable families removed wholesale:
// any GIT_TRACE* (tracing), GIT_CONFIG* (config injection), or GIT_ALTERNATE*
// (alternate object stores) name. Matching by prefix keys on git's own naming
// shape rather than an enumerated spelling list.
var gitEnvRemovePrefixes = []string{"GIT_TRACE", "GIT_CONFIG", "GIT_ALTERNATE"}

// gitEnvRemoveSuffixes are the environment-variable families removed by name
// shape rather than a spelling list: any GIT_*_PATHSPECS name — the global
// pathspec-magic controls GIT_LITERAL_PATHSPECS, GIT_GLOB_PATHSPECS,
// GIT_NOGLOB_PATHSPECS, GIT_ICASE_PATHSPECS. sanitizeEnvironment re-appends the
// single authoritative GIT_LITERAL_PATHSPECS=1; the rest must be scrubbed, not
// merely overridden, because git rejects a global literal setting combined with
// any other global pathspec setting ("fatal: global 'literal' pathspec setting
// is incompatible with all other global pathspec settings"), so an inherited
// GIT_ICASE_PATHSPECS left in place would make every pathspec-bearing ls-tree
// call fail rather than run literally.
var gitEnvRemoveSuffixes = []string{"_PATHSPECS"}

// removeGitEnv reports whether an environment variable NAME must never reach the
// child git process. Families are matched by prefix or by suffix shape; the
// discrete redirection/discovery variables that share no common affix are
// matched by exact name; the fixed controls are dropped here so
// sanitizeEnvironment can re-append exactly one authoritative copy of each.
func removeGitEnv(name string) bool {
	for _, prefix := range gitEnvRemovePrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	for _, suffix := range gitEnvRemoveSuffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	switch name {
	case "GIT_DIR",
		"GIT_COMMON_DIR",
		"GIT_WORK_TREE",
		"GIT_INDEX_FILE",
		"GIT_OBJECT_DIRECTORY",
		"GIT_NAMESPACE",
		"GIT_EXEC_PATH",
		"GIT_CEILING_DIRECTORIES",
		"GIT_DISCOVERY_ACROSS_FILESYSTEM":
		return true
	case "LC_ALL", "LANG", "GIT_TERMINAL_PROMPT", "GIT_OPTIONAL_LOCKS", "GIT_LITERAL_PATHSPECS":
		// Fixed controls: drop any inbound copy so the appended value wins
		// unambiguously. GIT_LITERAL_PATHSPECS is already covered by the
		// _PATHSPECS suffix scrub above; naming it here documents that it is a
		// re-appended control, not merely a neutralized family member.
		return true
	}
	return false
}

// sanitizeEnvironment removes, by semantic class, the environment variables that
// could redirect the repository, inject config, enable tracing, or change
// pathspec interpretation, then appends the fixed controls that pin locale,
// forbid interactive prompting, and force literal pathspecs. GIT_LITERAL_PATHSPECS=1
// makes every "-- <path>" vector a literal path by construction, so a repo path
// with leading pathspec-magic punctuation (a ':' prefix, ':(top)…', etc.) can
// neither vanish nor escape its requested scope; the _PATHSPECS suffix scrub in
// removeGitEnv first clears any inherited icase/glob/noglob setting this literal
// control would otherwise fatally conflict with. Every other variable — HOME,
// XDG, SSH_AUTH_SOCK, GIT_SSH_COMMAND, proxies, cert roots — survives untouched.
func sanitizeEnvironment(base []string) []string {
	out := make([]string, 0, len(base)+5)
	for _, kv := range base {
		name := kv
		if i := strings.IndexByte(kv, '='); i >= 0 {
			name = kv[:i]
		}
		if removeGitEnv(name) {
			continue
		}
		out = append(out, kv)
	}
	out = append(out,
		"LC_ALL=C",
		"LANG=C",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_OPTIONAL_LOCKS=0",
		"GIT_LITERAL_PATHSPECS=1",
	)
	return out
}

// redactedURL replaces every remote-location token removed from a diagnostic.
const redactedURL = "[redacted-url]"

// transportURLPattern matches an absolute transport URL — any scheme, with or
// without userinfo — up to the first whitespace or quote. Git's own stderr
// quotes the URL it failed on ("unable to access 'https://user:token@host/r'"),
// so the terminator set stops at the closing quote rather than swallowing the
// rest of the message.
var transportURLPattern = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.\-]*://[^\s'"]*`)

// scpLikeRemotePattern matches git's alternate remote spelling, the scp-like
// user@host:path form (and the bare user@host prefix of it). It runs after
// transportURLPattern, whose replacement carries no '@' and so cannot re-match.
var scpLikeRemotePattern = regexp.MustCompile(`[A-Za-z0-9._\-]+@[A-Za-z0-9._\-]+(:[^\s'"]*)?`)

// redactRemoteLocations removes remote URLs — and the credentials a URL's
// userinfo can carry — from text bound for a diagnostic. It keys on URL shape,
// never on an enumerated scheme or host list, because the transport that leaks
// is the one the enumeration missed. The spec's Detail contract permits the
// remote *name* and forbids the remote URL; this is the boundary that enforces
// it for every stderr-derived diagnostic in the package.
func redactRemoteLocations(s string) string {
	s = transportURLPattern.ReplaceAllString(s, redactedURL)
	return scpLikeRemotePattern.ReplaceAllString(s, redactedURL)
}

// stderrExcerpt returns a redacted, bounded, explicitly-truncated view of
// captured stderr: remote locations removed, then at most stderrExcerptLimit
// bytes, with " [truncated]" appended when the redacted text was longer. It is
// the only stderr-derived content permitted in a diagnostic Detail, so
// redaction happens here rather than at each call site — a per-site scrub is
// only as complete as the list of sites someone remembered to change.
//
// Redaction precedes truncation deliberately: bounding first could sever a URL
// mid-token and leave the surviving prefix — scheme, userinfo, and often the
// credential — inside the window.
func stderrExcerpt(stderr []byte) string {
	safe := redactRemoteLocations(string(stderr))
	if len(safe) <= stderrExcerptLimit {
		return safe
	}
	return safe[:stderrExcerptLimit] + " [truncated]"
}
