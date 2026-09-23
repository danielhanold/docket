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
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/danielhanold/docket/internal/gatedrive"
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
// "run-cancelled" (the owning epoch is cancelling/cancelled), "stale-run-epoch"
// (the owning epoch was superseded by a resume), or "run-completed" (the owning
// epoch is completing/completed a successful closeout, change 0441). It carries no
// credential, argv, environment, or child output — only the bounded reason token.
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
	// ErrRunCompleted: the owning run epoch finished (or is finishing) a SUCCESSFUL
	// closeout (change 0441) — completing/completed. No new mutation, launch, or
	// registration from that epoch admits, and the refusal is distinguishable from
	// cancellation so a caller can tell a successful retirement from a stop.
	ErrRunCompleted = &MutationFenceError{Reason: "run-completed"}
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

// fenceRefusalReasonMessage resolves a run-epoch mutation fence error into its
// stable machine reason token AND an accurate human message clause for the owned
// subject ("change" / "workspace"). Only the reason token is machine-readable —
// gate logic keys on it and it rides through unchanged. The message is explanatory
// and reason-aware: a run-completed fence (change 0441) is a SUCCESSFUL closeout, so
// its message never falsely claims cancellation; run-cancelled/stale-run-epoch keep
// the cancelled-or-superseded wording. An unrecognized reason falls back to the
// reason-neutral "no longer accepting mutations" phrasing. An owner-resolution
// refusal (ErrEpochOwnerAmbiguous / ErrEpochOwnerUnresolved, change 0446) reports its
// own kind as the reason, never relabelled as a cancellation.
func fenceRefusalReasonMessage(ferr error, subject string) (reason, message string) {
	reason = "run-cancelled"
	if fe, ok := AsMutationFenceError(ferr); ok {
		reason = fe.Reason
	} else if ee, ok := AsEpochError(ferr); ok &&
		(ee.Kind == ErrEpochOwnerAmbiguous || ee.Kind == ErrEpochOwnerUnresolved) {
		// An unresolved or contradictory CURRENT owner (change 0446 §§1, 5) is not a
		// cancellation: surface its own kind and the locator the error carries.
		return string(ee.Kind), "the run that owns this " + subject + " cannot be resolved (" +
			ferr.Error() + "); publish nothing"
	}
	switch reason {
	case "run-cancelled", "stale-run-epoch":
		message = "the run that owns this " + subject + " was cancelled or superseded; publish nothing"
	case "run-completed":
		message = "the run that owns this " + subject + " finished successfully and is no longer accepting mutations; publish nothing"
	default:
		message = "the run that owns this " + subject + " is no longer accepting mutations; publish nothing"
	}
	return reason, message
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
//     no epoch, or a pruned epoch no slot names) → (noopJournalDone, nil): admit
//     UNFENCED. This is the standalone contract — a mutation outside any workflow run
//     is unchanged.
//   - Owning epoch ACTIVE → journal an `admitted` entry under the epoch's
//     compare-and-swap and return (done, nil). `done(completed|uncertain)` updates
//     exactly that entry after the mutation resolves.
//   - Owning epoch CANCELLING/CANCELLED → (nil, ErrRunCancelled).
//   - Owning epoch SUPERSEDED → (nil, ErrStaleRunEpoch).
//   - Owning epoch COMPLETING/COMPLETED → (nil, ErrRunCompleted). A completed epoch
//     is normally already excluded from findEpochByWorktree, so this arm chiefly
//     fences a completing (mid-closeout) epoch; the completed arm is defense in depth.
//   - Two or more active owners bound to the worktree → (nil, ErrEpochOwnerAmbiguous):
//     a contradiction refuses locally, never resolved by order (change 0446 §5).
//   - No ambient owner, but the worktree's execution slot NAMES a RunEpochID that no
//     readable epoch record carries (corrupt, IO-unreadable, or absent) → fail closed
//     with ErrEpochOwnerUnresolved naming the epoch id and worktree (change 0446 §1).
//     A record the store cannot read is never treated as a free run when a current
//     reference names it; an unreadable epoch no slot names stays diagnostic.
//   - Any registry enumeration or slot-store fault while resolving that owner → fail
//     closed: (nil, err).
//
// The state gate and the `admitted` append happen in ONE epochCAS, so a cancellation
// that fences the epoch between the slot read and the journal write is observed
// atomically — there is no admit-then-fenced window.
func admitWorkflowMutation(repoDir, op string) (mutationJournalDone, error) {
	// Canonicalize the change's feature worktree so a different spelling of one
	// worktree cannot dodge the fence. A path that cannot be canonicalized (it does
	// not exist) owns no epoch — admit unfenced. This fail-open on the CALLER's own
	// uncanonicalizable repoDir is an accepted residual risk the change 0446 spec
	// records verbatim ("The path fence also fails open when the caller's own
	// `repoDir` cannot be canonicalized; keep that behavior unless implementation
	// shows a named caller relies on it, and record the decision"). Decision: kept.
	// The production callers (MutationAdmissionHook, PRPublish, WorkspacePublish)
	// pass the repository directory the mutation itself runs in, so a missing path
	// cannot run the mutation either; none relies on the fence refusing it.
	canon, err := canonicalWorktree(repoDir)
	if err != nil {
		return noopJournalDone, nil
	}

	gateKey, found, ferr := findEpochByWorktree(repoDir, canon)
	if ferr != nil {
		// The registry could not be enumerated, or two active owners contradict each
		// other: fail closed rather than admit a mutation whose owner is unknown.
		return nil, ferr
	}
	if !found {
		// No readable run epoch owns this worktree. Before admitting unfenced, honour
		// the worktree slot's positive reference (spec §1): a slot naming an epoch that
		// no readable record carries means the current owner is unresolved.
		if uerr := slotNamedEpochUnresolved(repoDir, canon); uerr != nil {
			return nil, uerr
		}
		// Standalone use, or the run's epoch was pruned with its gate record and no
		// slot names it: the mutation is unfenced.
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
		case EpochCompleting, EpochCompleted:
			// A successful closeout (fenced or finished) admits no new mutation, and
			// its refusal is distinguishable from cancellation (change 0441). Defense
			// in depth for completed: findEpochByWorktree already drops a completed
			// epoch from ambient lookup, so this arm is normally reached only for
			// completing.
			return ErrRunCompleted
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

// findEpochByWorktree resolves the CURRENT ambient owner of the canonical worktree
// canon by scanning the repository's rungate root (each gate-key directory may hold
// one epoch.json beside its gate record) and returns the owning epoch's gate key. It
// collects EVERY matching epoch first and then selects deterministically (change
// 0446 spec §5, following FindEpochByChange's established shape) — never the first
// directory-order match:
//
//   - exactly one active or completing match is the owner, regardless of directory
//     order and of any cancelled/cancelling record bound to the same path (a fresh run
//     is not masked by a never-superseded cancelled predecessor);
//   - two or more active/completing matches are a contradiction: a typed
//     ErrEpochOwnerAmbiguous naming the worktree, never one chosen by order;
//   - with no active/completing match, a fenced (cancelling, cancelled, or — only
//     when its Worktree was never cleared — superseded) match is still returned, so
//     the mutation fence keeps refusing until the recovery/replacement workflow
//     authorizes current work. Every fenced match refuses identically, so the
//     lexically first gate key is returned for stability.
//
// A match whose state is none of the known ones is counted with the active matches:
// it cannot be proven to have stopped owning the path, so it can neither be outranked
// silently nor let another active owner through unchallenged (admitWorkflowMutation
// fails it closed as run-cancelled when it is the sole match).
//
// A missing rungate root or no match is (found=false, err=nil). A directory-
// enumeration IO error is returned so the caller fails closed. A gate directory with
// no epoch.json, an epoch with no bound Worktree (a standalone or not-yet-claimed
// run, and every superseded epoch — SupersedeCancelledEpoch clears it), a fully
// COMPLETED epoch (a successful closeout no longer owns its worktree — change 0441; a
// completing epoch still does), or an UNREADABLE/corrupt record is skipped. Skipping
// an unreadable record is discovery, not required evidence (spec §1): the required
// half — a slot that NAMES an epoch no readable record carries — is enforced by
// admitWorkflowMutation through that positive slot reference, never by failing
// closed on every unreadable sibling.
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
	var owners, fenced []string
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
		if r.State == EpochCompleted {
			continue // a fully completed epoch no longer owns any worktree (change 0441)
		}
		if !epochOwnsWorktree(r.Worktree, canon) {
			continue
		}
		switch r.State {
		case EpochCancelling, EpochCancelled, EpochSuperseded:
			fenced = append(fenced, key)
		default: // active, completing, or an unknown state that cannot be outranked
			owners = append(owners, key)
		}
	}
	switch {
	case len(owners) == 1:
		return owners[0], true, nil
	case len(owners) > 1:
		sort.Strings(owners)
		return "", false, epochErr(ErrEpochOwnerAmbiguous, "find-by-worktree",
			fmt.Errorf("%d active run epochs (gate keys %s) are bound to worktree %s",
				len(owners), strings.Join(owners, ", "), canon))
	case len(fenced) > 0:
		sort.Strings(fenced)
		return fenced[0], true, nil
	}
	return "", false, nil
}

// slotNamedEpochUnresolved enforces the required-evidence half of ambient owner
// lookup (change 0446 spec §1): "when the requested worktree's slot names a
// `RunEpochID`, ambient owner lookup for that worktree must resolve that epoch to a
// readable record". It is consulted only after findEpochByWorktree found no readable
// owner. It opens the gatedrive admission store at the repository's Git common dir
// (the same store productionCancelSeams opens) and reads the worktree's slot:
//
//   - an absent or unreadable slot, or a slot with no RunEpochID, names nothing →
//     nil (the unfenced admit is unchanged — a slot the store cannot read is the
//     gate-admission authority's to refuse, not ambient ownership evidence);
//   - a slot naming an epoch some readable record carries → nil (that epoch simply
//     does not own this path now: completed, superseded, not yet bound, or bound
//     elsewhere);
//   - a slot naming an epoch NO readable record carries → ErrEpochOwnerUnresolved
//     naming the epoch id and worktree. The positive reference comes from the slot,
//     so the refusal never depends on reading the unreadable record itself;
//   - a common-dir, registry-enumeration, or ambiguous-id fault → that error (fail
//     closed).
//
// The epoch id is a public locator, never a credential (ADR-0111), so the locator
// carries it verbatim.
func slotNamedEpochUnresolved(repoDir, canon string) error {
	common, err := gateGitCommonDir(repoDir)
	if err != nil {
		return err
	}
	slot, _, lerr := gatedrive.OpenStore(common).LoadWorktreeExecution(canon)
	if lerr != nil || slot.RunEpochID == "" {
		return nil
	}
	_, ok, ferr := findEpochByID(filepath.Join(common, "docket", "rungate"), slot.RunEpochID)
	if ferr != nil {
		return ferr
	}
	if ok {
		return nil
	}
	return epochErr(ErrEpochOwnerUnresolved, "find-by-worktree",
		fmt.Errorf("the execution slot of worktree %s names run epoch %s, but no readable epoch record carries it; inspect or repair that epoch record before mutating this worktree",
			canon, slot.RunEpochID))
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
