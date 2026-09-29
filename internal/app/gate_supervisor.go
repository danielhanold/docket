package app

import "github.com/danielhanold/docket/internal/process"

// MaybeRunGateSupervisor runs the package-private gate supervisor when this
// process was re-executed as one. cli.Run calls it before Cobra parses
// anything: the supervisor is not a public command and must never touch the
// protocol streams. ok is false unless the private supervisor env var is set.
func MaybeRunGateSupervisor() (int, bool) {
	if !process.SupervisorRequested() {
		return 0, false
	}
	return process.RunSupervisorFromEnv(), true
}
