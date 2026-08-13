package cli

import (
	"errors"
	"io"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/app"
	"github.com/danielhanold/docket/internal/buildinfo"
)

// Run wires arguments and explicit streams through Cobra to the application
// and presents exactly one outcome. It returns the process exit code; only
// cmd/docket/main.go converts it into os.Exit.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer, info buildinfo.Info, facts buildinfo.RuntimeFacts) int {
	jsonMode := DetectJSONMode(args)
	p := Presenter{Stdout: stdout, Stderr: stderr, JSON: jsonMode}

	var result app.OperationResult
	helpConflict := false

	root := &cobra.Command{
		Use:   "docket",
		Short: "docket tracks planned work as changes and records decisions as ADRs",
		// Docket owns error presentation: Cobra must not print errors or
		// usage itself, or the one-document stdout contract breaks.
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("missing command")
		},
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.PersistentFlags().Bool("json", false, "emit protocol-v1 JSON on stdout")
	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(args)

	// JSON mode and help are mutually exclusive: any help path in JSON mode
	// records a conflict instead of writing help into the protocol stream.
	defaultHelp := root.HelpFunc()
	root.SetHelpFunc(func(c *cobra.Command, a []string) {
		if jsonMode {
			helpConflict = true
			return
		}
		defaultHelp(c, a)
	})

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Report this binary's build identity",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			result = app.Version(info)
			return nil
		},
	}

	diagnosticCmd := &cobra.Command{
		Use:   "diagnostic",
		Short: "Read-only diagnostics",
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("missing command")
		},
	}
	runtimeCmd := &cobra.Command{
		Use:   "runtime",
		Short: "Report the running Go toolchain and target tuple",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			result = app.DiagnosticRuntime(facts)
			return nil
		},
	}
	diagnosticCmd.AddCommand(runtimeCmd)
	root.AddCommand(versionCmd, diagnosticCmd)

	err := root.Execute()
	switch {
	case helpConflict:
		return p.Present(app.CLIError(app.ReasonJSONHelpConflict,
			"--json cannot be combined with --help, -h, or the help command"))
	case err != nil:
		res := app.CLIError(app.ReasonInvalidArguments, err.Error())
		if jsonMode {
			return p.Present(res)
		}
		return p.PresentHumanError(res)
	case result != nil:
		return p.Present(result)
	default:
		// Human help was rendered by Cobra on stdout; exit 0.
		return 0
	}
}
