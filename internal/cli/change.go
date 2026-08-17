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

	create := changeSubcommand("create",
		"Create a new proposed change from a JSON request",
		func(c *cobra.Command, deps app.PlanningDeps, repoDir string) error {
			var req app.ChangeCreateRequest
			if err := decodeRequestFlag(c, &req); err != nil {
				return err
			}
			setResult(app.ChangeCreate(c.Context(), deps, repoDir, req))
			return nil
		})

	groom := changeSubcommand("groom",
		"Groom a proposed change to build-ready (spec or trivial) from a JSON request",
		func(c *cobra.Command, deps app.PlanningDeps, repoDir string) error {
			var req app.ChangeGroomRequest
			if err := decodeRequestFlag(c, &req); err != nil {
				return err
			}
			setResult(app.ChangeGroom(c.Context(), deps, repoDir, req))
			return nil
		})

	block := changeSubcommand("block",
		"Block a change, recording the reason, from a JSON request",
		func(c *cobra.Command, deps app.PlanningDeps, repoDir string) error {
			var req app.ChangeBlockRequest
			if err := decodeRequestFlag(c, &req); err != nil {
				return err
			}
			setResult(app.ChangeBlock(c.Context(), deps, repoDir, req))
			return nil
		})

	deferCmd := changeSubcommand("defer",
		"Defer a change, recording why, from a JSON request",
		func(c *cobra.Command, deps app.PlanningDeps, repoDir string) error {
			var req app.ChangeDeferRequest
			if err := decodeRequestFlag(c, &req); err != nil {
				return err
			}
			setResult(app.ChangeDefer(c.Context(), deps, repoDir, req))
			return nil
		})

	kill := changeSubcommand("kill",
		"Kill a change, archiving it, from a JSON request",
		func(c *cobra.Command, deps app.PlanningDeps, repoDir string) error {
			var req app.ChangeKillRequest
			if err := decodeRequestFlag(c, &req); err != nil {
				return err
			}
			setResult(app.ChangeKill(c.Context(), deps, repoDir, req))
			return nil
		})

	claim := changeIDVersionSubcommand("claim",
		"Claim a build-ready change at an exact version, moving it to in-progress",
		func(c *cobra.Command, deps app.PlanningDeps, repoDir string, req app.ChangeClaimRequest) {
			setResult(app.ChangeClaim(c.Context(), deps, repoDir, req))
		})

	refreshClaim := changeIDVersionSubcommand("refresh-claim",
		"Re-stamp an in-progress change's claim lease at an exact version",
		func(c *cobra.Command, deps app.PlanningDeps, repoDir string, req app.ChangeClaimRequest) {
			setResult(app.ChangeRefreshClaim(c.Context(), deps, repoDir, req))
		})

	changeCmd.AddCommand(create, groom, block, deferCmd, kill, claim, refreshClaim)
	return changeCmd
}

// changeIDVersionSubcommand builds one `change <verb>` command whose input is the
// (id, version) pair rather than a JSON request body: the claim transitions
// carry no authored Markdown, so they take scalar flags (Global Constraints:
// request files are for authored Markdown, never these). run receives the
// resolved dependencies, repo directory, and decoded request.
func changeIDVersionSubcommand(verb, short string, run func(c *cobra.Command, deps app.PlanningDeps, repoDir string, req app.ChangeClaimRequest)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   verb,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, _ := c.Flags().GetString("repo-dir")
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
	cmd.Flags().Int("id", 0, "change id to operate on (required)")
	cmd.Flags().String("version", "", "exact record blob object id from the authoritative context read (required)")
	cmd.Flags().String("repo-dir", "", "repository directory to operate on (default: current directory)")
	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("version")
	return cmd
}

// changeSubcommand builds one `change <verb>` command with the shared --request
// / --repo-dir flags. run receives the resolved dependencies and repo directory;
// it decodes the request and invokes the operation. Constructing PlanningDeps
// here — after flag parsing, before decoding — keeps a Git-client failure
// classified as an argument error, exactly like the request-decode failures.
func changeSubcommand(verb, short string, run func(c *cobra.Command, deps app.PlanningDeps, repoDir string) error) *cobra.Command {
	cmd := &cobra.Command{
		Use:   verb,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, _ := c.Flags().GetString("repo-dir")
			deps, err := newPlanningDeps()
			if err != nil {
				return err
			}
			return run(c, deps, repoDir)
		},
	}
	cmd.Flags().String("request", "", "JSON request file, or - to read the request from stdin (required)")
	cmd.Flags().String("repo-dir", "", "repository directory to operate on (default: current directory)")
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
