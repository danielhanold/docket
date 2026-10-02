// Package process owns Docket's native per-run gate supervision: run
// identities, private durable run state, the re-exec'd supervisor, exact
// wait-status terminal records, ownership-gated signalling, and
// abandoned-run recovery. It is repository-independent and imports only
// the Go standard library (guarded by TestImportBoundaryStdlibOnly).
package process

import (
	"path/filepath"
	"time"
)

// Service performs the gate operations. The executable path is an explicit
// dependency: production passes the current docket binary; tests pass the
// test binary, whose TestMain routes supervisor re-execution back into
// this package.
type Service struct {
	executable string

	// Bounds are production constants (spec: 10s establishment, 10s TERM
	// wait, 5s KILL verification). Package-private so tests can shrink
	// them; nothing outside the package can.
	establishTimeout time.Duration
	stopTermWait     time.Duration
	stopKillWait     time.Duration
	pollInterval     time.Duration
}

// NewService builds a Service around the absolute path of the binary to
// re-execute as the supervisor.
func NewService(executable string) (*Service, error) {
	if !filepath.IsAbs(executable) {
		return nil, failf(FailInvalidInput, "new-service", "executable path %q is not absolute", executable)
	}
	return &Service{
		executable:       executable,
		establishTimeout: 10 * time.Second,
		stopTermWait:     10 * time.Second,
		stopKillWait:     5 * time.Second,
		pollInterval:     25 * time.Millisecond,
	}, nil
}

// State is the run-state vocabulary fixed by the spec.
type State string

const (
	StateRunning  State = "running"
	StatePassed   State = "passed"
	StateFailed   State = "failed"
	StateSignaled State = "signaled"
	StateStopped  State = "stopped"
	StateVanished State = "vanished"
)

// SupervisorExited reports whether st proves the run's supervisor has exited:
// passed, failed, signaled, stopped, and vanished all mean no live supervisor
// holds the run (and with it any worktree lock it was handed). Running does not,
// and neither does an empty or unrecognized state — an unknown state proves
// nothing. It is the single teardown predicate the gate driver and the run
// tracker share; callers never re-derive it as "not running".
func (st State) SupervisorExited() bool {
	switch st {
	case StatePassed, StateFailed, StateSignaled, StateStopped, StateVanished:
		return true
	default:
		return false
	}
}

// Terminal is the exact decoded child wait status. Kind is "exit" or
// "signal"; exactly one of ExitCode/Signal is meaningful per Kind.
type Terminal struct {
	Kind     string
	ExitCode int
	Signal   int
}
