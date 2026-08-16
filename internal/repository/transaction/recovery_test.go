package transaction

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/gitcli"
)

// This file is the ownership-and-recovery matrix for PruneAbandoned. Every
// scenario runs against a real Git topology so the six-point proof is exercised
// against actual worktree registrations, real HEADs, and real flocks — never a
// fake. The single invariant under test: PruneAbandoned removes ONLY candidates
// that pass ALL SIX ownership checks and leaves everything else byte-untouched
// with a verdict, never resetting a branch, deleting a ref, or globally pruning.

// recoveryEngine discovers the invocation repository and builds an Engine over the
// real git client and the pinned engine clock.
func recoveryEngine(t *testing.T, r *testRepos) (*Engine, *gitcli.Client, gitcli.Repository) {
	t.Helper()
	client, repo := r.discover(t)
	eng, err := NewEngine(client, engineClock)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	return eng, client, repo
}

// baseValidManifest returns a fully valid manifest for id whose repository identity
// is commonDir and whose target is refs/heads/main. Individual tests corrupt one
// field to exercise a single failing check while every other field stays canonical.
func baseValidManifest(id, commonDir string) manifest {
	stamp := txnTestClock.Now().UTC().Format(time.RFC3339)
	return manifest{
		Schema:        manifestSchemaVersion,
		TransactionID: id,
		CommonDir:     commonDir,
		Remote:        "origin",
		TargetRef:     "refs/heads/main",
		BaseCommit:    fixedBase,
		WorktreeRel:   worktreeDirName,
		Phase:         phaseAllocating,
		CreatedUTC:    stamp,
		UpdatedUTC:    stamp,
		PID:           os.Getpid(),
	}
}

// mkOwnedDir creates an owned-shape candidate directory (0700) under repo's
// transactions root and returns its path. It ensures the root exists first.
func mkOwnedDir(t *testing.T, repo gitcli.Repository, id string) string {
	t.Helper()
	root := transactionsRoot(repo)
	if err := ensureTransactionsRoot(root); err != nil {
		t.Fatalf("ensure transactions root: %v", err)
	}
	candRoot := filepath.Join(root, id)
	if err := mkdirMode(candRoot, txnDirMode); err != nil {
		t.Fatalf("mkdir candidate: %v", err)
	}
	return candRoot
}

// hexID returns a valid 32-hex transaction id built from a distinguishing prefix
// so table rows sort deterministically and read clearly in failures.
func hexID(prefix string) string {
	id := prefix
	for len(id) < 32 {
		id += "0"
	}
	return id[:32]
}

// hashTree returns a content hash of the entire tree rooted at path: every entry's
// relative name, type, and permission bits, plus regular-file contents and symlink
// targets. filepath.WalkDir never follows symlinks, so a symlinked root hashes as
// the link itself. A byte-untouched survival must produce an identical hash.
func hashTree(t *testing.T, path string) string {
	t.Helper()
	h := sha256.New()
	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, rerr := filepath.Rel(path, p)
		if rerr != nil {
			return rerr
		}
		info, ierr := os.Lstat(p)
		if ierr != nil {
			return ierr
		}
		fmt.Fprintf(h, "path=%s type=%v perm=%o\n", rel, info.Mode()&os.ModeType, info.Mode().Perm())
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			target, lerr := os.Readlink(p)
			if lerr != nil {
				return lerr
			}
			fmt.Fprintf(h, "link=%s\n", target)
		case info.Mode().IsRegular():
			data, derr := os.ReadFile(p)
			if derr != nil {
				return derr
			}
			h.Write(data)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("hash tree %s: %v", path, err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// abandonRegistered allocates a candidate, registers its detached worktree at
// commit, then releases the live lock — the exact shape of a normally abandoned
// candidate a crashed run left behind: valid manifest, released lock, registered
// worktree. It returns the candidate.
func abandonRegistered(t *testing.T, client *gitcli.Client, repo gitcli.Repository, r *testRepos, commit gitcli.ObjectID) *candidate {
	t.Helper()
	c, err := allocateCandidate(txnTestClock, repo, "origin", r.Target, fixedBase)
	if err != nil {
		t.Fatalf("allocateCandidate: %v", err)
	}
	if err := client.AddDetachedWorktree(context.Background(), repo, c.worktree, commit); err != nil {
		_ = c.live.release()
		t.Fatalf("AddDetachedWorktree: %v", err)
	}
	if err := c.live.release(); err != nil {
		t.Fatalf("release live lock: %v", err)
	}
	return c
}

// targetTip resolves the invocation's current target-ref commit — an ancestor of
// which is "already pushed" from recovery's point of view.
func targetTip(t *testing.T, r *testRepos) gitcli.ObjectID {
	t.Helper()
	return gitcli.ObjectID(hgitOut(t, r.Invocation, "rev-parse", string(r.Target)))
}

// pruneEntryFor returns the report entry for id, failing if absent.
func pruneEntryFor(t *testing.T, rep PruneReport, id string) PruneEntry {
	t.Helper()
	for _, e := range rep.Entries {
		if e.ID == id {
			return e
		}
	}
	t.Fatalf("no report entry for %q in %+v", id, rep.Entries)
	return PruneEntry{}
}

// TestPruneReportsLiveCandidatesUntouched proves that two concurrently active
// candidates in one clone — both holding their live locks — are both reported live
// and both survive. A held lock is the sole liveness signal.
func TestPruneReportsLiveCandidatesUntouched(t *testing.T) {
	requireGit(t)
	r := newMainModeRepos(t)
	eng, _, repo := recoveryEngine(t, r)

	c1, err := allocateCandidate(txnTestClock, repo, "origin", r.Target, fixedBase)
	if err != nil {
		t.Fatalf("allocate c1: %v", err)
	}
	defer func() { _ = c1.live.release() }()
	c2, err := allocateCandidate(txnTestClock, repo, "origin", r.Target, fixedBase)
	if err != nil {
		t.Fatalf("allocate c2: %v", err)
	}
	defer func() { _ = c2.live.release() }()

	rep, err := eng.PruneAbandoned(context.Background(), repo)
	if err != nil {
		t.Fatalf("PruneAbandoned: %v", err)
	}
	for _, c := range []*candidate{c1, c2} {
		e := pruneEntryFor(t, rep, c.id)
		if e.Verdict != verdictLive {
			t.Errorf("candidate %s verdict = %q, want live", c.id, e.Verdict)
		}
		if _, err := os.Stat(c.root); err != nil {
			t.Errorf("live candidate %s was removed: %v", c.id, err)
		}
	}
}

// TestPrunePrunesAbandonedCandidate proves a normally abandoned candidate is
// pruned — worktree deregistered, directory gone — and that Pushed is reported
// correctly for both an already-reachable (pushed) and a never-pushed commit.
func TestPrunePrunesAbandonedCandidate(t *testing.T) {
	requireGit(t)

	t.Run("pushed_commit_reachable", func(t *testing.T) {
		r := newMainModeRepos(t)
		eng, client, repo := recoveryEngine(t, r)
		// Worktree parked at the target tip: its commit is reachable from the target.
		c := abandonRegistered(t, client, repo, r, targetTip(t, r))

		rep, err := eng.PruneAbandoned(context.Background(), repo)
		if err != nil {
			t.Fatalf("PruneAbandoned: %v", err)
		}
		e := pruneEntryFor(t, rep, c.id)
		if e.Verdict != verdictPruned {
			t.Fatalf("verdict = %q, want pruned", e.Verdict)
		}
		if !e.Pushed {
			t.Errorf("Pushed = false, want true for a tip-reachable commit")
		}
		assertGoneAndDeregistered(t, client, repo, c)
	})

	t.Run("unpushed_commit_unreachable", func(t *testing.T) {
		r := newMainModeRepos(t)
		eng, client, repo := recoveryEngine(t, r)
		c := abandonRegistered(t, client, repo, r, targetTip(t, r))
		// Advance the worktree's detached HEAD to a local-only commit: unreachable
		// from the target, so the residue was never pushed.
		hgitOut(t, c.worktree, "commit", "--allow-empty", "-q", "--no-gpg-sign", "-m", "local only")

		rep, err := eng.PruneAbandoned(context.Background(), repo)
		if err != nil {
			t.Fatalf("PruneAbandoned: %v", err)
		}
		e := pruneEntryFor(t, rep, c.id)
		if e.Verdict != verdictPruned {
			t.Fatalf("verdict = %q, want pruned", e.Verdict)
		}
		if e.Pushed {
			t.Errorf("Pushed = true, want false for a local-only commit")
		}
		assertGoneAndDeregistered(t, client, repo, c)
	})
}

// assertGoneAndDeregistered proves the candidate directory is removed and its
// worktree is no longer registered with Git.
func assertGoneAndDeregistered(t *testing.T, client *gitcli.Client, repo gitcli.Repository, c *candidate) {
	t.Helper()
	if _, err := os.Stat(c.root); !os.IsNotExist(err) {
		t.Errorf("candidate root still present: %v", err)
	}
	infos, err := client.ListWorktrees(context.Background(), repo)
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}
	want := canonicalPath(c.worktree)
	for _, info := range infos {
		if canonicalPath(info.Path) == want {
			t.Errorf("worktree still registered after prune: %s", info.Path)
		}
	}
}

// TestPruneHeldLockBeatsAncientPIDAndTimestamp proves a held live lock defeats
// pruning even when the manifest advertises an ancient creation time and a dead
// PID: no age threshold overrides a held lock and PID liveness is never consulted.
func TestPruneHeldLockBeatsAncientPIDAndTimestamp(t *testing.T) {
	requireGit(t)
	r := newMainModeRepos(t)
	eng, _, repo := recoveryEngine(t, r)

	c, err := allocateCandidate(txnTestClock, repo, "origin", r.Target, fixedBase)
	if err != nil {
		t.Fatalf("allocateCandidate: %v", err)
	}
	defer func() { _ = c.live.release() }()

	// Rewrite the manifest with an ancient timestamp and an implausible PID while the
	// live lock is STILL held. Every other field stays valid, so only the held lock
	// can be what saves the candidate.
	m, err := c.readManifest()
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	m.CreatedUTC = "2000-01-01T00:00:00Z"
	m.UpdatedUTC = "2000-01-01T00:00:00Z"
	m.PID = 2147480000
	if err := writeManifestAtomic(c.root, m); err != nil {
		t.Fatalf("rewrite manifest: %v", err)
	}

	rep, err := eng.PruneAbandoned(context.Background(), repo)
	if err != nil {
		t.Fatalf("PruneAbandoned: %v", err)
	}
	e := pruneEntryFor(t, rep, c.id)
	if e.Verdict != verdictLive {
		t.Errorf("verdict = %q, want live (held lock must beat age/PID)", e.Verdict)
	}
	if _, err := os.Stat(c.root); err != nil {
		t.Errorf("held-lock candidate removed: %v", err)
	}
}

// TestPruneLeavesMalformedAndForeignByteUntouched drives every survival variant
// from the spec: each is reported with a verdict and left byte-for-byte identical.
func TestPruneLeavesMalformedAndForeignByteUntouched(t *testing.T) {
	requireGit(t)

	type variant struct {
		name    string
		id      string
		verdict string
		// build lays the on-disk state down under repo and returns the path whose
		// bytes must be identical afterward (candidate root or the entry itself).
		build func(t *testing.T, eng *Engine, client *gitcli.Client, repo gitcli.Repository) (id, hashPath string)
	}

	variants := []variant{
		{
			name: "missing_manifest",
			build: func(t *testing.T, _ *Engine, _ *gitcli.Client, repo gitcli.Repository) (string, string) {
				id := hexID("aaaa1")
				p := mkOwnedDir(t, repo, id)
				return id, p
			},
			verdict: verdictMalformed,
		},
		{
			name: "truncated_json",
			build: func(t *testing.T, _ *Engine, _ *gitcli.Client, repo gitcli.Repository) (string, string) {
				id := hexID("aaaa2")
				p := mkOwnedDir(t, repo, id)
				if err := os.WriteFile(filepath.Join(p, manifestFileName), []byte("{ \"schema\": 1"), txnFileMode); err != nil {
					t.Fatal(err)
				}
				return id, p
			},
			verdict: verdictMalformed,
		},
		{
			name: "unsupported_schema",
			build: func(t *testing.T, _ *Engine, _ *gitcli.Client, repo gitcli.Repository) (string, string) {
				id := hexID("aaaa3")
				p := mkOwnedDir(t, repo, id)
				m := baseValidManifest(id, repo.CommonDir)
				m.Schema = 99
				if err := writeManifestAtomic(p, m); err != nil {
					t.Fatal(err)
				}
				return id, p
			},
			verdict: verdictMalformed,
		},
		{
			name: "wrong_repository",
			build: func(t *testing.T, _ *Engine, _ *gitcli.Client, repo gitcli.Repository) (string, string) {
				id := hexID("aaaa4")
				p := mkOwnedDir(t, repo, id)
				// A canonical common dir belonging to a different repository.
				other := canonicalPath(t.TempDir())
				m := baseValidManifest(id, other)
				if err := writeManifestAtomic(p, m); err != nil {
					t.Fatal(err)
				}
				return id, p
			},
			verdict: verdictForeign,
		},
		{
			name: "worktree_rel_escapes",
			build: func(t *testing.T, _ *Engine, _ *gitcli.Client, repo gitcli.Repository) (string, string) {
				id := hexID("aaaa5")
				p := mkOwnedDir(t, repo, id)
				m := baseValidManifest(id, repo.CommonDir)
				m.WorktreeRel = "../../x"
				if err := writeManifestAtomic(p, m); err != nil {
					t.Fatal(err)
				}
				return id, p
			},
			verdict: verdictMalformed,
		},
		{
			name: "symlinked_root_to_foreign",
			build: func(t *testing.T, _ *Engine, _ *gitcli.Client, repo gitcli.Repository) (string, string) {
				id := hexID("aaaa6")
				root := transactionsRoot(repo)
				if err := ensureTransactionsRoot(root); err != nil {
					t.Fatal(err)
				}
				foreign := t.TempDir()
				if err := os.WriteFile(filepath.Join(foreign, "secret"), []byte("do not touch\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				link := filepath.Join(root, id)
				if err := os.Symlink(foreign, link); err != nil {
					t.Fatal(err)
				}
				return id, link
			},
			verdict: verdictForeign,
		},
		{
			name: "foreign_named_directory",
			build: func(t *testing.T, _ *Engine, _ *gitcli.Client, repo gitcli.Repository) (string, string) {
				root := transactionsRoot(repo)
				if err := ensureTransactionsRoot(root); err != nil {
					t.Fatal(err)
				}
				name := "not-32-hex"
				p := filepath.Join(root, name)
				if err := mkdirMode(p, txnDirMode); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(p, "keep"), []byte("keep\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				return name, p
			},
			verdict: verdictForeign,
		},
		{
			name: "ambiguous_registration",
			build: func(t *testing.T, _ *Engine, client *gitcli.Client, repo gitcli.Repository) (string, string) {
				// A valid, abandoned candidate — but Git has a registration inside its
				// tree at an unexpected path, so ownership is ambiguous.
				c, err := allocateCandidate(txnTestClock, repo, "origin", "refs/heads/main", fixedBase)
				if err != nil {
					t.Fatalf("allocate: %v", err)
				}
				rogue := filepath.Join(c.root, "rogue")
				if err := client.AddDetachedWorktree(context.Background(), repo, rogue, headCommit(t, repo)); err != nil {
					t.Fatalf("add rogue worktree: %v", err)
				}
				if err := c.live.release(); err != nil {
					t.Fatal(err)
				}
				return c.id, c.root
			},
			verdict: verdictForeign,
		},
	}

	for _, v := range variants {
		t.Run(v.name, func(t *testing.T) {
			r := newMainModeRepos(t)
			eng, client, repo := recoveryEngine(t, r)
			id, hp := v.build(t, eng, client, repo)

			before := hashTree(t, hp)
			rep, err := eng.PruneAbandoned(context.Background(), repo)
			if err != nil {
				t.Fatalf("PruneAbandoned: %v", err)
			}
			e := pruneEntryFor(t, rep, id)
			if e.Verdict != v.verdict {
				t.Errorf("verdict = %q, want %q (detail: %s)", e.Verdict, v.verdict, e.Detail)
			}
			if _, err := os.Lstat(hp); err != nil {
				t.Fatalf("survival entry vanished: %v", err)
			}
			if after := hashTree(t, hp); after != before {
				t.Errorf("survival entry was mutated: before %s, after %s", before, after)
			}
		})
	}
}

// headCommit resolves the invocation repo's current HEAD commit for a worktree add.
func headCommit(t *testing.T, repo gitcli.Repository) gitcli.ObjectID {
	t.Helper()
	return gitcli.ObjectID(hgitOut(t, repo.PrimaryWorktree, "rev-parse", "HEAD"))
}

// TestPruneRegistryLockSerializesAllocation proves the registry lock prevents
// PruneAbandoned from ever observing a half-published candidate directory. An
// allocator holds the registry lock across the whole mkdir→lock→manifest critical
// section; the prune, started while the lock is held, cannot list until the
// allocator releases — by which point the candidate is complete and live-locked, so
// it reports "live", never the "malformed" a half-published dir would yield. The
// ordering is enforced by channels and the lock, never a sleep.
func TestPruneRegistryLockSerializesAllocation(t *testing.T) {
	requireGit(t)
	r := newMainModeRepos(t)
	eng, _, repo := recoveryEngine(t, r)
	root := transactionsRoot(repo)

	inside := make(chan struct{})
	proceed := make(chan struct{})
	allocDone := make(chan struct{})
	var liveLock *fileLock
	var candID string

	go func() {
		_ = withRegistryLock(root, func() error {
			// Registry lock held. Announce, then wait until the prune is known to be
			// contending before publishing anything.
			close(inside)
			<-proceed
			id, err := newTransactionID()
			if err != nil {
				return err
			}
			candID = id
			candRoot := filepath.Join(root, id)
			if err := mkdirMode(candRoot, txnDirMode); err != nil {
				return err
			}
			if err := mkdirMode(filepath.Join(candRoot, hooksDirName), txnDirMode); err != nil {
				return err
			}
			lk, err := acquireLock(filepath.Join(candRoot, liveLockName), false)
			if err != nil {
				return err
			}
			liveLock = lk
			return writeManifestAtomic(candRoot, baseValidManifest(id, repo.CommonDir))
		})
		close(allocDone)
	}()

	<-inside
	reportCh := make(chan PruneReport, 1)
	errCh := make(chan error, 1)
	go func() {
		rep, err := eng.PruneAbandoned(context.Background(), repo)
		errCh <- err
		reportCh <- rep
	}()

	// The prune goroutine is now blocked on the registry lock (it started after the
	// allocator signalled `inside` and holds it). Release the allocator; only then
	// can the prune list — and it must see a complete, live-locked candidate.
	close(proceed)
	<-allocDone

	if err := <-errCh; err != nil {
		t.Fatalf("PruneAbandoned: %v", err)
	}
	rep := <-reportCh
	defer func() { _ = liveLock.release() }()

	if len(rep.Entries) != 1 {
		t.Fatalf("report entries = %+v, want exactly one", rep.Entries)
	}
	e := rep.Entries[0]
	if e.ID != candID || e.Verdict != verdictLive {
		t.Errorf("entry = %+v, want live entry for %s (a half-published dir would read malformed)", e, candID)
	}
}

// TestPruneReportsCleanupFailedOnForcedRemovalFailure proves that when Git cannot
// remove the registered worktree, the candidate is retained with a cleanup-failed
// verdict and a diagnostic rather than being force-deleted by pathname.
func TestPruneReportsCleanupFailedOnForcedRemovalFailure(t *testing.T) {
	requireGit(t)
	if os.Geteuid() == 0 {
		t.Skip("running as root: 000 permissions do not block removal")
	}
	r := newMainModeRepos(t)
	eng, client, repo := recoveryEngine(t, r)
	c := abandonRegistered(t, client, repo, r, targetTip(t, r))

	// Make the worktree directory unremovable, then restore it so t.TempDir cleanup
	// can proceed regardless of the test outcome.
	if err := os.Chmod(c.worktree, 0o000); err != nil {
		t.Fatalf("chmod worktree 000: %v", err)
	}
	defer func() { _ = os.Chmod(c.worktree, 0o700) }()

	rep, err := eng.PruneAbandoned(context.Background(), repo)
	if err != nil {
		t.Fatalf("PruneAbandoned: %v", err)
	}
	e := pruneEntryFor(t, rep, c.id)
	if e.Verdict != verdictCleanupFailed {
		t.Fatalf("verdict = %q, want cleanup-failed (detail: %s)", e.Verdict, e.Detail)
	}
	if _, err := os.Stat(c.root); err != nil {
		t.Errorf("cleanup-failed candidate was removed: %v", err)
	}
}

// TestPruneNeverGlobalPrunesOrTouchesUserCheckout proves that a full prune sweep
// leaves the invocation checkout byte-identical and never runs a global worktree
// prune: a second, unrelated but perfectly valid worktree registration planted in
// the repo still exists afterward.
func TestPruneNeverGlobalPrunesOrTouchesUserCheckout(t *testing.T) {
	requireGit(t)
	r := newMainModeRepos(t)
	eng, client, repo := recoveryEngine(t, r)

	// A legitimate, unrelated linked worktree the user keeps around.
	userWorktree := filepath.Join(t.TempDir(), "user-wt")
	if err := client.AddDetachedWorktree(context.Background(), repo, userWorktree, targetTip(t, r)); err != nil {
		t.Fatalf("add user worktree: %v", err)
	}

	// A prunable candidate to make the sweep actually do destructive work.
	c := abandonRegistered(t, client, repo, r, targetTip(t, r))

	// Snapshot the invocation checkout AFTER setup, BEFORE the sweep.
	beforeHead := hgitOut(t, r.Invocation, "rev-parse", "HEAD")
	beforeIndex := hgitOutRaw(t, r.Invocation, "ls-files", "--stage", "-z")
	beforeStatus := hgitOut(t, r.Invocation, "status", "--porcelain")

	rep, err := eng.PruneAbandoned(context.Background(), repo)
	if err != nil {
		t.Fatalf("PruneAbandoned: %v", err)
	}
	if e := pruneEntryFor(t, rep, c.id); e.Verdict != verdictPruned {
		t.Fatalf("candidate verdict = %q, want pruned", e.Verdict)
	}

	// The user's unrelated worktree is still registered — no global prune ran.
	infos, err := client.ListWorktrees(context.Background(), repo)
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}
	want := canonicalPath(userWorktree)
	found := false
	for _, info := range infos {
		if canonicalPath(info.Path) == want {
			found = true
		}
	}
	if !found {
		t.Errorf("unrelated user worktree was deregistered by the sweep")
	}

	// The invocation checkout is byte-identical.
	if got := hgitOut(t, r.Invocation, "rev-parse", "HEAD"); got != beforeHead {
		t.Errorf("HEAD moved: %s -> %s", beforeHead, got)
	}
	if got := hgitOutRaw(t, r.Invocation, "ls-files", "--stage", "-z"); got != beforeIndex {
		t.Errorf("index changed")
	}
	if got := hgitOut(t, r.Invocation, "status", "--porcelain"); got != beforeStatus {
		t.Errorf("working tree status changed:\n%s", got)
	}
}
