package cli

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/app"
	"github.com/danielhanold/docket/internal/gitcli"
)

// This file is the `docket repository` command family: thin adapters that
// resolve the invocation directory, construct the shared Git client seam, and
// dispatch to the matching internal/app service, letting the presenter own the
// outcome. Every policy decision — classification, refusal, effect sequencing —
// belongs to internal/app, so no body here branches on repository state. The
// `init` and `check` services are wired here; `migrate` is wired by a later task
// and until then reports the operation as not yet available.

// repositoryInitRunner, repositoryCheckRunner, and repositoryMigrateRunner are
// the app entry points the repository subcommands dispatch to. They are package
// variables so a test can stub them without a real repository; production points
// them at the real services (check and migrate arrive in later tasks).
var (
	repositoryInitRunner = func(ctx context.Context, d app.SetupDeps) app.OperationResult {
		return app.RunRepositoryInit(ctx, d)
	}
	repositoryCheckRunner = func(ctx context.Context, d app.SetupDeps) app.OperationResult {
		return app.RunRepositoryCheck(ctx, d)
	}
	repositoryMigrateRunner = func(ctx context.Context, d app.SetupDeps) app.OperationResult {
		return repositoryNotAvailable("repository.migrate")
	}
)

// newRepositoryCommand builds the `repository` command group. setResult is the
// closure that hands a computed operation result back to Run's single
// presentation point, mirroring every other command family.
func newRepositoryCommand(setResult func(app.OperationResult)) *cobra.Command {
	repositoryCmd := &cobra.Command{
		Use:   "repository",
		Short: "Initialize, migrate, and check the docket repository topology",
		// A command group resolves its subcommand before Args runs, so anything
		// reaching here named no subcommand; NoArgs names an offending token and
		// the bare `docket repository` falls through to RunE's missing-command
		// error.
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("missing command")
		},
	}

	initCmd := repositorySubcommand("init",
		"Initialize the docket metadata branch and persistent .docket worktree",
		func(c *cobra.Command, deps app.SetupDeps) {
			setResult(repositoryInitRunner(c.Context(), deps))
		})
	checkCmd := repositorySubcommand("check",
		"Report repository health with machine-readable findings (read-only)",
		func(c *cobra.Command, deps app.SetupDeps) {
			setResult(repositoryCheckRunner(c.Context(), deps))
		})
	migrateCmd := repositorySubcommand("migrate",
		"Convert a legacy single-branch repository to the docket topology",
		func(c *cobra.Command, deps app.SetupDeps) {
			setResult(repositoryMigrateRunner(c.Context(), deps))
		})

	repositoryCmd.AddCommand(initCmd, checkCmd, migrateCmd)
	return repositoryCmd
}

// repositorySubcommand builds one repository subcommand: it resolves --repo-dir,
// constructs the Git client, assembles the SetupDeps seam, and calls run with
// them. Any pre-dispatch failure (an unresolvable working directory, an
// unavailable Git client) is an argument-shaped error returned before the
// operation runs.
func repositorySubcommand(name, short string, run func(c *cobra.Command, deps app.SetupDeps)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, err := resolveRepoDir(c)
			if err != nil {
				return err
			}
			client, err := gitcli.NewClient()
			if err != nil {
				return err
			}
			run(c, app.SetupDeps{Git: client, RepoDir: repoDir})
			return nil
		},
	}
	cmd.Flags().String("repo-dir", "", "repository directory to operate on (default: current directory)")
	return cmd
}

// repositoryNotAvailable is the placeholder result the check and migrate
// subcommands return until their services are wired: an unsupported-operation
// argument error naming the operation.
func repositoryNotAvailable(operation string) app.OperationResult {
	return app.CLIError(app.ReasonInvalidArguments, operation+" is not yet available in this build")
}
