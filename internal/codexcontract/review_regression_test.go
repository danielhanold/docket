package codexcontract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func reviewAssignment() Assignment {
	return Assignment{
		SchemaVersion: 1, ChangeID: 425, Role: "docket-plan-writer", Phase: "plan", Mode: "fresh",
		Primary: "/repo", Feature: "/repo/feature", CommonDir: "/repo/.git", Branch: "codex/audit",
		EntryHEAD: strings.Repeat("a", 40), MetadataRevision: strings.Repeat("b", 40),
		ChangePath: "docs/changes/active/0425-audit.md", ArtifactPath: "docs/plan.md",
		DocketExecutable: "/candidate/docket", DocketCommit: strings.Repeat("c", 40),
		ReadRoots: []string{"/candidate"}, WritePaths: []string{"docs/plan.md"},
	}
}

func reviewSHA256(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func TestAssignmentRejectsUnsupportedFullObjectIDLengths(t *testing.T) {
	for _, n := range []int{40, 64} {
		a := reviewAssignment()
		a.EntryHEAD = strings.Repeat("a", n)
		a.MetadataRevision = strings.Repeat("b", n)
		a.DocketCommit = strings.Repeat("c", n)
		if err := ValidateAssignment(a); err != nil {
			t.Fatalf("valid %d-character object IDs rejected: %v", n, err)
		}
	}
	for _, n := range []int{39, 41, 48, 63, 65} {
		a := reviewAssignment()
		a.EntryHEAD = strings.Repeat("a", n)
		if ValidateAssignment(a) == nil {
			t.Errorf("accepted %d-character object ID", n)
		}
	}
	a := reviewAssignment()
	a.MetadataRevision = strings.Repeat("g", 40)
	if ValidateAssignment(a) == nil {
		t.Fatal("accepted a non-hexadecimal object ID")
	}
}

func TestPlannerAssignmentRequiresSolePlanArtifact(t *testing.T) {
	for _, scenario := range []string{"missing-artifact", "extra-write", "metadata-write"} {
		t.Run(scenario, func(t *testing.T) {
			a := reviewAssignment()
			switch scenario {
			case "missing-artifact":
				a.ArtifactPath = ""
			case "extra-write":
				a.WritePaths = append(a.WritePaths, "source.go")
			case "metadata-write":
				a.WritePaths = []string{".git/config"}
			}
			if ValidateAssignment(a) == nil {
				t.Fatal("accepted planner assignment violating sole-artifact boundary")
			}
		})
	}
}

func TestExplicitAutoPlanningRequiresNoResourcePackage(t *testing.T) {
	a := Assignment{PlanSkill: "auto", BuildSkill: "auto"}
	if err := ValidateResources(a); err != nil {
		t.Fatalf("explicit auto mode rejected: %v", err)
	}
}

func TestPlannerResourcesRequireSelectionsAndResultsTemplate(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	makeResource := func(id, name, body string) Resource {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return Resource{LogicalID: id, Path: path, SHA256: reviewSHA256([]byte(body)), Source: "package:test"}
	}
	plan := makeResource("plan", "plan.md", "plan\n")
	build := makeResource("build", "build.md", "build\n")
	template := makeResource("results-template", "skills/docket-implement-next/results-template.md", "results\n")
	decoy := makeResource("decoy-template", "other/results-template.md", "decoy\n")
	valid := reviewAssignment()
	valid.PlanSkill, valid.BuildSkill, valid.ResultsTemplate = plan.LogicalID, build.LogicalID, template.LogicalID
	valid.ReadRoots = []string{root}
	valid.Resources = []Resource{plan, build, template}
	valid.ResourceDependencies = map[string][]string{plan.LogicalID: {}, build.LogicalID: {}, template.LogicalID: {}}
	if err := ValidateResources(valid); err != nil {
		t.Fatalf("complete named planner resources: %v", err)
	}
	auto := valid
	auto.PlanSkill, auto.BuildSkill = "auto", "auto"
	auto.Resources = []Resource{template}
	auto.ResourceDependencies = map[string][]string{template.LogicalID: {}}
	if err := ValidateResources(auto); err != nil {
		t.Fatalf("explicit auto planner resources: %v", err)
	}
	for _, scenario := range []string{"missing-plan-selection", "missing-build-selection", "missing-template-selection", "missing-template-resource", "wrong-template-package", "mutated-template"} {
		t.Run(scenario, func(t *testing.T) {
			a := valid
			a.Resources = append([]Resource(nil), valid.Resources...)
			switch scenario {
			case "missing-plan-selection":
				a.PlanSkill = ""
			case "missing-build-selection":
				a.BuildSkill = ""
			case "missing-template-selection":
				a.ResultsTemplate = ""
			case "missing-template-resource":
				a.Resources = a.Resources[:2]
			case "wrong-template-package":
				a.ResultsTemplate = decoy.LogicalID
				a.Resources = append(a.Resources, decoy)
			case "mutated-template":
				if err := os.WriteFile(template.Path, []byte("changed\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.WriteFile(template.Path, []byte("results\n"), 0o600) })
			}
			if ValidateResources(a) == nil {
				t.Fatal("accepted incomplete planner preparation")
			}
		})
	}
}

func TestPlannerUnknownSelectorIsReportedBeforeTemplateIdentity(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "skills", "docket-implement-next", "results-template.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte("results\n")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	a := reviewAssignment()
	a.ReadRoots = []string{root}
	a.PlanSkill = "superpowers:writing-plans"
	a.BuildSkill = "docket-build"
	a.ResultsTemplate = path
	a.Resources = []Resource{{LogicalID: "results-template", Path: path, SHA256: reviewSHA256(body), Source: "fixture-manifest"}}
	err = ValidateResources(a)
	if err == nil || err.Error() != `selected resource "superpowers:writing-plans" is not declared` {
		t.Fatalf("unknown selector error = %v", err)
	}
}

func TestPlannerResourceDependenciesCoverEveryResourceAndLocalLink(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	makeResource := func(id, name, body string) Resource {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return Resource{LogicalID: id, Path: path, SHA256: reviewSHA256([]byte(body)), Source: "fixture-manifest"}
	}
	build := makeResource("build-skill", "build/SKILL.md", "Read [routing](references/task-routing.md).\n")
	routing := makeResource("build-task-routing", "build/references/task-routing.md", "route\n")
	template := makeResource("results-template", "skills/docket-implement-next/results-template.md", "results\n")
	a := reviewAssignment()
	a.ReadRoots = []string{root}
	a.PlanSkill, a.BuildSkill, a.ResultsTemplate = "auto", build.LogicalID, template.LogicalID
	a.Resources = []Resource{build, routing, template}
	if err := ValidateResources(a); err == nil || !strings.Contains(err.Error(), "resource_dependencies omits resource") {
		t.Fatalf("planner without complete dependency keys: %v", err)
	}
	a.ResourceDependencies = map[string][]string{
		build.LogicalID:    {},
		routing.LogicalID:  {},
		template.LogicalID: {},
	}
	if err := ValidateResources(a); err == nil || !strings.Contains(err.Error(), "omits direct local dependency") {
		t.Fatalf("planner without direct link edge: %v", err)
	}
	a.ResourceDependencies[build.LogicalID] = []string{routing.LogicalID}
	if err := ValidateResources(a); err != nil {
		t.Fatalf("complete planner dependency graph: %v", err)
	}
}

func TestResourceClosureRejectsUndeclaredAbsoluteLocalLink(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "SKILL.md")
	body := []byte("Read [mandatory](/unavailable/mandatory.md) before acting.\n")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	r := Resource{LogicalID: "skill", Path: path, SHA256: reviewSHA256(body), Source: "package:audit"}
	if ValidateResources(Assignment{ReadRoots: []string{root}, Resources: []Resource{r}, PlanSkill: "skill"}) == nil {
		t.Fatal("accepted undeclared absolute local dependency")
	}
}

func TestResourceClosureAcceptsDeclaredAbsoluteLocalAndExternalLinks(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dependencyPath := filepath.Join(root, "mandatory.md")
	dependencyBody := []byte("dependency\n")
	if err := os.WriteFile(dependencyPath, dependencyBody, 0o600); err != nil {
		t.Fatal(err)
	}
	skillPath := filepath.Join(root, "SKILL.md")
	skillBody := []byte("Read [mandatory](" + dependencyPath + ") and [documentation](https://example.invalid/reference).\n")
	if err := os.WriteFile(skillPath, skillBody, 0o600); err != nil {
		t.Fatal(err)
	}
	a := Assignment{ReadRoots: []string{root}, PlanSkill: "skill", Resources: []Resource{
		{LogicalID: "skill", Path: skillPath, SHA256: reviewSHA256(skillBody), Source: "package:test"},
		{LogicalID: "dependency", Path: dependencyPath, SHA256: reviewSHA256(dependencyBody), Source: "package:test"},
	}}
	if err := ValidateResources(a); err != nil {
		t.Fatalf("declared absolute dependency and external hyperlink: %v", err)
	}
}

func TestReceiptRejectsMalformedDriveSemantics(t *testing.T) {
	base := map[string]any{
		"protocol_version": 1,
		"operation":        "gate.drive.start",
		"result":           "applied",
		"drive": map[string]any{
			"protocol_version": 1,
			"drive_id":         "drive",
			"generation":       "owner",
			"deadline":         "2026-09-15T12:00:00Z",
			"outcome":          "PASSED",
			"run_root":         "/private/audit-runs",
			"raw_run_dir":      "/private/audit-runs/run",
		},
	}
	body, _ := json.Marshal(base)
	if _, err := ParseReceipt("gate.drive.start", body, nil, 0, Assignment{RunRoot: "/private/audit-runs"}); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"missing-outcome", "unknown-outcome", "missing-deadline", "unknown-result", "failed-exit-with-pass", "failed-with-raw-path", "waiting-with-raw-path"} {
		t.Run(scenario, func(t *testing.T) {
			var wire map[string]any
			_ = json.Unmarshal(body, &wire)
			drive := wire["drive"].(map[string]any)
			exitCode := 0
			switch scenario {
			case "missing-outcome":
				delete(drive, "outcome")
			case "unknown-outcome":
				drive["outcome"] = "BANANA"
			case "missing-deadline":
				delete(drive, "deadline")
			case "unknown-result":
				wire["result"] = "BANANA"
			case "failed-exit-with-pass":
				exitCode = 2
			case "failed-with-raw-path":
				drive["outcome"] = "FAILED"
			case "waiting-with-raw-path":
				drive["outcome"] = "WAITING"
				delete(drive, "run_root")
			}
			mutated, _ := json.Marshal(wire)
			if receipt, err := ParseReceipt("gate.drive.start", mutated, []byte("original diagnostic"), exitCode, Assignment{RunRoot: "/private/audit-runs"}); err == nil {
				t.Fatalf("accepted malformed receipt: classification=%q result=%q", receipt.Classification, receipt.Result)
			}
		})
	}
}

func TestWorkerPayloadTaggedUnionRejectsForeignAuthority(t *testing.T) {
	for _, kind := range []string{"planner", "review", "worker", "repair"} {
		t.Run(kind, func(t *testing.T) {
			a := reviewAssignment()
			switch kind {
			case "review":
				a.Role, a.Phase, a.Mode = "docket-review-standard", "review", "review"
				a.ReviewBase, a.ReviewHEAD, a.EntryHEAD, a.BuildEvidence = strings.Repeat("e", 40), strings.Repeat("a", 40), strings.Repeat("a", 40), "evidence"
				a.Resources = []Resource{{LogicalID: "evidence"}}
			case "worker":
				a.Role, a.Phase, a.TaskID = "docket-build-standard", "build", "1"
			case "repair":
				a.Role, a.Phase, a.Mode = "docket-integration-repair", "repair", "repair"
			}
			p := WorkerPayload{SchemaVersion: 1, Kind: kind, AssignmentPath: "/private/a.json", AssignmentSHA256: strings.Repeat("d", 64), TaskText: "task", Attempt: "unrelated-finalize-authority", ResolverReservation: "unrelated-resolver-authority"}
			p.EntryArgv = EntryCheckerArgv(a, p.AssignmentPath, p.AssignmentSHA256)
			if kind == "worker" {
				p.ScopeID, p.ChildCapability = "scope", "child"
			}
			if kind == "repair" {
				p.PredecessorDriveID, p.PredecessorOwnerGen = "unexpected-worker-drive", "unexpected-worker-generation"
			}
			if ValidateWorkerPayload(p, a) == nil {
				t.Fatal("accepted fields from a different authority variant")
			}
		})
	}
}

func TestPayloadVariantsRejectEachForeignAuthorityField(t *testing.T) {
	type variant struct {
		assignment Assignment
		payload    WorkerPayload
		foreign    map[string]func(*WorkerPayload)
	}
	base := func(kind, role, phase, mode string) variant {
		a := reviewAssignment()
		a.Role, a.Phase, a.Mode = role, phase, mode
		p := WorkerPayload{SchemaVersion: 1, Kind: kind, AssignmentPath: "/private/a.json", AssignmentSHA256: strings.Repeat("d", 64), TaskText: "task"}
		return variant{assignment: a, payload: p}
	}
	workerFields := map[string]func(*WorkerPayload){
		"scope":       func(p *WorkerPayload) { p.ScopeID, p.ChildCapability = "scope", "child" },
		"context":     func(p *WorkerPayload) { p.GateContext = "context" },
		"epoch":       func(p *WorkerPayload) { p.RunEpochID = "epoch" },
		"predecessor": func(p *WorkerPayload) { p.PredecessorDriveID, p.PredecessorOwnerGen = "drive", "generation" },
		"recovered": func(p *WorkerPayload) {
			p.Recovered = &RecoveredPayload{ScopeID: "scope", DriveID: "drive", OwnerGeneration: "generation"}
		},
	}
	variants := map[string]variant{}
	for _, kind := range []string{"planner", "review"} {
		role, phase, mode := "docket-plan-writer", "plan", "fresh"
		if kind == "review" {
			role, phase, mode = "docket-review-standard", "review", "review"
		}
		v := base(kind, role, phase, mode)
		v.foreign = map[string]func(*WorkerPayload){"scope": workerFields["scope"], "context": workerFields["context"], "epoch": workerFields["epoch"], "predecessor": workerFields["predecessor"], "recovered": workerFields["recovered"], "attempt": func(p *WorkerPayload) { p.Attempt = "attempt" }, "reservation": func(p *WorkerPayload) { p.ResolverReservation = "reservation" }}
		variants[kind] = v
	}
	worker := base("worker", "docket-build-standard", "build", "fresh")
	worker.assignment.TaskID = "1"
	worker.payload.ScopeID, worker.payload.ChildCapability = "scope", "child"
	worker.foreign = map[string]func(*WorkerPayload){"attempt": func(p *WorkerPayload) { p.Attempt = "attempt" }, "reservation": func(p *WorkerPayload) { p.ResolverReservation = "reservation" }}
	variants["worker"] = worker
	resolver := base("resolver", "docket-rebase-resolver", "resolver", "resolver")
	resolver.payload.Attempt, resolver.payload.ResolverReservation = "attempt", "reservation"
	resolver.foreign = workerFields
	variants["resolver"] = resolver
	repair := base("repair", "docket-integration-repair", "repair", "repair")
	repair.payload.Attempt = "attempt"
	repair.foreign = map[string]func(*WorkerPayload){"scope": workerFields["scope"], "context": workerFields["context"], "epoch": workerFields["epoch"], "predecessor": workerFields["predecessor"], "recovered": workerFields["recovered"], "reservation": func(p *WorkerPayload) { p.ResolverReservation = "reservation" }}
	variants["repair"] = repair
	for name, v := range variants {
		t.Run(name, func(t *testing.T) {
			v.payload.EntryArgv = EntryCheckerArgv(v.assignment, v.payload.AssignmentPath, v.payload.AssignmentSHA256)
			if err := ValidateWorkerPayload(v.payload, v.assignment); err != nil {
				t.Fatalf("valid %s payload: %v", name, err)
			}
			for field, mutate := range v.foreign {
				t.Run(field, func(t *testing.T) {
					p := v.payload
					mutate(&p)
					if ValidateWorkerPayload(p, v.assignment) == nil {
						t.Fatalf("%s payload accepted foreign %s authority", name, field)
					}
				})
			}
		})
	}
}
