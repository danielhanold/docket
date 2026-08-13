package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

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
	helpRendered := false

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
		helpRendered = true
	})

	// Docket owns the help command too. Cobra's built-in one answers an
	// unresolvable topic with "Unknown help topic" plus usage on stdout and
	// never calls the help func, which would leak non-protocol bytes into the
	// JSON stream and exit 0. Routing every topic — resolvable or not —
	// through this RunE keeps one policy: conflict in JSON mode, Cobra text
	// for a topic that resolves, invalid input for one that does not.
	root.SetHelpCommand(&cobra.Command{
		Use:   "help [command]",
		Short: "Help about any command",
		RunE: func(c *cobra.Command, a []string) error {
			if jsonMode {
				helpConflict = true
				return nil
			}
			target, _, err := c.Root().Find(a)
			if target == nil || err != nil {
				return fmt.Errorf("unknown help topic %q", strings.Join(a, " "))
			}
			target.InitDefaultHelpFlag()
			target.InitDefaultVersionFlag()
			return target.Help()
		},
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
	case helpRendered:
		// Human help was rendered by Cobra on stdout; exit 0.
		return 0
	default:
		// Unreachable by construction: every command either sets result or
		// returns an error, and every help path sets helpConflict or
		// helpRendered. Reaching here means nothing was rendered, so exit 0
		// would report success for an empty run. Diagnose on stderr only —
		// stdout must stay empty rather than carry a half-formed document.
		fmt.Fprintln(stderr, "docket: internal error: command produced no result")
		return app.ExitCode(app.ResultInternalError)
	}
}
