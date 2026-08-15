package gitcli

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
)

// stderrExcerptLimit bounds how many raw stderr bytes may appear in a
// diagnostic; anything longer is truncated with an explicit marker.
const stderrExcerptLimit = 1024

// runRequest is the package-private execution seam every operation uses.
type runRequest struct {
	op      Operation // operation label carried into any Failure
	dir     string    // working directory; callers set it after discovery
	args    []string  // git argument vector, no leading "git"
	stdin   []byte    // nil = no stdin
	network bool      // selects the network vs local default timeout
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
	timeout := c.localTimeout
	if req.network {
		timeout = c.networkTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.executable, req.args...)
	cmd.Dir = req.dir
	cmd.Env = c.env
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

// removeGitEnv reports whether an environment variable NAME must never reach the
// child git process. Families are matched by prefix; the discrete
// redirection/discovery variables that share no common prefix are matched by
// exact name; the fixed controls are dropped here so sanitizeEnvironment can
// re-append exactly one authoritative copy of each.
func removeGitEnv(name string) bool {
	for _, prefix := range gitEnvRemovePrefixes {
		if strings.HasPrefix(name, prefix) {
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
		"GIT_CEILING_DIRECTORIES",
		"GIT_DISCOVERY_ACROSS_FILESYSTEM":
		return true
	case "LC_ALL", "LANG", "GIT_TERMINAL_PROMPT", "GIT_OPTIONAL_LOCKS":
		// Fixed controls: drop any inbound copy so the appended value wins
		// unambiguously.
		return true
	}
	return false
}

// sanitizeEnvironment removes, by semantic class, the environment variables that
// could redirect the repository, inject config, or enable tracing, then appends
// the fixed controls that pin locale and forbid interactive prompting. Every
// other variable — HOME, XDG, SSH_AUTH_SOCK, GIT_SSH_COMMAND, proxies, cert
// roots — survives untouched.
func sanitizeEnvironment(base []string) []string {
	out := make([]string, 0, len(base)+4)
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
	)
	return out
}

// stderrExcerpt returns a bounded, explicitly-truncated view of captured
// stderr: at most stderrExcerptLimit bytes, with " [truncated]" appended when
// the original was longer. It is the only stderr-derived content permitted in a
// diagnostic Detail.
func stderrExcerpt(stderr []byte) string {
	if len(stderr) <= stderrExcerptLimit {
		return string(stderr)
	}
	return string(stderr[:stderrExcerptLimit]) + " [truncated]"
}
