//go:build integration

package transaction

import (
	"context"
	"github.com/danielhanold/docket/internal/testsupport"
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
)

// TestIntegrationTxnRecoveryPruneReportsLiveCandidatesUntouched proves that two concurrently active
// candidates in one clone — both holding their live locks — are both reported live
// and both survive. A held lock is the sole liveness signal.
func TestIntegrationTxnRecoveryPruneReportsLiveCandidatesUntouched(t *testing.T) {
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

// TestIntegrationTxnRecoveryPrunePrunesAbandonedCandidate proves a normally abandoned candidate is
// pruned — worktree deregistered, directory gone — and that Pushed is reported
// correctly for both an already-reachable (pushed) and a never-pushed commit.
func TestIntegrationTxnRecoveryPrunePrunesAbandonedCandidate(t *testing.T) {
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

// TestIntegrationTxnRecoveryPruneHeldLockBeatsAncientPIDAndTimestamp proves a held live lock defeats
// pruning even when the manifest advertises an ancient creation time and a dead
// PID: no age threshold overrides a held lock and PID liveness is never consulted.
func TestIntegrationTxnRecoveryPruneHeldLockBeatsAncientPIDAndTimestamp(t *testing.T) {
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

// TestIntegrationTxnRecoveryPruneLeavesMalformedAndForeignByteUntouched drives every survival variant
// from the spec: each is reported with a verdict and left byte-for-byte identical.
func TestIntegrationTxnRecoveryPruneLeavesMalformedAndForeignByteUntouched(t *testing.T) {
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
				other := canonicalPath(testsupport.TempDir(t))
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
				foreign := testsupport.TempDir(t)
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

// TestIntegrationTxnRecoveryPruneReportsCleanupFailedOnForcedRemovalFailure proves that when Git cannot
// remove the registered worktree, the candidate is retained with a cleanup-failed
// verdict and a diagnostic rather than being force-deleted by pathname.
func TestIntegrationTxnRecoveryPruneReportsCleanupFailedOnForcedRemovalFailure(t *testing.T) {
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

// TestIntegrationTxnRecoveryPruneNeverGlobalPrunesOrTouchesUserCheckout proves that a full prune sweep
// leaves the invocation checkout byte-identical and never runs a global worktree
// prune: a second, unrelated but perfectly valid worktree registration planted in
// the repo still exists afterward.
func TestIntegrationTxnRecoveryPruneNeverGlobalPrunesOrTouchesUserCheckout(t *testing.T) {
	requireGit(t)
	r := newMainModeRepos(t)
	eng, client, repo := recoveryEngine(t, r)

	// A legitimate, unrelated linked worktree the user keeps around.
	userWorktree := filepath.Join(testsupport.TempDir(t), "user-wt")
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
