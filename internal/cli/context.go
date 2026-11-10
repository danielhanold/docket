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
		Short: "Assemble read-only context bundles for docket workflows",
		// A command group resolves its subcommand before Args runs, so anything
		// reaching here named no subcommand; NoArgs names an offending token and
		// the bare `docket context` falls through to RunE's missing-command error.
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("missing command")
		},
	}

	implementation := &cobra.Command{
		Use:         "implementation",
		Short:       "Assemble the authoritative implementation-context bundle (read-only)",
		Args:        cobra.NoArgs,
		Annotations: capability("context.implementation", EffectRead),
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, err := resolveRepoDir(c)
			if err != nil {
				return err
			}
			id, _ := c.Flags().GetInt("id")
			deps, err := newPlanningDeps()
			if err != nil {
				return err
			}
			setResult(app.ContextImplementation(c.Context(), deps, repoDir, app.ImplementationContextRequest{ID: id}))
			return nil
		},
	}
	implementation.Flags().String("repo-dir", "", "repository `dir` to read (default: current directory)")
	implementation.Flags().Int("id", 0, "inspect this exact change `id` instead of applying the selection policy")

	finalize := &cobra.Command{
		Use:         "finalize",
		Short:       "Assemble the authoritative finalize-context bundle (read-only)",
		Args:        cobra.NoArgs,
		Annotations: capability("context.finalize", EffectRead),
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, err := resolveRepoDir(c)
			if err != nil {
				return err
			}
			id, _ := c.Flags().GetInt("id")
			allowlist, _ := c.Flags().GetIntSlice("allowlist")
			deps, err := newFinalizeDeps()
			if err != nil {
				return err
			}
			setResult(app.ContextFinalize(c.Context(), deps, repoDir, app.FinalizeContextRequest{ID: id, Allowlist: allowlist}))
			return nil
		},
	}
	finalize.Flags().String("repo-dir", "", "repository `dir` to read (default: current directory)")
	finalize.Flags().Int("id", 0, "inspect this exact change `id` instead of applying the selection policy")
	finalize.Flags().IntSlice("allowlist", nil, "bound the finalize selection to these change `id`s (repeatable/comma-separated)")

	ticket := &cobra.Command{
		Use:         "ticket",
		Short:       "Assemble what a ticket update needs for one change (read-only)",
		Args:        cobra.NoArgs,
		Annotations: capability("context.ticket", EffectRead),
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, err := resolveRepoDir(c)
			if err != nil {
				return err
			}
			id, _ := c.Flags().GetInt("id")
			event, _ := c.Flags().GetString("event")
			deps, err := newPlanningDeps()
			if err != nil {
				return err
			}
			setResult(app.ContextTicketRead(c.Context(), deps, repoDir, app.TicketContextRequest{ID: id, Event: event}))
			return nil
		},
	}
	ticket.Flags().String("repo-dir", "", "repository `dir` to read (default: current directory)")
	ticket.Flags().Int("id", 0, "the change `id` to read")
	ticket.Flags().String("event", "", "the `event` the update follows: spec_written, pr_opened, pr_merged, or results_completed; omit for an update with no event")

	contextCmd.AddCommand(implementation, finalize, ticket)
	return contextCmd
}
