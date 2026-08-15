package transaction

// This file owns the private candidate lifecycle: the transactions root beneath
// the repository's common directory, cross-process ownership via flock, and the
// atomically published per-candidate manifest.
//
// Locking follows internal/install/lock.go's reasoning exactly (reference only —
// the transaction package must not import internal/install). flock's lock lives
// on the open file description, so the kernel drops it when the last descriptor
// closes, including the implicit close of a process that died however it died. A
// crash therefore costs the next run nothing, and liveness is "the lock is held
// right now", never a PID or a timestamp recorded in the manifest. The lock
// FILES are deliberately left on disk on release — they are mutexes, not flags,
// and their mere presence means nothing. That is exactly why the manifest's PID
// field is diagnostic only: a recovering run proves abandonment by acquiring the
// candidate's live lock, not by reading who wrote it.
//
// Two locks with a fixed acquire order keep allocation and recovery mutually
// exclusive without deadlock: the registry lock (one per transactions root) is
// held only across the brief allocate/inventory critical section and never
// across mutation work; each candidate's own live lock is held for the whole
// life of the transaction. A caller therefore never blocks on the registry lock
// while holding a live lock for longer than the allocation itself.

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/danielhanold/docket/internal/gitcli"
)

const (
	// manifestSchemaVersion is the on-disk manifest format version. A recovering
	// run refuses any candidate whose schema it does not recognize.
	manifestSchemaVersion = 1

	// manifestFileName is the published manifest inside a candidate directory.
	manifestFileName = "manifest.json"
	// registryLockName is the transactions-root mutex serializing allocation and
	// inventory.
	registryLockName = "registry.lock"
	// liveLockName is a candidate's ownership lock, held for the transaction's
	// whole life. Its presence proves nothing; only holding it proves ownership.
	liveLockName = "live.lock"
	// worktreeDirName is the fixed basename of a candidate's detached worktree.
	worktreeDirName = "worktree"
	// hooksDirName is the fixed basename of a candidate's owned empty hooks dir,
	// passed per command as core.hooksPath so no repository hook ever fires.
	hooksDirName = "hooks"

	// txnDirMode and txnFileMode are the documented private modes. They are
	// enforced with an explicit Chmod after creation because O_CREATE / Mkdir
	// perms are masked by the process umask, and this tree must stay private
	// regardless of the ambient umask.
	txnDirMode  os.FileMode = 0o700
	txnFileMode os.FileMode = 0o600
)

// errLockHeld is the sentinel a non-blocking acquire returns when another open
// file description already holds the flock. It is a contention signal, not a
// defect: the answer is to try again (allocation) or to treat the candidate as
// live (recovery).
var errLockHeld = errors.New("transaction: lock is held by another process")

// phase is the coarse lifecycle stage recorded in a candidate's manifest. It is
// advisory for a recovering run (paired with reachability), never authority for
// liveness — the held live lock is the only liveness signal.
type phase string

// The closed set of candidate phases.
const (
	phaseAllocating phase = "allocating"
	phaseReady      phase = "ready"
	phaseCommitted  phase = "committed"
	phasePushed     phase = "pushed"
)

// manifest is a candidate's published description. It is written atomically and
// carries enough repository identity for a recovering run to prove ownership
// (CommonDir + WorktreeRel) before it deletes anything. PID is diagnostic only.
type manifest struct {
	Schema        int               `json:"schema"`
	TransactionID string            `json:"transaction_id"`
	CommonDir     string            `json:"common_dir"` // canonical
	Remote        gitcli.RemoteName `json:"remote"`
	TargetRef     gitcli.RefName    `json:"target_ref"`
	BaseCommit    gitcli.ObjectID   `json:"base_commit"`
	WorktreeRel   string            `json:"worktree_rel"` // "worktree"
	Phase         phase             `json:"phase"`
	CreatedUTC    string            `json:"created_utc"` // RFC3339, from Clock
	UpdatedUTC    string            `json:"updated_utc"`
	PID           int               `json:"pid"` // diagnostic only — never liveness
}

// transactionsRoot is the private root of all candidates for repo:
// <CommonDir>/docket/transactions. It sits under the shared common directory so
// it is invisible to any working-tree status and shared across every linked
// worktree of the repository.
func transactionsRoot(repo gitcli.Repository) string {
	return filepath.Join(repo.CommonDir, "docket", "transactions")
}

// newTransactionID mints a 32-character lowercase-hex id from 128 bits of
// crypto/rand entropy. It is directory-safe and collision-resistant enough that
// two concurrent allocations under the registry lock never need a retry.
func newTransactionID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("transaction: reading random id: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

// candidate is an allocated, live-locked transaction workspace.
type candidate struct {
	id       string
	root     string // <transactionsRoot>/<id>
	worktree string // <root>/worktree
	hooks    string // <root>/hooks
	live     *fileLock
}

// fileLock is a held flock on an open file description. A nil pointer or a nil
// file both mean "not held", so a caller that never acquired one cannot be
// mistaken for one that did.
type fileLock struct{ f *os.File }

// acquireLock opens (creating if needed) the file at path and takes an exclusive
// flock on it. When block is false it uses LOCK_NB and maps the would-block error
// onto errLockHeld; when block is true it waits. The lock file's mode is fixed to
// txnFileMode with an explicit Chmod because O_CREATE's perm is umask-masked.
func acquireLock(path string, block bool) (*fileLock, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, txnFileMode)
	if err != nil {
		return nil, fmt.Errorf("transaction: opening lock %s: %w", path, err)
	}
	if err := os.Chmod(path, txnFileMode); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("transaction: setting mode on %s: %w", path, err)
	}
	how := syscall.LOCK_EX
	if !block {
		how |= syscall.LOCK_NB
	}
	if err := syscall.Flock(int(f.Fd()), how); err != nil {
		_ = f.Close()
		if !block && errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, fmt.Errorf("%w: %s", errLockHeld, path)
		}
		return nil, fmt.Errorf("transaction: locking %s: %w", path, err)
	}
	return &fileLock{f: f}, nil
}

// release drops the lock and closes its descriptor. Closing alone would release
// the flock, but the explicit unlock states the intent, and niling the file
// makes a double release a no-op rather than acting on a descriptor number the
// kernel may since have reused.
func (l *fileLock) release() error {
	if l == nil || l.f == nil {
		return nil
	}
	_ = syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	err := l.f.Close()
	l.f = nil
	if err != nil {
		return fmt.Errorf("transaction: releasing lock: %w", err)
	}
	return nil
}

// ensureTransactionsRoot creates the transactions root (and its parent docket
// directory) and forces it to txnDirMode. The explicit Chmod defeats a
// permissive umask so the private root is 0700 no matter the ambient value.
func ensureTransactionsRoot(root string) error {
	if err := os.MkdirAll(root, txnDirMode); err != nil {
		return fmt.Errorf("transaction: creating transactions root %s: %w", root, err)
	}
	if err := os.Chmod(root, txnDirMode); err != nil {
		return fmt.Errorf("transaction: setting mode on %s: %w", root, err)
	}
	return nil
}

// withRegistryLock runs fn while holding the transactions-root registry lock,
// blocking until it is available. The lock is released before withRegistryLock
// returns, so it never covers mutation work — only the allocate/inventory
// critical section fn performs.
func withRegistryLock(root string, fn func() error) error {
	if err := ensureTransactionsRoot(root); err != nil {
		return err
	}
	lk, err := acquireLock(filepath.Join(root, registryLockName), true)
	if err != nil {
		return err
	}
	defer func() { _ = lk.release() }()
	return fn()
}

// allocateCandidate creates a fresh, live-locked candidate under repo's
// transactions root. Under the registry lock it mints an id, makes the candidate
// root and empty hooks directory (0700, explicit Chmod), acquires the live lock
// BEFORE any manifest exists — so a recovering run can never see a manifest
// whose owner it cannot test — and only then atomically publishes the manifest
// in phase "allocating". The registry lock is dropped on return; the returned
// candidate still holds its live lock, which the caller owns until cleanup.
func allocateCandidate(clk Clock, repo gitcli.Repository, remote gitcli.RemoteName,
	ref gitcli.RefName, base gitcli.ObjectID) (*candidate, error) {
	root := transactionsRoot(repo)
	var c *candidate
	err := withRegistryLock(root, func() error {
		id, err := newTransactionID()
		if err != nil {
			return err
		}
		candRoot := filepath.Join(root, id)
		hooks := filepath.Join(candRoot, hooksDirName)

		if err := mkdirMode(candRoot, txnDirMode); err != nil {
			return err
		}
		if err := mkdirMode(hooks, txnDirMode); err != nil {
			return err
		}

		// Live lock first: no manifest is visible to a recovering run until its
		// owner already holds this lock, so an inventory can always test liveness.
		live, err := acquireLock(filepath.Join(candRoot, liveLockName), false)
		if err != nil {
			return err
		}

		now := clk.Now().UTC().Format(time.RFC3339)
		m := manifest{
			Schema:        manifestSchemaVersion,
			TransactionID: id,
			CommonDir:     repo.CommonDir,
			Remote:        remote,
			TargetRef:     ref,
			BaseCommit:    base,
			WorktreeRel:   worktreeDirName,
			Phase:         phaseAllocating,
			CreatedUTC:    now,
			UpdatedUTC:    now,
			PID:           os.Getpid(),
		}
		if err := writeManifestAtomic(candRoot, m); err != nil {
			_ = live.release()
			return err
		}

		c = &candidate{
			id:       id,
			root:     candRoot,
			worktree: filepath.Join(candRoot, worktreeDirName),
			hooks:    hooks,
			live:     live,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return c, nil
}

// mkdirMode makes exactly dir and forces its mode with an explicit Chmod so a
// permissive umask cannot loosen it.
func mkdirMode(dir string, mode os.FileMode) error {
	if err := os.Mkdir(dir, mode); err != nil {
		return fmt.Errorf("transaction: creating %s: %w", dir, err)
	}
	if err := os.Chmod(dir, mode); err != nil {
		return fmt.Errorf("transaction: setting mode on %s: %w", dir, err)
	}
	return nil
}

// setPhase rewrites the manifest with a new phase and a fresh UpdatedUTC stamp.
// The rewrite is atomic (temp+rename), so a concurrent reader always observes a
// complete document.
func (c *candidate) setPhase(clk Clock, p phase) error {
	m, err := c.readManifest()
	if err != nil {
		return err
	}
	m.Phase = p
	m.UpdatedUTC = clk.Now().UTC().Format(time.RFC3339)
	return writeManifestAtomic(c.root, m)
}

// readManifest loads and decodes the candidate's current manifest.
func (c *candidate) readManifest() (manifest, error) {
	data, err := os.ReadFile(filepath.Join(c.root, manifestFileName))
	if err != nil {
		return manifest{}, fmt.Errorf("transaction: reading manifest: %w", err)
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return manifest{}, fmt.Errorf("transaction: decoding manifest: %w", err)
	}
	return m, nil
}

// writeManifestAtomic publishes the manifest in candRoot: it writes a
// same-directory temp file (so the rename stays on one filesystem), chmods it to
// txnFileMode, fsyncs the file, renames it over the destination, and fsyncs the
// directory so the rename is durable. Any exit before the rename removes the
// temp file, so a failed publish never leaves a stray beside the manifest.
func writeManifestAtomic(candRoot string, m manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("transaction: encoding manifest: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(candRoot, ".manifest.json.*.tmp")
	if err != nil {
		return fmt.Errorf("transaction: staging manifest: %w", err)
	}
	tmpName := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("transaction: writing manifest: %w", err)
	}
	if err := tmp.Chmod(txnFileMode); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("transaction: setting mode on manifest temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("transaction: syncing manifest: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("transaction: closing manifest temp: %w", err)
	}
	if err := os.Rename(tmpName, filepath.Join(candRoot, manifestFileName)); err != nil {
		return fmt.Errorf("transaction: publishing manifest: %w", err)
	}
	if err := syncDir(candRoot); err != nil {
		return err
	}
	committed = true
	return nil
}

// syncDir fsyncs a directory so a rename into it is durable across a crash.
func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("transaction: opening %s for sync: %w", dir, err)
	}
	defer func() { _ = d.Close() }()
	if err := d.Sync(); err != nil {
		return fmt.Errorf("transaction: syncing %s: %w", dir, err)
	}
	return nil
}
