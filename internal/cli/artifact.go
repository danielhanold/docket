package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/app"
)

// This file is the `docket artifact` command family: thin adapters that read
// their flags, hand them to the matching internal/app operation, and let the
// presenter own the outcome. No body here branches on repository content — path
// containment, marker validation, and change resolution all belong to
// internal/app.

// newArtifactCommand builds the `artifact` command group. setResult is the
// closure that hands a computed operation result back to Run's single
// presentation point, mirroring newContextCommand.
func newArtifactCommand(setResult func(app.OperationResult)) *cobra.Command {
	artifactCmd := &cobra.Command{
		Use:   "artifact",
		Short: "Render docket-managed blocks into workflow artifacts",
		// A command group resolves its subcommand before Args runs, so anything
		// reaching here named no subcommand; NoArgs names an offending token and
		// the bare `docket artifact` falls through to RunE's missing-command error.
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("missing command")
		},
	}

	backlink := &cobra.Command{
		Use:         "backlink",
		Short:       "Stamp the managed backlink block onto an artifact, pointing home to its change",
		Args:        cobra.NoArgs,
		Annotations: capability("artifact.backlink", EffectLocalWrite),
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, err := resolveRepoDir(c)
			if err != nil {
				return err
			}
			artifact, _ := c.Flags().GetString("artifact")
			change, _ := c.Flags().GetString("change")
			deps, err := newPlanningDeps()
			if err != nil {
				return err
			}
			setResult(app.ArtifactBacklink(c.Context(), deps, repoDir,
				app.ArtifactBacklinkRequest{ArtifactPath: artifact, ChangePath: change}))
			return nil
		},
	}
	backlink.Flags().String("repo-dir", "", "worktree `dir` the artifact lives in (default: current directory)")
	backlink.Flags().String("artifact", "", "canonical repository-relative `path` of the artifact to stamp (required)")
	backlink.Flags().String("change", "", "canonical repository-relative `path` of the change to point home to (required)")
	_ = backlink.MarkFlagRequired("artifact")
	_ = backlink.MarkFlagRequired("change")

	artifactCmd.AddCommand(backlink)
	return artifactCmd
}
