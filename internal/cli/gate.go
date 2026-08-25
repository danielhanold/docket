package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/app"
	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/gitcli"
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

	gateCmd.AddCommand(launch, observe, stop, recover, cleanup, newGateDriveCommand(setResult))
	return gateCmd
}

// gateDriveBudgetMinutes is the observation budget the CLI adapter binds when it
// composes the gate-drive service. Finalize (the other adapter over the same
// state machine) resolves the budget from authoritative config; the operator CLI
// surface supplies its own command via the `-- <argv>` boundary and a fixed
// budget, so a caller never smuggles either through the drive request itself.
const gateDriveBudgetMinutes = 30

// newGateDriveCommand builds the `gate drive` subcommand group: the high-level,
// resumable, ownership-transferable driver over the same state machine the
// internal/app seam composes. Every subcommand is a thin adapter — it reads its
// opaque identifiers and the `-- <argv>` boundary, composes the service through
// the app boundary, and lets the presenter own the outcome. No body here branches
// on a drive's state.
func newGateDriveCommand(setResult func(app.OperationResult)) *cobra.Command {
	driveCmd := &cobra.Command{
		Use:   "drive",
		Short: "Drive a supervised gate run through resumable, ownership-transferable slices",
		// A command group resolves its subcommand before Args runs, so anything
		// reaching here named no subcommand; NoArgs names an offending token and
		// the bare `docket gate drive` falls through to RunE's missing-command error.
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("missing command")
		},
	}

	start := &cobra.Command{
		Use:   "start --repo-dir <dir> --run-root <dir> -- <argv...>",
		Short: "Start a drive over the suite command given after `--` and advance one slice",
		// The suite argv arrives only after `--`; the flags before it are
		// docket's, everything after is the suite command. Args is left arbitrary
		// so Cobra collects the argv, and RunE enforces the boundary itself.
		RunE: func(c *cobra.Command, args []string) error {
			repoDir, err := resolveRepoDir(c)
			if err != nil {
				return err
			}
			// ArgsLenAtDash reports the count of positional words before `--`, or
			// -1 when no `--` was present. The suite command must be introduced by
			// `--` (dash >= 0), with no positional word before it (dash == 0) and
			// at least one word after it. A boundary failure is a command failure,
			// not a workflow outcome — no drive is launched before it holds.
			dash := c.ArgsLenAtDash()
			if dash < 0 {
				return errors.New("gate drive start requires the suite command after a `--` separator")
			}
			if dash != 0 {
				return errors.New("gate drive start takes no positional arguments before `--`; the suite command follows `--`")
			}
			argv := args[dash:]
			if len(argv) == 0 {
				return errors.New("gate drive start requires at least one command word after `--`")
			}
			svc, err := buildGateDriveService(c.Context(), repoDir, argv)
			if err != nil {
				return err
			}
			runRoot, _ := c.Flags().GetString("run-root")
			cwd, _ := c.Flags().GetString("cwd")
			if cwd == "" {
				cwd = repoDir
			}
			idempotent, _ := c.Flags().GetBool("idempotent-suite-gate")
			changeID, _ := c.Flags().GetString("change-id")
			taskID, _ := c.Flags().GetString("task-id")
			phase, _ := c.Flags().GetString("phase")
			branch, _ := c.Flags().GetString("branch")
			ref, _ := c.Flags().GetString("ref")
			envHash, _ := c.Flags().GetString("env-hash")
			setResult(gateDrivePresenter{inner: svc.Start(app.GateDriveStartRequest{
				RepoDir:             repoDir,
				Worktree:            repoDir,
				ChangeID:            changeID,
				TaskID:              taskID,
				Phase:               phase,
				Branch:              branch,
				Ref:                 ref,
				Cwd:                 cwd,
				EnvHash:             envHash,
				RunRoot:             runRoot,
				IdempotentSuiteGate: idempotent,
			})})
			return nil
		},
	}
	start.Flags().String("repo-dir", "", "repository worktree to fingerprint and run in (default: current directory)")
	start.Flags().String("run-root", "", "absolute directory that holds raw run slots (required)")
	start.Flags().String("cwd", "", "working directory for the launched suite command (default: --repo-dir)")
	start.Flags().String("change-id", "", "change id the drive certifies (recorded only)")
	start.Flags().String("task-id", "", "task id the drive certifies (recorded only)")
	start.Flags().String("phase", "", "workflow phase the drive certifies (recorded only)")
	start.Flags().String("branch", "", "branch recorded alongside the fingerprint")
	start.Flags().String("ref", "", "ref recorded alongside the fingerprint")
	start.Flags().String("env-hash", "", "canonical launch-environment hash (recorded only)")
	start.Flags().Bool("idempotent-suite-gate", false, "mark the gate idempotent, eligible for the single relaunch")
	_ = start.MarkFlagRequired("run-root")

	advance := &cobra.Command{
		Use:   "advance --drive-id <id> --owner-gen <gen>",
		Short: "Resume the current attempt of a drive through at most one slice",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, err := resolveRepoDir(c)
			if err != nil {
				return err
			}
			svc, err := buildGateDriveService(c.Context(), repoDir, nil)
			if err != nil {
				return err
			}
			id, _ := c.Flags().GetString("drive-id")
			gen, _ := c.Flags().GetString("owner-gen")
			setResult(gateDrivePresenter{inner: svc.Advance(id, gen)})
			return nil
		},
	}
	advance.Flags().String("repo-dir", "", "repository worktree of the drive (default: current directory)")
	advance.Flags().String("drive-id", "", "opaque drive id to resume (required)")
	advance.Flags().String("owner-gen", "", "opaque owner generation proving ownership (required)")
	_ = advance.MarkFlagRequired("drive-id")
	_ = advance.MarkFlagRequired("owner-gen")

	handoff := &cobra.Command{
		Use:   "handoff --drive-id <id> --owner-gen <gen>",
		Short: "Transfer a live drive to a fresh owner, minting a single-use handoff token",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, err := resolveRepoDir(c)
			if err != nil {
				return err
			}
			svc, err := buildGateDriveService(c.Context(), repoDir, nil)
			if err != nil {
				return err
			}
			id, _ := c.Flags().GetString("drive-id")
			gen, _ := c.Flags().GetString("owner-gen")
			setResult(gateDrivePresenter{inner: svc.Handoff(id, gen)})
			return nil
		},
	}
	handoff.Flags().String("repo-dir", "", "repository worktree of the drive (default: current directory)")
	handoff.Flags().String("drive-id", "", "opaque drive id to hand off (required)")
	handoff.Flags().String("owner-gen", "", "opaque owner generation proving ownership (required)")
	_ = handoff.MarkFlagRequired("drive-id")
	_ = handoff.MarkFlagRequired("owner-gen")

	claim := &cobra.Command{
		Use:   "claim --drive-id <id> --handoff-id <token>",
		Short: "Consume a single-use handoff token for a fresh owner generation",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, err := resolveRepoDir(c)
			if err != nil {
				return err
			}
			svc, err := buildGateDriveService(c.Context(), repoDir, nil)
			if err != nil {
				return err
			}
			id, _ := c.Flags().GetString("drive-id")
			handoffID, _ := c.Flags().GetString("handoff-id")
			setResult(gateDrivePresenter{inner: svc.Claim(id, handoffID)})
			return nil
		},
	}
	claim.Flags().String("repo-dir", "", "repository worktree of the drive (default: current directory)")
	claim.Flags().String("drive-id", "", "opaque drive id to claim (required)")
	claim.Flags().String("handoff-id", "", "opaque single-use handoff token to consume (required)")
	_ = claim.MarkFlagRequired("drive-id")
	_ = claim.MarkFlagRequired("handoff-id")

	driveCmd.AddCommand(start, advance, handoff, claim)
	return driveCmd
}

// buildGateDriveService composes the in-process gate-drive seam for one CLI
// invocation: it discovers the repository's Git common directory (the durable
// drive store root), resolves this binary's path (the detached supervisor
// re-exec target), and binds the operator-supplied command as the suite command.
// It reaches the process supervisor only through the app boundary, never
// internal/process directly. A discovery, executable, or service-construction
// failure is a command failure surfaced as an argument error.
func buildGateDriveService(ctx context.Context, repoDir string, command []string) (*app.GateDriveService, error) {
	client, err := gitcli.NewClient()
	if err != nil {
		return nil, err
	}
	repo, err := client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})
	if err != nil {
		return nil, err
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	svc, res, reason := app.NewGateDriveService(repo.CommonDir, exe, gateDriveEffective(command))
	if svc == nil {
		return nil, fmt.Errorf("gate drive service unavailable: %s (%s)", res, reason)
	}
	return svc, nil
}

// gateDriveEffective builds the effective configuration the CLI adapter hands the
// service. The operator's argv (joined into one shell command, launched exactly
// as the finalize gate launches its resolved command via `/bin/sh -c`) is the
// suite command; the budget is the fixed CLI-surface value. A nil command leaves
// the suite command unset, which is correct for advance/handoff/claim, whose
// service construction never consults it.
func gateDriveEffective(command []string) config.Effective {
	eff := config.Effective{
		GateObservation: config.Value[int]{Value: gateDriveBudgetMinutes, Provenance: config.Provenance{Layer: config.LayerBuiltIn}},
	}
	if len(command) > 0 {
		eff.Finalize.TestCommand = config.Value[string]{
			Value:      strings.Join(command, " "),
			Provenance: config.Provenance{Layer: config.LayerBuiltIn},
		}
	}
	return eff
}

// gateDrivePresenter adapts app.GateDriveResult so the PROCESS EXIT STATUS derives
// from the drive's typed workflow outcome, while the emitted JSON stays
// byte-identical to the shared protocol-v1 document. The shared document's
// envelope result is `applied` for a successful operation even when the verdict is
// FAILED or HALTED (the JSON consumer keys on drive.outcome, per the app seam's
// "Typed outcomes"); keying the exit code on ExitCode(result) would therefore
// report a red or halted suite as exit-0 success. The JSON body is json.Marshal of
// the inner result unchanged; only the coarse exit status is remapped, and a
// command failure (no drive document) keeps the inner envelope's result.
type gateDrivePresenter struct{ inner app.GateDriveResult }

func (g gateDrivePresenter) Env() app.Envelope {
	if g.inner.Drive == nil {
		return g.inner.Env()
	}
	return app.NewEnvelope(g.inner.Env().Operation, gateDriveExitResult(g.inner.Drive.Outcome))
}

func (g gateDrivePresenter) HumanText() string { return g.inner.HumanText() }

func (g gateDrivePresenter) MarshalJSON() ([]byte, error) { return json.Marshal(g.inner) }

// gateDriveExitResult maps a typed drive outcome to the coarse process exit
// bucket. WAITING and PASSED are exit-0 (the slice ended safely, or the suite
// passed); FAILED and HALTED — and any unrecognized outcome, fail-closed — are
// non-zero. This mapping governs only the exit code: the JSON body and human text
// carry the true outcome unchanged.
func gateDriveExitResult(o gatedrive.Outcome) app.Result {
	switch o {
	case gatedrive.WAITING, gatedrive.PASSED:
		return app.ResultApplied
	case gatedrive.FAILED:
		return app.ResultGateFailed
	default:
		return app.ResultGateFailed
	}
}
