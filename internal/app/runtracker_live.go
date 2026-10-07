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
			loc.Remedy = runCancelCommand(key)
		case RunCancelling:
			loc.Remedy = "re-run the same " + runCancelCommand(key) + " until it reports cancelled"
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

func byHandRemedy(dir string) string {
	return "inspect or remove " + dir + " by hand"
}
