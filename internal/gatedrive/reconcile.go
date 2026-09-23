// Cancellation's pending-launch accounting (change 0437 Task 6). Fencing an epoch
// (rungate_epoch.go, run.cancel) stops what the durable participant/slot records
// already NAME, but a launch admitted just before the fence can still be reserved,
// in-flight, or attached as a relaunch replacement the worktree slot has not caught
// up to. ReconcileEpochLaunches is the read cancellation consumes to SEE those
// obligations: it walks the drive registry, attributes each drive to its run epoch
// through the SAME linkage predicate the launch paths use (resolveDriveEpoch), and
// for each nonterminal drive of the fenced epoch proves — through the per-drive
// claimant flock and the process seam — whether its launch is settled or still
// pending. It launches nothing, mutates no drive verdict, and preserves unresolved
// evidence: it never erases a reservation that may have launched. The epoch is
// already fenced, so this path takes NO epoch lock (the lock order forbids holding
// the epoch while probing a per-drive claim).
//
// Attribution, not inheritance (change 0446 spec §4). The census accounts the
// TARGET epoch's obligations, never all history: a drive record it cannot read, or
// whose epoch linkage is lost, blocks the epoch only when a CURRENT reference names
// it — the target worktree's slot (its reservation token, or the scope it names) or
// a scope carrying this RunEpochID (its current/pending drive ids). Any other
// unreadable or unlinked record is an informational history-unattributed finding
// that does not clear Accounted. resolveDriveEpoch itself is untouched: losing a
// drive's ownership proof still refuses that drive's own launch/relaunch; only the
// census's attribution of the failure changes.
package gatedrive

import (
	"errors"
	"io/fs"
	"os"
	"sort"
)

// EpochLaunchReport is the bounded accounting of one epoch's launch obligations on
// one canonical worktree. Accounted is true only when every epoch-linked launch the
// durable records name is provably settled (never-launched, or an identified run
// proven stopped); any pending, busy, unresolved, or unreadable obligation makes it
// false. Findings carry drive ids + disposition tokens only — never a reservation
// token, argv, or env value — so a caller can surface them safely.
type EpochLaunchReport struct {
	Accounted bool
	Findings  []string
}

// ReconcileEpochLaunches reconciles, for an ALREADY-FENCED epoch, every epoch-linked
// launch obligation the durable records name: scoped drives whose scope carries
// epochID, and scopeless drives whose AdmissionToken matches a worktree slot
// recording epochID (both resolved through resolveDriveEpoch — the exact-reservation
// linkage the launch paths use, never restated here). For each nonterminal drive of
// the epoch it takes the claimant flock NONBLOCKING — a busy claim is pending work,
// never waited on — then re-reads the record and resolves the EXACT reservation
// (AdmissionToken, or RelaunchToken when RelaunchReserved): a proven never-launched
// accounts it; an identified/attached run is stopped through proc and accounts only
// on proven teardown; unresolved, a resolution/read/stop error, or a missing/corrupt
// required record keeps Accounted=false. A merely-reserved drive that never launched
// is a pending admission (a delayed StartAdmitted the fence still has to refuse), so
// it too keeps Accounted=false until that refusal settles the record terminal — a
// later replay then accounts (an obsolete slot RawRunDir is not an inventory, and a
// released slot alone never settles a launch obligation). It launches nothing and
// mutates no drive verdict, so it must never erase a reservation that may have
// launched. It takes NO epoch lock (the epoch is already fenced; the lock order
// forbids holding the epoch while probing a claim).
//
// An empty epochID has no epoch-linked launches to reconcile (a keyless/standalone
// run), so it accounts vacuously. An empty worktreeRoot is NOT proof of quiescence
// (a superseded epoch has an empty Worktree yet its scope-linked drives are still
// enumerable by RunEpochID): the registry walk still runs and only the slot-side
// references are skipped. A superseded epoch's caller supplies the replacement's
// worktree (the app's resolveTerminalEpochSlot), so the slot side is checked there.
func (d *Driver) ReconcileEpochLaunches(worktreeRoot, epochID string) (EpochLaunchReport, error) {
	return d.accountEpochLaunches(worktreeRoot, epochID, false)
}

// ObserveEpochLaunches is the SUCCESS-closeout view of one epoch's launch
// obligations (change 0441): the SAME walk, epoch linkage, and claimant probe as
// ReconcileEpochLaunches, but observation-only — it never stops a process, never
// settles a never-launched reservation terminal, and never mutates a record. Both
// exported methods share one inventory (accountEpochLaunches) rather than copying a
// second walk. A pending never-launched ticket stays pending (launch-pending) until
// the completing launch-gate refusal settles it terminal; a replay then accounts.
func (d *Driver) ObserveEpochLaunches(worktreeRoot, epochID string) (EpochLaunchReport, error) {
	return d.accountEpochLaunches(worktreeRoot, epochID, true)
}

// accountEpochLaunches is the shared body behind ReconcileEpochLaunches (observeOnly
// false: the cancellation mode that stops identified runs and settles proven
// never-launched reservations terminal) and ObserveEpochLaunches (observeOnly true:
// the success-closeout mode that stops nothing, settles nothing, and mutates no
// record). The walk, epoch linkage, and per-drive claimant probe are identical; only
// the terminal per-drive disposition differs, threaded through reconcileEpochDrive.
//
// The walk applies change 0446 spec §4's attribution rules, in order, per record:
//
//  1. Terminal before linkage. A readable drive whose outcome is terminal
//     (isTerminalOutcome: PASSED, FAILED, or HALTED) has a settled LAUNCH axis
//     whether or not its linkage still resolves. This settles only the drive
//     record's launch axis, never the execution: HALTED is not slot-release proof
//     or completion evidence, and teardown stays the slot and participant checks'
//     job (the "no blanket trust in HALTED" rule).
//  2. Supported history. A record the executable reader refuses is retried through
//     loadHistoricalDrive; a supported schema-2 record with a terminal outcome is
//     settled history.
//  3. Positive reference decides blocking. A still-unreadable record, or a readable
//     nonterminal one whose resolveDriveEpoch linkage is lost, blocks
//     (record-unreadable:/linkage-unresolved:) only when a current reference names
//     it (censusRefs.names / censusRefs.namesUnreadable); otherwise it is an
//     informational history-unattributed:<id>.
//  4. A scope named by this epoch's current slot that cannot be read keeps the
//     epoch unaccounted (censusReferences).
//  5. An empty worktreeRoot skips only the slot-side references (the caller supplies
//     the replacement worktree for those); it never accounts vacuously.
//  6. Token rotation. An older nonterminal scopeless drive whose AdmissionToken the
//     slot no longer holds resolves ok=false and, named by no current reference, is
//     historical: every launch path verifies the current token before launching
//     (StartAdmitted's verifyAdmittedSlot, authorizeRelaunch's resolveDriveEpoch,
//     and the crash-window recovery's recoveryEpochRevoked, which settles instead
//     of relaunching), so it has lost launch authority and the current token holder
//     carries any live obligation.
func (d *Driver) accountEpochLaunches(worktreeRoot, epochID string, observeOnly bool) (EpochLaunchReport, error) {
	report := EpochLaunchReport{Accounted: true}
	if epochID == "" {
		return report, nil
	}

	refs := d.censusReferences(worktreeRoot, epochID, &report)

	entries, err := os.ReadDir(d.store.root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return report, nil // no drive registry yet: nothing was ever launched
		}
		// The registry itself is unreadable: fail closed rather than claim accounted.
		report.Accounted = false
		report.Findings = append(report.Findings, "registry-unreadable")
		return report, nil
	}
	// Deterministic id order so the findings are stable across runs (mirrors
	// inventoryLegacyDrives' sorted walk).
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	// First pass: load every record, so whether the slot's current token has a
	// READABLE holder is known before any unreadable record is attributed.
	type walked struct {
		id         string
		rec        driveRecord
		unreadable bool
	}
	var walk []walked
	holderFound := false
	for _, entry := range entries {
		id := entry.Name()
		if !entry.IsDir() || validateID(id) != nil {
			continue // not a drive record directory
		}
		rec, lerr := d.store.Load(id)
		if lerr != nil {
			if storeErrIs(lerr, ErrNotFound) {
				// A record-less directory (an in-flight or crashed-mid-creation drive)
				// has launched no process and names no obligation, so it is skipped
				// exactly as the legacy inventory skips it.
				continue
			}
			// Rule 2: a supported historical (schema-2) record with a terminal outcome
			// is settled history, not an unreadable obligation.
			if h, herr := d.store.loadHistoricalDrive(id); herr == nil && isTerminalOutcome(h.LastOutcome) {
				continue
			}
			walk = append(walk, walked{id: id, unreadable: true})
			continue
		}
		if refs.slotToken != "" && rec.AdmissionToken == refs.slotToken {
			holderFound = true
		}
		walk = append(walk, walked{id: id, rec: rec})
	}

	for _, w := range walk {
		if w.unreadable {
			// Rule 3 for an unreadable record: the reference comes from the slot/scope
			// side only — never from reading the record itself.
			if refs.namesUnreadable(w.id, holderFound) {
				report.Accounted = false
				report.Findings = append(report.Findings, "record-unreadable:"+w.id)
			} else {
				report.Findings = append(report.Findings, "history-unattributed:"+w.id)
			}
			continue
		}
		// Rule 1: terminal before linkage. The launch axis of a terminal drive is
		// settled; its teardown is the slot/participant checks' to prove.
		if isTerminalOutcome(w.rec.LastOutcome) {
			continue
		}
		linked, ok, _ := d.resolveDriveEpoch(w.rec)
		if !ok {
			// The drive's epoch linkage is LOST or unreadable (an unreadable scope, or a
			// scopeless AdmissionToken the worktree slot no longer matches). Rule 3: it
			// fails closed only when a current reference names it; an unlinked record
			// nothing current names is history (rule 6 covers a rotated scopeless token).
			if refs.names(w.id, w.rec) {
				report.Accounted = false
				report.Findings = append(report.Findings, "linkage-unresolved:"+w.id)
			} else {
				report.Findings = append(report.Findings, "history-unattributed:"+w.id)
			}
			continue
		}
		if linked != epochID {
			continue // a clean resolution to another epoch (or epoch-less): not this epoch's obligation
		}
		settled, finding := d.reconcileEpochDrive(w.id, w.rec, observeOnly)
		if finding != "" {
			report.Findings = append(report.Findings, finding)
		}
		if !settled {
			report.Accounted = false
		}
	}
	return report, nil
}

// censusRefs is the set of CURRENT references to the target epoch's drives, built
// from existing records only (no reverse index): the drive ids scopes carrying the
// epoch name as current or pending, and the target worktree slot's reservation token
// when that slot names the epoch.
type censusRefs struct {
	ids map[string]bool
	// slotToken is the target slot's ReservationToken when the slot's RunEpochID is
	// the target epoch ("" otherwise, or when no worktree was supplied).
	slotToken string
	// slotOccupied reports that the target epoch's slot still holds an unreleased
	// scoped/scopeless reservation, whose token some drive record must carry.
	slotOccupied bool
}

// names reports whether a current reference names a READABLE drive: its id is a
// scope's current/pending drive, or it carries the epoch slot's current token.
func (r censusRefs) names(id string, rec driveRecord) bool {
	if r.ids[id] {
		return true
	}
	return r.slotToken != "" && rec.AdmissionToken == r.slotToken
}

// namesUnreadable reports whether a current reference names an UNREADABLE drive
// without reading it: a scope names its id, or the epoch's occupied slot carries a
// token no readable drive holds (holderFound false), so every unreadable record is a
// candidate holder of that current reservation. A released slot is not inferred from:
// release is proof its latest execution was vacated, and an orphan token (a start
// whose reserved record was removed) is not an obligation.
func (r censusRefs) namesUnreadable(id string, holderFound bool) bool {
	if r.ids[id] {
		return true
	}
	return r.slotOccupied && !holderFound
}

// censusReferences builds the census's current-reference set for epochID and
// records the fail-closed findings for a required reference it cannot read: an
// unreadable scope registry (the epoch's scopes cannot be enumerated), an unreadable
// target slot, or an unreadable scope the epoch's slot names (rule 4). An unreadable
// scope nothing current names is skipped: it establishes no reference, and any
// nonterminal drive under it surfaces as history-unattributed in the walk.
func (d *Driver) censusReferences(worktreeRoot, epochID string, report *EpochLaunchReport) censusRefs {
	refs := censusRefs{ids: map[string]bool{}}
	addScope := func(s scopeRecord) {
		if s.CurrentDriveID != "" {
			refs.ids[s.CurrentDriveID] = true
		}
		if s.PendingAckDriveID != "" {
			refs.ids[s.PendingAckDriveID] = true
		}
	}

	scopes, err := os.ReadDir(d.store.scopeRoot)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		report.Accounted = false
		report.Findings = append(report.Findings, "scope-registry-unreadable")
	}
	for _, entry := range scopes {
		if !entry.IsDir() || validateID(entry.Name()) != nil {
			continue
		}
		s, lerr := d.store.LoadScope(entry.Name())
		if lerr != nil {
			continue
		}
		if s.RunEpochID == epochID {
			addScope(s)
		}
	}

	if worktreeRoot == "" {
		return refs // rule 5: the slot-side references are the caller's to supply
	}
	slot, _, serr := d.store.LoadWorktreeExecution(worktreeRoot)
	if serr != nil {
		if storeErrIs(serr, ErrNotFound) {
			return refs // no slot: nothing current is named from the slot side
		}
		// The target worktree's authoritative slot cannot be read: its references are
		// unknown, so fail closed rather than infer the slot is empty.
		report.Accounted = false
		report.Findings = append(report.Findings, "slot-unreadable")
		return refs
	}
	if slot.RunEpochID != epochID {
		return refs // the slot names another epoch (or none): its occupant is not this epoch's
	}
	refs.slotToken = slot.ReservationToken
	refs.slotOccupied = slot.ReservationToken != "" && slot.State != admissionReleased && slot.Kind != "raw"
	if slot.ScopeID != "" {
		s, lerr := d.store.LoadScope(slot.ScopeID)
		if lerr != nil {
			// Rule 4: a scope this epoch's current slot names cannot be read, so its
			// current/pending drives cannot be followed. Fail closed.
			report.Accounted = false
			report.Findings = append(report.Findings, "scope-unreadable:"+slot.ScopeID)
		} else {
			addScope(s)
		}
	}
	return refs
}

// reconcileEpochDrive accounts one epoch-linked drive's launch obligation and
// reports whether it is settled plus a bounded, credential-free finding (drive id +
// disposition token). It launches nothing and never mutates the drive verdict when
// observeOnly is set. A terminal drive is already accounted by the slot/participant
// teardown. A nonterminal drive is probed under its claimant flock: a busy claim is
// pending launch/attach work (never waited on); a free claim lets it re-read the
// record and resolve the exact reservation. observeOnly selects the per-drive
// disposition for an identified/attached run and a proven never-launched reservation:
// cancellation stops / settles them, closeout only observes (never-launched stays
// pending as launch-pending, an attached run is observed, not stopped).
func (d *Driver) reconcileEpochDrive(id string, rec driveRecord, observeOnly bool) (settled bool, finding string) {
	if isTerminalOutcome(rec.LastOutcome) {
		return true, "" // teardown accounted by the slot/participant reconciliation
	}
	claim, busy, cerr := d.store.tryRelaunchClaim(id)
	if cerr != nil {
		return false, "resolution-unresolved:" + id
	}
	if busy {
		// A held claim is a launch still in flight (StartAdmitted across launch/attach,
		// or a relaunch reservation resolving): pending work, never proof of a crash and
		// never waited on.
		return false, "claim-busy:" + id
	}
	defer claim.close()

	// Re-read under the held claim: the exact durable state may have advanced between
	// the walk's read and the claim acquisition.
	cur, lerr := d.store.Load(id)
	if lerr != nil {
		return false, "record-unreadable:" + id
	}
	if isTerminalOutcome(cur.LastOutcome) {
		return true, "" // a concurrent settle (the fence's StartAdmitted refusal) accounts it
	}

	// A drive that never launched and reserved no relaunch is a pending admission: the
	// ticket's delayed StartAdmitted is still outstanding. The fence refuses it — which
	// settles the record terminal — but reconcile must report the obligation pending
	// until then (a released slot alone never settles it). A later replay accounts.
	if cur.RawRunDir == "" && !cur.RelaunchReserved {
		return false, "launch-pending:" + id
	}

	// A reserved-but-unattached relaunch (the crash window recoverReservedRelaunch
	// resolves): the replacement may or may not have launched. Resolve the RELAUNCH
	// token — never the admission token, which names the original run.
	if cur.RelaunchReserved {
		if cur.RelaunchToken == "" {
			return false, "resolution-unresolved:" + id
		}
		return d.reconcileReservation(id, cur.RunRoot, cur.RelaunchToken, cur.OwnerGeneration, observeOnly)
	}

	// An attached run the DRIVE record names directly (the original, or a relaunch's
	// replacement the worktree slot still records the predecessor for): cancellation
	// stops it and accounts only on proven teardown; closeout only OBSERVES it.
	if observeOnly {
		return d.observeIdentifiedRun(id, cur.RawRunDir)
	}
	return d.stopIdentifiedRun(id, cur.RawRunDir)
}

// reconcileReservation resolves a reserved (but unattached) launch through the
// process seam and reports whether the obligation is settled. In cancellation mode a
// proven never-launched is settled by settleNeverLaunchedCancelled (which also
// forecloses a later recovery launch) and an identified run is stopped through proc;
// in observeOnly mode a never-launched stays pending (launch-pending — settled later
// by the completing launch-gate refusal, never here) and an identified run is only
// observed. An unresolved verdict or a resolve error preserves unresolved evidence:
// pending. ownerGen is the drive's own owner generation (read from the record under
// the held claim) — the CAS credential settleNeverLaunchedCancelled needs; it is a
// locator, not authority.
func (d *Driver) reconcileReservation(id, runRoot, token, ownerGen string, observeOnly bool) (bool, string) {
	res, rerr := d.proc.ResolveReservation(runRoot, token)
	if rerr != nil || res == nil {
		return false, "resolution-unresolved:" + id
	}
	switch res.Disposition {
	case "never-launched":
		if observeOnly {
			// Closeout must not foreclose the ticket terminal: it stays pending until
			// the completing launch-gate refusal settles it, and a replay accounts.
			return false, "launch-pending:" + id
		}
		return d.settleNeverLaunchedCancelled(id, ownerGen)
	case "identified":
		if observeOnly {
			return d.observeIdentifiedRun(id, res.RunDir)
		}
		return d.stopIdentifiedRun(id, res.RunDir)
	default: // "unresolved" or any unexpected disposition: preserve evidence
		return false, "resolution-unresolved:" + id
	}
}

// observeIdentifiedRun is the observation-only counterpart of stopIdentifiedRun: it
// reads one identified run's state through the process seam and reports whether
// teardown is PROVEN, using the same proof rule (stopProvesTeardown) the driver's own
// stop legs use — but it issues NO stop and mutates no record. An empty run dir or an
// observation error preserves unresolved evidence (resolution-unresolved); a live or
// signalled run stays pending (run-live); a proven-terminal run accounts the
// obligation with an informational run-terminal finding.
func (d *Driver) observeIdentifiedRun(id, runDir string) (bool, string) {
	if runDir == "" {
		return false, "resolution-unresolved:" + id
	}
	observation, err := d.proc.Observe(runDir)
	if err != nil || observation == nil {
		return false, "resolution-unresolved:" + id
	}
	if stopProvesTeardown(observation.State) {
		return true, "run-terminal:" + id
	}
	return false, "run-live:" + id
}

// settleNeverLaunchedCancelled settles a reserved relaunch that provably never ran,
// under the per-drive claim reconcileEpochDrive already holds. That held claim
// excludes any concurrent launcher (Advance's recoverReservedRelaunch, StartAdmitted,
// and reserveRelaunch all take the SAME flock), so the reservation is genuinely idle
// AND cannot be launched while the claim is held. Settling the drive terminal HALTED
// "run-cancelled" here — BEFORE the caller releases the claim — closes the
// launch-after-cancel window (spec AC4): a later Advance recovery whose read-only
// epoch pass raced ahead of this fence (recoveryEpochRevoked read the epoch still
// live) now finds a terminal record at isTerminalOutcome and returns the recorded
// verdict rather than authorizing a new launch. The CAS preserves the consumed
// reservation (RelaunchReserved is never cleared, mirroring haltReservedRelaunchCause)
// so the sole relaunch is never refunded.
//
// An already-terminal record (a concurrent settle) is equally accounted. A record
// that moved out from under the claim (a lost owner or reservation) or a store fault
// fails closed to pending — reconcile never claims an obligation settled while the
// drive might still recover a launch.
func (d *Driver) settleNeverLaunchedCancelled(id, ownerGen string) (bool, string) {
	err := d.store.ownerCAS(id, func(r *driveRecord) error {
		if verr := verifyOwner(r, ownerGen); verr != nil {
			return verr
		}
		if isTerminalOutcome(r.LastOutcome) {
			return errAlreadyTerminal
		}
		if !r.RelaunchReserved {
			return errRelaunchRaceLost
		}
		r.LastOutcome = HALTED
		r.LastCause = "run-cancelled"
		return nil
	})
	if err == nil || errors.Is(err, errAlreadyTerminal) {
		return true, "" // settled terminal: provably idle AND foreclosed from relaunch
	}
	return false, "resolution-unresolved:" + id
}

// stopIdentifiedRun stops one identified run through the process seam and reports
// whether teardown is PROVEN, using the same proof rule the driver's own stop legs
// use (a performed stop, or a state that proves the group is gone). An empty run dir,
// a stop error, or an unproven stop preserves unresolved evidence: pending. A proven
// stop accounts the obligation (an informational replacement-stopped finding records
// it). It never mutates the drive record — reconcile stops the process but never
// erases a reservation that may have launched.
func (d *Driver) stopIdentifiedRun(id, runDir string) (bool, string) {
	if runDir == "" {
		return false, "resolution-unresolved:" + id
	}
	out, serr := d.proc.Stop(runDir, "gatedrive: reconciling a cancelled epoch's pending launch")
	if serr != nil || out == nil {
		return false, "resolution-unresolved:" + id
	}
	if out.Performed || stopProvesTeardown(out.State) {
		return true, "replacement-stopped:" + id
	}
	return false, "stop-unproven:" + id
}
