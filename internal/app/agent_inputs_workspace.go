package app

import (
	"context"
	"fmt"
	"maps"

	"github.com/danielhanold/docket/internal/codexcontract"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/repository"
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

func (v agentFinalizeInputValidator) ValidateRoleInputs(ctx context.Context, a codexcontract.Assignment, p codexcontract.WorkerPayload, repoDir, stage string) error {
	if p.Kind == "repair" {
		return validateRepairEntry(ctx, v.deps, repoDir, a.ChangeID, p.Attempt)
	}
	return validateResolverEntry(ctx, v.deps, repoDir, a, p.Attempt, p.ResolverReservation, stage == "prepare")
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
	// The metadata branch is shared by every change. Compare the assignment's
	// inputs, not its global tip, both at workspace inspection and at recheck.
	var expected map[string]string
	seen := map[string]bool{a.MetadataRevision: true}
	for _, revision := range []string{wc.metadataRevision, pin.MetadataRevision} {
		if seen[revision] {
			continue
		}
		seen[revision] = true
		ancestor, err := v.planning.Client.IsAncestor(ctx, wc.repo, gitcli.ObjectID(a.MetadataRevision), gitcli.ObjectID(revision))
		if err != nil {
			return fmt.Errorf("metadata-history: %w", err)
		}
		if !ancestor {
			return fmt.Errorf("metadata-history-diverged")
		}
		if expected == nil {
			assigned := pin
			assigned.MetadataRevision = a.MetadataRevision
			expected, err = agentMetadataBindings(ctx, v.planning.Reader, assigned, a.ChangeID)
			if err != nil {
				return fmt.Errorf("assigned-metadata-invalid: %w", err)
			}
		}
		observed := pin
		observed.MetadataRevision = revision
		actual, err := agentMetadataBindings(ctx, v.planning.Reader, observed, a.ChangeID)
		if err != nil {
			return fmt.Errorf("metadata-inputs-invalid: %w", err)
		}
		if !maps.Equal(expected, actual) {
			return fmt.Errorf("metadata-inputs-mismatch")
		}
	}
	return nil
}

// agentMetadataBindings pins the change and its transitive dependency/stack
// inputs, linked specifications and ADRs. Related changes and generated boards
// are not assignment authority. Reads use immutable Git objects, never a mutable
// metadata checkout or a caller-supplied replacement digest.
func agentMetadataBindings(ctx context.Context, reader StatusReader, pin StatusPin, id int) (map[string]string, error) {
	blobs, err := reader.ReadCorpus(ctx, pin)
	if err != nil {
		return nil, err
	}
	inputs, _ := parseCorpus(blobs)
	built, err := repository.BuildSnapshot(repository.BuildInput{Config: pin.Config.Effective, Documents: inputs})
	if err != nil {
		return nil, err
	}
	versions := make(map[string]string, len(blobs))
	for _, blob := range blobs {
		versions[blob.Path] = blob.Version
	}
	bindings := map[string]string{}
	add := func(path string) error {
		if versions[path] == "" {
			return fmt.Errorf("missing record %s", path)
		}
		bindings[path] = versions[path]
		return nil
	}
	seen := map[domain.ChangeID]bool{}
	pending := []domain.ChangeID{domain.ChangeID(id)}
	for len(pending) > 0 {
		id := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if seen[id] {
			continue
		}
		seen[id] = true
		change, found := built.Snapshot.Change(id)
		if found != domain.LookupFound {
			return nil, fmt.Errorf("missing or ambiguous change %d", id)
		}
		if err := add(change.Path()); err != nil {
			return nil, err
		}
		if spec := change.Spec().Value; spec != "" {
			artifact, err := reader.ReadArtifact(ctx, pin, sourceMetadata, spec)
			if err != nil {
				return nil, err
			}
			if !artifact.Found || artifact.Version == "" {
				return nil, fmt.Errorf("missing spec %s", spec)
			}
			bindings[spec] = artifact.Version
		}
		for _, id := range change.ADRs() {
			adr, found := built.Snapshot.ADR(id)
			if found != domain.LookupFound {
				return nil, fmt.Errorf("missing or ambiguous ADR %d", id)
			}
			if err := add(adr.Path()); err != nil {
				return nil, err
			}
		}
		pending = append(pending, change.DependsOn()...)
		if parent := change.StackedOn(); parent.State == domain.FieldPresent {
			pending = append(pending, domain.ChangeID(parent.Value))
		}
	}
	return bindings, nil
}
