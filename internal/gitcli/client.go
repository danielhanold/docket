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
}

// clientConfig accumulates option state before NewClient validates and freezes
// it into a Client.
type clientConfig struct {
	executable     string
	localTimeout   time.Duration
	networkTimeout time.Duration
	baseEnv        []string
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
		executable:     abs,
		env:            sanitizeEnvironment(cfg.baseEnv),
		localTimeout:   cfg.localTimeout,
		networkTimeout: cfg.networkTimeout,
	}, nil
}
