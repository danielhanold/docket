package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/app"
	"github.com/danielhanold/docket/internal/buildinfo"
	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/install"
)

// Run wires arguments and explicit streams through Cobra to the application
// and presents exactly one outcome. It returns the process exit code; only
// cmd/docket/main.go converts it into os.Exit.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer, info buildinfo.Info, facts buildinfo.RuntimeFacts) int {
	return run(args, stdin, stdout, stderr, info, facts)
}

// run is Run plus the seam that lets a test register commands of its own. The
// asset-dependence guard is a property of the TREE, so the only honest way to
// prove it refuses a gated command is to hand the production wiring one —
// docket ships none yet.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer, info buildinfo.Info, facts buildinfo.RuntimeFacts, extra ...*cobra.Command) int {
	prescan := DetectJSONMode(args)

	var result app.OperationResult
	helpConflict := false
	helpRendered := false
	// gateOperation names the command the asset-dependence guard refused, so
	// the refusal document reports the operation the user asked for rather
	// than the guard.
	gateOperation := ""

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
	// The configuration subcommand is a thin adapter: it reads its three flags,
	// hands the filesystem layers to the operation, and lets the presenter own
	// the outcome. Every policy question — which mode, which result, which
	// exit code — belongs to app.DiagnosticConfig, so this body has no branch
	// on configuration content at all.
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect resolved configuration and its capability envelope",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, _ := c.Flags().GetString("repo-dir")
			defBranch, _ := c.Flags().GetString("default-branch")
			forMutation, _ := c.Flags().GetBool("for-mutation")
			sources, err := config.LoadFilesystemSources(config.FSOptions{RepoDir: repoDir})
			if err != nil {
				// An unreadable file or an unusable --repo-dir is an argument
				// problem, not a configuration verdict: it takes the same
				// invalid-arguments path as any other bad flag value, and
				// never produces a half-formed inspection document.
				return err
			}
			result = app.DiagnosticConfig(sources, config.ResolveContext{DefaultBranch: defBranch}, forMutation)
			return nil
		},
	}
	// The status command is a thin adapter, mirroring configCmd: it reads its
	// flags, hands them to the operation over the Git-backed reader, and lets the
	// presenter own the outcome. Flag-value validation — bad priority spellings,
	// change types outside the resolved change_types set — belongs to app.Status,
	// which alone knows the closed sets, so this body branches on nothing.
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Report backlog status, readiness, selection, and repository health (read-only)",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, _ := c.Flags().GetString("repo-dir")
			types, _ := c.Flags().GetStringArray("type")
			priorities, _ := c.Flags().GetStringArray("priority")
			client, err := gitcli.NewClient()
			if err != nil {
				return err
			}
			result = app.Status(c.Context(), app.NewGitStatusReader(client),
				app.StatusOptions{RepoDir: repoDir, Types: types, Priorities: priorities})
			return nil
		},
	}
	statusCmd.Flags().String("repo-dir", "", "repository directory to read (default: current directory)")
	statusCmd.Flags().StringArray("type", nil, "filter the displayed projection to a configured change type (repeatable)")
	statusCmd.Flags().StringArray("priority", nil, "filter the displayed projection to a priority: critical, high, medium, or low (repeatable)")

	configCmd.Flags().String("repo-dir", "", "repository directory to inspect (required; used verbatim, no Git discovery)")
	configCmd.Flags().String("default-branch", "", "default branch supplied to integration_branch: auto")
	configCmd.Flags().Bool("for-mutation", false, "run the mutation preflight (operation config.preflight)")
	_ = configCmd.MarkFlagRequired("repo-dir")

	// The three installation commands are thin adapters, like the ones above:
	// they read their flags, assemble the operation's inputs, and let the
	// presenter own the outcome. Every classification decision belongs to
	// internal/app, so no body here branches on what the installer found.
	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Install docket's skills, agents, and dispatch material into your harnesses",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			harnesses, _ := c.Flags().GetStringArray("harness")
			opts, refusal := installOptions(harnesses, info)
			if refusal != nil {
				result = refusal.result(app.OperationInstall)
				return nil
			}
			result = app.RunInstall(opts)
			return nil
		},
	}
	installCmd.Flags().StringArray("harness", nil,
		"harness to install into: claude, codex, cursor, or opencode (repeatable; default: detect)")

	installCheckCmd := &cobra.Command{
		Use:   "check",
		Short: "Report whether this machine's installation is current (writes nothing)",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			opts, refusal := installOptions(nil, info)
			if refusal != nil {
				result = refusal.result(app.OperationInstallCheck)
				return nil
			}
			result = app.RunInstallCheck(opts)
			return nil
		},
	}

	developmentCmd := &cobra.Command{
		Use:   "development",
		Short: "Contributor operations against a docket checkout",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("missing command")
		},
	}
	developmentInstallCmd := &cobra.Command{
		Use:   "install",
		Short: "Install from a checkout, linking harnesses at the source tree",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			harnesses, _ := c.Flags().GetStringArray("harness")
			source, _ := c.Flags().GetString("source")
			binDir, _ := c.Flags().GetString("bin-dir")
			opts, refusal := installOptions(harnesses, info)
			if refusal != nil {
				result = refusal.result(app.OperationDevelopmentInstall)
				return nil
			}
			result = app.RunDevelopmentInstall(install.DevOptions{
				Options:    opts,
				SourceRoot: source,
				BinDir:     binDir,
				GoRunner:   install.DefaultGoRunner,
			})
			return nil
		},
	}
	developmentInstallCmd.Flags().String("source", "", "docket checkout to install from (required)")
	developmentInstallCmd.Flags().String("bin-dir", "", "directory the built binary is installed into (default: XDG_BIN_HOME or ~/.local/bin)")
	developmentInstallCmd.Flags().StringArray("harness", nil,
		"harness to install into: claude, codex, cursor, or opencode (repeatable; default: detect)")
	_ = developmentInstallCmd.MarkFlagRequired("source")

	// The change command family is a thin adapter tree, like the commands above:
	// each subcommand reads a JSON request, hands it to its internal/app planning
	// operation, and assigns the outcome to the shared result for the presenter.
	changeCmd := newChangeCommand(func(r app.OperationResult) { result = r })
	learningCmd := newLearningCommand(func(r app.OperationResult) { result = r })
	adrCmd := newADRCommand(func(r app.OperationResult) { result = r })

	installCmd.AddCommand(installCheckCmd)
	developmentCmd.AddCommand(developmentInstallCmd)
	diagnosticCmd.AddCommand(runtimeCmd, configCmd)
	root.AddCommand(versionCmd, statusCmd, changeCmd, learningCmd, adrCmd, diagnosticCmd, installCmd, developmentCmd)
	root.AddCommand(extra...)

	// The asset-dependence guard. Everything docket ships today is registered
	// as asset-independent, which is the point: a command that reads installed
	// assets must be added to the set deliberately or be refused, and a
	// forgotten command fails closed rather than reading a version tree that
	// may not exist or may speak a protocol this binary does not.
	root.PersistentPreRunE = func(c *cobra.Command, _ []string) error {
		key := commandKey(c)
		if assetIndependent[key] {
			return nil
		}
		gateOperation = operationName(key)
		roots, err := install.ResolveRoots(os.UserHomeDir, os.Getenv)
		if err != nil {
			return &InstallRefusal{Reason: install.ReasonInvalidOptions, Err: err}
		}
		return RequireCompatibleInstallation(roots)
	}

	// The hidden completion commands are rejected before Cobra ever sees the
	// arguments; everything else routes through Execute as usual.
	err := rejectHiddenCompletionCommand(args)
	if err == nil {
		err = root.Execute()
	}

	// Nothing has been presented yet, so the presenter is built after Execute
	// with the mode Cobra ended up parsing.
	p := Presenter{Stdout: stdout, Stderr: stderr, JSON: jsonMode()}

	var refusal *InstallRefusal
	switch {
	case helpConflict:
		return p.Present(app.CLIError(app.ReasonJSONHelpConflict,
			"--json cannot be combined with --help, -h, or the help command"))
	case errors.As(err, &refusal):
		// A guard refusal is a verdict about this machine, not a malformed
		// invocation: it presents as the operation's own document so a
		// consumer reads the same reason vocabulary it would from `install
		// check`, rather than an argument error.
		return p.Present(refusal.result(gateOperation))
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
		// No ROOT PERSISTENT flag consumes a following value, so the first
		// argument that is not flag-shaped is the command word. Subcommand
		// flags that do take a value (--repo-dir, --default-branch) can only
		// appear AFTER that command word, so this scan never reaches them.
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
