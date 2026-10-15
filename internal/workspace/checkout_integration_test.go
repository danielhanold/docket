//go:build integration

package workspace

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/testsupport"
)

// These tests prove Checkout against real temporary repositories: the remote
// feature branch is pushed from the writer clone, and the primary clone under
// test adopts it into <primary>/.worktrees/<slug>. They reuse the Prepare
// harness (harness_integration_test.go, prepare_integration_test.go).

const coFeature = "feat/" + prepSlug

// writerCommit adds one commit on branch in the writer clone, pushes it, and
// returns the new commit.
func (r *wsRepos) writerCommit(t *testing.T, branch, file string) gitcli.ObjectID {
	t.Helper()
	gitOut(t, r.Writer, "checkout", "-q", branch)
	writeWorktreeFile(t, r.Writer, file, "writer "+file+"\n")
	gitOut(t, r.Writer, "add", "-A")
	gitOut(t, r.Writer, "commit", "-q", "-m", file)
	gitOut(t, r.Writer, "push", "-q", "origin", branch)
	h := gitcli.ObjectID(gitOut(t, r.Writer, "rev-parse", "HEAD"))
	gitOut(t, r.Writer, "checkout", "-q", "main")
	return h
}

func (r *wsRepos) mainTip(t *testing.T) gitcli.ObjectID {
	t.Helper()
	return gitcli.ObjectID(gitOut(t, r.Writer, "rev-parse", "main"))
}

// localFeatureAt creates the local feature branch in the primary at commit at.
func (r *wsRepos) localFeatureAt(t *testing.T, at gitcli.ObjectID) {
	t.Helper()
	gitOut(t, r.Primary, "fetch", "-q", "origin", coFeature)
	gitOut(t, r.Primary, "branch", coFeature, string(at))
}

func commitIn(t *testing.T, dir, file string) gitcli.ObjectID {
	t.Helper()
	writeWorktreeFile(t, dir, file, file+"\n")
	gitOut(t, dir, "add", "-A")
	gitOut(t, dir, "commit", "-q", "-m", file)
	return gitcli.ObjectID(gitOut(t, dir, "rev-parse", "HEAD"))
}

// localCommit adds a commit to the local feature branch that the remote never saw.
func (r *wsRepos) localCommit(t *testing.T) {
	t.Helper()
	tmp := filepath.Join(filepath.Dir(r.Primary), "tmp-wt")
	gitOut(t, r.Primary, "worktree", "add", "-q", tmp, coFeature)
	commitIn(t, tmp, "local-only.txt")
	gitOut(t, r.Primary, "worktree", "remove", tmp)
}

// foreignAt attaches the local feature branch to a worktree docket did not
// make, returning its canonical path.
func (r *wsRepos) foreignAt(t *testing.T, path string) string {
	t.Helper()
	gitOut(t, r.Primary, "worktree", "add", "-q", path, coFeature)
	cp, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return cp
}

func (r *wsRepos) elsewhere() string { return filepath.Join(filepath.Dir(r.Primary), "foreign") }

// conflictedRebase leaves dir stopped mid-rebase at a conflict: main gains a
// conflicting edit of pushBranch's feature file and dir rebases onto it.
func (r *wsRepos) conflictedRebase(t *testing.T, dir string) {
	t.Helper()
	gitOut(t, r.Writer, "checkout", "-q", "main")
	writeWorktreeFile(t, r.Writer, "on-feat-"+prepSlug+".txt", "conflicting base edit\n") // pushBranch's feature file
	gitOut(t, r.Writer, "add", "-A")
	gitOut(t, r.Writer, "commit", "-q", "-m", "conflict")
	gitOut(t, r.Writer, "push", "-q", "origin", "main")
	gitOut(t, dir, "fetch", "-q", "origin", "main")
	if _, err := gitTry(dir, "rebase", "origin/main"); err == nil {
		t.Fatal("fixture: the rebase did not stop at a conflict")
	}
}

func targetFor(t *testing.T, branch string) Target {
	t.Helper()
	tgt, err := NewTarget(7, prepSlug, freshTarget(t, 7).Base, branch)
	if err != nil {
		t.Fatal(err)
	}
	return tgt
}

func coReq(repo gitcli.Repository, tgt Target, head gitcli.ObjectID, meta string) CheckoutRequest {
	return CheckoutRequest{Repository: repo, Remote: "origin", Target: tgt, ExpectedHead: head, MetadataWorktree: meta}
}

func runCheckout(t *testing.T, svc *Service, repo gitcli.Repository, tgt Target, head gitcli.ObjectID, meta string) CheckoutResult {
	t.Helper()
	res, err := svc.Checkout(context.Background(), coReq(repo, tgt, head, meta))
	if err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	return res
}

// assertAttached proves the full adopted-workspace postcondition: attached at
// the canonical path on the feature ref at want, a ready manifest whose base is
// contained in want, and Inspect agreeing the workspace is ready.
func assertAttached(t *testing.T, svc *Service, repo gitcli.Repository, tgt Target, res CheckoutResult, want gitcli.ObjectID) {
	t.Helper()
	ws := wsPathOf(repo)
	if res.Disposition != CheckoutAttached || res.Path != ws || res.HeadCommit != want {
		t.Fatalf("= %q %s@%s (%s: %s); want attached %s@%s", res.Disposition, res.Path, res.HeadCommit, res.Reason, res.Message, ws, want)
	}
	if got := symbolicHead(t, ws); got != string(tgt.FeatureRef) {
		t.Errorf("HEAD = %q; want %q", got, tgt.FeatureRef)
	}
	m, present, err := loadManifest(metaDirOf(repo, tgt))
	if err != nil || !present || m.Phase != PhaseReady {
		t.Fatalf("manifest %v %q %v; want ready", present, m.Phase, err)
	}
	if _, err := gitTry(repo.PrimaryWorktree, "merge-base", "--is-ancestor", string(m.BaseCommit), string(want)); err != nil {
		t.Errorf("base %s not contained in %s", m.BaseCommit, want)
	}
	if insp, err := svc.Inspect(context.Background(), InspectRequest{Repository: repo, Target: tgt}); err != nil || insp.Kind != StateReady {
		t.Errorf("Inspect = %q, %v", insp.Kind, err)
	}
}

func adopt(t *testing.T, r *wsRepos, repo gitcli.Repository, tgt Target, head gitcli.ObjectID) string {
	t.Helper()
	svc, _ := r.newService(t)
	assertAttached(t, svc, repo, tgt, runCheckout(t, svc, repo, tgt, head, ""), head)
	return wsPathOf(repo)
}

// foreignWith arranges a foreign worktree elsewhere holding the feature branch
// at the head, then applies mutate to it; it returns the head and that worktree.
func foreignWith(mutate func(t *testing.T, r *wsRepos, p string)) func(*testing.T, *wsRepos, gitcli.Repository, Target, gitcli.ObjectID) (gitcli.ObjectID, string) {
	return func(t *testing.T, r *wsRepos, _ gitcli.Repository, _ Target, head gitcli.ObjectID) (gitcli.ObjectID, string) {
		r.localFeatureAt(t, head)
		p := r.foreignAt(t, r.elsewhere())
		mutate(t, r, p)
		return head, p
	}
}

type attachRow struct {
	name, branch string
	docket       bool
	arrange      func(t *testing.T, r *wsRepos, repo gitcli.Repository, tgt Target, head gitcli.ObjectID) (gitcli.ObjectID, string, gitcli.ObjectID)
}

var attachRows = []attachRow{
	{name: "remote-only", arrange: func(t *testing.T, r *wsRepos, repo gitcli.Repository, tgt Target, head gitcli.ObjectID) (gitcli.ObjectID, string, gitcli.ObjectID) {
		return head, "", ""
	}},
	{name: "remote-only-docket-topology", docket: true, arrange: func(t *testing.T, r *wsRepos, repo gitcli.Repository, tgt Target, head gitcli.ObjectID) (gitcli.ObjectID, string, gitcli.ObjectID) {
		return head, "", ""
	}},
	// Review Focus 2: a recorded branch that is not feat/<slug> still lands at .worktrees/<slug>.
	{name: "recorded-branch-differs", branch: "feature/renamed-head", arrange: func(t *testing.T, r *wsRepos, repo gitcli.Repository, tgt Target, head gitcli.ObjectID) (gitcli.ObjectID, string, gitcli.ObjectID) {
		return head, "", ""
	}},
	{name: "local-at-head", arrange: func(t *testing.T, r *wsRepos, repo gitcli.Repository, tgt Target, head gitcli.ObjectID) (gitcli.ObjectID, string, gitcli.ObjectID) {
		r.localFeatureAt(t, head)
		return head, "", ""
	}},
	{name: "local-behind", arrange: func(t *testing.T, r *wsRepos, repo gitcli.Repository, tgt Target, head gitcli.ObjectID) (gitcli.ObjectID, string, gitcli.ObjectID) {
		r.localFeatureAt(t, head)
		return r.writerCommit(t, coFeature, "two.txt"), "", head
	}},
	{name: "resume-nothing-created", arrange: func(t *testing.T, r *wsRepos, repo gitcli.Repository, tgt Target, head gitcli.ObjectID) (gitcli.ObjectID, string, gitcli.ObjectID) {
		writeStateManifest(t, repo, tgt, r.mainTip(t), PhaseAllocating)
		return head, "", ""
	}},
	{name: "resume-branch-created", arrange: func(t *testing.T, r *wsRepos, repo gitcli.Repository, tgt Target, head gitcli.ObjectID) (gitcli.ObjectID, string, gitcli.ObjectID) {
		writeStateManifest(t, repo, tgt, r.mainTip(t), PhaseAllocating)
		r.localFeatureAt(t, head)
		return head, "", ""
	}},
	{name: "resume-worktree-attached", arrange: func(t *testing.T, r *wsRepos, repo gitcli.Repository, tgt Target, head gitcli.ObjectID) (gitcli.ObjectID, string, gitcli.ObjectID) {
		writeStateManifest(t, repo, tgt, r.mainTip(t), PhaseAllocating)
		r.localFeatureAt(t, head)
		gitOut(t, r.Primary, "worktree", "add", "-q", wsPathOf(repo), coFeature)
		return head, "", ""
	}},
	{name: "foreign-elsewhere", arrange: func(t *testing.T, r *wsRepos, repo gitcli.Repository, tgt Target, head gitcli.ObjectID) (gitcli.ObjectID, string, gitcli.ObjectID) {
		r.localFeatureAt(t, head)
		p := r.foreignAt(t, r.elsewhere())
		writeWorktreeFile(t, p, ".docket/build-output.txt", "ignored build output\n")
		return head, p, ""
	}},
	{name: "foreign-at-target", arrange: func(t *testing.T, r *wsRepos, repo gitcli.Repository, tgt Target, head gitcli.ObjectID) (gitcli.ObjectID, string, gitcli.ObjectID) {
		r.localFeatureAt(t, head)
		return head, r.foreignAt(t, wsPathOf(repo)), ""
	}},
}

func TestIntegrationWorkspaceCheckoutAttaches(t *testing.T) {
	for _, tc := range attachRows {
		t.Run(tc.name, func(t *testing.T) {
			r := mainModeRepo(t)
			if tc.docket {
				r = docketModeRepo(t)
			}
			branch := coFeature
			if tc.branch != "" {
				branch = tc.branch
			}
			head := r.pushBranch(t, branch, "main")
			svc, repo := r.newService(t)
			tgt := targetFor(t, branch)
			want, removed, ff := tc.arrange(t, r, repo, tgt, head)
			before := r.snapshotAll(t)
			res := runCheckout(t, svc, repo, tgt, want, "")
			assertAttached(t, svc, repo, tgt, res, want)
			if res.RemovedWorktree != removed || res.FastForwardedFrom != ff {
				t.Errorf("removed=%q ff=%q; want %q %q", res.RemovedWorktree, res.FastForwardedFrom, removed, ff)
			}
			if removed != "" && removed != wsPathOf(repo) {
				if _, err := os.Lstat(removed); !os.IsNotExist(err) {
					t.Errorf("foreign folder survived (%v)", err)
				}
			}
			r.assertAllUnchanged(t, before)
		})
	}
}

type refuseRow struct {
	name, branch, disp, reason, proof, mention string
	owned, meta                                bool
	arrange                                    func(t *testing.T, r *wsRepos, repo gitcli.Repository, tgt Target, head gitcli.ObjectID) (gitcli.ObjectID, string)
}

func unsafeRow(name, proof, mention string, meta bool, mutate func(t *testing.T, r *wsRepos, p string)) refuseRow {
	return refuseRow{name: name, disp: "blocked", reason: CheckoutReasonForeignWorktreeUnsafe, proof: proof, mention: mention, meta: meta, arrange: foreignWith(mutate)}
}

func noMutation(*testing.T, *wsRepos, string) {}

var refuseRows = []refuseRow{
	{name: "local-ahead", disp: "blocked", reason: CheckoutReasonLocalBranchDiverged, mention: "git branch -D " + coFeature,
		arrange: func(t *testing.T, r *wsRepos, repo gitcli.Repository, tgt Target, head gitcli.ObjectID) (gitcli.ObjectID, string) {
			r.localFeatureAt(t, head)
			r.localCommit(t)
			return head, ""
		}},
	{name: "local-diverged", disp: "blocked", reason: CheckoutReasonLocalBranchDiverged,
		arrange: func(t *testing.T, r *wsRepos, repo gitcli.Repository, tgt Target, head gitcli.ObjectID) (gitcli.ObjectID, string) {
			r.localFeatureAt(t, head)
			r.localCommit(t)
			return r.writerCommit(t, coFeature, "r.txt"), ""
		}},
	{name: "branch-in-primary", disp: "blocked", reason: CheckoutReasonBranchInPrimary, mention: "switch",
		arrange: func(t *testing.T, r *wsRepos, repo gitcli.Repository, tgt Target, head gitcli.ObjectID) (gitcli.ObjectID, string) {
			r.localFeatureAt(t, head)
			gitOut(t, r.Primary, "checkout", "-q", coFeature)
			return head, ""
		}},
	{name: "head-mismatch", disp: "contended", reason: CheckoutReasonHeadMismatch,
		arrange: func(t *testing.T, r *wsRepos, repo gitcli.Repository, tgt Target, head gitcli.ObjectID) (gitcli.ObjectID, string) {
			return r.mainTip(t), ""
		}},
	{name: "remote-absent", branch: "feat/never-pushed", disp: "blocked", reason: CheckoutReasonRemoteFeatureAbsent,
		arrange: func(t *testing.T, r *wsRepos, repo gitcli.Repository, tgt Target, head gitcli.ObjectID) (gitcli.ObjectID, string) {
			return head, ""
		}},
	{name: "target-folder", disp: "blocked", reason: CheckoutReasonTargetPathOccupied,
		arrange: func(t *testing.T, r *wsRepos, repo gitcli.Repository, tgt Target, head gitcli.ObjectID) (gitcli.ObjectID, string) {
			writeWorktreeFile(t, wsPathOf(repo), "notes.txt", "mine\n")
			return head, ""
		}},
	{name: "target-other-branch", disp: "blocked", reason: CheckoutReasonTargetPathOccupied,
		arrange: func(t *testing.T, r *wsRepos, repo gitcli.Repository, tgt Target, head gitcli.ObjectID) (gitcli.ObjectID, string) {
			gitOut(t, r.Primary, "worktree", "add", "-q", "-b", "feat/else", wsPathOf(repo), "main")
			return head, wsPathOf(repo)
		}},
	{name: "foreign-manifest", disp: "blocked", reason: CheckoutReasonForeignManifest,
		arrange: func(t *testing.T, r *wsRepos, repo gitcli.Repository, tgt Target, head gitcli.ObjectID) (gitcli.ObjectID, string) {
			writeWorktreeFile(t, metaDirOf(repo, tgt), manifestFileName, "{not json")
			return head, ""
		}},
	{name: "owned-dirty", disp: "blocked", reason: CheckoutReasonWorkspaceDirty, owned: true,
		arrange: func(t *testing.T, r *wsRepos, repo gitcli.Repository, tgt Target, head gitcli.ObjectID) (gitcli.ObjectID, string) {
			ws := adopt(t, r, repo, tgt, head)
			writeWorktreeFile(t, ws, "scratch.txt", "scratch\n")
			return head, ws
		}},
	{name: "owned-ahead", disp: "blocked", reason: CheckoutReasonLocalHeadMismatch, owned: true,
		arrange: func(t *testing.T, r *wsRepos, repo gitcli.Repository, tgt Target, head gitcli.ObjectID) (gitcli.ObjectID, string) {
			ws := adopt(t, r, repo, tgt, head)
			commitIn(t, ws, "more.txt")
			return head, ws
		}},
	// Review Focus 4: an owned workspace mid-rebase is left to finalize.rebase.
	{name: "owned-mid-rebase", disp: "blocked", reason: CheckoutReasonWorkspaceNotReady, owned: true,
		arrange: func(t *testing.T, r *wsRepos, repo gitcli.Repository, tgt Target, head gitcli.ObjectID) (gitcli.ObjectID, string) {
			ws := adopt(t, r, repo, tgt, head)
			r.conflictedRebase(t, ws)
			return head, ws
		}},
	{name: "owned-cleaned", disp: "blocked", reason: CheckoutReasonWorkspaceNotReady, owned: true,
		arrange: func(t *testing.T, r *wsRepos, repo gitcli.Repository, tgt Target, head gitcli.ObjectID) (gitcli.ObjectID, string) {
			writeStateManifest(t, repo, tgt, r.mainTip(t), PhaseCleaned)
			return head, ""
		}},
	{name: "resume-base-not-contained", disp: "blocked", reason: CheckoutReasonWorkspaceNotReady, owned: true,
		arrange: func(t *testing.T, r *wsRepos, repo gitcli.Repository, tgt Target, head gitcli.ObjectID) (gitcli.ObjectID, string) {
			moved := r.writerCommit(t, "main", "moved.txt")
			gitOut(t, r.Primary, "fetch", "-q", "origin", "main")
			writeStateManifest(t, repo, tgt, moved, PhaseAllocating)
			return head, ""
		}},
	unsafeRow("proof-dirty-tracked", ProofDirty, "", false, func(t *testing.T, r *wsRepos, p string) {
		writeWorktreeFile(t, p, "main.go", "package main // edited\n")
	}),
	unsafeRow("proof-staged", ProofDirty, "", false, func(t *testing.T, r *wsRepos, p string) {
		writeWorktreeFile(t, p, "staged.txt", "staged\n")
		gitOut(t, p, "add", "staged.txt")
	}),
	unsafeRow("proof-untracked", ProofDirty, "", false, func(t *testing.T, r *wsRepos, p string) {
		writeWorktreeFile(t, p, "new.txt", "new\n")
	}),
	unsafeRow("proof-operation", ProofOperation, "", false, func(t *testing.T, r *wsRepos, p string) {
		gitOut(t, p, "bisect", "start")
	}),
	unsafeRow("proof-head", ProofHeadMismatch, "", false, func(t *testing.T, r *wsRepos, p string) {
		commitIn(t, p, "unpushed.txt")
	}),
	unsafeRow("proof-locked", ProofLocked, "git worktree unlock", false, func(t *testing.T, r *wsRepos, p string) {
		gitOut(t, r.Primary, "worktree", "lock", p)
	}),
	unsafeRow("proof-metadata", ProofMetadataWorktree, "", true, noMutation),
	{name: "proof-manifest-claims", disp: "blocked", reason: CheckoutReasonForeignWorktreeUnsafe, proof: ProofManifestClaims,
		arrange: func(t *testing.T, r *wsRepos, repo gitcli.Repository, tgt Target, head gitcli.ObjectID) (gitcli.ObjectID, string) {
			_, p := foreignWith(noMutation)(t, r, repo, tgt, head)
			other, err := NewTarget(8, "other-change", tgt.Base, "feat/other-change")
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC().Format(time.RFC3339)
			if err := writeManifest(metaDirOf(repo, other), Manifest{
				Schema: manifestSchemaVersion, ID: workspaceID(other.FeatureRef), CommonDir: repo.CommonDir,
				ChangeID: other.ChangeID, Slug: other.Slug, FeatureRef: other.FeatureRef, BaseRef: other.BaseRef,
				BaseCommit: r.mainTip(t), Path: p, Phase: PhaseReady, CreatedUTC: now, UpdatedUTC: now,
			}); err != nil {
				t.Fatal(err)
			}
			return head, p
		}},
	{name: "proof-path-missing", disp: "blocked", reason: CheckoutReasonForeignWorktreeUnsafe, proof: ProofPathMissing, mention: "git worktree prune",
		arrange: func(t *testing.T, r *wsRepos, repo gitcli.Repository, tgt Target, head gitcli.ObjectID) (gitcli.ObjectID, string) {
			_, p := foreignWith(noMutation)(t, r, repo, tgt, head)
			if err := os.RemoveAll(p); err != nil {
				t.Fatal(err)
			}
			return head, ""
		}},
	{name: "proof-not-attached", disp: "blocked", reason: CheckoutReasonForeignWorktreeUnsafe, proof: ProofNotAttached,
		arrange: func(t *testing.T, r *wsRepos, repo gitcli.Repository, tgt Target, head gitcli.ObjectID) (gitcli.ObjectID, string) {
			gitOut(t, r.Primary, "fetch", "-q", "origin", coFeature)
			gitOut(t, r.Primary, "worktree", "add", "-q", "--detach", wsPathOf(repo), string(head))
			return head, wsPathOf(repo)
		}},
}

// Every refusal leaves the local branch, the kept worktree, and the primary
// byte-identical and never advances a manifest to ready.
func TestIntegrationWorkspaceCheckoutRefusals(t *testing.T) {
	for _, tc := range refuseRows {
		t.Run(tc.name, func(t *testing.T) {
			r := mainModeRepo(t)
			head := r.pushBranch(t, coFeature, "main")
			svc, repo := r.newService(t)
			branch := coFeature
			if tc.branch != "" {
				branch = tc.branch
			}
			tgt := targetFor(t, branch)
			expected, keep := tc.arrange(t, r, repo, tgt, head)
			var snap map[string]string
			if keep != "" {
				snap = snapshotTree(t, keep)
			}
			tip, _ := gitTry(r.Primary, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
			before := r.snapshotAll(t)
			meta := ""
			if tc.meta {
				meta = keep
			}
			res := runCheckout(t, svc, repo, tgt, expected, meta)
			if string(res.Disposition) != tc.disp || res.Reason != tc.reason || res.FailedProof != tc.proof {
				t.Fatalf("= %q/%q proof %q (%s); want %s/%s proof %q", res.Disposition, res.Reason, res.FailedProof, res.Message, tc.disp, tc.reason, tc.proof)
			}
			if !strings.Contains(res.Message, tc.mention) {
				t.Errorf("message %q lacks %q", res.Message, tc.mention)
			}
			if keep != "" {
				assertUnchanged(t, snap, keep)
			}
			if after, _ := gitTry(r.Primary, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch); after != tip {
				t.Errorf("local branch moved %q -> %q", tip, after)
			}
			if m, ok, _ := loadManifest(metaDirOf(repo, tgt)); ok && m.Phase == PhaseReady && !tc.owned {
				t.Error("a refused checkout adopted the workspace")
			}
			r.assertAllUnchanged(t, before)
		})
	}
}

// A second checkout of an adopted workspace is a read-only no-op.
func TestIntegrationWorkspaceCheckoutExistingIsNoOp(t *testing.T) {
	r := mainModeRepo(t)
	head := r.pushBranch(t, coFeature, "main")
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	ws := adopt(t, r, repo, tgt, head)
	snap := snapshotTree(t, ws)
	manifestPath := filepath.Join(metaDirOf(repo, tgt), manifestFileName)
	manBefore := readFileBytes(t, manifestPath)
	res := runCheckout(t, svc, repo, tgt, head, "")
	if res.Disposition != CheckoutExisting || res.HeadCommit != head || res.Path != ws {
		t.Fatalf("= %q %s@%s (%s: %s); want existing %s@%s", res.Disposition, res.Path, res.HeadCommit, res.Reason, res.Message, ws, head)
	}
	assertUnchanged(t, snap, ws)
	if got := readFileBytes(t, manifestPath); got != manBefore {
		t.Errorf("manifest rewritten:\n before=%s\n after =%s", manBefore, got)
	}
}

// Review Focus 3: two racing checkouts serialize on the operation lock, so one
// attaches and the other finds the ready workspace.
func TestIntegrationWorkspaceCheckoutConcurrentRunsSerialize(t *testing.T) {
	r := mainModeRepo(t)
	head := r.pushBranch(t, coFeature, "main")
	svcA, repo := r.newService(t)
	svcB, _ := r.newService(t)
	tgt := freshTarget(t, 7)
	start := make(chan struct{})
	got := make(chan string, 2)
	for _, svc := range []*Service{svcA, svcB} {
		go func(svc *Service) {
			<-start
			res, err := svc.Checkout(context.Background(), coReq(repo, tgt, head, ""))
			if err != nil {
				got <- "error: " + err.Error()
				return
			}
			got <- string(res.Disposition)
		}(svc)
	}
	close(start)
	var outs []string
	deadline := time.After(2 * time.Minute)
	for len(outs) < 2 {
		select {
		case o := <-got:
			outs = append(outs, o)
		case <-deadline:
			t.Fatalf("timed out with %v", outs)
		}
	}
	sort.Strings(outs)
	if outs[0] != string(CheckoutAttached) || outs[1] != string(CheckoutExisting) {
		t.Fatalf("dispositions = %v; want [attached existing]", outs)
	}
}

// writeFatalGit is writeFailingGit with git's fatal exit status (128). Exit 1 is
// not a failure for every probe: `symbolic-ref --quiet` exiting 1 is git's
// clean "HEAD is detached" answer, so only a fatal exit injects a probe error.
func writeFatalGit(t *testing.T, failSubcommand string) string {
	t.Helper()
	p := filepath.Join(testsupport.TempDir(t), "git")
	script := "#!/bin/sh\nif [ \"$1\" = \"" + failSubcommand + "\" ]; then\n  echo \"fatal: fake git: $1 disabled for test\" >&2\n  exit 128\nfi\nexec git \"$@\"\n"
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// A failing probe is a *Failure from the expected stage; nothing is adopted or removed.
func TestIntegrationWorkspaceCheckoutProbeErrorsTouchNothing(t *testing.T) {
	for _, tc := range []struct {
		sub     string
		foreign bool
		stage   Stage
	}{{"fetch", false, "fetch"}, {"merge-base", false, "inventory"}, {"status", true, "probe"}, {"symbolic-ref", true, "probe"}} {
		t.Run(tc.sub, func(t *testing.T) {
			r := mainModeRepo(t)
			head := r.pushBranch(t, coFeature, "main")
			_, repo := r.newService(t) // real discovery: discovery itself runs `git worktree list`
			tgt := freshTarget(t, 7)
			var p string
			var snap map[string]string
			if tc.foreign {
				r.localFeatureAt(t, head)
				p = r.foreignAt(t, r.elsewhere())
				snap = snapshotTree(t, p)
			}
			c, err := gitcli.NewClient(gitcli.WithExecutable(writeFatalGit(t, tc.sub)))
			if err != nil {
				t.Fatal(err)
			}
			svc, _ := NewService(c)
			_, err = svc.Checkout(context.Background(), coReq(repo, tgt, head, ""))
			if f, ok := AsFailure(err); !ok || f.Stage != tc.stage {
				t.Fatalf("failing %s = %v; want a *Failure at stage %s", tc.sub, err, tc.stage)
			}
			if tc.foreign {
				assertUnchanged(t, snap, p)
			}
			if _, ok, _ := loadManifest(metaDirOf(repo, tgt)); ok {
				t.Error("a manifest was published")
			}
		})
	}
}

// Review Focus 1: a foreign worktree mid-rebase outside .worktrees/ is detached,
// so git alone knows it still holds the branch. The attach fails on git's
// refusal with the rebase untouched and an allocating breadcrumb left; once the
// human aborts the rebase, a rerun removes the worktree and attaches.
func TestIntegrationWorkspaceCheckoutForeignRebaseElsewhereRecovers(t *testing.T) {
	r := mainModeRepo(t)
	head := r.pushBranch(t, coFeature, "main")
	r.localFeatureAt(t, head)
	foreign := r.foreignAt(t, r.elsewhere())
	r.conflictedRebase(t, foreign)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	if _, err := svc.Checkout(context.Background(), coReq(repo, tgt, head, "")); err == nil {
		t.Fatal("Checkout over a foreign rebase = nil error; want git's refusal")
	}
	rd := gitOut(t, foreign, "rev-parse", "--git-path", "rebase-merge")
	if !filepath.IsAbs(rd) {
		rd = filepath.Join(foreign, rd)
	}
	if _, err := os.Stat(rd); err != nil {
		t.Fatalf("the foreign rebase was disturbed: %v", err)
	}
	if m, ok, err := loadManifest(metaDirOf(repo, tgt)); err != nil || !ok || m.Phase != PhaseAllocating {
		t.Fatalf("manifest %v %q %v; want an allocating breadcrumb", ok, m.Phase, err)
	}
	gitOut(t, foreign, "rebase", "--abort")
	res := runCheckout(t, svc, repo, tgt, head, "")
	assertAttached(t, svc, repo, tgt, res, head)
	if res.RemovedWorktree != foreign {
		t.Errorf("RemovedWorktree = %q; want %q", res.RemovedWorktree, foreign)
	}
}
