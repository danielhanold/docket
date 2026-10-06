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
	"github.com/danielhanold/docket/internal/githubcli"
)

// This file is the `docket repository` command family: thin adapters that
// resolve the invocation directory, construct the shared Git client seam, and
// dispatch to the matching internal/app service, letting the presenter own the
// outcome. Every policy decision — classification, refusal, effect sequencing —
// belongs to internal/app, so no body here branches on repository state. The one
// exception the design requires is `migrate`'s and `repair`'s two-pass confirm
// flow, which lives here because it is a terminal interaction: the service
// returns the plan, and the CLI prints it and re-invokes with an explicit
// authorization keyed on exactly the pinned revision the human saw (learning
// decide-and-act-on-the-same-copy).

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
	repositoryRepairRunner = func(ctx context.Context, d app.SetupDeps, o app.RepairOptions) app.OperationResult {
		return app.RunRepositoryRepair(ctx, d, o)
	}
	// repositoryRepairGitHub constructs the GitHub client `repository repair
	// --pr-backlinks` reads and edits PR bodies through. It is called only when
	// that flag is set; tests replace it.
	repositoryRepairGitHub = func() (app.RepairGitHub, error) {
		gh, err := githubcli.NewClient()
		if err != nil {
			return nil, err
		}
		return gh, nil
	}
	repositoryPrepareRunner = func(ctx context.Context, d app.SetupDeps, o app.PrepareOptions) app.OperationResult {
		return app.RunRepositoryPrepare(ctx, d, o)
	}
	repositoryConfigureTestsRunner = func(ctx context.Context, d app.SetupDeps, o app.ConfigureTestsOptions) app.OperationResult {
		return app.RunRepositoryConfigureTests(ctx, d, o)
	}
	repositorySyncIntegrationRunner = func(ctx context.Context, d app.SetupDeps) app.OperationResult {
		return app.RunRepositorySyncIntegration(ctx, d)
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
		Short: "Initialize, migrate, check, and repair the docket repository topology",
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
		},
		// metadata-write (the parentless metadata root, published create-only to
		// the metadata branch) + local-write (local branch, .docket worktree,
		// managed .gitignore, dispatch surfaces). The metadata-branch publish is
		// metadata-write, not external — external is only remote refs OUTSIDE the
		// metadata branch.
		EffectMetadataWrite, EffectLocalWrite)
	checkCmd := repositorySubcommand("check",
		"Report repository health with machine-readable findings (read-only)",
		func(c *cobra.Command, deps app.SetupDeps) {
			setResult(repositoryCheckRunner(c.Context(), deps))
		},
		EffectRead)
	migrateCmd := newRepositoryMigrateCommand(setResult)
	repairCmd := newRepositoryRepairCommand(setResult)
	prepareCmd := repositorySubcommand("prepare",
		"Prepare the repository for a workflow: pin topology and attach or fast-forward the .docket worktree (the startup check)",
		func(c *cobra.Command, deps app.SetupDeps) {
			// deps.RepoDir already carries the resolved --repo-dir; RunRepository
			// prepare keeps its own PrepareOptions.RepoDir override empty here.
			setResult(repositoryPrepareRunner(c.Context(), deps, app.PrepareOptions{}))
		},
		// local-write: attaches or fast-forwards the local .docket worktree and
		// its local metadata ref; it authors no new metadata content and never
		// pushes.
		EffectLocalWrite)
	configureTestsCmd := repositorySubcommand("configure-tests",
		"Generate the pending .docket.yml build/finalize test policy for an already-initialized repository",
		func(c *cobra.Command, deps app.SetupDeps) {
			// Changed, not the value: an explicit empty --command must reach the
			// app (which refuses it), never read as "run discovery".
			var o app.ConfigureTestsOptions
			if c.Flags().Changed("command") {
				v, _ := c.Flags().GetString("command")
				o.Command = &v
			}
			setResult(repositoryConfigureTestsRunner(c.Context(), deps, o))
		},
		// local-write: (re)generates the pending, unstaged .docket.yml edit —
		// never commits, never stages.
		EffectLocalWrite)
	configureTestsCmd.Flags().String("command", "",
		"suite `command` to set as both build.test_command and finalize.test_command with both gates local (skips discovery)")

	syncIntegrationCmd := repositorySubcommand("sync-integration",
		"Fast-forward the primary checkout to the freshly fetched integration tip when it is safe (explicit skips otherwise)",
		func(c *cobra.Command, deps app.SetupDeps) {
			setResult(repositorySyncIntegrationRunner(c.Context(), deps))
		},
		// local-write: fetches objects/remote-tracking state and may fast-forward
		// the primary checkout; it never pushes and changes no planning metadata.
		EffectLocalWrite)

	repositoryCmd.AddCommand(initCmd, checkCmd, migrateCmd, repairCmd, prepareCmd, configureTestsCmd, syncIntegrationCmd)
	return repositoryCmd
}

// newRepositoryMigrateCommand builds `docket repository migrate` with its
// --yes/--repair-frontmatter flags and the two-pass confirm flow. --yes performs
// the authorized migration directly; without it the service is called for a
// preview, which is either presented as a confirmation-required plan
// (non-interactive) or printed and confirmed on a terminal, then re-invoked with
// an explicit authorization pinned to exactly the revision the preview showed.
// On an already-migrated repository the service is a no-op naming
// `docket repository repair`.
func newRepositoryMigrateCommand(setResult func(app.OperationResult)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Convert a legacy single-branch repository to the docket topology",
		Args:  cobra.NoArgs,
		// metadata-write (seed + prune commits published to the metadata branch)
		// + local-write (local finish and .docket attachment). The metadata-
		// branch publish is metadata-write, not external.
		Annotations: capability("repository.migrate", EffectMetadataWrite, EffectLocalWrite),
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
			// pinned to the exact revision the preview showed. The confirmed run
			// authorizes repairs unconditionally (RepairAuthorized: true): the human
			// confirmed the whole previewed plan, and the preview carried the complete
			// repair diff, so --repair-frontmatter is not additionally required here.
			// This differs from the non-interactive path above, where the service still
			// requires --repair-frontmatter when repairs are present.
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
			setResult(repositoryMigrateRunner(c.Context(), deps, app.MigrateOptions{Authorized: true, RepairAuthorized: true, ExpectedSource: expected}))
			return nil
		},
	}
	cmd.Flags().String("repo-dir", "", "repository `dir` to operate on (default: current directory)")
	cmd.Flags().Bool("yes", false, "authorize the migration without an interactive confirmation")
	cmd.Flags().Bool("repair-frontmatter", false, "authorize the mechanical frontmatter repairs a legacy migration's plan lists")
	return cmd
}

// newRepositoryRepairCommand builds `docket repository repair` with its --yes
// flag and the two-pass confirm flow. --yes performs the authorized repair
// directly; without it the service returns a preview, presented as-is
// non-interactively, or — on a terminal, and only when the preview is a
// confirmation request — printed and confirmed, then re-invoked authorized and
// pinned to exactly the metadata revision the preview showed. A no-op or a
// refusal is presented without a prompt.
func newRepositoryRepairCommand(setResult func(app.OperationResult)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repair",
		Short: "Preview and apply the mechanical repairs `repository check` reports on a migrated repository",
		Args:  cobra.NoArgs,
		// metadata-write: one repair descendant published to the metadata branch
		// under an exact lease. external-write: --pr-backlinks edits merged PR
		// descriptions on GitHub. It never touches the local .docket worktree.
		Annotations: capability("repository.repair", EffectExternalWrite, EffectMetadataWrite),
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
			prBacklinks, _ := c.Flags().GetBool("pr-backlinks")
			if prBacklinks {
				gh, err := repositoryRepairGitHub()
				if err != nil {
					return err
				}
				deps.GitHub = gh
			}
			yes, _ := c.Flags().GetBool("yes")
			if yes {
				setResult(repositoryRepairRunner(c.Context(), deps, app.RepairOptions{Authorized: true, PRBacklinks: prBacklinks}))
				return nil
			}

			preview := repositoryRepairRunner(c.Context(), deps, app.RepairOptions{PRBacklinks: prBacklinks})
			jsonMode, _ := c.Flags().GetBool("json")
			confirmable, ok := preview.(interface{ ConfirmationRequired() bool })
			if jsonMode || !repositoryConfirmInteractive() || !ok || !confirmable.ConfirmationRequired() {
				setResult(preview)
				return nil
			}
			fmt.Fprintln(c.OutOrStdout(), preview.HumanText())
			fmt.Fprint(c.OutOrStdout(), "repair? [y/N] ")
			if !repositoryReadYes(c.InOrStdin()) {
				setResult(preview)
				return nil
			}
			expected := ""
			if p, ok := preview.(interface{ SourceRev() string }); ok {
				expected = p.SourceRev()
			}
			setResult(repositoryRepairRunner(c.Context(), deps, app.RepairOptions{Authorized: true, ExpectedSource: expected, PRBacklinks: prBacklinks}))
			return nil
		},
	}
	cmd.Flags().String("repo-dir", "", "repository `dir` to operate on (default: current directory)")
	cmd.Flags().Bool("yes", false, "authorize the previewed repairs without an interactive confirmation")
	cmd.Flags().Bool("pr-backlinks", false, "repoint merged pull requests whose change backlink names a path that no longer exists (reads and edits PR descriptions on GitHub)")
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
// The capability id is the dotted command path "repository."+name; effects are
// declared by each caller (the repository verbs differ — check reads, prepare
// and configure-tests write locally, init writes metadata + local) rather than
// looked up from a name inside the helper.
func repositorySubcommand(name, short string, run func(c *cobra.Command, deps app.SetupDeps), effects ...Effect) *cobra.Command {
	cmd := &cobra.Command{
		Use:         name,
		Short:       short,
		Args:        cobra.NoArgs,
		Annotations: capability("repository."+name, effects...),
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
	cmd.Flags().String("repo-dir", "", "repository `dir` to operate on (default: current directory)")
	return cmd
}
