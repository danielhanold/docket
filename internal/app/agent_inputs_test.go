package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/codexcontract"
	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/testsupport"
)

type inputObserver struct {
	out AgentRootObservation
	err error
}

func writeDocketVersionStub(t *testing.T, dir, commit string) string {
	t.Helper()
	canonicalDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(canonicalDir, "docket")
	body := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' '{\"commit\":\"%s\"}'\n", commit)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func (f inputObserver) ObserveAgentInputs(context.Context, codexcontract.Assignment, string) (AgentRootObservation, error) {
	return f.out, f.err
}

type inputScope struct {
	req   gatedrive.StartRequest
	calls int
}

type inputWorkspace struct {
	calls int
	err   error
}

type inputRole struct {
	calls   int
	payload codexcontract.WorkerPayload
}

func (r *inputRole) ValidateRoleInputs(_ context.Context, _ codexcontract.Assignment, p codexcontract.WorkerPayload, _ string) error {
	r.calls++
	r.payload = p
	return nil
}

func (f *inputWorkspace) ValidateAgentWorkspace(context.Context, codexcontract.Assignment, string) error {
	f.calls++
	return f.err
}

func TestCheckAgentInputsFinalizeRolesUsePrivateAuthorityAtEntry(t *testing.T) {
	for _, tc := range []struct {
		mode, role, phase, kind string
		workspaceCalls          int
	}{
		{"resolver", "docket-rebase-resolver", "resolver", "resolver", 0},
		{"repair", "docket-integration-repair", "repair", "repair", 1},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			dir, err := filepath.EvalSymlinks(testsupport.TempDir(t))
			if err != nil {
				t.Fatal(err)
			}
			commit := strings.Repeat("2", 40)
			docketPath := writeDocketVersionStub(t, dir, commit)
			identity := codexcontract.RootIdentity{Platform: "test", Device: 1, Inode: 2, GitDir: "/repo/.git/worktrees/wt"}
			a := codexcontract.Assignment{SchemaVersion: 1, ChangeID: 425, Role: tc.role, Phase: tc.phase, Mode: tc.mode, Primary: "/repo", Feature: "/repo/wt", CommonDir: "/repo/.git", Branch: "codex/change", EntryHEAD: strings.Repeat("0", 40), MetadataRevision: strings.Repeat("1", 40), ChangePath: "docs/changes/active/0425.md", DocketExecutable: docketPath, DocketCommit: commit, ReadRoots: []string{dir}, WritePaths: []string{"conflict.go"}, RootIdentity: &identity}
			ab, _ := json.Marshal(a)
			assignmentPath := filepath.Join(dir, "assignment.json")
			if err := os.WriteFile(assignmentPath, ab, 0o600); err != nil {
				t.Fatal(err)
			}
			as := sha256.Sum256(ab)
			ad := hex.EncodeToString(as[:])
			p := codexcontract.WorkerPayload{SchemaVersion: 1, Kind: tc.kind, AssignmentPath: assignmentPath, AssignmentSHA256: ad, EntryArgv: []string{docketPath, "agent", "check-inputs", "--assignment", assignmentPath, "--sha256", ad, "--stage", "entry", "--json"}, TaskText: "finalize child", Attempt: "attempt-1"}
			if tc.kind == "resolver" {
				p.ResolverReservation = "reservation-1"
			}
			pb, _ := json.Marshal(p)
			payloadPath := filepath.Join(dir, "payload.json")
			if err := os.WriteFile(payloadPath, pb, 0o600); err != nil {
				t.Fatal(err)
			}
			ps := sha256.Sum256(pb)
			role, ws := &inputRole{}, &inputWorkspace{}
			obs := inputObserver{out: AgentRootObservation{Primary: a.Primary, Feature: a.Feature, CommonDir: a.CommonDir, Branch: a.Branch, HEAD: a.EntryHEAD, Clean: true, RootIdentity: identity, EntryDescendant: true}}
			r := CheckAgentInputs(context.Background(), AgentInputDeps{Observer: obs, Workspace: ws, Role: role}, CheckInputsRequest{Assignment: assignmentPath, SHA256: ad, Payload: payloadPath, PayloadSHA256: hex.EncodeToString(ps[:]), Stage: "entry", RepoDir: a.Primary})
			if r.Result != ResultApplied {
				t.Fatalf("result=%s reason=%s", r.Result, r.Reason)
			}
			if role.calls != 1 || role.payload.Attempt != "attempt-1" || ws.calls != tc.workspaceCalls {
				t.Fatalf("role=%d workspace=%d payload=%+v", role.calls, ws.calls, role.payload)
			}
		})
	}
}

func TestCheckAgentInputsPlannerAndReviewerRequirePinnedPayloadAtEntry(t *testing.T) {
	for _, tc := range []struct {
		name, role, phase, mode, kind string
	}{
		{"planner", "docket-plan-writer", "plan", "fresh", "planner"},
		{"reviewer", "docket-review-standard", "review", "review", "review"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, err := filepath.EvalSymlinks(testsupport.TempDir(t))
			if err != nil {
				t.Fatal(err)
			}
			commit := strings.Repeat("2", 40)
			docketPath := writeDocketVersionStub(t, dir, commit)
			identity := codexcontract.RootIdentity{Platform: "test", Device: 1, Inode: 2, GitDir: "/repo/.git/worktrees/wt"}
			a := codexcontract.Assignment{SchemaVersion: 1, ChangeID: 425, Role: tc.role, Phase: tc.phase, Mode: tc.mode, Primary: "/repo", Feature: "/repo/wt", CommonDir: "/repo/.git", Branch: "codex/change", EntryHEAD: commit, MetadataRevision: strings.Repeat("1", 40), ChangePath: "docs/changes/active/0425.md", DocketExecutable: docketPath, DocketCommit: commit, ReadRoots: []string{dir}, RootIdentity: &identity}
			if tc.kind == "planner" {
				a.ArtifactPath = "docs/plans/425.md"
				a.WritePaths = []string{a.ArtifactPath}
			} else {
				evidenceBody := []byte("green at pinned head\n")
				evidencePath := filepath.Join(dir, "build-evidence.md")
				if err := os.WriteFile(evidencePath, evidenceBody, 0o600); err != nil {
					t.Fatal(err)
				}
				evidenceHash := sha256.Sum256(evidenceBody)
				a.ReviewBase = strings.Repeat("0", 40)
				a.ReviewHEAD = commit
				a.BuildEvidence = "build-evidence"
				a.Resources = []codexcontract.Resource{{LogicalID: a.BuildEvidence, Path: evidencePath, SHA256: hex.EncodeToString(evidenceHash[:]), Source: "controller"}}
			}
			ab, err := json.Marshal(a)
			if err != nil {
				t.Fatal(err)
			}
			assignmentPath := filepath.Join(dir, "assignment.json")
			if err := os.WriteFile(assignmentPath, ab, 0o600); err != nil {
				t.Fatal(err)
			}
			assignmentSum := sha256.Sum256(ab)
			assignmentDigest := hex.EncodeToString(assignmentSum[:])
			payload := codexcontract.WorkerPayload{SchemaVersion: 1, Kind: tc.kind, AssignmentPath: assignmentPath, AssignmentSHA256: assignmentDigest, EntryArgv: codexcontract.EntryCheckerArgv(a, assignmentPath, assignmentDigest), TaskText: tc.name + " assignment"}
			payloadBytes, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			payloadPath := filepath.Join(dir, "payload.json")
			if err := os.WriteFile(payloadPath, payloadBytes, 0o600); err != nil {
				t.Fatal(err)
			}
			payloadSum := sha256.Sum256(payloadBytes)
			payloadDigest := hex.EncodeToString(payloadSum[:])
			deps := AgentInputDeps{Observer: inputObserver{out: AgentRootObservation{Primary: a.Primary, Feature: a.Feature, CommonDir: a.CommonDir, Branch: a.Branch, HEAD: a.EntryHEAD, Clean: true, RootIdentity: identity}}, Workspace: &inputWorkspace{}}

			missing := CheckAgentInputs(context.Background(), deps, CheckInputsRequest{Assignment: assignmentPath, SHA256: assignmentDigest, Stage: "entry", RepoDir: a.Primary})
			if missing.Result != ResultInvalidInput || !strings.Contains(missing.Reason, "payload-invalid") {
				t.Fatalf("missing payload result=%s reason=%s", missing.Result, missing.Reason)
			}

			mutatedBytes := append([]byte(nil), payloadBytes...)
			mutatedBytes[len(mutatedBytes)-2] ^= 1
			if err := os.WriteFile(payloadPath, mutatedBytes, 0o600); err != nil {
				t.Fatal(err)
			}
			mutated := CheckAgentInputs(context.Background(), deps, CheckInputsRequest{Assignment: assignmentPath, SHA256: assignmentDigest, Stage: "entry", Payload: payloadPath, PayloadSHA256: payloadDigest, RepoDir: a.Primary})
			if mutated.Result != ResultInvalidInput || !strings.Contains(mutated.Reason, "sha256 mismatch") {
				t.Fatalf("mutated payload result=%s reason=%s", mutated.Result, mutated.Reason)
			}

			if err := os.WriteFile(payloadPath, payloadBytes, 0o600); err != nil {
				t.Fatal(err)
			}
			valid := CheckAgentInputs(context.Background(), deps, CheckInputsRequest{Assignment: assignmentPath, SHA256: assignmentDigest, Stage: "entry", Payload: payloadPath, PayloadSHA256: payloadDigest, RepoDir: a.Primary})
			if valid.Result != ResultApplied {
				t.Fatalf("valid payload result=%s reason=%s", valid.Result, valid.Reason)
			}
		})
	}
}

func (f *inputScope) ValidateChildInputs(r gatedrive.StartRequest) error {
	f.req = r
	f.calls++
	return nil
}
func (*inputScope) ValidateRecoveredInputs(gatedrive.RecoveredInputs) error { return nil }

func TestCheckAgentInputsDispatchValidatesFinalPayloadAgainstScope(t *testing.T) {
	dir := testsupport.TempDir(t)
	canonicalDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	assignmentPath := filepath.Join(dir, "assignment.json")
	payloadPath := filepath.Join(dir, "payload.json")
	commit := strings.Repeat("2", 40)
	docketPath := writeDocketVersionStub(t, canonicalDir, commit)
	identity := codexcontract.RootIdentity{Platform: "test", Device: 1, Inode: 2, GitDir: "/repo/.git/worktrees/wt"}
	a := codexcontract.Assignment{SchemaVersion: 1, ChangeID: 425, Role: "docket-build-standard", Phase: "build", TaskID: "task-1", Mode: "fresh", Primary: "/repo", Feature: "/repo/wt", CommonDir: "/repo/.git", Branch: "codex/change", EntryHEAD: strings.Repeat("0", 40), MetadataRevision: strings.Repeat("1", 40), ChangePath: "docs/changes/active/0425.md", DocketExecutable: docketPath, DocketCommit: commit, ReadRoots: []string{canonicalDir}, RootIdentity: &identity}
	ab, _ := json.Marshal(a)
	if err := os.WriteFile(assignmentPath, ab, 0o600); err != nil {
		t.Fatal(err)
	}
	as := sha256.Sum256(ab)
	ad := hex.EncodeToString(as[:])
	p := codexcontract.WorkerPayload{SchemaVersion: 1, Kind: "worker", AssignmentPath: assignmentPath, AssignmentSHA256: ad, EntryArgv: []string{docketPath, "agent", "check-inputs", "--assignment", assignmentPath, "--sha256", ad, "--stage", "entry", "--json"}, TaskText: "task", ScopeID: "scope", ChildCapability: "child", GateContext: "ctx", RunEpochID: "epoch"}
	pb, _ := json.Marshal(p)
	if err := os.WriteFile(payloadPath, pb, 0o600); err != nil {
		t.Fatal(err)
	}
	ps := sha256.Sum256(pb)
	scope := &inputScope{}
	workspace := &inputWorkspace{}
	obs := inputObserver{out: AgentRootObservation{Primary: a.Primary, Feature: a.Feature, CommonDir: a.CommonDir, Branch: a.Branch, HEAD: a.EntryHEAD, Clean: true, CallerRoot: a.Primary, RootIdentity: identity}}
	prepared := CheckAgentInputs(context.Background(), AgentInputDeps{Observer: obs, Workspace: workspace}, CheckInputsRequest{Assignment: assignmentPath, SHA256: ad, Stage: "prepare", RepoDir: a.Primary})
	encoded, _ := json.Marshal(prepared)
	var wire struct {
		EntryArgv []string `json:"entry_argv"`
	}
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatal(err)
	}
	wantEntry := []string{docketPath, "agent", "check-inputs", "--assignment", assignmentPath, "--sha256", ad, "--stage", "entry", "--json"}
	if prepared.Result != ResultApplied || !reflect.DeepEqual(wire.EntryArgv, wantEntry) {
		t.Fatalf("prepare must supply the pinned checker command: result=%s entry=%v", prepared.Result, wire.EntryArgv)
	}
	p.EntryArgv = wire.EntryArgv
	if err := codexcontract.ValidateWorkerPayload(p, a); err != nil {
		t.Fatalf("returned command rejected: %v", err)
	}
	pb, err = json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(payloadPath, pb, 0o600); err != nil {
		t.Fatal(err)
	}
	ps = sha256.Sum256(pb)
	writeDocketVersionStub(t, canonicalDir, strings.Repeat("3", 40))
	mismatch := CheckAgentInputs(context.Background(), AgentInputDeps{Observer: obs, Scope: scope, Workspace: workspace}, CheckInputsRequest{Assignment: assignmentPath, SHA256: ad, Stage: "dispatch", Payload: payloadPath, PayloadSHA256: hex.EncodeToString(ps[:]), RepoDir: a.Primary})
	if mismatch.Result != ResultInvalidInput || !strings.Contains(mismatch.Reason, "docket executable commit mismatch") {
		t.Fatalf("mismatched candidate binary result=%s reason=%s", mismatch.Result, mismatch.Reason)
	}
	writeDocketVersionStub(t, canonicalDir, commit)
	workspace.calls = 0
	r := CheckAgentInputs(context.Background(), AgentInputDeps{Observer: obs, Scope: scope, Workspace: workspace}, CheckInputsRequest{Assignment: assignmentPath, SHA256: ad, Stage: "dispatch", Payload: payloadPath, PayloadSHA256: hex.EncodeToString(ps[:]), RepoDir: a.Primary})
	if r.Result != ResultApplied {
		t.Fatalf("result=%s reason=%s", r.Result, r.Reason)
	}
	if scope.calls != 1 || scope.req.TaskID != "task-1" || scope.req.ScopeID != "scope" || scope.req.ChildCapability != "child" {
		t.Fatalf("scope request=%+v calls=%d", scope.req, scope.calls)
	}
	if workspace.calls != 1 {
		t.Fatalf("workspace validation calls=%d", workspace.calls)
	}
}
