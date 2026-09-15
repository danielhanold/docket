//go:build integration

package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/codexcontract"
	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/testsupport"
)

func reviewPin(t *testing.T, path string, value any) string {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func reviewRealAssignment(t *testing.T, symlinkOutput bool) (codexcontract.Assignment, AgentInputDeps, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(testsupport.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	primary, feature := filepath.Join(root, "primary"), filepath.Join(root, "feature")
	runGit(t, root, "init", "-b", "main", primary)
	runGit(t, primary, "config", "user.name", "Review")
	runGit(t, primary, "config", "user.email", "review@example.invalid")
	writeRepoFile(t, primary, "README.md", "base\n")
	if symlinkOutput {
		if err := os.Symlink(primary, filepath.Join(primary, "docs")); err != nil {
			t.Fatal(err)
		}
	}
	runGit(t, primary, "add", "README.md")
	if symlinkOutput {
		runGit(t, primary, "add", "docs")
	}
	runGit(t, primary, "commit", "-m", "base")
	runGit(t, primary, "worktree", "add", "-b", "codex/review-inputs", feature, "HEAD")
	client, err := gitcli.NewClient()
	if err != nil {
		t.Fatal(err)
	}
	repo, err := client.Discover(context.Background(), gitcli.DiscoverOptions{InvocationPath: primary})
	if err != nil {
		t.Fatal(err)
	}
	worktree, err := client.DiscoverWorktree(context.Background(), gitcli.DiscoverOptions{InvocationPath: feature})
	if err != nil {
		t.Fatal(err)
	}
	identity, err := codexcontract.ObserveRootIdentity(worktree.Root, worktree.GitDir)
	if err != nil {
		t.Fatal(err)
	}
	templatePath := filepath.Join(worktree.Root, ".agents", "skills", "docket-implement-next", "results-template.md")
	if err := os.MkdirAll(filepath.Dir(templatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	catalog, err := assets.EmbeddedCatalog()
	if err != nil {
		t.Fatal(err)
	}
	templateBody, err := catalog.Bytes("skills/docket-implement-next/results-template.md")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(templatePath, templateBody, 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, feature, "add", ".agents")
	runGit(t, feature, "commit", "-m", "fixture: install candidate assets")
	head := runGit(t, feature, "rev-parse", "HEAD")
	binary := writeDocketVersionStub(t, root, head)
	templateSum := sha256.Sum256(templateBody)
	a := codexcontract.Assignment{
		SchemaVersion: 1, ChangeID: 425, Role: "docket-plan-writer", Phase: "plan", Mode: "fresh",
		Primary: repo.PrimaryWorktree, Feature: worktree.Root, CommonDir: repo.CommonDir,
		Branch: "codex/review-inputs", EntryHEAD: head, MetadataRevision: head,
		ChangePath: "docs/changes/active/0425-review.md", ArtifactPath: "docs/plan.md",
		DocketExecutable: binary, DocketCommit: head, ReadRoots: []string{root},
		WritePaths: []string{"docs/plan.md"}, RootIdentity: &identity,
		PlanSkill: "auto", BuildSkill: "auto", ResultsTemplate: "results-template",
		Resources:            []codexcontract.Resource{{LogicalID: "results-template", Path: templatePath, SHA256: hex.EncodeToString(templateSum[:]), Source: "asset-set:" + catalog.Manifest.AssetSetID}},
		ResourceDependencies: map[string][]string{"results-template": {}},
	}
	deps, err := NewAgentInputDeps(binary)
	if err != nil {
		t.Fatal(err)
	}
	deps.Workspace = &inputWorkspace{}
	return a, deps, root
}

func reviewCheck(t *testing.T, a codexcontract.Assignment, deps AgentInputDeps, root, stage string) CheckInputsResult {
	t.Helper()
	assignmentPath := filepath.Join(root, "assignment.json")
	assignmentDigest := reviewPin(t, assignmentPath, a)
	req := CheckInputsRequest{Assignment: assignmentPath, SHA256: assignmentDigest, Stage: stage, RepoDir: a.Primary}
	if stage != "active" {
		payload := codexcontract.WorkerPayload{SchemaVersion: 1, Kind: "planner", AssignmentPath: assignmentPath, AssignmentSHA256: assignmentDigest, EntryArgv: codexcontract.EntryCheckerArgv(a, assignmentPath, assignmentDigest), TaskText: "write the plan"}
		payloadPath := filepath.Join(root, "payload.json")
		req.Payload, req.PayloadSHA256 = payloadPath, reviewPin(t, payloadPath, payload)
	}
	return CheckAgentInputs(context.Background(), deps, req)
}

func TestIntegrationWorkflowAgentEntryRejectsSymlinkedOutputOutsideFeature(t *testing.T) {
	a, deps, root := reviewRealAssignment(t, true)
	result := reviewCheck(t, a, deps, root, "entry")
	if result.Result == ResultApplied {
		t.Fatal("entry accepted an output path redirected through a symlink")
	}
}

func TestIntegrationWorkflowAgentEntryUsesProductionCommonDirectoryScopeIdentity(t *testing.T) {
	for _, scenario := range []string{"valid", "foreign-repository"} {
		t.Run(scenario, func(t *testing.T) {
			a, deps, root := reviewRealAssignment(t, false)
			a.Role, a.Phase, a.TaskID = "docket-build-standard", "build", "1"
			a.ArtifactPath, a.PlanSkill, a.BuildSkill, a.ResultsTemplate, a.Resources = "", "", "", "", nil
			a.ResourceDependencies = nil
			a.WritePaths = []string{"owned.go"}
			repoIdentity := a.CommonDir
			if scenario == "foreign-repository" {
				repoIdentity = filepath.Join(root, "foreign.git")
			}
			store := gatedrive.OpenStore(a.CommonDir)
			grant, err := store.PrepareScope(gatedrive.ScopeRequest{RepoIdentity: repoIdentity, Worktree: a.Feature, ChangeID: "425", TaskID: "1", Phase: "build", Branch: a.Branch})
			if err != nil {
				t.Fatal(err)
			}
			driver := gatedrive.NewSystemDriver(store, nil)
			deps.Scope = driver
			assignmentPath := filepath.Join(root, "assignment.json")
			assignmentDigest := reviewPin(t, assignmentPath, a)
			payload := codexcontract.WorkerPayload{SchemaVersion: 1, Kind: "worker", AssignmentPath: assignmentPath, AssignmentSHA256: assignmentDigest, EntryArgv: codexcontract.EntryCheckerArgv(a, assignmentPath, assignmentDigest), TaskText: "implement task", ScopeID: grant.ScopeID, ChildCapability: grant.ChildCapability}
			payloadPath := filepath.Join(root, "payload.json")
			payloadDigest := reviewPin(t, payloadPath, payload)
			result := CheckAgentInputs(context.Background(), deps, CheckInputsRequest{Assignment: assignmentPath, SHA256: assignmentDigest, Payload: payloadPath, PayloadSHA256: payloadDigest, Stage: "entry", RepoDir: a.Primary})
			if scenario == "valid" && result.Result != ResultApplied {
				t.Fatalf("valid production scope rejected: %s", result.Reason)
			}
			if scenario == "foreign-repository" && (result.Result == ResultApplied || !strings.Contains(result.Reason, "scope-identity-mismatch")) {
				t.Fatalf("foreign repository scope result=%s reason=%s", result.Result, result.Reason)
			}
		})
	}
}

func TestIntegrationWorkflowAgentScopeValidatorWiresEpochCancellation(t *testing.T) {
	a, _, root := reviewRealAssignment(t, false)
	a.Role, a.Phase, a.TaskID = "docket-build-standard", "build", "1"
	key := mintTestGateKey(t, a.CommonDir)
	epoch, err := MintEpochRecord(a.CommonDir, key, "425")
	if err != nil {
		t.Fatal(err)
	}
	if err := epochCAS(a.CommonDir, key, func(record *EpochRecord) error {
		record.State = EpochCancelled
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	store := gatedrive.OpenStore(a.CommonDir)
	grant, err := store.PrepareScope(gatedrive.ScopeRequest{RepoIdentity: a.CommonDir, Worktree: a.Feature, ChangeID: "425", TaskID: "1", Phase: "build", Branch: a.Branch, RunEpochID: epoch.EpochID})
	if err != nil {
		t.Fatal(err)
	}
	validator, err := NewAgentScopeValidator(a.CommonDir, writeDocketVersionStub(t, root, a.EntryHEAD))
	if err != nil {
		t.Fatal(err)
	}
	err = validator.ValidateChildInputs(gatedrive.StartRequest{RepoDir: a.CommonDir, Worktree: a.Feature, ChangeID: "425", TaskID: "1", Phase: "build", Branch: a.Branch, ScopeID: grant.ScopeID, ChildCapability: grant.ChildCapability, RunEpochID: epoch.EpochID})
	if err == nil {
		t.Fatal("production scope validator accepted a cancelled epoch")
	}
}

func TestIntegrationWorkflowPlannerEntryRequiresPreparedPlanningResources(t *testing.T) {
	a, deps, root := reviewRealAssignment(t, false)
	a.PlanSkill, a.BuildSkill, a.ResultsTemplate, a.Resources = "", "", "", nil
	a.ResourceDependencies = nil
	if result := reviewCheck(t, a, deps, root, "entry"); result.Result == ResultApplied {
		t.Fatal("planner entry accepted without selected skills and results template")
	}
}

func TestIntegrationWorkflowPlannerActiveRejectsExtraAssignedArtifact(t *testing.T) {
	a, deps, root := reviewRealAssignment(t, false)
	a.WritePaths = append(a.WritePaths, "source.go")
	writeRepoFile(t, a.Feature, "source.go", "package extra\n")
	runGit(t, a.Feature, "add", "source.go")
	runGit(t, a.Feature, "commit", "-m", "extra artifact")
	if result := reviewCheck(t, a, deps, root, "active"); result.Result == ResultApplied {
		t.Fatal("planner active accepted a source artifact")
	}
}

func TestIntegrationWorkflowRepairActiveRequiresPrivateRoleAuthority(t *testing.T) {
	a, deps, root := reviewRealAssignment(t, false)
	a.Role, a.Phase, a.Mode = "docket-integration-repair", "repair", "repair"
	a.ArtifactPath, a.PlanSkill, a.BuildSkill, a.ResultsTemplate, a.Resources = "", "", "", "", nil
	a.ResourceDependencies = nil
	a.WritePaths = []string{"repair.go"}
	deps.Role = &inputRole{}
	assignmentPath := filepath.Join(root, "assignment.json")
	assignmentDigest := reviewPin(t, assignmentPath, a)
	without := CheckAgentInputs(context.Background(), deps, CheckInputsRequest{Assignment: assignmentPath, SHA256: assignmentDigest, Stage: "active", RepoDir: a.Primary})
	if without.Result == ResultApplied {
		t.Fatal("repair active check accepted no role authority")
	}
	payload := codexcontract.WorkerPayload{SchemaVersion: 1, Kind: "repair", AssignmentPath: assignmentPath, AssignmentSHA256: assignmentDigest, EntryArgv: codexcontract.EntryCheckerArgv(a, assignmentPath, assignmentDigest), TaskText: "repair integration", Attempt: "attempt-1"}
	payloadPath := filepath.Join(root, "payload.json")
	payloadDigest := reviewPin(t, payloadPath, payload)
	with := CheckAgentInputs(context.Background(), deps, CheckInputsRequest{Assignment: assignmentPath, SHA256: assignmentDigest, Payload: payloadPath, PayloadSHA256: payloadDigest, Stage: "active", RepoDir: a.Primary})
	if with.Result != ResultApplied {
		t.Fatalf("repair active check rejected valid authority: %s", with.Reason)
	}
}

func TestIntegrationWorkflowWorkerActiveSupportsAssignmentOnlyAndValidatedPayload(t *testing.T) {
	a, deps, root := reviewRealAssignment(t, false)
	a.Role, a.Phase, a.TaskID = "docket-build-standard", "build", "1"
	a.ArtifactPath, a.PlanSkill, a.BuildSkill, a.ResultsTemplate, a.Resources = "", "", "", "", nil
	a.ResourceDependencies = nil
	a.WritePaths = []string{"owned.go"}
	assignmentPath := filepath.Join(root, "assignment.json")
	assignmentDigest := reviewPin(t, assignmentPath, a)
	without := CheckAgentInputs(context.Background(), deps, CheckInputsRequest{Assignment: assignmentPath, SHA256: assignmentDigest, Stage: "active", RepoDir: a.Primary})
	if without.Result != ResultApplied {
		t.Fatalf("worker assignment-only active check rejected: %s", without.Reason)
	}

	store := gatedrive.OpenStore(a.CommonDir)
	grant, err := store.PrepareScope(gatedrive.ScopeRequest{RepoIdentity: a.CommonDir, Worktree: a.Feature, ChangeID: "425", TaskID: "1", Phase: "build", Branch: a.Branch})
	if err != nil {
		t.Fatal(err)
	}
	deps.Scope = gatedrive.NewSystemDriver(store, nil)
	payload := codexcontract.WorkerPayload{SchemaVersion: 1, Kind: "worker", AssignmentPath: assignmentPath, AssignmentSHA256: assignmentDigest, EntryArgv: codexcontract.EntryCheckerArgv(a, assignmentPath, assignmentDigest), TaskText: "implement task", ScopeID: grant.ScopeID, ChildCapability: grant.ChildCapability}
	payloadPath := filepath.Join(root, "payload.json")
	payloadDigest := reviewPin(t, payloadPath, payload)
	with := CheckAgentInputs(context.Background(), deps, CheckInputsRequest{Assignment: assignmentPath, SHA256: assignmentDigest, Payload: payloadPath, PayloadSHA256: payloadDigest, Stage: "active", RepoDir: a.Primary})
	if with.Result != ResultApplied {
		t.Fatalf("worker active check rejected validated payload: %s", with.Reason)
	}
}
