package install

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/danielhanold/docket/internal/document"
)

// UninstallOptions describes an all-harness or explicitly scoped uninstall.
// SupportedHarnesses is the caller's current harness catalog; it validates
// explicit filters, while an unfiltered run deliberately also removes recorded
// harnesses that this binary no longer knows how to render.
type UninstallOptions struct {
	Roots              UserRoots
	FS                 FSOps
	Harnesses          []string
	SupportedHarnesses []string
	DryRun             bool
}

// Uninstall retires only harness-attributed targets proven unchanged from the
// installed-state record. The installation's own binary and every unattributed
// target remain recorded and untouched.
func Uninstall(options UninstallOptions) Outcome {
	out := Outcome{StatePath: options.Roots.StatePath()}
	selected, err := validateUninstallOptions(options)
	if err != nil {
		return fail(out, selectionReason(err), err)
	}

	if options.DryRun {
		lock, _, err := acquireReadOnlyInstallLock(options.Roots)
		if err != nil {
			return fail(out, lockReason(err), err)
		}
		if lock == nil {
			return out
		}
		defer lock.release()
		if txnID, found, err := DetectRecovery(options.Roots); err != nil {
			return fail(out, ReasonTransactionRecoveryRequired, err)
		} else if found {
			out.Actions = append(out.Actions, Action{Op: OpRecover, Path: filepath.Join(options.Roots.TransactionsDir(), txnID), Detail: "an interrupted transaction requires recovery"})
			return fail(out, ReasonTransactionRecoveryRequired, fmt.Errorf("%w: transaction %s requires recovery", ErrJournalInvalid, txnID))
		}
		plan, err := planUninstall(options.Roots, selected)
		if err != nil {
			return uninstallPlanFailure(out, err)
		}
		return reportUninstallPlan(out, plan, false)
	}

	lock, err := acquireInstallLock(options.Roots)
	if err != nil {
		return fail(out, lockReason(err), err)
	}
	defer lock.release()

	for {
		txnID, found, err := DetectRecovery(options.Roots)
		if err != nil {
			return fail(out, ReasonTransactionRecoveryRequired, err)
		}
		if !found {
			break
		}
		if err := Recover(options.FS, options.Roots, txnID); err != nil {
			return fail(out, ReasonTransactionRecoveryRequired, err)
		}
		out.Applied = true
		out.Actions = append(out.Actions, Action{Op: OpRecover, Path: filepath.Join(options.Roots.TransactionsDir(), txnID), Detail: "rolled back an interrupted transaction"})
	}
	if txnID, found, err := DetectRecovery(options.Roots); err != nil || found {
		if err == nil {
			err = fmt.Errorf("%w: transaction %s remains after recovery", ErrJournalInvalid, txnID)
		}
		return fail(out, ReasonTransactionRecoveryRequired, err)
	}

	plan, err := planUninstall(options.Roots, selected)
	if err != nil {
		return uninstallPlanFailure(out, err)
	}
	out = reportUninstallPlan(out, plan, out.Applied)
	if out.Err != nil {
		return out
	}
	if plan.prior == nil || (len(plan.removals) == 0 && plan.settled) {
		return out
	}
	desiredBytes, err := encodeState(plan.desired)
	if err != nil {
		return fail(out, ReasonInternal, err)
	}
	txn, err := BeginTxnWithRemovals(options.FS, options.Roots, nil, plan.removals)
	if err != nil {
		return fail(out, transactionReason(err), err)
	}
	if err := txn.Apply(); err != nil {
		return fail(out, transactionReason(err), err)
	}
	if err := txn.CommitDocs([]StateDoc{{Path: options.Roots.StatePath(), Bytes: desiredBytes}}); err != nil {
		return fail(out, ReasonFilesystemFailed, err)
	}
	out.Applied = true

	collection := CollectionOutcome{}
	if plan.desired.AssetSetID == "" && plan.desired.Mode == ModeRelease {
		collection = collectEmptyReleaseLocked(options, lock)
	} else {
		collection = collectLocked(CollectOptions{Roots: options.Roots, FS: options.FS}, lock)
	}
	if collection.Err != nil {
		// State publication is the uninstall's commit point. Collection is a
		// subsequent best-effort reclamation under the same lock, never a reason
		// to resurrect integrations that were successfully retired.
		return fail(out, ReasonFilesystemFailed, collection.Err)
	}
	if collection.Applied {
		out.Applied = true
	}
	return out
}

// collectEmptyReleaseLocked is the one adapter the generic collector cannot
// express: an inactive release state intentionally has no AssetSetID, hence no
// direct version reference. It reuses Task 5's proof, race seams, durable
// quarantine journal, and recovery path; only reference derivation is replaced
// by the already-published fact that the state is still empty.
func collectEmptyReleaseLocked(options UninstallOptions, lock *installLock) CollectionOutcome {
	out := CollectionOutcome{}
	if !lock.held() {
		out.Err = errors.New("install: empty-state collection requires the installation lock")
		return out
	}
	dataRoot, err := canonicalPath(options.Roots.DataRoot)
	if err != nil {
		out.Err = fmt.Errorf("install: canonicalising data root: %w", err)
		return out
	}
	options.Roots.DataRoot = dataRoot
	if journal, err := loadCollectionJournal(options.Roots); err != nil {
		out.Err = err
		out.Pending = []string{options.Roots.CollectionJournalPath()}
		return out
	} else if journal != nil {
		if err := reconcileCollectionJournal(options.FS, options.Roots); err != nil {
			out.Err = err
			out.Pending = []string{options.Roots.CollectionJournalPath()}
			return out
		}
		out.Applied = true
	}
	paths, versions, err := collectionCandidates(options.Roots)
	if err != nil {
		out.Err = err
		return out
	}
	for _, candidate := range paths {
		entry := CollectionEntry{AssetSetID: filepath.Base(candidate), Path: candidate}
		if !strictVersionChild(versions, candidate) {
			entry.Status, entry.Detail = CollectionStatusUnverified, "candidate is not strictly contained by the versions root"
			out.Entries = append(out.Entries, entry)
			continue
		}
		info, err := os.Lstat(candidate)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			if err == nil {
				err = errors.New("candidate is not a non-symlink directory")
			}
			entry.Status, entry.Detail = CollectionStatusUnverified, err.Error()
			out.Entries = append(out.Entries, entry)
			continue
		}
		canonical, err := canonicalPath(candidate)
		if err != nil || !strictVersionChild(versions, canonical) {
			if err == nil {
				err = errors.New("candidate escaped versions root")
			}
			entry.Status, entry.Detail = CollectionStatusUnverified, err.Error()
			out.Entries = append(out.Entries, entry)
			continue
		}
		entry.Path = canonical
		initial, err := ProveVersionTree(canonical)
		if err != nil {
			entry.Status, entry.Detail = CollectionStatusUnverified, err.Error()
			out.Entries = append(out.Entries, entry)
			continue
		}
		entry.AssetSetID = initial.Manifest.AssetSetID
		entry.Status = CollectionStatusCollected
		out.Entries = append(out.Entries, entry)

		collectBeforeReferenceRefresh(entry.Path)
		state, err := LoadState(options.Roots.StatePath())
		if err != nil || state == nil || state.Mode != ModeRelease || state.AssetSetID != "" || len(state.Harnesses) != 0 {
			if err == nil {
				err = fmt.Errorf("%w: installed state became active during empty-state collection", ErrStateInvalid)
			}
			out.Entries[len(out.Entries)-1].Status = CollectionStatusFailed
			out.Entries[len(out.Entries)-1].Detail = err.Error()
			out.Err = err
			break
		}
		collectBeforeQuarantine(entry.Path)
		canonical, err = canonicalPath(entry.Path)
		if err != nil || !strictVersionChild(versions, canonical) {
			if err == nil {
				err = errors.New("candidate escaped versions root before quarantine")
			}
			out.Entries[len(out.Entries)-1].Status = CollectionStatusFailed
			out.Entries[len(out.Entries)-1].Detail = err.Error()
			out.Err = err
			break
		}
		info, err = os.Lstat(entry.Path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			if err == nil {
				err = errors.New("candidate changed kind before quarantine")
			}
			out.Entries[len(out.Entries)-1].Status = CollectionStatusFailed
			out.Entries[len(out.Entries)-1].Detail = err.Error()
			out.Err = err
			break
		}
		fresh, err := ProveVersionTree(entry.Path)
		if err != nil || fresh.Manifest.AssetSetID != initial.Manifest.AssetSetID || fresh.Legacy != initial.Legacy {
			if err == nil {
				err = errors.New("candidate identity changed before quarantine")
			}
			out.Entries[len(out.Entries)-1].Status = CollectionStatusFailed
			out.Entries[len(out.Entries)-1].Detail = err.Error()
			out.Err = err
			break
		}
		journal := collectionJournal{FormatVersion: collectionJournalFormatVersion, OriginalAssetSetID: fresh.Manifest.AssetSetID,
			Manifest: fresh.Manifest, SourcePath: entry.Path, QuarantinePath: options.Roots.CollectionQuarantineDir(),
			Legacy: fresh.Legacy, Phase: collectionPhasePrepared}
		if err = writeCollectionJournal(options.FS, options.Roots, &journal); err == nil {
			err = options.FS.Rename(journal.SourcePath, journal.QuarantinePath)
			if err != nil {
				err = pending("quarantining verified source", err)
			}
		}
		if err == nil {
			journal.Phase = collectionPhaseQuarantined
			err = writeCollectionJournal(options.FS, options.Roots, &journal)
			if err != nil {
				err = pending("publishing quarantined phase", err)
			}
		}
		if err == nil {
			err = reconcileCollectionJournal(options.FS, options.Roots)
		}
		if err != nil {
			out.Entries[len(out.Entries)-1].Status = CollectionStatusFailed
			out.Entries[len(out.Entries)-1].Detail = err.Error()
			out.Pending = []string{options.Roots.CollectionJournalPath()}
			out.Err = err
			break
		}
		out.Applied = true
	}
	return sortedCollection(out)
}

type uninstallPlan struct {
	prior     *State
	desired   *State
	removals  []TargetRecord
	reported  []TargetRecord
	conflicts []Action
	settled   bool
}

func validateUninstallOptions(options UninstallOptions) (map[string]bool, error) {
	if options.FS == nil {
		return nil, fmt.Errorf("%w: uninstall requires filesystem operations", ErrInvalidInput)
	}
	if options.Roots.Home == "" || !filepath.IsAbs(options.Roots.Home) || options.Roots.DataRoot == "" || !filepath.IsAbs(options.Roots.DataRoot) {
		return nil, fmt.Errorf("%w: roots carry no absolute home and data root", ErrInvalidInput)
	}
	known := make(map[string]bool, len(options.SupportedHarnesses))
	for _, name := range options.SupportedHarnesses {
		if name == "" || known[name] {
			return nil, fmt.Errorf("%w: invalid supported harness %q", ErrInvalidInput, name)
		}
		known[name] = true
	}
	selected := make(map[string]bool, len(options.Harnesses))
	for _, name := range options.Harnesses {
		if !known[name] {
			return nil, fmt.Errorf("%w: unknown harness %q", ErrInvalidInput, name)
		}
		selected[name] = true
	}
	return selected, nil
}

func planUninstall(roots UserRoots, explicit map[string]bool) (uninstallPlan, error) {
	prior, err := LoadState(roots.StatePath())
	if err != nil {
		return uninstallPlan{}, err
	}
	if prior == nil {
		return uninstallPlan{settled: true}, nil
	}
	all := len(explicit) == 0
	selected := func(name string) bool { return name != "" && (all || explicit[name]) }

	desired := *prior
	desired.Harnesses = make([]string, 0, len(prior.Harnesses))
	desired.Targets = make([]TargetRecord, 0, len(prior.Targets))
	for _, name := range prior.Harnesses {
		if !selected(name) {
			desired.Harnesses = append(desired.Harnesses, name)
		}
	}
	plan := uninstallPlan{prior: prior, desired: &desired}
	for _, rec := range prior.Targets {
		if !selected(rec.Harness) {
			desired.Targets = append(desired.Targets, rec)
			continue
		}
		satisfied, needsRemoval, conflictReason, err := proveUninstallRemoval(rec)
		if err != nil {
			return uninstallPlan{}, err
		}
		if !satisfied {
			detail := conflictReason
			if detail == "" {
				detail = ReasonOwnershipConflict
			}
			plan.conflicts = append(plan.conflicts, Action{Op: OpConflict, Path: rec.Path, Detail: detail})
			continue
		}
		plan.reported = append(plan.reported, rec)
		if needsRemoval {
			plan.removals = append(plan.removals, rec)
		}
	}
	if len(desired.Harnesses) == 0 {
		desired.Harnesses = []string{}
		desired.AssetSetID = ""
	}
	sort.Strings(desired.Harnesses)
	plan.settled, err = stateSettled(prior, &desired)
	return plan, err
}

func proveUninstallRemoval(rec TargetRecord) (bool, bool, string, error) {
	if err := validateTarget(rec); err != nil {
		return false, false, "", fmt.Errorf("%w: target %s: %v", ErrStateInvalid, rec.Path, err)
	}
	info, err := os.Lstat(rec.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return true, false, "", nil
	}
	if err != nil {
		return false, false, "", fmt.Errorf("install: inspecting %s: %w", rec.Path, err)
	}
	if rec.Kind == KindManagedBlock && info.Mode().IsRegular() {
		data, err := os.ReadFile(rec.Path)
		if err != nil {
			return false, false, "", fmt.Errorf("install: reading %s: %w", rec.Path, err)
		}
		doc, err := document.Parse(data)
		if err != nil {
			return false, false, ReasonManagedBlockInvalid, nil
		}
		if _, ok := doc.Block(rec.BlockName); !ok {
			return true, false, "", nil
		}
	}
	matches, err := recordMatchesDisk(rec)
	if err != nil {
		return false, false, "", err
	}
	return matches, matches, ReasonOwnershipConflict, nil
}

func uninstallPlanFailure(out Outcome, err error) Outcome {
	switch {
	case errors.Is(err, ErrStateInvalid):
		return fail(out, ReasonStateInvalid, err)
	case errors.Is(err, ErrInvalidTarget):
		return fail(out, ReasonInternal, err)
	default:
		return fail(out, ReasonFilesystemFailed, err)
	}
}

func reportUninstallPlan(out Outcome, plan uninstallPlan, applied bool) Outcome {
	if plan.prior != nil {
		out.Mode = plan.prior.Mode
		out.AssetProtocol = plan.prior.AssetProtocol
		out.AssetSetID = plan.desired.AssetSetID
		out.Harnesses = append([]string{}, plan.desired.Harnesses...)
	}
	out.Applied = applied
	if len(plan.conflicts) > 0 {
		out.Actions = append(out.Actions, plan.conflicts...)
		return fail(out, conflictActionReason(plan.conflicts), fmt.Errorf("%w: %d target(s) are not provably docket's", ErrPlanConflict, len(plan.conflicts)))
	}
	for _, rec := range plan.reported {
		out.Actions = append(out.Actions, Action{Op: OpRemove, Path: rec.Path, Detail: rec.Harness})
	}
	return out
}

func conflictActionReason(actions []Action) string {
	for _, action := range actions {
		if action.Detail == ReasonManagedBlockInvalid {
			return ReasonManagedBlockInvalid
		}
	}
	return ReasonOwnershipConflict
}
