package codexcontract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

type WorkerPayload struct {
	SchemaVersion       int               `json:"schema_version" docket:"required"`
	Kind                string            `json:"kind" docket:"required,enum=payload_kinds"`
	AssignmentPath      string            `json:"assignment_path" docket:"required"`
	AssignmentSHA256    string            `json:"assignment_sha256" docket:"required"`
	EntryArgv           []string          `json:"entry_argv" docket:"required"`
	TaskText            string            `json:"task_text" docket:"required"`
	ScopeID             string            `json:"scope_id,omitempty"`
	ChildCapability     string            `json:"child_capability,omitempty"`
	GateContext         string            `json:"gate_context,omitempty"`
	RunEpochID          string            `json:"run_epoch_id,omitempty"`
	PredecessorDriveID  string            `json:"predecessor_drive_id,omitempty"`
	PredecessorOwnerGen string            `json:"predecessor_owner_gen,omitempty"`
	Recovered           *RecoveredPayload `json:"recovered,omitempty"`
	Attempt             string            `json:"attempt,omitempty"`
	ResolverReservation string            `json:"resolver_reservation,omitempty"`
}

var AllPayloadKinds = []string{"planner", "review", "worker", "resolver", "repair"}

type RecoveredPayload struct {
	ScopeID         string `json:"scope_id"`
	DriveID         string `json:"drive_id"`
	OwnerGeneration string `json:"owner_generation"`
}

// EntryCheckerArgv is the assignment-only command pinned into private payloads.
// Outer payload and repository flags are deliberately not part of this command.
func EntryCheckerArgv(a Assignment, path, digest string) []string {
	return []string{a.DocketExecutable, "agent", "check-inputs", "--assignment", path, "--sha256", digest, "--stage", "entry", "--json"}
}

func ValidateWorkerPayload(p WorkerPayload, a Assignment) error {
	if p.SchemaVersion != 1 {
		return fmt.Errorf("unsupported worker payload schema_version %d", p.SchemaVersion)
	}
	if !filepath.IsAbs(p.AssignmentPath) || p.AssignmentSHA256 == "" || p.TaskText == "" {
		return fmt.Errorf("payload fixed inputs are incomplete")
	}
	want := EntryCheckerArgv(a, p.AssignmentPath, p.AssignmentSHA256)
	if len(p.EntryArgv) != len(want) {
		return fmt.Errorf("entry argv does not match the pinned checker")
	}
	for i := range want {
		if p.EntryArgv[i] != want[i] {
			return fmt.Errorf("entry argv does not match the pinned checker")
		}
	}
	if p.Kind == "planner" || p.Kind == "review" {
		wantRole := p.Kind == "planner" && a.Role == "docket-plan-writer" || p.Kind == "review" && strings.HasPrefix(a.Role, "docket-review-")
		if !wantRole {
			return fmt.Errorf("%s payload does not match assignment role", p.Kind)
		}
		if carriesWorkerAuthority(p) || p.GateContext != "" || p.RunEpochID != "" || p.Attempt != "" || p.ResolverReservation != "" {
			return fmt.Errorf("%s payload carries worker authority", p.Kind)
		}
		return nil
	}
	if p.Kind == "resolver" {
		if a.Role != "docket-rebase-resolver" || a.Mode != "resolver" || p.Attempt == "" || p.ResolverReservation == "" {
			return fmt.Errorf("resolver payload does not match reserved assignment")
		}
		if carriesWorkerAuthority(p) || p.GateContext != "" || p.RunEpochID != "" {
			return fmt.Errorf("resolver payload carries worker authority")
		}
		return nil
	}
	if p.Kind == "repair" {
		if a.Role != "docket-integration-repair" || a.Mode != "repair" || p.Attempt == "" || p.ResolverReservation != "" || carriesWorkerAuthority(p) || p.GateContext != "" || p.RunEpochID != "" {
			return fmt.Errorf("repair payload does not match assignment")
		}
		return nil
	}
	if p.Kind != "worker" {
		return fmt.Errorf("unknown payload kind %q", p.Kind)
	}
	if !strings.Contains(a.Role, "build") || a.TaskID == "" {
		return fmt.Errorf("worker payload does not match a worker assignment")
	}
	if p.Attempt != "" || p.ResolverReservation != "" {
		return fmt.Errorf("worker payload carries role authority")
	}
	if (p.PredecessorDriveID == "") != (p.PredecessorOwnerGen == "") {
		return fmt.Errorf("predecessor receipt is incomplete")
	}
	if p.Recovered != nil {
		if p.ScopeID != "" || p.ChildCapability != "" || p.PredecessorDriveID != "" || p.PredecessorOwnerGen != "" || a.Mode != "continuation" {
			return fmt.Errorf("recovered payload carries new worker authority")
		}
		if p.Recovered.ScopeID == "" || p.Recovered.DriveID == "" || p.Recovered.OwnerGeneration == "" {
			return fmt.Errorf("recovered payload is incomplete")
		}
		return nil
	}
	if p.ScopeID == "" || p.ChildCapability == "" {
		return fmt.Errorf("worker payload is incomplete")
	}
	return nil
}

func carriesWorkerAuthority(p WorkerPayload) bool {
	return p.ScopeID != "" || p.ChildCapability != "" || p.PredecessorDriveID != "" || p.PredecessorOwnerGen != "" || p.Recovered != nil
}
func DecodeWorkerPayload(b []byte) (WorkerPayload, error) {
	if len(b) > 1<<20 {
		return WorkerPayload{}, fmt.Errorf("worker payload exceeds 1 MiB")
	}
	if err := rejectDuplicateKeys(b); err != nil {
		return WorkerPayload{}, err
	}
	var p WorkerPayload
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return WorkerPayload{}, fmt.Errorf("invalid worker payload: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return WorkerPayload{}, fmt.Errorf("invalid worker payload trailer")
	}
	return p, nil
}
