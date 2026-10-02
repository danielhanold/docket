// The run launch census (change 0437 Task 6; attribution by run context, change
// 0490). Fencing a run (runtracker_run_record.go, run.cancel) stops what the run's
// own records name, but the run's gates live in the drive registry: a drive
// started inside a dispatched run stores the hash of that run's child context
// (driveRecord.RunContextHash), equal to the run-tracker record's
// child_context_hash. The census has two modes (change 0491). Cancel mode
// (ReconcileRunLaunches, run.cancel) stops a running supervisor and settles a
// proven never-launched launch HALTED "run-cancelled". Verdict mode
// (VerdictRunLaunches, the keyed run.verdict's closeout) stops nothing and signals
// nothing; it settles only a proven never-launched FIRST launch, HALTED
// "launch-abandoned", and leaves a live supervisor run-live and a reserved
// relaunch launch-pending.
//
// Attribution. The census walks the drive registry and accounts every drive whose
// RunContextHash equals the run's context hash — every such drive, not only the
// one a worktree lock holder note happens to name. A drive without a run context is never
// attributed (raw launches never were). An unreadable record is informational
// (history-unattributed), because nothing positively names it; a supported
// schema-2 record with a terminal outcome is settled history. The census reads no
// worktree lock or holder note, and no recovery scope (change 0489).
//
// Teardown proof is the lock model's proof: the supervisor is gone. For each run
// dir a drive records (RawRunDir, PriorRawRunDir) a dir that no longer exists is
// clean absence; any observed state other than running counts as torn down; a
// running supervisor is stopped (cancel mode) and must then observe as not
// running, or is reported run-live (verdict mode). A probe or stop error is never
// clean absence: it keeps the run pending.
// PASSED/FAILED drives are settled by their verdict (the supervisor wrote it before
// exiting); a HALTED drive can still have a live supervisor, so its run dirs are
// proven too — or, for a first launch that failed before attaching one, its
// launch token is resolved (reconcileHaltedDrive). A nonterminal first launch
// whose run root does not exist was never launched (reconcileFirstLaunch). A
// nonterminal drive is probed under its per-drive claimant flock (nonblocking: a
// busy claim is pending work, never waited on). The census launches nothing and
// takes NO run lock (the run is already fenced; the lock order forbids holding the
// run while probing a per-drive claim).
package gatedrive

import (
	"errors"
	"io/fs"
	"os"
	"sort"

	"github.com/danielhanold/docket/internal/process"
)

// censusMode selects what the run launch census may do (change 0491).
type censusMode int

const (
	// censusCancel is run.cancel's census: it stops a running supervisor and settles a
	// proven never-launched launch HALTED run-cancelled.
	censusCancel censusMode = iota
	// censusVerdict is the keyed run.verdict's census: it stops nothing and signals
	// nothing. It settles only a proven never-launched FIRST launch, HALTED
	// launch-abandoned; a running supervisor is run-live, and a reserved relaunch
	// stays launch-pending (change 0493 retires finalize's relaunch).
	censusVerdict
)

// The HALT causes the census writes on a proven never-launched launch.
const (
	causeRunCancelled    = "run-cancelled"
	causeLaunchAbandoned = "launch-abandoned"
)

// firstLaunchCause is the HALT cause a proven never-launched first launch settles to.
func (m censusMode) firstLaunchCause() string {
	if m == censusVerdict {
		return causeLaunchAbandoned
	}
	return causeRunCancelled
}

// RunLaunchReport is the bounded accounting of one run's launch obligations.
// Accounted is true only when every drive attributed to the run is provably
// settled (a PASSED/FAILED verdict, a never-launched launch, or every recorded
// supervisor proven gone); any pending, busy, unresolved, or live obligation makes
// it false. Findings carry drive ids + disposition tokens only — never a
// reservation token, argv, or env value — so a caller can surface them safely.
type RunLaunchReport struct {
	Accounted bool
	Findings  []string
}

// ReconcileRunLaunches reconciles, for an ALREADY-FENCED run, every drive whose
// RunContextHash equals runContextHash (cancellation mode): a running supervisor
// is stopped and must then observe as not running; a first launch or reserved
// relaunch that never attached is resolved through its exact reservation token,
// and a proven never-launched one is settled HALTED "run-cancelled" under the held
// claim so no later launch can follow the cancel. An empty runContextHash names no
// drive and accounts vacuously.
func (d *Driver) ReconcileRunLaunches(runContextHash string) (RunLaunchReport, error) {
	return d.accountRunLaunches(runContextHash, censusCancel)
}

// VerdictRunLaunches is the keyed run.verdict's view of one run's launch
// obligations (change 0491; it replaces change 0441's observe-only twin, whose
// only caller was the closeout): the same walk, attribution, and claimant probe as
// ReconcileRunLaunches, but it never stops or signals a process. A proven
// never-launched FIRST launch is settled HALTED "launch-abandoned" under the held
// claim, so a successful run is not stranded by a launcher killed between Admit
// and StartAdmitted; a live supervisor is run-live and a reserved relaunch stays
// launch-pending, and both keep the report unaccounted.
func (d *Driver) VerdictRunLaunches(runContextHash string) (RunLaunchReport, error) {
	return d.accountRunLaunches(runContextHash, censusVerdict)
}

// accountRunLaunches walks the drive registry in id order and accounts every
// drive whose RunContextHash equals runContextHash — the hash run.start minted
// for the run and every tracked gate.drive.start stores. A drive without a run
// context is never attributed; an unreadable record is informational
// (history-unattributed), because nothing positively names it.
func (d *Driver) accountRunLaunches(runContextHash string, mode censusMode) (RunLaunchReport, error) {
	report := RunLaunchReport{Accounted: true}
	if runContextHash == "" {
		return report, nil // no run context: no drive can be attributed to the run
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
	// Deterministic id order so the findings are stable across runs.
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
				// has launched no process and names no obligation.
				continue
			}
			if h, herr := d.store.loadHistoricalDrive(id); herr == nil && isTerminalOutcome(h.LastOutcome) {
				continue // settled schema-2 history
			}
			report.Findings = append(report.Findings, "history-unattributed:"+id)
			continue
		}
		if rec.RunContextHash != runContextHash {
			continue // another run's drive, or one started outside any run
		}
		settled, finding := d.reconcileRunDrive(id, rec, mode)
		if finding != "" {
			report.Findings = append(report.Findings, finding)
		}
		if !settled {
			report.Accounted = false
		}
	}
	return report, nil
}

// reconcileRunDrive accounts one attributed drive and reports whether it is
// settled plus a bounded, credential-free finding (drive id + disposition token).
//   - PASSED/FAILED: settled by the verdict the supervisor wrote before exiting.
//   - HALTED: launches nothing more, so no claim is taken; its supervisors must
//     be proven gone (reconcileHaltedDrive) — through its recorded run dirs, or,
//     for a first launch that never attached one, through its launch token.
//   - Nonterminal: the claimant flock is tried nonblocking (busy → claim-busy), the
//     record re-read under it, and then a reserved relaunch resolves its RELAUNCH
//     token, a first launch that never attached resolves its launch (admission)
//     token, and an attached drive proves its run dirs gone.
func (d *Driver) reconcileRunDrive(id string, rec driveRecord, mode censusMode) (settled bool, finding string) {
	switch rec.LastOutcome {
	case PASSED, FAILED:
		return true, ""
	case HALTED:
		return d.reconcileHaltedDrive(id, rec, mode)
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

	// Re-read under the held claim: the durable state may have advanced between the
	// walk's read and the claim acquisition.
	cur, lerr := d.store.Load(id)
	if lerr != nil {
		return false, "record-unreadable:" + id
	}
	switch cur.LastOutcome {
	case PASSED, FAILED:
		return true, ""
	case HALTED:
		return d.reconcileHaltedDrive(id, cur, mode)
	}

	// A reserved-but-unattached relaunch (the crash window recoverReservedRelaunch
	// resolves): resolve the RELAUNCH token — never the admission token, which names
	// the original launch.
	if cur.RelaunchReserved {
		if cur.RelaunchToken == "" {
			return false, "resolution-unresolved:" + id
		}
		return d.reconcileReservation(id, cur.RunRoot, cur.RelaunchToken, cur.OwnerGeneration, mode)
	}

	// A first launch whose run dir was never attached: the launch was handed the
	// drive's admission token, so resolve exactly that reservation.
	if cur.RawRunDir == "" {
		if cur.AdmissionToken == "" {
			return false, "resolution-unresolved:" + id
		}
		return d.reconcileFirstLaunch(id, cur, mode)
	}

	return d.proveRunDirsGone(id, cur, mode)
}

// reconcileHaltedDrive accounts a HALTED drive. A HALTED label proves nothing
// about the supervisor: a first launch whose process.Launch returned an error is
// written HALTED "launch-failed" with no run dir (launchScopeless), yet the launch
// may have spawned a supervisor before its response was lost. Such a drive — no
// attached run dir, a launch token on record — resolves that exact token
// (resolveHaltedFirstLaunch); every other HALTED drive proves its recorded run
// dirs gone.
func (d *Driver) reconcileHaltedDrive(id string, rec driveRecord, mode censusMode) (bool, string) {
	if rec.RawRunDir == "" && rec.AdmissionToken != "" {
		return d.resolveHaltedFirstLaunch(id, rec, mode)
	}
	return d.proveRunDirsGone(id, rec, mode)
}

// resolveHaltedFirstLaunch resolves a HALTED first launch's launch token. An
// identified run must prove its supervisor gone (stopped in cancellation mode,
// run-live in verdict mode while it runs). A proven never-launched launch is
// settled in both modes and the record is left as is: the drive is already
// terminal, so no later launch can follow (unlike reconcileFirstLaunch, nothing
// needs foreclosing). A run root that no longer exists holds no run dir, so it is
// clean absence, as supervisorGone treats a removed run dir. An unresolved verdict,
// a resolve error, or any other probe error keeps the run pending.
func (d *Driver) resolveHaltedFirstLaunch(id string, rec driveRecord, mode censusMode) (bool, string) {
	if rec.RunRoot != "" {
		if _, err := os.Lstat(rec.RunRoot); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return true, ""
			}
			return false, "resolution-unresolved:" + id
		}
	}
	res, rerr := d.proc.ResolveReservation(rec.RunRoot, rec.AdmissionToken)
	if rerr != nil || res == nil {
		return false, "resolution-unresolved:" + id
	}
	switch res.Disposition {
	case "never-launched":
		return true, ""
	case "identified":
		return d.supervisorGone(id, res.RunDir, mode)
	default:
		return false, "resolution-unresolved:" + id
	}
}

// reconcileReservation resolves a reserved (but unattached) relaunch through the
// process seam. In cancellation mode a proven never-launched is settled by
// settleNeverLaunchedCancelled (which also forecloses a later recovery launch); in
// verdict mode it stays pending (launch-pending; change 0493 retires the
// relaunch). An identified run must prove its supervisor gone (supervisorGone). An
// unresolved verdict or a resolve error preserves unresolved evidence: pending. ownerGen is the drive's own owner
// generation (read under the held claim) — the CAS credential the settle needs; it
// is a locator, not authority.
func (d *Driver) reconcileReservation(id, runRoot, token, ownerGen string, mode censusMode) (bool, string) {
	res, rerr := d.proc.ResolveReservation(runRoot, token)
	if rerr != nil || res == nil {
		return false, "resolution-unresolved:" + id
	}
	switch res.Disposition {
	case "never-launched":
		if mode == censusVerdict {
			return false, "launch-pending:" + id
		}
		return d.settleNeverLaunchedCancelled(id, ownerGen)
	case "identified":
		return d.supervisorGone(id, res.RunDir, mode)
	default: // "unresolved" or any unexpected disposition: preserve evidence
		return false, "resolution-unresolved:" + id
	}
}

// reconcileFirstLaunch resolves a first launch whose run dir was never attached the
// way reconcileReservation resolves a reserved relaunch, through the drive's
// admission token (the token StartAdmitted hands the launch). A run root that does
// not exist was never created by a launch (or was cleaned), so it holds no
// reservation: clean absence, settled like a proven never-launched launch, as
// resolveHaltedFirstLaunch and supervisorGone already treat it (change 0491). Any
// other Lstat error is unknown, never absence, and stays pending. A proven
// never-launched first launch is settled HALTED with the mode's cause
// (run-cancelled / launch-abandoned); an identified run must prove its supervisor
// gone; anything else is pending.
func (d *Driver) reconcileFirstLaunch(id string, cur driveRecord, mode censusMode) (bool, string) {
	if cur.RunRoot != "" {
		if _, err := os.Lstat(cur.RunRoot); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return d.settleNeverLaunchedFirstLaunch(id, cur.OwnerGeneration, mode.firstLaunchCause())
			}
			return false, "resolution-unresolved:" + id
		}
	}
	res, rerr := d.proc.ResolveReservation(cur.RunRoot, cur.AdmissionToken)
	if rerr != nil || res == nil {
		return false, "resolution-unresolved:" + id
	}
	switch res.Disposition {
	case "never-launched":
		return d.settleNeverLaunchedFirstLaunch(id, cur.OwnerGeneration, mode.firstLaunchCause())
	case "identified":
		return d.supervisorGone(id, res.RunDir, mode)
	default:
		return false, "resolution-unresolved:" + id
	}
}

// settleNeverLaunchedCancelled settles a reserved relaunch that provably never ran,
// under the per-drive claim reconcileRunDrive already holds. That held claim
// excludes any concurrent launcher (Advance's recoverReservedRelaunch, StartAdmitted,
// and reserveRelaunch all take the SAME flock), so the reservation is genuinely idle
// AND cannot be launched while the claim is held. Settling the drive terminal HALTED
// "run-cancelled" here — BEFORE the caller releases the claim — closes the
// launch-after-cancel window (spec AC4): a later Advance recovery — which checks
// no run, since no gate start or relaunch checks one (change 0491) — finds a
// terminal record at isTerminalOutcome and returns the recorded verdict rather
// than launching the replacement. The CAS preserves the consumed
// reservation (RelaunchReserved is never cleared, mirroring haltReservedRelaunchCause)
// so the sole relaunch is never refunded.
//
// An already-terminal record (a concurrent settle) is equally accounted. A record
// that moved out from under the claim (a lost owner or reservation) or a store fault
// fails closed to pending — reconcile never claims an obligation settled while the
// drive might still recover a launch.
func (d *Driver) settleNeverLaunchedCancelled(id, ownerGen string) (bool, string) {
	return d.settleNeverLaunched(id, ownerGen, causeRunCancelled, func(r *driveRecord) bool { return r.RelaunchReserved })
}

// settleNeverLaunchedFirstLaunch settles a first launch that provably never ran,
// under the held per-drive claim, HALTED with the census mode's cause —
// "run-cancelled" in cancel mode, "launch-abandoned" in verdict mode (change
// 0491) — the first-launch counterpart of settleNeverLaunchedCancelled. Its CAS
// guard is that the drive still has no attached run dir and reserved no relaunch;
// a record that moved on (errRelaunchRaceLost) or a store fault stays pending, and
// an already-terminal record is accounted. A delayed StartAdmitted then re-reads a terminal record
// under the claim and refuses rather than launching.
func (d *Driver) settleNeverLaunchedFirstLaunch(id, ownerGen, cause string) (bool, string) {
	return d.settleNeverLaunched(id, ownerGen, cause, func(r *driveRecord) bool {
		return r.RawRunDir == "" && !r.RelaunchReserved
	})
}

// settleNeverLaunched is the shared CAS behind every never-launched settle: it
// writes HALTED cause only while the owner generation still matches and
// stillUnlaunched holds, accounting an already-terminal record and keeping any
// other outcome pending (resolution-unresolved).
func (d *Driver) settleNeverLaunched(id, ownerGen, cause string, stillUnlaunched func(*driveRecord) bool) (bool, string) {
	err := d.store.ownerCAS(id, func(r *driveRecord) error {
		if verr := verifyOwner(r, ownerGen); verr != nil {
			return verr
		}
		if isTerminalOutcome(r.LastOutcome) {
			return errAlreadyTerminal
		}
		if !stillUnlaunched(r) {
			return errRelaunchRaceLost
		}
		r.LastOutcome = HALTED
		r.LastCause = cause
		return nil
	})
	if err == nil || errors.Is(err, errAlreadyTerminal) {
		return true, "" // settled terminal: provably idle AND foreclosed from launch
	}
	return false, "resolution-unresolved:" + id
}

// proveRunDirsGone applies the lock model's teardown proof to each of a drive's
// recorded run dirs (RawRunDir, then PriorRawRunDir): the drive is torn down when
// every recorded supervisor is gone. The first dir that cannot be proven gone
// decides a pending result; otherwise the finding is the strongest one seen
// (replacement-stopped over run-terminal), so a stop this census performed is
// always reported.
func (d *Driver) proveRunDirsGone(id string, rec driveRecord, mode censusMode) (bool, string) {
	finding := ""
	for _, dir := range []string{rec.RawRunDir, rec.PriorRawRunDir} {
		if dir == "" {
			continue
		}
		gone, f := d.supervisorGone(id, dir, mode)
		if !gone {
			return false, f
		}
		if f != "" && finding != "replacement-stopped:"+id {
			finding = f
		}
	}
	return true, finding
}

// supervisorGone proves one run dir's supervisor gone. A run dir that does not
// exist is clean absence (its run root was removed after the terminal): torn down,
// with no Observe call. An observed exit (supervisorExited: passed, failed,
// signaled, stopped, vanished) counts as torn down. A running supervisor is
// stopped in cancellation mode and must then observe as not running; verdict mode
// reports it run-live without stopping. An empty run dir, an Lstat error other
// than not-exist, or an observe/stop error is unprovable and keeps the run pending
// — a probe error is never clean absence.
func (d *Driver) supervisorGone(id, runDir string, mode censusMode) (bool, string) {
	if runDir == "" {
		return false, "resolution-unresolved:" + id
	}
	if _, err := os.Lstat(runDir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return true, ""
		}
		return false, "resolution-unresolved:" + id
	}
	o, err := d.proc.Observe(runDir)
	if err != nil || o == nil {
		return false, "resolution-unresolved:" + id
	}
	switch {
	case o.State.SupervisorExited():
		return true, "run-terminal:" + id
	case o.State != process.StateRunning:
		return false, "resolution-unresolved:" + id // an unknown state proves nothing
	}
	if mode == censusVerdict {
		return false, "run-live:" + id
	}
	if _, serr := d.proc.Stop(runDir, "gatedrive: run.cancel stopping a cancelled run's gate"); serr != nil {
		return false, "resolution-unresolved:" + id
	}
	o, err = d.proc.Observe(runDir)
	if err != nil || o == nil {
		return false, "resolution-unresolved:" + id
	}
	switch {
	case o.State.SupervisorExited():
		return true, "replacement-stopped:" + id
	case o.State == process.StateRunning:
		return false, "run-live:" + id
	default:
		return false, "resolution-unresolved:" + id
	}
}
