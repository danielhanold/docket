package app

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// liveRunStateUnreadable is the State a liveRunLocator carries for a run.json the
// store cannot read: its liveness is unknown, and unknown is never "not live".
const liveRunStateUnreadable = "unreadable"

// liveRunLocator names one run that may still be live, for a repository-wide
// precondition that refuses while any run is live. Remedy is the exact next step
// a human takes to settle it.
type liveRunLocator struct{ Key, State, ChangeID, Remedy string }

// liveRunsUnder scans every <stateDir>/<runTrackerDirName>/*/run.json through
// readStoredRun and returns the runs that may still be live, sorted by key.
//
// Live = RunActive, RunCompleting, RunCancelling, or any state this binary does
// not know; RunCompleted, RunCancelled, and RunSuperseded are not. A run.json the
// store cannot read (corrupt, unknown schema, an I/O error) is a locator with
// State liveRunStateUnreadable. A key directory without run.json holds no run and
// is skipped, as is any non-directory entry. A missing root is (nil, nil); any
// other failure to list the root is an error.
//
// Each locator's Remedy follows liveRunCancelAuthority, so it names a command that
// will act on the run in its current state: run cancel only where runCancelOwner
// accepts, the keyed run verdict for a readable run cancel would refuse, and the
// by-hand remedy where neither command can load the run.
func liveRunsUnder(stateDir string) ([]liveRunLocator, error) {
	root := filepath.Join(stateDir, runTrackerDirName)
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", root, err)
	}
	var live []liveRunLocator
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		key := e.Name()
		dir := filepath.Join(root, key)
		rec, _, rerr := readStoredRun(dir, "live-runs")
		if rerr != nil {
			if re, ok := AsRunError(rerr); ok && re.Kind == ErrRunNotFound {
				continue
			}
			live = append(live, liveRunLocator{
				Key: key, State: liveRunStateUnreadable, Remedy: byHandRemedy(dir),
			})
			continue
		}
		loc := liveRunLocator{Key: key, State: string(rec.State), ChangeID: rec.ChangeID}
		switch rec.State {
		case RunCompleted, RunCancelled, RunSuperseded:
			continue
		case RunActive, RunCompleting:
			switch liveRunCancelAuthority(dir, rec.ChangeID) {
			case liveRunCancelAccepts:
				loc.Remedy = runCancelCommand(key)
			case liveRunCancelRefuses:
				// Cancel refuses a run it cannot prove owned (ADR-0128); the keyed
				// verdict re-resolves ownership and retires a run that claimed nothing
				// (change 0540).
				loc.Remedy = runVerdictCommand(key)
			default:
				loc.Remedy = byHandRemedy(dir)
			}
		case RunCancelling:
			if liveRunCancelAuthority(dir, rec.ChangeID) == liveRunCancelAccepts {
				loc.Remedy = "re-run the same " + runCancelCommand(key) + " until it reports cancelled"
			} else {
				// Fenced cancelling with no claim to cancel under: no command settles it.
				loc.Remedy = byHandRemedy(dir)
			}
		default:
			// run cancel refuses a state it does not know, so only a person can settle it.
			loc.Remedy = byHandRemedy(dir)
		}
		live = append(live, loc)
	}
	sort.Slice(live, func(i, j int) bool { return live[i].Key < live[j].Key })
	return live, nil
}

// runCancelCommand is the ready-to-run cancel for key; <why> stays a literal
// placeholder for the human's reason, as in runStartStopNote.
func runCancelCommand(key string) string {
	return "docket run cancel --key " + key + " --reason <why>"
}

// runVerdictCommand is the ready-to-run keyed verdict for key: for a run cancel
// would refuse, it re-resolves ownership and, on any run-done, retires the run.
func runVerdictCommand(key string) string {
	return "docket run verdict " + key
}

// liveRunAuthority is what run cancel would decide about one live run, read from
// its own key directory.
type liveRunAuthority int

const (
	// liveRunAuthorityUnknown: record.json is missing or unreadable, or the claim
	// binding is unreadable. Neither run cancel nor run verdict can act on the run.
	liveRunAuthorityUnknown liveRunAuthority = iota
	// liveRunCancelAccepts: runCancelOwner accepts — run cancel would act.
	liveRunCancelAccepts
	// liveRunCancelRefuses: the run is readable but runCancelOwner refuses.
	liveRunCancelRefuses
)

// liveRunCancelAuthority asks runCancelOwner — the exact ownership proof runCancel
// applies — about the run in dir, whose run.json names runChangeID. It reads the
// files only and starts no git.
func liveRunCancelAuthority(dir, runChangeID string) liveRunAuthority {
	buf, err := os.ReadFile(filepath.Join(dir, runTrackerRecordFileName))
	if err != nil {
		return liveRunAuthorityUnknown
	}
	rec, err := decodeRunTrackerRecord(buf, "live-runs")
	if err != nil {
		return liveRunAuthorityUnknown
	}
	binding, hasBinding, berr := readRunTrackerClaimBinding(dir, "live-runs")
	if berr != nil {
		// Cancel refuses claim-unreadable and the verdict stops binding-unreadable
		// without retiring the run: neither command settles it.
		return liveRunAuthorityUnknown
	}
	if _, refusal := runCancelOwner(rec, binding, hasBinding, nil, runChangeID); refusal != "" {
		return liveRunCancelRefuses
	}
	return liveRunCancelAccepts
}

func byHandRemedy(dir string) string {
	return "inspect or remove " + dir + " by hand"
}
