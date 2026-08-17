package process

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

const recordSchema = 1

const (
	manifestFile      = "manifest.json"
	terminalFile      = "terminal.json"
	stopIntentFile    = "stop-intent.json"
	stoppedFile       = "stopped.json"
	abandonedFile     = "abandoned.json"
	failureFile       = "failure.json"
	liveLockFile      = "live.lock"
	registryLockFile  = "registry.lock"
	stdoutLogFile     = "stdout.log"
	stderrLogFile     = "stderr.log"
	supervisorLogFile = "supervisor.log"
)

// manifestRecord is the run's identity and phase ledger. Phases:
// "allocated", "established", "running", "terminal".
type manifestRecord struct {
	Schema        int    `json:"schema"`
	RunID         string `json:"run_id"`
	Token         string `json:"token"`
	Root          string `json:"root"`
	RunDir        string `json:"run_dir"`
	SupervisorPID int    `json:"supervisor_pid"`
	PGID          int    `json:"pgid"`
	SID           int    `json:"sid"`
	Phase         string `json:"phase"`
	Cwd           string `json:"cwd"`
	Argv0         string `json:"argv0"`
	Argc          int    `json:"argc"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// terminalRecord is the exact decoded child wait status. Kind is "exit" or
// "signal"; the sole writer is the supervisor.
type terminalRecord struct {
	Schema     int    `json:"schema"`
	RunID      string `json:"run_id"`
	Kind       string `json:"kind"`
	ExitCode   int    `json:"exit_code"`
	Signal     int    `json:"signal"`
	RecordedAt string `json:"recorded_at"`
}

// stopIntentRecord records that a stop was requested for the run.
type stopIntentRecord struct {
	Schema     int    `json:"schema"`
	RunID      string `json:"run_id"`
	Reason     string `json:"reason"`
	RecordedAt string `json:"recorded_at"`
}

// stoppedRecord records that a stop was verified complete.
type stoppedRecord struct {
	Schema     int    `json:"schema"`
	RunID      string `json:"run_id"`
	VerifiedAt string `json:"verified_at"`
}

// abandonedRecord records that a run was found abandoned during recovery.
type abandonedRecord struct {
	Schema     int    `json:"schema"`
	RunID      string `json:"run_id"`
	Cause      string `json:"cause"`
	RecordedAt string `json:"recorded_at"`
}

// failureRecord records a supervisor start-failure — distinct from a
// terminal child record.
type failureRecord struct {
	Schema     int    `json:"schema"`
	RunID      string `json:"run_id"`
	Stage      string `json:"stage"`
	Reason     string `json:"reason"`
	RecordedAt string `json:"recorded_at"`
}

// readRecord reads one schema-1 JSON record. Three outcomes, never
// collapsed: cleanly absent -> (false, nil); malformed or wrong schema ->
// FailInvalidState; any other filesystem error -> FailExternal.
func readRecord(runDir, name string, v any, schema func() int) (bool, error) {
	raw, err := os.ReadFile(filepath.Join(runDir, name))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, failf(FailExternal, "read-record", "reading %s: %v", name, err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return false, failf(FailInvalidState, "read-record", "%s is not a valid record: %v", name, err)
	}
	if schema() != recordSchema {
		return false, failf(FailInvalidState, "read-record", "%s has unrecognized schema %d", name, schema())
	}
	return true, nil
}

func readManifest(runDir string) (*manifestRecord, error) {
	var r manifestRecord
	ok, err := readRecord(runDir, manifestFile, &r, func() int { return r.Schema })
	if !ok {
		return nil, err
	}
	return &r, nil
}

func readTerminal(runDir string) (*terminalRecord, error) {
	var r terminalRecord
	ok, err := readRecord(runDir, terminalFile, &r, func() int { return r.Schema })
	if !ok {
		return nil, err
	}
	return &r, nil
}

func readStopIntent(runDir string) (*stopIntentRecord, error) {
	var r stopIntentRecord
	ok, err := readRecord(runDir, stopIntentFile, &r, func() int { return r.Schema })
	if !ok {
		return nil, err
	}
	return &r, nil
}

func readStopped(runDir string) (*stoppedRecord, error) {
	var r stoppedRecord
	ok, err := readRecord(runDir, stoppedFile, &r, func() int { return r.Schema })
	if !ok {
		return nil, err
	}
	return &r, nil
}

func readAbandoned(runDir string) (*abandonedRecord, error) {
	var r abandonedRecord
	ok, err := readRecord(runDir, abandonedFile, &r, func() int { return r.Schema })
	if !ok {
		return nil, err
	}
	return &r, nil
}

func readFailureRecord(runDir string) (*failureRecord, error) {
	var r failureRecord
	ok, err := readRecord(runDir, failureFile, &r, func() int { return r.Schema })
	if !ok {
		return nil, err
	}
	return &r, nil
}
