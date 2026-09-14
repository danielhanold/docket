package app

import (
	"context"
	"fmt"

	"github.com/danielhanold/docket/internal/codexcontract"
	"github.com/danielhanold/docket/internal/workspace"
)

type agentWorkspaceValidator struct {
	planning  PlanningDeps
	workspace WorkspaceDeps
}

type agentFinalizeInputValidator struct{ deps FinalizeDeps }

func NewAgentFinalizeInputValidator(deps FinalizeDeps) AgentRoleInputValidator {
	return agentFinalizeInputValidator{deps: deps}
}

func (v agentFinalizeInputValidator) ValidateRoleInputs(ctx context.Context, a codexcontract.Assignment, p codexcontract.WorkerPayload, repoDir string) error {
	if p.Kind == "repair" {
		return validateRepairEntry(ctx, v.deps, repoDir, a.ChangeID, p.Attempt)
	}
	return validateResolverEntry(ctx, v.deps, repoDir, a.ChangeID, p.Attempt, p.ResolverReservation, a.WritePaths)
}

func NewAgentWorkspaceValidator(planning PlanningDeps, workspaceDeps WorkspaceDeps) AgentWorkspaceValidator {
	return agentWorkspaceValidator{planning: planning, workspace: workspaceDeps}
}

func (v agentWorkspaceValidator) ValidateAgentWorkspace(ctx context.Context, a codexcontract.Assignment, repoDir string) error {
	wc, refusal := loadWorkspaceContext(ctx, v.planning, repoDir, a.ChangeID, OperationAgentCheckInputs)
	if refusal != nil {
		return fmt.Errorf("%s", refusal.Reason)
	}
	if wc.change.Path() != a.ChangePath {
		return fmt.Errorf("change-path-mismatch")
	}
	target, targetRefusal := resolveWorkspaceTarget(OperationAgentCheckInputs, wc)
	if targetRefusal != nil {
		return fmt.Errorf("%s", targetRefusal.Reason)
	}
	if string(target.FeatureRef) != "refs/heads/"+a.Branch {
		return fmt.Errorf("feature-ref-mismatch")
	}
	insp, err := v.workspace.Service.Inspect(ctx, workspace.InspectRequest{Repository: wc.repo, Target: target})
	if err != nil {
		return fmt.Errorf("workspace-inspect: %w", err)
	}
	if insp.Path != a.Feature {
		return fmt.Errorf("feature-path-mismatch")
	}
	if insp.Kind != workspace.StateReady && insp.Kind != workspace.StateDirty {
		return fmt.Errorf("workspace-state-%s", insp.Kind)
	}
	pin, err := v.planning.Reader.PinContext(ctx, repoDir)
	if err != nil {
		return fmt.Errorf("metadata-recheck: %w", err)
	}
	if pin.MetadataRevision != a.MetadataRevision {
		return fmt.Errorf("metadata-revision-mismatch")
	}
	return nil
}
