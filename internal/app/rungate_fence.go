// Run-epoch mutation-boundary fencing (change 0375 Task 11). Cancellation fences
// the coordinator, not only its test supervisor: after a run epoch is fenced
// (rungate_cancel.go flips active→cancelling), no NEW workflow mutation from that
// epoch may be admitted, and an operation already in flight is journaled so a
// cancellation stays PENDING until it is observed/reconciled (it is never reported
// `cancelled` while an owned writer or unresolved external effect remains). This
// file is the admission gate the shared mutation boundaries pass through.
//
// DERIVATION RULE (spec "Later-workflow-action fencing": "Derive the covered
// mutations from maintained consumers ... do not rely on a remembered list").
// The covered production mutation paths are NOT an enumerated allowlist; they are
// derived mechanically:
//
//   - The metadata transaction engine: EVERY internal/repository/transaction
//     Engine.Execute caller is covered at once by the engine's AdmissionHook
//     (MutationAdmissionHook below), invoked before any per-attempt Git work. A new
//     Engine.Execute caller inherits the fence for free.
//   - The two NON-engine external mutators, found by repo-wide search for the
//     GitHub / workspace-publish mutators rather than memory: PR creation
//     (pr_publish.go, githubcli.EnsurePullRequest) and workspace publish
//     (workspace_ops.go, the feature-head push to origin). Each calls
//     admitWorkflowMutation directly at its boundary.
//   - change.mark-implemented (change_implemented.go) is covered VIA the engine
//     hook — its transition is one Engine.Execute — never a second admission call.
//
// The repoguard-style correspondence that every PRPublish / WorkspacePublish /
// Engine.Execute production path reaches admission is Task 14's computed guard
// (internal/repoguard/gatelaunch_admission_test.go); this file provides the single
// admission function all three reach and the anchor the guard keys on.
//
// LINKAGE. The owning epoch of a mutation is the run epoch bound to the change's
// canonical feature worktree — the SAME link run.cancel (rungate_cancel.go) drives
// the slot teardown through: EpochRecord.Worktree, the canonical feature worktree an
// epoch records once its run claims the workspace (spec "Bind a run epoch to the
// existing verified claim instance and canonical workspace"). A mutation boundary
// runs with repoDir = that feature worktree, so it canonicalizes repoDir and finds
// the epoch whose Worktree matches. A worktree no epoch owns (no matching record, or
// only standalone/empty-worktree epochs) admits the mutation UNFENCED — standalone
// use is unchanged. Two spellings of one worktree (a `/tmp` symlink to
// `/private/tmp`, say) canonicalize to one path, so a different spelling cannot
// dodge the fence.
//
// AUTHORITY vs. LOCATOR. The epoch id is a public locator, never a credential
// (ADR-0111): dispatch-context authority still governs who may run the mutation.
// The fence only revalidates that the located run epoch has not been cancelled; it
// never grants or withholds authority.
package app

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/danielhanold/docket/internal/repository/transaction"
)

// The AdmittedMutation.Status values Task 11 writes into the epoch journal (Task 10
// reads them; mutationStatusCompleted is the sole "accounted" status, defined in
// rungate_cancel.go). `admitted` is journaled before a mutation is performed;
// `completed` marks a mutation that finished with an observed outcome; `uncertain`
// marks one whose remote outcome could not be observed — both keep the durable
// record so a cancellation reconciles against a real journal, but only `completed`
// lets a cancellation reach `cancelled`.
const (
	mutationStatusAdmitted  = "admitted"
	mutationStatusUncertain = "uncertain"
)

// MutationFenceError is the typed refusal a fenced run epoch raises at a mutation
// boundary. Reason is one of the stable tokens the fence vocabulary defines —
// "run-cancelled" (the owning epoch is cancelling/cancelled) or "stale-run-epoch"
// (the owning epoch was superseded by a resume). It carries no credential, argv,
// environment, or child output — only the bounded reason token.
type MutationFenceError struct {
	Reason string
}

func (e *MutationFenceError) Error() string {
	return "workflow mutation refused: " + e.Reason
}

// The two fence refusals, as reusable sentinels (compared by identity through the
// errors chain, or by Reason). run-cancelled fences a cancelling/cancelled epoch;
// stale-run-epoch fences a superseded one.
var (
	// ErrRunCancelled: the owning run epoch is cancelling or cancelled — no new
	// mutation from that epoch is admitted (spec "After cancellation is recorded, no
	// new mutation from the old epoch is admitted").
	ErrRunCancelled = &MutationFenceError{Reason: "run-cancelled"}
	// ErrStaleRunEpoch: the owning run epoch was superseded by a confirmed resume —
	// this caller carries a stale run identity and is refused.
	ErrStaleRunEpoch = &MutationFenceError{Reason: "stale-run-epoch"}
)

// AsMutationFenceError unwraps err to a *MutationFenceError when one is in the
// chain, so a caller can surface the stable reason token.
func AsMutationFenceError(err error) (*MutationFenceError, bool) {
	var e *MutationFenceError
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// mutationJournalDone is the completion callback admitWorkflowMutation returns for
// an admitted (active-epoch) mutation. The caller invokes it exactly once after the
// mutation resolves — with mutationStatusCompleted on an observed success, or
// mutationStatusUncertain when the remote outcome could not be observed. It is
// best-effort: a journal-write failure never fails the (already-performed) mutation,
// and a repeated cancellation re-enumerates the journal to reconcile it.
type mutationJournalDone func(status string)

// noopJournalDone is the completion callback for an UNFENCED mutation (no owning
// epoch): there is nothing to reconcile, so completion is a no-op.
func noopJournalDone(string) {}

// admitWorkflowMutation is the run-epoch admission gate every covered mutation
// boundary passes through. repoDir is the change's canonical feature worktree; op
// is the operation key journaled. It returns a completion callback and an error:
//
//   - No owning epoch (no registered worktree, no slot, a standalone-gate slot with
//     no epoch, or a pruned epoch) → (noopJournalDone, nil): admit UNFENCED. This is
//     the standalone contract — a mutation outside any workflow run is unchanged.
//   - Owning epoch ACTIVE → journal an `admitted` entry under the epoch's
//     compare-and-swap and return (done, nil). `done(completed|uncertain)` updates
//     exactly that entry after the mutation resolves.
//   - Owning epoch CANCELLING/CANCELLED → (nil, ErrRunCancelled).
//   - Owning epoch SUPERSEDED → (nil, ErrStaleRunEpoch).
//   - Any epoch-store IO/corruption error while a slot NAMES an epoch → fail closed:
//     (nil, err). A record the store cannot read is never treated as a free run.
//
// The state gate and the `admitted` append happen in ONE epochCAS, so a cancellation
// that fences the epoch between the slot read and the journal write is observed
// atomically — there is no admit-then-fenced window.
func admitWorkflowMutation(repoDir, op string) (mutationJournalDone, error) {
	// Canonicalize the change's feature worktree so a different spelling of one
	// worktree cannot dodge the fence. A path that cannot be canonicalized (it does
	// not exist) owns no epoch — admit unfenced.
	canon, err := canonicalWorktree(repoDir)
	if err != nil {
		return noopJournalDone, nil
	}

	gateKey, found, ferr := findEpochByWorktree(repoDir, canon)
	if ferr != nil {
		// The registry could not be enumerated: fail closed rather than admit a
		// mutation whose owning epoch might be fenced.
		return nil, ferr
	}
	if !found {
		// No run epoch owns this worktree (standalone use, or the run's epoch was
		// pruned with its gate record): the mutation is unfenced.
		return noopJournalDone, nil
	}

	// Atomic state gate + `admitted` journal. epochCAS serializes on the per-key
	// epoch lock, so a concurrent run.cancel either loses the race (this admit wins
	// and is later reconciled) or wins (this returns the fence refusal) — never both.
	var idx int
	cerr := epochCAS(repoDir, gateKey, func(rec *EpochRecord) error {
		switch rec.State {
		case EpochActive:
			idx = len(rec.AdmittedMutations)
			rec.AdmittedMutations = append(rec.AdmittedMutations, AdmittedMutation{
				OpKey:  op,
				Status: mutationStatusAdmitted,
			})
			return nil
		case EpochSuperseded:
			return ErrStaleRunEpoch
		case EpochCancelling, EpochCancelled:
			return ErrRunCancelled
		default:
			// An unknown/corrupt state is never a free run: fail closed as cancelled.
			return ErrRunCancelled
		}
	})
	if cerr != nil {
		return nil, cerr
	}

	done := func(status string) {
		// Best-effort reconciliation: update exactly the entry this admission appended.
		// No state gate — marking an in-flight mutation completed/uncertain must work
		// even after the epoch was fenced, so a cancellation can move from pending to
		// cancelled once every admitted mutation is reconciled.
		_ = epochCAS(repoDir, gateKey, func(rec *EpochRecord) error {
			if idx >= 0 && idx < len(rec.AdmittedMutations) {
				rec.AdmittedMutations[idx].Status = status
			}
			return nil
		})
	}
	return done, nil
}

// mutationJournalStatus maps a boundary's final protocol Result to the completion
// status its admitted-mutation journal entry gets. An UNOBSERVED remote outcome —
// an external failure (a transport error, or an explicit unknown disposition) or an
// interruption — is `uncertain`, so a cancellation stays pending until it is
// reconciled; every observed outcome (applied, no-op, contended, or a pre-mutation
// refusal that pushed nothing) is `completed`. Fail-safe: when in doubt an outcome
// is treated as uncertain, never prematurely completed.
func mutationJournalStatus(r Result) string {
	switch r {
	case ResultExternalFailed, ResultInterrupted:
		return mutationStatusUncertain
	default:
		return mutationStatusCompleted
	}
}

// canonicalWorktree resolves path to its absolute, every-symlink-hop-resolved form —
// the same canonical shape an epoch's Worktree is bound in — so two spellings of one
// worktree compare equal. It reaches no Git: a mutation boundary is invoked with the
// worktree root as repoDir, which is all the fence needs to match.
func canonicalWorktree(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

// findEpochByWorktree locates the run epoch whose canonical Worktree equals canon by
// scanning the repository's rungate root (each gate-key directory may hold one
// epoch.json beside its gate record). It returns that epoch's gate key. A missing
// rungate root or no match is (found=false, err=nil) — no epoch owns the worktree.
// A directory-enumeration IO error is returned so the caller fails closed. A gate
// directory with no epoch.json, an epoch with no bound Worktree (a standalone or
// not-yet-claimed run), or an UNREADABLE/corrupt record is skipped — the last a
// conservative, bounded fail-open confined to a corrupt UNRELATED record (the epoch
// store fails closed on its own reads elsewhere), never a free run for the matched
// epoch.
func findEpochByWorktree(repoDir, canon string) (gateKey string, found bool, err error) {
	root, rerr := gateRoot(repoDir)
	if rerr != nil {
		return "", false, rerr
	}
	entries, derr := os.ReadDir(root)
	if derr != nil {
		if errors.Is(derr, fs.ErrNotExist) {
			return "", false, nil // no rungate root: no epochs
		}
		return "", false, derr
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		key := e.Name()
		r, _, rerr := readStoredEpoch(filepath.Join(root, key), "find-epoch")
		if rerr != nil {
			continue // no epoch.json here, or a corrupt/unreadable record: cannot match
		}
		if r.Worktree == "" {
			continue // a standalone or not-yet-claimed run owns no worktree
		}
		if epochOwnsWorktree(r.Worktree, canon) {
			return key, true, nil
		}
	}
	return "", false, nil
}

// epochOwnsWorktree reports whether an epoch's stored Worktree names the same
// canonical worktree as canon. It canonicalizes the stored value defensively (an
// epoch bound before this canonicalization, or a fixture that stored a raw spelling)
// and falls back to a direct compare when the stored path no longer resolves (its
// worktree was cleaned up).
func epochOwnsWorktree(stored, canon string) bool {
	if stored == canon {
		return true
	}
	if resolved, err := canonicalWorktree(stored); err == nil {
		return resolved == canon
	}
	return false
}

// MutationAdmissionHook returns the transaction engine's AdmissionHook closure for
// repoDir (the change's feature worktree). It is wired where PlanningDeps are
// assembled, so EVERY Engine.Execute caller is fenced at once (the derivation rule
// above). The metadata transaction engine is atomic and idempotent — a lease-based
// push whose remote outcome the engine's own post-push probe always resolves to
// applied-or-failed within the same Execute — so it leaves NO lingering uncertain
// remote effect for a cancellation to separately reconcile. The engine journal
// entry is therefore admitted-and-completed at the admission boundary: the fence's
// load-bearing job for the engine is REFUSING a fenced epoch's new mutation, while
// the two async external boundaries (PR creation, workspace publish) carry the
// admitted→completed/uncertain in-flight journaling that genuinely matters.
func MutationAdmissionHook(repoDir string) func(transaction.OperationKey) error {
	return func(op transaction.OperationKey) error {
		done, err := admitWorkflowMutation(repoDir, string(op))
		if err != nil {
			return err
		}
		done(mutationStatusCompleted)
		return nil
	}
}

// Compile-time assertion: the hook closure matches the engine's AdmissionHook field.
var _ = func() func(transaction.OperationKey) error { return MutationAdmissionHook("") }
