package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/app"
)

// This file is the `docket leak` command family: a thin adapter that reads its
// flags and hands them to the read-only app.LeakCheck operation over the real
// Git-backed seams, letting the presenter own the outcome. Which rules run, the
// options they run with, and how a missing or unreadable file is refused all
// belong to internal/app.

// newLeakCommand builds the `leak` command group. setResult is the closure that
// hands a computed operation result back to Run's single presentation point,
// mirroring newContextCommand.
func newLeakCommand(setResult func(app.OperationResult)) *cobra.Command {
	leakCmd := &cobra.Command{
		Use:   "leak",
		Short: "Check text bound for outsiders against the leak check (read-only)",
		// A command group resolves its subcommand before Args runs, so anything
		// reaching here named no subcommand; NoArgs names an offending token and
		// the bare `docket leak` falls through to RunE's missing-command error.
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("missing command")
		},
	}

	check := &cobra.Command{
		Use:         "check",
		Short:       "Scan files for docket fingerprints with the publish leak check's rules (read-only)",
		Args:        cobra.NoArgs,
		Annotations: capability(app.OperationLeakCheck, EffectRead),
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, err := resolveRepoDir(c)
			if err != nil {
				return err
			}
			files, _ := c.Flags().GetStringArray("file")
			deps, err := newPlanningDeps()
			if err != nil {
				return err
			}
			setResult(app.LeakCheck(c.Context(), deps, repoDir, app.LeakCheckRequest{Files: files}))
			return nil
		},
	}
	// --file is deliberately not required: the operation refuses a missing file
	// as invalid-input, so the refusal is one protocol document like any other.
	check.Flags().StringArray("file", nil, "a `path` to scan, its name included (repeatable)")
	check.Flags().String("repo-dir", "", "repository `dir` to read (default: current directory)")

	leakCmd.AddCommand(check)
	return leakCmd
}
