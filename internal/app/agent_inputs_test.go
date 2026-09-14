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

func (f *inputScope) ValidateChildInputs(r gatedrive.StartRequest) error {
	f.req = r
	f.calls++
	return nil
}
func (*inputScope) ValidateRecoveredInputs(gatedrive.RecoveredInputs) error { return nil }

func TestCheckAgentInputsDispatchValidatesFinalPayloadAgainstScope(t *testing.T) {
	dir := testsupport.TempDir(t)
	assignmentPath := filepath.Join(dir, "assignment.json")
	payloadPath := filepath.Join(dir, "payload.json")
	a := codexcontract.Assignment{SchemaVersion: 1, ChangeID: 425, Role: "docket-build-standard", Phase: "build", TaskID: "task-1", Mode: "fresh", Primary: "/repo", Feature: "/repo/wt", CommonDir: "/repo/.git", Branch: "codex/change", EntryHEAD: strings.Repeat("0", 40), MetadataRevision: strings.Repeat("1", 40), ChangePath: "docs/changes/active/0425.md", DocketExecutable: "/candidate/docket", DocketCommit: strings.Repeat("2", 40), ReadRoots: []string{dir}}
	ab, _ := json.Marshal(a)
	if err := os.WriteFile(assignmentPath, ab, 0o600); err != nil {
		t.Fatal(err)
	}
	as := sha256.Sum256(ab)
	ad := hex.EncodeToString(as[:])
	p := codexcontract.WorkerPayload{SchemaVersion: 1, Kind: "worker", AssignmentPath: assignmentPath, AssignmentSHA256: ad, EntryArgv: []string{"/candidate/docket", "agent", "check-inputs", "--assignment", assignmentPath, "--sha256", ad, "--stage", "entry", "--json"}, TaskText: "task", ScopeID: "scope", ChildCapability: "child", GateContext: "ctx", RunEpochID: "epoch"}
	pb, _ := json.Marshal(p)
	if err := os.WriteFile(payloadPath, pb, 0o600); err != nil {
		t.Fatal(err)
	}
	ps := sha256.Sum256(pb)
	scope := &inputScope{}
	obs := inputObserver{out: AgentRootObservation{Primary: a.Primary, Feature: a.Feature, CommonDir: a.CommonDir, Branch: a.Branch, HEAD: a.EntryHEAD, Clean: true, CallerRoot: a.Primary}}
	r := CheckAgentInputs(context.Background(), AgentInputDeps{Observer: obs, Scope: scope}, CheckInputsRequest{Assignment: assignmentPath, SHA256: ad, Stage: "dispatch", Payload: payloadPath, PayloadSHA256: hex.EncodeToString(ps[:]), RepoDir: a.Primary})
	if r.Result != ResultApplied {
		t.Fatalf("result=%s reason=%s", r.Result, r.Reason)
	}
	if scope.calls != 1 || scope.req.TaskID != "task-1" || scope.req.ScopeID != "scope" || scope.req.ChildCapability != "child" {
		t.Fatalf("scope request=%+v calls=%d", scope.req, scope.calls)
	}
}
