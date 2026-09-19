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
// An empty worktreeRoot or epochID has no epoch-linked worktree launches to
// reconcile (a keyless/standalone run), so it accounts vacuously.
func (d *Driver) ReconcileEpochLaunches(worktreeRoot, epochID string) (EpochLaunchReport, error) {
	report := EpochLaunchReport{Accounted: true}
	if worktreeRoot == "" || epochID == "" {
		return report, nil
	}

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
			// A corrupt / unknown-schema / IO-unreadable record cannot be attributed to
			// an epoch, so it cannot be proven NOT to be this epoch's obligation: fail
			// closed rather than silently drop it.
			report.Accounted = false
			report.Findings = append(report.Findings, "record-unreadable:"+id)
			continue
		}
		linked, ok, _ := d.resolveDriveEpoch(rec)
		if !ok {
			// The drive's epoch linkage is LOST or unreadable (an unreadable scope, or a
			// scopeless AdmissionToken the worktree slot no longer matches): it cannot be
			// proven NOT to be this epoch's obligation, so fail closed rather than silently
			// skip it — mirroring the record-unreadable leg above and the launch paths
			// (authorizeRelaunch/recoveryEpochRevoked), which treat a lost linkage as
			// refuse/revoked.
			report.Accounted = false
			report.Findings = append(report.Findings, "linkage-unresolved:"+id)
			continue
		}
		if linked != epochID {
			continue // a clean resolution to another epoch (or epoch-less): not this epoch's obligation
		}
		settled, finding := d.reconcileEpochDrive(id, rec)
		if finding != "" {
			report.Findings = append(report.Findings, finding)
		}
		if !settled {
			report.Accounted = false
		}
	}
	return report, nil
}

// reconcileEpochDrive reconciles one epoch-linked drive's launch obligation and
// reports whether it is settled plus a bounded, credential-free finding (drive id +
// disposition token). It launches nothing and never mutates the drive verdict. A
// terminal drive is already accounted by the slot/participant teardown. A
// nonterminal drive is probed under its claimant flock: a busy claim is pending
// launch/attach work (never waited on); a free claim lets it re-read the record and
// resolve the exact reservation.
func (d *Driver) reconcileEpochDrive(id string, rec driveRecord) (settled bool, finding string) {
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
		return d.reconcileReservation(id, cur.RunRoot, cur.RelaunchToken, cur.OwnerGeneration)
	}

	// An attached run the DRIVE record names directly (the original, or a relaunch's
	// replacement the worktree slot still records the predecessor for): stop it through
	// proc and account only on proven teardown.
	return d.stopIdentifiedRun(id, cur.RawRunDir)
}

// reconcileReservation resolves a reserved (but unattached) launch through the
// process seam and reports whether the obligation is settled. A proven never-launched
// is settled by settleNeverLaunchedCancelled (which also forecloses a later recovery
// launch). An identified run is stopped through proc. An unresolved verdict or a
// resolve error preserves unresolved evidence: pending. ownerGen is the drive's own
// owner generation (read from the record under the held claim) — the CAS credential
// settleNeverLaunchedCancelled needs; it is a locator, not authority.
func (d *Driver) reconcileReservation(id, runRoot, token, ownerGen string) (bool, string) {
	res, rerr := d.proc.ResolveReservation(runRoot, token)
	if rerr != nil || res == nil {
		return false, "resolution-unresolved:" + id
	}
	switch res.Disposition {
	case "never-launched":
		return d.settleNeverLaunchedCancelled(id, ownerGen)
	case "identified":
		return d.stopIdentifiedRun(id, res.RunDir)
	default: // "unresolved" or any unexpected disposition: preserve evidence
		return false, "resolution-unresolved:" + id
	}
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
