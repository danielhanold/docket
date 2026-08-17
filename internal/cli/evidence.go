package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/app"
)

// This file is the `docket evidence` command family: thin adapters that read
// their flags, hand them to the matching internal/app operation, and let the
// presenter own the outcome. No body here decides evidence policy — the passed-
// only gate, the observed gate command, the head match, and the verify verdict
// all belong to internal/app.

// newEvidenceCommand builds the `evidence` command group. setResult is the
// closure that hands a computed operation result back to Run's single
// presentation point, mirroring newWorkspaceCommand.
func newEvidenceCommand(setResult func(app.OperationResult)) *cobra.Command {
	evidenceCmd := &cobra.Command{
		Use:   "evidence",
		Short: "Record and verify the build-evidence that certifies an exact tested commit",
		// A command group resolves its subcommand before Args runs, so anything
		// reaching here named no subcommand; NoArgs names an offending token and
		// the bare `docket evidence` falls through to RunE's missing-command error.
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("missing command")
		},
	}

	record := &cobra.Command{
		Use:   "record",
		Short: "Record build evidence from a passed gate run at the current feature head",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, _ := c.Flags().GetString("repo-dir")
			id, _ := c.Flags().GetInt("id")
			run, _ := c.Flags().GetString("run")
			head, _ := c.Flags().GetString("head")
			deps, wdeps, err := newWorkspaceDeps()
			if err != nil {
				return err
			}
			setResult(app.EvidenceRecord(c.Context(), deps, wdeps, repoDir,
				app.EvidenceRecordRequest{ID: id, RunDir: run, Head: head}))
			return nil
		},
	}
	record.Flags().Int("id", 0, "change id the evidence belongs to (required)")
	record.Flags().String("run", "", "absolute gate run directory to observe (required)")
	record.Flags().String("head", "", "exact feature head the evidence must certify (required)")
	record.Flags().String("repo-dir", "", "repository directory to operate on (default: current directory)")
	_ = record.MarkFlagRequired("id")
	_ = record.MarkFlagRequired("run")
	_ = record.MarkFlagRequired("head")

	verify := &cobra.Command{
		Use:   "verify",
		Short: "Verify a build-evidence record's canonical bytes against an exact head (read-only)",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			source, _ := c.Flags().GetString("record")
			head, _ := c.Flags().GetString("head")
			body, err := readRecordSource(c.InOrStdin(), source)
			if err != nil {
				return err
			}
			setResult(app.EvidenceVerify(app.EvidenceVerifyRequest{RecordFile: body, Head: head}))
			return nil
		},
	}
	verify.Flags().String("record", "", "evidence record file, or - to read the record bytes from stdin (required)")
	verify.Flags().String("head", "", "exact head to verify the record against (required)")
	_ = verify.MarkFlagRequired("record")
	_ = verify.MarkFlagRequired("head")

	evidenceCmd.AddCommand(record, verify)
	return evidenceCmd
}

// readRecordSource reads the raw evidence-record bytes from source — "-" for
// stdin, any other value a filesystem path. Unlike the JSON request decoders,
// the record file is authored Markdown bytes the evidence codec reparses, so it
// is read verbatim with no decoding.
func readRecordSource(stdin io.Reader, source string) ([]byte, error) {
	if source == "-" {
		body, err := io.ReadAll(stdin)
		if err != nil {
			return nil, fmt.Errorf("reading --record from stdin: %w", err)
		}
		return body, nil
	}
	body, err := os.ReadFile(source)
	if err != nil {
		return nil, fmt.Errorf("reading --record %q: %w", source, err)
	}
	return body, nil
}
