package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

// This file is the shared plumbing every 0312 planning operation composes: the
// production transaction.StateLoader over the config-derived corpus, the
// request-content digest, the board-surface preflight fence, and the fold from
// a transaction outcome into the protocol-v1 result taxonomy. It decides no
// per-operation policy; each mutation task (change/learning/adr) supplies its
// own validation, plan closure, and result struct on top of these seams.

// PlanningDeps is every seam a planning operation needs; tests inject fakes.
type PlanningDeps struct {
	Client *gitcli.Client
	Engine interface {
		Execute(ctx context.Context, req transaction.Request) (transaction.Result, error)
	}
	Reader StatusReader      // 0310 pin/read seams for preflight + entity versions
	Clock  transaction.Clock // sole time source; operations never call time.Now
}

// planningError carries a protocol result alongside a machine reason and prose,
// so a preflight refusal computed before any transaction (e.g. the board-surface
// fence) maps cleanly onto the v1 taxonomy without a message-text switch. It is
// the app-layer analogue of the transaction engine's typed Failure.
type planningError struct {
	Result  Result
	Reason  string
	Message string
}

func (e *planningError) Error() string { return e.Message }

// asPlanningError reports whether err is, or wraps, a *planningError.
func asPlanningError(err error) (*planningError, bool) {
	var pe *planningError
	if errors.As(err, &pe) {
		return pe, true
	}
	return nil, false
}

// planningLoader is the production StateLoader. It reads the corpus through the
// same directory-prefix classification the status read uses, parses each record,
// and builds one snapshot. A single record that will not parse is a domain
// finding, not a loader failure — the engine reads the report and refuses on
// error-severity findings, exactly as it does for any other repository defect.
type planningLoader struct {
	eff config.Effective
}

// newPlanningLoader builds the production StateLoader over the resolved
// configuration's corpus directories.
func newPlanningLoader(eff config.Effective) transaction.StateLoader {
	return planningLoader{eff: eff}
}

// Load reads and validates the complete state visible through t. A tree/read
// failure is a Go error; a domain-invalid corpus — including a record whose
// bytes will not parse — is reported through the returned report. It mirrors
// parseCorpus in status.go: one bad record never aborts the read.
func (l planningLoader) Load(ctx context.Context, t transaction.Tree) (transaction.LoadedState, error) {
	entries, err := t.ListTree(ctx, corpusPrefixes(l.eff))
	if err != nil {
		return transaction.LoadedState{}, fmt.Errorf("planning loader: listing corpus: %w", err)
	}

	type classified struct {
		kind     repository.RecordKind
		location repository.RecordLocation
	}
	var paths []gitcli.RepoPath
	var meta []classified
	for _, e := range entries {
		if e.Type != "blob" {
			continue
		}
		kind, loc, ok := classifyCorpusPath(l.eff, string(e.Path))
		if !ok {
			continue
		}
		paths = append(paths, e.Path)
		meta = append(meta, classified{kind: kind, location: loc})
	}

	blobs, err := t.ReadBlobs(ctx, paths)
	if err != nil {
		return transaction.LoadedState{}, fmt.Errorf("planning loader: reading records: %w", err)
	}
	if len(blobs) != len(paths) {
		return transaction.LoadedState{}, fmt.Errorf("planning loader: read %d blobs for %d paths", len(blobs), len(paths))
	}

	in := repository.BuildInput{Config: l.eff}
	documents := make(map[string]document.Document, len(blobs))
	sources := make(map[string][]byte, len(blobs))
	var parseFindings []domain.Finding
	// blobs[i] aligns with paths[i] and meta[i]: overlay/base ReadBlobs return
	// results in request order.
	for i, b := range blobs {
		if !b.Found {
			// A path just listed cannot vanish from the same pinned tree; skip a
			// defensive gap rather than record a phantom.
			continue
		}
		rel := string(b.Path)
		doc, perr := document.Parse(b.Blob.Bytes)
		if perr != nil {
			parseFindings = append(parseFindings, planningParseFinding(meta[i].kind, rel, perr))
			continue
		}
		in.Documents = append(in.Documents, repository.InputDocument{
			Kind: meta[i].kind, Location: meta[i].location, Path: rel, Document: doc,
		})
		documents[rel] = doc
		sources[rel] = append([]byte(nil), b.Blob.Bytes...)
	}

	result, err := repository.BuildSnapshot(in)
	if err != nil {
		// A build error means the CALL was malformed — a loader/contract defect.
		return transaction.LoadedState{}, fmt.Errorf("planning loader: building snapshot: %w", err)
	}

	report := result.Report
	if len(parseFindings) > 0 {
		report = domain.NewValidationReport(append(result.Report.Findings(), parseFindings...))
	}

	return transaction.LoadedState{
		Snapshot:  result.Snapshot,
		Report:    report,
		Documents: documents,
		Sources:   sources,
	}, nil
}

// ValidateEvolution runs the landed before→after rules over the two states'
// exact source bytes.
func (l planningLoader) ValidateEvolution(before, after transaction.LoadedState) []domain.Finding {
	return repository.ValidateEvolution(repository.EvolutionInput{
		Before:        repository.BuildResult{Snapshot: before.Snapshot, Report: before.Report},
		After:         repository.BuildResult{Snapshot: after.Snapshot, Report: after.Report},
		BeforeSources: before.Sources,
		AfterSources:  after.Sources,
	})
}

// planningParseFinding normalizes a document.Parse failure into an
// error-severity domain finding carrying the typed error's kind as its code —
// the same normalization parseFinding applies in the status read.
func planningParseFinding(kind repository.RecordKind, rel string, err error) domain.Finding {
	code := "parse-failed"
	var de *document.Error
	if errors.As(err, &de) {
		code = string(de.Kind)
	}
	return domain.Finding{
		Code:     code,
		Severity: domain.SeverityError,
		Entity:   domain.EntityRef{Kind: planningEntityKind(kind), Path: rel},
	}
}

// planningEntityKind maps a record kind onto the finding entity kind.
func planningEntityKind(k repository.RecordKind) domain.EntityKind {
	switch k {
	case repository.KindChange:
		return domain.EntityChange
	case repository.KindADR:
		return domain.EntityADR
	case repository.KindLearning:
		return domain.EntityLearning
	default:
		return ""
	}
}

// canonicalDigest computes "sha256:<hex>" binding the operation key and the
// canonical compact JSON of payload. payload must be a closed request struct
// (no maps), so json.Marshal is canonical: fields serialize in declaration
// order with no insignificant whitespace. The operation is mixed in so two
// operations carrying an identical payload never collide.
func canonicalDigest(operation string, payload any) (transaction.RequestDigest, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("planning: marshaling digest payload: %w", err)
	}
	h := sha256.New()
	h.Write([]byte(operation))
	h.Write([]byte{0})
	h.Write(body)
	return transaction.RequestDigest("sha256:" + hex.EncodeToString(h.Sum(nil))), nil
}

// boardSurfaceGitHub is the fenced surface token: declaring it is an
// unsupported configuration for this slice, refused at preflight before any
// transaction runs.
const boardSurfaceGitHub = "github"

// boardSurfaceInline is the inline-board surface token: when present, every
// change operation's plan carries the rendered board bytes.
const boardSurfaceInline = "inline"

// ReasonUnsupportedBoardSurface is the stable machine reason a github-fenced
// board configuration reports.
const ReasonUnsupportedBoardSurface = "unsupported-board-surface"

// fenceBoardSurface reads the resolved board_surfaces: a `github` token is an
// unsupported configuration (refused before any transaction); otherwise inline
// reports whether the inline board surface is enabled. The github check runs
// first, so a `[inline github]` configuration is fenced rather than silently
// enabling inline.
func fenceBoardSurface(eff config.Effective) (inline bool, err error) {
	for _, s := range eff.BoardSurfaces.Value {
		if s == boardSurfaceGitHub {
			return false, &planningError{
				Result:  ResultUnsupportedConfig,
				Reason:  ReasonUnsupportedBoardSurface,
				Message: "board_surfaces contains \"github\", which docket does not publish to in this version; remove it or set board_surfaces to [inline] or []",
			}
		}
	}
	for _, s := range eff.BoardSurfaces.Value {
		if s == boardSurfaceInline {
			return true, nil
		}
	}
	return false, nil
}

// mapOutcome folds a transaction outcome into the protocol-v1 result taxonomy
// and reports whether the outcome was an idempotent replay. refusalKind is the
// result a domain refusal maps to — an operation passes ResultInvalidState for a
// state-shaped refusal or ResultInvalidInput for a request-shaped one.
func mapOutcome(res transaction.Result, err error, refusalKind Result) (Result, bool) {
	switch res.Disposition {
	case transaction.DispositionApplied:
		return ResultApplied, false
	case transaction.DispositionAlreadyApplied:
		return ResultApplied, true
	case transaction.DispositionNoOp:
		return ResultNoOp, false
	case transaction.DispositionContended:
		return ResultContended, false
	case transaction.DispositionRefused:
		return refusalKind, false
	case transaction.DispositionInterrupted:
		return ResultInterrupted, false
	case transaction.DispositionFailed:
		return mapFailure(err), false
	default:
		return ResultInternalError, false
	}
}

// mapFailure maps a failed transaction's typed Failure kind onto the taxonomy.
// A missing or non-Failure error on a failed disposition is a contract
// violation, reported as internal-error.
func mapFailure(err error) Result {
	f, ok := transaction.AsFailure(err)
	if !ok {
		return ResultInternalError
	}
	switch f.Kind {
	case transaction.KindInvalidInput:
		return ResultInvalidInput
	case transaction.KindInvalidState, transaction.KindValidation:
		return ResultInvalidState
	case transaction.KindExternal, transaction.KindUnknownResult:
		return ResultExternalFailed
	case transaction.KindCancelled:
		return ResultInterrupted
	default:
		return ResultInternalError
	}
}
