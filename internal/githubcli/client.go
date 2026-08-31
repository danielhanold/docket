// Package githubcli is a typed adapter over controlled `gh` command execution:
// GitHub repository-identity discovery and pull-request response decoding. It
// starts only the `gh` executable, never a shell, and offers no exported generic
// command runner. It imports no other docket package — head commits, branches,
// titles, and bodies arrive as plain validated strings.
package githubcli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultLocalTimeout   = 30 * time.Second
	defaultNetworkTimeout = 5 * time.Minute
	// newClientOp labels construction-time Failures; it is not a callable
	// adapter operation.
	newClientOp = "new-client"
)

// waitDelay bounds how long cmd.Wait keeps draining inherited output pipes once
// the context has fired and the process has been killed. Without it a grandchild
// that inherited stdout/stderr — a credential helper — holds the pipe open and
// cmd.Run blocks past the deadline indefinitely.
const waitDelay = 2 * time.Second

// Client owns the resolved gh executable, a sanitized base environment, and the
// local/network timeout policy. All gh execution flows through the
// package-private run method; there is no exported command runner.
type Client struct {
	executable     string
	env            []string
	localTimeout   time.Duration
	networkTimeout time.Duration
	// networkReadTimeout and networkWriteTimeout split the network budget by
	// direction: a network read (list/view probes and verification reprobes) is
	// bounded by the read budget, a network write (pr create/edit/merge/comment)
	// by the write budget. Both default to networkTimeout, so a client that never
	// sets them is behaviorally identical to one carrying a single shared network
	// budget.
	networkReadTimeout  time.Duration
	networkWriteTimeout time.Duration
}

// clientConfig accumulates option state before NewClient validates and freezes
// it into a Client.
type clientConfig struct {
	executable     string
	localTimeout   time.Duration
	networkTimeout time.Duration
	// networkReadTimeout/networkWriteTimeout are only honored when their
	// respective *Set flag is true; the flag distinguishes an explicit budget
	// (validated > 0) from an absent one (inherit networkTimeout). A zero value
	// with the flag set is an explicit non-positive budget and is rejected.
	networkReadTimeout  time.Duration
	networkWriteTimeout time.Duration
	networkReadSet      bool
	networkWriteSet     bool
	baseEnv             []string
}

// Option configures a Client at construction.
type Option func(*clientConfig)

// WithExecutable pins the gh executable path (tests: explicit fake gh). An empty
// path falls back to resolving "gh" on PATH.
func WithExecutable(path string) Option {
	return func(cfg *clientConfig) { cfg.executable = path }
}

// WithLocalTimeout sets the default deadline for local (non-network) gh
// operations. It must be > 0, else NewClient returns an invalid-input Failure.
func WithLocalTimeout(d time.Duration) Option {
	return func(cfg *clientConfig) { cfg.localTimeout = d }
}

// WithNetworkTimeout sets the default deadline for network gh operations. It
// must be > 0, else NewClient returns an invalid-input Failure.
func WithNetworkTimeout(d time.Duration) Option {
	return func(cfg *clientConfig) { cfg.networkTimeout = d }
}

// WithNetworkReadTimeout sets the deadline for network read operations (the
// list/view probes and post-mutation verification reprobes). It must be > 0,
// else NewClient returns an invalid-input Failure. Absent, the read budget
// inherits networkTimeout.
func WithNetworkReadTimeout(d time.Duration) Option {
	return func(cfg *clientConfig) {
		cfg.networkReadTimeout = d
		cfg.networkReadSet = true
	}
}

// WithNetworkWriteTimeout sets the deadline for network write operations (pr
// create/edit/merge/comment mutations). It must be > 0, else NewClient returns
// an invalid-input Failure. Absent, the write budget inherits networkTimeout.
func WithNetworkWriteTimeout(d time.Duration) Option {
	return func(cfg *clientConfig) {
		cfg.networkWriteTimeout = d
		cfg.networkWriteSet = true
	}
}

// WithBaseEnvironment pins the base environment (tests: fully pinned
// environment). It is sanitized before use; production defaults to os.Environ().
func WithBaseEnvironment(env []string) Option {
	return func(cfg *clientConfig) { cfg.baseEnv = env }
}

// NewClient resolves the gh executable once via exec.LookPath, records its
// absolute path, validates the timeout policy, and freezes a sanitized base
// environment. A missing/unresolvable executable yields external; a non-positive
// timeout yields invalid-input.
func NewClient(opts ...Option) (*Client, error) {
	cfg := clientConfig{
		localTimeout:   defaultLocalTimeout,
		networkTimeout: defaultNetworkTimeout,
		baseEnv:        os.Environ(),
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.localTimeout <= 0 {
		return nil, newFailure(newClientOp, StageValidate, KindInvalidInput, "local timeout must be positive", nil)
	}
	if cfg.networkTimeout <= 0 {
		return nil, newFailure(newClientOp, StageValidate, KindInvalidInput, "network timeout must be positive", nil)
	}
	// Resolve the directional budgets: an unset budget inherits networkTimeout;
	// an explicitly-set one is validated > 0, mirroring the base checks above.
	networkReadTimeout := cfg.networkTimeout
	if cfg.networkReadSet {
		if cfg.networkReadTimeout <= 0 {
			return nil, newFailure(newClientOp, StageValidate, KindInvalidInput, "network read timeout must be positive", nil)
		}
		networkReadTimeout = cfg.networkReadTimeout
	}
	networkWriteTimeout := cfg.networkTimeout
	if cfg.networkWriteSet {
		if cfg.networkWriteTimeout <= 0 {
			return nil, newFailure(newClientOp, StageValidate, KindInvalidInput, "network write timeout must be positive", nil)
		}
		networkWriteTimeout = cfg.networkWriteTimeout
	}
	name := cfg.executable
	if name == "" {
		name = "gh"
	}
	resolved, err := exec.LookPath(name)
	if err != nil {
		return nil, newFailure(newClientOp, StageLaunch, KindExternal, "gh executable not found", err)
	}
	abs, err := filepath.Abs(resolved)
	if err != nil {
		return nil, newFailure(newClientOp, StageLaunch, KindExternal, "gh executable path not resolvable", err)
	}
	return &Client{
		executable:          abs,
		env:                 sanitizeEnvironment(cfg.baseEnv),
		localTimeout:        cfg.localTimeout,
		networkTimeout:      cfg.networkTimeout,
		networkReadTimeout:  networkReadTimeout,
		networkWriteTimeout: networkWriteTimeout,
	}, nil
}

// NetworkReadTimeout reports the deadline applied to network read operations.
func (c *Client) NetworkReadTimeout() time.Duration { return c.networkReadTimeout }

// NetworkWriteTimeout reports the deadline applied to network write operations.
func (c *Client) NetworkWriteTimeout() time.Duration { return c.networkWriteTimeout }

// runRequest is the package-private execution seam every operation uses.
type runRequest struct {
	op      string   // operation label carried into any Failure
	dir     string   // working directory the gh process runs in
	args    []string // gh argument vector, no leading "gh"
	stdin   []byte   // nil = no stdin; authored Markdown reaches gh here, never argv
	network bool     // selects the network vs local default timeout
	// write selects the network write budget over the read budget; it is
	// meaningful only when network is true (a local operation ignores it). A
	// remote mutation — pr create/edit/merge/comment — sets it; every network read
	// (list/view probe, verification reprobe) leaves it false.
	write bool
}

// runResult carries the captured output of one gh invocation. A non-zero exit is
// returned as data, not a Failure; classification is the caller's job.
type runResult struct {
	stdout   []byte
	stderr   []byte // raw; excerpted only at diagnostic time
	exitCode int
}

// run executes one gh command under the client's sanitized environment and
// timeout policy. It never invokes a shell: exec.CommandContext runs the
// absolute executable with the argument vector as-is. On context
// cancellation/timeout the process is killed and reaped (cmd.Run completes Wait)
// before returning, and the kind is chosen from ctx.Err(). A start failure is
// external; a non-zero process exit is returned in runResult.
func (c *Client) run(ctx context.Context, req runRequest) (runResult, *Failure) {
	timeout := c.localTimeout
	if req.network {
		// A network write (req.write) is bounded by the write budget; every other
		// network operation is a read bounded by the read budget. Both default to
		// networkTimeout, so an unsplit client is unchanged.
		if req.write {
			timeout = c.networkWriteTimeout
		} else {
			timeout = c.networkReadTimeout
		}
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.executable, req.args...)
	cmd.Dir = req.dir
	cmd.Env = c.env
	cmd.WaitDelay = waitDelay
	if req.stdin != nil {
		cmd.Stdin = bytes.NewReader(req.stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			if errors.Is(ctxErr, context.DeadlineExceeded) {
				return runResult{}, newFailure(req.op, StageInvoke, KindTimedOut, "gh execution exceeded its deadline", ctxErr)
			}
			return runResult{}, newFailure(req.op, StageInvoke, KindCancelled, "gh execution was cancelled", ctxErr)
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return runResult{stdout: stdout.Bytes(), stderr: stderr.Bytes(), exitCode: exitErr.ExitCode()}, nil
		}
		return runResult{}, newFailure(req.op, StageLaunch, KindExternal, "failed to start gh", err)
	}
	return runResult{stdout: stdout.Bytes(), stderr: stderr.Bytes(), exitCode: 0}, nil
}

// ghEnvRetargetNames are the environment variables that would let a caller's
// ambient configuration retarget which repository or host a gh call operates on.
// They are stripped from the base so every effect is governed solely by the
// explicit --repo argument and the requested working directory; a caller's
// GH_REPO or GH_HOST cannot redirect it. Normal authentication channels
// (GH_TOKEN, GITHUB_TOKEN, GH_ENTERPRISE_TOKEN, the gh config dir) are left
// untouched so real GitHub auth still works.
var ghEnvRetargetNames = map[string]struct{}{
	"GH_REPO": {},
	"GH_HOST": {},
}

// sanitizeEnvironment removes the repository/host retargeting variables from the
// base environment and leaves every other variable — HOME, XDG, PATH, the auth
// tokens, proxies, cert roots — untouched.
func sanitizeEnvironment(base []string) []string {
	out := make([]string, 0, len(base))
	for _, kv := range base {
		name := kv
		if i := strings.IndexByte(kv, '='); i >= 0 {
			name = kv[:i]
		}
		if _, drop := ghEnvRetargetNames[name]; drop {
			continue
		}
		out = append(out, kv)
	}
	return out
}
