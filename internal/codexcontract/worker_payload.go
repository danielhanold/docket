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
	SchemaVersion       int               `json:"schema_version"`
	Kind                string            `json:"kind"`
	AssignmentPath      string            `json:"assignment_path"`
	AssignmentSHA256    string            `json:"assignment_sha256"`
	EntryArgv           []string          `json:"entry_argv"`
	TaskText            string            `json:"task_text"`
	ScopeID             string            `json:"scope_id,omitempty"`
	ChildCapability     string            `json:"child_capability,omitempty"`
	GateContext         string            `json:"gate_context,omitempty"`
	RunEpochID          string            `json:"run_epoch_id,omitempty"`
	PredecessorDriveID  string            `json:"predecessor_drive_id,omitempty"`
	PredecessorOwnerGen string            `json:"predecessor_owner_gen,omitempty"`
	Recovered           *RecoveredPayload `json:"recovered,omitempty"`
}

type RecoveredPayload struct {
	ScopeID         string `json:"scope_id"`
	DriveID         string `json:"drive_id"`
	OwnerGeneration string `json:"owner_generation"`
}

func ValidateWorkerPayload(p WorkerPayload, a Assignment) error {
	if p.SchemaVersion != 1 {
		return fmt.Errorf("unsupported worker payload schema_version %d", p.SchemaVersion)
	}
	if p.Kind == "planner" || p.Kind == "review" {
		if p.ScopeID != "" || p.ChildCapability != "" || p.PredecessorDriveID != "" || p.PredecessorOwnerGen != "" {
			return fmt.Errorf("%s payload carries worker authority", p.Kind)
		}
		return nil
	}
	if p.Kind != "worker" {
		return fmt.Errorf("unknown payload kind %q", p.Kind)
	}
	if !strings.Contains(a.Role, "build") || a.TaskID == "" {
		return fmt.Errorf("worker payload does not match a worker assignment")
	}
	if !filepath.IsAbs(p.AssignmentPath) || p.AssignmentSHA256 == "" || p.TaskText == "" || p.ScopeID == "" || p.ChildCapability == "" {
		return fmt.Errorf("worker payload is incomplete")
	}
	if (p.PredecessorDriveID == "") != (p.PredecessorOwnerGen == "") {
		return fmt.Errorf("predecessor receipt is incomplete")
	}
	want := []string{a.DocketExecutable, "agent", "check-inputs", "--assignment", p.AssignmentPath, "--sha256", p.AssignmentSHA256, "--stage", "entry", "--json"}
	if len(p.EntryArgv) != len(want) {
		return fmt.Errorf("entry argv does not match the pinned checker")
	}
	for i := range want {
		if p.EntryArgv[i] != want[i] {
			return fmt.Errorf("entry argv does not match the pinned checker")
		}
	}
	return nil
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
