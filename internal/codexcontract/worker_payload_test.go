package codexcontract

import (
	"strings"
	"testing"
)

func TestValidateWorkerPayloadBindsFinalScopeAndAssignment(t *testing.T) {
	a := Assignment{Role: "docket-build-standard", TaskID: "task-1", DocketExecutable: "/candidate/docket"}
	p := WorkerPayload{SchemaVersion: 1, Kind: "worker", AssignmentPath: "/private/assignment.json", AssignmentSHA256: strings.Repeat("a", 64), EntryArgv: []string{"/candidate/docket", "agent", "check-inputs", "--assignment", "/private/assignment.json", "--sha256", strings.Repeat("a", 64), "--stage", "entry", "--json"}, TaskText: "implement task 1", ScopeID: "scope-1", ChildCapability: "child-secret"}
	if err := ValidateWorkerPayload(p, a); err != nil {
		t.Fatalf("valid payload: %v", err)
	}
	for name, mutate := range map[string]func(*WorkerPayload){
		"draft capability omitted":      func(p *WorkerPayload) { p.ChildCapability = "" },
		"assignment digest substituted": func(p *WorkerPayload) { p.AssignmentSHA256 = strings.Repeat("b", 64) },
		"entry argv substituted":        func(p *WorkerPayload) { p.EntryArgv[0] = "docket" },
		"repository flag embedded":      func(p *WorkerPayload) { p.EntryArgv = append(p.EntryArgv, "--repo-dir", "/repo") },
		"json flag omitted":             func(p *WorkerPayload) { p.EntryArgv = p.EntryArgv[:len(p.EntryArgv)-1] },
		"half predecessor":              func(p *WorkerPayload) { p.PredecessorDriveID = "drive-1" },
	} {
		t.Run(name, func(t *testing.T) {
			q := p
			q.EntryArgv = append([]string(nil), p.EntryArgv...)
			mutate(&q)
			if ValidateWorkerPayload(q, a) == nil {
				t.Fatal("accepted invalid final payload")
			}
		})
	}
}

func TestWorkerPayloadStrictDecodeRejectsParentAuthority(t *testing.T) {
	b := []byte(`{"schema_version":1,"kind":"worker","assignment_path":"/private/a","assignment_sha256":"` + strings.Repeat("a", 64) + `","entry_argv":[],"task_text":"x","scope_id":"s","child_capability":"c","parent_capability":"must-not-leak"}`)
	if _, err := DecodeWorkerPayload(b); err == nil {
		t.Fatal("accepted parent capability in child payload")
	}
}

func TestRepairPayloadRequiresOwnedAttemptLocator(t *testing.T) {
	a := Assignment{Role: "docket-integration-repair", Mode: "repair", DocketExecutable: "/candidate/docket"}
	p := WorkerPayload{SchemaVersion: 1, Kind: "repair", AssignmentPath: "/private/assignment.json", AssignmentSHA256: strings.Repeat("a", 64), EntryArgv: []string{"/candidate/docket", "agent", "check-inputs", "--assignment", "/private/assignment.json", "--sha256", strings.Repeat("a", 64), "--stage", "entry", "--json"}, TaskText: "repair integration", Attempt: "attempt-1"}
	if err := ValidateWorkerPayload(p, a); err != nil {
		t.Fatalf("valid repair payload: %v", err)
	}
	p.Attempt = ""
	if err := ValidateWorkerPayload(p, a); err == nil {
		t.Fatal("accepted repair payload without an owned attempt locator")
	}
}
