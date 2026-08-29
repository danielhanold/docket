// This file owns single-target child execution and the live-child registry.
// ExecuteTarget runs one target under bash in its Sandbox, captures combined
// output, counts the ok/NOT OK markers exactly as the oracle does
// (scripts/run-tests.sh launch()), and atomically publishes the durable Result.
// The child is placed in its own process group (Setpgid) and registered with a
// procRegistry so the signal layer (Task 6) can reach the whole process tree —
// exec.CommandContext is deliberately NOT used, because its ctx-kill reaches
// only the direct child, orphaning grandchildren.
package suiterunner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"syscall"
	"time"
)

// reOK / reNotOK match the oracle's marker grammar per log line:
// `^ok[[:space:]]*-` and `^NOT OK` (scripts/run-tests.sh launch()).
var (
	reOK    = regexp.MustCompile(`^ok[[:space:]]*-`)
	reNotOK = regexp.MustCompile(`^NOT OK`)
)

// procRegistry tracks live child process-group ids so the signal layer can
// forward to whole groups. Task 6 owns its signal methods; the type and
// Register/Unregister/Snapshot live here so ExecuteTarget can compile and the
// scheduler can share one registry across the run.
type procRegistry struct {
	mu    sync.Mutex
	pgids map[int]bool
}

func newProcRegistry() *procRegistry {
	return &procRegistry{pgids: make(map[int]bool)}
}

// Register records a live child pgid. A non-positive pgid is ignored: a
// negative or zero value would later be forwarded as kill(-pgid) to a group
// that is not the child's, so it is never admitted.
func (r *procRegistry) Register(pgid int) {
	if pgid <= 0 {
		return
	}
	r.mu.Lock()
	r.pgids[pgid] = true
	r.mu.Unlock()
}

// Unregister drops a pgid once its child has been reaped — a finished job's
// pgid is reusable, and signalling a recycled group is a worse bug than the one
// the registry prevents.
func (r *procRegistry) Unregister(pgid int) {
	r.mu.Lock()
	delete(r.pgids, pgid)
	r.mu.Unlock()
}

// Snapshot returns the currently-live pgids in ascending order, for
// deterministic iteration by the signal layer and tests.
func (r *procRegistry) Snapshot() []int {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]int, 0, len(r.pgids))
	for p := range r.pgids {
		out = append(out, p)
	}
	sort.Ints(out)
	return out
}

// Signal forwards sig to every live child process GROUP via kill(-pgid, sig).
// This is the signal layer's forwarding path (InstallSignalHandling). Signalling
// the GROUP, not the leader's pid, is the whole point of the Setpgid child layout
// (see ExecuteTarget): a target and every process it forks share one process
// group but have distinct pids, so kill(pid) would reach only the leader and
// orphan its grandchildren — the exact data-destroying failure the Bash oracle's
// on_signal handler (scripts/run-tests.sh) works around pid-by-pid. Each pgid is
// re-guarded >0 even though Register already refuses non-positive values: passing
// pgid<=0 to kill(-pgid) would address the caller's own group or a broad set of
// processes, so the guard is load-bearing, not defensive dead code. A per-group
// error (ESRCH for a group that already exited — the success case for a kill) is
// intentionally ignored: the interrupt path must fail open, never wedge on one.
func (r *procRegistry) Signal(sig syscall.Signal) {
	for _, pgid := range r.Snapshot() {
		if pgid <= 0 {
			continue
		}
		_ = syscall.Kill(-pgid, sig)
	}
}

// ExecuteTarget runs one target under bash in its sandbox, captures combined
// output to <work>/logs/<stem>.log, counts ok/NOT OK markers, and atomically
// publishes the Result to <work>/stat/<stem>.json. It sets the child's process
// group (Setpgid) and registers it with reg so the signal layer can reach the
// whole tree. extraEnv entries (e.g. DOCKET_RUNTESTS_SOLO=1) are appended last,
// so they override the sandbox defaults. The returned Result is the
// runner-observed truth the aggregator later cross-checks against the durable
// file. A non-nil error signals an infrastructure failure (could not stage the
// sandbox, start bash, or publish the record) — a non-zero test exit is NOT an
// error, it is carried in Result.RC.
func ExecuteTarget(ctx context.Context, bash string, t Target, work string, reg *procRegistry, extraEnv []string) (Result, error) {
	stem := statStem(t.Base)
	jobdir := filepath.Join(work, "jobs", stem)
	logsDir := filepath.Join(work, "logs")
	statDir := filepath.Join(work, "stat")
	for _, d := range []string{jobdir, logsDir, statDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return Result{}, fmt.Errorf("suiterunner: execute %s: mkdir %q: %w", t.Base, d, err)
		}
	}

	env, err := Sandbox(jobdir)
	if err != nil {
		return Result{}, fmt.Errorf("suiterunner: execute %s: %w", t.Base, err)
	}
	env = append(env, extraEnv...)

	logPath := filepath.Join(logsDir, stem+".log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return Result{}, fmt.Errorf("suiterunner: execute %s: create log: %w", t.Base, err)
	}

	cmd := exec.Command(bash, t.Path)
	cmd.Env = env
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	// A new process group so the signal layer can reach the target AND every
	// grandchild via kill(-pgid). NOT exec.CommandContext: its ctx-kill signals
	// only the direct child. ctx is honored by the scheduler (it stops launching
	// on cancel) and by the signal layer, not by a per-child kill here.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	start := time.Now()
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return Result{}, fmt.Errorf("suiterunner: execute %s: start bash: %w", t.Base, err)
	}

	// With Setpgid and an unset Pgid, the child leads its own group, so its pgid
	// equals its pid. Fall back to the pid if the group lookup races a fast exit.
	pgid, gerr := syscall.Getpgid(cmd.Process.Pid)
	if gerr != nil {
		pgid = cmd.Process.Pid
	}
	reg.Register(pgid)

	runErr := cmd.Wait()
	reg.Unregister(pgid)
	logFile.Close()

	seconds := int(time.Since(start).Seconds())
	rc := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			rc = exitErr.ExitCode()
		} else {
			return Result{}, fmt.Errorf("suiterunner: execute %s: wait: %w", t.Base, runErr)
		}
	}

	ok, notok, err := countMarkers(logPath)
	if err != nil {
		return Result{}, fmt.Errorf("suiterunner: execute %s: %w", t.Base, err)
	}

	res := Result{
		Schema:  resultSchema,
		Target:  t.Base,
		RC:      rc,
		Seconds: seconds,
		OK:      ok,
		NotOK:   notok,
	}
	if err := WriteResult(statDir, res); err != nil {
		return Result{}, fmt.Errorf("suiterunner: execute %s: %w", t.Base, err)
	}
	return res, nil
}

// statStem strips the .sh suffix from a target base to form the stat/log stem,
// mirroring the oracle's `base="${base%.sh}"`.
func statStem(base string) string {
	if len(base) > 3 && base[len(base)-3:] == ".sh" {
		return base[:len(base)-3]
	}
	return base
}

// countMarkers scans a log file line by line and counts ok/NOT OK markers with
// the oracle's exact anchored patterns.
func countMarkers(logPath string) (ok, notok int, err error) {
	data, err := os.ReadFile(logPath)
	if err != nil {
		return 0, 0, fmt.Errorf("read log %q: %w", logPath, err)
	}
	from := 0
	for i := 0; i <= len(data); i++ {
		if i == len(data) || data[i] == '\n' {
			line := data[from:i]
			if reOK.Match(line) {
				ok++
			}
			if reNotOK.Match(line) {
				notok++
			}
			from = i + 1
		}
	}
	return ok, notok, nil
}
