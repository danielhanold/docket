package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

// backlinkArtifactLoader is the scoped StateLoader for the two integration-ref
// backlink legs (runCloseoutBacklinkLeg, finalizeCleanupBacklinkRepair). The
// legs patch only the docket:backlink block of merged plan/results artifacts —
// paths that are not corpus records — so validating the integration branch's
// full corpus (newPlanningLoader) was never the right gate: the integration
// branch legitimately holds a partial corpus, and a pre-existing error in a
// record the mutation cannot touch must not refuse the patch. This loader
// reads exactly the targeted artifact paths and reports a parse failure of a
// TARGETED artifact as the one in-scope error; an absent artifact is a clean
// skip (the operation's own benign no-op case). Snapshot is left zero-valued:
// both backlink operations read through st.Tree only and never consult
// st.State.Snapshot, and the legs declare no entity expectations.
type backlinkArtifactLoader struct {
	paths []gitcli.RepoPath
}

// newBacklinkArtifactLoader builds the scoped loader over every artifact path
// the targets patch.
func newBacklinkArtifactLoader(targets []closeoutBacklinkTarget) transaction.StateLoader {
	var paths []gitcli.RepoPath
	for _, tg := range targets {
		for _, p := range tg.artifactPaths {
			paths = append(paths, gitcli.RepoPath(p))
		}
	}
	return backlinkArtifactLoader{paths: paths}
}

// Load reads the targeted artifacts through t. A tree/read failure is a Go
// error; a targeted artifact whose bytes fail document.Parse is an
// error-severity finding naming that path — the engine refuses on it at
// LoadBefore, and the leg's pending finding then carries the cause.
func (l backlinkArtifactLoader) Load(ctx context.Context, t transaction.Tree) (transaction.LoadedState, error) {
	blobs, err := t.ReadBlobs(ctx, l.paths)
	if err != nil {
		return transaction.LoadedState{}, fmt.Errorf("backlink loader: reading artifacts: %w", err)
	}
	documents := make(map[string]document.Document, len(blobs))
	sources := make(map[string][]byte, len(blobs))
	var findings []domain.Finding
	for _, b := range blobs {
		if !b.Found {
			continue // absent on the integration ref: the operation's benign skip
		}
		rel := string(b.Path)
		doc, perr := document.Parse(b.Blob.Bytes)
		if perr != nil {
			findings = append(findings, backlinkParseFinding(rel, perr))
			continue
		}
		documents[rel] = doc
		sources[rel] = append([]byte(nil), b.Blob.Bytes...)
	}
	return transaction.LoadedState{
		Report:    domain.NewValidationReport(findings),
		Documents: documents,
		Sources:   sources,
	}, nil
}

// ValidateEvolution: the corpus evolution rules govern records; the targeted
// plan/results artifacts are not records, so no before/after rule applies.
func (l backlinkArtifactLoader) ValidateEvolution(_, _ transaction.LoadedState) []domain.Finding {
	return nil
}

// backlinkParseFinding normalizes a document.Parse failure on a targeted
// artifact into an error-severity finding, mirroring planningParseFinding's
// code normalization; the entity kind is empty because the artifact is not a
// corpus record.
func backlinkParseFinding(rel string, err error) domain.Finding {
	code := string(FCParseFailed)
	var de *document.Error
	if errors.As(err, &de) {
		code = string(de.Kind)
	}
	return domain.Finding{
		Code:     code,
		Severity: domain.SeverityError,
		Entity:   domain.EntityRef{Path: rel},
	}
}

// backlinkLegDetail renders the typed cause of a backlink leg that did not
// land, so the terminal-backlink-pending finding is self-diagnosing. A failed
// disposition renders the typed *transaction.Failure (stage/kind: detail); a
// refused disposition renders each refusal finding's code and path — after the
// scoped loader, the only refusals left are in-scope artifact-level ones. Any
// other disposition renders empty (the coarse token alone already says it).
func backlinkLegDetail(res transaction.Result, execErr error) string {
	if fs := failureStatus(res, execErr); fs != nil {
		out := fs.Kind
		if fs.Stage != "" {
			out = fs.Stage + "/" + fs.Kind
		}
		if fs.Detail != "" {
			out += ": " + fs.Detail
		}
		return out
	}
	if res.Disposition == transaction.DispositionRefused && len(res.Findings) > 0 {
		parts := make([]string, 0, len(res.Findings))
		for _, f := range res.Findings {
			p := f.Code
			if f.Entity.Path != "" {
				p += " at " + f.Entity.Path
			}
			parts = append(parts, p)
		}
		return "refused: " + strings.Join(parts, "; ")
	}
	return ""
}
