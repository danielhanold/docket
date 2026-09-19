// The production epoch launch gate (change 0437 Task 5). It is the app-owned
// authoritative liveness read the native gate driver runs its durable
// admission/reservation body under (gatedrive.EpochLaunchGate): a run epoch that a
// cancellation has fenced, a resume has superseded, or that does not own the
// worktree a start names must NOT be allowed to launch or reserve a new gate
// execution.
//
// SERIALIZATION. The gate holds the SAME per-key epoch.lock the mutation fence and
// the cancellation transition serialize on (rungate_epoch.go's epochCAS), across
// both the liveness read AND reserve, so a concurrent active→cancelling fence has
// exactly two outcomes: it lands BEFORE the gate's locked read (the gate refuses and
// reserve never runs) or AFTER reserve committed the driver's durable reservation
// (the fence then observes that reservation). There is no admit-then-fenced window.
// The gate never holds the lock across a process launch — reserve is only the
// bounded reservation body; the driver launches OUTSIDE the gate (change 0437 Task 2).
//
// NO EPOCH WRITE. The gate is a READ under the lock. It NEVER calls epochCAS and
// never rewrites the record: a trailing rewrite could fail after reserve already
// succeeded, so the liveness path stays strictly read-only (spec "No epoch write
// from the liveness path"). It reuses the existing lock/read primitives
// (acquireEpochLock, readStoredEpoch) and the existing worktree-ownership predicate
// (epochOwnsWorktree), never a second copy.
//
// FAIL CLOSED. Every location or liveness failure refuses through the EXISTING fence
// vocabulary — cancelling/cancelled → ErrRunCancelled, superseded → ErrStaleRunEpoch,
// a missing/wrong/omitted worktree binding → ErrStaleRunEpoch, and a
// missing/ambiguous/corrupt/unreadable epoch → the typed EpochError. A record the
// store cannot resolve to a single live, worktree-bound epoch is never a free pass.
package app

import (
	"path/filepath"

	"github.com/danielhanold/docket/internal/gatedrive"
)

// epochLaunchGate builds the production gatedrive.EpochLaunchGate over this
// repository's run-epoch registry (rooted at gitCommonDir, the same root
// epochRevokedResolver derives). The returned gate locates the epoch by its public
// id (unique match), acquires that key's epoch.lock, RE-READS the record under the
// lock (the unlocked scan only located the directory), validates that the epoch is
// active AND owns the worktree the start names, and only then runs reserve while
// still holding the lock. It NEVER writes the epoch record. It fires only for a
// non-empty epoch id (the driver's epochGated helper calls it only then), so a
// standalone gate that carries no run epoch keeps its existing behavior.
func epochLaunchGate(gitCommonDir string) gatedrive.EpochLaunchGate {
	rungateRoot := filepath.Join(gitCommonDir, "docket", "rungate")
	return func(epochID, worktree string, reserve func() error) error {
		// Canonicalize the worktree the start names ONCE, outside the lock (fingerprint
		// and path resolution stay out of the epoch critical section). A worktree the
		// gate cannot canonicalize cannot be proven owned by the epoch — fail closed.
		canon, cerr := canonicalWorktree(worktree)
		if cerr != nil {
			return ErrStaleRunEpoch
		}
		// Locate the unique gate-key directory holding this epoch. A missing,
		// ambiguous, corrupt, or unreadable registry is a typed EpochError refusal —
		// never a free pass for a run whose liveness cannot be established.
		dir, _, ferr := findEpochDirByID(rungateRoot, epochID)
		if ferr != nil {
			return ferr
		}
		// Hold the per-key epoch lock across the liveness read AND reserve, so a
		// concurrent cancellation fence serializes against this gate rather than
		// interleaving with the durable reservation.
		lock, lerr := acquireEpochLock(dir)
		if lerr != nil {
			return lerr
		}
		defer lock.Close()
		// Re-read under the lock: the unlocked scan only located the directory, and a
		// fence may have landed since. A record that has become unreadable fails closed.
		rec, _, rerr := readStoredEpoch(dir, "epoch-launch-gate")
		if rerr != nil {
			return rerr
		}
		switch rec.State {
		case EpochActive:
			// Live: fall through to the worktree-ownership check.
		case EpochCancelling, EpochCancelled:
			return ErrRunCancelled
		case EpochSuperseded:
			return ErrStaleRunEpoch
		default:
			// An unknown state is never a live epoch: fail closed as cancelled, mirroring
			// admitWorkflowMutation's unknown-state handling.
			return ErrRunCancelled
		}
		// The epoch must OWN the worktree the start names. An empty binding (an epoch
		// that never claimed a worktree) or a different worktree is the same
		// fail-closed refusal — omission and substitution are one refusal.
		if rec.Worktree == "" || !epochOwnsWorktree(rec.Worktree, canon) {
			return ErrStaleRunEpoch
		}
		return reserve()
	}
}
