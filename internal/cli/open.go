package cli

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/app"
	"github.com/danielhanold/docket/internal/gitcli"
)

var (
	// openRunner runs the open operation; tests replace it to observe the
	// options the command parsed without resolving or launching anything.
	openRunner = func(ctx context.Context, d app.OpenDeps, o app.OpenOptions) app.OperationResult {
		return app.Open(ctx, d, o)
	}
	// openDepsFor wires the open operation to real git, the repository
	// prepare operation, and the platform's system opener. Tests replace it.
	openDepsFor = func() (app.OpenDeps, error) {
		client, err := gitcli.NewClient()
		if err != nil {
			return app.OpenDeps{}, err
		}
		return app.OpenDeps{
			Reader:   app.NewGitStatusReader(client),
			Checkout: client.WorktreeCheckoutState,
			Prepare: func(ctx context.Context, dir string) app.RepositoryPrepareResult {
				return app.RunRepositoryPrepare(ctx, app.SetupDeps{Git: client, RepoDir: dir}, app.PrepareOptions{})
			},
			Opener:  app.NewSystemOpener(runtime.GOOS, exec.LookPath, app.RunOpenerProcess),
			HomeDir: os.UserHomeDir,
			Getwd:   os.Getwd,
			GOOS:    runtime.GOOS,
		}, nil
	}
)

// newOpenCommand builds `docket open <what> [<id>]`: a thin adapter that parses
// the target and change id, then hands them to app.Open. setResult receives
// the operation's result document for the root to present.
func newOpenCommand(setResult func(app.OperationResult)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "open <what> [<id>]",
		Short: "Open the board, or a change's record, spec, plan, results, or pull request",
		Long: `Without an id, opens the change whose recorded branch is checked out. Shared
repositories open GitHub pages unless open.artifacts is local; private ones open
the metadata checkout's file; pr opens the pull request.

--export writes a clean Markdown copy of a change, spec, plan, or results file
and opens that copy (--print prints its path). It always reads the local
metadata checkout. The copy drops the frontmatter, docket's generated blocks
and markers, every link (keeping its text), and docket-only sections; explicit
change references become quoted titles. Fenced code and inline code stay
exactly as written.

The copy is named <date>-<type>-<name>.md, type one of issue, spec, plan, or
results, and lands in ~/Downloads on macOS or the home directory on Linux.
--output-path names a directory to write into or the exact file to write; an
existing file is replaced. Leftover docket machinery and mentions of docket are
reported as warnings on stderr; --warnings picks which. Warnings never block.`,
		Args: func(c *cobra.Command, args []string) error {
			what, id, err := app.ParseOpenArgs(args)
			if err != nil {
				return err
			}
			o, err := openOptionsFromFlags(c, what, id)
			if err != nil {
				return err
			}
			return app.ValidateOpenExport(o)
		},
		Annotations: capability("open", EffectRead, EffectLocalWrite, EffectProcessControl),
		RunE: func(c *cobra.Command, args []string) error {
			what, id, err := app.ParseOpenArgs(args)
			if err != nil {
				return err
			}
			repoDir, err := resolveRepoDir(c)
			if err != nil {
				return err
			}
			o, err := openOptionsFromFlags(c, what, id)
			if err != nil {
				return err
			}
			o.RepoDir = repoDir
			deps, err := openDepsFor()
			if err != nil {
				return err
			}
			setResult(openRunner(c.Context(), deps, o))
			return nil
		},
	}
	cmd.Flags().Bool("export", false, "write a clean Markdown copy of the artifact and open that copy")
	cmd.Flags().String("output-path", "", "directory or exact file `path` for the --export copy (default: ~/Downloads on macOS, the home directory on Linux)")
	cmd.Flags().String("warnings", app.WarningsAll, "`which` --export warnings to report: all, leftovers, or mentions")
	cmd.Flags().Bool("print", false, "print the URL or path without opening it")
	cmd.Flags().String("repo-dir", "", "repository `dir` to operate on (default: current directory)")
	return cmd
}

// openOptionsFromFlags builds the open options from the parsed target and the
// command's flags. Warnings is set only when --warnings was given, so the
// export check sees an explicit value and never the flag's default; an
// explicitly empty --output-path is refused.
func openOptionsFromFlags(c *cobra.Command, what string, id int) (app.OpenOptions, error) {
	f := c.Flags()
	o := app.OpenOptions{What: what, ID: id}
	o.Print, _ = f.GetBool("print")
	o.Export, _ = f.GetBool("export")
	o.OutputPath, _ = f.GetString("output-path")
	if f.Changed("output-path") && o.OutputPath == "" {
		return o, errors.New("--output-path needs a path")
	}
	if f.Changed("warnings") {
		o.Warnings, _ = f.GetString("warnings")
	}
	return o, nil
}
