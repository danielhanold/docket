package workspace

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/testsupport"
)

// The Inspect tests build each StateKind with the real-Git harness primitives
// and assert the classification, the exact DirtyPaths summary, and that Inspect
// mutates nothing — a full tree snapshot before and after every Inspect is
// identical. Malformed and foreign manifests are data (StateForeign with a
// parse detail), never an error; only an unreadable manifest slot is an error.

// inspectOK runs Inspect and fails on error, returning the Inspection.
func inspectOK(t *testing.T, svc *Service, repo gitcli.Repository, tgt Target) Inspection {
	t.Helper()
	insp, err := svc.Inspect(context.Background(), InspectRequest{Repository: repo, Target: tgt})
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	return insp
}

// assertInspectReadOnly runs Inspect and proves it changed no observable byte of
// the workspace or any preserved worktree.
func assertInspectReadOnly(t *testing.T, svc *Service, r *wsRepos, repo gitcli.Repository, tgt Target) Inspection {
	t.Helper()
	ws := wsPathOf(repo)
	var beforeWs map[string]string
	if _, err := os.Stat(ws); err == nil {
		beforeWs = snapshotTree(t, ws)
	}
	beforePreserve := r.snapshotAll(t)
	insp := inspectOK(t, svc, repo, tgt)
	if beforeWs != nil {
		assertUnchanged(t, beforeWs, ws)
	}
	r.assertAllUnchanged(t, beforePreserve)
	return insp
}

func TestInspectReady(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	first := prepareOK(t, svc, repo, tgt)

	insp := assertInspectReadOnly(t, svc, r, repo, tgt)
	if insp.Kind != StateReady {
		t.Errorf("Kind = %q; want ready", insp.Kind)
	}
	if !insp.Registered {
		t.Errorf("Registered = false; want true")
	}
	if insp.Branch != tgt.FeatureRef {
		t.Errorf("Branch = %q; want %q", insp.Branch, tgt.FeatureRef)
	}
	if insp.HeadCommit != first.HeadCommit || insp.BranchHead != first.HeadCommit {
		t.Errorf("HeadCommit=%q BranchHead=%q; want both %q", insp.HeadCommit, insp.BranchHead, first.HeadCommit)
	}
	if !insp.BaseReached {
		t.Errorf("BaseReached = false; want true")
	}
	if len(insp.DirtyPaths) != 0 {
		t.Errorf("DirtyPaths = %v; want empty", insp.DirtyPaths)
	}
	if insp.Phase != PhaseReady {
		t.Errorf("Phase = %q; want ready", insp.Phase)
	}
}

// TestInspectReadyAfterParentRebase proves a READY workspace whose branch was
// legitimately rebased — head no longer reaches the recorded creation base —
// still classifies ready: the recorded base is a fact (BaseReached=false), not
// an identity requirement. The allocating-phase ancestry protection and every
// registration/ref/head/clean check are unchanged (change 0429).
func TestInspectReadyAfterParentRebase(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	prepareOK(t, svc, repo, tgt)

	// Rewrite the recorded base to a commit the feature head does not reach (a
	// later origin-main commit, fetched into the object store) — the same
	// construction cleanup_test's moved-head row uses, now a legitimate state.
	c1 := r.advanceMain(t)
	gitOut(t, r.Primary, "fetch", "-q", "origin", "main")
	m, present, err := loadManifest(metaDirOf(repo, tgt))
	if err != nil || !present {
		t.Fatalf("loadManifest present=%v err=%v", present, err)
	}
	m.BaseCommit = c1
	if err := writeManifest(metaDirOf(repo, tgt), m); err != nil {
		t.Fatalf("writeManifest(rewritten base): %v", err)
	}

	insp := assertInspectReadOnly(t, svc, r, repo, tgt)
	if insp.Kind != StateReady {
		t.Errorf("Kind = %q; want ready after a parent rebase", insp.Kind)
	}
	if insp.BaseReached {
		t.Errorf("BaseReached = true; want false (fact recorded, not gated)")
	}
}

func TestInspectDirty(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	prepareOK(t, svc, repo, tgt)
	ws := wsPathOf(repo)

	// Staged new file, dirtied tracked file, untracked file.
	writeWorktreeFile(t, ws, "staged.txt", "staged\n")
	gitOut(t, ws, "add", "staged.txt")
	writeWorktreeFile(t, ws, "main.go", "package main // dirty\n")
	writeWorktreeFile(t, ws, "untracked.txt", "untracked\n")

	insp := assertInspectReadOnly(t, svc, r, repo, tgt)
	if insp.Kind != StateDirty {
		t.Errorf("Kind = %q; want dirty-owned", insp.Kind)
	}
	want := []string{"main.go", "staged.txt", "untracked.txt"}
	if !reflect.DeepEqual(insp.DirtyPaths, want) {
		t.Errorf("DirtyPaths = %v; want %v", insp.DirtyPaths, want)
	}
}

func TestInspectResumable(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	base := gitcli.ObjectID(gitOut(t, r.Primary, "rev-parse", "main"))
	writeStateManifest(t, repo, tgt, base, PhaseAllocating)

	insp := assertInspectReadOnly(t, svc, r, repo, tgt)
	if insp.Kind != StateResumable {
		t.Errorf("Kind = %q; want allocating (resumable)", insp.Kind)
	}
	if insp.Registered {
		t.Errorf("Registered = true; want false on a bare allocating partial")
	}
	if insp.BaseCommit != base {
		t.Errorf("BaseCommit = %q; want %q", insp.BaseCommit, base)
	}
}

func TestInspectCleaned(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	prepareOK(t, svc, repo, tgt)

	// A cleaned tombstone: the checkout removed (branch kept), manifest advanced
	// to cleaned. This mimics the state Task 7 Cleanup leaves.
	gitOut(t, r.Primary, "worktree", "remove", "--force", "--", wsPathOf(repo))
	m, present, err := loadManifest(metaDirOf(repo, tgt))
	if err != nil || !present {
		t.Fatalf("loadManifest: present=%v err=%v", present, err)
	}
	m.Phase = PhaseCleaned
	m.UpdatedUTC = time.Now().UTC().Format(time.RFC3339)
	if err := writeManifest(metaDirOf(repo, tgt), m); err != nil {
		t.Fatalf("writeManifest(cleaned): %v", err)
	}

	insp := assertInspectReadOnly(t, svc, r, repo, tgt)
	if insp.Kind != StateCleaned {
		t.Errorf("Kind = %q; want cleaned", insp.Kind)
	}
	if insp.Registered {
		t.Errorf("Registered = true; want false for a tombstone")
	}
	// The local branch survives cleanup.
	if !branchExists(r.Primary, "feat/"+prepSlug) {
		t.Errorf("feat branch missing; cleanup keeps the branch")
	}
}

func TestInspectBranchGone(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	base := gitcli.ObjectID(gitOut(t, r.Primary, "rev-parse", "main"))

	// A ready manifest whose recorded feature branch was never created / is gone.
	writeStateManifest(t, repo, tgt, base, PhaseReady)

	insp := assertInspectReadOnly(t, svc, r, repo, tgt)
	if insp.Kind != StateBranchGone {
		t.Errorf("Kind = %q; want branch-missing", insp.Kind)
	}
}

func TestInspectMismatch(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	prepareOK(t, svc, repo, tgt)

	// Ready manifest, branch still present, but the registered checkout is gone:
	// path/registration/manifest disagree.
	gitOut(t, r.Primary, "worktree", "remove", "--force", "--", wsPathOf(repo))

	insp := inspectOK(t, svc, repo, tgt)
	if insp.Kind != StateMismatch {
		t.Errorf("Kind = %q; want mismatch", insp.Kind)
	}
	if insp.Registered {
		t.Errorf("Registered = true; want false")
	}
}

func TestInspectForeignMalformed(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	if err := os.MkdirAll(metaDirOf(repo, tgt), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(metaDirOf(repo, tgt), manifestFileName), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	insp := inspectOK(t, svc, repo, tgt)
	if insp.Kind != StateForeign {
		t.Errorf("Kind = %q; want foreign", insp.Kind)
	}
	if insp.Detail == "" {
		t.Errorf("Detail empty; want the parse detail carried as data")
	}
}

// TestInspectAbsent is the regression for change 0368: with no manifest, no
// local feature branch, nothing at the intended path, and no registration,
// Inspect reports the proven-absent state instead of foreign. This test MUST
// fail against the pre-0368 conflation (which returned StateForeign here).
func TestInspectAbsent(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)

	insp := inspectOK(t, svc, repo, tgt)
	if insp.Kind != StateAbsent {
		t.Errorf("Kind = %q; want absent for an all-clean missing workspace", insp.Kind)
	}
}

// TestInspectAbsentBlockedByLeftovers pins each leftover that keeps a
// manifest-absent slot classified StateForeign rather than StateAbsent: a
// surviving local feature branch, anything at the intended path (including a
// dangling symlink), and a worktree registration on the feature ref or at the
// target path (change 0368).
func TestInspectAbsentBlockedByLeftovers(t *testing.T) {
	t.Run("local-branch", func(t *testing.T) {
		r := mainModeRepo(t)
		svc, repo := r.newService(t)
		tgt := freshTarget(t, 7)
		gitOut(t, r.Primary, "branch", strings.TrimPrefix(string(tgt.FeatureRef), "refs/heads/"), "main")
		insp := inspectOK(t, svc, repo, tgt)
		if insp.Kind != StateForeign {
			t.Errorf("Kind = %q; want foreign when the local feature branch exists", insp.Kind)
		}
	})
	t.Run("occupied-path-dir", func(t *testing.T) {
		r := mainModeRepo(t)
		svc, repo := r.newService(t)
		tgt := freshTarget(t, 7)
		if err := os.MkdirAll(wsPathOf(repo), 0o755); err != nil {
			t.Fatal(err)
		}
		insp := inspectOK(t, svc, repo, tgt)
		if insp.Kind != StateForeign {
			t.Errorf("Kind = %q; want foreign when the target path is occupied", insp.Kind)
		}
	})
	t.Run("dangling-symlink", func(t *testing.T) {
		r := mainModeRepo(t)
		svc, repo := r.newService(t)
		tgt := freshTarget(t, 7)
		if err := os.MkdirAll(filepath.Dir(wsPathOf(repo)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(testsupport.TempDir(t), "gone"), wsPathOf(repo)); err != nil {
			t.Fatal(err)
		}
		insp := inspectOK(t, svc, repo, tgt)
		if insp.Kind != StateForeign {
			t.Errorf("Kind = %q; want foreign for a dangling symlink at the target path", insp.Kind)
		}
	})
	t.Run("registration-on-feature-ref-elsewhere", func(t *testing.T) {
		r := mainModeRepo(t)
		svc, repo := r.newService(t)
		tgt := freshTarget(t, 7)
		elsewhere := filepath.Join(r.Primary, "..", "elsewhere-ws")
		gitOut(t, r.Primary, "worktree", "add", "-b",
			strings.TrimPrefix(string(tgt.FeatureRef), "refs/heads/"), elsewhere, "main")
		insp := inspectOK(t, svc, repo, tgt)
		if insp.Kind != StateForeign {
			t.Errorf("Kind = %q; want foreign when a registration references the feature ref", insp.Kind)
		}
	})
	t.Run("stale-registration-at-path", func(t *testing.T) {
		r := mainModeRepo(t)
		svc, repo := r.newService(t)
		tgt := freshTarget(t, 7)
		gitOut(t, r.Primary, "worktree", "add", "-b", "throwaway/stale-reg", wsPathOf(repo), "main")
		if err := os.RemoveAll(wsPathOf(repo)); err != nil { // directory gone, registration remains
			t.Fatal(err)
		}
		insp := inspectOK(t, svc, repo, tgt)
		if insp.Kind != StateForeign {
			t.Errorf("Kind = %q; want foreign for a stale registration at the target path", insp.Kind)
		}
	})
}

// TestInspectForeignUnownedCommonDir isolates the manifest OWNERSHIP gate
// (ownsManifest's CommonDir identity conjunct). It writes a structurally valid
// manifest whose ONLY defect is a foreign CommonDir — every other field matches
// this repository and target — and requires StateForeign with the identity
// detail. Unlike Prepare and Cleanup (which re-verify live registration and so
// backstop the ownership check), Inspect returns StateForeign directly on an
// unowned manifest with no other gate behind it, so this is the case that
// reddens when the CommonDir conjunct is dropped.
func TestInspectForeignUnownedCommonDir(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)

	other := mainModeRepo(t)
	_, otherRepo := other.newService(t)
	foreign := Manifest{
		Schema: manifestSchemaVersion, ID: workspaceID(tgt.FeatureRef), CommonDir: otherRepo.CommonDir,
		ChangeID: tgt.ChangeID, Slug: tgt.Slug, FeatureRef: tgt.FeatureRef, BaseRef: tgt.BaseRef,
		BaseCommit: gitcli.ObjectID(gitOut(t, r.Primary, "rev-parse", "main")),
		Path:       wsPathOf(repo), Phase: PhaseReady,
		CreatedUTC: time.Now().UTC().Format(time.RFC3339), UpdatedUTC: time.Now().UTC().Format(time.RFC3339),
	}
	if err := writeManifest(metaDirOf(repo, tgt), foreign); err != nil {
		t.Fatalf("writeManifest(foreign): %v", err)
	}

	insp := inspectOK(t, svc, repo, tgt)
	if insp.Kind != StateForeign {
		t.Errorf("Kind = %q; want foreign for an unowned (foreign-CommonDir) manifest", insp.Kind)
	}
	if insp.Detail == "" {
		t.Errorf("Detail empty; want the identity-mismatch reason carried as data")
	}
}

func TestInspectUnreadableIsError(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	base := gitcli.ObjectID(gitOut(t, r.Primary, "rev-parse", "main"))
	writeStateManifest(t, repo, tgt, base, PhaseReady)

	dir := metaDirOf(repo, tgt)
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	_, err := svc.Inspect(context.Background(), InspectRequest{Repository: repo, Target: tgt})
	if err == nil {
		t.Fatalf("Inspect on unreadable manifest dir = nil error; want failure")
	}
	f, ok := AsFailure(err)
	if !ok {
		t.Fatalf("error %v is not a *Failure", err)
	}
	if f.Kind != KindExternal {
		t.Errorf("Kind = %q; want external", f.Kind)
	}
}
