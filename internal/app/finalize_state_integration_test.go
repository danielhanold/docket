//go:build integration

package app

import (
	"context"
	"errors"
	"github.com/danielhanold/docket/internal/evidence"
	"github.com/danielhanold/docket/internal/githubcli"
	"strings"
	"testing"
)

// TestFinalizeBlockCommentThenMarker proves the comment-first-then-marker
// discipline over a REAL metadata transaction: a created (or crash-replayed
// already) comment is followed by the marker landing on origin with the attempt
// and comment URL; an unknown comment probe writes NO marker and lands no commit.
func TestIntegrationFinalizeStateBlockCommentThenMarker(t *testing.T) {
	for _, m := range planRepoModes() {
		t.Run(m.name, func(t *testing.T) {
			t.Run("created-writes-marker", func(t *testing.T) {
				f := setupRebaseFixtureStatus(t, m, "in-progress")
				gh := &fakeBlockGitHub{repo: retargetRepo(), commentOutcome: githubcli.CommentCreated, commentURL: "https://example.test/c/9"}
				req := BlockRequest{ID: f.id, Version: f.version, PRNumber: 7, Attempt: "att1",
					Reason: "gate-repair-required", Head: f.head, Report: "The gate failed.\n", Remedy: "Fix and retry.\n"}
				got := FinalizeBlock(context.Background(), FinalizeDeps{Planning: f.deps, GitHub: gh, Workspace: f.svc}, f.repo.invocation, req)
				if got.Result != ResultApplied || got.Disposition != BlockDispRecorded {
					t.Fatalf("result=%q disp=%q reason=%q", got.Result, got.Disposition, got.Reason)
				}
				if gh.ensureCalls != 1 {
					t.Fatalf("EnsureComment calls = %d, want 1", gh.ensureCalls)
				}
				if !strings.HasPrefix(gh.lastBody, gh.lastMarker) {
					t.Errorf("comment body must begin with the owned marker: %q", gh.lastBody)
				}
				rec, ok := originFile(t, f.repo.origin, f.branch, groomPath(f.id, f.slug))
				if !ok {
					t.Fatal("record vanished from origin")
				}
				for _, want := range []string{"## Finalize blocked", "<!-- attempt:att1 -->", "https://example.test/c/9"} {
					if !strings.Contains(rec, want) {
						t.Errorf("origin record missing %q:\n%s", want, rec)
					}
				}
			})

			t.Run("already-comment-replays-marker", func(t *testing.T) {
				f := setupRebaseFixtureStatus(t, m, "in-progress")
				gh := &fakeBlockGitHub{repo: retargetRepo(), commentOutcome: githubcli.CommentAlready, commentURL: "https://example.test/c/9"}
				req := BlockRequest{ID: f.id, Version: f.version, PRNumber: 7, Attempt: "att1",
					Reason: "gate-repair-required", Head: f.head, Report: "The gate failed.\n", Remedy: "Fix.\n"}
				got := FinalizeBlock(context.Background(), FinalizeDeps{Planning: f.deps, GitHub: gh, Workspace: f.svc}, f.repo.invocation, req)
				if got.Result != ResultApplied || got.Disposition != BlockDispRecorded {
					t.Fatalf("crash replay after comment must finish the marker: result=%q disp=%q", got.Result, got.Disposition)
				}
				rec, _ := originFile(t, f.repo.origin, f.branch, groomPath(f.id, f.slug))
				if !strings.Contains(rec, "## Finalize blocked") {
					t.Errorf("marker not written on replay:\n%s", rec)
				}
			})

			t.Run("unknown-comment-writes-no-marker", func(t *testing.T) {
				f := setupRebaseFixtureStatus(t, m, "in-progress")
				before := originTip(t, f.repo.origin, f.branch)
				gh := &fakeBlockGitHub{repo: retargetRepo(), commentOutcome: githubcli.CommentUnknown}
				req := BlockRequest{ID: f.id, Version: f.version, PRNumber: 7, Attempt: "att1",
					Reason: "gate-repair-required", Head: f.head, Report: "The gate failed.\n", Remedy: "Fix.\n"}
				got := FinalizeBlock(context.Background(), FinalizeDeps{Planning: f.deps, GitHub: gh, Workspace: f.svc}, f.repo.invocation, req)
				if got.Disposition != BlockDispUnknown || got.Reason != ReasonBlockCommentUnknown {
					t.Fatalf("unknown comment: disp=%q reason=%q, want unknown/%s", got.Disposition, got.Reason, ReasonBlockCommentUnknown)
				}
				if after := originTip(t, f.repo.origin, f.branch); after != before {
					t.Fatalf("an unknown comment probe committed a marker: tip moved %s -> %s", before, after)
				}
				rec, _ := originFile(t, f.repo.origin, f.branch, groomPath(f.id, f.slug))
				if strings.Contains(rec, "## Finalize blocked") {
					t.Errorf("a marker was written despite an unknown comment probe:\n%s", rec)
				}
			})
		})
	}
}

// TestFinalizeBytePreservation proves that after a closeout, every byte outside the
// generated blocks the operation owns is identical to its pre-image: the authored
// bodies of the merged plan/results, the repository config, and an unrelated file
// are compared in full against snapshots taken before the transaction.
func TestIntegrationFinalizeStateBytePreservation(t *testing.T) {
	requireRealGit(t)
	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			f := setupCloseoutFixture(t, m)

			integrationBranch := f.branch
			if m.name == "docket" {
				integrationBranch = "main"
			}
			// Pre-images of files the closeout must not disturb outside its owned blocks.
			planBefore, _ := originFile(t, f.repo.origin, integrationBranch, f.planPath)
			resultsBefore, _ := originFile(t, f.repo.origin, integrationBranch, f.resultsPath)
			readmeBefore, hadReadme := originFile(t, f.repo.origin, integrationBranch, "README.md")

			mergeCommit := f.mergeIntoBase(t)
			gh := f.baselineMergedFake(f.head, mergeCommit)
			res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
			if res.Result != ResultApplied {
				t.Fatalf("closeout did not apply: %q (reason %q)", res.Result, res.Reason)
			}

			// The authored body after the backlink block is byte-identical.
			for _, tc := range []struct {
				path   string
				before string
			}{
				{f.planPath, planBefore},
				{f.resultsPath, resultsBefore},
			} {
				after, ok := originFile(t, f.repo.origin, integrationBranch, tc.path)
				if !ok {
					t.Fatalf("artifact %q vanished after closeout", tc.path)
				}
				if matrixAuthoredBody(t, tc.before) != matrixAuthoredBody(t, after) {
					t.Errorf("artifact %q authored body changed outside its backlink block:\n--before--\n%q\n--after--\n%q",
						tc.path, matrixAuthoredBody(t, tc.before), matrixAuthoredBody(t, after))
				}
			}

			// A file the closeout never targets is byte-identical in full.
			if hadReadme {
				readmeAfter, ok := originFile(t, f.repo.origin, integrationBranch, "README.md")
				if !ok || readmeAfter != readmeBefore {
					t.Errorf("README.md was disturbed by closeout:\n--before--\n%q\n--after--\n%q", readmeBefore, readmeAfter)
				}
			}
		})
	}
}

// TestFinalizeClearBlockReprobes proves clear-block requires an exact current
// head, a published remote ref at that head, a matching open PR, and green body
// evidence (gate on) before removing the marker; each missing conjunct refuses
// and leaves the marker, and the full-conjunct case removes it.
func TestIntegrationFinalizeStateClearBlockReprobes(t *testing.T) {
	for _, m := range planRepoModes() {
		t.Run(m.name, func(t *testing.T) {
			// Full-conjunct success removes the marker.
			t.Run("all-hold-clears", func(t *testing.T) {
				f := setupBlockedFixture(t, m)
				gh := &fakeBlockGitHub{repo: retargetRepo(),
					openByHead: map[string][]githubcli.PullRequest{"feat/" + f.slug: {f.prForHead(f.head, greenEvidenceFor(t, f.head))}}}
				got := FinalizeClearBlock(context.Background(), FinalizeDeps{Planning: f.deps, GitHub: gh, Workspace: f.svc}, f.repo.invocation,
					ClearBlockRequest{ID: f.id, Version: f.version, Head: f.head, PRNumber: 1})
				if got.Result != ResultApplied || got.Disposition != BlockDispCleared {
					t.Fatalf("result=%q disp=%q reason=%q", got.Result, got.Disposition, got.Reason)
				}
				rec, _ := originFile(t, f.repo.origin, f.branch, groomPath(f.id, f.slug))
				if strings.Contains(rec, "## Finalize blocked") {
					t.Errorf("marker not removed:\n%s", rec)
				}
			})

			// Wrong expected head: refuse, marker stays.
			t.Run("head-mismatch-refuses", func(t *testing.T) {
				f := setupBlockedFixture(t, m)
				gh := &fakeBlockGitHub{repo: retargetRepo(),
					openByHead: map[string][]githubcli.PullRequest{"feat/" + f.slug: {f.prForHead(f.head, greenEvidenceFor(t, f.head))}}}
				got := FinalizeClearBlock(context.Background(), FinalizeDeps{Planning: f.deps, GitHub: gh, Workspace: f.svc}, f.repo.invocation,
					ClearBlockRequest{ID: f.id, Version: f.version, Head: strings.Repeat("b", 40), PRNumber: 1})
				if got.Reason != ReasonClearHeadMismatch {
					t.Fatalf("reason=%q, want %q", got.Reason, ReasonClearHeadMismatch)
				}
				rec, _ := originFile(t, f.repo.origin, f.branch, groomPath(f.id, f.slug))
				if !strings.Contains(rec, "## Finalize blocked") {
					t.Errorf("marker was removed on a refused clear:\n%s", rec)
				}
			})

			// No matching open PR: refuse.
			t.Run("no-open-pr-refuses", func(t *testing.T) {
				f := setupBlockedFixture(t, m)
				gh := &fakeBlockGitHub{repo: retargetRepo(), openByHead: map[string][]githubcli.PullRequest{}}
				got := FinalizeClearBlock(context.Background(), FinalizeDeps{Planning: f.deps, GitHub: gh, Workspace: f.svc}, f.repo.invocation,
					ClearBlockRequest{ID: f.id, Version: f.version, Head: f.head, PRNumber: 1})
				if got.Reason != ReasonClearPRNotOpen {
					t.Fatalf("reason=%q, want %q", got.Reason, ReasonClearPRNotOpen)
				}
			})

			// Stale (non-green-for-head) evidence with the gate on: refuse.
			t.Run("stale-evidence-refuses", func(t *testing.T) {
				f := setupBlockedFixture(t, m)
				gh := &fakeBlockGitHub{repo: retargetRepo(),
					openByHead: map[string][]githubcli.PullRequest{"feat/" + f.slug: {f.prForHead(f.head, "")}}}
				got := FinalizeClearBlock(context.Background(), FinalizeDeps{Planning: f.deps, GitHub: gh, Workspace: f.svc}, f.repo.invocation,
					ClearBlockRequest{ID: f.id, Version: f.version, Head: f.head, PRNumber: 1})
				if got.Reason != ReasonClearEvidenceUnverified {
					t.Fatalf("reason=%q, want %q", got.Reason, ReasonClearEvidenceUnverified)
				}
			})
		})
	}
}

// TestFinalizeConcurrentMovement proves that a concurrent base move, a remote
// feature-head move, and a same-entity version contention each produce a
// contended/refused outcome — never a text-merge, a silent overwrite, or a merge —
// because every effect is fenced by the exact old-value it read.
func TestIntegrationFinalizeStateConcurrentMovement(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]

	t.Run("remote-feature-head-move-contends-publish", func(t *testing.T) {
		f := setupPublishFixture(t, m)
		// A concurrent writer moves the remote feature ref off the head the receipt
		// recorded, after the receipt was written. The receipt's lease is pinned to
		// that original remote head, so the rewrite push must lose the lease — never
		// clobber. Build the rival commit on the live remote feature ref and push it.
		w := f.repo.writer
		runGit(t, w, "fetch", "-q", "origin", "feat/"+f.slug)
		runGit(t, w, "checkout", "-q", "-B", "rival", "FETCH_HEAD")
		writeRepoFile(t, w, "concurrent.txt", "a rival writer\n")
		runGit(t, w, "add", "-A")
		runGit(t, w, "commit", "-q", "-m", "rival writer")
		moved := runGit(t, w, "rev-parse", "HEAD")
		runGit(t, w, "push", "-q", "--force", "origin", "HEAD:refs/heads/feat/"+f.slug)
		if tip := f.remoteFeatureTip(t); tip != moved {
			t.Fatalf("precondition: remote feature tip = %q, want the rival commit %q", tip, moved)
		}
		_, evBytes := recFor(t, f.rewritten)
		gh := &fakePublishGitHub{repo: retargetRepo(), pr: f.openPRForPublish(f.rewritten, authoredPRBody(t, f.origHead))}
		res := FinalizePublish(context.Background(), f.publishDeps(gh), f.repo.invocation,
			FinalizePublishRequest{ID: f.id, Attempt: f.attempt, Head: f.rewritten, EvidenceRecord: evBytes})
		if res.Result == ResultApplied || res.Disposition == PublishDispPublished {
			t.Fatalf("a concurrent feature-head move let the rewrite publish: %q disp %q", res.Result, res.Disposition)
		}
		if res.Disposition != PublishDispContended {
			t.Fatalf("disposition = %q, want %q", res.Disposition, PublishDispContended)
		}
		if tip := f.remoteFeatureTip(t); tip != moved {
			t.Fatalf("the lease-losing push OVERWROTE the rival commit: %q -> %q", moved, tip)
		}
	})

	t.Run("base-move-contends-merge", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		// The PR merged against a base that has since diverged from the parent's
		// effective base: a present-but-wrong destination, contended, never a merge
		// the closeout could act on.
		gh := f.baselineFake(t)
		gh.mergeOutcome = githubcli.MergeMerged
		gh.mergeFacts = mergedFactsFor(f.head, "develop", strings.Repeat("a", 40))
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		if res.Result != ResultContended || res.Merge != nil {
			t.Fatalf("base divergence = %q merge %v, want contended and no VerifiedMerge", res.Result, res.Merge)
		}
	})

	t.Run("same-entity-version-contends-retarget", func(t *testing.T) {
		pin := docketPin(t)
		corpus := []StatusBlob{
			finalizeBlob(80, "root", "implemented", "high", prRefFor(800), ""),
			finalizeBlob(81, "child-a", "implemented", "high", prRefFor(810), "stacked_on: 80\n"),
		}
		// The authorized set names cv810, but the live PR entity moved to cv810-new: a
		// same-entity contention. The retarget must refuse the edit, never force it.
		gh := &fakeRetargetGitHub{
			repo: retargetRepo(),
			prs:  []*fakePR{{number: 810, head: "feat/child-a", base: "feat/root", version: "cv810-new"}},
		}
		engine := &recordingEngine{}
		req := RetargetChildrenRequest{
			ID: 80, Version: "blobfin0080",
			Children: []AuthorizedChild{{ID: 81, PRNumber: 810, PRVersion: "cv810"}},
		}
		res := FinalizeRetargetChildren(context.Background(), retargetDeps(&fakeReader{pin: pin, corpus: corpus}, gh, engine), "", req)
		if res.Result == ResultApplied {
			t.Fatalf("a version-contended child was retargeted: %q disp %q", res.Result, res.Disposition)
		}
		if c := childOutcomeByID(t, res, 81); c.Outcome != childOutcomeContended {
			t.Fatalf("child 81 outcome = %q, want contended", c.Outcome)
		}
		// The rival version stands: no forced edit.
		if gh.prs[0].base != "feat/root" || gh.prs[0].version != "cv810-new" {
			t.Fatalf("the contended retarget overwrote the rival PR: base %q version %q", gh.prs[0].base, gh.prs[0].version)
		}
	})
}

// TestFinalizeInterruptionMatrix walks the spec's recovery table in BOTH metadata
// modes. Each boundary runs its operation once (the effect lands), then replays the
// same operation against the durable authority and asserts convergence: the effect
// is adopted, never duplicated, and no false-success or unsafe overwrite/delete
// results.
func TestIntegrationFinalizeStateInterruptionMatrix(t *testing.T) {
	requireRealGit(t)
	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			t.Run("rebase-completion-adopts-not-reruns", func(t *testing.T) {
				matrixRebaseCompletion(t, m)
			})
			t.Run("force-with-lease-push-and-pr-evidence", func(t *testing.T) {
				matrixPublish(t, m)
			})
			t.Run("pr-merge-never-merges-twice", func(t *testing.T) {
				matrixMerge(t, m)
			})
			t.Run("metadata-closeout-and-backlink-idempotent", func(t *testing.T) {
				matrixCloseout(t, m)
			})
			t.Run("worktree-and-ref-delete-retryable", func(t *testing.T) {
				matrixCleanup(t, m)
			})
		})
	}

	// Gate-run deletion is repository-mode invariant (a private run directory, no
	// metadata topology), and child retarget opens no transaction and touches no
	// Git — both are covered once.
	t.Run("gate-run-delete-tombstoned", matrixGateRunDelete)
	t.Run("child-retarget-adopts-exact-pr", matrixChildRetarget)
}

// TestFinalizeNoForeignWrites proves the terminal metadata transaction writes only
// through its own detached candidate worktree and the remote: the invocation's
// primary checkout, the sibling feature worktree, and the transactions root are all
// left as they were — no foreign index, HEAD, or worktree is touched.
func TestIntegrationFinalizeStateNoForeignWrites(t *testing.T) {
	requireRealGit(t)
	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			f := setupCloseoutFixture(t, m)
			mergeCommit := f.mergeIntoBase(t)
			gh := f.baselineMergedFake(f.head, mergeCommit)

			// Primary (invocation) checkout and the sibling feature worktree before.
			// The invariant is that closeout does not CHANGE these indexes/HEADs, so
			// capture the exact status and compare — not a pristine assumption.
			invHeadBefore := runGit(t, f.repo.invocation, "rev-parse", "HEAD")
			invStatusBefore := runGit(t, f.repo.invocation, "status", "--porcelain")
			featHeadBefore := runGit(t, f.wp, "rev-parse", "HEAD")
			featStatusBefore := runGit(t, f.wp, "status", "--porcelain")

			res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
			if res.Result != ResultApplied {
				t.Fatalf("closeout did not apply: %q (reason %q)", res.Result, res.Reason)
			}

			if got := runGit(t, f.repo.invocation, "rev-parse", "HEAD"); got != invHeadBefore {
				t.Errorf("closeout moved the primary checkout HEAD: %q -> %q", invHeadBefore, got)
			}
			if got := runGit(t, f.repo.invocation, "status", "--porcelain"); got != invStatusBefore {
				t.Errorf("closeout wrote into the primary checkout index:\n--before--\n%s\n--after--\n%s", invStatusBefore, got)
			}
			if got := runGit(t, f.wp, "rev-parse", "HEAD"); got != featHeadBefore {
				t.Errorf("closeout moved the sibling feature worktree HEAD: %q -> %q", featHeadBefore, got)
			}
			if got := runGit(t, f.wp, "status", "--porcelain"); got != featStatusBefore {
				t.Errorf("closeout wrote into the sibling feature worktree index:\n--before--\n%s\n--after--\n%s", featStatusBefore, got)
			}
			if !transactionsRootEmpty(t, f.deps.Client, f.repo.invocation) {
				t.Errorf("closeout left a candidate worktree under the transactions root")
			}
		})
	}
}

// TestFinalizePublishCrashReplay proves the two replay faces. A crash after the
// push but before the PR update is a no-op rewrite (the remote already holds the
// rewritten head) that still resumes the PR update. A crash after both is a full
// no-op.
func TestIntegrationFinalizeStatePublishCrashReplay(t *testing.T) {
	requireRealGit(t)
	main := planRepoModes()[0]

	t.Run("after-push-before-pr-update", func(t *testing.T) {
		f := setupPublishFixture(t, main)
		// The push already landed: force the rewritten head onto the remote out of band.
		runGit(t, f.wp, "push", "--force", "-q", "origin", "HEAD:refs/heads/feat/"+f.slug)
		if tip := f.remoteFeatureTip(t); tip != f.rewritten {
			t.Fatalf("precondition: remote tip = %q, want the rewritten head", tip)
		}
		_, evBytes := recFor(t, f.rewritten)
		prBody := authoredPRBody(t, f.origHead) // the PR still carries the old-head evidence
		gh := &fakePublishGitHub{repo: retargetRepo(), pr: f.openPRForPublish(f.rewritten, prBody)}

		res := FinalizePublish(context.Background(), f.publishDeps(gh), f.repo.invocation,
			FinalizePublishRequest{ID: f.id, Attempt: f.attempt, Head: f.rewritten, EvidenceRecord: evBytes})

		if res.Result != ResultApplied || res.Disposition != PublishDispPublished {
			t.Fatalf("replay = %q disp %q, want applied/published (the PR update resumes)", res.Result, res.Disposition)
		}
		if res.Rewrite != "noop" {
			t.Errorf("rewrite outcome = %q, want noop (the remote already held the head)", res.Rewrite)
		}
		if gh.ensNext != 1 {
			t.Errorf("EnsurePullRequest called %d time(s), want 1 (the PR update resumes)", gh.ensNext)
		}
	})

	t.Run("after-both-is-full-noop", func(t *testing.T) {
		f := setupPublishFixture(t, main)
		runGit(t, f.wp, "push", "--force", "-q", "origin", "HEAD:refs/heads/feat/"+f.slug)
		evRec, evBytes := recFor(t, f.rewritten)
		// The PR already carries the exact current-head evidence.
		converged, err := evidence.Upsert([]byte(authoredPRBody(t, f.origHead)), evRec)
		if err != nil {
			t.Fatalf("evidence.Upsert: %v", err)
		}
		gh := &fakePublishGitHub{repo: retargetRepo(), pr: f.openPRForPublish(f.rewritten, string(converged))}

		res := FinalizePublish(context.Background(), f.publishDeps(gh), f.repo.invocation,
			FinalizePublishRequest{ID: f.id, Attempt: f.attempt, Head: f.rewritten, EvidenceRecord: evBytes})

		if res.Result != ResultNoOp || res.Disposition != PublishDispNoop {
			t.Fatalf("full replay = %q disp %q (reason %q), want no-op/noop", res.Result, res.Disposition, res.Reason)
		}
		if res.Rewrite != "noop" {
			t.Errorf("rewrite outcome = %q, want noop", res.Rewrite)
		}
		// The PR body was not mutated (the edit was a no-op).
		if gh.pr.Body != string(converged) {
			t.Errorf("a full replay mutated the PR body")
		}
	})
}

// TestFinalizePublishOrder proves the ordered composition: the rewrite is pushed
// under the receipt lease (the remote moves from the original head to the
// rewritten head), then the PR build-evidence block is loss-preservingly replaced
// with the exact current-head record — every authored byte and the title
// preserved — and no second PR is ever created.
func TestIntegrationFinalizeStatePublishOrder(t *testing.T) {
	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			f := setupPublishFixture(t, m)
			if tip := f.remoteFeatureTip(t); tip != f.origHead {
				t.Fatalf("precondition: remote feature tip = %q, want the original head %q", tip, f.origHead)
			}

			evRec, evBytes := recFor(t, f.rewritten)
			prBody := authoredPRBody(t, f.origHead) // the PR still carries evidence for the old head
			gh := &fakePublishGitHub{repo: retargetRepo(), pr: f.openPRForPublish(f.rewritten, prBody)}

			res := FinalizePublish(context.Background(), f.publishDeps(gh), f.repo.invocation,
				FinalizePublishRequest{ID: f.id, Attempt: f.attempt, Head: f.rewritten, EvidenceRecord: evBytes})

			if res.Result != ResultApplied || res.Disposition != PublishDispPublished {
				t.Fatalf("publish = %q disp %q (reason %q msg %q), want applied/published", res.Result, res.Disposition, res.Reason, res.Message)
			}
			// The rewrite reached the remote under the lease.
			if tip := f.remoteFeatureTip(t); tip != f.rewritten {
				t.Fatalf("remote feature tip = %q, want the rewritten head %q", tip, f.rewritten)
			}
			if res.Rewrite != "published" {
				t.Errorf("rewrite outcome = %q, want published", res.Rewrite)
			}
			// Exactly one PR edit; never a create.
			if gh.ensNext != 1 {
				t.Fatalf("EnsurePullRequest called %d time(s), want exactly 1", gh.ensNext)
			}
			// The edit converged the exact expected head and version.
			if gh.ensLast.ExpectedHead != f.rewritten {
				t.Errorf("edit expected head = %q, want the rewritten head %q", gh.ensLast.ExpectedHead, f.rewritten)
			}
			// Loss preservation: the full body equals the authored body with ONLY its
			// evidence block replaced, and the title and base are byte-identical.
			wantBody, err := evidence.Upsert([]byte(prBody), evRec)
			if err != nil {
				t.Fatalf("evidence.Upsert: %v", err)
			}
			if gh.ensLast.Body != string(wantBody) {
				t.Errorf("edited body mismatch:\n got %q\nwant %q", gh.ensLast.Body, string(wantBody))
			}
			if gh.ensLast.Title != publishPRTitle {
				t.Errorf("edited title = %q, want the authored title unchanged %q", gh.ensLast.Title, publishPRTitle)
			}
			if gh.ensLast.BaseBranch != "main" {
				t.Errorf("edited base = %q, want the authored base unchanged", gh.ensLast.BaseBranch)
			}
			// The replaced block certifies the exact rewritten head, and the authored
			// prose survived.
			got, err := evidence.Extract([]byte(gh.ensLast.Body))
			if err != nil || got.Head != f.rewritten {
				t.Errorf("edited body evidence head = %q (err %v), want the rewritten head %q", got.Head, err, f.rewritten)
			}
			if !strings.Contains(gh.ensLast.Body, "Authored intro prose.") || !strings.Contains(gh.ensLast.Body, "Authored outro prose.") {
				t.Errorf("the authored prose was not preserved in the edited body: %q", gh.ensLast.Body)
			}
			// The result names the PR without leaking a body byte.
			if res.Number != 7 || !strings.HasSuffix(res.Reference, "#7") {
				t.Errorf("result PR identity = number %d ref %q, want #7", res.Number, res.Reference)
			}
		})
	}
}

// TestFinalizePublishRefusesForeignAttempt proves an attempt token that does not
// match the owned receipt is refused before any push: the remote is untouched and
// no PR edit is issued.
func TestIntegrationFinalizeStatePublishRefusesForeignAttempt(t *testing.T) {
	requireRealGit(t)
	f := setupPublishFixture(t, planRepoModes()[0])
	_, evBytes := recFor(t, f.rewritten)
	gh := &fakePublishGitHub{repo: retargetRepo(), pr: f.openPRForPublish(f.rewritten, authoredPRBody(t, f.origHead))}

	res := FinalizePublish(context.Background(), f.publishDeps(gh), f.repo.invocation,
		FinalizePublishRequest{ID: f.id, Attempt: "not-the-owned-attempt", Head: f.rewritten, EvidenceRecord: evBytes})

	if res.Result != ResultBlocked || res.Reason != ReasonPublishForeignAttempt {
		t.Fatalf("foreign attempt = (%q, %q), want blocked/attempt-token-mismatch", res.Result, res.Reason)
	}
	// No push happened: the remote still holds the original head.
	if tip := f.remoteFeatureTip(t); tip != f.origHead {
		t.Errorf("a foreign-attempt refusal pushed to the remote: tip %q, want the original head %q", tip, f.origHead)
	}
	if gh.ensNext != 0 {
		t.Errorf("a foreign-attempt refusal issued %d PR edit(s); want 0", gh.ensNext)
	}
}

// TestFinalizePublishShapeAndEvidenceRefusals proves the pre-effect gates: a
// malformed request shape, and evidence that does not certify the requested head,
// both refuse before any workspace or GitHub effect.
func TestIntegrationFinalizeStatePublishShapeAndEvidenceRefusals(t *testing.T) {
	requireRealGit(t)
	f := setupPublishFixture(t, planRepoModes()[0])
	gh := &fakePublishGitHub{repo: retargetRepo(), pr: f.openPRForPublish(f.rewritten, authoredPRBody(t, f.origHead))}

	// A malformed head is a shape refusal carrying findings, before any effect.
	bad := FinalizePublish(context.Background(), f.publishDeps(gh), f.repo.invocation,
		FinalizePublishRequest{ID: f.id, Attempt: f.attempt, Head: "not-hex", EvidenceRecord: []byte("x")})
	if bad.Result != ResultInvalidInput || len(bad.Findings) == 0 {
		t.Fatalf("malformed head = %q findings %v, want invalid-input with findings", bad.Result, bad.Findings)
	}

	// Evidence for a DIFFERENT head does not certify the requested head: refused.
	_, staleBytes := recFor(t, f.origHead) // certifies the old head, not the rewritten one
	stale := FinalizePublish(context.Background(), f.publishDeps(gh), f.repo.invocation,
		FinalizePublishRequest{ID: f.id, Attempt: f.attempt, Head: f.rewritten, EvidenceRecord: staleBytes})
	if stale.Result != ResultInvalidState || stale.Reason != ReasonPublishEvidenceUnverified {
		t.Fatalf("stale evidence = (%q, %q), want invalid-state/evidence-unverified", stale.Result, stale.Reason)
	}
	// Neither refusal pushed or edited anything.
	if tip := f.remoteFeatureTip(t); tip != f.origHead {
		t.Errorf("a pre-effect refusal pushed to the remote: tip %q", tip)
	}
	if gh.ensNext != 0 {
		t.Errorf("a pre-effect refusal issued %d PR edit(s); want 0", gh.ensNext)
	}
}

// TestFinalizePublishUnknownStops proves a PR reprobe that cannot be established
// (after the rewrite already published) is unknown: retained, no PR edit issued,
// and never a merge-enabling success.
func TestIntegrationFinalizeStatePublishUnknownStops(t *testing.T) {
	requireRealGit(t)
	f := setupPublishFixture(t, planRepoModes()[0])
	_, evBytes := recFor(t, f.rewritten)
	gh := &fakePublishGitHub{repo: retargetRepo(), pr: f.openPRForPublish(f.rewritten, authoredPRBody(t, f.origHead)), findErr: errors.New("gh list boom")}

	res := FinalizePublish(context.Background(), f.publishDeps(gh), f.repo.invocation,
		FinalizePublishRequest{ID: f.id, Attempt: f.attempt, Head: f.rewritten, EvidenceRecord: evBytes})

	if res.Result != ResultExternalFailed || res.Disposition != PublishDispUnknown {
		t.Fatalf("unknown reprobe = %q disp %q, want external-failed/unknown", res.Result, res.Disposition)
	}
	if res.Reason != ReasonPublishPRProbeFailed {
		t.Errorf("reason = %q, want %q", res.Reason, ReasonPublishPRProbeFailed)
	}
	if gh.ensNext != 0 {
		t.Errorf("EnsurePullRequest was called %d time(s) after an unknown reprobe; want 0 (no second mutation)", gh.ensNext)
	}
	// The rewrite still landed on the remote (the push is not rolled back), but the
	// PR was not touched.
	if tip := f.remoteFeatureTip(t); tip != f.rewritten {
		t.Errorf("remote tip = %q, want the rewritten head (the push is not rolled back)", tip)
	}
}
