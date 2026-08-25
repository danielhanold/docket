package cli

import (
	"context"
	"errors"
	"os"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/app"
	"github.com/danielhanold/docket/internal/gitcli"
)

// This file is the `docket run` command family: a thin adapter that reads its
// flags, hands them to the read-only app.RunVerify operation over the real
// Git-backed seams plus the landed workspace service and the githubcli adapter,
// and lets the presenter own the outcome. Every policy question — which
// postconditions, which verdict, which reasons, which exit code — belongs to
// internal/app, so no body here branches on repository, workspace, or GitHub
// content.

// newRunCommand builds the `run` command group. setResult is the closure that
// hands a computed operation result back to Run's single presentation point,
// mirroring newPRCommand.
func newRunCommand(setResult func(app.OperationResult)) *cobra.Command {
	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Report on a change's claim-to-implemented run (read-only)",
		// A command group resolves its subcommand before Args runs, so anything
		// reaching here named no subcommand; NoArgs names an offending token and
		// the bare `docket run` falls through to RunE's missing-command error.
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("missing command")
		},
	}

	verify := &cobra.Command{
		Use:   "verify",
		Short: "Verify one change's implemented-run postconditions and report a closed verdict",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, err := resolveRepoDir(c)
			if err != nil {
				return err
			}
			id, _ := c.Flags().GetInt("id")
			deps, wdeps, gdeps, err := newPRDeps()
			if err != nil {
				return err
			}
			// Best-effort wire the local run-waiting receipt reader. It is an
			// ADDITIVE derivation: if the drive store root or the supervisor cannot
			// be resolved, run verify simply reports its ordinary postcondition
			// verdict rather than deriving run-waiting — never a report failure.
			wdeps.Waiting = newWaitingReader(c.Context(), repoDir)
			setResult(app.RunVerify(c.Context(), deps, wdeps, gdeps, repoDir, app.RunVerifyRequest{ID: id}))
			return nil
		},
	}
	verify.Flags().Int("id", 0, "change id whose run to verify (required)")
	verify.Flags().String("repo-dir", "", "repository directory to operate on (default: current directory)")
	_ = verify.MarkFlagRequired("id")

	runCmd.AddCommand(verify)
	return runCmd
}

// newWaitingReader composes the production run-waiting receipt reader for repoDir,
// rooting the durable drive store at the repository's Git common directory and
// binding the native supervisor at this binary's path. Every resolution step is
// best-effort: any failure returns a nil reader, and run verify then reports its
// ordinary postcondition verdict without deriving run-waiting. internal/cli never
// imports internal/process — it reaches the supervisor only through the app
// boundary (app.NewWaitingReceiptReader), exactly as the gate-drive adapters do.
func newWaitingReader(ctx context.Context, repoDir string) app.WaitingReceiptReader {
	client, err := gitcli.NewClient()
	if err != nil {
		return nil
	}
	repo, err := client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})
	if err != nil {
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return nil
	}
	reader, err := app.NewWaitingReceiptReader(repo.CommonDir, exe)
	if err != nil {
		return nil
	}
	return reader
}
