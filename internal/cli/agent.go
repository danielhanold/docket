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
	var runKey string
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
			client := codexentry.Client{}
			// Optional lifecycle linkage (change 0375 Task 13; key-only since change
			// 0491): --run-key registers this entry's thread as a run participant and,
			// for a root coordinator only, connects a catchable Stop to the run's
			// cancellation path and spawns the detached death guardian for an
			// uncatchable death. No documented flow passes it, so the linkage stays
			// dormant unless a caller opts in.
			isRootCoordinator := contract.LaunchPosture == harness.LaunchRootCoordinator
			if runKey != "" {
				// Preflight the key BEFORE anything is spawned: an unknown key refuses
				// run-not-found instead of surfacing as a generic root-entry failure
				// after Codex already started a thread.
				if lerr := app.CheckRunKey(effectiveCWD, runKey); lerr != nil {
					res, reason, _ := app.ClassifyRunRecordError(lerr)
					setResult(runLinkageRefusal(role, res, reason))
					return nil
				}
				link := runLinkageFor(c.Context(), effectiveCWD, runKey, isRootCoordinator)
				client.Registrar, client.Terminal, client.Canceller = link.registrar, link.terminal, link.canceller
				if isRootCoordinator {
					if guardian, gerr := spawnAgentDeathGuardian(effectiveCWD, runKey); gerr == nil {
						defer guardian.Complete()
					}
				}
			}
			out, err := client.Enter(c.Context(), codexentry.Request{Contract: contract, UserRequest: string(request), CWD: effectiveCWD, ApprovalPolicy: approval, Sandbox: sandbox, Skills: skills})
			if err != nil {
				// A registration-time run fault (e.g. the run was fenced after the
				// preflight) keeps its named token (change 0463).
				if res, reason, ok := app.ClassifyRunRecordError(err); ok {
					setResult(runLinkageRefusal(role, res, reason))
					return nil
				}
				setResult(app.AgentEnterResult{Envelope: app.NewEnvelope(app.OperationAgentEnter, app.ResultExternalFailed), Role: role, Reason: "root-entry-failed", Message: err.Error()})
				return nil
			}
			result := app.AgentEnterResult{Envelope: app.NewEnvelope(app.OperationAgentEnter, app.ResultApplied), Role: role, ThreadID: out.ThreadID, TurnID: out.TurnID, Output: out.Output}
			if out.TerminalRecordFailed {
				// The turn terminated but its terminal evidence could not be
				// persisted: surface it so a downstream closeout knows the record
				// is missing. The run itself is not flipped to failure (change 0441).
				result.Message = "terminal evidence unrecorded; closeout will block until resolved"
			}
			setResult(result)
			return nil
		},
	}
	enter.Flags().StringVar(&role, "role", "", "registered docket role `name` (required)")
	enter.Flags().StringVar(&requestSource, "request", "", "request `file`, or - for stdin (required)")
	enter.Flags().StringVar(&cwd, "cwd", "", "absolute repository working `dir` (required)")
	enter.Flags().StringVar(&approval, "approval-policy", "", "caller approval `policy` (required)")
	enter.Flags().StringVar(&sandbox, "sandbox", "", "caller sandbox `mode` (required)")
	enter.Flags().StringVar(&worktree, "worktree", "", "verified feature worktree `dir` (required for feature child roles)")
	enter.Flags().StringVar(&runKey, "run-key", "", "run `key` for lifecycle registration (optional; a locator, not a credential)")
	for _, flag := range []string{"role", "request", "cwd", "approval-policy", "sandbox"} {
		_ = enter.MarkFlagRequired(flag)
	}
	group.AddCommand(enter)
	return group
}

// runLinkage is the optional lifecycle linkage --run-key wires into one entry.
type runLinkage struct {
	registrar codexentry.ParticipantRegistrar
	terminal  codexentry.TerminalRecorder
	canceller codexentry.LifecycleCanceller
}

// runLinkageFor returns the linkage for runKey (change 0491): none for an empty key;
// otherwise participant registration and terminal recording by key, plus — for a
// root coordinator only — the catchable-Stop cancellation. A feature child
// registers but receives no cancellation authority.
func runLinkageFor(ctx context.Context, repoDir, runKey string, isRootCoordinator bool) runLinkage {
	if runKey == "" {
		return runLinkage{}
	}
	kind := "task"
	if isRootCoordinator {
		kind = "coordinator"
	}
	l := runLinkage{
		registrar: runParticipantRegistrar{repoDir: repoDir, runKey: runKey, kind: kind},
		terminal:  runTerminalRecorder{repoDir: repoDir, runKey: runKey},
	}
	if isRootCoordinator {
		l.canceller = runLifecycleCanceller{ctx: ctx, repoDir: repoDir, runKey: runKey}
	}
	return l
}

// runParticipantRegistrar adapts app.RegisterRunParticipant to codexentry's
// ParticipantRegistrar: it registers this entry's thread as a participant of the
// run its key names (change 0375 Task 13; key-only since change 0491); the
// registration carries no capability.
type runParticipantRegistrar struct {
	repoDir, runKey, kind string
}

func (r runParticipantRegistrar) RegisterParticipant(handle string) error {
	return app.RegisterRunParticipant(r.repoDir, r.runKey, app.RunParticipant{
		Kind:         r.kind,
		NativeHandle: handle,
	})
}

// runLinkageRefusal renders a typed run linkage failure as the agent.enter
// refusal (change 0463): the named reason token and a credential-free next action.
// It never includes the presented value.
func runLinkageRefusal(role string, res app.Result, reason string) app.AgentEnterResult {
	msg := app.RunRecordNextAction(reason)
	if msg == "" {
		msg = "run linkage refused (" + reason + ")"
	}
	return app.AgentEnterResult{Envelope: app.NewEnvelope(app.OperationAgentEnter, res), Role: role, Reason: reason, Message: msg}
}

// runTerminalRecorder adapts app.RecordRunParticipantTerminal to codexentry's
// TerminalRecorder (change 0441 Task 9): after the entry's turn settles, it stamps
// the exact terminal observation onto the matching participant of the run its key
// names; the record carries no capability, and app validates the status value.
type runTerminalRecorder struct {
	repoDir, runKey string
}

func (r runTerminalRecorder) RecordTerminal(handle, turnID, status string) error {
	return app.RecordRunParticipantTerminal(r.repoDir, r.runKey, handle, turnID, status)
}

// runLifecycleCanceller adapts app.RunCancel to codexentry's LifecycleCanceller
// for a root coordinator that catches a Stop. RunCancel validates the run's own
// authority (the record's parent capability + confirmed claim binding) — the
// coordinator's signal cannot manufacture authority it lacks. Task 10's RunCancel
// reconciles from the run journal and ignores the planning deps, so zero-value
// deps are passed rather than requiring a GitHub client at Stop time.
type runLifecycleCanceller struct {
	ctx             context.Context
	repoDir, runKey string
}

func (c runLifecycleCanceller) CancelRun(reason string) error {
	res := app.RunCancel(c.ctx, app.PlanningDeps{}, app.WorkspaceDeps{}, c.repoDir, c.runKey, reason)
	if res.Result == app.ResultBlocked {
		return fmt.Errorf("run cancel refused: %s", res.Disposition)
	}
	return nil
}

// spawnAgentDeathGuardian re-execs this binary as a detached run death guardian for
// runKey under repoDir, so an uncatchable owner death fences the run
// automatically. It resolves the completion-marker path the guardian watches and
// passes this binary's own path as the re-exec target.
func spawnAgentDeathGuardian(repoDir, runKey string) (*app.GuardianHandle, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	marker, err := app.AgentGuardianMarkerPath(repoDir, runKey)
	if err != nil {
		return nil, err
	}
	return app.SpawnAgentGuardian(exe, repoDir, runKey, marker)
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
