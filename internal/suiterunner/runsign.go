// This file owns the run sign: the crash-safe marker every suite run with a
// budget-state store puts up so that a solo budget re-check (soloConfirm) can
// tell whether another `docket development test` run of the same repository,
// in any worktree, was running while it measured.
//
// A sign is a regular file in the sign folder beside the budget-state store
// (runSignDir), held under an exclusive kernel advisory lock (flock, through
// process.TryExclusiveLock) for the whole run. The flock is the liveness proof:
// the kernel drops it when the owning process exits for any reason, SIGKILL and
// power loss included, so a dead run's leftover file probes free and can never
// look alive, and pid reuse cannot fool it the way a pid-in-a-file check can.
//
// Names are single-use (pid plus a random suffix) and only the owner ever
// creates its sign, so a checker may safely delete a visible sign it finds
// unlocked: nobody can re-open that name and lock it afterwards. Contrast
// internal/gatedrive's worktree lock, which never deletes its lock file because
// a fixed name can be re-opened by a concurrent locker after an unlink.
//
// A sign is never visible unlocked while its owner lives: the owner creates and
// locks it under a temporary name (runSignTempName) and only then renames it to
// its visible name (the lock stays with the inode). Checkers never count a
// temporary name, and sweep a stale unlocked one so a crash mid-publish cannot
// pile them up.
//
// Every failure here is advisory. A sign that cannot be put up costs only the
// other runs' view of this one; it never fails a run.
package suiterunner

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/danielhanold/docket/internal/process"
)

// runSignDirName is the sign folder's name, beside the budget-state store.
const runSignDirName = "development-test-runs"

// runSignName matches a visible sign; runSignTempName matches the temporary name
// a sign is created and locked under before it is published. Only regular files
// matching runSignName ever count as signs.
var (
	runSignName     = regexp.MustCompile(`^run-[0-9]+-[0-9a-f]{16}\.sign$`)
	runSignTempName = regexp.MustCompile(`^\.tmp-run-[0-9]+-[0-9a-f]{16}\.sign$`)
)

// runSignTempMaxAge is how old an unlocked temporary sign must be before a scan
// sweeps it as the leftover of a crash mid-publish. A package var so tests can
// stamp mtimes against it.
var runSignTempMaxAge = 10 * time.Minute

// runSignDir returns the sign folder for a budget-state store path: the
// directory of statePath joined with runSignDirName. An empty statePath (no
// store) has no sign folder and returns "".
func runSignDir(statePath string) string {
	if statePath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(statePath), runSignDirName)
}

// runSign is one run's held sign. f owns the flock: the caller must keep the
// *runSign reachable until takeDown, because a garbage-collected *os.File is
// closed, which silently drops the lock.
type runSign struct {
	f    *os.File
	path string
}

// putUpRunSign creates dir if needed and publishes a held sign naming repoRoot.
// The temporary file is locked BEFORE it is renamed to its visible name, so a
// visible sign is never unlocked while its owner lives. On any failure after
// the lock is taken it removes the temporary file and releases the lock.
func putUpRunSign(dir, repoRoot string) (*runSign, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return nil, fmt.Errorf("random sign suffix: %w", err)
	}
	pid := os.Getpid()
	name := fmt.Sprintf("run-%d-%s.sign", pid, hex.EncodeToString(suffix[:]))
	tmp := filepath.Join(dir, ".tmp-"+name)
	visible := filepath.Join(dir, name)

	f, busy, err := process.TryExclusiveLock(tmp)
	if busy {
		// Not ours to remove: some other descriptor holds a name we never used.
		return nil, fmt.Errorf("temporary sign %s is already locked", filepath.Base(tmp))
	}
	if err != nil {
		os.Remove(tmp)
		return nil, err
	}
	content := fmt.Sprintf("worktree=%s\npid=%d\nstarted=%s\n",
		repoRoot, pid, time.Now().UTC().Format(time.RFC3339))
	if _, err := f.WriteString(content); err != nil {
		os.Remove(tmp)
		f.Close()
		return nil, err
	}
	if runSignLockedHook != nil {
		runSignLockedHook(tmp)
	}
	if err := os.Rename(tmp, visible); err != nil {
		os.Remove(tmp)
		f.Close()
		return nil, err
	}
	return &runSign{f: f, path: visible}, nil
}

// runSignLockedHook is a test seam called, when non-nil, by putUpRunSign with
// the temporary sign's path after the lock is taken and before the rename to the
// visible name, so a test can check that nothing is visible yet and the
// temporary file is already held. Production leaves it nil.
var runSignLockedHook func(tmp string)

// Path is the sign's visible path, or "" for a nil sign.
func (s *runSign) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

// takeDown removes the visible sign FIRST and only then closes the file (which
// releases the lock), so a live run's visible sign is never unlocked. Nil-safe
// and idempotent.
func (s *runSign) takeDown() {
	if s == nil || s.f == nil {
		return
	}
	os.Remove(s.path)
	s.f.Close()
	s.f = nil
}

// peerScan is what one look at the sign folder found: the worktrees of the
// other runs whose signs were held, and whether any part of the folder could
// not be checked (can't tell). The zero value is a clean scan.
type peerScan struct {
	worktrees []string
	unknown   bool
}

// seen reports whether the scan found another run, or could not tell — either
// one means a solo reading must not be taken as uncontended.
func (p peerScan) seen() bool {
	return len(p.worktrees) > 0 || p.unknown
}

// merge combines two scans: the sorted, deduplicated union of worktrees and the
// OR of unknown.
func (p peerScan) merge(q peerScan) peerScan {
	out := peerScan{unknown: p.unknown || q.unknown}
	set := map[string]bool{}
	for _, w := range append(append([]string(nil), p.worktrees...), q.worktrees...) {
		if !set[w] {
			set[w] = true
			out.worktrees = append(out.worktrees, w)
		}
	}
	sort.Strings(out.worktrees)
	return out
}

// describe is the reason text a not-counted re-check line carries.
func (p peerScan) describe() string {
	running := "another suite was running (" + strings.Join(p.worktrees, ", ") + ")"
	switch {
	case len(p.worktrees) > 0 && p.unknown:
		return running + "; could not check for other suites"
	case len(p.worktrees) > 0:
		return running
	case p.unknown:
		return "the runner could not check for other suites"
	}
	return ""
}

// scanPeers looks once at the sign folder dir and reports the other runs whose
// signs are held there, excluding the sign at path self (the caller's own; ""
// excludes nothing).
//
//   - A missing folder is a clean scan; any other folder read error is can't
//     tell (unknown).
//   - Temporary names never count. An unlocked temporary file older than
//     runSignTempMaxAge is a crash mid-publish and is removed.
//   - Only regular files matching runSignName are signs; anything else is
//     ignored.
//   - A held sign names its run by its worktree= line, or by its file name when
//     that line is absent or unreadable — a held sign is a live run either way.
//   - An unlocked sign is a dead run (its flock died with it): ignored and
//     deleted, which is safe because sign names are single-use.
//   - A sign that vanished between listing and probing is ignored.
//   - A sign that cannot be probed, or whose entry cannot be stat'ed, is can't
//     tell (unknown).
//
// Accepted race: two scans probing the same dead sign at once, or a scan whose
// probe briefly holds a free sign while another scan probes it, can make the
// other scan read that sign as held. The worst outcome is a spurious "seen" —
// a re-check not counted that stays due — which is the safe direction.
func scanPeers(dir, self string) peerScan {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return peerScan{}
		}
		return peerScan{unknown: true}
	}
	var out peerScan
	for _, e := range entries {
		name := e.Name()
		path := filepath.Join(dir, name)
		if runSignTempName.MatchString(name) {
			sweepStaleTemp(e, path)
			continue // temporary names never count
		}
		if !runSignName.MatchString(name) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			if os.IsNotExist(err) {
				continue // vanished since the listing
			}
			out.unknown = true
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if path == self {
			continue
		}
		switch process.ProbeLock(path) {
		case process.LockProbeHeld:
			out.worktrees = append(out.worktrees, signWorktree(path))
		case process.LockProbeFree:
			os.Remove(path) // a dead run's sign
		case process.LockProbeMissing:
			// vanished between the listing and the probe
		default:
			out.unknown = true
		}
	}
	sort.Strings(out.worktrees)
	return out
}

// sweepStaleTemp removes an unlocked temporary sign older than
// runSignTempMaxAge. Age is checked before the lock is probed, so a fresh
// temporary file — one its owner may be about to lock — is never touched.
func sweepStaleTemp(e fs.DirEntry, path string) {
	info, err := e.Info()
	if err != nil || !info.Mode().IsRegular() || time.Since(info.ModTime()) <= runSignTempMaxAge {
		return
	}
	if process.ProbeLock(path) == process.LockProbeFree {
		os.Remove(path)
	}
}

// signWorktree reads a held sign's worktree= line, falling back to the sign's
// file name when the line is absent or the file cannot be read.
func signWorktree(path string) string {
	body, err := os.ReadFile(path)
	if err == nil {
		sc := bufio.NewScanner(bytes.NewReader(body))
		for sc.Scan() {
			if w, ok := strings.CutPrefix(sc.Text(), "worktree="); ok && w != "" {
				return w
			}
		}
	}
	return filepath.Base(path)
}
