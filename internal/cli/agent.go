package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/app"
	"github.com/danielhanold/docket/internal/buildinfo"
	"github.com/danielhanold/docket/internal/codexentry"
	"github.com/danielhanold/docket/internal/harness"
	"github.com/danielhanold/docket/internal/harness/codex"
)

func newAgentCommand(info buildinfo.Info, setResult func(app.OperationResult)) *cobra.Command {
	group := &cobra.Command{Use: "agent", Short: "Enter harness agent roles"}
	var role, requestSource, cwd, approval, sandbox string
	enter := &cobra.Command{
		Use:   "enter",
		Short: "Enter a compositional Codex role as a foreground root thread",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			if role == "" || requestSource == "" || cwd == "" || approval == "" || sandbox == "" {
				return fmt.Errorf("--role, --request, --cwd, --approval-policy, and --sandbox are required")
			}
			if !filepath.IsAbs(cwd) {
				return fmt.Errorf("--cwd must be absolute")
			}
			if err := codexentry.ValidateExecutionContext(approval, sandbox); err != nil {
				return err
			}
			st, err := os.Stat(cwd)
			if err != nil || !st.IsDir() {
				return fmt.Errorf("--cwd must name an existing directory")
			}
			request, err := readRecordSource(c.InOrStdin(), requestSource)
			if err != nil {
				return err
			}
			opts, refusal := installOptions(c.Context(), []string{"codex"}, "", false, info)
			if refusal != nil {
				setResult(app.AgentEnterResult{Envelope: app.NewEnvelope(app.OperationAgentEnter, app.ResultInvalidState), Reason: "role-contract-unavailable", Message: refusal.Error()})
				return nil
			}
			contract, err := codex.RoleContractFor(harness.PlanInput{Assets: opts.Catalog, Agents: opts.Config.Effective.Agents}, role)
			if err != nil {
				setResult(app.AgentEnterResult{Envelope: app.NewEnvelope(app.OperationAgentEnter, app.ResultInvalidInput), Role: role, Reason: "unknown-role", Message: err.Error()})
				return nil
			}
			if contract.LaunchPosture != harness.LaunchRootCoordinator {
				setResult(app.AgentEnterResult{Envelope: app.NewEnvelope(app.OperationAgentEnter, app.ResultInvalidState), Role: role, Reason: "ordinary-child-role", Message: "role is registered for ordinary child launch"})
				return nil
			}
			skills := make([]codexentry.SkillInput, 0, len(contract.Skills))
			for _, name := range contract.Skills {
				skills = append(skills, codexentry.SkillInput{Name: name, Path: filepath.Join(opts.Roots.Home, ".agents", "skills", name, "SKILL.md")})
			}
			out, err := (codexentry.Client{}).Enter(c.Context(), codexentry.Request{Contract: contract, UserRequest: string(request), CWD: cwd, ApprovalPolicy: approval, Sandbox: sandbox, Skills: skills})
			if err != nil {
				setResult(app.AgentEnterResult{Envelope: app.NewEnvelope(app.OperationAgentEnter, app.ResultExternalFailed), Role: role, Reason: "root-entry-failed", Message: err.Error()})
				return nil
			}
			setResult(app.AgentEnterResult{Envelope: app.NewEnvelope(app.OperationAgentEnter, app.ResultApplied), Role: role, ThreadID: out.ThreadID, TurnID: out.TurnID, Output: out.Output})
			return nil
		},
	}
	enter.Flags().StringVar(&role, "role", "", "registered docket role name (required)")
	enter.Flags().StringVar(&requestSource, "request", "", "request file, or - for stdin (required)")
	enter.Flags().StringVar(&cwd, "cwd", "", "absolute repository working directory (required)")
	enter.Flags().StringVar(&approval, "approval-policy", "", "caller approval policy (required)")
	enter.Flags().StringVar(&sandbox, "sandbox", "", "caller sandbox mode (required)")
	group.AddCommand(enter)
	return group
}
