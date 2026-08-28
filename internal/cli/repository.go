package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/app"
	"github.com/danielhanold/docket/internal/gitcli"
)

// This file is the `docket repository` command family: thin adapters that
// resolve the invocation directory, construct the shared Git client seam, and
// dispatch to the matching internal/app service, letting the presenter own the
// outcome. Every policy decision — classification, refusal, effect sequencing —
// belongs to internal/app, so no body here branches on repository state. The one
// exception the design requires is `migrate`'s two-pass confirm flow, which lives
// here because it is a terminal interaction: the service returns the plan, and the
// CLI prints it and re-invokes with an explicit authorization keyed on exactly the
// pinned revision the human saw (learning decide-and-act-on-the-same-copy).

// repositoryInitRunner, repositoryCheckRunner, and repositoryMigrateRunner are
// the app entry points the repository subcommands dispatch to. They are package
// variables so a test can stub them without a real repository.
var (
	repositoryInitRunner = func(ctx context.Context, d app.SetupDeps) app.OperationResult {
		return app.RunRepositoryInit(ctx, d)
	}
	repositoryCheckRunner = func(ctx context.Context, d app.SetupDeps) app.OperationResult {
		return app.RunRepositoryCheck(ctx, d)
	}
	repositoryMigrateRunner = func(ctx context.Context, d app.SetupDeps, o app.MigrateOptions) app.OperationResult {
		return app.RunRepositoryMigrate(ctx, d, o)
	}
)

// repositoryConfirmInteractive reports whether migrate may prompt for
// confirmation on a terminal. It is a seam so a test can force the interactive
// branch without a real TTY; production reads whether stdin is a character
// device.
var repositoryConfirmInteractive = func() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

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
	migrateCmd := newRepositoryMigrateCommand(setResult)

	repositoryCmd.AddCommand(initCmd, checkCmd, migrateCmd)
	return repositoryCmd
}

// newRepositoryMigrateCommand builds `docket repository migrate` with its
// --yes/--repair-frontmatter flags and the two-pass confirm flow. --yes performs
// the authorized migration directly; without it the service is called for a
// preview, which is either presented as a confirmation-required plan
// (non-interactive) or printed and confirmed on a terminal, then re-invoked with
// an explicit authorization pinned to exactly the revision the preview showed.
func newRepositoryMigrateCommand(setResult func(app.OperationResult)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Convert a legacy single-branch repository to the docket topology",
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
			deps := app.SetupDeps{Git: client, RepoDir: repoDir}
			yes, _ := c.Flags().GetBool("yes")
			repair, _ := c.Flags().GetBool("repair-frontmatter")

			if yes {
				setResult(repositoryMigrateRunner(c.Context(), deps, app.MigrateOptions{Authorized: true, RepairAuthorized: repair}))
				return nil
			}

			preview := repositoryMigrateRunner(c.Context(), deps, app.MigrateOptions{RepairAuthorized: repair})
			jsonMode, _ := c.Flags().GetBool("json")
			if jsonMode || !repositoryConfirmInteractive() {
				setResult(preview)
				return nil
			}

			// Interactive: print the plan, prompt, and on yes re-invoke authorized,
			// pinned to the exact revision the preview showed.
			fmt.Fprintln(c.OutOrStdout(), preview.HumanText())
			fmt.Fprint(c.OutOrStdout(), "migrate? [y/N] ")
			if !repositoryReadYes(c.InOrStdin()) {
				setResult(preview)
				return nil
			}
			expected := ""
			if p, ok := preview.(interface{ SourceRev() string }); ok {
				expected = p.SourceRev()
			}
			setResult(repositoryMigrateRunner(c.Context(), deps, app.MigrateOptions{Authorized: true, RepairAuthorized: repair, ExpectedSource: expected}))
			return nil
		},
	}
	cmd.Flags().String("repo-dir", "", "repository directory to operate on (default: current directory)")
	cmd.Flags().Bool("yes", false, "authorize the migration without an interactive confirmation")
	cmd.Flags().Bool("repair-frontmatter", false, "authorize the mechanical frontmatter repairs the plan lists")
	return cmd
}

// repositoryReadYes reads one line from r and reports whether it affirms the
// prompt (an empty or absent line is No — the prompt is [y/N]).
func repositoryReadYes(r io.Reader) bool {
	sc := bufio.NewScanner(r)
	if !sc.Scan() {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(sc.Text())) {
	case "y", "yes":
		return true
	default:
		return false
	}
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
