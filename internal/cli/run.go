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

	// gate-before arms the implement-next run gate: it re-syncs, records the
	// before-set + dispatch epoch in a durable record, and prints `gate-armed
	// <key>` (or `gate-unarmed <reason>`). The sole positional argument is the
	// gate target; only `implement-next` is accepted, and any other value is an
	// invalid-input result (non-zero exit) the app layer owns. It reuses the same
	// read-only planning seams as verify.
	gateBefore := &cobra.Command{
		Use:   "gate-before <target>",
		Short: "Arm the run gate for a dispatched workflow and print gate-armed <key>",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			repoDir, err := resolveRepoDir(c)
			if err != nil {
				return err
			}
			deps, _, _, err := newPRDeps()
			if err != nil {
				return err
			}
			setResult(app.RunGateBefore(c.Context(), deps, repoDir, args[0]))
			return nil
		},
	}
	gateBefore.Flags().String("repo-dir", "", "repository directory to operate on (default: current directory)")

	// gate-verdict <key> reports the attributed run-gate verdict: it loads the
	// durable record armed by gate-before, attributes exactly one new in-progress
	// claim, delegates the run predicate to app.RunVerify, and prints one line of
	// the attributed vocabulary (gate-done / gate-retry-once / gate-stop …). It
	// wires the SAME read-only planning + workspace + GitHub seams as verify,
	// including the best-effort local run-waiting receipt reader, because the run
	// predicate it delegates to is verify's. Every outcome is a report line that
	// exits 0; the key is the sole positional argument.
	gateVerdict := &cobra.Command{
		Use:   "gate-verdict <key>",
		Short: "Report the attributed run-gate verdict for a dispatched workflow",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			repoDir, err := resolveRepoDir(c)
			if err != nil {
				return err
			}
			deps, wdeps, gdeps, err := newPRDeps()
			if err != nil {
				return err
			}
			wdeps.Waiting = newWaitingReader(c.Context(), repoDir)
			setResult(app.RunGateVerdict(c.Context(), deps, wdeps, gdeps, repoDir, args[0]))
			return nil
		},
	}
	gateVerdict.Flags().String("repo-dir", "", "repository directory to operate on (default: current directory)")

	runCmd.AddCommand(verify, gateBefore, gateVerdict)
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
