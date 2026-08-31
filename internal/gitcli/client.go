package gitcli

import (
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	defaultLocalTimeout   = 30 * time.Second
	defaultNetworkTimeout = 5 * time.Minute
	// newClientOp labels construction-time Failures; it is not a callable
	// adapter operation.
	newClientOp Operation = "new-client"
)

// Client owns the resolved git executable, a sanitized base environment, and
// the local/network timeout policy. All git execution flows through the
// package-private run method; there is no exported command runner.
type Client struct {
	executable     string
	env            []string
	localTimeout   time.Duration
	networkTimeout time.Duration
	// networkReadTimeout and networkWriteTimeout split the network budget by
	// direction: a network read (fetch, ls-remote, discovery probes) is bounded
	// by the read budget, a network write (push, lease delete) by the write
	// budget. Both default to networkTimeout, so a client that never sets them is
	// behaviorally identical to one carrying a single shared network budget.
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

// WithExecutable pins the git executable path (tests: explicit fake/real git).
// An empty path falls back to resolving "git" on PATH.
func WithExecutable(path string) Option {
	return func(cfg *clientConfig) { cfg.executable = path }
}

// WithLocalTimeout sets the default deadline for local (non-network) git
// operations. It must be > 0, else NewClient returns an invalid-request Failure.
func WithLocalTimeout(d time.Duration) Option {
	return func(cfg *clientConfig) { cfg.localTimeout = d }
}

// WithNetworkTimeout sets the default deadline for network git operations. It
// must be > 0, else NewClient returns an invalid-request Failure.
func WithNetworkTimeout(d time.Duration) Option {
	return func(cfg *clientConfig) { cfg.networkTimeout = d }
}

// WithNetworkReadTimeout sets the deadline for network read operations (fetch,
// ls-remote, discovery probes). It must be > 0, else NewClient returns an
// invalid-request Failure. Absent, the read budget inherits networkTimeout.
func WithNetworkReadTimeout(d time.Duration) Option {
	return func(cfg *clientConfig) {
		cfg.networkReadTimeout = d
		cfg.networkReadSet = true
	}
}

// WithNetworkWriteTimeout sets the deadline for network write operations (push,
// lease delete). It must be > 0, else NewClient returns an invalid-request
// Failure. Absent, the write budget inherits networkTimeout.
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

// NewClient resolves the git executable once via exec.LookPath, records its
// absolute path, validates the timeout policy, and freezes a sanitized base
// environment. A missing/unresolvable executable yields executable-unavailable;
// a non-positive timeout yields invalid-request.
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
		return nil, newFailure(newClientOp, KindInvalidRequest, "local timeout must be positive", nil)
	}
	if cfg.networkTimeout <= 0 {
		return nil, newFailure(newClientOp, KindInvalidRequest, "network timeout must be positive", nil)
	}
	// Resolve the directional budgets: an unset budget inherits networkTimeout;
	// an explicitly-set one is validated > 0, mirroring the base checks above.
	networkReadTimeout := cfg.networkTimeout
	if cfg.networkReadSet {
		if cfg.networkReadTimeout <= 0 {
			return nil, newFailure(newClientOp, KindInvalidRequest, "network read timeout must be positive", nil)
		}
		networkReadTimeout = cfg.networkReadTimeout
	}
	networkWriteTimeout := cfg.networkTimeout
	if cfg.networkWriteSet {
		if cfg.networkWriteTimeout <= 0 {
			return nil, newFailure(newClientOp, KindInvalidRequest, "network write timeout must be positive", nil)
		}
		networkWriteTimeout = cfg.networkWriteTimeout
	}
	name := cfg.executable
	if name == "" {
		name = "git"
	}
	resolved, err := exec.LookPath(name)
	if err != nil {
		return nil, newFailure(newClientOp, KindExecutableUnavailable, "git executable not found", err)
	}
	abs, err := filepath.Abs(resolved)
	if err != nil {
		return nil, newFailure(newClientOp, KindExecutableUnavailable, "git executable path not resolvable", err)
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
