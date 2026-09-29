// The production run launch gate (change 0437 Task 5). It is the app-owned
// authoritative liveness read the native gate driver runs its durable
// admission/reservation body under (gatedrive.RunLaunchGate): a run that a
// cancellation has fenced, a resume has superseded, or that does not own the
// worktree a start names must NOT be allowed to launch or reserve a new gate
// execution.
//
// SERIALIZATION. The gate holds the SAME per-key run.lock the mutation fence and
// the cancellation transition serialize on (runtracker_run_record.go's runRecordCAS), across
// both the liveness read AND reserve, so a concurrent active→cancelling fence has
// exactly two outcomes: it lands BEFORE the gate's locked read (the gate refuses and
// reserve never runs) or AFTER reserve committed the driver's durable reservation
// (the fence then observes that reservation). There is no admit-then-fenced window.
// The gate never holds the lock across a process launch — reserve is only the
// bounded reservation body; the driver launches OUTSIDE the gate (change 0437 Task 2).
//
// NO RUN-RECORD WRITE. The gate is a READ under the lock. It NEVER calls runRecordCAS and
// never rewrites the record: a trailing rewrite could fail after reserve already
// succeeded, so the liveness path stays strictly read-only (spec "No run write
// from the liveness path"). It reuses the existing lock/read primitives
// (acquireRunLock, readStoredRun) and the existing worktree-ownership predicate
// (runOwnsWorktree), never a second copy.
//
// FAIL CLOSED. Every location or liveness failure refuses through the EXISTING fence
// vocabulary — cancelling/cancelled → ErrRunCancelled, superseded → ErrStaleRunID,
// completing/completed (a successful closeout, change 0441) → ErrRunCompleted,
// a missing/wrong/omitted worktree binding → ErrStaleRunID, and a
// missing/ambiguous/corrupt/unreadable run → the typed RunError. A record the
// store cannot resolve to a single live, worktree-bound run is never a free pass.
package app

import (
	"path/filepath"

	"github.com/danielhanold/docket/internal/gatedrive"
)

// runLaunchGate builds the production gatedrive.RunLaunchGate over this
// repository's run registry (rooted at gitCommonDir, the same root
// runRevokedResolver derives). The returned gate locates the run by its public
// id (unique match), acquires that key's run.lock, RE-READS the record under the
// lock (the unlocked scan only located the directory), validates that the run is
// active AND owns the worktree the start names, and only then runs reserve while
// still holding the lock. It NEVER writes the run record. It fires only for a
// non-empty run id (the driver's runLaunchGated helper calls it only then), so a
// standalone gate that carries no run keeps its existing behavior.
func runLaunchGate(gitCommonDir string) gatedrive.RunLaunchGate {
	runTrackerRoot := filepath.Join(gitCommonDir, "docket", runTrackerDirName)
	return func(runID, worktree string, reserve func() error) error {
		// Canonicalize the worktree the start names ONCE, outside the lock (fingerprint
		// and path resolution stay out of the run critical section). A worktree the
		// gate cannot canonicalize cannot be proven owned by the run — fail closed.
		canon, cerr := canonicalWorktree(worktree)
		if cerr != nil {
			return ErrStaleRunID
		}
		// Locate the unique run-key directory holding this run. A missing,
		// ambiguous, corrupt, or unreadable registry is a typed RunError refusal —
		// never a free pass for a run whose liveness cannot be established.
		dir, _, ferr := findRunDirByID(runTrackerRoot, runID)
		if ferr != nil {
			return ferr
		}
		// Hold the per-key run lock across the liveness read AND reserve, so a
		// concurrent cancellation fence serializes against this gate rather than
		// interleaving with the durable reservation.
		lock, lerr := acquireRunLock(dir)
		if lerr != nil {
			return lerr
		}
		defer lock.Close()
		// Re-read under the lock: the unlocked scan only located the directory, and a
		// fence may have landed since. A record that has become unreadable fails closed.
		rec, _, rerr := readStoredRun(dir, "run-launch-gate")
		if rerr != nil {
			return rerr
		}
		switch rec.State {
		case RunActive:
			// Live: fall through to the worktree-ownership check.
		case RunCancelling, RunCancelled:
			return ErrRunCancelled
		case RunSuperseded:
			return ErrStaleRunID
		case RunCompleting, RunCompleted:
			// A successful closeout (fenced or finished) admits no launch, delayed
			// ticket, or relaunch (change 0441). This refusal is what later settles a
			// pre-fence never-launched ticket terminal, and it is distinguishable from
			// a cancellation.
			return ErrRunCompleted
		default:
			// An unknown state is never a live run: fail closed as cancelled, mirroring
			// admitWorkflowMutation's unknown-state handling.
			return ErrRunCancelled
		}
		// The run must OWN the worktree the start names. An empty binding (a run
		// that never claimed a worktree) or a different worktree is the same
		// fail-closed refusal — omission and substitution are one refusal.
		if rec.Worktree == "" || !runOwnsWorktree(rec.Worktree, canon) {
			return ErrStaleRunID
		}
		return reserve()
	}
}
