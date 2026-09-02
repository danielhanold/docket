package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/app"
)

// This file is the `docket adr` command family: thin adapters that read a
// closed JSON request from a file or stdin and hand it to the matching
// internal/app ADR operation over the real Git-backed seams, letting the
// presenter own the outcome. It reuses the shared request-file and dependency
// plumbing (changeSubcommand, newPlanningDeps, decodeRequestFlag) in change.go
// — the request-file conventions are identical across the whole planning
// command surface. Every policy question — id allocation, Accepted-target
// gating, index rendering — belongs to internal/app, so no body here branches
// on request content.

// newADRCommand builds the `adr` command group. setResult is the closure that
// hands a computed operation result back to Run's single presentation point,
// mirroring newChangeCommand.
func newADRCommand(setResult func(app.OperationResult)) *cobra.Command {
	adrCmd := &cobra.Command{
		Use:   "adr",
		Short: "Record, supersede, and reverse architecture decisions",
		// A command group resolves its subcommand before Args runs, so anything
		// reaching here named no subcommand; NoArgs names an offending token and
		// the bare `docket adr` falls through to RunE's missing-command error.
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("missing command")
		},
	}

	record := changeSubcommand("adr", "record",
		"Record a new Accepted ADR from a JSON request",
		func(c *cobra.Command, deps app.PlanningDeps, repoDir string) error {
			var req app.ADRRecordRequest
			if err := decodeRequestFlag(c, &req); err != nil {
				return err
			}
			setResult(app.ADRRecordOp(c.Context(), deps, repoDir, req))
			return nil
		}, EffectMetadataWrite)

	supersede := changeSubcommand("adr", "supersede",
		"Supersede an Accepted ADR with a new one from a JSON request",
		func(c *cobra.Command, deps app.PlanningDeps, repoDir string) error {
			var req app.ADRReplaceRequest
			if err := decodeRequestFlag(c, &req); err != nil {
				return err
			}
			setResult(app.ADRSupersede(c.Context(), deps, repoDir, req))
			return nil
		}, EffectMetadataWrite)

	reverse := changeSubcommand("adr", "reverse",
		"Reverse an Accepted ADR with a new one from a JSON request",
		func(c *cobra.Command, deps app.PlanningDeps, repoDir string) error {
			var req app.ADRReplaceRequest
			if err := decodeRequestFlag(c, &req); err != nil {
				return err
			}
			setResult(app.ADRReverse(c.Context(), deps, repoDir, req))
			return nil
		}, EffectMetadataWrite)

	adrCmd.AddCommand(record, supersede, reverse)
	return adrCmd
}
