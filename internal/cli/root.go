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
	prescan := DetectJSONMode(args)

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

	// jsonMode reports the selected output transport. pflag parses the bound
	// --json flag with strconv.ParseBool, so it accepts spellings the pre-scan's
	// deliberately bounded grammar does not — --json=1, --json=TRUE, --json=t.
	// The Cobra-bound value is therefore authoritative whenever pflag actually
	// parsed the flag (Changed); the pre-scan stands in only when parsing never
	// reached it, as in `docket version --bogus --json`, where Cobra stops at
	// the unknown token, or `docket --json=1 bogus`, where command resolution
	// fails before any flag is parsed. In those fallback cases only the
	// pre-scan's three spellings can still select JSON mode — the documented
	// boundary of a transport scan that deliberately is not a second parser.
	// Reading the flag rather than the pre-scan is what keeps
	// `docket --json=1 version` from parsing cleanly and then emitting human
	// text at exit 0, which no machine consumer could detect.
	//
	// Ordering: pflag parses flags before Cobra dispatches to the help func or
	// the help command, so both read the same value the final presentation does.
	// That keeps the spec's conflict rule — "--json cannot be combined with
	// --help, -h, or the help command" — keyed on the mode actually selected:
	// `--json=1 --help` conflicts, and `--json=0 --help` renders human help.
	jsonMode := func() bool {
		f := root.PersistentFlags().Lookup("json")
		if f == nil || !f.Changed {
			return prescan
		}
		return f.Value.String() == "true"
	}

	// JSON mode and help are mutually exclusive: any help path in JSON mode
	// records a conflict instead of writing help into the protocol stream.
	defaultHelp := root.HelpFunc()
	root.SetHelpFunc(func(c *cobra.Command, a []string) {
		if jsonMode() {
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
			if jsonMode() {
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
		// A command group resolves its subcommand before Args runs, so
		// anything reaching here is a word that named no subcommand.
		// Without this, Cobra's legacy default accepts arbitrary args and
		// RunE reports "missing command" for `diagnostic runtimee` — which
		// misdirects, since a command word WAS supplied. NoArgs names the
		// offending token instead; the bare `docket diagnostic` still falls
		// through to RunE's "missing command".
		Args: cobra.NoArgs,
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

	// The hidden completion commands are rejected before Cobra ever sees the
	// arguments; everything else routes through Execute as usual.
	err := rejectHiddenCompletionCommand(args)
	if err == nil {
		err = root.Execute()
	}

	// Nothing has been presented yet, so the presenter is built after Execute
	// with the mode Cobra ended up parsing.
	p := Presenter{Stdout: stdout, Stderr: stderr, JSON: jsonMode()}

	switch {
	case helpConflict:
		return p.Present(app.CLIError(app.ReasonJSONHelpConflict,
			"--json cannot be combined with --help, -h, or the help command"))
	case err != nil:
		res := app.CLIError(app.ReasonInvalidArguments, err.Error())
		if p.JSON {
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

// rejectHiddenCompletionCommand refuses Cobra's hidden shell-completion
// commands as ordinary unknown commands, returning nil for every other
// invocation.
//
// Cobra registers __complete and __completeNoDesc in initCompleteCmd, which
// runs before it consults CompletionOptions.DisableDefaultCmd — so disabling
// the visible `completion` command hides the built-in generator but leaves
// both hidden spellings executable. They write completion candidates to stdout
// and "Completion ended with directive:" to stderr on their own, bypassing the
// presenter and breaking the one-document, empty-stderr contract. Docket does
// not ship shell completion, so the only correct answer is the unknown-command
// error Cobra would give any other unrecognized word.
func rejectHiddenCompletionCommand(args []string) error {
	for _, a := range args {
		// A standalone -- terminates command resolution in Cobra's own
		// stripFlags, so nothing after it can name a command.
		if a == "--" {
			return nil
		}
		// Docket declares no flag that consumes a following value, so the
		// first argument that is not flag-shaped is the command word.
		if strings.HasPrefix(a, "-") && a != "-" {
			continue
		}
		if a == "__complete" || a == "__completeNoDesc" {
			return fmt.Errorf("unknown command %q for %q", a, "docket")
		}
		return nil
	}
	return nil
}
