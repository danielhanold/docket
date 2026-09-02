package cli

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/app"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
)

// sweepNetworkdeadlines: unlike the standalone finalize subcommands, which keep
// gitcli/githubcli's five-minute default network budget, the maintenance sweep
// probes the remote once per historical record and must not stall the whole pass
// on one wedged connection. Healthy remote ops (ls-remote, a GraphQL PR batch,
// an integration fetch) were measured at ~0.5s; 30s read and 60s write are
// generous ceilings well above that, tight enough to fail a hung connection fast
// yet loose enough never to clip a slow-but-live operation (learnings:
// tolerance-constant-calibrated-on-one-machine — these are ceilings, not tuned
// thresholds, so a slower machine stays comfortably under them). Sweep-only: no
// other command carries these deadlines.
const (
	sweepNetworkReadTimeout  = 30 * time.Second
	sweepNetworkWriteTimeout = 60 * time.Second
)

// newSweepFinalizeDeps assembles the finalize seams the maintenance sweep drives,
// identical in shape to newFinalizeDeps but over Git and GitHub clients carrying
// the sweep-only read/write network deadlines above. Both the top-level probes
// and every nested reader (the transaction engine, status reader, workspace
// service, PR prober, PR batch reader, gate, and CleanupGit) are built over these
// exact two policy-carrying clients, so no reachable network path escapes onto a
// second default client at the five-minute budget.
func newSweepFinalizeDeps() (app.FinalizeDeps, error) {
	gitClient, err := gitcli.NewClient(
		gitcli.WithNetworkReadTimeout(sweepNetworkReadTimeout),
		gitcli.WithNetworkWriteTimeout(sweepNetworkWriteTimeout),
	)
	if err != nil {
		return app.FinalizeDeps{}, err
	}
	ghClient, err := githubcli.NewClient(
		githubcli.WithNetworkReadTimeout(sweepNetworkReadTimeout),
		githubcli.WithNetworkWriteTimeout(sweepNetworkWriteTimeout),
	)
	if err != nil {
		return app.FinalizeDeps{}, err
	}
	return newFinalizeDepsOver(gitClient, ghClient)
}

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
// mutation and reports every item as a structured entry. Two flags ride on it —
// --repo-dir names the target directory and --scope selects the closed sweep
// scope (full or implementation) — and there is no authored request body.
func newMaintenanceSweepSubcommand(setResult func(app.OperationResult)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sweep",
		Short: "Close out merged changes, retry terminal cleanup, and reclaim expired claims in one pass",
		Args:  cobra.NoArgs,
		// metadata-write + local-write + external-write: composes closeout,
		// cleanup, and reclaim and inherits the full union of their effects.
		Annotations: capability("maintenance.sweep", EffectMetadataWrite, EffectLocalWrite, EffectExternalWrite),
		RunE: func(c *cobra.Command, _ []string) error {
			// Scope resolves once, here, to a typed value — the app layer never
			// re-derives it — and an unknown/empty value refuses before any
			// repo, network, or mutation work is even wired.
			scopeStr, err := c.Flags().GetString("scope")
			if err != nil {
				return err
			}
			var scope app.SweepScope
			switch scopeStr {
			case "full":
				scope = app.SweepScopeFull
			case "implementation":
				scope = app.SweepScopeImplementation
			default:
				return fmt.Errorf("invalid --scope %q: must be full or implementation", scopeStr)
			}
			repoDir, err := resolveRepoDir(c)
			if err != nil {
				return err
			}
			deps, err := newSweepFinalizeDeps()
			if err != nil {
				return err
			}
			setResult(app.MaintenanceSweep(c.Context(), deps, repoDir, scope))
			return nil
		},
	}
	cmd.Flags().String("repo-dir", "", "repository `dir` to operate on (default: current directory)")
	cmd.Flags().String("scope", "full", "sweep scope `name`: full (whole worklist, the default) or implementation (startup preflight; defers independent historical cleanup retries)")
	return cmd
}
