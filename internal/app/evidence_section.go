package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/evidence"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

// This file owns the change record's operation-owned "## Build evidence"
// section: the durable home of a change's build evidence. The section body is
// exactly the evidence codec's build-evidence block (evidence.Render), so the
// bytes a reader extracts are the bytes the PR description used to carry. The
// section is never authored: change.groom's owned-heading allowlist
// (render.ChangeOwnedHeadings) does not contain it.

// buildEvidenceHeading is the exact H2 line of the section.
const buildEvidenceHeading = "## Build evidence"

// recordEvidenceSection returns the section body, whether it is present, and an
// error for a duplicated heading. Headings inside fenced code are content
// (namedSectionBody scans fence-aware).
func recordEvidenceSection(record []byte) (string, bool, error) {
	return namedSectionBody(record, buildEvidenceHeading)
}

// ReadRecordEvidence extracts the record's build evidence. An absent section is
// evidence.ErrMissing; a duplicated section or a malformed block is an error.
func ReadRecordEvidence(record []byte) (evidence.Record, error) {
	body, present, err := recordEvidenceSection(record)
	if err != nil {
		return evidence.Record{}, fmt.Errorf("record evidence: %w", err)
	}
	if !present {
		return evidence.Record{}, evidence.ErrMissing
	}
	return evidence.Extract([]byte(body))
}

// VerifyRecordEvidence is evidence.Verify scoped to the record's section: missing
// when absent, malformed when the section is duplicated or its block unreadable.
func VerifyRecordEvidence(record []byte, head string) evidence.Verdict {
	body, present, err := recordEvidenceSection(record)
	if err != nil {
		return evidence.VerdictMalformed
	}
	if !present {
		return evidence.VerdictMissing
	}
	return evidence.Verify([]byte(body), head)
}

// UpsertRecordEvidence makes rec (green or skipped) the whole body of the
// record's "## Build evidence" section, appending the section at EOF when it is
// absent and preserving every other byte. The record is re-normalized through
// the codec constructors, rendered in the record's own line ending, and the
// candidate must read back as exactly rec before any bytes are returned.
func UpsertRecordEvidence(record []byte, rec evidence.Record) ([]byte, error) {
	var norm evidence.Record
	var err error
	switch rec.Result {
	case evidence.ResultGreen:
		norm, err = evidence.NewRecord(rec.Command, rec.Head, rec.RanAt)
	case evidence.ResultSkipped:
		norm, err = evidence.NewSkippedRecord(rec.Head, rec.RanAt)
	default:
		return nil, fmt.Errorf("record evidence: result %q is neither green nor skipped", rec.Result)
	}
	if err != nil {
		return nil, err
	}
	doc, err := document.Parse(record)
	if err != nil {
		return nil, fmt.Errorf("record evidence: malformed record: %w", err)
	}
	block := evidence.Render(norm)
	if le := doc.LineEnding(); le != "" && le != "\n" {
		block = strings.ReplaceAll(block, "\n", le)
	}
	out, err := render.ApplySectionEdits(record, []string{buildEvidenceHeading},
		[]render.SectionEdit{{Heading: buildEvidenceHeading, Intent: render.SectionReplace, Markdown: block}})
	if err != nil {
		return nil, fmt.Errorf("record evidence: %w", err)
	}
	got, err := ReadRecordEvidence(out)
	if err != nil {
		return nil, fmt.Errorf("record evidence: candidate failed reparse: %w", err)
	}
	if got != norm {
		return nil, fmt.Errorf("record evidence: candidate reparsed to a different record")
	}
	return out, nil
}

// recordEvidenceFacts is the record-section reader the finalize gates share: the
// recorded head and command, and whether the record is green. Any read failure
// — absent, duplicated, malformed — reads as no green evidence.
func recordEvidenceFacts(record []byte) (head, command string, green bool) {
	rec, err := ReadRecordEvidence(record)
	if err != nil {
		return "", "", false
	}
	return rec.Head, rec.Command, rec.Result == evidence.ResultGreen
}

// recordEvidenceOutcome is the folded result of one record-evidence write.
type recordEvidenceOutcome struct {
	Result   Result // ResultApplied, ResultNoOp, ResultContended, or a failure class
	Reason   string
	Message  string
	Revision string // the applied metadata commit, when applied
}

// recordBuildEvidence writes rec into change id's "## Build evidence" section in
// one exact-revision metadata transaction keyed on the record blob it reads: a
// record that moves between this read and the transaction is contended and left
// untouched, never overwritten. An unchanged section is a clean no-op (no
// commit), so a replay of the same record converges without a second write.
func recordBuildEvidence(ctx context.Context, deps PlanningDeps, repoDir, opKey string, id int, rec evidence.Record) recordEvidenceOutcome {
	pin, err := deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return recordEvidenceOutcome{Result: result, Reason: reason, Message: err.Error()}
	}
	eff := pin.Config.Effective
	inline, err := resolveBoardSurface(eff)
	if err != nil {
		if pe, ok := asPlanningError(err); ok {
			return recordEvidenceOutcome{Result: pe.Result, Reason: pe.Reason, Message: pe.Message}
		}
		return recordEvidenceOutcome{Result: ResultInternalError, Reason: ReasonStatusInternalError, Message: err.Error()}
	}
	_, recPath, revision, refusal := resolveImplementedChange(ctx, deps, pin, eff, id)
	if refusal != nil {
		out := recordEvidenceOutcome{Result: refusal.Result}
		if len(refusal.Findings) > 0 {
			out.Reason, out.Message = refusal.Findings[0].Code, refusal.Findings[0].Message
		}
		return out
	}
	repo, err := deps.Client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})
	if err != nil {
		result, reason := classifyStatusError(ctx, classifyGitFailure(err))
		return recordEvidenceOutcome{Result: result, Reason: reason, Message: err.Error()}
	}
	res, execErr := deps.Engine.Execute(ctx, transaction.Request{
		Repository: repo,
		Remote:     metadataRemote(pin.Layout),
		TargetRef:  metadataRef(pin.Layout),
		Expected: []transaction.EntityExpectation{{
			Path:     gitcli.RepoPath(recPath),
			Revision: transaction.ExpectedRevision{Kind: transaction.RevisionBlob, ObjectID: gitcli.ObjectID(revision)},
		}},
		Loader:    newPlanningLoader(eff),
		Scope:     changeScope(id, recPath, false),
		Operation: recordEvidenceOp{opKey: opKey, changeID: id, rec: rec, eff: eff, inline: inline, changesDir: eff.ChangesDir.Value},
	})
	result, _ := mapOutcome(res, execErr, ResultInvalidState)
	out := recordEvidenceOutcome{Result: result}
	switch result {
	case ResultApplied:
		out.Revision = string(res.AppliedCommit)
	case ResultNoOp:
	case ResultContended:
		out.Message = fmt.Sprintf("change %04d's record moved under the build-evidence write; re-read context and retry", id)
	default:
		if len(res.Findings) > 0 {
			out.Reason = res.Findings[0].Code
			out.Message = res.Findings[0].Detail["message"]
		}
		if out.Message == "" && execErr != nil {
			out.Message = execErr.Error()
		}
		if out.Message == "" {
			out.Message = fmt.Sprintf("the build-evidence write for change %04d did not apply (%s)", id, res.Disposition)
		}
	}
	return out
}

// recordEvidenceOp is the SemanticOperation recordBuildEvidence drives. It edits
// only the change record's "## Build evidence" section. No board-visible field
// changes, so the inline board re-render (every change-record mutator renders
// it) declares no board mutation unless the committed board was already stale.
// An unchanged section is the engine's clean no-op path.
type recordEvidenceOp struct {
	opKey      string
	changeID   int
	rec        evidence.Record
	eff        config.Effective
	inline     bool
	changesDir string
}

func (o recordEvidenceOp) Key() transaction.OperationKey { return transaction.OperationKey(o.opKey) }

func (o recordEvidenceOp) Plan(ctx context.Context, st transaction.AttemptState) (transaction.MutationPlan, transaction.OperationResult, error) {
	c, out := st.State.Snapshot.Change(domain.ChangeID(o.changeID))
	if out != domain.LookupFound {
		return refuseLifecycle(FindingCode(ReasonImplementedUnknownChange), fmt.Sprintf("change %04d is not a single record in the current corpus", o.changeID))
	}
	src, ok := st.State.Sources[c.Path()]
	if !ok {
		return refuseLifecycle(FCPathMismatch, fmt.Sprintf("no record source loaded at %q for change %04d", c.Path(), o.changeID))
	}
	updated, err := UpsertRecordEvidence(src, o.rec)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, err
	}
	// Receipt fields are alphabetical: the engine's receipt validator requires
	// the sorted compact form.
	receipt, err := json.Marshal(struct {
		Head string `json:"head"`
		ID   int    `json:"id"`
		Op   string `json:"op"`
	}{Head: o.rec.Head, ID: o.changeID, Op: o.opKey})
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, err
	}
	var files []transaction.FileMutation
	if !bytes.Equal(updated, src) {
		files = append(files, transaction.FileMutation{Path: gitcli.RepoPath(c.Path()), Kind: transaction.MutationReplace, Bytes: updated})
	}
	if o.inline {
		candidate, err := buildGroomCandidate(o.eff, st.State.Documents, c.Path(), updated)
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, err
		}
		boardPath := path.Join(o.changesDir, "BOARD.md")
		if err := includeBoard(ctx, st.Tree, boardPath, candidate, boardUnrenderable(st.State, o.changesDir), boardPresentation(o.eff), &files); err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("record evidence: %w", err)
		}
	}
	return transaction.MutationPlan{
		Files:         files,
		CommitSubject: fmt.Sprintf("change %04d build evidence recorded", o.changeID),
		Receipt:       receipt,
	}, transaction.OperationResult{}, nil
}
