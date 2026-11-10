package app

import (
	"context"
	"fmt"
	"slices"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/repository"
)

// This file is the read-only `context ticket` operation: it gathers, from one
// pinned snapshot of the metadata branch, everything an agent needs to write a
// short update to a change's linked ticket — the change's identity and record
// path, its ticket link state, whether a named event has happened (and the
// effective tickets.auto switch for it), the PR and merge facts, and the spec
// and results artifacts with links pinned to the metadata commit. It works for
// any change, active or archived. "Not linked", "partial", "not happened", and
// "private" are reported, not refused: the caller decides what to do with
// them. Only a malformed request or a change id that names no single record is
// refused. It never mutates, never opens a transaction, and never calls a
// ticket system or GitHub.

// OperationContextTicket is the operation key the ticket read records in its
// result envelope.
const OperationContextTicket = "context.ticket"

// Event tokens (also the tickets.auto.<event> keys).
const (
	TicketEventSpecWritten      = "spec_written"
	TicketEventPROpened         = "pr_opened"
	TicketEventPRMerged         = "pr_merged"
	TicketEventResultsCompleted = "results_completed"
)

// Not-happened reasons an event verdict reports.
const (
	ReasonTicketNoSpec             = "no-spec"
	ReasonTicketNoPR               = "no-pr"
	ReasonTicketMergeNotRecorded   = "merge-not-recorded"
	ReasonTicketNotResultsArtifact = "not-results-artifact"
	ReasonTicketNotCompleted       = "not-completed"
)

// Ticket link states.
const (
	TicketStateLinked    = "linked"
	TicketStateNotLinked = "not-linked"
	TicketStatePartial   = "partial"
)

// Refusal reasons. An id claimed by more than one record reuses
// ReasonContextAmbiguousID (invalid-state).
const (
	// ReasonTicketMissingID: --id absent or not positive (invalid-input).
	ReasonTicketMissingID = "missing-id"
	// ReasonTicketUnknownEvent: --event is not exactly one of ticketEvents
	// (invalid-input).
	ReasonTicketUnknownEvent = "unknown-event"
	// ReasonTicketNotFound: no record carries the id (invalid-input).
	ReasonTicketNotFound = "not-found"
)

// Ordered token lists; the skill guard iterates these so the skill and this
// producer cannot drift apart.
var ticketEvents = []string{TicketEventSpecWritten, TicketEventPROpened, TicketEventPRMerged, TicketEventResultsCompleted}
var ticketEventReasons = []string{ReasonTicketNoSpec, ReasonTicketNoPR, ReasonTicketMergeNotRecorded, ReasonTicketNotResultsArtifact, ReasonTicketNotCompleted}
var ticketStates = []string{TicketStateLinked, TicketStateNotLinked, TicketStatePartial}

// TicketContextRequest is the closed request. Event "" is a no-event read.
type TicketContextRequest struct {
	ID    int    `json:"id"`
	Event string `json:"event,omitempty"`
}

// TicketContextChange is the change's identity and its record's repo-relative
// path on the metadata branch.
type TicketContextChange struct {
	ID          int    `json:"id"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Status      string `json:"status"`
	Deliverable string `json:"deliverable"`
	Path        string `json:"path"`
}

// TicketContextTicket is the change's ticket link state; provider, id, and url
// are set only when the state is linked.
type TicketContextTicket struct {
	State    string `json:"state"`
	Provider string `json:"provider,omitempty"`
	ID       string `json:"id,omitempty"`
	URL      string `json:"url,omitempty"`
}

// TicketContextEvent is the verdict for the requested event. Switch is the
// effective tickets.auto.<event> value, reported for automatic posting and
// ignored on request.
type TicketContextEvent struct {
	Name     string `json:"name"`
	Switch   bool   `json:"switch"`
	Happened bool   `json:"happened"`
	Reason   string `json:"reason,omitempty"`
}

// TicketContextMerge is where a merged change landed.
type TicketContextMerge struct {
	DestinationBranch string `json:"destination_branch"`
	InIntegration     bool   `json:"in_integration"`
}

// TicketContextFacts are the PR and merge facts the record carries.
type TicketContextFacts struct {
	PRURL string              `json:"pr_url"`
	Merge *TicketContextMerge `json:"merge,omitempty"`
}

// TicketContextArtifact is one spec or results artifact that exists at the
// metadata commit. URL is pinned to that commit, and empty in a private
// repository or with no repository web URL.
type TicketContextArtifact struct {
	Role string `json:"role"`
	Path string `json:"path"`
	URL  string `json:"url"`
}

// TicketContext is the read's bundle; every fact derives from one pin.
type TicketContext struct {
	MetadataCommit string                  `json:"metadata_commit"`
	Visibility     string                  `json:"visibility"`
	Change         TicketContextChange     `json:"change"`
	Ticket         TicketContextTicket     `json:"ticket"`
	Event          *TicketContextEvent     `json:"event,omitempty"`
	Facts          TicketContextFacts      `json:"facts"`
	Artifacts      []TicketContextArtifact `json:"artifacts"`

	// metadataRef names the metadata branch for the human summary only.
	metadataRef string
}

// TicketContextResult is the protocol-v1 document the operation returns.
type TicketContextResult struct {
	Envelope
	Context *TicketContext `json:"context,omitempty"`
	Reason  string         `json:"reason,omitempty"`
	Message string         `json:"message,omitempty"`
}

// HumanText renders the one-line human summary: identity, revision, ticket
// state, and the event verdict only — never a title, URL, or authored body.
func (r TicketContextResult) HumanText() string {
	if r.Result == ResultApplied && r.Context != nil {
		c := r.Context
		line := fmt.Sprintf("%s: change %04d %s on %s@%s — ticket %s",
			OperationContextTicket, c.Change.ID, c.Change.Slug, c.metadataRef, shortCommit(c.MetadataCommit), c.Ticket.State)
		if e := c.Event; e != nil {
			if e.Happened {
				line += fmt.Sprintf("; event %s happened", e.Name)
			} else {
				line += fmt.Sprintf("; event %s not happened (%s)", e.Name, e.Reason)
			}
		}
		return line
	}
	if r.Reason != "" {
		return fmt.Sprintf("%s: %s (%s)", r.Operation, r.Result, r.Reason)
	}
	return fmt.Sprintf("%s: %s", r.Operation, r.Result)
}

func newTicketContextResult(result Result, reason, message string, bundle *TicketContext) TicketContextResult {
	if bundle != nil && bundle.Artifacts == nil {
		bundle.Artifacts = []TicketContextArtifact{}
	}
	return TicketContextResult{
		Envelope: NewEnvelope(OperationContextTicket, result),
		Context:  bundle,
		Reason:   reason,
		Message:  message,
	}
}

// ContextTicketRead assembles the ticket-update bundle for one change. It is
// read-only: the request is validated before any read, then one pin, one corpus
// read, one snapshot.
func ContextTicketRead(ctx context.Context, deps PlanningDeps, repoDir string, req TicketContextRequest) TicketContextResult {
	if req.ID <= 0 {
		return newTicketContextResult(ResultInvalidInput, ReasonTicketMissingID,
			"a positive change id is required", nil)
	}
	if req.Event != "" && !slices.Contains(ticketEvents, req.Event) {
		return newTicketContextResult(ResultInvalidInput, ReasonTicketUnknownEvent,
			fmt.Sprintf("unknown event %q; want one of %v", req.Event, ticketEvents), nil)
	}

	pin, err := deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return newTicketContextResult(result, reason, err.Error(), nil)
	}
	blobs, err := deps.Reader.ReadCorpus(ctx, pin)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return newTicketContextResult(result, reason, err.Error(), nil)
	}
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: pin.Config.Effective, Documents: inputs})
	if err != nil {
		return newTicketContextResult(ResultInternalError, ReasonStatusInternalError, err.Error(), nil)
	}
	snap := build.Snapshot

	c, out := snap.Change(domain.ChangeID(req.ID))
	switch out {
	case domain.LookupAbsent:
		return newTicketContextResult(ResultInvalidInput, ReasonTicketNotFound,
			fmt.Sprintf("no change %04d is present in the corpus", req.ID), nil)
	case domain.LookupAmbiguous:
		return newTicketContextResult(ResultInvalidState, ReasonContextAmbiguousID,
			fmt.Sprintf("more than one record claims change id %04d; refusing to choose", req.ID), nil)
	}

	commit := metadataRevision(pin)
	links := linkContextOf(pin)
	var artifacts []TicketContextArtifact
	for _, a := range []struct {
		role  string
		field domain.OptionalString
	}{{"spec", c.Spec()}, {"results", c.Results()}} {
		if a.field.State != domain.FieldPresent || a.field.Value == "" {
			continue
		}
		found, err := deps.Reader.ArtifactExists(ctx, pin, sourceMetadata, a.field.Value)
		if err != nil {
			result, reason := classifyStatusError(ctx, err)
			return newTicketContextResult(result, reason, err.Error(), nil)
		}
		if !found {
			continue
		}
		artifacts = append(artifacts, TicketContextArtifact{
			Role: a.role,
			Path: a.field.Value,
			URL:  links.BlobURLAtCommit(a.field.Value, commit),
		})
	}

	bundle := &TicketContext{
		MetadataCommit: commit,
		Visibility:     string(pin.Layout.Mode),
		Change: TicketContextChange{
			ID:          int(c.ID()),
			Slug:        c.Slug(),
			Title:       c.Title(),
			Status:      c.RawStatus(),
			Deliverable: deliverableText(c),
			Path:        c.Path(),
		},
		Ticket:      ticketState(c),
		Facts:       TicketContextFacts{PRURL: presentValue(c.PR()), Merge: ticketMerge(snap, c, pin)},
		Artifacts:   artifacts,
		metadataRef: pin.Layout.MetadataBranch,
	}
	if req.Event != "" {
		bundle.Event = ticketEvent(c, req.Event, pin)
	}
	return newTicketContextResult(ResultApplied, "", "", bundle)
}

// presentValue is a field's value when present, else "".
func presentValue(f domain.OptionalString) string {
	if f.State != domain.FieldPresent {
		return ""
	}
	return f.Value
}

// ticketState classifies the change's ticket link.
func ticketState(c domain.Change) TicketContextTicket {
	if t, ok := c.Ticket(); ok {
		return TicketContextTicket{State: TicketStateLinked, Provider: t.Provider, ID: t.ID, URL: t.URL}
	}
	if c.HasTicketFields() {
		return TicketContextTicket{State: TicketStatePartial}
	}
	return TicketContextTicket{State: TicketStateNotLinked}
}

// prMerged reports whether the change's PR has merged: a done PR change with a
// recorded pr:, or a stacked-merged change.
func prMerged(c domain.Change) bool {
	switch c.Status() {
	case domain.StatusStackedMerged:
		return true
	case domain.StatusDone:
		return c.Deliverable() == domain.DeliverablePR && presentValue(c.PR()) != ""
	}
	return false
}

// ticketMerge reports where a merged change landed, or nil when it has not
// merged. A stacked-merged change landed on its parent's recorded branch ("" when
// the parent does not resolve).
func ticketMerge(snap domain.Snapshot, c domain.Change, pin StatusPin) *TicketContextMerge {
	if !prMerged(c) {
		return nil
	}
	if c.Status() == domain.StatusStackedMerged {
		dest := ""
		if parent, out := domain.StackParent(snap, c); out == domain.LookupFound {
			dest = presentValue(parent.Branch())
		}
		return &TicketContextMerge{DestinationBranch: dest, InIntegration: false}
	}
	integration := pin.IntegrationBranch
	if integration == "" {
		integration = pin.DefaultBranch
	}
	return &TicketContextMerge{DestinationBranch: integration, InIntegration: true}
}

// ticketEvent is the verdict for one (already validated) event token.
func ticketEvent(c domain.Change, name string, pin StatusPin) *TicketContextEvent {
	auto := pin.Config.Effective.Tickets.Auto
	e := &TicketContextEvent{Name: name}
	hasPR := presentValue(c.PR()) != ""
	switch name {
	case TicketEventSpecWritten:
		e.Switch = auto.SpecWritten.Value
		if presentValue(c.Spec()) == "" {
			e.Reason = ReasonTicketNoSpec
		}
	case TicketEventPROpened:
		e.Switch = auto.PROpened.Value
		if !hasPR {
			e.Reason = ReasonTicketNoPR
		}
	case TicketEventPRMerged:
		e.Switch = auto.PRMerged.Value
		switch {
		case prMerged(c):
		case !hasPR:
			e.Reason = ReasonTicketNoPR
		default:
			e.Reason = ReasonTicketMergeNotRecorded
		}
	case TicketEventResultsCompleted:
		e.Switch = auto.ResultsCompleted.Value
		switch {
		case c.Deliverable() != domain.DeliverableResultsArtifact:
			e.Reason = ReasonTicketNotResultsArtifact
		case c.Status() != domain.StatusDone:
			e.Reason = ReasonTicketNotCompleted
		}
	}
	e.Happened = e.Reason == ""
	return e
}
