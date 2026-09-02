package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/app"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

// This file is the `docket change` command family: thin adapters that read a
// closed JSON request from a file or stdin, hand it to the matching
// internal/app planning operation over the real Git-backed seams, and let the
// presenter own the outcome. The authored Markdown a request carries rides
// inside the JSON strings and is never interpolated into any shell command —
// the operation writes it through the transaction engine, never a subprocess.
// Every policy question — validation, allocation, lifecycle legality, the
// board fence — belongs to internal/app, so no body here branches on request
// content.

// systemClock is the production time source the planning operations read
// through transaction.Clock; the operations never call time.Now themselves.
type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// newChangeCommand builds the `change` command group. setResult is the closure
// that hands a computed operation result back to Run's single presentation
// point, mirroring how the inline status command assigns its result there.
func newChangeCommand(setResult func(app.OperationResult)) *cobra.Command {
	changeCmd := &cobra.Command{
		Use:   "change",
		Short: "Create and transition changes in the docket backlog",
		// A command group resolves its subcommand before Args runs, so anything
		// reaching here named no subcommand; NoArgs names an offending token and
		// the bare `docket change` falls through to RunE's missing-command error.
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("missing command")
		},
	}

	create := changeSubcommand("change", "create",
		"Create a new proposed change from a JSON request",
		func(c *cobra.Command, deps app.PlanningDeps, repoDir string) error {
			var req app.ChangeCreateRequest
			if err := decodeRequestFlag(c, &req); err != nil {
				return err
			}
			setResult(app.ChangeCreate(c.Context(), deps, repoDir, req))
			return nil
		}, EffectMetadataWrite)

	groom := changeSubcommand("change", "groom",
		"Groom a proposed change to build-ready (spec or trivial) from a JSON request",
		func(c *cobra.Command, deps app.PlanningDeps, repoDir string) error {
			var req app.ChangeGroomRequest
			if err := decodeRequestFlag(c, &req); err != nil {
				return err
			}
			setResult(app.ChangeGroom(c.Context(), deps, repoDir, req))
			return nil
		}, EffectMetadataWrite)

	block := changeSubcommand("change", "block",
		"Block a change, recording the reason, from a JSON request",
		func(c *cobra.Command, deps app.PlanningDeps, repoDir string) error {
			var req app.ChangeBlockRequest
			if err := decodeRequestFlag(c, &req); err != nil {
				return err
			}
			setResult(app.ChangeBlock(c.Context(), deps, repoDir, req))
			return nil
		}, EffectMetadataWrite)

	deferCmd := changeSubcommand("change", "defer",
		"Defer a change, recording why, from a JSON request",
		func(c *cobra.Command, deps app.PlanningDeps, repoDir string) error {
			var req app.ChangeDeferRequest
			if err := decodeRequestFlag(c, &req); err != nil {
				return err
			}
			setResult(app.ChangeDefer(c.Context(), deps, repoDir, req))
			return nil
		}, EffectMetadataWrite)

	kill := changeSubcommand("change", "kill",
		"Kill a change, archiving it, from a JSON request",
		func(c *cobra.Command, deps app.PlanningDeps, repoDir string) error {
			var req app.ChangeKillRequest
			if err := decodeRequestFlag(c, &req); err != nil {
				return err
			}
			setResult(app.ChangeKill(c.Context(), deps, repoDir, req))
			return nil
		}, EffectMetadataWrite)

	claim := changeIDVersionSubcommand("claim",
		"Claim a build-ready change at an exact version, moving it to in-progress",
		func(c *cobra.Command, deps app.PlanningDeps, repoDir string, req app.ChangeClaimRequest) {
			setResult(app.ChangeClaim(c.Context(), deps, repoDir, req))
		}, EffectMetadataWrite)

	refreshClaim := changeIDVersionSubcommand("refresh-claim",
		"Re-stamp an in-progress change's claim lease at an exact version",
		func(c *cobra.Command, deps app.PlanningDeps, repoDir string, req app.ChangeClaimRequest) {
			setResult(app.ChangeRefreshClaim(c.Context(), deps, repoDir, req))
		}, EffectMetadataWrite)

	reconcile := changeInputSubcommand("reconcile",
		"Reconcile an in-progress change against current reality from a JSON request",
		func(c *cobra.Command, deps app.PlanningDeps, repoDir string) error {
			var req app.ChangeReconcileRequest
			if err := decodeInputFlag(c, &req); err != nil {
				return err
			}
			setResult(app.ChangeReconcile(c.Context(), deps, repoDir, req))
			return nil
		}, EffectMetadataWrite)

	attachPlan := changeAttachSubcommand("attach-plan",
		"Verify a written plan from Git and link it to an in-progress change",
		func(c *cobra.Command, deps app.PlanningDeps, wdeps app.WorkspaceDeps, repoDir string, req app.ChangeAttachRequest) {
			setResult(app.ChangeAttachPlan(c.Context(), deps, wdeps, repoDir, req))
		}, EffectMetadataWrite)

	attachResults := changeAttachSubcommand("attach-results",
		"Verify an authored results record from Git and link it to an in-progress change",
		func(c *cobra.Command, deps app.PlanningDeps, wdeps app.WorkspaceDeps, repoDir string, req app.ChangeAttachRequest) {
			setResult(app.ChangeAttachResults(c.Context(), deps, wdeps, repoDir, req))
		}, EffectMetadataWrite)

	halt := changeInputSubcommand("halt",
		"Record a bounded run-halted report on an in-progress change from a JSON request",
		func(c *cobra.Command, deps app.PlanningDeps, repoDir string) error {
			id, _ := c.Flags().GetInt("id")
			version, _ := c.Flags().GetString("version")
			var in changeHaltInput
			if err := decodeInputFlag(c, &in); err != nil {
				return err
			}
			setResult(app.ChangeHalt(c.Context(), deps, repoDir, app.HaltRequest{ID: id, Version: version, Report: in.Report}))
			return nil
		}, EffectMetadataWrite)
	halt.Flags().Int("id", 0, "in-progress change `id` to halt (required)")
	halt.Flags().String("version", "", "exact record blob object `id` from the authoritative context read (required)")
	_ = halt.MarkFlagRequired("id")
	_ = halt.MarkFlagRequired("version")

	resumeHalted := newResumeHaltedSubcommand(setResult)

	reclaim := newReclaimSubcommand(setResult)

	markImplemented := newMarkImplementedSubcommand(setResult)

	repairIdentity := newRepairIdentitySubcommand(setResult)

	changeCmd.AddCommand(create, groom, block, deferCmd, kill, claim, refreshClaim, reconcile, attachPlan, attachResults, halt, resumeHalted, reclaim, markImplemented, repairIdentity)
	return changeCmd
}

// newRepairIdentitySubcommand builds `change repair-identity`: the version-pinned
// single-field identity repair the finalize identity checkpoint hands a human's
// decision to. Its scalar identities and the approved evidence ride on flags —
// the op writes exactly one frontmatter field (branch: or pr:), so there is no
// authored request body. Exactly one mode is chosen: --adopt-pr-head (trust the
// PR, the missing/mismatched-branch recovery) with --expect-pr/--expect-head, or
// --adopt-pr (trust the record) with --expect-branch. The app layer owns the
// mode/evidence validation and the closed reason-token vocabulary, so a
// contradictory flag combination is refused as invalid-request there. It
// composes the finalize seams — the read-only planning seams, the GitHub adapter
// (the exact PR read), and the workspace service (the ownership gate) — the same
// wiring the other terminal-half operations use.
func newRepairIdentitySubcommand(setResult func(app.OperationResult)) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "repair-identity",
		Short:       "Repair a change's recorded identity at an exact version: adopt the PR's head as branch, or a PR reference as pr",
		Args:        cobra.NoArgs,
		Annotations: capability("change.repair-identity", EffectMetadataWrite),
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, err := resolveRepoDir(c)
			if err != nil {
				return err
			}
			id, _ := c.Flags().GetInt("id")
			expectVersion, _ := c.Flags().GetString("expect-version")
			adoptPRHead, _ := c.Flags().GetBool("adopt-pr-head")
			expectPR, _ := c.Flags().GetInt("expect-pr")
			expectHead, _ := c.Flags().GetString("expect-head")
			adoptPR, _ := c.Flags().GetString("adopt-pr")
			expectBranch, _ := c.Flags().GetString("expect-branch")
			deps, err := newFinalizeDeps()
			if err != nil {
				return err
			}
			setResult(app.RepairIdentity(c.Context(), deps, repoDir, app.RepairIdentityRequest{
				ID:             id,
				ExpectVersion:  expectVersion,
				AdoptPRHead:    adoptPRHead,
				ExpectPRNumber: expectPR,
				ExpectHead:     expectHead,
				AdoptPR:        adoptPR,
				ExpectBranch:   expectBranch,
			}))
			return nil
		},
	}
	cmd.Flags().Int("id", 0, "change `id` whose recorded identity to repair (required)")
	cmd.Flags().String("expect-version", "", "exact change-record version `token` from the finalize report (required)")
	cmd.Flags().Bool("adopt-pr-head", false, "trust the PR: adopt the exact PR's reported head branch as branch:")
	cmd.Flags().Int("expect-pr", 0, "the exact PR `n`umber the approved evidence showed (with --adopt-pr-head)")
	cmd.Flags().String("expect-head", "", "the head branch `ref` the human approved (with --adopt-pr-head)")
	cmd.Flags().String("adopt-pr", "", "trust the record: adopt this PR `ref`erence as pr: (with --expect-branch)")
	cmd.Flags().String("expect-branch", "", "the recorded branch `name` the human approved (with --adopt-pr)")
	cmd.Flags().String("repo-dir", "", "repository `dir` to operate on (default: current directory)")
	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("expect-version")
	return cmd
}

// changeHaltInput is the bounded request-file payload for `change halt`: the
// authored run-halted report. The scalar identity (id, version) rides on flags —
// only the authored Markdown travels through the request file (Global
// Constraints).
type changeHaltInput struct {
	Report string `json:"report"`
}

// newResumeHaltedSubcommand builds `change resume-halted`: human-authorized
// recovery of a halted run. It requires the exact marked record, the explicit
// --acknowledge-quiescent acknowledgement, reprobes the owned workspace, then
// refreshes the claim and removes the marker. The scalar identity rides on flags;
// there is no authored request body. It composes the read-only planning seams and
// the workspace service (to reprobe the owned checkout).
func newResumeHaltedSubcommand(setResult func(app.OperationResult)) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "resume-halted",
		Short:       "Recover a halted run: reprobe the workspace, refresh the claim, and remove the marker",
		Args:        cobra.NoArgs,
		Annotations: capability("change.resume-halted", EffectMetadataWrite),
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, err := resolveRepoDir(c)
			if err != nil {
				return err
			}
			id, _ := c.Flags().GetInt("id")
			version, _ := c.Flags().GetString("version")
			ack, _ := c.Flags().GetBool("acknowledge-quiescent")
			deps, wdeps, err := newWorkspaceDeps()
			if err != nil {
				return err
			}
			setResult(app.ChangeResumeHalted(c.Context(), deps, wdeps, repoDir, app.ResumeRequest{
				ID:                   id,
				Version:              version,
				AcknowledgeQuiescent: ack,
			}))
			return nil
		},
	}
	cmd.Flags().Int("id", 0, "halted change `id` to resume (required)")
	cmd.Flags().String("version", "", "exact record blob object `id` from the authoritative context read (required)")
	cmd.Flags().Bool("acknowledge-quiescent", false, "explicit acknowledgement that the prior worker is quiescent (required to resume)")
	cmd.Flags().String("repo-dir", "", "repository `dir` to operate on (default: current directory)")
	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("version")
	return cmd
}

// newReclaimSubcommand builds `change reclaim`: the proof-gated return of a
// strictly-expired in-progress claim to proposed. Its scalar identities (id,
// version) ride on flags; the reclaim generates its own dated log entry, so it
// takes no request file. It composes the read-only planning seams and the
// workspace service — the workspace inspection is the reclaim's ownership and
// live-gate probe.
func newReclaimSubcommand(setResult func(app.OperationResult)) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "reclaim",
		Short:       "Return a strictly-expired, branchless, workspaceless in-progress claim to proposed",
		Args:        cobra.NoArgs,
		Annotations: capability("change.reclaim", EffectMetadataWrite),
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, err := resolveRepoDir(c)
			if err != nil {
				return err
			}
			id, _ := c.Flags().GetInt("id")
			version, _ := c.Flags().GetString("version")
			deps, wdeps, err := newWorkspaceDeps()
			if err != nil {
				return err
			}
			setResult(app.ChangeReclaim(c.Context(), deps, wdeps, repoDir, app.ChangeReclaimRequest{
				ID:      id,
				Version: version,
			}))
			return nil
		},
	}
	cmd.Flags().Int("id", 0, "expired in-progress change `id` to reclaim (required)")
	cmd.Flags().String("version", "", "exact record blob object `id` from the authoritative context read (required)")
	cmd.Flags().String("repo-dir", "", "repository `dir` to operate on (default: current directory)")
	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("version")
	return cmd
}

// newMarkImplementedSubcommand builds `change mark-implemented`: the final
// verified transition. Its scalar identities (id, version, head, pr) ride on
// flags; the canonical build-evidence record rides in a file (or stdin) via
// --evidence, reparsed by the operation. It composes the read-only planning
// seams, the workspace service, and the githubcli adapter — the same three seams
// pr publish uses — so the reprobe verifies the published PR without a second
// GitHub client.
func newMarkImplementedSubcommand(setResult func(app.OperationResult)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mark-implemented",
		Short: "Mark an in-progress change implemented after reprobing its head, evidence, and published PR",
		Args:  cobra.NoArgs,
		// metadata-write: applies the exact-version transaction recording the
		// implemented transition; the head/evidence/PR reprobes are read-only.
		Annotations: capability("change.mark-implemented", EffectMetadataWrite),
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, err := resolveRepoDir(c)
			if err != nil {
				return err
			}
			id, _ := c.Flags().GetInt("id")
			version, _ := c.Flags().GetString("version")
			head, _ := c.Flags().GetString("head")
			prRef, _ := c.Flags().GetString("pr")
			evSource, _ := c.Flags().GetString("evidence")

			record, err := readRecordSource(c.InOrStdin(), evSource)
			if err != nil {
				return err
			}
			deps, wdeps, gdeps, err := newPRDeps()
			if err != nil {
				return err
			}
			setResult(app.ChangeMarkImplemented(c.Context(), deps, wdeps, gdeps, repoDir, app.MarkImplementedRequest{
				ID:             id,
				Version:        version,
				Head:           head,
				PR:             prRef,
				EvidenceRecord: record,
			}))
			return nil
		},
	}
	cmd.Flags().Int("id", 0, "change `id` to mark implemented (required)")
	cmd.Flags().String("version", "", "exact record blob object `id` from the authoritative context read (required)")
	cmd.Flags().String("head", "", "exact tested feature head `ref` the transition must certify (required)")
	cmd.Flags().String("pr", "", "canonical PR `ref`erence returned by pr publish (required)")
	cmd.Flags().String("evidence", "", "canonical build-evidence record `file`, or - for stdin (required)")
	cmd.Flags().String("repo-dir", "", "repository `dir` to operate on (default: current directory)")
	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("version")
	_ = cmd.MarkFlagRequired("head")
	_ = cmd.MarkFlagRequired("pr")
	_ = cmd.MarkFlagRequired("evidence")
	return cmd
}

// changeAttachSubcommand builds one `change <verb>` command whose input is the
// (id, version, path, commit) tuple: an attach verifies a written artifact from
// Git and links it, so it takes scalar flags (Global Constraints: request files
// are for authored Markdown, never these). It builds the workspace-backed deps
// the attach operation needs to inspect the owned checkout.
func changeAttachSubcommand(verb, short string, run func(c *cobra.Command, deps app.PlanningDeps, wdeps app.WorkspaceDeps, repoDir string, req app.ChangeAttachRequest), effects ...Effect) *cobra.Command {
	cmd := &cobra.Command{
		Use:         verb,
		Short:       short,
		Args:        cobra.NoArgs,
		Annotations: capability("change."+verb, effects...),
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, err := resolveRepoDir(c)
			if err != nil {
				return err
			}
			id, _ := c.Flags().GetInt("id")
			version, _ := c.Flags().GetString("version")
			artifactPath, _ := c.Flags().GetString("path")
			commit, _ := c.Flags().GetString("commit")
			deps, wdeps, err := newWorkspaceDeps()
			if err != nil {
				return err
			}
			run(c, deps, wdeps, repoDir, app.ChangeAttachRequest{ID: id, Version: version, Path: artifactPath, Commit: commit})
			return nil
		},
	}
	cmd.Flags().Int("id", 0, "change `id` to attach the artifact to (required)")
	cmd.Flags().String("version", "", "exact record blob object `id` from the authoritative context read (required)")
	cmd.Flags().String("path", "", "canonical repository-relative artifact `path` (required)")
	cmd.Flags().String("commit", "", "exact feature commit `sha` the writer reported (required)")
	cmd.Flags().String("repo-dir", "", "repository `dir` to operate on (default: current directory)")
	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("version")
	_ = cmd.MarkFlagRequired("path")
	_ = cmd.MarkFlagRequired("commit")
	return cmd
}

// changeInputSubcommand builds one `change <verb>` command whose authored-
// Markdown request rides in a JSON body read from `--input <request-file>` (or
// `-` for stdin). It mirrors changeSubcommand but names the flag --input, the
// spelling the reconcile CLI uses; the decode goes through the same
// exactly-one-document strict decoder.
func changeInputSubcommand(verb, short string, run func(c *cobra.Command, deps app.PlanningDeps, repoDir string) error, effects ...Effect) *cobra.Command {
	cmd := &cobra.Command{
		Use:         verb,
		Short:       short,
		Args:        cobra.NoArgs,
		Annotations: capability("change."+verb, effects...),
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, err := resolveRepoDir(c)
			if err != nil {
				return err
			}
			deps, err := newPlanningDeps()
			if err != nil {
				return err
			}
			return run(c, deps, repoDir)
		},
	}
	cmd.Flags().String("input", "", "JSON request `file`, or - to read the request from stdin (required)")
	cmd.Flags().String("repo-dir", "", "repository `dir` to operate on (default: current directory)")
	_ = cmd.MarkFlagRequired("input")
	return cmd
}

// decodeInputFlag reads the command's --input source and strictly decodes one
// JSON document into dst, reusing decodeRequest's exactly-one-document rule.
func decodeInputFlag(c *cobra.Command, dst any) error {
	source, _ := c.Flags().GetString("input")
	return decodeRequest(c.InOrStdin(), source, dst)
}

// changeIDVersionSubcommand builds one `change <verb>` command whose input is the
// (id, version) pair rather than a JSON request body: the claim transitions
// carry no authored Markdown, so they take scalar flags (Global Constraints:
// request files are for authored Markdown, never these). run receives the
// resolved dependencies, repo directory, and decoded request.
func changeIDVersionSubcommand(verb, short string, run func(c *cobra.Command, deps app.PlanningDeps, repoDir string, req app.ChangeClaimRequest), effects ...Effect) *cobra.Command {
	cmd := &cobra.Command{
		Use:         verb,
		Short:       short,
		Args:        cobra.NoArgs,
		Annotations: capability("change."+verb, effects...),
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, err := resolveRepoDir(c)
			if err != nil {
				return err
			}
			id, _ := c.Flags().GetInt("id")
			version, _ := c.Flags().GetString("version")
			deps, err := newPlanningDeps()
			if err != nil {
				return err
			}
			run(c, deps, repoDir, app.ChangeClaimRequest{ID: id, Version: version})
			return nil
		},
	}
	cmd.Flags().Int("id", 0, "change `id` to operate on (required)")
	cmd.Flags().String("version", "", "exact record blob object `id` from the authoritative context read (required)")
	cmd.Flags().String("repo-dir", "", "repository `dir` to operate on (default: current directory)")
	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("version")
	return cmd
}

// changeSubcommand builds one `change <verb>` command with the shared --request
// / --repo-dir flags. run receives the resolved dependencies and repo directory;
// it decodes the request and invokes the operation. Constructing PlanningDeps
// here — after flag parsing, before decoding — keeps a Git-client failure
// classified as an argument error, exactly like the request-decode failures.
//
// group is the command's parent group ("change", "learning", or "adr"); the
// capability id is the dotted command path group+"."+verb, and the effects are
// declared by the caller — never a name→effects lookup inside the helper.
func changeSubcommand(group, verb, short string, run func(c *cobra.Command, deps app.PlanningDeps, repoDir string) error, effects ...Effect) *cobra.Command {
	cmd := &cobra.Command{
		Use:         verb,
		Short:       short,
		Args:        cobra.NoArgs,
		Annotations: capability(group+"."+verb, effects...),
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, err := resolveRepoDir(c)
			if err != nil {
				return err
			}
			deps, err := newPlanningDeps()
			if err != nil {
				return err
			}
			return run(c, deps, repoDir)
		},
	}
	cmd.Flags().String("request", "", "JSON request `file`, or - to read the request from stdin (required)")
	cmd.Flags().String("repo-dir", "", "repository `dir` to operate on (default: current directory)")
	_ = cmd.MarkFlagRequired("request")
	return cmd
}

// newPlanningDeps assembles the production seams every planning operation needs:
// a real Git client, a transaction engine over that client and the system clock,
// the Git-backed status reader, and the same clock as the operations' sole time
// source.
func newPlanningDeps() (app.PlanningDeps, error) {
	client, err := gitcli.NewClient()
	if err != nil {
		return app.PlanningDeps{}, err
	}
	return newPlanningDepsOver(client)
}

// newPlanningDepsOver assembles the read-only planning seams over an already
// constructed Git client, so a caller that must apply a non-default network
// policy (the maintenance sweep's newSweepFinalizeDeps) builds the transaction
// engine and status reader over the exact policy-carrying client instance rather
// than a second default one. newPlanningDeps is the default-policy entry point.
func newPlanningDepsOver(client *gitcli.Client) (app.PlanningDeps, error) {
	clock := systemClock{}
	engine, err := transaction.NewEngine(client, clock)
	if err != nil {
		return app.PlanningDeps{}, err
	}
	return app.PlanningDeps{
		Client: client,
		Engine: engine,
		Reader: app.NewGitStatusReader(client),
		Clock:  clock,
	}, nil
}

// decodeRequestFlag reads the command's --request source and strictly decodes
// one JSON document into dst.
func decodeRequestFlag(c *cobra.Command, dst any) error {
	source, _ := c.Flags().GetString("request")
	return decodeRequest(c.InOrStdin(), source, dst)
}

// decodeRequest reads a closed JSON request from source — "-" for stdin, any
// other value a filesystem path — and decodes exactly one document into dst with
// unknown fields rejected. An unknown field, malformed JSON, trailing content,
// or an unreadable path is an argument error the caller surfaces as invalid
// input.
func decodeRequest(stdin io.Reader, source string, dst any) error {
	var r io.Reader
	if source == "-" {
		r = stdin
	} else {
		f, err := os.Open(source)
		if err != nil {
			return fmt.Errorf("reading --request %q: %w", source, err)
		}
		defer f.Close()
		r = f
	}

	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("decoding --request JSON: %w", err)
	}
	// Exactly one document: a second decode must be the clean end of the stream.
	if err := dec.Decode(&json.RawMessage{}); err != io.EOF {
		return errors.New("--request must contain exactly one JSON document")
	}
	return nil
}
