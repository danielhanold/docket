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
	r.Result, r.Drive, r.ScopeID, r.ChildCapability, r.ParentCapability = wire.Result, wire.Drive, wire.ScopeID, wire.ChildCapability, wire.ParentCapability
	switch operation {
	case "gate.drive.prepare-scope":
		if wire.Drive != nil || wire.ScopeID == "" || wire.ChildCapability == "" || wire.ParentCapability == "" {
			return r, fmt.Errorf("scope receipt is incomplete")
		}
		r.Classification = "scope"
	case "gate.drive.start", "gate.drive.advance", "gate.drive.acknowledge", "gate.drive.handoff", "gate.drive.claim", "gate.drive.takeover":
		if wire.Drive == nil || wire.Drive.ProtocolVersion != 1 || wire.Drive.DriveID == "" || wire.Drive.Generation == "" {
			return r, fmt.Errorf("drive receipt is incomplete")
		}
		r.Classification = string(wire.Drive.Outcome)
		if wire.Drive.Outcome == gatedrive.WAITING {
			if wire.Drive.RunRoot != "" {
				return r, fmt.Errorf("WAITING receipt exposes run_root")
			}
		} else {
			if !filepath.IsAbs(wire.Drive.RunRoot) {
				return r, fmt.Errorf("terminal receipt run_root must be absolute")
			}
			if assignment.RunRoot != "" && !pathWithin(assignment.RunRoot, wire.Drive.RunRoot) {
				return r, fmt.Errorf("terminal receipt run_root does not match assignment")
			}
		}
	default:
		return r, fmt.Errorf("unsupported receipt operation %q", operation)
	}
	return r, nil
}

func pathWithin(root, p string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(p))
	return err == nil && rel != ".." && !filepath.IsAbs(rel) && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
