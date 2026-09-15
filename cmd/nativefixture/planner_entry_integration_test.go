//go:build integration

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/app"
	"github.com/danielhanold/docket/internal/codexcontract"
)

// Exercise the generated entry command against real CLI/workspace dependencies.
// App-only tests supplying RepoDir cannot catch an omitted CLI default.
func checkNativePlannerEntryDefaultsToStartup(t *testing.T, root, destination, binary string, fixture manifest) {
	t.Helper()
	for _, path := range []*string{&root, &destination, &binary} {
		canonical, err := filepath.EvalSymlinks(*path)
		if err != nil {
			t.Fatal(err)
		}
		*path = canonical
	}
	primary := filepath.Join(destination, "primary")
	invoke := func(dir string, args ...string) []byte {
		t.Helper()
		cmd := exec.Command(binary, args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("candidate %v: %v: %s", args, err, out)
		}
		return out
	}
	var status struct {
		Context struct {
			MetadataRevision string `json:"metadata_revision"`
		} `json:"context"`
		Changes []struct {
			ID      int    `json:"id"`
			Version string `json:"version"`
		} `json:"changes"`
	}
	readStatus := func() string {
		t.Helper()
		if err := json.Unmarshal(invoke(primary, "status", "--repo-dir", primary, "--json"), &status); err != nil {
			t.Fatal(err)
		}
		for _, c := range status.Changes {
			if c.ID == fixture.ChangeID {
				return c.Version
			}
		}
		t.Fatal("fixture change missing")
		return ""
	}
	id := strconv.Itoa(fixture.ChangeID)
	invoke(primary, "change", "claim", "--id", id, "--version", readStatus(), "--repo-dir", primary, "--json")
	var workspace app.WorkspaceOpResult
	if err := json.Unmarshal(invoke(primary, "workspace", "prepare", "--id", id, "--version", readStatus(), "--repo-dir", primary, "--json"), &workspace); err != nil {
		t.Fatal(err)
	}
	if workspace.Path == "" || workspace.FeatureRef == "" {
		t.Fatalf("missing workspace identity: %+v", workspace)
	}
	readStatus()
	checkNativePlannerResourceContract(t, root, destination, binary, fixture, workspace, status.Context.MetadataRevision)
	template := filepath.Join(root, "skills", "docket-implement-next", "results-template.md")
	if err := os.MkdirAll(filepath.Dir(template), 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte("# Results\n")
	if err := os.WriteFile(template, body, 0o600); err != nil {
		t.Fatal(err)
	}
	a := codexcontract.Assignment{SchemaVersion: 1, ChangeID: fixture.ChangeID, Role: "docket-plan-writer", Phase: "plan", Mode: "fresh", Primary: primary, Feature: workspace.Path, CommonDir: filepath.Join(primary, ".git"), Branch: strings.TrimPrefix(workspace.FeatureRef, "refs/heads/"), EntryHEAD: fixture.PrimaryHEAD, MetadataRevision: status.Context.MetadataRevision, ChangePath: fixture.ChangePath, ArtifactPath: "docs/plans/native.md", DocketExecutable: binary, DocketCommit: fixture.SourceCommit, ReadRoots: []string{root}, WritePaths: []string{"docs/plans/native.md"}, PlanSkill: "auto", BuildSkill: "auto", ResultsTemplate: "template", Resources: []codexcontract.Resource{{LogicalID: "template", Path: template, SHA256: hash(body), Source: "package:docket-implement-next"}}, ResourceDependencies: map[string][]string{"template": {}}}
	assignmentPath := filepath.Join(root, "planner-assignment.json")
	writeJSON := func(path string, v any) string {
		t.Helper()
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, b, 0o600); err != nil {
			t.Fatal(err)
		}
		return hash(b)
	}
	digest := writeJSON(assignmentPath, a)
	var prepared app.CheckInputsResult
	if err := json.Unmarshal(invoke(primary, "agent", "check-inputs", "--assignment", assignmentPath, "--sha256", digest, "--stage", "prepare", "--repo-dir", primary, "--json"), &prepared); err != nil {
		t.Fatal(err)
	}
	identity := prepared.Observation.RootIdentity
	a.RootIdentity = &identity
	// This witness describes the feature root; GitDir is a path, not its inode.
	gitIdentity, err := codexcontract.ObserveRootIdentity(a.RootIdentity.GitDir, a.RootIdentity.GitDir)
	if err != nil {
		t.Fatal(err)
	}
	if gitIdentity.Inode == a.RootIdentity.Inode {
		t.Fatal("fixture does not distinguish root and Git-directory inode")
	}
	payloadPath := filepath.Join(root, "planner-payload.json")
	entry := func(dir string, explicit bool) app.CheckInputsResult {
		t.Helper()
		digest = writeJSON(assignmentPath, a)
		if err := json.Unmarshal(invoke(primary, "agent", "check-inputs", "--assignment", assignmentPath, "--sha256", digest, "--stage", "prepare", "--repo-dir", primary, "--json"), &prepared); err != nil {
			t.Fatal(err)
		}
		p := codexcontract.WorkerPayload{SchemaVersion: 1, Kind: "planner", AssignmentPath: assignmentPath, AssignmentSHA256: digest, EntryArgv: prepared.EntryArgv, TaskText: "Write the assigned plan"}
		pd := writeJSON(payloadPath, p)
		args := append(append([]string{}, p.EntryArgv[1:]...), "--payload", payloadPath, "--payload-sha256", pd)
		if explicit {
			args = append(args, "--repo-dir", primary)
		}
		cmd := exec.Command(p.EntryArgv[0], args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		var got app.CheckInputsResult
		if decodeErr := json.Unmarshal(out, &got); decodeErr != nil {
			t.Fatalf("entry: err=%v decode=%v out=%s", err, decodeErr, out)
		}
		return got
	}
	for _, dir := range []string{primary, workspace.Path} {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			got := entry(dir, false)
			if got.Result != app.ResultApplied {
				t.Fatalf("generated entry with omitted --repo-dir: result=%s reason=%s", got.Result, got.Reason)
			}
			if !a.RootIdentity.Equal(got.Observation.RootIdentity) {
				t.Fatal("entry changed the frozen root witness")
			}
		})
	}
	if got := entry(root, true); got.Result != app.ResultApplied {
		t.Fatalf("explicit repo-dir ignored: %s", got.Reason)
	}
	if got := entry(root, false); got.Result == app.ResultApplied {
		t.Fatal("foreign startup accepted without explicit repo-dir")
	}
	a.RootIdentity.Inode = gitIdentity.Inode
	if got := entry(primary, false); got.Reason != "root-identity-mismatch" {
		t.Fatalf("wrong root inode: result=%s reason=%s", got.Result, got.Reason)
	}
	// Continue through the real local-remote backlink/attachment/status boundary.
	// The metadata record intentionally does not live on the feature branch.
	planPath := "docs/superpowers/plans/native.md"
	if err := os.MkdirAll(filepath.Dir(filepath.Join(workspace.Path, planPath)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace.Path, planPath), []byte("# Plan\n\nAdd Double and test it.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	invoke(workspace.Path, "artifact", "backlink", "--artifact", planPath, "--change", fixture.ChangePath, "--repo-dir", workspace.Path, "--json")
	if err := run(workspace.Path, "git", "add", planPath); err != nil {
		t.Fatal(err)
	}
	if err := run(workspace.Path, "git", "commit", "-m", "docs: native plan\n\nDocket-Plan-Path: "+planPath); err != nil {
		t.Fatal(err)
	}
	head, err := gitOut(workspace.Path, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	invoke(primary, "change", "attach-plan", "--id", id, "--version", readStatus(), "--path", planPath, "--commit", head, "--repo-dir", primary, "--json")
	for _, dir := range []string{primary, workspace.Path} {
		var observed app.StatusResult
		if err := json.Unmarshal(invoke(dir, "status", "--repo-dir", dir, "--json"), &observed); err != nil {
			t.Fatal(err)
		}
		for _, finding := range observed.Findings {
			if finding.Code == "artifact-missing" && finding.Path == planPath {
				t.Errorf("official local backlink rejected after attachment from %s: %+v", dir, finding)
			}
		}
	}
}

func checkNativePlannerResourceContract(t *testing.T, root, destination, binary string, fixture manifest, workspace app.WorkspaceOpResult, metadataRevision string) {
	t.Helper()
	primary := filepath.Join(destination, "primary")
	planningRoot := filepath.Join(root, "planning-resources")
	if err := os.MkdirAll(planningRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	planSkillPath := filepath.Join(planningRoot, "SKILL.md")
	planPromptPath := filepath.Join(planningRoot, "plan-document-reviewer-prompt.md")
	if err := os.WriteFile(planSkillPath, []byte("Read [the plan reviewer prompt](plan-document-reviewer-prompt.md).\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(planPromptPath, []byte("Review the plan.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	resource := func(id, path, source string) codexcontract.Resource {
		t.Helper()
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return codexcontract.Resource{LogicalID: id, Path: path, SHA256: hash(body), Source: source}
	}
	buildRoot := filepath.Join(primary, ".agents", "skills", "docket-build")
	planResultsPath := filepath.Join(primary, ".agents", "skills", "docket-implement-next", "references", "codex-planning-results.md")
	resultsTemplatePath := filepath.Join(workspace.Path, ".agents", "skills", "docket-implement-next", "results-template.md")
	baseResources := []codexcontract.Resource{
		resource("plan-skill", planSkillPath, "planning-resources.json"),
		resource("plan-review-prompt", planPromptPath, "planning-resources.json"),
		resource("build-skill", filepath.Join(buildRoot, "SKILL.md"), "fixture-manifest"),
		resource("plan-results-contract", planResultsPath, "fixture-manifest"),
		resource("results-template", resultsTemplatePath, "fixture-manifest"),
	}
	a := codexcontract.Assignment{
		SchemaVersion: 1, ChangeID: fixture.ChangeID, Role: "docket-plan-writer", Phase: "plan", Mode: "fresh",
		Primary: primary, Feature: workspace.Path, CommonDir: filepath.Join(primary, ".git"), Branch: strings.TrimPrefix(workspace.FeatureRef, "refs/heads/"),
		EntryHEAD: fixture.PrimaryHEAD, MetadataRevision: metadataRevision, ChangePath: fixture.ChangePath,
		ArtifactPath: "docs/plans/resource-contract.md", DocketExecutable: binary, DocketCommit: fixture.SourceCommit,
		Resources: baseResources, ReadRoots: []string{root}, WritePaths: []string{"docs/plans/resource-contract.md"},
		ResourceDependencies: map[string][]string{
			"plan-skill":            {"plan-review-prompt"},
			"build-skill":           {},
			"plan-results-contract": {},
			"results-template":      {},
		},
		PlanSkill: "superpowers:writing-plans", BuildSkill: "docket-build", ResultsTemplate: resultsTemplatePath,
	}
	runAssignment := func(label string, assignment codexcontract.Assignment) (app.CheckInputsResult, int) {
		t.Helper()
		body, err := json.Marshal(assignment)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, "planner-resource-assignment-"+label+".json")
		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(binary, "agent", "check-inputs", "--assignment", path, "--sha256", hash(body), "--stage", "prepare", "--repo-dir", primary, "--json")
		cmd.Dir = primary
		out, runErr := cmd.CombinedOutput()
		code := 0
		if runErr != nil {
			code = cmd.ProcessState.ExitCode()
		}
		var result app.CheckInputsResult
		if err := json.Unmarshal(out, &result); err != nil {
			t.Fatalf("%s assignment: exit=%d err=%v decode=%v out=%s", label, code, runErr, err, out)
		}
		return result, code
	}
	malformed, code := runAssignment("retained-malformed", a)
	if code != 2 || malformed.Reason != `resources-invalid: selected resource "superpowers:writing-plans" is not declared` {
		t.Fatalf("retained malformed assignment: exit=%d result=%s reason=%q", code, malformed.Result, malformed.Reason)
	}
	a.PlanSkill, a.BuildSkill, a.ResultsTemplate = "plan-skill", "build-skill", "results-template"
	incomplete, code := runAssignment("selector-corrected-incomplete", a)
	if code != 2 || !strings.Contains(incomplete.Reason, `resource "build-skill" links to undeclared local dependency`) {
		t.Fatalf("selector-corrected incomplete assignment: exit=%d result=%s reason=%q", code, incomplete.Result, incomplete.Reason)
	}
	a.Resources = append(a.Resources,
		resource("build-task-routing", filepath.Join(buildRoot, "references", "task-routing.md"), "fixture-manifest"),
		resource("build-gate-execution", filepath.Join(buildRoot, "references", "gate-execution.md"), "fixture-manifest"),
		resource("build-gate-caller-loop", filepath.Join(buildRoot, "references", "gate-caller-loop.md"), "fixture-manifest"),
		resource("build-gate-execution-evidence", filepath.Join(buildRoot, "references", "gate-execution-evidence.md"), "fixture-manifest"),
	)
	a.ResourceDependencies = map[string][]string{
		"plan-skill":                    {"plan-review-prompt"},
		"plan-review-prompt":            {},
		"build-skill":                   {"build-task-routing", "build-gate-execution"},
		"build-task-routing":            {},
		"build-gate-execution":          {"build-gate-caller-loop", "build-gate-execution-evidence"},
		"build-gate-caller-loop":        {"build-gate-execution"},
		"build-gate-execution-evidence": {"build-gate-execution"},
		"plan-results-contract":         {},
		"results-template":              {},
	}
	complete, code := runAssignment("corrected-complete", a)
	if code != 0 || complete.Result != app.ResultApplied {
		t.Fatalf("corrected complete assignment: exit=%d result=%s reason=%q", code, complete.Result, complete.Reason)
	}
}
