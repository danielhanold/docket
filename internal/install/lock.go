package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// Two installations running at once is a correctness problem for this
// installer, not a throughput one, and the transaction engine is exactly where
// it bites: a journal on disk means "an interrupted run left work to undo", and
// nothing in the journal says whose run it is. A second install starting while
// a first is mid-apply therefore reads the first's live journal as wreckage and
// rolls it back underneath it — see recoverPending — leaving a world neither
// journal describes and a published state that describes neither.
//
// The fix is to make the whole mutating span of an installation — recovery,
// version-tree extraction, plan, apply, commit — mutually exclusive. Under that
// lock the ambiguity disappears: a journal found while holding it cannot belong
// to a live run, because a live run would be holding the lock itself. Recovery
// is safe precisely because it is unreachable without the lock, which is why
// recoverPending takes one rather than trusting its callers to have taken it.
//
// Why flock and not an O_EXCL lockfile. An O_EXCL lockfile's unlock is a file
// removal, so a run killed between the create and the remove — SIGKILL, a power
// cut, a panic — leaves a lockfile that no later run may safely remove, since
// removing it is indistinguishable from stealing a live lock. Every subsequent
// install then refuses forever and the only cure is a user deleting a file
// docket never told them about. flock's lock lives on the open file description
// instead: the kernel drops it when the last descriptor closes, which includes
// the implicit close of a dying process, however it died. A crash therefore
// costs the next run nothing. The lock FILE is deliberately left behind on
// release — it is a mutex, not a flag, and its mere presence means nothing.
//
// This ties the installer to a platform with flock, which is every platform it
// ships to: TestCrossCompileApprovedTargets pins the buildable tuples to darwin
// and linux, on both of which syscall.Flock exists. A fifth target would have to
// answer this question before it could answer any other — its harness roots are
// XDG and dot-directory paths and its targets are symlinks.

const (
	// installLockName is the whole-installation mutex. It sits at the root of
	// the data tree rather than inside state/ or transactions/ because it guards
	// both of them and belongs to neither, and because a directory this lock may
	// have to be taken before creating must not be the lock's own parent.
	installLockName = "install.lock"
	// installLockMode keeps the lock private to the user whose installation it
	// serializes. Nothing ever reads its contents; the file exists to carry the
	// lock.
	installLockMode = 0o600
	// dataRootMode is the mode the data root is created with when an install is
	// the first thing to need it. It matches versionsDirMode: everything under
	// the data root is docket's own bookkeeping about this user's installation.
	dataRootMode = 0o700
)

// ErrInstallLocked is another installation holding the exclusive lock. It is a
// wait-and-retry condition rather than a defect, and it is the one refusal that
// says nothing about the user's filesystem.
var ErrInstallLocked = errors.New("install: another installation is already running")

// installLock is a held exclusive installation lock. The zero value and a nil
// pointer are both "not held", so a caller that never acquired one cannot be
// mistaken for one that did.
type installLock struct {
	path string
	f    *os.File
}

// held reports whether this lock is currently owned by this process.
func (l *installLock) held() bool { return l != nil && l.f != nil }

// acquireInstallLock takes the exclusive installation lock without waiting, and
// returns ErrInstallLocked when another process holds it. Not waiting is the
// point: an installer that blocked would leave a user staring at a silent
// process with no way to know whether it was working or wedged, while the
// refusal is honest, immediate, and safe to retry.
//
// It does not route through FSOps. The lock is not installed material — it is
// never journaled, never rolled back, and never part of a plan — and the seam
// carries no way to hand back the open descriptor the lock lives on. Check must
// therefore never call this function; that is what keeps `install check`
// read-only, and TestCheckNeverTakesTheLock is what proves it.
func acquireInstallLock(roots UserRoots) (*installLock, error) {
	if roots.DataRoot == "" || !filepath.IsAbs(roots.DataRoot) {
		return nil, fmt.Errorf("%w: roots carry no absolute data root", ErrInvalidInput)
	}
	path := roots.LockPath()
	if err := os.MkdirAll(roots.DataRoot, dataRootMode); err != nil {
		return nil, fmt.Errorf("install: creating %s: %w", roots.DataRoot, err)
	}
	// O_CREATE and not O_EXCL: an existing lock file is the normal case, whether
	// it was left by a clean release or by a process that died holding it. Which
	// of the two it was is a question only flock can answer, and it answers it
	// below.
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, installLockMode)
	if err != nil {
		return nil, fmt.Errorf("install: opening the installation lock %s: %w", path, err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, fmt.Errorf("%w: %s is held by another docket process; wait for it to finish, then re-run",
				ErrInstallLocked, path)
		}
		return nil, fmt.Errorf("install: locking %s: %w", path, err)
	}
	return &installLock{path: path, f: f}, nil
}

// release drops the lock. Closing the descriptor would be enough — the kernel
// releases the flock with it — but the explicit unlock states the intent, and
// the nil-out makes a double release a no-op rather than a close of a
// descriptor number something else may since have been given.
func (l *installLock) release() {
	if !l.held() {
		return
	}
	_ = syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	_ = l.f.Close()
	l.f = nil
}

// lockReason maps an acquisition failure onto the service's stable vocabulary.
// Contention is its own reason: it is the one failure where nothing is wrong
// and the answer is simply to run again in a moment.
func lockReason(err error) string {
	switch {
	case errors.Is(err, ErrInstallLocked):
		return ReasonInstallInProgress
	case errors.Is(err, ErrInvalidInput):
		return ReasonInvalidOptions
	default:
		return ReasonFilesystemFailed
	}
}
