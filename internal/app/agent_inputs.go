package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/danielhanold/docket/internal/codexcontract"
	"github.com/danielhanold/docket/internal/gatedrive"
)

const OperationAgentCheckInputs = "agent.check-inputs"

type CheckInputsRequest struct {
	Assignment    string `json:"assignment" docket:"required"`
	SHA256        string `json:"sha256" docket:"required"`
	Stage         string `json:"stage" docket:"required"`
	Payload       string `json:"payload,omitempty"`
	PayloadSHA256 string `json:"payload_sha256,omitempty"`
	RepoDir       string `json:"repo_dir,omitempty"`
}
type AgentRootObservation struct {
	Primary      string                     `json:"primary"`
	Feature      string                     `json:"feature"`
	CommonDir    string                     `json:"common_dir"`
	Branch       string                     `json:"branch"`
	HEAD         string                     `json:"head"`
	Clean        bool                       `json:"clean"`
	CallerRoot   string                     `json:"caller_root"`
	RootIdentity codexcontract.RootIdentity `json:"root_identity"`
	Fingerprint  *gatedrive.Fingerprint     `json:"fingerprint,omitempty"`
	ChangedPaths []string                   `json:"-"`
}
type CheckInputsResult struct {
	Envelope
	Reason           string               `json:"reason,omitempty"`
	AssignmentSHA256 string               `json:"assignment_sha256,omitempty"`
	PayloadSHA256    string               `json:"payload_sha256,omitempty"`
	Observation      AgentRootObservation `json:"observation,omitempty"`
}

func (r CheckInputsResult) HumanText() string {
	if r.Reason != "" {
		return r.Reason
	}
	return "agent inputs valid"
}

type AgentInputObserver interface {
	ObserveAgentInputs(context.Context, codexcontract.Assignment, string) (AgentRootObservation, error)
}
type AgentChildInputValidator interface {
	ValidateChildInputs(gatedrive.StartRequest) error
	ValidateRecoveredInputs(gatedrive.RecoveredInputs) error
}
type AgentWorkspaceValidator interface {
	ValidateAgentWorkspace(context.Context, codexcontract.Assignment, string) error
}
type AgentInputDeps struct {
	Observer  AgentInputObserver
	Scope     AgentChildInputValidator
	Workspace AgentWorkspaceValidator
}

func CheckAgentInputs(ctx context.Context, deps AgentInputDeps, req CheckInputsRequest) CheckInputsResult {
	fail := func(result Result, reason string) CheckInputsResult {
		return CheckInputsResult{Envelope: NewEnvelope(OperationAgentCheckInputs, result), Reason: reason, AssignmentSHA256: req.SHA256, PayloadSHA256: req.PayloadSHA256}
	}
	if req.Stage != "prepare" && req.Stage != "dispatch" && req.Stage != "entry" && req.Stage != "active" {
		return fail(ResultInvalidInput, "invalid-stage")
	}
	a, err := codexcontract.ReadAssignment(req.Assignment, req.SHA256)
	if err != nil {
		return fail(ResultInvalidInput, "assignment-invalid: "+err.Error())
	}
	if err := codexcontract.ValidateDocketExecutable(a); err != nil {
		return fail(ResultInvalidInput, "docket-executable-invalid: "+err.Error())
	}
	if err := codexcontract.ValidateResources(a); err != nil {
		return fail(ResultInvalidInput, "resources-invalid: "+err.Error())
	}
	if deps.Observer == nil {
		return fail(ResultInvalidState, "observer-unavailable")
	}
	if deps.Workspace == nil {
		return fail(ResultInvalidState, "workspace-validator-unavailable")
	}
	if err := deps.Workspace.ValidateAgentWorkspace(ctx, a, req.RepoDir); err != nil {
		return fail(ResultInvalidState, "workspace-binding-invalid: "+err.Error())
	}
	obs, err := deps.Observer.ObserveAgentInputs(ctx, a, req.RepoDir)
	if err != nil {
		return fail(ResultInvalidState, "root-identity-mismatch: "+err.Error())
	}
	if obs.Primary != a.Primary || obs.Feature != a.Feature || obs.CommonDir != a.CommonDir || obs.Branch != a.Branch {
		return fail(ResultInvalidState, "root-identity-mismatch")
	}
	if req.Stage != "prepare" {
		if a.RootIdentity == nil {
			return fail(ResultInvalidInput, "root-identity-missing")
		}
		if !a.RootIdentity.Equal(obs.RootIdentity) {
			return fail(ResultInvalidState, "root-identity-mismatch")
		}
	}
	if req.Stage != "active" || a.Mode == "review" {
		if obs.HEAD != a.EntryHEAD && (a.Mode != "review" || obs.HEAD != a.ReviewHEAD) {
			return fail(ResultInvalidState, "head-mismatch")
		}
	}
	if (req.Stage == "entry" || req.Stage == "prepare") && (a.Mode == "fresh" || a.Mode == "review") && !obs.Clean {
		return fail(ResultInvalidState, "worktree-dirty")
	}
	if req.Stage == "entry" && (a.Mode == "continuation" || a.Mode == "escalation") {
		if a.InheritedFingerprint == nil || obs.Fingerprint == nil || !a.InheritedFingerprint.Equal(*obs.Fingerprint) {
			return fail(ResultInvalidState, "inherited-fingerprint-mismatch")
		}
		if !samePaths(obs.ChangedPaths, a.InheritedPaths) {
			return fail(ResultInvalidState, "inherited-paths-mismatch")
		}
	}
	if req.Stage == "active" && a.Mode != "review" {
		allowed := append(append([]string{}, a.WritePaths...), a.InheritedPaths...)
		for _, path := range obs.ChangedPaths {
			if !withinOwnedPath(allowed, path) {
				return fail(ResultInvalidState, "unowned-active-path")
			}
		}
	}
	if req.Stage == "dispatch" || (req.Stage == "entry" && req.Payload != "") {
		b, err := readPinnedFile(req.Payload, req.PayloadSHA256)
		if err != nil {
			return fail(ResultInvalidInput, "payload-invalid: "+err.Error())
		}
		p, err := codexcontract.DecodeWorkerPayload(b)
		if err != nil {
			return fail(ResultInvalidInput, "payload-invalid: "+err.Error())
		}
		if p.AssignmentPath != req.Assignment || p.AssignmentSHA256 != req.SHA256 {
			return fail(ResultInvalidInput, "payload-invalid: assignment digest or locator mismatch")
		}
		if err := codexcontract.ValidateWorkerPayload(p, a); err != nil {
			return fail(ResultInvalidInput, "payload-invalid: "+err.Error())
		}
		if p.Kind == "worker" {
			if deps.Scope == nil {
				return fail(ResultInvalidState, "scope-validator-unavailable")
			}
			if p.Recovered != nil {
				err = deps.Scope.ValidateRecoveredInputs(gatedrive.RecoveredInputs{ScopeID: p.Recovered.ScopeID, DriveID: p.Recovered.DriveID, OwnerGeneration: p.Recovered.OwnerGeneration, ChangeID: fmt.Sprint(a.ChangeID), TaskID: a.TaskID, Phase: a.Phase, GateContext: p.GateContext, RunEpochID: p.RunEpochID})
			} else {
				err = deps.Scope.ValidateChildInputs(gatedrive.StartRequest{RepoDir: a.Feature, Worktree: a.Feature, ChangeID: fmt.Sprint(a.ChangeID), TaskID: a.TaskID, Phase: a.Phase, Branch: a.Branch, Ref: "refs/heads/" + a.Branch, Cwd: a.Feature, RunRoot: a.RunRoot, ScopeID: p.ScopeID, ChildCapability: p.ChildCapability, GateContext: p.GateContext, RunEpochID: p.RunEpochID, PredecessorDriveID: p.PredecessorDriveID, PredecessorOwnerGen: p.PredecessorOwnerGen})
			}
			if err != nil {
				return fail(ResultInvalidState, "scope-inputs-invalid: "+err.Error())
			}
		}
	}
	return CheckInputsResult{Envelope: NewEnvelope(OperationAgentCheckInputs, ResultApplied), AssignmentSHA256: req.SHA256, PayloadSHA256: req.PayloadSHA256, Observation: obs}
}

func samePaths(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	g, w := append([]string{}, got...), append([]string{}, want...)
	slices.Sort(g)
	slices.Sort(w)
	return slices.Equal(g, w)
}

func withinOwnedPath(roots []string, path string) bool {
	for _, root := range roots {
		if path == root || strings.HasPrefix(path, strings.TrimSuffix(root, "/")+"/") {
			return true
		}
	}
	return false
}

func readPinnedFile(path, digest string) ([]byte, error) {
	if path == "" || digest == "" {
		return nil, fmt.Errorf("path and sha256 are required")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(b) > 1<<20 {
		return nil, fmt.Errorf("file exceeds 1 MiB")
	}
	want, err := hex.DecodeString(digest)
	if err != nil || len(want) != sha256.Size {
		return nil, fmt.Errorf("invalid sha256")
	}
	got := sha256.Sum256(b)
	if string(want) != string(got[:]) {
		return nil, fmt.Errorf("sha256 mismatch")
	}
	return b, nil
}

func readPinnedFileUnchecked(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(b) > 1<<20 {
		return nil, fmt.Errorf("file exceeds 1 MiB")
	}
	return b, nil
}
