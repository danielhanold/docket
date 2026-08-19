package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/app"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/workspace"
)

// This file is the top-level `docket finalize` command family: thin adapters
// that read their flags, hand them to the matching internal/app finalize
// operation over the real Git/GitHub/workspace seams, and let the presenter own
// the outcome. The terminal-half mutation subcommands land here in later change
// 0316 tasks; today the group is registered so its tree and shared dependency
// wiring exist for those subcommands to attach to. Every lifecycle, Git,
// GitHub, stack, reclaim, and cleanup policy belongs to internal/app, so no body
// here branches on repository content.

// newFinalizeCommand builds the `finalize` command group. setResult is the
// closure that hands a computed operation result back to Run's single
// presentation point, mirroring newChangeCommand.
func newFinalizeCommand(_ func(app.OperationResult)) *cobra.Command {
	finalizeCmd := &cobra.Command{
		Use:   "finalize",
		Short: "Sequence a change's terminal half: rebase, publish, merge, and closeout",
		// A command group resolves its subcommand before Args runs, so anything
		// reaching here named no subcommand; NoArgs names an offending token and
		// the bare `docket finalize` falls through to RunE's missing-command error.
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("missing command")
		},
	}
	return finalizeCmd
}

// newFinalizeDeps assembles the production seams every finalize operation needs:
// the read-only planning seams (reader/engine/git client/clock), the GitHub
// client, the workspace service over the same git client, and the PR-facts
// prober composed over the GitHub client.
func newFinalizeDeps() (app.FinalizeDeps, error) {
	planning, err := newPlanningDeps()
	if err != nil {
		return app.FinalizeDeps{}, err
	}
	ghClient, err := githubcli.NewClient()
	if err != nil {
		return app.FinalizeDeps{}, err
	}
	ws, err := workspace.NewService(planning.Client)
	if err != nil {
		return app.FinalizeDeps{}, err
	}
	return app.FinalizeDeps{
		Planning:  planning,
		GitHub:    ghClient,
		Workspace: ws,
		PRProber:  app.NewGitHubFinalizeProber(ghClient),
	}, nil
}
