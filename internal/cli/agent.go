package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/app"
	"github.com/danielhanold/docket/internal/buildinfo"
	"github.com/danielhanold/docket/internal/codexentry"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/harness"
	"github.com/danielhanold/docket/internal/harness/codex"
	"github.com/danielhanold/docket/internal/install"
)

func newAgentCommand(info buildinfo.Info, setResult func(app.OperationResult)) *cobra.Command {
	group := &cobra.Command{Use: "agent", Short: "Enter harness agent roles"}
	var role, requestSource, cwd, approval, sandbox, worktree string
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
			var git *gitcli.Client
			if contract.LaunchPosture == harness.LaunchChild && contract.WorktreeScope == harness.WorktreeScopeFeature {
				git, err = gitcli.NewClient()
				if err != nil {
					return fmt.Errorf("creating git client: %w", err)
				}
			}
			effectiveCWD, reason, err := resolveAgentEntryCWD(c.Context(), git, contract, cwd, worktree)
			if err != nil {
				return err
			}
			if reason != "" {
				setResult(app.AgentEnterResult{Envelope: app.NewEnvelope(app.OperationAgentEnter, app.ResultInvalidState), Role: role, Reason: reason, Message: agentEntryRefusalMessage(reason)})
				return nil
			}
			contract, rolePath, err := resolveInstalledRoleContract(c.Context(), git, opts, contract, effectiveCWD)
			if err != nil {
				setResult(app.AgentEnterResult{Envelope: app.NewEnvelope(app.OperationAgentEnter, app.ResultInvalidState), Role: role, Reason: "role-contract-unavailable", Message: err.Error()})
				return nil
			}
			if err := validateInstalledRole(opts, contract, rolePath); err != nil {
				setResult(app.AgentEnterResult{Envelope: app.NewEnvelope(app.OperationAgentEnter, app.ResultInvalidState), Role: role, Reason: "role-contract-unavailable", Message: err.Error()})
				return nil
			}
			skills := make([]codexentry.SkillInput, 0, len(contract.Skills))
			for _, name := range contract.Skills {
				skills = append(skills, codexentry.SkillInput{Name: name, Path: filepath.Join(opts.Roots.Home, ".agents", "skills", name, "SKILL.md")})
			}
			out, err := (codexentry.Client{}).Enter(c.Context(), codexentry.Request{Contract: contract, UserRequest: string(request), CWD: effectiveCWD, ApprovalPolicy: approval, Sandbox: sandbox, Skills: skills})
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
	enter.Flags().StringVar(&worktree, "worktree", "", "verified feature worktree `dir` (required for feature child roles)")
	for _, flag := range []string{"role", "request", "cwd", "approval-policy", "sandbox"} {
		_ = enter.MarkFlagRequired(flag)
	}
	group.AddCommand(enter)
	return group
}

// resolveAgentEntryCWD keeps role admission and checkout identity together at
// the app-server boundary. Root coordinators retain their caller's literal cwd;
// a feature child can enter only the exact linked worktree registered to the
// caller's repository.
func resolveAgentEntryCWD(ctx context.Context, git *gitcli.Client, contract codex.RoleContract, callerCWD, requestedWorktree string) (effectiveCWD string, reason string, err error) {
	if contract.LaunchPosture == harness.LaunchRootCoordinator {
		if requestedWorktree != "" {
			return "", "worktree-contradicts-role", nil
		}
		return callerCWD, "", nil
	}
	if contract.LaunchPosture != harness.LaunchChild || contract.WorktreeScope != harness.WorktreeScopeFeature {
		return "", "ordinary-child-role", nil
	}
	if requestedWorktree == "" {
		return "", "worktree-required", nil
	}
	if !filepath.IsAbs(callerCWD) {
		return "", "", fmt.Errorf("--cwd must be absolute")
	}
	if !filepath.IsAbs(requestedWorktree) {
		return "", "", fmt.Errorf("--worktree must be absolute")
	}
	if !isDirectory(callerCWD) {
		return "", "caller-worktree-not-directory", nil
	}
	if !isDirectory(requestedWorktree) {
		return "", "worktree-not-directory", nil
	}
	callerRepo, discoverErr := git.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: callerCWD})
	if discoverErr != nil {
		return "", "caller-worktree-unregistered", nil
	}
	targetRepo, discoverErr := git.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: requestedWorktree})
	if discoverErr != nil {
		return "", "worktree-unregistered", nil
	}
	if callerRepo.CommonDir != targetRepo.CommonDir {
		return "", "worktree-foreign-repository", nil
	}
	target, discoverErr := git.DiscoverWorktree(ctx, gitcli.DiscoverOptions{InvocationPath: requestedWorktree})
	if discoverErr != nil {
		return "", "worktree-unregistered", nil
	}
	canonicalRequested, canonicalErr := canonicalAgentEntryWorktreePath(requestedWorktree)
	if canonicalErr != nil {
		return "", "worktree-not-directory", nil
	}
	if canonicalRequested != target.Root {
		return "", "worktree-not-root", nil
	}
	if target.Root == targetRepo.PrimaryWorktree {
		return "", "worktree-primary", nil
	}
	registered, listErr := git.ListWorktrees(ctx, callerRepo)
	if listErr != nil {
		return "", "worktree-registration-unavailable", nil
	}
	for _, info := range registered {
		canonicalRegistered, canonicalErr := canonicalAgentEntryWorktreePath(info.Path)
		if canonicalErr == nil && canonicalRegistered == target.Root {
			return target.Root, "", nil
		}
	}
	return "", "worktree-unregistered", nil
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func canonicalAgentEntryWorktreePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

func agentEntryRefusalMessage(reason string) string {
	return "agent entry refused: " + reason
}

// Protocol compatibility alone cannot prove that the role Codex registered is
// the one this binary will enter. The native role is selected using Codex's
// repository-before-global precedence and parsed into the inventory contract.
func resolveInstalledRoleContract(ctx context.Context, git *gitcli.Client, opts install.Options, inventory codex.RoleContract, effectiveCWD string) (codex.RoleContract, string, error) {
	globalPath := filepath.Join(opts.Roots.Home, ".codex", "agents", inventory.Name+".toml")
	rolePath := globalPath
	if git == nil {
		var err error
		git, err = gitcli.NewClient()
		if err != nil {
			return codex.RoleContract{}, "", fmt.Errorf("creating git client: %w", err)
		}
	}
	if wt, err := git.DiscoverWorktree(ctx, gitcli.DiscoverOptions{InvocationPath: effectiveCWD}); err == nil {
		repoPath := filepath.Join(wt.Root, ".codex", "agents", inventory.Name+".toml")
		if _, statErr := os.Lstat(repoPath); statErr == nil {
			rolePath = repoPath
		} else if !os.IsNotExist(statErr) {
			return codex.RoleContract{}, "", fmt.Errorf("inspecting repository role contract at %s: %w", repoPath, statErr)
		}
	}
	data, err := os.ReadFile(rolePath)
	if err != nil {
		return codex.RoleContract{}, "", fmt.Errorf("installed role contract unavailable at %s: %w; run docket install", rolePath, err)
	}
	contract, err := codex.RoleContractFromDefinition(data, inventory)
	if err != nil {
		return codex.RoleContract{}, "", fmt.Errorf("installed role contract invalid at %s: %w; run docket install", rolePath, err)
	}
	return contract, rolePath, nil
}

// validateInstalledRole proves the global fallback byte-for-byte against the
// installer plan and every skill preload against the catalog. Repository roles
// are already syntax- and identity-checked by resolveInstalledRoleContract;
// their native values are deliberately allowed to differ from the global plan.
func validateInstalledRole(opts install.Options, contract codex.RoleContract, rolePath string) error {
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
			// The planner proves the global fallback byte-for-byte. A repository
			// definition is Codex's higher-precedence registered source and was
			// parsed and identity-checked above; comparing it to the global plan
			// would erase the precedence this command must preserve.
			if rolePath == target.Path {
				if err := check(target.Path, target.Content); err != nil {
					return err
				}
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
