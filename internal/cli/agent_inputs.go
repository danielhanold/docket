package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/app"
	"github.com/danielhanold/docket/internal/codexcontract"
)

func newAgentCheckInputsCommand(setResult func(app.OperationResult)) *cobra.Command {
	var req app.CheckInputsRequest
	cmd := &cobra.Command{Use: "check-inputs", Short: "Validate a pinned native-agent assignment and payload", Args: cobra.NoArgs, Annotations: capability(app.OperationAgentCheckInputs, EffectRead), RunE: func(c *cobra.Command, _ []string) error {
		a, err := codexcontract.ReadAssignment(req.Assignment, req.SHA256)
		if err != nil {
			setResult(app.CheckInputsResult{Envelope: app.NewEnvelope(app.OperationAgentCheckInputs, app.ResultInvalidInput), Reason: "assignment-invalid: " + err.Error()})
			return nil
		}
		deps, err := app.NewAgentInputDeps(a.DocketExecutable)
		if err != nil {
			return fmt.Errorf("agent input observer: %w", err)
		}
		planning, workspaceDeps, err := newWorkspaceDeps(req.RepoDir)
		if err != nil {
			return fmt.Errorf("agent workspace validator: %w", err)
		}
		deps.Workspace = app.NewAgentWorkspaceValidator(planning, workspaceDeps)
		if req.Stage == "dispatch" || (req.Stage == "entry" && req.Payload != "") {
			deps.Scope, err = app.NewAgentScopeValidator(a.CommonDir, a.DocketExecutable)
			if err != nil {
				return fmt.Errorf("agent scope validator: %w", err)
			}
		}
		setResult(app.CheckAgentInputs(c.Context(), deps, req))
		return nil
	}}
	cmd.Flags().StringVar(&req.Assignment, "assignment", "", "absolute assignment file")
	cmd.Flags().StringVar(&req.SHA256, "sha256", "", "assignment sha256")
	cmd.Flags().StringVar(&req.Stage, "stage", "", "prepare, dispatch, entry, or active")
	cmd.Flags().StringVar(&req.Payload, "payload", "", "absolute private payload file")
	cmd.Flags().StringVar(&req.PayloadSHA256, "payload-sha256", "", "private payload sha256")
	cmd.Flags().StringVar(&req.RepoDir, "repo-dir", "", "absolute primary or feature root")
	for _, f := range []string{"assignment", "sha256", "stage"} {
		_ = cmd.MarkFlagRequired(f)
	}
	return cmd
}
