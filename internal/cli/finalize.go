package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/app"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/workspace"
)

// This file is the top-level `docket finalize` command family: thin adapters
// that read their flags, hand them to the matching internal/app finalize
// operation over the real Git/GitHub/workspace seams, and let the presenter own
// the outcome. The terminal-half mutation subcommands land here in later change
// 0316 tasks; today the group is registered so its tree and shared dependency
// wiring exist for those subcommands to attach to. Every lifecycle, Git,
// GitHub, stack, reclaim, and cleanup policy belongs to internal/app, so no body
// here branches on repository content.

// newFinalizeCommand builds the `finalize` command group. setResult is the
// closure that hands a computed operation result back to Run's single
// presentation point, mirroring newChangeCommand.
func newFinalizeCommand(setResult func(app.OperationResult)) *cobra.Command {
	finalizeCmd := &cobra.Command{
		Use:   "finalize",
		Short: "Sequence a change's terminal half: rebase, publish, merge, and closeout",
		// A command group resolves its subcommand before Args runs, so anything
		// reaching here named no subcommand; NoArgs names an offending token and
		// the bare `docket finalize` falls through to RunE's missing-command error.
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("missing command")
		},
	}
	finalizeCmd.AddCommand(newFinalizeRetargetChildrenSubcommand(setResult))
	finalizeCmd.AddCommand(newFinalizeRebaseSubcommand(setResult))
	finalizeCmd.AddCommand(newFinalizeRebaseContinueSubcommand(setResult))
	finalizeCmd.AddCommand(newFinalizeRebaseAbortSubcommand(setResult))
	return finalizeCmd
}

// retargetChildrenInput is the bounded request-file payload for `finalize
// retarget-children`: the exact human-authorized child set from context finalize.
// The scalar identities (parent id, entity version) ride on flags — only the
// authored authorization set travels through the request file (Global
// Constraints). DisallowUnknownFields (via decodeInputFlag) rejects any other key.
type retargetChildrenInput struct {
	Children []app.AuthorizedChild `json:"children"`
}

// newFinalizeRetargetChildrenSubcommand builds `finalize retarget-children`: it
// reads the parent id and pinned entity version from flags, decodes the exact
// authorized child set from --input, and hands the assembled request to the
// operation over the shared finalize seams. No lifecycle, Git, GitHub, or stack
// policy lives here — the operation owns all of it.
func newFinalizeRetargetChildrenSubcommand(setResult func(app.OperationResult)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "retarget-children",
		Short: "Retarget each authorized open child PR onto the parent's effective base",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, _ := c.Flags().GetString("repo-dir")
			id, _ := c.Flags().GetInt("id")
			version, _ := c.Flags().GetString("version")

			var input retargetChildrenInput
			if err := decodeInputFlag(c, &input); err != nil {
				return err
			}
			deps, err := newFinalizeDeps()
			if err != nil {
				return err
			}
			setResult(app.FinalizeRetargetChildren(c.Context(), deps, repoDir, app.RetargetChildrenRequest{
				ID:       id,
				Version:  version,
				Children: input.Children,
			}))
			return nil
		},
	}
	cmd.Flags().Int("id", 0, "parent change id whose open children are retargeted (required)")
	cmd.Flags().String("version", "", "exact parent record blob object id from the authoritative context read (required)")
	cmd.Flags().String("input", "", "JSON request file with the authorized child set, or - for stdin (required)")
	cmd.Flags().String("repo-dir", "", "repository directory to operate on (default: current directory)")
	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("version")
	_ = cmd.MarkFlagRequired("input")
	return cmd
}

// newFinalizeRebaseSubcommand builds `finalize rebase`: it rebases an implemented
// change's feature branch onto its effective base and composes the local gate. The
// scalar identity (id, pinned version, expected head) rides on flags; there is no
// authored request body.
func newFinalizeRebaseSubcommand(setResult func(app.OperationResult)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rebase",
		Short: "Rebase a change's feature branch onto its effective base and run the local gate",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, _ := c.Flags().GetString("repo-dir")
			id, _ := c.Flags().GetInt("id")
			version, _ := c.Flags().GetString("version")
			head, _ := c.Flags().GetString("head")
			deps, err := newFinalizeDeps()
			if err != nil {
				return err
			}
			setResult(app.FinalizeRebase(c.Context(), deps, repoDir, app.FinalizeRebaseRequest{
				ID: id, Version: version, Head: head,
			}))
			return nil
		},
	}
	cmd.Flags().Int("id", 0, "implemented change id to rebase (required)")
	cmd.Flags().String("version", "", "exact record blob object id from the authoritative context read (required)")
	cmd.Flags().String("head", "", "expected local feature head the rebase begins from (required)")
	cmd.Flags().String("repo-dir", "", "repository directory to operate on (default: current directory)")
	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("version")
	_ = cmd.MarkFlagRequired("head")
	return cmd
}

// newFinalizeRebaseContinueSubcommand builds `finalize rebase-continue`: it feeds a
// conflict-resolver's report into the owned rebase, staging exactly the reported
// (and verified) paths and continuing. The scalar identity (id, attempt token)
// rides on flags; the authored resolver report rides in --input (never argv).
func newFinalizeRebaseContinueSubcommand(setResult func(app.OperationResult)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rebase-continue",
		Short: "Continue an owned rebase from a verified conflict-resolver report",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, _ := c.Flags().GetString("repo-dir")
			id, _ := c.Flags().GetInt("id")
			attempt, _ := c.Flags().GetString("attempt")
			var report app.ResolverReport
			if err := decodeInputFlag(c, &report); err != nil {
				return err
			}
			deps, err := newFinalizeDeps()
			if err != nil {
				return err
			}
			setResult(app.FinalizeRebaseContinue(c.Context(), deps, repoDir, id, attempt, report))
			return nil
		},
	}
	finalizeReportFlags(cmd)
	return cmd
}

// newFinalizeRebaseAbortSubcommand builds `finalize rebase-abort`: it proves the
// owned attempt, aborts the rebase, and verifies the original head was restored.
// The scalar identity rides on flags; the authored resolver report rides in
// --input (never argv).
func newFinalizeRebaseAbortSubcommand(setResult func(app.OperationResult)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rebase-abort",
		Short: "Abort an owned rebase and verify restoration to the recorded original head",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, _ := c.Flags().GetString("repo-dir")
			id, _ := c.Flags().GetInt("id")
			attempt, _ := c.Flags().GetString("attempt")
			var report app.ResolverReport
			if err := decodeInputFlag(c, &report); err != nil {
				return err
			}
			deps, err := newFinalizeDeps()
			if err != nil {
				return err
			}
			setResult(app.FinalizeRebaseAbort(c.Context(), deps, repoDir, id, attempt, report))
			return nil
		},
	}
	finalizeReportFlags(cmd)
	return cmd
}

// finalizeReportFlags declares the shared scalar-identity + report-file flags the
// resolver-fed rebase subcommands take: id and attempt on flags, the authored
// report in --input.
func finalizeReportFlags(cmd *cobra.Command) {
	cmd.Flags().Int("id", 0, "change id whose owned rebase is continued or aborted (required)")
	cmd.Flags().String("attempt", "", "the owned rebase attempt token from the conflicted result (required)")
	cmd.Flags().String("input", "", "JSON resolver report file, or - for stdin (required)")
	cmd.Flags().String("repo-dir", "", "repository directory to operate on (default: current directory)")
	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("attempt")
	_ = cmd.MarkFlagRequired("input")
}

// newFinalizeDeps assembles the production seams every finalize operation needs:
// the read-only planning seams (reader/engine/git client/clock), the GitHub
// client, the workspace service over the same git client, and the PR-facts
// prober composed over the GitHub client.
func newFinalizeDeps() (app.FinalizeDeps, error) {
	planning, err := newPlanningDeps()
	if err != nil {
		return app.FinalizeDeps{}, err
	}
	ghClient, err := githubcli.NewClient()
	if err != nil {
		return app.FinalizeDeps{}, err
	}
	ws, err := workspace.NewService(planning.Client)
	if err != nil {
		return app.FinalizeDeps{}, err
	}
	return app.FinalizeDeps{
		Planning:  planning,
		GitHub:    ghClient,
		Workspace: ws,
		PRProber:  app.NewGitHubFinalizeProber(ghClient),
		Gate:      app.NewFinalizeGate(planning, app.WorkspaceDeps{Service: ws}),
	}, nil
}
