package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/app"
)

// This file is the `docket learning` command family: thin adapters that read a
// closed JSON request from a file or stdin and hand it to the matching
// internal/app manual-learning operation over the real Git-backed seams,
// letting the presenter own the outcome. It reuses the shared request-file and
// dependency plumbing (changeSubcommand, newPlanningDeps, decodeRequestFlag) in
// change.go — the request-file conventions are identical across the whole
// planning command surface. Every policy question — slug shape, duplicate
// detection, the learnings.enabled fence — belongs to internal/app, so no body
// here branches on request content.

// newLearningCommand builds the `learning` command group. setResult is the
// closure that hands a computed operation result back to Run's single
// presentation point, mirroring newChangeCommand.
func newLearningCommand(setResult func(app.OperationResult)) *cobra.Command {
	learningCmd := &cobra.Command{
		Use:   "learning",
		Short: "Record and update manual learning findings",
		// A command group resolves its subcommand before Args runs, so anything
		// reaching here named no subcommand; NoArgs names an offending token and
		// the bare `docket learning` falls through to RunE's missing-command error.
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("missing command")
		},
	}

	record := changeSubcommand("learning", "record",
		"Record a new manual learning finding from a JSON request",
		func(c *cobra.Command, deps app.PlanningDeps, repoDir string) error {
			var req app.LearningRecordRequest
			if err := decodeRequestFlag(c, &req); err != nil {
				return err
			}
			setResult(app.LearningRecordOp(c.Context(), deps, repoDir, req))
			return nil
		}, EffectMetadataWrite)

	update := changeSubcommand("learning", "update",
		"Update an existing manual learning finding from a JSON request",
		func(c *cobra.Command, deps app.PlanningDeps, repoDir string) error {
			var req app.LearningUpdateRequest
			if err := decodeRequestFlag(c, &req); err != nil {
				return err
			}
			setResult(app.LearningUpdate(c.Context(), deps, repoDir, req))
			return nil
		}, EffectMetadataWrite)

	learningCmd.AddCommand(record, update)
	return learningCmd
}
