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
	templateBody := []byte("# Results\n")
	templatePath := filepath.Join(controlRoot, "skills", "docket-implement-next", "results-template.md")
	if err := os.MkdirAll(filepath.Dir(templatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(templatePath, templateBody, 0o600); err != nil {
		t.Fatal(err)
	}
	templateHash := sha256.Sum256(templateBody)
	a := codexcontract.Assignment{SchemaVersion: 1, ChangeID: 425, Role: "docket-plan-writer", Phase: "plan", Mode: "fresh", Primary: repo.PrimaryWorktree, Feature: featureWorktree.Root, CommonDir: repo.CommonDir, Branch: "codex/change", EntryHEAD: head, MetadataRevision: head, ChangePath: "docs/changes/active/0425.md", ArtifactPath: "docs/plans/425.md", DocketExecutable: docketPath, DocketCommit: head, ReadRoots: []string{controlRoot, filepath.Dir(docketPath)}, WritePaths: []string{"docs/plans/425.md"}, RootIdentity: &identity, PlanSkill: "auto", BuildSkill: "auto", ResultsTemplate: "results-template", Resources: []codexcontract.Resource{{LogicalID: "results-template", Path: templatePath, SHA256: hex.EncodeToString(templateHash[:]), Source: "package:docket-implement-next"}}}
	ab, _ := json.Marshal(a)
	ap := filepath.Join(root, "assignment.json")
	if err := os.WriteFile(ap, ab, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(ab)
	digest := hex.EncodeToString(sum[:])
	payload := codexcontract.WorkerPayload{SchemaVersion: 1, Kind: "planner", AssignmentPath: ap, AssignmentSHA256: digest, EntryArgv: codexcontract.EntryCheckerArgv(a, ap, digest), TaskText: "write the pinned plan"}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	payloadPath := filepath.Join(root, "payload.json")
	if err := os.WriteFile(payloadPath, payloadBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	payloadSum := sha256.Sum256(payloadBytes)
	deps, err := NewAgentInputDeps(docketPath)
	if err != nil {
		t.Fatal(err)
	}
	deps.Workspace = &inputWorkspace{}
	res := CheckAgentInputs(context.Background(), deps, CheckInputsRequest{Assignment: ap, SHA256: digest, Stage: "entry", Payload: payloadPath, PayloadSHA256: hex.EncodeToString(payloadSum[:]), RepoDir: primary})
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

func TestIntegrationFinalizeRebaseResolverBootstrapsRootWitnessThroughPrepare(t *testing.T) {
	f, conflicted, deps := setupConflictedRebase(t, planRepoModes()[0])
	reserve := FinalizeResolverReserve(context.Background(), deps, f.repo.invocation, f.id, conflicted.Attempt)
	if reserve.Disposition != ReserveReserved {
		t.Fatalf("reserve=%+v", reserve)
	}
	ctx := context.Background()
	wt, err := deps.Planning.Client.DiscoverWorktree(ctx, gitcli.DiscoverOptions{InvocationPath: f.wp})
	if err != nil {
		t.Fatal(err)
	}
	pin, err := deps.Planning.Reader.PinContext(ctx, f.repo.invocation)
	if err != nil {
		t.Fatal(err)
	}
	head := runGit(t, f.wp, "rev-parse", "HEAD")
	privateDir := testsupport.TempDir(t)
	docketPath := writeDocketVersionStub(t, privateDir, head)
	assignmentPath := filepath.Join(privateDir, "assignment.json")
	payloadPath := filepath.Join(privateDir, "payload.json")
	a := codexcontract.Assignment{SchemaVersion: 1, ChangeID: f.id, Role: "docket-rebase-resolver", Phase: "resolver", Mode: "resolver", Primary: f.gitrepo.PrimaryWorktree, Feature: wt.Root, CommonDir: f.gitrepo.CommonDir, Branch: f.target.FeatureBranch(), EntryHEAD: head, MetadataRevision: pin.MetadataRevision, ChangePath: groomPath(f.id, f.slug), DocketExecutable: docketPath, DocketCommit: head, ReadRoots: []string{filepath.Dir(docketPath)}, WritePaths: conflicted.UnmergedPaths}
	inputDeps, err := NewAgentInputDeps(docketPath)
	if err != nil {
		t.Fatal(err)
	}
	inputDeps.Role = NewAgentFinalizeInputValidator(deps)

	pinInputs := func(assignment codexcontract.Assignment, reservation, stage string) CheckInputsRequest {
		t.Helper()
		assignmentBytes, err := json.Marshal(assignment)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(assignmentPath, assignmentBytes, 0o600); err != nil {
			t.Fatal(err)
		}
		assignmentSum := sha256.Sum256(assignmentBytes)
		assignmentDigest := hex.EncodeToString(assignmentSum[:])
		payload := codexcontract.WorkerPayload{SchemaVersion: 1, Kind: "resolver", AssignmentPath: assignmentPath, AssignmentSHA256: assignmentDigest, EntryArgv: codexcontract.EntryCheckerArgv(assignment, assignmentPath, assignmentDigest), TaskText: "resolve the owned rebase conflict", Attempt: conflicted.Attempt, ResolverReservation: reservation}
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(payloadPath, payloadBytes, 0o600); err != nil {
			t.Fatal(err)
		}
		payloadSum := sha256.Sum256(payloadBytes)
		return CheckInputsRequest{Assignment: assignmentPath, SHA256: assignmentDigest, Payload: payloadPath, PayloadSHA256: hex.EncodeToString(payloadSum[:]), Stage: stage, RepoDir: f.repo.invocation}
	}

	provisional := pinInputs(a, reserve.Reservation, "prepare")
	withoutPayload := provisional
	withoutPayload.Payload, withoutPayload.PayloadSHA256 = "", ""
	if result := CheckAgentInputs(ctx, inputDeps, withoutPayload); result.Result == ResultApplied {
		t.Fatal("resolver prepare accepted no provisional role authority")
	}
	if result := CheckAgentInputs(ctx, inputDeps, pinInputs(a, "wrong-reservation", "prepare")); result.Result == ResultApplied {
		t.Fatal("resolver prepare accepted a foreign reservation")
	}
	provisional = pinInputs(a, reserve.Reservation, "prepare")
	prepared := CheckAgentInputs(ctx, inputDeps, provisional)
	if prepared.Result != ResultApplied {
		t.Fatalf("provisional resolver prepare failed: result=%s reason=%s", prepared.Result, prepared.Reason)
	}
	if prepared.Observation.RootIdentity.Platform == "" {
		t.Fatal("resolver prepare returned no root witness")
	}

	a.RootIdentity = &prepared.Observation.RootIdentity
	if result := CheckAgentInputs(ctx, inputDeps, pinInputs(a, reserve.Reservation, "prepare")); result.Result != ResultApplied {
		t.Fatalf("final resolver prepare failed: result=%s reason=%s", result.Result, result.Reason)
	}
	if result := CheckAgentInputs(ctx, inputDeps, pinInputs(a, reserve.Reservation, "dispatch")); result.Result != ResultApplied {
		t.Fatalf("final resolver dispatch failed: result=%s reason=%s", result.Result, result.Reason)
	}
	if result := CheckAgentInputs(ctx, inputDeps, pinInputs(a, reserve.Reservation, "entry")); result.Result != ResultApplied {
		t.Fatalf("final resolver entry failed: result=%s reason=%s", result.Result, result.Reason)
	}
}
