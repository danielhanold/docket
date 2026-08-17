package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/app"
)

// This file is the `docket context` command family: thin adapters that read
// their flags, hand them to the matching read-only internal/app context
// operation over the real Git-backed seams, and let the presenter own the
// outcome. Every policy question — selection, readiness, claim eligibility,
// effective-base resolution — belongs to internal/app, so no body here branches
// on repository content.

// newContextCommand builds the `context` command group. setResult is the
// closure that hands a computed operation result back to Run's single
// presentation point, mirroring newChangeCommand.
func newContextCommand(setResult func(app.OperationResult)) *cobra.Command {
	contextCmd := &cobra.Command{
		Use:   "context",
		Short: "Assemble read-only context bundles for the implementation workflow",
		// A command group resolves its subcommand before Args runs, so anything
		// reaching here named no subcommand; NoArgs names an offending token and
		// the bare `docket context` falls through to RunE's missing-command error.
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("missing command")
		},
	}

	implementation := &cobra.Command{
		Use:   "implementation",
		Short: "Assemble the authoritative implementation-context bundle (read-only)",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, _ := c.Flags().GetString("repo-dir")
			id, _ := c.Flags().GetInt("id")
			deps, err := newPlanningDeps()
			if err != nil {
				return err
			}
			setResult(app.ContextImplementation(c.Context(), deps, repoDir, app.ImplementationContextRequest{ID: id}))
			return nil
		},
	}
	implementation.Flags().String("repo-dir", "", "repository directory to read (default: current directory)")
	implementation.Flags().Int("id", 0, "inspect this exact change id instead of applying the selection policy")

	contextCmd.AddCommand(implementation)
	return contextCmd
}
