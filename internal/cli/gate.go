package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/app"
)

// This file is the `docket gate` command family: thin adapters that read their
// flags and the `-- <argv...>` boundary, hand the request to the matching
// internal/app gate operation, and let the presenter own the outcome. Every
// policy question — validation, ownership, state mapping, the exit code —
// belongs to internal/app (over internal/process); no body here branches on a
// run's state. internal/cli must never import internal/process (the dependency
// direction cli -> app -> process is guarded in gate_test.go), so this file
// reaches the supervisor only through the app boundary.

// newGateCommand builds the `gate` command group. setResult is the closure that
// hands a computed operation result back to Run's single presentation point,
// mirroring newChangeCommand and the inline diagnostic group.
func newGateCommand(setResult func(app.OperationResult)) *cobra.Command {
	gateCmd := &cobra.Command{
		Use:   "gate",
		Short: "Launch, observe, stop, and recover supervised local gate runs",
		// A command group resolves its subcommand before Args runs, so anything
		// reaching here named no subcommand; NoArgs names an offending token and
		// the bare `docket gate` falls through to RunE's missing-command error —
		// byte-parity with the diagnostic and development groups.
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("missing command")
		},
	}

	launch := &cobra.Command{
		Use:   "launch --root <dir> --cwd <dir> -- <argv...>",
		Short: "Launch a supervised run, its command given after a -- separator",
		// The command argv arrives only after `--`; the flags before it are
		// docket's, everything after is the child's. Args is left arbitrary so
		// Cobra collects the argv, and RunE enforces the boundary itself.
		RunE: func(c *cobra.Command, args []string) error {
			root, _ := c.Flags().GetString("root")
			cwd, _ := c.Flags().GetString("cwd")
			// ArgsLenAtDash reports the count of positional words before `--`,
			// or -1 when no `--` was present. The child argv must be introduced
			// by `--` (dash >= 0), with no positional word before it (dash == 0)
			// and at least one word after it.
			dash := c.ArgsLenAtDash()
			if dash < 0 {
				return errors.New("gate launch requires the command argv after a `--` separator")
			}
			if dash != 0 {
				return errors.New("gate launch takes no positional arguments before `--`; the command argv follows `--`")
			}
			argv := args[dash:]
			if len(argv) == 0 {
				return errors.New("gate launch requires at least one command word after `--`")
			}
			setResult(app.GateLaunch(root, cwd, argv))
			return nil
		},
	}
	launch.Flags().String("root", "", "absolute directory that holds run slots (required)")
	launch.Flags().String("cwd", "", "absolute working directory for the launched command (required)")
	_ = launch.MarkFlagRequired("root")
	_ = launch.MarkFlagRequired("cwd")

	observe := &cobra.Command{
		Use:   "observe <run-dir>",
		Short: "Report a run's state (read-only)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			setResult(app.GateObserve(args[0]))
			return nil
		},
	}

	stop := &cobra.Command{
		Use:   "stop <run-dir>",
		Short: "Stop a supervised run, recording the reason",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			reason, _ := c.Flags().GetString("reason")
			setResult(app.GateStop(args[0], reason))
			return nil
		},
	}
	stop.Flags().String("reason", "", "human-supplied reason recorded with the stop intent")

	recover := &cobra.Command{
		Use:   "recover --root <dir>",
		Short: "Mark proved-abandoned owned runs under a root, retaining everything else",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			root, _ := c.Flags().GetString("root")
			setResult(app.GateRecover(root))
			return nil
		},
	}
	recover.Flags().String("root", "", "absolute directory that holds run slots to scan (required)")
	_ = recover.MarkFlagRequired("root")

	cleanup := &cobra.Command{
		Use:   "cleanup <run-dir>",
		Short: "Remove one owned, terminal, reported run directory's logs, retaining every other run",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			// GateCleanup reads only the run directory; the FinalizeDeps parameter is
			// present for signature parity with the other cleanup operation and is
			// unused, so an empty value is correct here.
			setResult(app.GateCleanup(c.Context(), app.FinalizeDeps{}, args[0]))
			return nil
		},
	}

	gateCmd.AddCommand(launch, observe, stop, recover, cleanup)
	return gateCmd
}
