package process

import (
	"os"
	"path/filepath"
	"sort"
)

// abandonedCause is the stable cause an abandoned marker records: a supervisor
// that released its live lock leaving no terminal record, with the recorded
// group provably gone. Observe surfaces it verbatim (bounded) as a vanished
// run's cause.
const abandonedCause = "supervisor lock released with no terminal record and the recorded group cleanly absent"

// RecoveryEntry is Recover's per-slot verdict. Disposition is one of
// "live", "terminal", "stopped", "abandoned-marked", "already-abandoned",
// "needs-inspection", "foreign", "invalid"; Reason is a bounded human note.
type RecoveryEntry struct {
	RunID       string
	RunDir      string
	Disposition string
	Reason      string
}

// RecoverOutcome is Recover's full verdict. Entries is sorted by RunID
// ascending; Marked counts only markers this pass newly wrote (never the
// already-abandoned no-ops), so a second pass over the same root marks nothing.
type RecoverOutcome struct {
	Entries []RecoveryEntry
	Marked  int
}

// recoverGroupProbe answers whether a run's recorded process group is live.
// Production is groupAlive; a test overrides it to drive the unprovable-probe
// branch deterministically on every platform, proving the guard that keeps
// probeUnknown out of the clean-absence (abandoned) arm — a probe error
// (EPERM, an unreaped zombie group leader) is never mistaken for provable
// absence, which alone authorizes a marker.
var recoverGroupProbe = groupAlive

// Recover scans a root of run slots and, for each owned run the supervisor
// abandoned cleanly, writes an abandoned marker — signalling nothing and
// deleting nothing. Every other slot is reported and left byte-untouched. The
// order is load-bearing (spec: Recover):
//
//  1. Validate root: an absolute, existing directory.
//  2. Snapshot the slots under the registry lock so allocation cannot expose a
//     half-built slot mid-scan; keep Lstat-real run-id directories as
//     candidates, report symlinks and non-run-id directories as foreign, skip
//     plain files (the registry lock among them). Release the lock.
//  3. Per candidate, with no lock held: an unreadable/malformed/self-
//     disagreeing manifest is invalid; a held live lock is live; an unprovable
//     live-lock probe is needs-inspection; a free lock re-reads terminal then
//     stopped (a durable verdict outranks any liveness guess); an existing
//     abandoned marker is already-abandoned; otherwise only a provably-absent
//     recorded group authorizes a fresh abandoned marker — a live or unprovable
//     group is left for inspection.
func (s *Service) Recover(root string) (*RecoverOutcome, error) {
	if !filepath.IsAbs(root) {
		return nil, failf(FailInvalidInput, "recover", "root must be an absolute path")
	}
	if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
		return nil, failf(FailInvalidInput, "recover", "root must be an existing directory")
	}

	snap, err := s.recoverSnapshot(root)
	if err != nil {
		return nil, err
	}

	out := &RecoverOutcome{}
	for _, name := range snap.foreign {
		out.Entries = append(out.Entries, RecoveryEntry{
			RunID:       name,
			RunDir:      filepath.Join(root, name),
			Disposition: "foreign",
			Reason:      "not a run slot; left untouched",
		})
	}
	for _, name := range snap.candidates {
		entry, cerr := s.classifyRun(filepath.Join(root, name), name)
		if cerr != nil {
			return nil, cerr
		}
		if entry.Disposition == "abandoned-marked" {
			out.Marked++
		}
		out.Entries = append(out.Entries, entry)
	}
	sort.Slice(out.Entries, func(i, j int) bool { return out.Entries[i].RunID < out.Entries[j].RunID })
	return out, nil
}

// recoverSnapshotResult is the registry-lock-guarded slot census: run-id
// directories to classify, and foreign names (symlinks, non-run-id
// directories) to report untouched.
type recoverSnapshotResult struct {
	candidates []string
	foreign    []string
}

// recoverSnapshot takes the slot census under the registry lock and releases
// it before returning — recovery holds no lock while it probes or writes a
// per-run marker, exactly as the classify pass runs unlocked.
func (s *Service) recoverSnapshot(root string) (recoverSnapshotResult, error) {
	lock, err := acquireFlock(filepath.Join(root, registryLockFile))
	if err != nil {
		return recoverSnapshotResult{}, err
	}
	defer lock.Close()

	entries, err := os.ReadDir(root)
	if err != nil {
		return recoverSnapshotResult{}, failf(FailExternal, "recover", "reading root: %v", err)
	}
	var res recoverSnapshotResult
	for _, e := range entries {
		name := e.Name()
		li, lerr := os.Lstat(filepath.Join(root, name))
		if lerr != nil {
			continue // vanished between readdir and lstat; nothing to classify
		}
		if li.Mode()&os.ModeSymlink != 0 {
			// A symlink at a run slot is never followed — it cannot be a run we
			// own; report it untouched.
			res.foreign = append(res.foreign, name)
			continue
		}
		if !li.IsDir() {
			// Plain files (the registry lock among them) are not run slots.
			continue
		}
		if !runIDPattern.MatchString(name) {
			res.foreign = append(res.foreign, name)
			continue
		}
		res.candidates = append(res.candidates, name)
	}
	return res, nil
}

// classifyRun decides one run-id slot's disposition and, only for a cleanly
// abandoned owned run, writes its marker. It never signals and never deletes.
// A returned error is a marker write failure alone; every read failure fails
// closed to a no-mark disposition rather than aborting the whole scan.
func (s *Service) classifyRun(runDir, name string) (RecoveryEntry, error) {
	entry := RecoveryEntry{RunID: name, RunDir: runDir}

	// Manifest first: without a self-agreeing manifest there is no run identity
	// to prove ownership against, so the slot is invalid and untouched.
	m, err := readManifest(runDir)
	if err != nil || m == nil || m.RunID != name {
		entry.Disposition = "invalid"
		entry.Reason = "unreadable, malformed, or self-disagreeing manifest; left untouched"
		return entry, nil
	}

	// Live lock: a held lock means a supervisor still owns the run. An
	// unprovable probe is never mistaken for a free lock.
	held, ans := probeFlock(filepath.Join(runDir, liveLockFile))
	if ans == probeUnknown {
		entry.Disposition = "needs-inspection"
		entry.Reason = "live lock unprovable; not marked"
		return entry, nil
	}
	if held {
		entry.Disposition = "live"
		entry.Reason = "supervisor holds the live lock"
		return entry, nil
	}

	// Free lock: the supervisor is gone. A durable verdict outranks any
	// liveness guess — terminal, then a completed-stop marker.
	if term, terr := readTerminal(runDir); terr != nil {
		entry.Disposition = "needs-inspection"
		entry.Reason = "terminal record unreadable; not marked"
		return entry, nil
	} else if term != nil {
		entry.Disposition = "terminal"
		entry.Reason = "durable terminal record present"
		return entry, nil
	}
	if stopped, serr := readStopped(runDir); serr != nil {
		entry.Disposition = "needs-inspection"
		entry.Reason = "stopped marker unreadable; not marked"
		return entry, nil
	} else if stopped != nil {
		entry.Disposition = "stopped"
		entry.Reason = "completed-stop marker present"
		return entry, nil
	}

	// A supervisor start-failure record (failure.json) is deliberately NOT
	// given a recover disposition: unlike Observe, which surfaces failure.json
	// as the vanished Cause, recover intentionally leaves failure records
	// untouched for human inspection. Such a slot falls through to the group
	// probe below and lands as needs-inspection — an intended asymmetry.

	// An abandoned marker from an earlier pass is never re-written — the pass
	// that wrote it already counted it.
	if ab, aerr := readAbandoned(runDir); aerr != nil {
		entry.Disposition = "needs-inspection"
		entry.Reason = "abandoned marker unreadable; not marked"
		return entry, nil
	} else if ab != nil {
		entry.Disposition = "already-abandoned"
		entry.Reason = "abandoned marker already present"
		return entry, nil
	}

	// No verdict, no marker, supervisor gone: the recorded group is the last
	// question. ONLY provable clean absence authorizes a marker — probeLive
	// keeps the group's members in play, and probeUnknown is an unprovable read
	// that must never collapse into absence (or an EPERM read becomes a false
	// mark). Both are left for inspection, signalled and deleted nothing.
	switch recoverGroupProbe(m.PGID) {
	case probeAbsent:
		if werr := writeAtomicJSON(filepath.Join(runDir, abandonedFile), &abandonedRecord{
			Schema:     recordSchema,
			RunID:      m.RunID,
			Cause:      abandonedCause,
			RecordedAt: supervisorStamp(),
		}); werr != nil {
			return entry, werr
		}
		entry.Disposition = "abandoned-marked"
		entry.Reason = abandonedCause
		return entry, nil
	case probeLive, probeUnknown:
		entry.Disposition = "needs-inspection"
		entry.Reason = "recorded group is live or unprovable; not marked"
		return entry, nil
	}
	// Unreachable: probeAnswer is exhaustively handled above.
	entry.Disposition = "needs-inspection"
	entry.Reason = "recorded group state indeterminate; not marked"
	return entry, nil
}
