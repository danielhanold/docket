package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/app"
)

// This file is the top-level `docket maintenance` command family: thin adapters
// that read their flags, hand them to the matching internal/app maintenance
// operation over the real Git/GitHub/workspace seams, and let the presenter own
// the outcome. `docket status` stays read-only; batch mutation of docket's
// terminal half is reached only through `docket maintenance sweep`. Every
// lifecycle, Git, GitHub, stack, reclaim, and cleanup policy belongs to
// internal/app, so no body here branches on repository content.

// newMaintenanceCommand builds the `maintenance` command group. setResult is the
// closure that hands a computed operation result back to Run's single
// presentation point, mirroring newFinalizeCommand and newGateCommand.
func newMaintenanceCommand(setResult func(app.OperationResult)) *cobra.Command {
	maintenanceCmd := &cobra.Command{
		Use:   "maintenance",
		Short: "Reclaim docket's terminal half in batch (docket status stays read-only)",
		// A command group resolves its subcommand before Args runs, so anything
		// reaching here named no subcommand; NoArgs names an offending token and
		// the bare `docket maintenance` falls through to RunE's missing-command
		// error — byte-parity with the finalize and gate groups.
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("missing command")
		},
	}
	maintenanceCmd.AddCommand(newMaintenanceSweepSubcommand(setResult))
	return maintenanceCmd
}

// newMaintenanceSweepSubcommand builds `maintenance sweep`: one pinned inventory,
// processed in a deterministic order, that closes out merged changes (stacked
// children before ancestors), retries terminal backlink repair and ownership-safe
// cleanup for archived/done records and completed stacks, and reclaims expired
// claims when reclaim.auto is on. It reloads fresh authority before every
// mutation and reports every item as a structured entry. Only the target
// directory rides on a flag; there is no authored request body.
func newMaintenanceSweepSubcommand(setResult func(app.OperationResult)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sweep",
		Short: "Close out merged changes, retry terminal cleanup, and reclaim expired claims in one pass",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, _ := c.Flags().GetString("repo-dir")
			deps, err := newFinalizeDeps()
			if err != nil {
				return err
			}
			setResult(app.MaintenanceSweep(c.Context(), deps, repoDir))
			return nil
		},
	}
	cmd.Flags().String("repo-dir", "", "repository directory to operate on (default: current directory)")
	return cmd
}
