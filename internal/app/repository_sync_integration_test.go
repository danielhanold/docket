//go:build integration

package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
)

// This file is change 0388 Task 4: the real-process repository.sync-integration
// tests driven through the PRODUCTION entry point app.RunRepositorySyncIntegration
// over real git executables and local bare origins. Unlike repository_sync_test.go
// (which proves the ladder over recording seams), these prove the live wiring:
// loadOperationalContext (discovery, pinned-blob config, legacy refusal, fetch-and-
// pin) composed with the gitcli adapter's WorktreeCheckoutState / ChangedPaths /
// IsAncestor / FastForwardWorktree. The topology fixtures (newDocketModeRepo,
// newLegacyRepo, writerAdvance, worktreeChecksum) are reused verbatim from
// status_git_test.go — a bare file origin plus writer and invocation clones — so a
// sync that read the wrong branch or moved the wrong worktree is observable.

// syncOne runs the production entry point for one invocation directory.
func syncOne(ctx context.Context, client *gitcli.Client, repoDir string) RepositorySyncResult {
	return RunRepositorySyncIntegration(ctx, SetupDeps{Git: client, RepoDir: repoDir})
}

// headOID returns the worktree HEAD commit id read by a direct git oracle.
func headOID(t *testing.T, dir string) string {
	t.Helper()
	return runGit(t, dir, "rev-parse", "HEAD")
}

// evalSym canonicalizes a path the way discovery does, so a PrimaryPath comparison
// survives the /var -> /private/var symlink on macOS temp dirs.
func evalSym(t *testing.T, p string) string {
	t.Helper()
	c, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatalf("EvalSymlinks(%s): %v", p, err)
	}
	return c
}

func TestIntegrationSyncIntegrationBranch(t *testing.T) {
	requireRealGit(t)
	ctx := context.Background()

	// (1) advance to the freshly fetched tip, then an idempotent repeat.
	t.Run("advance then already-current", func(t *testing.T) {
		r := newDocketModeRepo(t, nil, nil)
		client := newGitClient(t)
		before := headOID(t, r.invocation)
		tip := r.writerAdvance(t, "main", map[string]string{"feature.txt": "v1\n"})
		if tip == before {
			t.Fatalf("writerAdvance did not move origin main")
		}

		res := syncOne(ctx, client, r.invocation)
		if res.Result != ResultApplied || res.Disposition != SyncDispAdvanced {
			t.Fatalf("first sync = %s/%s, want applied/advanced (%s)", res.Result, res.Disposition, res.HumanText())
		}
		if res.BeforeOID != before || res.TargetOID != tip || res.AfterOID != tip {
			t.Fatalf("oids = before %s target %s after %s; want %s/%s/%s", res.BeforeOID, res.TargetOID, res.AfterOID, before, tip, tip)
		}
		if got := headOID(t, r.invocation); got != tip {
			t.Fatalf("primary HEAD = %s, want fast-forwarded to %s", got, tip)
		}

		// Idempotent repeat: nothing to do.
		res2 := syncOne(ctx, client, r.invocation)
		if res2.Result != ResultNoOp || res2.Disposition != SyncDispAlreadyCurrent {
			t.Fatalf("second sync = %s/%s, want no-op/already-current", res2.Result, res2.Disposition)
		}
		if got := headOID(t, r.invocation); got != tip {
			t.Fatalf("primary HEAD moved on the idempotent repeat: %s", got)
		}
	})

	// (2) a configured integration branch that differs from the default branch:
	// only that branch's tip is synced, config is read from the DEFAULT branch blob.
	t.Run("configured integration branch differs from default", func(t *testing.T) {
		r := newDocketModeRepo(t, map[string]string{".docket.yml": "integration_branch: release\n"}, nil)
		client := newGitClient(t)
		// Create release on origin, then check the invocation out onto it.
		r.writerAdvance(t, "release", map[string]string{"rel.txt": "v1\n"})
		runGit(t, r.invocation, "fetch", "-q", "origin")
		runGit(t, r.invocation, "checkout", "-q", "-b", "release", "origin/release")
		mainBefore := runGit(t, r.invocation, "rev-parse", "refs/heads/main")
		relBefore := headOID(t, r.invocation)
		relTip := r.writerAdvance(t, "release", map[string]string{"rel.txt": "v2\n"})
		if relTip == relBefore {
			t.Fatalf("release did not advance")
		}

		res := syncOne(ctx, client, r.invocation)
		if res.Result != ResultApplied || res.Disposition != SyncDispAdvanced {
			t.Fatalf("sync = %s/%s, want applied/advanced (%s)", res.Result, res.Disposition, res.HumanText())
		}
		if res.IntegrationBranch != "release" {
			t.Fatalf("integration branch = %q, want release", res.IntegrationBranch)
		}
		if got := headOID(t, r.invocation); got != relTip {
			t.Fatalf("release HEAD = %s, want %s", got, relTip)
		}
		// The default branch was never the target: local main is untouched.
		if got := runGit(t, r.invocation, "rev-parse", "refs/heads/main"); got != mainBefore {
			t.Fatalf("local main moved (%s != %s); only the configured integration branch may sync", got, mainBefore)
		}
	})

	// (3) invocation from a linked worktree (a metadata- or feature-style checkout):
	// the PRIMARY worktree advances, the invocation worktree never moves.
	t.Run("invocation from a linked worktree advances only the primary", func(t *testing.T) {
		for _, tc := range []struct{ name, rel, wtBranch string }{
			{"metadata worktree", ".docket", "wt-metadata"},
			{"feature worktree", ".worktrees/feat-x", "wt-feature"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				// A real docket repo gitignores its linked-worktree roots, so a
				// worktree nested under the primary is not seen as untracked dirt.
				r := newDocketModeRepo(t, map[string]string{".gitignore": ".docket/\n.worktrees/\n"}, nil)
				client := newGitClient(t)
				wtPath := filepath.Join(r.invocation, tc.rel)
				// A linked worktree on its own branch (its branch identity is
				// incidental — the property is that discovery resolves the PRIMARY).
				runGit(t, r.invocation, "worktree", "add", "-b", tc.wtBranch, wtPath)
				wtHeadBefore := headOID(t, wtPath)

				primaryBefore := headOID(t, r.invocation)
				tip := r.writerAdvance(t, "main", map[string]string{"feature.txt": "v1\n"})
				if tip == primaryBefore {
					t.Fatalf("origin main did not advance")
				}

				res := syncOne(ctx, client, wtPath)
				if res.Result != ResultApplied || res.Disposition != SyncDispAdvanced {
					t.Fatalf("sync from %s = %s/%s, want applied/advanced (%s)", tc.rel, res.Result, res.Disposition, res.HumanText())
				}
				if res.PrimaryPath != evalSym(t, r.invocation) {
					t.Fatalf("PrimaryPath = %q, want the primary %q, not the invocation worktree", res.PrimaryPath, evalSym(t, r.invocation))
				}
				if got := headOID(t, r.invocation); got != tip {
					t.Fatalf("primary HEAD = %s, want %s", got, tip)
				}
				// The invocation worktree did not move.
				if got := headOID(t, wtPath); got != wtHeadBefore {
					t.Fatalf("linked worktree HEAD moved: %s != %s", got, wtHeadBefore)
				}
				if got := runGit(t, wtPath, "symbolic-ref", "--short", "HEAD"); got != tc.wtBranch {
					t.Fatalf("linked worktree branch = %q, want %q", got, tc.wtBranch)
				}
			})
		}
	})

	// (4) every deliberate safety skip leaves the checkout byte-identical; an
	// ignored-only file is NOT dirt and does not block the advance.
	t.Run("skips preserve state; ignored files are not dirt", func(t *testing.T) {
		// setup mutates the invocation clone and returns whether origin main was
		// advanced (so a clean tree WOULD have advanced — proving the skip is real).
		cases := []struct {
			name       string
			extraMain  map[string]string
			advance    bool // advance origin main before the sync
			setup      func(t *testing.T, r *gitRepo)
			wantDisp   string
			wantReason string
			wantResult Result
			advances   bool // the sync mutates (ignored-only file case)
		}{
			{
				name:       "dirty tracked file",
				advance:    true,
				setup:      func(t *testing.T, r *gitRepo) { writeRepoFile(t, r.invocation, "main.go", "package main // edited\n") },
				wantDisp:   SyncDispSkipped,
				wantReason: ReasonSyncDirtyWorktree,
				wantResult: ResultNoOp,
			},
			{
				name:       "non-ignored untracked file",
				advance:    true,
				setup:      func(t *testing.T, r *gitRepo) { writeRepoFile(t, r.invocation, "scratch.txt", "loose\n") },
				wantDisp:   SyncDispSkipped,
				wantReason: ReasonSyncDirtyWorktree,
				wantResult: ResultNoOp,
			},
			{
				name:       "detached HEAD",
				advance:    true,
				setup:      func(t *testing.T, r *gitRepo) { runGit(t, r.invocation, "checkout", "-q", "--detach", "HEAD") },
				wantDisp:   SyncDispSkipped,
				wantReason: ReasonSyncDetachedHead,
				wantResult: ResultNoOp,
			},
			{
				name:       "checked-out other branch",
				advance:    true,
				setup:      func(t *testing.T, r *gitRepo) { runGit(t, r.invocation, "checkout", "-q", "-b", "sidebranch") },
				wantDisp:   SyncDispSkipped,
				wantReason: ReasonSyncOtherBranch,
				wantResult: ResultNoOp,
			},
			{
				name:    "in-progress merge",
				advance: true,
				setup: func(t *testing.T, r *gitRepo) {
					writeRepoFile(t, r.invocation, "conflict.txt", "local\n")
					runGit(t, r.invocation, "add", "--", "conflict.txt")
					runGit(t, r.invocation, "commit", "-q", "-m", "local side")
					runGit(t, r.invocation, "checkout", "-q", "-b", "other", "HEAD~1")
					writeRepoFile(t, r.invocation, "conflict.txt", "other\n")
					runGit(t, r.invocation, "add", "--", "conflict.txt")
					runGit(t, r.invocation, "commit", "-q", "-m", "other side")
					runGit(t, r.invocation, "checkout", "-q", "main")
					_, _ = tryGit(r.invocation, "merge", "other") // conflicts; ignore exit
				},
				wantDisp:   SyncDispSkipped,
				wantReason: ReasonSyncOperationInProgress,
				wantResult: ResultNoOp,
			},
			{
				name:    "local ahead",
				advance: false, // origin stays put; the local branch is strictly ahead
				setup: func(t *testing.T, r *gitRepo) {
					writeRepoFile(t, r.invocation, "ahead.txt", "local commit\n")
					runGit(t, r.invocation, "add", "--", "ahead.txt")
					runGit(t, r.invocation, "commit", "-q", "-m", "local ahead")
				},
				wantDisp:   SyncDispSkipped,
				wantReason: ReasonSyncLocalAhead,
				wantResult: ResultNoOp,
			},
			{
				name:    "diverged",
				advance: true, // origin advances one way, the local branch another
				setup: func(t *testing.T, r *gitRepo) {
					writeRepoFile(t, r.invocation, "local-only.txt", "local commit\n")
					runGit(t, r.invocation, "add", "--", "local-only.txt")
					runGit(t, r.invocation, "commit", "-q", "-m", "local diverge")
				},
				wantDisp:   SyncDispSkipped,
				wantReason: ReasonSyncDiverged,
				wantResult: ResultNoOp,
			},
			{
				name:       "ignored-only file is not dirt",
				extraMain:  map[string]string{".gitignore": "*.ignored\n"},
				advance:    true,
				setup:      func(t *testing.T, r *gitRepo) { writeRepoFile(t, r.invocation, "scratch.ignored", "ignored bytes\n") },
				wantDisp:   SyncDispAdvanced,
				wantResult: ResultApplied,
				advances:   true,
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				r := newDocketModeRepo(t, tc.extraMain, nil)
				client := newGitClient(t)
				var tip string
				if tc.advance {
					tip = r.writerAdvance(t, "main", map[string]string{"feature.txt": "v1\n"})
				}
				tc.setup(t, r)

				// Snapshot the checkout exactly as the sync will first see it.
				headBefore := headOID(t, r.invocation)
				sumBefore := worktreeChecksum(t, r.invocation)

				res := syncOne(ctx, client, r.invocation)

				if res.Disposition != tc.wantDisp {
					t.Fatalf("disposition = %q, want %q (%s)", res.Disposition, tc.wantDisp, res.HumanText())
				}
				if tc.wantReason != "" && res.Reason != tc.wantReason {
					t.Fatalf("reason = %q, want %q", res.Reason, tc.wantReason)
				}
				if res.Result != tc.wantResult {
					t.Fatalf("result = %q, want %q", res.Result, tc.wantResult)
				}
				if tc.advances {
					// The ignored file survives, and the branch actually moved.
					if got := headOID(t, r.invocation); got != tip {
						t.Fatalf("advance HEAD = %s, want %s", got, tip)
					}
					if _, err := os.Stat(filepath.Join(r.invocation, "scratch.ignored")); err != nil {
						t.Fatalf("ignored file must survive the advance: %v", err)
					}
					return
				}
				// A skip leaves branch, HEAD, index, and files byte-identical.
				if got := headOID(t, r.invocation); got != headBefore {
					t.Fatalf("skip moved HEAD: %s != %s", got, headBefore)
				}
				if got := worktreeChecksum(t, r.invocation); !equalChecksums(got, sumBefore) {
					t.Fatalf("skip perturbed the worktree contents")
				}
			})
		}
	})

	// (5) a failed fetch with a stale remote-tracking tip still present: the sync
	// fails and never launders the stale tip into a synchronization; primary untouched.
	t.Run("failed fetch never uses the stale remote-tracking tip", func(t *testing.T) {
		r := newDocketModeRepo(t, nil, nil)
		client := newGitClient(t)
		// origin/main remote-tracking ref is present from the clone; break origin so
		// the loader's fresh fetch/probe cannot succeed.
		before := headOID(t, r.invocation)
		if _, err := tryGit(r.invocation, "rev-parse", "--verify", "refs/remotes/origin/main"); err != nil {
			t.Fatalf("fixture must carry a stale origin/main remote-tracking tip: %v", err)
		}
		runGit(t, r.invocation, "remote", "set-url", "origin", filepath.Join(r.root, "does-not-exist.git"))

		res := syncOne(ctx, client, r.invocation)
		if res.Result == ResultApplied || res.Result == ResultNoOp {
			t.Fatalf("a broken origin must not read as success, got %s/%s", res.Result, res.Disposition)
		}
		if res.Disposition != SyncDispFailed {
			t.Fatalf("disposition = %q, want failed", res.Disposition)
		}
		if got := headOID(t, r.invocation); got != before {
			t.Fatalf("primary HEAD moved on a failed fetch: %s != %s", got, before)
		}
	})

	// (6) a legacy (non-docket) topology is refused through the classifier's own
	// typed contract — a classifier reason, never a sync-minted one.
	t.Run("legacy topology refusal uses the classifier reason", func(t *testing.T) {
		r := newLegacyRepo(t, map[string]string{"docs/changes/active/0001-x.md": changeRecord(1, "x", "X")})
		client := newGitClient(t)
		before := headOID(t, r.invocation)

		res := syncOne(ctx, client, r.invocation)
		if res.Disposition != SyncDispRefused {
			t.Fatalf("disposition = %q, want refused (%s)", res.Disposition, res.HumanText())
		}
		if res.Result != ResultInvalidState {
			t.Fatalf("result = %q, want invalid-state", res.Result)
		}
		if res.Reason != ReasonLegacyRepository {
			t.Fatalf("reason = %q, want the classifier reason %q", res.Reason, ReasonLegacyRepository)
		}
		if got := headOID(t, r.invocation); got != before {
			t.Fatalf("a refusal moved HEAD: %s != %s", got, before)
		}
	})
}

// TestIntegrationSyncChangedPathsExcludesIgnored proves the dirty seam's contract
// directly: gitcli.ChangedPaths (the seam RunRepositorySyncIntegration wires into
// `dirty`) reports tracked/index/non-ignored-untracked dirt but NOT ignored files,
// so an ignored-only working tree is clean (status.go passes --untracked-files=all
// WITHOUT --ignored). A non-ignored untracked file is dirt; an ignored one is not.
func TestIntegrationSyncChangedPathsExcludesIgnored(t *testing.T) {
	requireRealGit(t)
	ctx := context.Background()
	r := newDocketModeRepo(t, map[string]string{".gitignore": "*.ignored\n"}, nil)
	client := newGitClient(t)

	// A pristine clone (only the committed .gitignore) is clean.
	changes, err := client.ChangedPaths(ctx, r.invocation)
	if err != nil {
		t.Fatalf("ChangedPaths clean: %v", err)
	}
	if len(changes) != 0 {
		t.Fatalf("clean clone reports %d changed paths: %+v", len(changes), changes)
	}

	// An ignored-only file leaves the tree clean.
	writeRepoFile(t, r.invocation, "scratch.ignored", "ignored\n")
	changes, err = client.ChangedPaths(ctx, r.invocation)
	if err != nil {
		t.Fatalf("ChangedPaths ignored: %v", err)
	}
	if len(changes) != 0 {
		t.Fatalf("an ignored file must not count as dirt, got %+v", changes)
	}

	// A non-ignored untracked file IS dirt.
	writeRepoFile(t, r.invocation, "scratch.txt", "loose\n")
	changes, err = client.ChangedPaths(ctx, r.invocation)
	if err != nil {
		t.Fatalf("ChangedPaths untracked: %v", err)
	}
	if len(changes) != 1 || string(changes[0].Path) != "scratch.txt" {
		t.Fatalf("want exactly the non-ignored untracked path, got %+v", changes)
	}
}
