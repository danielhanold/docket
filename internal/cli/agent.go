package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/app"
	"github.com/danielhanold/docket/internal/buildinfo"
	"github.com/danielhanold/docket/internal/codexentry"
	"github.com/danielhanold/docket/internal/harness"
	"github.com/danielhanold/docket/internal/harness/codex"
	"github.com/danielhanold/docket/internal/install"
)

func newAgentCommand(info buildinfo.Info, setResult func(app.OperationResult)) *cobra.Command {
	group := &cobra.Command{Use: "agent", Short: "Enter harness agent roles"}
	var role, requestSource, cwd, approval, sandbox string
	enter := &cobra.Command{
		Use:   "enter",
		Short: "Enter a compositional Codex role as a foreground root thread",
		Args:  cobra.NoArgs,
		Annotations: capability("agent.enter", EffectProcessControl,
			EffectLocalWrite),
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
			if err := validateInstalledRole(opts, contract); err != nil {
				setResult(app.AgentEnterResult{Envelope: app.NewEnvelope(app.OperationAgentEnter, app.ResultInvalidState), Role: role, Reason: "role-contract-unavailable", Message: err.Error()})
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
	enter.Flags().StringVar(&role, "role", "", "registered docket role `name` (required)")
	enter.Flags().StringVar(&requestSource, "request", "", "request `file`, or - for stdin (required)")
	enter.Flags().StringVar(&cwd, "cwd", "", "absolute repository working `dir` (required)")
	enter.Flags().StringVar(&approval, "approval-policy", "", "caller approval `policy` (required)")
	enter.Flags().StringVar(&sandbox, "sandbox", "", "caller sandbox `mode` (required)")
	for _, flag := range []string{"role", "request", "cwd", "approval-policy", "sandbox"} {
		_ = enter.MarkFlagRequired(flag)
	}
	group.AddCommand(enter)
	return group
}

// Protocol compatibility alone cannot prove that the role Codex registered is
// the one this binary will enter. Compare the selected installed target with
// the same planner used by install, and compare its preloads with the catalog.
// This reads only: edited or stale contracts require an explicit reinstall.
func validateInstalledRole(opts install.Options, contract codex.RoleContract) error {
	targets, err := codex.New().Plan(harness.PlanInput{
		Roots: opts.Roots, Assets: opts.Catalog, Agents: opts.Config.Effective.Agents,
		AssetsDir: opts.Roots.VersionDir(opts.Catalog.Manifest.AssetSetID),
	})
	if err != nil {
		return err
	}
	check := func(path string, expected []byte) error {
		actual, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("installed role contract unavailable at %s: %w; run docket install", path, err)
		}
		if !bytes.Equal(actual, expected) {
			return fmt.Errorf("installed role contract differs at %s; run docket install before root entry", path)
		}
		return nil
	}
	found := false
	for _, target := range targets {
		if target.Kind == install.KindFile && filepath.Base(target.Path) == contract.Name+".toml" {
			found = true
			if err := check(target.Path, target.Content); err != nil {
				return err
			}
			break
		}
	}
	if !found {
		return fmt.Errorf("installed role target unavailable for %s", contract.Name)
	}
	for _, name := range contract.Skills {
		expected, err := opts.Catalog.Bytes("skills/" + name + "/SKILL.md")
		if err != nil {
			return err
		}
		if err := check(filepath.Join(opts.Roots.Home, ".agents", "skills", name, "SKILL.md"), expected); err != nil {
			return err
		}
	}
	return nil
}
