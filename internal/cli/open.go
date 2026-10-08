package cli

import (
	"context"
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
			Opener: app.NewSystemOpener(runtime.GOOS, exec.LookPath, app.RunOpenerProcess),
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
the metadata checkout's file; pr opens the pull request.`,
		Args: func(_ *cobra.Command, args []string) error {
			_, _, err := app.ParseOpenArgs(args)
			return err
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
			printOnly, _ := c.Flags().GetBool("print")
			deps, err := openDepsFor()
			if err != nil {
				return err
			}
			setResult(openRunner(c.Context(), deps, app.OpenOptions{RepoDir: repoDir, What: what, ID: id, Print: printOnly}))
			return nil
		},
	}
	cmd.Flags().Bool("print", false, "print the URL or path without opening it")
	cmd.Flags().String("repo-dir", "", "repository `dir` to operate on (default: current directory)")
	return cmd
}
