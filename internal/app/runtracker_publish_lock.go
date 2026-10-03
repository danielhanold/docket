// Publisher liveness for the run mutation journal (change 0494). A publication
// (pr.publish / workspace.publish) holds a per-entry kernel lock from BEFORE its
// admitted entry is written until AFTER its outcome is written (admitWorkflowMutation).
// The kernel frees the lock when the publisher exits or dies, so an admitted entry
// whose lock reads free has no publisher left to write its outcome.
// classifyAdmittedMutation is the ONE rule every journal reader applies: run.cancel
// teardown (reconcileRunTeardown), terminal repair and resume
// (verifyTerminalRunQuiescence), and the success closeout (accountCompletionMutations).
// An entry blocks only while its publisher may still be running. A provably gone
// publisher, or an uncertain entry (written only by the publisher's own callback, so
// its publisher has returned), is the informational mutation-abandoned:<op>. Every
// unprovable case keeps the existing blocking mutation-pending:<op>. The lock rules
// are ADR-0132's: release by close only (never LOCK_UN), never delete a lock file,
// and only try a lock, never wait on it.
package app

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/danielhanold/docket/internal/process"
)

// The journal finding prefixes; the operation key follows the colon.
const (
	findingMutationPending   = "mutation-pending:"
	findingMutationAbandoned = "mutation-abandoned:"
)

const (
	publishLockPrefix = "publish-"
	publishLockSuffix = ".lock"
)

// Test seams. Production values never change.
var (
	publishLockMintToken = runToken
	publishLockAcquire   = process.TryExclusiveLock
	publishLockProbe     = process.ProbeLock
)

// validPublishLockToken reports whether tok has runToken's exact shape: 32
// lowercase hex characters. A journal token of any other shape (a corrupt or
// hand-edited record) is never joined into a path.
func validPublishLockToken(tok string) bool {
	if len(tok) != 32 {
		return false
	}
	for i := 0; i < len(tok); i++ {
		c := tok[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// publishLockPath is the lock file for token in the run-key directory dir. It
// refuses (ok=false) an empty dir or a malformed token.
func publishLockPath(dir, token string) (string, bool) {
	if dir == "" || !validPublishLockToken(token) {
		return "", false
	}
	return filepath.Join(dir, publishLockPrefix+token+publishLockSuffix), true
}

// heldPublishLocks retains every live publish lock's file, keyed by its token, so
// the lock is held until releasePublishLock closes it or the process exits. An
// *os.File left unreachable is closed by its finalizer, which would free the lock
// at the next GC and make a live publisher that dropped its done callback read as
// gone (mutation-abandoned). Retention here fails that case safe: the entry keeps
// blocking as mutation-pending, exactly as before change 0494. Only
// releasePublishLock removes an entry.
var heldPublishLocks = struct {
	sync.Mutex
	files map[string]*os.File
}{files: map[string]*os.File{}}

// acquirePublishLock mints a fresh token and takes its lock in dir without waiting,
// retaining the lock in heldPublishLocks until releasePublishLock(token). Any
// failure (no token, no dir, an I/O error, or a busy answer on a fresh token)
// returns "". The caller then journals the entry without a token and publishes
// anyway: a lock failure never refuses a publish.
func acquirePublishLock(dir string) string {
	tok, err := publishLockMintToken()
	if err != nil {
		return ""
	}
	path, ok := publishLockPath(dir, tok)
	if !ok {
		return ""
	}
	f, busy, err := publishLockAcquire(path)
	if err != nil || busy || f == nil {
		if f != nil {
			_ = f.Close()
		}
		return ""
	}
	heldPublishLocks.Lock()
	defer heldPublishLocks.Unlock()
	if _, dup := heldPublishLocks.files[tok]; dup {
		// A token already held in this process cannot be a fresh one; never
		// overwrite (and so orphan) the retained file.
		_ = f.Close()
		return ""
	}
	heldPublishLocks.files[tok] = f
	return tok
}

// releasePublishLock releases token's publish lock by closing its file (never
// LOCK_UN, and the lock file is never deleted: ADR-0132). An empty, unknown, or
// already-released token is a no-op, so release is idempotent.
func releasePublishLock(token string) {
	if token == "" {
		return
	}
	heldPublishLocks.Lock()
	f, ok := heldPublishLocks.files[token]
	delete(heldPublishLocks.files, token)
	heldPublishLocks.Unlock()
	if ok && f != nil {
		_ = f.Close()
	}
}

// publisherGone reports whether m is an admitted entry whose publisher provably
// exited without writing an outcome: a well-formed LockToken whose lock file
// exists and reads free. A held lock, a missing file, a probe error, an empty dir,
// or no or a malformed token is NOT proof (false).
func publisherGone(dir string, m AdmittedMutation) bool {
	if m.Status != mutationStatusAdmitted {
		return false
	}
	path, ok := publishLockPath(dir, m.LockToken)
	if !ok {
		return false
	}
	return publishLockProbe(path) == process.LockProbeFree
}

// classifyAdmittedMutation is the single journal rule (spec section 2). completed:
// accounted, no finding. uncertain, or admitted with a provably gone publisher:
// accounted, mutation-abandoned:<op>. Anything else (a held lock, a missing lock
// file, a probe error, no token, or an unknown status): blocks, mutation-pending:<op>.
// It never returns an error a caller could turn into a refusal.
func classifyAdmittedMutation(dir string, m AdmittedMutation) (blocks bool, finding string) {
	switch {
	case m.Status == mutationStatusCompleted:
		return false, ""
	case m.Status == mutationStatusUncertain:
		return false, findingMutationAbandoned + m.OpKey
	case publisherGone(dir, m):
		return false, findingMutationAbandoned + m.OpKey
	default:
		return true, findingMutationPending + m.OpKey
	}
}

// accountAdmittedMutations applies classifyAdmittedMutation to every entry in order
// and reports whether any blocks, plus the findings in journal order. It never
// re-reads the journal; accountAdmittedMutationsReloading does.
func accountAdmittedMutations(dir string, muts []AdmittedMutation) (blocked bool, findings []string) {
	return accountAdmittedMutationsReloading(dir, muts, nil)
}

// accountAdmittedMutationsReloading is accountAdmittedMutations with a re-read of
// the journal for one race. muts was read before the locks are probed, so a
// publisher may write completed and release its lock in between: its entry reads
// admitted with a free lock. Before reporting such an entry mutation-abandoned it
// calls reload (at most once) and re-checks the same entry, matched by index, op
// key, and lock token. Now completed, it is accounted with no finding. Anything
// else (still admitted, now uncertain, no match, a nil reload, or a reload error)
// keeps today's mutation-abandoned. The re-check only drops an informational
// finding; it never changes blocked.
func accountAdmittedMutationsReloading(dir string, muts []AdmittedMutation, reload func() ([]AdmittedMutation, error)) (blocked bool, findings []string) {
	var fresh []AdmittedMutation
	reloaded := false
	for i, m := range muts {
		b, f := classifyAdmittedMutation(dir, m)
		if f != "" && m.Status == mutationStatusAdmitted && reload != nil {
			// Only the gone-publisher path yields a finding for an admitted entry.
			if !reloaded {
				reloaded = true
				if r, err := reload(); err == nil {
					fresh = r
				}
			}
			if i < len(fresh) && fresh[i].OpKey == m.OpKey && fresh[i].LockToken == m.LockToken &&
				fresh[i].Status == mutationStatusCompleted {
				f = ""
			}
		}
		if f != "" {
			findings = append(findings, f)
		}
		if b {
			blocked = true
		}
	}
	return blocked, findings
}

// accountRunMutations accounts runKey's journal entries muts (already read from its
// run record) through accountAdmittedMutationsReloading, re-reading the run record
// for the read-then-probe race. It is what every run-tracker journal reader calls.
func accountRunMutations(repoDir, runKey string, muts []AdmittedMutation) (bool, []string) {
	reload := func() ([]AdmittedMutation, error) {
		ep, _, err := LoadRunRecord(repoDir, runKey)
		if err != nil {
			return nil, err
		}
		return ep.AdmittedMutations, nil
	}
	return accountAdmittedMutationsReloading(runJournalDir(repoDir, runKey), muts, reload)
}

// runJournalDir resolves the run-key directory the publish locks of runKey live in.
// An unresolvable directory is "", under which every token-bearing admitted entry
// stays blocking (publishLockPath refuses an empty dir).
func runJournalDir(repoDir, runKey string) string {
	dir, err := runKeyDir(repoDir, runKey, "journal-classify")
	if err != nil {
		return ""
	}
	return dir
}
