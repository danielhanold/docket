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
// "launch-abandoned", and leaves a live supervisor run-live.
//
// Attribution. The census walks the drive registry and accounts every drive whose
// RunContextHash equals the run's context hash — every such drive, not only the
// one a worktree lock holder note happens to name. A drive without a run context is never
// attributed (raw launches never were). An unreadable record is informational
// (history-unattributed), because nothing positively names it; a supported
// schema-2 record with a terminal outcome is settled history. The census reads no
// worktree lock or holder note, and no recovery scope (change 0489).
//
// Teardown proof is the lock model's proof: the supervisor is gone. For the one
// run dir a drive records (RawRunDir; a drive never relaunches since change 0493)
// a dir that no longer exists is clean absence; any observed state other than
// running counts as torn down; a running supervisor is stopped (cancel mode) and
// must then observe as not running, or is reported run-live (verdict mode). A
// probe or stop error is never clean absence: it keeps the run pending.
// A dead supervisor's suite can outlive it (change 0492): when the supervisor's
// recorded process group still has members, the drive still counts as torn down,
// and its finding is tree-survives:<drive>:<pgid> instead of run-terminal. The
// census reports that and never signals the group — with the supervisor dead, no
// lock proves the group is the run's own. An unclear or failed leftover probe
// keeps run-terminal, exactly as before.
// PASSED/FAILED drives are settled by their verdict (the supervisor wrote it before
// exiting); a HALTED drive can still have a live supervisor, so its run dir is
// proven too — or, for a first launch that failed before attaching one, its
// launch token is resolved (reconcileHaltedDrive). A nonterminal first launch
// whose run root does not exist was never launched (reconcileFirstLaunch). A
// nonterminal drive is probed under its per-drive claimant flock (nonblocking: a
// busy claim is pending work, never waited on). The census launches nothing and
// takes NO run lock (in cancel mode the run is already fenced; the verdict-mode
// census may run over an active, unfenced run on the run-incomplete path via
// settleNeverLaunchedForVerdict; either way the lock order forbids holding the
// run while probing a per-drive claim).
package gatedrive

import (
	"errors"
	"io/fs"
	"os"
	"sort"
	"strconv"

	"github.com/danielhanold/docket/internal/process"
)

// censusMode selects what the run launch census may do (change 0491).
type censusMode int

const (
	// censusCancel is run.cancel's census: it stops a running supervisor and settles a
	// proven never-launched launch HALTED run-cancelled.
	censusCancel censusMode = iota
	// censusVerdict is the keyed run.verdict's census: it stops nothing and signals
	// nothing. It settles only a proven never-launched launch, HALTED
	// launch-abandoned; a running supervisor is run-live.
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
// is stopped and must then observe as not running; a launch that never attached
// is resolved through its exact launch token, and a proven never-launched one is
// settled HALTED "run-cancelled" under the held claim so no later launch can
// follow the cancel. An empty runContextHash names no drive and accounts
// vacuously.
func (d *Driver) ReconcileRunLaunches(runContextHash string) (RunLaunchReport, error) {
	return d.accountRunLaunches(runContextHash, censusCancel)
}

// VerdictRunLaunches is the keyed run.verdict's view of one run's launch
// obligations (change 0491; it replaces change 0441's observe-only twin, whose
// only caller was the closeout): the same walk, attribution, and claimant probe as
// ReconcileRunLaunches, but it never stops or signals a process. A proven
// never-launched FIRST launch is settled HALTED "launch-abandoned" under the held
// claim, so a successful run is not stranded by a launcher killed between Admit
// and StartAdmitted; a live supervisor is run-live and keeps the report
// unaccounted.
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
//     be proven gone (reconcileHaltedDrive) — through its recorded run dir, or,
//     for a first launch that never attached one, through its launch token.
//   - Nonterminal: the claimant flock is tried nonblocking (busy → claim-busy), the
//     record re-read under it, and then a launch that never attached resolves its
//     launch (admission) token, and an attached drive proves its run dir gone.
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
		// A held claim is a launch still in flight (StartAdmitted across launch/attach):
		// pending work, never proof of a crash and never waited on.
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
// dir gone.
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

// reconcileFirstLaunch resolves a first launch whose run dir was never attached
// through the drive's admission token (the token StartAdmitted hands the launch).
// A run root that does not exist was never created by a launch (or was
// cleaned), so it holds no reservation: clean absence, settled like a proven
// never-launched launch, as resolveHaltedFirstLaunch and supervisorGone already
// treat it (change 0491). Any other Lstat error is unknown, never absence, and
// stays pending. A proven
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

// settleNeverLaunchedFirstLaunch settles a launch that provably never ran, under
// the held per-drive claim, HALTED with the census mode's cause — "run-cancelled"
// in cancel mode, "launch-abandoned" in verdict mode (change 0491). Its CAS guard
// is that the drive still has no attached run dir; a record that moved on
// (errLaunchStateMoved) or a store fault stays pending, and an already-terminal
// record is accounted. A delayed StartAdmitted then re-reads a terminal record
// under the claim and refuses rather than launching.
func (d *Driver) settleNeverLaunchedFirstLaunch(id, ownerGen, cause string) (bool, string) {
	return d.settleNeverLaunched(id, ownerGen, cause, func(r *driveRecord) bool {
		return r.RawRunDir == ""
	})
}

// errLaunchStateMoved is the census's CAS-lost sentinel: the drive it was about
// to settle as never launched attached a run dir after the census read it, so
// the settle is abandoned and the drive stays pending (resolution-unresolved).
// It never escapes the census.
var errLaunchStateMoved = errors.New("gatedrive: drive launch state moved under the census")

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
			return errLaunchStateMoved
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

// proveRunDirsGone applies the lock model's teardown proof to the drive's one
// recorded run dir (RawRunDir; a drive never relaunches since change 0493): the
// drive is torn down when that supervisor is gone. A drive with no recorded run
// dir names no supervisor and is settled.
func (d *Driver) proveRunDirsGone(id string, rec driveRecord, mode censusMode) (bool, string) {
	if rec.RawRunDir == "" {
		return true, ""
	}
	return d.supervisorGone(id, rec.RawRunDir, mode)
}

// supervisorGone proves one run dir's supervisor gone. A run dir that does not
// exist is clean absence (its run root was removed after the terminal): torn down,
// with no Observe call. An observed exit (supervisorExited: passed, failed,
// signaled, stopped, vanished) counts as torn down. Its finding is run-terminal,
// or tree-survives:<drive>:<pgid> when process.Service.ProbeLeftover proves the
// dead supervisor's group still has members — still torn down, reported, never
// signalled (change 0492). A running supervisor is
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
		// The supervisor is gone, so the drive is torn down whatever is left. A
		// populated group whose leader is gone is reported, never signalled
		// (change 0492): with its supervisor dead, no lock proves the group is the
		// run's own. Only a successful leftover answer changes the finding; none,
		// unclear, and a probe error keep run-terminal.
		if lo, lerr := d.proc.ProbeLeftover(runDir); lerr == nil && lo.Answer == process.LeftoverPresent {
			return true, "tree-survives:" + id + ":" + strconv.Itoa(lo.PGID)
		}
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
