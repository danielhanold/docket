package cli

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/app"
)

// exportRunner runs the export operation; tests replace it to observe the
// options the command parsed without reading, writing, or launching anything.
// The command shares open's dependency wiring, openDepsFor.
var exportRunner = func(ctx context.Context, d app.OpenDeps, o app.ExportOptions) app.OperationResult {
	return app.Export(ctx, d, o)
}

// newExportCommand builds `docket export <type> [<id>]`: a thin adapter that
// parses the type and change id, then hands them to app.Export. setResult
// receives the operation's result document for the root to present.
func newExportCommand(setResult func(app.OperationResult)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export <type> [<id>]",
		Short: "Write a clean Markdown copy of a change, spec, plan, or results file",
		Long: `Without an id, exports the change whose recorded branch is checked out. It
always reads the local metadata checkout. The copy drops the frontmatter,
docket's generated blocks and markers, every link (keeping its text), and
docket-only sections; explicit change references become quoted titles. Fenced
code and inline code stay exactly as written.

The copy is named <date>-<kind>-<name>.md, kind one of ticket, spec, plan, or
results. By default it lands in ~/Downloads on macOS or the home directory on
Linux and opens. --output-path names a directory to write into or the exact
file to write, and opens nothing; --print opens nothing wherever the copy
goes. An existing file is replaced. Leftover docket machinery and mentions of
docket are reported as warnings on stderr; --warnings picks which. Warnings
never block.`,
		Args: func(c *cobra.Command, args []string) error {
			what, id, err := app.ParseExportArgs(args)
			if err != nil {
				return err
			}
			_, err = exportOptionsFromFlags(c, what, id)
			return err
		},
		Annotations: capability("export", EffectRead, EffectLocalWrite, EffectProcessControl),
		RunE: func(c *cobra.Command, args []string) error {
			what, id, err := app.ParseExportArgs(args)
			if err != nil {
				return err
			}
			repoDir, err := resolveRepoDir(c)
			if err != nil {
				return err
			}
			o, err := exportOptionsFromFlags(c, what, id)
			if err != nil {
				return err
			}
			o.RepoDir = repoDir
			deps, err := openDepsFor()
			if err != nil {
				return err
			}
			setResult(exportRunner(c.Context(), deps, o))
			return nil
		},
	}
	cmd.Flags().String("output-path", "", "directory or exact file `path` for the copy, which is then not opened (default: ~/Downloads on macOS, the home directory on Linux)")
	cmd.Flags().String("warnings", app.WarningsAll, "`which` warnings to report: all, leftovers, or mentions")
	cmd.Flags().Bool("print", false, "print the copy's path without opening it")
	cmd.Flags().String("repo-dir", "", "repository `dir` to operate on (default: current directory)")
	return cmd
}

// exportOptionsFromFlags builds the export options from the parsed type and the
// command's flags. Warnings is set only when --warnings was given, so the
// operation sees an explicit value and never the flag's default; an explicitly
// empty --output-path and an unknown --warnings value are refused.
func exportOptionsFromFlags(c *cobra.Command, what string, id int) (app.ExportOptions, error) {
	f := c.Flags()
	o := app.ExportOptions{What: what, ID: id}
	o.Print, _ = f.GetBool("print")
	o.OutputPath, _ = f.GetString("output-path")
	if f.Changed("output-path") && o.OutputPath == "" {
		return o, errors.New("--output-path needs a path")
	}
	if f.Changed("warnings") {
		o.Warnings, _ = f.GetString("warnings")
	}
	return o, app.ValidateExportWarnings(o.Warnings)
}
