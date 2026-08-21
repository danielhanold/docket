package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/app"
	"github.com/danielhanold/docket/internal/githubcli"
)

// This file is the `docket pr` command family: thin adapters that read their
// flags, decode the authored PR prose request and the canonical evidence bytes,
// hand them to the matching internal/app operation over the real Git-backed seams
// plus the landed workspace service and the githubcli adapter, and let the
// presenter own the outcome. Every policy question — identity agreement, body
// assembly, disposition mapping, redaction — belongs to internal/app, so no body
// here branches on repository or GitHub content.

// prBodyRequest is the authored title+body request the `--body` file carries.
// Authored prose rides in a request file, never a shell-escaped flag (Global
// Constraints); the strict decoder rejects any unknown field.
type prBodyRequest struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// newPRCommand builds the `pr` command group. setResult is the closure that hands
// a computed operation result back to Run's single presentation point, mirroring
// newWorkspaceCommand.
func newPRCommand(setResult func(app.OperationResult)) *cobra.Command {
	prCmd := &cobra.Command{
		Use:   "pr",
		Short: "Publish the ready-for-review pull request for an in-progress change's tested head",
		// A command group resolves its subcommand before Args runs, so anything
		// reaching here named no subcommand; NoArgs names an offending token and
		// the bare `docket pr` falls through to RunE's missing-command error.
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("missing command")
		},
	}

	publish := &cobra.Command{
		Use:   "publish",
		Short: "Publish (create or adopt) the pull request for a published feature head with its build evidence",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, err := resolveRepoDir(c)
			if err != nil {
				return err
			}
			id, _ := c.Flags().GetInt("id")
			head, _ := c.Flags().GetString("head")
			bodySource, _ := c.Flags().GetString("body")
			evSource, _ := c.Flags().GetString("evidence")

			var body prBodyRequest
			if err := decodeRequest(c.InOrStdin(), bodySource, &body); err != nil {
				return err
			}
			evidence, err := readRecordSource(c.InOrStdin(), evSource)
			if err != nil {
				return err
			}
			deps, wdeps, gdeps, err := newPRDeps()
			if err != nil {
				return err
			}
			setResult(app.PRPublish(c.Context(), deps, wdeps, gdeps, repoDir, app.PRPublishRequest{
				ID:             id,
				Head:           head,
				Title:          body.Title,
				Body:           body.Body,
				EvidenceRecord: evidence,
			}))
			return nil
		},
	}
	publish.Flags().Int("id", 0, "change id whose pull request to publish (required)")
	publish.Flags().String("head", "", "exact published feature head the PR must certify (required)")
	publish.Flags().String("body", "", "JSON request file with the authored PR title and body, or - for stdin (required)")
	publish.Flags().String("evidence", "", "canonical build-evidence record file, or - for stdin (required)")
	publish.Flags().String("repo-dir", "", "repository directory to operate on (default: current directory)")
	_ = publish.MarkFlagRequired("id")
	_ = publish.MarkFlagRequired("head")
	_ = publish.MarkFlagRequired("body")
	_ = publish.MarkFlagRequired("evidence")

	prCmd.AddCommand(publish)
	return prCmd
}

// newPRDeps assembles the read-only planning seams, the landed workspace service,
// and the githubcli adapter — the three seams `pr publish` composes. A gh that
// cannot be resolved is an argument-time error, exactly like a Git-client failure.
func newPRDeps() (app.PlanningDeps, app.WorkspaceDeps, app.GitHubDeps, error) {
	deps, wdeps, err := newWorkspaceDeps()
	if err != nil {
		return app.PlanningDeps{}, app.WorkspaceDeps{}, app.GitHubDeps{}, err
	}
	client, err := githubcli.NewClient()
	if err != nil {
		return app.PlanningDeps{}, app.WorkspaceDeps{}, app.GitHubDeps{}, err
	}
	return deps, wdeps, app.GitHubDeps{Service: client}, nil
}
