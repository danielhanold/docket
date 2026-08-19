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
	finalizeCmd.AddCommand(newFinalizePublishSubcommand(setResult))
	finalizeCmd.AddCommand(newFinalizeBlockSubcommand(setResult))
	finalizeCmd.AddCommand(newFinalizeClearBlockSubcommand(setResult))
	finalizeCmd.AddCommand(newFinalizeMergeSubcommand(setResult))
	return finalizeCmd
}

// finalizeBlockInput is the bounded request-file payload for `finalize block`:
// the authored report that crosses to the PR comment and the authored concrete
// remedy recorded in the marker. The scalar identities (id, version, pr number,
// attempt, reason, head) ride on flags — only the authored Markdown travels
// through the request file (Global Constraints). DisallowUnknownFields (via
// decodeInputFlag) rejects any other key.
type finalizeBlockInput struct {
	Report string `json:"report"`
	Remedy string `json:"remedy"`
}

// newFinalizeBlockSubcommand builds `finalize block`: it ensures the owned PR
// comment first, then upserts the single durable "## Finalize blocked" marker in
// one exact-version transaction. The scalar identity rides on flags; the authored
// report and remedy ride in --input (never argv).
func newFinalizeBlockSubcommand(setResult func(app.OperationResult)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "block",
		Short: "Record a blocked finalize attempt: an owned PR comment then a durable marker",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, _ := c.Flags().GetString("repo-dir")
			id, _ := c.Flags().GetInt("id")
			version, _ := c.Flags().GetString("version")
			prNumber, _ := c.Flags().GetInt("pr-number")
			attempt, _ := c.Flags().GetString("attempt")
			reason, _ := c.Flags().GetString("reason")
			head, _ := c.Flags().GetString("head")
			var in finalizeBlockInput
			if err := decodeInputFlag(c, &in); err != nil {
				return err
			}
			deps, err := newFinalizeDeps()
			if err != nil {
				return err
			}
			setResult(app.FinalizeBlock(c.Context(), deps, repoDir, app.BlockRequest{
				ID:       id,
				Version:  version,
				PRNumber: prNumber,
				Attempt:  attempt,
				Reason:   reason,
				Head:     head,
				Report:   in.Report,
				Remedy:   in.Remedy,
			}))
			return nil
		},
	}
	cmd.Flags().Int("id", 0, "change id whose finalize attempt is blocked (required)")
	cmd.Flags().String("version", "", "exact record blob object id from the authoritative context read (required)")
	cmd.Flags().Int("pr-number", 0, "pull-request number the owned comment is ensured on (required)")
	cmd.Flags().String("attempt", "", "opaque owned attempt token keying the comment marker and marker idempotency (required)")
	cmd.Flags().String("reason", "", "stable machine reason token for the block (required)")
	cmd.Flags().String("head", "", "verified feature head recorded as a fact (required)")
	cmd.Flags().String("input", "", "JSON request file with the authored report and remedy, or - for stdin (required)")
	cmd.Flags().String("repo-dir", "", "repository directory to operate on (default: current directory)")
	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("version")
	_ = cmd.MarkFlagRequired("pr-number")
	_ = cmd.MarkFlagRequired("attempt")
	_ = cmd.MarkFlagRequired("reason")
	_ = cmd.MarkFlagRequired("head")
	_ = cmd.MarkFlagRequired("input")
	return cmd
}

// newFinalizeClearBlockSubcommand builds `finalize clear-block`: it reprobes an
// exact current head, a published remote ref, a matching open PR, and green body
// evidence (gate on) before transactionally removing the marker. The scalar
// identity rides on flags; there is no authored request body.
func newFinalizeClearBlockSubcommand(setResult func(app.OperationResult)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clear-block",
		Short: "Remove a finalize-blocked marker after reprobing head, remote ref, PR, and evidence",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, _ := c.Flags().GetString("repo-dir")
			id, _ := c.Flags().GetInt("id")
			version, _ := c.Flags().GetString("version")
			head, _ := c.Flags().GetString("head")
			prNumber, _ := c.Flags().GetInt("pr-number")
			deps, err := newFinalizeDeps()
			if err != nil {
				return err
			}
			setResult(app.FinalizeClearBlock(c.Context(), deps, repoDir, app.ClearBlockRequest{
				ID:       id,
				Version:  version,
				Head:     head,
				PRNumber: prNumber,
			}))
			return nil
		},
	}
	cmd.Flags().Int("id", 0, "change id whose finalize-blocked marker to clear (required)")
	cmd.Flags().String("version", "", "exact record blob object id from the authoritative context read (required)")
	cmd.Flags().String("head", "", "exact current feature head the reprobe must confirm (required)")
	cmd.Flags().Int("pr-number", 0, "canonical pull-request number whose open state is reprobed (required)")
	cmd.Flags().String("repo-dir", "", "repository directory to operate on (default: current directory)")
	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("version")
	_ = cmd.MarkFlagRequired("head")
	_ = cmd.MarkFlagRequired("pr-number")
	return cmd
}

// newFinalizeMergeSubcommand builds `finalize merge`: it merges one exact pull
// request at its authorized head after a fresh recheck of every merge conjunct,
// then verifies the merge authoritatively. The scalar identity (id, pinned
// version, expected head) rides on flags; --admin requests an admin-override
// merge. Invoking this attended command is itself the human authorization, so
// the request always carries ExplicitID — the app layer is what gates --admin on
// it, refusing any merge whose admin was not explicitly named.
func newFinalizeMergeSubcommand(setResult func(app.OperationResult)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "merge",
		Short: "Merge a change's pull request at its expected head and verify it authoritatively",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, _ := c.Flags().GetString("repo-dir")
			id, _ := c.Flags().GetInt("id")
			version, _ := c.Flags().GetString("version")
			head, _ := c.Flags().GetString("head")
			admin, _ := c.Flags().GetBool("admin")
			deps, err := newFinalizeDeps()
			if err != nil {
				return err
			}
			setResult(app.FinalizeMerge(c.Context(), deps, repoDir, app.FinalizeMergeRequest{
				ID:      id,
				Version: version,
				Head:    head,
				Admin:   admin,
				// The attended `finalize merge --id` invocation IS the explicit human
				// authorization the approval and finalize-blocked overrides read.
				ExplicitID: true,
			}))
			return nil
		},
	}
	cmd.Flags().Int("id", 0, "change id whose pull request to merge (required)")
	cmd.Flags().String("version", "", "exact record blob object id from the authoritative context read (required)")
	cmd.Flags().String("head", "", "exact feature head the merge must match (required)")
	cmd.Flags().Bool("admin", false, "request an admin-override merge (honored only on this attended, explicitly-named run)")
	cmd.Flags().String("repo-dir", "", "repository directory to operate on (default: current directory)")
	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("version")
	_ = cmd.MarkFlagRequired("head")
	return cmd
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

// newFinalizePublishSubcommand builds `finalize publish`: it publishes a rewritten
// feature head onto its remote ref under the owned rebase receipt's exact lease and
// converges the PR build-evidence block onto that head. The scalar identity (id,
// attempt token, expected head) rides on flags; the canonical evidence bytes ride
// in --evidence (a request file or stdin), never a shell-escaped flag.
func newFinalizePublishSubcommand(setResult func(app.OperationResult)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "publish",
		Short: "Publish a rebased feature head under its receipt lease and update the PR build-evidence block",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, _ := c.Flags().GetString("repo-dir")
			id, _ := c.Flags().GetInt("id")
			attempt, _ := c.Flags().GetString("attempt")
			head, _ := c.Flags().GetString("head")
			evSource, _ := c.Flags().GetString("evidence")
			ev, err := readRecordSource(c.InOrStdin(), evSource)
			if err != nil {
				return err
			}
			deps, err := newFinalizeDeps()
			if err != nil {
				return err
			}
			setResult(app.FinalizePublish(c.Context(), deps, repoDir, app.FinalizePublishRequest{
				ID:             id,
				Attempt:        attempt,
				Head:           head,
				EvidenceRecord: ev,
			}))
			return nil
		},
	}
	cmd.Flags().Int("id", 0, "change id whose rewritten head to publish (required)")
	cmd.Flags().String("attempt", "", "the owned rebase attempt token that authorizes the rewrite push (required)")
	cmd.Flags().String("head", "", "exact rewritten feature head to publish and certify (required)")
	cmd.Flags().String("evidence", "", "canonical build-evidence record file, or - for stdin (required)")
	cmd.Flags().String("repo-dir", "", "repository directory to operate on (default: current directory)")
	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("attempt")
	_ = cmd.MarkFlagRequired("head")
	_ = cmd.MarkFlagRequired("evidence")
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
