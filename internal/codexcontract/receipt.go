package codexcontract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/danielhanold/docket/internal/gatedrive"
)

type Receipt struct {
	Operation        string              `json:"operation"`
	Result           string              `json:"result"`
	Classification   string              `json:"classification"`
	Drive            *gatedrive.DriveDoc `json:"drive,omitempty"`
	ScopeID          string              `json:"scope_id,omitempty"`
	ChildCapability  string              `json:"child_capability,omitempty"`
	ParentCapability string              `json:"parent_capability,omitempty"`
	Stdout           []byte              `json:"stdout"`
	Stderr           []byte              `json:"stderr"`
	ExitCode         int                 `json:"exit_code"`
	Reason           string              `json:"reason,omitempty"`
	Message          string              `json:"message,omitempty"`
}

func ParseReceipt(operation string, stdout, stderr []byte, exitCode int, assignment Assignment) (Receipt, error) {
	r := Receipt{Operation: operation, Stdout: append([]byte(nil), stdout...), Stderr: append([]byte(nil), stderr...), ExitCode: exitCode}
	var wire struct {
		ProtocolVersion  int                 `json:"protocol_version"`
		Operation        string              `json:"operation"`
		Result           string              `json:"result"`
		Drive            *gatedrive.DriveDoc `json:"drive,omitempty"`
		ScopeID          string              `json:"scope_id,omitempty"`
		ChildCapability  string              `json:"child_capability,omitempty"`
		ParentCapability string              `json:"parent_capability,omitempty"`
		Reason           string              `json:"reason,omitempty"`
		Message          string              `json:"message,omitempty"`
	}
	if err := rejectDuplicateKeys(stdout); err != nil {
		return r, fmt.Errorf("invalid receipt: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(stdout))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&wire); err != nil {
		return r, fmt.Errorf("invalid receipt: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return r, fmt.Errorf("invalid receipt trailer")
	}
	if wire.ProtocolVersion != 1 || wire.Operation != operation || wire.Result == "" {
		return r, fmt.Errorf("receipt envelope does not match operation")
	}
	if !knownReceiptResult(wire.Result) {
		return r, fmt.Errorf("receipt result is unknown")
	}
	r.Result, r.Drive, r.ScopeID, r.ChildCapability, r.ParentCapability, r.Reason, r.Message = wire.Result, wire.Drive, wire.ScopeID, wire.ChildCapability, wire.ParentCapability, wire.Reason, wire.Message
	switch operation {
	case "gate.drive.prepare-scope":
		if wire.Reason != "" && wire.ScopeID == "" && wire.ChildCapability == "" && wire.ParentCapability == "" {
			if wire.Result == "applied" || exitCode == 0 {
				return r, fmt.Errorf("scope halt receipt result/exit is inconsistent")
			}
			r.Classification = "halt"
			return r, nil
		}
		if wire.Result != "applied" || exitCode != 0 || wire.Drive != nil || wire.ScopeID == "" || wire.ChildCapability == "" || wire.ParentCapability == "" {
			return r, fmt.Errorf("scope receipt is incomplete")
		}
		r.Classification = "scope"
	case "gate.drive.start", "gate.drive.advance", "gate.drive.acknowledge", "gate.drive.handoff", "gate.drive.claim", "gate.drive.takeover":
		if wire.Drive == nil && wire.Reason != "" {
			if wire.Result == "applied" || exitCode == 0 {
				return r, fmt.Errorf("drive halt receipt result/exit is inconsistent")
			}
			r.Classification = "halt"
			return r, nil
		}
		if wire.Result == "applied" && exitCode != 0 && diagnosticHalt(wire.Drive) {
			r.Classification = "halt"
			return r, nil
		}
		if wire.Result != "applied" || wire.Drive == nil || wire.Drive.ProtocolVersion != 1 || wire.Drive.DriveID == "" || wire.Drive.Generation == "" || wire.Drive.Deadline.IsZero() || !knownDriveOutcome(wire.Drive.Outcome) {
			return r, fmt.Errorf("drive receipt is incomplete")
		}
		if (wire.Drive.Outcome == gatedrive.WAITING || wire.Drive.Outcome == gatedrive.PASSED) != (exitCode == 0) {
			return r, fmt.Errorf("drive receipt outcome/exit is inconsistent")
		}
		r.Classification = string(wire.Drive.Outcome)
		transfer := operation == "gate.drive.handoff" || operation == "gate.drive.claim" || operation == "gate.drive.takeover"
		if transfer {
			if wire.Drive.RunRoot != "" {
				return r, fmt.Errorf("transfer receipt exposes run_root")
			}
			if wire.Drive.RawRunDir != "" && (wire.Drive.Outcome != gatedrive.PASSED || !filepath.IsAbs(assignment.RunRoot) || !filepath.IsAbs(wire.Drive.RawRunDir) || !pathWithin(assignment.RunRoot, wire.Drive.RawRunDir)) {
				return r, fmt.Errorf("transfer receipt raw_run_dir does not match the assigned run root")
			}
		} else if wire.Drive.Outcome == gatedrive.WAITING {
			if wire.Drive.RunRoot != "" || wire.Drive.RawRunDir != "" {
				return r, fmt.Errorf("WAITING receipt exposes run_root")
			}
		} else {
			if !filepath.IsAbs(wire.Drive.RunRoot) {
				return r, fmt.Errorf("terminal receipt run_root must be absolute")
			}
			if !filepath.IsAbs(assignment.RunRoot) || wire.Drive.RunRoot != assignment.RunRoot {
				return r, fmt.Errorf("terminal receipt run_root does not match assignment")
			}
			if wire.Drive.Outcome == gatedrive.PASSED && (!filepath.IsAbs(wire.Drive.RawRunDir) || !pathWithin(wire.Drive.RunRoot, wire.Drive.RawRunDir)) {
				return r, fmt.Errorf("PASSED receipt raw_run_dir does not match run_root")
			}
			if wire.Drive.Outcome != gatedrive.PASSED && wire.Drive.RawRunDir != "" {
				return r, fmt.Errorf("non-PASSED receipt exposes raw_run_dir")
			}
		}
	default:
		return r, fmt.Errorf("unsupported receipt operation %q", operation)
	}
	return r, nil
}

func diagnosticHalt(doc *gatedrive.DriveDoc) bool {
	return doc != nil && doc.ProtocolVersion == 1 && doc.Outcome == gatedrive.HALTED && doc.Cause != "" && doc.RunRoot == "" && doc.RawRunDir == ""
}

func knownDriveOutcome(outcome gatedrive.Outcome) bool {
	return outcome == gatedrive.WAITING || outcome == gatedrive.PASSED || outcome == gatedrive.FAILED || outcome == gatedrive.HALTED
}

func knownReceiptResult(result string) bool {
	switch result {
	case "applied", "no-op", "contended", "invalid-input", "invalid-state", "blocked", "unsupported-config", "gate-failed", "external-failed", "interrupted", "internal-error":
		return true
	default:
		return false
	}
}

func pathWithin(root, p string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(p))
	return err == nil && rel != ".." && !filepath.IsAbs(rel) && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
