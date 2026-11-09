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
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

// This file is `change link-ticket`: the operation that links one change to
// one ticket in an outside ticket system, replaces that link, or clears it.
// A ticket link is three frontmatter fields stored exactly as given —
// ticket_provider, ticket_id, and ticket_url — and the op never contacts the
// ticket system; it only records what the caller sends.
//
// The request pins the exact record revision, like the lifecycle ops. It runs
// in one of two modes:
//
//   - link: all three fields, each passing domain.ValidateTicket. A record
//     with no ticket fields is linked. A record already carrying this exact
//     link is a no-op (already-linked). A record carrying a different link,
//     or ticket fields that do not form a valid link, is refused
//     (already-linked-elsewhere) unless the request says replace: true.
//     Replace happens only on an explicit request. A human may make that
//     request in plain words ("point it at CLOUD-2 instead"), but an agent
//     never sets replace on its own initiative to get past the refusal.
//   - unlink: unlink: true and no ticket field. Every ticket key the record
//     carries is set to an empty value (an absent key is never inserted); a
//     record with no ticket values is a no-op (nothing-to-unlink).
//
// Status never refuses: a done or killed record in the archive is linked the
// same way as an active one. Each write also refreshes updated:, re-renders
// the artifact block (its Ticket row), and the inline board.

// OperationChangeLinkTicket is the operation key `change link-ticket` records
// in its result envelope and its transaction trailer.
const OperationChangeLinkTicket = "change.link-ticket"

// The closed disposition set `change link-ticket` reports.
const (
	// LinkTicketDispLinked: a record with no ticket fields was linked.
	LinkTicketDispLinked = "linked"
	// LinkTicketDispAlreadyLinked: the record already carries exactly this
	// link; nothing was written.
	LinkTicketDispAlreadyLinked = "already-linked"
	// LinkTicketDispReplaced: an existing link (or a broken set of ticket
	// fields) was replaced on an explicit replace request.
	LinkTicketDispReplaced = "replaced"
	// LinkTicketDispUnlinked: the record's ticket fields were cleared.
	LinkTicketDispUnlinked = "unlinked"
	// LinkTicketDispNothingToUnlink: the record carries no ticket value;
	// nothing was written.
	LinkTicketDispNothingToUnlink = "nothing-to-unlink"
)

// ChangeLinkTicketRequest is the closed, caller-supplied request for one
// link, replace, or unlink. Path and Revision pin the exact submitted record.
// A ticket field counts as given when it is non-empty.
type ChangeLinkTicketRequest struct {
	ChangeID       int    `json:"change_id" docket:"required"`
	Path           string `json:"path" docket:"required"`
	Revision       string `json:"revision" docket:"required"`
	TicketProvider string `json:"ticket_provider,omitempty"`
	TicketID       string `json:"ticket_id,omitempty"`
	TicketURL      string `json:"ticket_url,omitempty"`
	Replace        bool   `json:"replace,omitempty"`
	Unlink         bool   `json:"unlink,omitempty"`
}

// ChangeLinkTicketResult is the protocol-v1 document `change link-ticket`
// returns. Findings is present on every path (marshalled as [] never null).
type ChangeLinkTicketResult struct {
	Envelope
	ID          int             `json:"id,omitempty" docket:"success-only"`
	Disposition string          `json:"disposition,omitempty" docket:"enum=link_ticket_dispositions"`
	Revision    string          `json:"committed_revision,omitempty" docket:"success-only"`
	Findings    []StatusFinding `json:"findings"`
}

// HumanText renders one line on an applied or no-op result; otherwise the
// operation and result, followed by each finding's message on its own
// indented line.
func (r ChangeLinkTicketResult) HumanText() string {
	switch r.Result {
	case ResultApplied, ResultNoOp:
		line := fmt.Sprintf("%s: change %04d %s", r.Operation, r.ID, r.Disposition)
		if r.Revision != "" {
			line += " — " + r.Revision
		}
		return line
	default:
		var b strings.Builder
		fmt.Fprintf(&b, "%s: %s", r.Operation, r.Result)
		for _, f := range r.Findings {
			if f.Message != "" {
				b.WriteString("\n  " + f.Message)
			}
		}
		return b.String()
	}
}

// newChangeLinkTicketResult stamps the envelope and normalizes Findings so the
// array marshals as [] on every path.
func newChangeLinkTicketResult(result Result, r ChangeLinkTicketResult) ChangeLinkTicketResult {
	r.Envelope = NewEnvelope(OperationChangeLinkTicket, result)
	if r.Findings == nil {
		r.Findings = []StatusFinding{}
	}
	return r
}

// changeLinkTicketReceipt is the canonical receipt persisted with a
// link-ticket commit. Field order is alphabetical for the engine's
// canonical-form validator.
type changeLinkTicketReceipt struct {
	Disposition string `json:"disposition"`
	ID          int    `json:"id"`
	Op          string `json:"op"`
}

// validateLinkTicketRequest runs the request checks that need no repository,
// in order, stopping at the first group that finds anything: the pinned
// identity, the unlink/ticket-field conflict, an empty link request, then the
// ticket fields themselves.
func validateLinkTicketRequest(req ChangeLinkTicketRequest) []StatusFinding {
	if findings := validateLifecycleShape("change_id", req.ChangeID, req.Path, req.Revision); len(findings) > 0 {
		return findings
	}
	anyField := req.TicketProvider != "" || req.TicketID != "" || req.TicketURL != ""
	if req.Unlink {
		if anyField || req.Replace {
			return []StatusFinding{lifecycleFinding(FCUnlinkWithTicket,
				"unlink clears the ticket link, so it takes no ticket field and no replace")}
		}
		return nil
	}
	if !anyField {
		return []StatusFinding{lifecycleFinding(FCEmptyTicketRequest,
			"send ticket_provider, ticket_id, and ticket_url to link a ticket, or unlink: true to clear the link")}
	}
	return ticketRequestFindings(req.TicketProvider, req.TicketID, req.TicketURL)
}

// ChangeLinkTicket validates the request, pins authoritative context, and
// drives one exact-revision transaction that links, replaces, or clears the
// change's ticket link. Every failure that predates the transaction (a bad
// request, a github board surface) returns without an engine call.
func ChangeLinkTicket(ctx context.Context, deps PlanningDeps, repoDir string, req ChangeLinkTicketRequest) ChangeLinkTicketResult {
	if findings := validateLinkTicketRequest(req); len(findings) > 0 {
		return newChangeLinkTicketResult(ResultInvalidInput, ChangeLinkTicketResult{Findings: findings})
	}

	pin, err := deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return newChangeLinkTicketResult(result, ChangeLinkTicketResult{Findings: []StatusFinding{lifecycleFinding(FindingCode(reason), err.Error())}})
	}
	eff := pin.Config.Effective

	inline, err := resolveBoardSurface(eff)
	if err != nil {
		if pe, ok := asPlanningError(err); ok {
			return newChangeLinkTicketResult(pe.Result, ChangeLinkTicketResult{Findings: []StatusFinding{lifecycleFinding(FindingCode(pe.Reason), pe.Message)}})
		}
		return newChangeLinkTicketResult(ResultInternalError, ChangeLinkTicketResult{Findings: []StatusFinding{lifecycleFinding(FindingCode(ReasonStatusInternalError), err.Error())}})
	}

	repo, err := deps.Client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})
	if err != nil {
		result, reason := classifyStatusError(ctx, classifyGitFailure(err))
		return newChangeLinkTicketResult(result, ChangeLinkTicketResult{Findings: []StatusFinding{lifecycleFinding(FindingCode(reason), err.Error())}})
	}

	op := changeLinkTicketOp{
		changeID:   req.ChangeID,
		path:       req.Path,
		ticket:     domain.Ticket{Provider: req.TicketProvider, ID: req.TicketID, URL: req.TicketURL},
		replace:    req.Replace,
		unlink:     req.Unlink,
		eff:        eff,
		clock:      deps.Clock,
		inline:     inline,
		link:       linkContextOf(pin),
		changesDir: eff.ChangesDir.Value,
	}
	res, execErr := deps.Engine.Execute(ctx, transaction.Request{
		Repository: repo,
		Remote:     metadataRemote(pin.Layout),
		TargetRef:  metadataRef(pin.Layout),
		Expected: []transaction.EntityExpectation{{
			Path:     gitcli.RepoPath(req.Path),
			Revision: transaction.ExpectedRevision{Kind: transaction.RevisionBlob, ObjectID: gitcli.ObjectID(req.Revision)},
		}},
		Loader:    newPlanningLoader(eff),
		Scope:     changeScope(req.ChangeID, req.Path, false),
		Operation: op,
	})
	return linkTicketResultFromOutcome(req, res, execErr)
}

// linkTicketResultFromOutcome folds a transaction outcome into the result
// document. An applied result takes its disposition from the receipt; a no-op
// (the only empty plans are already-linked and nothing-to-unlink) takes it
// from the request's mode; a refusal is state-shaped and shows each
// refusal's own sentence.
func linkTicketResultFromOutcome(req ChangeLinkTicketRequest, res transaction.Result, execErr error) ChangeLinkTicketResult {
	result, _ := mapOutcome(res, execErr, ResultInvalidState)

	out := ChangeLinkTicketResult{Findings: findingsToStatus(res.Findings)}
	for i, f := range res.Findings {
		if msg := f.Detail["message"]; msg != "" {
			out.Findings[i].Message = msg
		}
	}
	switch result {
	case ResultApplied:
		out.ID = req.ChangeID
		var rec changeLinkTicketReceipt
		if len(res.Receipt) > 0 && json.Unmarshal(res.Receipt, &rec) == nil {
			out.ID = rec.ID
			out.Disposition = rec.Disposition
		}
		out.Revision = string(res.AppliedCommit)
	case ResultNoOp:
		out.ID = req.ChangeID
		out.Disposition = LinkTicketDispAlreadyLinked
		if req.Unlink {
			out.Disposition = LinkTicketDispNothingToUnlink
		}
	}
	r := newChangeLinkTicketResult(result, out)
	r.Failure = failureStatus(res, execErr)
	return r
}

// changeLinkTicketOp is the SemanticOperation the engine drives per attempt.
// Every field is fixed before the transaction; the decision whether to write
// re-runs from the attempt's own fresh state.
type changeLinkTicketOp struct {
	changeID   int
	path       string
	ticket     domain.Ticket
	replace    bool
	unlink     bool
	eff        config.Effective
	clock      transaction.Clock
	inline     bool
	link       render.LinkContext
	changesDir string
}

func (o changeLinkTicketOp) Key() transaction.OperationKey {
	return transaction.OperationKey(OperationChangeLinkTicket)
}

// Plan decides from the attempt's snapshot whether the request writes, and
// with which disposition, then writes the ticket fields, the refreshed
// updated date, the re-rendered artifact block, and the inline board.
func (o changeLinkTicketOp) Plan(ctx context.Context, st transaction.AttemptState) (transaction.MutationPlan, transaction.OperationResult, error) {
	c, out := st.State.Snapshot.Change(domain.ChangeID(o.changeID))
	if out != domain.LookupFound {
		return refuseLifecycle(FCNotFound, fmt.Sprintf("change %04d is not present in the current corpus", o.changeID))
	}
	if c.Path() != o.path {
		return refuseLifecycle(FCPathMismatch,
			fmt.Sprintf("change %04d's record is at %q, not the requested %q", o.changeID, c.Path(), o.path))
	}
	src, ok := st.State.Sources[c.Path()]
	if !ok {
		return refuseLifecycle(FCPathMismatch,
			fmt.Sprintf("no record source loaded at %q for change %04d", c.Path(), o.changeID))
	}

	var (
		disposition string
		values      map[string]document.Value
	)
	if o.unlink {
		if !c.HasTicketFields() {
			return o.noOpPlan(LinkTicketDispNothingToUnlink)
		}
		doc, err := document.Parse(src)
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("link ticket: parsing record: %w", err)
		}
		values = map[string]document.Value{}
		for _, key := range domain.TicketFields {
			if _, present := doc.Field(key); present {
				values[key] = document.Null()
			}
		}
		disposition = LinkTicketDispUnlinked
	} else {
		current, linked := c.Ticket()
		switch {
		case linked && current == o.ticket:
			return o.noOpPlan(LinkTicketDispAlreadyLinked)
		case linked || c.HasTicketFields():
			if !o.replace {
				return refuseLifecycle(FCAlreadyLinkedElsewhere, alreadyLinkedElsewhereMessage(o.changeID, current, linked))
			}
			disposition = LinkTicketDispReplaced
		default:
			disposition = LinkTicketDispLinked
		}
		values = map[string]document.Value{
			domain.TicketFieldProvider: document.String(o.ticket.Provider),
			domain.TicketFieldID:       document.String(o.ticket.ID),
			domain.TicketFieldURL:      document.String(o.ticket.URL),
		}
	}

	// Each field gets its own parse/apply cycle: inserted fields would
	// otherwise collide at the pre-fence insertion point (see
	// changeRelinkOp.Plan).
	intermediate := src
	for _, key := range domain.TicketFields {
		v, ok := values[key]
		if !ok {
			continue
		}
		var err error
		intermediate, err = rewriteTicketFieldBytes(intermediate, key, v)
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("link ticket: patching %s: %w", key, err)
		}
	}
	intermediate, err := upsertFieldBytes(intermediate, "updated", document.String(o.clock.Now().UTC().Format("2006-01-02")))
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("link ticket: stamping updated: %w", err)
	}

	candidate, err := buildGroomCandidate(o.eff, st.State.Documents, c.Path(), intermediate)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, err
	}
	gc, gout := candidate.Change(domain.ChangeID(o.changeID))
	if gout != domain.LookupFound {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("link ticket: mutated record %04d absent from candidate snapshot", o.changeID)
	}
	body, err := render.ArtifactBlockContent(gc, candidate, o.link)
	if err != nil {
		return refuseLifecycle(FCArtifactRenderFailed, err.Error())
	}
	doc2, err := document.Parse(intermediate)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("link ticket: reparsing patched record: %w", err)
	}
	var ps document.PatchSet
	ps.ReplaceBlock("artifacts", body)
	finalBytes, err := doc2.Apply(ps)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("link ticket: writing artifact block: %w", err)
	}

	files := []transaction.FileMutation{
		{Path: gitcli.RepoPath(c.Path()), Kind: transaction.MutationReplace, Bytes: finalBytes},
	}
	if o.inline {
		boardPath := path.Join(o.changesDir, "BOARD.md")
		if err := includeBoard(ctx, st.Tree, boardPath, candidate, boardUnrenderable(st.State, o.changesDir), boardPresentation(o.eff), &files); err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("link ticket: %w", err)
		}
	}

	return o.plan(files, disposition)
}

// rewriteTicketFieldBytes upserts one ticket field into src. A hand-edited
// ticket value the field patcher cannot edit in place — a mapping, a block
// scalar, a multi-line scalar — is rewritten whole: its entry lines are first
// reduced to a valueless "key:" line at the same position, which the patcher
// then fills with v. Replace and unlink clear whatever is present, so the
// ticket-link-invalid remedy works on every shape the decoder accepts.
func rewriteTicketFieldBytes(src []byte, key string, v document.Value) ([]byte, error) {
	doc, err := document.Parse(src)
	if err != nil {
		return nil, err
	}
	if f, ok := doc.Field(key); ok && f.Shape == document.ShapeUnsupported {
		ending := "\n"
		if bytes.HasSuffix(src[f.Entry.Start:f.Entry.End], []byte("\r\n")) {
			ending = "\r\n"
		}
		cleared := make([]byte, 0, len(src))
		cleared = append(cleared, src[:f.Entry.Start]...)
		cleared = append(cleared, key+":"+ending...)
		cleared = append(cleared, src[f.Entry.End:]...)
		src = cleared
	}
	return upsertFieldBytes(src, key, v)
}

// noOpPlan is the empty plan the engine reports as a no-op. It writes no file,
// but the engine still validates the plan, which needs a commit subject and a
// receipt.
func (o changeLinkTicketOp) noOpPlan(disposition string) (transaction.MutationPlan, transaction.OperationResult, error) {
	return o.plan(nil, disposition)
}

// plan assembles the mutation plan for files under disposition: its receipt
// and its commit subject.
func (o changeLinkTicketOp) plan(files []transaction.FileMutation, disposition string) (transaction.MutationPlan, transaction.OperationResult, error) {
	receipt, err := json.Marshal(changeLinkTicketReceipt{Disposition: disposition, ID: o.changeID, Op: OperationChangeLinkTicket})
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("link ticket: encoding receipt: %w", err)
	}
	return transaction.MutationPlan{
		Files:         files,
		CommitSubject: fmt.Sprintf("change %04d ticket %s", o.changeID, disposition),
		Receipt:       receipt,
	}, transaction.OperationResult{}, nil
}

// alreadyLinkedElsewhereMessage explains a refused link over an existing one
// and the only way past it: a replace the human asked for.
func alreadyLinkedElsewhereMessage(id int, current domain.Ticket, linked bool) string {
	what := "ticket fields that do not form a valid link"
	if linked {
		what = fmt.Sprintf("a link to ticket %s (%s)", current.ID, current.URL)
	}
	return fmt.Sprintf("change %04d already carries %s. To replace it, send the request again with replace: true — "+
		"but only when the human asked for the replacement, never on your own initiative.", id, what)
}
