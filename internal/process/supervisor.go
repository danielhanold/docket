package process

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Env/fd contract for the re-exec'd supervisor. Launch activates supervisor
// mode by setting supervisorRunDirEnv and passes the JSON-encoded child argv
// in supervisorArgvEnv. The inherited live.lock descriptor arrives at
// supervisorLockFD and the handshake pipe's write end at supervisorHandshakeFD
// (the first and second ExtraFiles slots).
const (
	supervisorRunDirEnv   = "DOCKET_GATE_SUPERVISOR_RUN_DIR"
	supervisorArgvEnv     = "DOCKET_GATE_SUPERVISOR_ARGV"
	supervisorLockFD      = 3
	supervisorHandshakeFD = 4
)

// SupervisorRequested reports whether this process was re-executed as a gate
// supervisor. It is the whole predicate the pre-Cobra hook keys on: true iff
// the private run-dir env var is set, which only Launch does.
func SupervisorRequested() bool {
	return os.Getenv(supervisorRunDirEnv) != ""
}

// envWithout returns env with every "KEY=..." entry whose key appears in keys
// removed. Launch's private env vars are stripped from the child environment
// so the supervised command never observes them.
func envWithout(env []string, keys ...string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		drop := false
		for _, k := range keys {
			if strings.HasPrefix(kv, k+"=") {
				drop = true
				break
			}
		}
		if !drop {
			out = append(out, kv)
		}
	}
	return out
}

func supervisorStamp() string { return time.Now().UTC().Format(time.RFC3339) }

// RunSupervisorFromEnv runs the entire supervisor lifetime and returns the
// process exit code (0 after a durable terminal record, 1 on supervisor
// failure). It never calls os.Exit — cmd/docket/main.go owns the single exit
// site; the supervisor path returns an int up through cli.Run like every
// other path.
func RunSupervisorFromEnv() int {
	runDir := os.Getenv(supervisorRunDirEnv)
	// The run-dir base is the 32-hex run id; it is the identity for any
	// failure record we might have to write before the manifest is read.
	runID := filepath.Base(runDir)

	// (1) Adopt the inherited lock and handshake descriptors and mark both
	// close-on-exec: ExtraFiles arrive WITHOUT CLOEXEC, and neither the live
	// lock nor the handshake pipe may leak into the supervised command — a
	// leaked lock fd would outlive the supervisor in the child.
	lockFile := os.NewFile(uintptr(supervisorLockFD), liveLockFile)
	pipe := os.NewFile(uintptr(supervisorHandshakeFD), "handshake")
	syscall.CloseOnExec(supervisorLockFD)
	syscall.CloseOnExec(supervisorHandshakeFD)

	closeLock := func() {
		if lockFile != nil {
			lockFile.Close()
		}
	}
	closePipe := func() {
		if pipe != nil {
			pipe.Close()
		}
	}
	// handshake writes one wake-up line; the launcher re-reads the atomic
	// records as truth. A broken pipe (launcher already gone) returns EPIPE
	// on a non-std fd, which Go surfaces as an error rather than a crash, so
	// ignoring it is safe.
	handshake := func(line string) {
		if pipe != nil {
			io.WriteString(pipe, line)
		}
	}

	// (2) Route all supervisor diagnostics to supervisor.log — never to
	// stdout/stderr (the launcher points those at supervisor.log too, but the
	// supervisor writes to neither directly).
	var diag *log.Logger
	if lf, err := openSupervisorFile(filepath.Join(runDir, supervisorLogFile), os.O_APPEND); err == nil {
		defer lf.Close()
		diag = log.New(lf, "supervisor: ", log.LstdFlags|log.LUTC)
	} else {
		diag = log.New(io.Discard, "", 0)
	}

	// writeFailure records a supervisor start-failure (distinct from a
	// terminal child record), wakes the launcher, and releases the lock LAST
	// so the durable record is visible before the lock frees.
	writeFailure := func(stage, reason string) int {
		diag.Printf("failing at %s: %s", stage, reason)
		_ = writeAtomicJSON(filepath.Join(runDir, failureFile), &failureRecord{
			Schema: recordSchema, RunID: runID, Stage: stage,
			Reason: reason, RecordedAt: supervisorStamp(),
		})
		handshake("failed\n")
		closePipe()
		closeLock()
		return 1
	}

	// (3) Prove pid == pgid == sid on ourselves: Launch started us with
	// Setsid, so a live session leader is the addressable-session invariant
	// the launcher waits to observe before publishing "established".
	pid := os.Getpid()
	pgid, pans := getPGID(pid)
	sid, sans := getSID(pid)
	if pans != probeLive || sans != probeLive || pgid != pid || sid != pid {
		return writeFailure("establish-session", "supervisor is not a live session leader")
	}

	// (4) Re-read the allocated manifest, stamp the proven identity, publish
	// the established phase, and wake the launcher.
	m, err := readManifest(runDir)
	if err != nil {
		return writeFailure("read-manifest", "run manifest is unreadable")
	}
	if m == nil {
		return writeFailure("read-manifest", "run manifest is absent")
	}
	m.SupervisorPID = pid
	m.PGID = pgid
	m.SID = sid
	m.Phase = "established"
	m.UpdatedAt = supervisorStamp()
	if err := writeAtomicJSON(filepath.Join(runDir, manifestFile), m); err != nil {
		return writeFailure("publish-established", "recording the established manifest failed")
	}
	handshake("established\n")

	// (5) Survive a group-directed TERM/INT: by taking delivery on a channel
	// the supervisor replaces the default terminate disposition and outlives
	// the signal long enough to wait on and record the child. The child keeps
	// its default disposition and dies by the group signal.
	sigCh := make(chan os.Signal, 8)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		for s := range sigCh {
			diag.Printf("received %v; continuing to supervise until the child is reaped", s)
		}
	}()

	// (6) Decode the child argv, then build the command with a scrubbed env
	// (both private vars removed), the manifest cwd, /dev/null stdin, and the
	// two durable log files. No SysProcAttr — the child joins the supervisor's
	// session/group so one group signal reaches it.
	var argv []string
	if err := json.Unmarshal([]byte(os.Getenv(supervisorArgvEnv)), &argv); err != nil || len(argv) == 0 {
		return writeFailure("decode-argv", "supervised command argv is missing or malformed")
	}
	stdoutF, err := openSupervisorFile(filepath.Join(runDir, stdoutLogFile), os.O_TRUNC)
	if err != nil {
		return writeFailure("open-stdout-log", "opening the stdout log failed")
	}
	defer stdoutF.Close()
	stderrF, err := openSupervisorFile(filepath.Join(runDir, stderrLogFile), os.O_TRUNC)
	if err != nil {
		return writeFailure("open-stderr-log", "opening the stderr log failed")
	}
	defer stderrF.Close()
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		return writeFailure("open-devnull", "opening /dev/null for child stdin failed")
	}
	defer devNull.Close()

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = envWithout(os.Environ(), supervisorRunDirEnv, supervisorArgvEnv)
	cmd.Dir = m.Cwd
	cmd.Stdin = devNull
	cmd.Stdout = stdoutF
	cmd.Stderr = stderrF

	// (7) Start the child. A start failure is a supervisor failure record —
	// never a fabricated terminal record. The bounded reason omits argv so the
	// path never reaches protocol error text; the raw error goes to the log.
	if err := cmd.Start(); err != nil {
		diag.Printf("start failed: %v", err)
		return writeFailure("start-command", "starting the supervised command failed")
	}
	m.Phase = "running"
	m.UpdatedAt = supervisorStamp()
	if err := writeAtomicJSON(filepath.Join(runDir, manifestFile), m); err != nil {
		diag.Printf("recording running phase failed: %v", err)
	}
	handshake("running\n")

	// (8) Wait, then decode the EXACT wait status: a normal exit stays
	// kind=exit with the true code, a signal death stays kind=signal with the
	// true signal number. No 128+signal heuristic anywhere.
	_ = cmd.Wait()
	ws, ok := cmd.ProcessState.Sys().(syscall.WaitStatus)
	var term *terminalRecord
	switch {
	case ok && ws.Exited():
		term = &terminalRecord{Schema: recordSchema, RunID: runID, Kind: "exit", ExitCode: ws.ExitStatus(), RecordedAt: supervisorStamp()}
	case ok && ws.Signaled():
		term = &terminalRecord{Schema: recordSchema, RunID: runID, Kind: "signal", Signal: int(ws.Signal()), RecordedAt: supervisorStamp()}
	default:
		return writeFailure("decode-wait", "child wait status was neither a normal exit nor a signal")
	}
	if err := writeAtomicJSON(filepath.Join(runDir, terminalFile), term); err != nil {
		return writeFailure("record-terminal", "recording the terminal status failed")
	}
	m.Phase = "terminal"
	m.UpdatedAt = supervisorStamp()
	if err := writeAtomicJSON(filepath.Join(runDir, manifestFile), m); err != nil {
		diag.Printf("recording terminal phase failed: %v", err)
	}
	// The terminal record is durable now. Wake the launcher, then release the
	// lock LAST so no observer can see a free lock before the terminal record.
	handshake("terminal\n")
	closePipe()
	closeLock()
	return 0
}

// openSupervisorFile opens (creating) a 0600 run-directory file with the given
// extra open flag (O_APPEND or O_TRUNC), enforcing the mode with an explicit
// chmod because a create-time mode is umask-masked.
func openSupervisorFile(path string, extraFlag int) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|extraFlag, 0o600)
	if err != nil {
		return nil, err
	}
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}
