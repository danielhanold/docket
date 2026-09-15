//go:build integration

package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/codexcontract"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/testsupport"
)

func TestIntegrationWorkflowAgentInputsAcceptsPrimaryStartupForRegisteredFeature(t *testing.T) {
	root := testsupport.TempDir(t)
	controlRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(root, "primary")
	feature := filepath.Join(root, "feature")
	runGit := func(dir string, args ...string) string {
		c := exec.Command("git", args...)
		c.Dir = dir
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid", "GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid")
		b, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, b)
		}
		return strings.TrimSpace(string(b))
	}
	runGit(root, "init", "-b", "main", primary)
	if err := os.WriteFile(filepath.Join(primary, "README.md"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(primary, "add", "README.md")
	runGit(primary, "commit", "-m", "base")
	runGit(primary, "worktree", "add", "-b", "codex/change", "--", feature, "HEAD")
	client, err := gitcli.NewClient()
	if err != nil {
		t.Fatal(err)
	}
	repo, err := client.Discover(context.Background(), gitcli.DiscoverOptions{InvocationPath: primary})
	if err != nil {
		t.Fatal(err)
	}
	featureWorktree, err := client.DiscoverWorktree(context.Background(), gitcli.DiscoverOptions{InvocationPath: feature})
	if err != nil {
		t.Fatal(err)
	}
	head := runGit(feature, "rev-parse", "HEAD")
	docketPath := writeDocketVersionStub(t, root, head)
	identity, err := codexcontract.ObserveRootIdentity(featureWorktree.Root, featureWorktree.GitDir)
	if err != nil {
		t.Fatal(err)
	}
	a := codexcontract.Assignment{SchemaVersion: 1, ChangeID: 425, Role: "docket-plan-writer", Phase: "plan", Mode: "fresh", Primary: repo.PrimaryWorktree, Feature: featureWorktree.Root, CommonDir: repo.CommonDir, Branch: "codex/change", EntryHEAD: head, MetadataRevision: head, ChangePath: "docs/changes/active/0425.md", ArtifactPath: "docs/plans/425.md", DocketExecutable: docketPath, DocketCommit: head, ReadRoots: []string{controlRoot, filepath.Dir(docketPath)}, WritePaths: []string{"docs/plans/425.md"}, RootIdentity: &identity}
	ab, _ := json.Marshal(a)
	ap := filepath.Join(root, "assignment.json")
	if err := os.WriteFile(ap, ab, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(ab)
	deps, err := NewAgentInputDeps(docketPath)
	if err != nil {
		t.Fatal(err)
	}
	deps.Workspace = &inputWorkspace{}
	res := CheckAgentInputs(context.Background(), deps, CheckInputsRequest{Assignment: ap, SHA256: hex.EncodeToString(sum[:]), Stage: "entry", RepoDir: primary})
	if res.Result != ResultApplied {
		t.Fatalf("result=%s reason=%s", res.Result, res.Reason)
	}
}

func checkFinalizeEntry(t *testing.T, f *rebaseFixture, finalize FinalizeDeps, role, mode, kind, attempt, reservation string, writePaths []string) CheckInputsResult {
	t.Helper()
	ctx := context.Background()
	wt, err := finalize.Planning.Client.DiscoverWorktree(ctx, gitcli.DiscoverOptions{InvocationPath: f.wp})
	if err != nil {
		t.Fatal(err)
	}
	identity, err := codexcontract.ObserveRootIdentity(wt.Root, wt.GitDir)
	if err != nil {
		t.Fatal(err)
	}
	pin, err := finalize.Planning.Reader.PinContext(ctx, f.repo.invocation)
	if err != nil {
		t.Fatal(err)
	}
	head := runGit(t, f.wp, "rev-parse", "HEAD")
	docketPath := writeDocketVersionStub(t, testsupport.TempDir(t), head)
	a := codexcontract.Assignment{SchemaVersion: 1, ChangeID: f.id, Role: role, Phase: mode, Mode: mode, Primary: f.gitrepo.PrimaryWorktree, Feature: wt.Root, CommonDir: f.gitrepo.CommonDir, Branch: f.target.FeatureBranch(), EntryHEAD: head, MetadataRevision: pin.MetadataRevision, ChangePath: groomPath(f.id, f.slug), DocketExecutable: docketPath, DocketCommit: head, ReadRoots: []string{filepath.Dir(docketPath)}, WritePaths: writePaths, RootIdentity: &identity}
	ab, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	ap := filepath.Join(testsupport.TempDir(t), "assignment.json")
	if err := os.WriteFile(ap, ab, 0o600); err != nil {
		t.Fatal(err)
	}
	as := sha256.Sum256(ab)
	ad := hex.EncodeToString(as[:])
	p := codexcontract.WorkerPayload{SchemaVersion: 1, Kind: kind, AssignmentPath: ap, AssignmentSHA256: ad, EntryArgv: []string{docketPath, "agent", "check-inputs", "--assignment", ap, "--sha256", ad, "--stage", "entry", "--json"}, TaskText: "finalize child", Attempt: attempt, ResolverReservation: reservation}
	pb, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	pp := filepath.Join(filepath.Dir(ap), "payload.json")
	if err := os.WriteFile(pp, pb, 0o600); err != nil {
		t.Fatal(err)
	}
	ps := sha256.Sum256(pb)
	deps, err := NewAgentInputDeps(docketPath)
	if err != nil {
		t.Fatal(err)
	}
	deps.Workspace = NewAgentWorkspaceValidator(finalize.Planning, WorkspaceDeps{Service: f.svc})
	deps.Role = NewAgentFinalizeInputValidator(finalize)
	return CheckAgentInputs(ctx, deps, CheckInputsRequest{Assignment: ap, SHA256: ad, Payload: pp, PayloadSHA256: hex.EncodeToString(ps[:]), Stage: "entry", RepoDir: f.repo.invocation})
}

func TestIntegrationFinalizeRebaseAgentInputsWiresResolverAndRepairAuthority(t *testing.T) {
	t.Run("resolver reservation", func(t *testing.T) {
		f, conflicted, deps := setupConflictedRebase(t, planRepoModes()[0])
		reserve := FinalizeResolverReserve(context.Background(), deps, f.repo.invocation, f.id, conflicted.Attempt)
		if reserve.Disposition != ReserveReserved {
			t.Fatalf("reserve=%+v", reserve)
		}
		valid := checkFinalizeEntry(t, f, deps, "docket-rebase-resolver", "resolver", "resolver", conflicted.Attempt, reserve.Reservation, conflicted.UnmergedPaths)
		if valid.Result != ResultApplied {
			t.Fatalf("valid resolver result=%s reason=%s", valid.Result, valid.Reason)
		}
		wrong := checkFinalizeEntry(t, f, deps, "docket-rebase-resolver", "resolver", "resolver", conflicted.Attempt, "wrong-reservation", conflicted.UnmergedPaths)
		if wrong.Result == ResultApplied {
			t.Fatal("unowned resolver reservation was accepted")
		}
		other := filepath.Join(testsupport.TempDir(t), "unrelated-detached")
		head := runGit(t, f.wp, "rev-parse", "HEAD")
		runGit(t, f.gitrepo.PrimaryWorktree, "worktree", "add", "--detach", other, head)
		foreignWorkspace := *f
		foreignWorkspace.wp = other
		wrong = checkFinalizeEntry(t, &foreignWorkspace, deps, "docket-rebase-resolver", "resolver", "resolver", conflicted.Attempt, reserve.Reservation, conflicted.UnmergedPaths)
		if wrong.Result == ResultApplied {
			t.Fatal("resolver entry accepted an unrelated detached worktree using another worktree's valid reservation")
		}
	})
	t.Run("repair attempt", func(t *testing.T) {
		f := setupRebaseFixture(t, planRepoModes()[0])
		f.advanceBase(t)
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		deps := f.finalizeDeps(gh, &fakeGate{result: LocalGateResult{Outcome: FinalizeGateFailed, RunDir: "/run/red"}})
		failed := FinalizeRebase(context.Background(), deps, f.repo.invocation, FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		if failed.Disposition != RebaseDispFailed || failed.Attempt == "" {
			t.Fatalf("failed rebase=%+v", failed)
		}
		valid := checkFinalizeEntry(t, f, deps, "docket-integration-repair", "repair", "repair", failed.Attempt, "", []string{"repair.go"})
		if valid.Result != ResultApplied {
			t.Fatalf("valid repair result=%s reason=%s", valid.Result, valid.Reason)
		}
		wrong := checkFinalizeEntry(t, f, deps, "docket-integration-repair", "repair", "repair", "foreign-attempt", "", []string{"repair.go"})
		if wrong.Result == ResultApplied {
			t.Fatal("unowned repair attempt was accepted")
		}
	})
}
