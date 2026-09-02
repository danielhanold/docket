package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/app"
	"github.com/danielhanold/docket/internal/workspace"
)

// This file is the `docket workspace` command family: thin adapters that read
// their scalar flags, hand them to the matching internal/app operation over the
// real Git-backed seams plus the landed workspace service, and let the presenter
// own the outcome. Every policy question — base resolution, the claimed-version
// gate, the expected-head check, disposition mapping — belongs to internal/app,
// so no body here branches on repository content.

// newWorkspaceCommand builds the `workspace` command group. setResult is the
// closure that hands a computed operation result back to Run's single
// presentation point, mirroring newContextCommand.
func newWorkspaceCommand(setResult func(app.OperationResult)) *cobra.Command {
	workspaceCmd := &cobra.Command{
		Use:   "workspace",
		Short: "Prepare, inspect, and publish feature workspaces for in-progress changes",
		// A command group resolves its subcommand before Args runs, so anything
		// reaching here named no subcommand; NoArgs names an offending token and
		// the bare `docket workspace` falls through to RunE's missing-command error.
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("missing command")
		},
	}

	prepare := &cobra.Command{
		Use:         "prepare",
		Short:       "Prepare (or resume) the feature workspace for an in-progress change at an exact version",
		Args:        cobra.NoArgs,
		Annotations: capability("workspace.prepare", EffectLocalWrite),
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, err := resolveRepoDir(c)
			if err != nil {
				return err
			}
			id, _ := c.Flags().GetInt("id")
			version, _ := c.Flags().GetString("version")
			deps, wdeps, err := newWorkspaceDeps()
			if err != nil {
				return err
			}
			setResult(app.WorkspacePrepare(c.Context(), deps, wdeps, repoDir,
				app.WorkspaceIDRequest{ID: id, Version: version}))
			return nil
		},
	}
	prepare.Flags().Int("id", 0, "change id to prepare a workspace for (required)")
	prepare.Flags().String("version", "", "exact record blob object id from the claim receipt (required)")
	prepare.Flags().String("repo-dir", "", "repository directory to operate on (default: current directory)")
	_ = prepare.MarkFlagRequired("id")
	_ = prepare.MarkFlagRequired("version")

	inspect := &cobra.Command{
		Use:         "inspect",
		Short:       "Classify a change's feature workspace state (read-only)",
		Args:        cobra.NoArgs,
		Annotations: capability("workspace.inspect", EffectRead),
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, err := resolveRepoDir(c)
			if err != nil {
				return err
			}
			id, _ := c.Flags().GetInt("id")
			deps, wdeps, err := newWorkspaceDeps()
			if err != nil {
				return err
			}
			setResult(app.WorkspaceInspect(c.Context(), deps, wdeps, repoDir,
				app.WorkspaceIDRequest{ID: id}))
			return nil
		},
	}
	inspect.Flags().Int("id", 0, "change id whose workspace to inspect (required)")
	inspect.Flags().String("repo-dir", "", "repository directory to operate on (default: current directory)")
	_ = inspect.MarkFlagRequired("id")

	publish := &cobra.Command{
		Use:         "publish",
		Short:       "Publish a change's ready workspace head to the remote feature ref",
		Args:        cobra.NoArgs,
		Annotations: capability("workspace.publish", EffectExternalWrite),
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, err := resolveRepoDir(c)
			if err != nil {
				return err
			}
			id, _ := c.Flags().GetInt("id")
			head, _ := c.Flags().GetString("head")
			deps, wdeps, err := newWorkspaceDeps()
			if err != nil {
				return err
			}
			setResult(app.WorkspacePublish(c.Context(), deps, wdeps, repoDir,
				app.WorkspacePublishRequest{ID: id, Head: head}))
			return nil
		},
	}
	publish.Flags().Int("id", 0, "change id whose workspace head to publish (required)")
	publish.Flags().String("head", "", "exact local head the ready workspace must carry (required)")
	publish.Flags().String("repo-dir", "", "repository directory to operate on (default: current directory)")
	_ = publish.MarkFlagRequired("id")
	_ = publish.MarkFlagRequired("head")

	workspaceCmd.AddCommand(prepare, inspect, publish)
	return workspaceCmd
}

// newWorkspaceDeps assembles the read-only planning seams plus the landed
// workspace service, sharing one real Git client between them.
func newWorkspaceDeps() (app.PlanningDeps, app.WorkspaceDeps, error) {
	deps, err := newPlanningDeps()
	if err != nil {
		return app.PlanningDeps{}, app.WorkspaceDeps{}, err
	}
	svc, err := workspace.NewService(deps.Client)
	if err != nil {
		return app.PlanningDeps{}, app.WorkspaceDeps{}, err
	}
	return deps, app.WorkspaceDeps{Service: svc}, nil
}
