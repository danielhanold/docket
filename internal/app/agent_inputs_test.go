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

	"github.com/danielhanold/docket/internal/codexcontract"
	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/testsupport"
)

type inputObserver struct {
	out AgentRootObservation
	err error
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
			docketPath := filepath.Join(dir, "docket")
			if err := os.WriteFile(docketPath, []byte("fixture"), 0o755); err != nil {
				t.Fatal(err)
			}
			identity := codexcontract.RootIdentity{Platform: "test", Device: 1, Inode: 2, GitDir: "/repo/.git/worktrees/wt"}
			a := codexcontract.Assignment{SchemaVersion: 1, ChangeID: 425, Role: tc.role, Phase: tc.phase, Mode: tc.mode, Primary: "/repo", Feature: "/repo/wt", CommonDir: "/repo/.git", Branch: "codex/change", EntryHEAD: strings.Repeat("0", 40), MetadataRevision: strings.Repeat("1", 40), ChangePath: "docs/changes/active/0425.md", DocketExecutable: docketPath, DocketCommit: strings.Repeat("2", 40), ReadRoots: []string{dir}, WritePaths: []string{"conflict.go"}, RootIdentity: &identity}
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
	docketPath := filepath.Join(canonicalDir, "docket")
	if err := os.WriteFile(docketPath, []byte("fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	identity := codexcontract.RootIdentity{Platform: "test", Device: 1, Inode: 2, GitDir: "/repo/.git/worktrees/wt"}
	a := codexcontract.Assignment{SchemaVersion: 1, ChangeID: 425, Role: "docket-build-standard", Phase: "build", TaskID: "task-1", Mode: "fresh", Primary: "/repo", Feature: "/repo/wt", CommonDir: "/repo/.git", Branch: "codex/change", EntryHEAD: strings.Repeat("0", 40), MetadataRevision: strings.Repeat("1", 40), ChangePath: "docs/changes/active/0425.md", DocketExecutable: docketPath, DocketCommit: strings.Repeat("2", 40), ReadRoots: []string{canonicalDir}, RootIdentity: &identity}
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
