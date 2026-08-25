// The production run-waiting receipt reader: the WaitingReceiptReader `run
// verify` composes to derive the local run-waiting verdict from agreeing
// receipts. It reaches the durable gate-drive store and the native process
// supervisor through the same seams the gate-drive service uses (internal/app is
// the only layer allowed to touch internal/process), and it is a PURE READER —
// it opens no transaction, writes nothing, and never signals a process.
//
// It gathers, for one change: the single drive receipt with an explicit
// unclaimed handoff (gatedrive.FindWaitingReceipt), the live worktree
// recomputation (gatedrive.ComputeLiveFingerprint), whether the linked worktree
// still exists, whether the native ownership receipt still matches the active
// attempt (a read-only process observation), and whether the fixed deadline is
// still live. Any recompute fault folds to found=false — never an invented
// waiting — leaving the reporter to report its ordinary postcondition verdict.
package app

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/process"
)

// receiptWaitingReader is the production WaitingReceiptReader over the durable
// gate-drive store and the native process supervisor.
type receiptWaitingReader struct {
	store *gatedrive.Store
	proc  *process.Service
	now   func() time.Time
}

// NewWaitingReceiptReader composes the production run-waiting receipt reader,
// rooting the durable drive store at the repository's Git common directory and
// binding the native process supervisor at exePath (the detached-supervisor
// re-exec target). A process-service construction failure is returned so the
// caller can leave run verify's waiting derivation unwired rather than failing
// the whole report.
func NewWaitingReceiptReader(gitCommonDir, exePath string) (WaitingReceiptReader, error) {
	proc, err := process.NewService(exePath)
	if err != nil {
		return nil, err
	}
	return receiptWaitingReader{
		store: gatedrive.OpenStore(gitCommonDir),
		proc:  proc,
		now:   time.Now,
	}, nil
}

// Read gathers the agreeing local receipts for changeID. It returns found=false
// (never an invented waiting) when no unclaimed-handoff drive matches the change,
// or when the drive-start identity cannot be recomputed from the current
// worktree. Only a real store-read fault is returned as an error.
func (r receiptWaitingReader) Read(_ context.Context, _ string, changeID int) (WaitingReceipt, bool, error) {
	dr, ok, err := r.store.FindWaitingReceipt(strconv.Itoa(changeID))
	if err != nil || !ok {
		return WaitingReceipt{}, false, err
	}

	// Recompute the live worktree identity. A recompute fault (a vanished or
	// unreadable worktree) is not a safe continuation, so fold it to no-waiting.
	live, ferr := gatedrive.ComputeLiveFingerprint(dr.WorktreePath)
	if ferr != nil {
		return WaitingReceipt{}, false, nil
	}
	_, statErr := os.Stat(dr.WorktreePath)
	worktreeExists := statErr == nil

	// The referenced raw run's native ownership receipt must still name the active
	// attempt. A read-only observation whose recorded run id and directory match
	// the drive's persisted references confirms it; any read fault is "no match".
	rawRunMatches := false
	if obs, oerr := r.proc.Observe(dr.RawRunDir); oerr == nil && obs != nil {
		rawRunMatches = obs.RunID == dr.RawOwnership && obs.RunDir == dr.RawRunDir
	}

	now := r.now()
	return WaitingReceipt{
		DriveID:             dr.DriveID,
		HasUnclaimedHandoff: dr.HasUnclaimedHandoff,
		ChangeID:            dr.ChangeID,
		TaskID:              dr.TaskID,
		Phase:               dr.Phase,
		Branch:              dr.Branch,
		WorktreePath:        dr.WorktreePath,
		WorktreeExists:      worktreeExists,
		DriveHead:           dr.Head,
		DriveFingerprint:    dr.Fingerprint,
		LiveFingerprint:     live,
		DeadlineLive:        !now.After(dr.Deadline),
		TerminalWaiting:     dr.LastOutcome == gatedrive.PASSED || dr.LastOutcome == gatedrive.FAILED,
		RawRunMatches:       rawRunMatches,
	}, true, nil
}
