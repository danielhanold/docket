// The per-worktree admission lock (change 0490, superseding ADR-0118's durable
// slot). One canonical worktree admits at most one live top-level gate: every
// launch site first takes a NON-BLOCKING exclusive flock on
//
//	<git-common-dir>/docket/worktree-locks/<key>/busy.lock
//
// and hands it to the gate supervisor (process.LaunchRequest.WorktreeLock), which
// holds it for its whole life; the kernel releases it when the supervisor exits
// or dies. "Busy" therefore means "a live supervisor holds it", and a dead gate
// frees the worktree with no recovery step. The lock is only ever tried, never
// waited on; it is released by close only (never LOCK_UN). Lock files and their
// directories are never deleted: unlinking a lock file races a concurrent opener
// into holding a lock on an orphaned inode.
//
// holder.json beside busy.lock is a DIAGNOSTIC only: it names the last holder
// and is printed on a busy refusal only after Observe confirms its run is still
// running. Admission, cancel, and every other decision ignore it.
package gatedrive

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/danielhanold/docket/internal/process"
)

const (
	worktreeLockFile    = "busy.lock"
	worktreeHolderFile  = "holder.json"
	opWorktreeAdmission = "worktree-admission"
)

// HolderNote is the diagnostic record of the gate that last took a worktree lock.
type HolderNote struct {
	Kind      string    `json:"kind"` // "drive" | "raw"
	DriveID   string    `json:"drive_id,omitempty"`
	RunDir    string    `json:"run_dir"`
	ChangeID  string    `json:"change_id,omitempty"`
	Owner     string    `json:"owner,omitempty"` // "build" | "finalize" | "raw" | ""
	WrittenAt time.Time `json:"written_at"`
}

// HolderObserver is the read-only run observation the busy diagnosis needs.
// Both ProcessSeam and *process.Service satisfy it.
type HolderObserver interface {
	Observe(runDir string) (*process.Observation, error)
}

// WorktreeLock is a held worktree lock. Its file travels to process.Launch
// through TakeFile; Release closes it when a caller abandons the admission
// before launching. dir survives the handoff so the holder note can be written
// after the launch.
type WorktreeLock struct {
	file *os.File
	dir  string
}

// worktreeLockKey is the lowercase hex sha256 of a canonical worktree root.
func worktreeLockKey(canonicalRoot string) string {
	sum := sha256.Sum256([]byte(canonicalRoot))
	return hex.EncodeToString(sum[:])
}

// TryWorktreeLock takes the worktree lock for canonicalRoot (a root already
// resolved through gitcli.DiscoverWorktree — every symlink hop resolved, see
// GitSeam.WorktreeRoot), never blocking. A held lock is a typed ErrWorktreeBusy
// whose Incumbent names the holder only when obs confirms its run is running;
// any other failure is a typed ErrIO and is never read as free.
func (s *Store) TryWorktreeLock(canonicalRoot string, obs HolderObserver) (*WorktreeLock, error) {
	if !filepath.IsAbs(canonicalRoot) {
		return nil, storeErr(ErrInvalidID, opWorktreeAdmission, nil)
	}
	if err := ensurePrivateDir(s.lockRoot); err != nil {
		return nil, storeErr(ErrIO, opWorktreeAdmission, err)
	}
	dir := filepath.Join(s.lockRoot, worktreeLockKey(canonicalRoot))
	if err := ensurePrivateDir(dir); err != nil {
		return nil, storeErr(ErrIO, opWorktreeAdmission, err)
	}
	f, busy, err := process.TryExclusiveLock(filepath.Join(dir, worktreeLockFile))
	if err != nil {
		return nil, storeErr(ErrIO, opWorktreeAdmission, err)
	}
	if busy {
		oe := ownershipErr(ErrWorktreeBusy, opWorktreeAdmission)
		oe.Incumbent = liveHolder(dir, obs)
		return nil, oe
	}
	return &WorktreeLock{file: f, dir: dir}, nil
}

// TakeFile transfers the locked file to the caller (process.Launch takes
// ownership and closes it on every path). A second call returns nil.
func (l *WorktreeLock) TakeFile() *os.File {
	if l == nil {
		return nil
	}
	f := l.file
	l.file = nil
	return f
}

// Release closes a lock that was never handed to Launch. Idempotent and
// nil-safe. Closing is the only release: no code calls LOCK_UN on this lock.
func (l *WorktreeLock) Release() {
	if l == nil || l.file == nil {
		return
	}
	_ = l.file.Close()
	l.file = nil
}

// WriteHolder records the holder note atomically beside (never over) busy.lock.
// Best effort: a failed write is ignored and never fails the launch.
func (l *WorktreeLock) WriteHolder(n HolderNote) {
	if l == nil {
		return
	}
	if n.WrittenAt.IsZero() {
		n.WrittenAt = time.Now().UTC()
	}
	_ = writeAtomicJSON(filepath.Join(l.dir, worktreeHolderFile), n)
}

// PriorHolder reads the current note, for a relaunch that keeps its owner.
func (l *WorktreeLock) PriorHolder() (HolderNote, bool) {
	if l == nil {
		return HolderNote{}, false
	}
	return readHolderNote(l.dir)
}

func readHolderNote(dir string) (HolderNote, bool) {
	buf, err := os.ReadFile(filepath.Join(dir, worktreeHolderFile))
	if err != nil {
		return HolderNote{}, false
	}
	var n HolderNote
	if json.Unmarshal(buf, &n) != nil {
		return HolderNote{}, false
	}
	return n, true
}

// liveHolder projects the holder note into an IncumbentSnapshot only when obs
// confirms its run is running. A missing/unreadable note, a nil observer, an
// observation error, or any non-running state yields nil ("holder unknown"): a
// stale note from an earlier holder is never shown.
func liveHolder(dir string, obs HolderObserver) *IncumbentSnapshot {
	n, ok := readHolderNote(dir)
	if !ok || obs == nil || n.RunDir == "" {
		return nil
	}
	o, err := obs.Observe(n.RunDir)
	if err != nil || o == nil || o.State != process.StateRunning {
		return nil
	}
	return &IncumbentSnapshot{
		Kind:      n.Kind,
		DriveID:   n.DriveID,
		RawRunID:  filepath.Base(n.RunDir),
		RawRunDir: n.RunDir,
		ChangeID:  n.ChangeID,
		Owner:     n.Owner,
	}
}
