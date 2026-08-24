package app

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

// This file is `finalize closeout`: the atomic terminal metadata transaction
// that turns a verified merge into a done, archived change. It takes no
// caller-supplied `done` boolean and no archive date — it reloads the metadata,
// reprobes the recorded pull request and its merge destination, and derives the
// UTC archive date from the verified GitHub mergedAt, so a false `done` cannot be
// asserted from a stale request.
//
// Three verified terminal shapes:
//
//   1. Ordinary: the merge destination is the integration branch. One metadata
//      transaction applies domain.MarkDone AFTER a merge-commit reachability proof
//      against the freshly-fetched destination tip, stamps updated from the merge
//      date, clears the claim while preserving historical branch/PR fields,
//      relocates the record to the dated archive path (the kill path's relocation
//      idiom: MutationCreate archive + MutationDelete active in one plan), and
//      rerenders the artifact block, every metadata-resident backlink, and the
//      inline board. A renderer/marker/validation/lease/push failure produces no
//      remotely partial outcome. A response-lost success is a no-op keyed on the
//      canonical archive record (the change already reads done).
//
//   2. Stacked-merged: the destination is the change's live stack parent's
//      branch. domain.MarkStackedMerged is applied IN PLACE, any stale
//      finalize-blocked marker is cleared, and the board is rerendered. The change
//      is NOT archived; its feature branch and workspace are retained.
//
//   3. Stack root carry: the destination is the integration branch AND the change
//      roots a stack of stacked-merged descendants. domain.DeriveRootCloseoutSet
//      proves, from the authoritative graph and live PR facts, the chain of merged
//      destinations that carried every descendant into the root. One transaction
//      archives the root plus every proven descendant using the ROOT's merge date
//      for every archive filename, and renders one board over the final
//      population. A single unproven descendant keeps the whole root recoverable —
//      zero descendant writes.
//
// Backlinks across repository modes: the metadata transaction lands first and
// retargets every backlink block resident ON the metadata ref (always the spec;
// in main mode the plan and results too). In docket mode the merged plan and
// results live on the integration ref, so a FOLLOW-UP isolated integration-ref
// transaction patches only their existing docket:backlink blocks under the
// integration ref's exact lease. That second leg is generated-link maintenance,
// not terminal publishing: it copies no metadata record and edits no authored
// bytes. A failed or contended second leg leaves the change truthfully `done` and
// emits a typed terminal-backlink-pending finding; its idempotency is keyed on the
// remote block bytes (a block already pointing at the archive path is a no-op).

// OperationFinalizeCloseout is the operation key the metadata closeout
// transaction records in its result envelope and trailer.
const OperationFinalizeCloseout = "finalize.closeout"

// OperationFinalizeCloseoutBacklink is the operation key the docket-mode
// integration-ref backlink leg records in its trailer.
const OperationFinalizeCloseoutBacklink = "finalize.closeout-backlink"

// The closed set of `finalize closeout` dispositions.
const (
	// CloseoutDispDoneArchived: the change was marked done and relocated to the
	// dated archive path in one transaction.
	CloseoutDispDoneArchived = "done-archived"
	// CloseoutDispStackedMerged: the change's code merged into its live parent's
	// branch; it was marked stacked-merged in place, not archived.
	CloseoutDispStackedMerged = "stacked-merged"
	// CloseoutDispRootArchived: a stack root plus its proven carried descendants
	// were marked done and archived in one transaction.
	CloseoutDispRootArchived = "root-archived"
	// CloseoutDispAlready: the promised terminal state already exists (a response-
	// lost replay); a verified no-op.
	CloseoutDispAlready = "already"
	// CloseoutDispChildrenRetargetRequired: a descendant is not yet stacked-merged,
	// so the root cannot be carried; retarget/finish the descendants first.
	CloseoutDispChildrenRetargetRequired = "children-retarget-required"
	// CloseoutDispContended: a lost race the caller resolves by re-reading context
	// (an unreachable merge commit, a stale record version).
	CloseoutDispContended = "contended"
	// CloseoutDispBlocked: a retained precondition refusal (an unmerged PR, an
	// illegal source status, a destination that is neither the integration branch
	// nor a live parent branch).
	CloseoutDispBlocked = "blocked"
	// CloseoutDispUnknown: an external effect could not be established (a probe or
	// reachability error). Retained; never permits a terminal write.
	CloseoutDispUnknown = "unknown"
	// CloseoutDispFailed: a transaction failure; the cause is in the envelope's
	// failure field.
	CloseoutDispFailed = "failed"
)

// The stable machine reasons `finalize closeout` reports. Message text is
// explanatory and must not be parsed.
const (
	// ReasonCloseoutUnknownChange: an id names no record in the corpus.
	ReasonCloseoutUnknownChange = "unknown-change"
	// ReasonCloseoutAmbiguousID: an id is claimed by more than one record.
	ReasonCloseoutAmbiguousID = "ambiguous-change"
	// ReasonCloseoutIllegalSource: the change's status is not a legal closeout
	// source (only implemented or stacked-merged may be closed out).
	ReasonCloseoutIllegalSource = "illegal-source-status"
	// ReasonCloseoutNotFinalizable: the change carries no canonical PR reference.
	ReasonCloseoutNotFinalizable = "not-finalizable"
	// ReasonCloseoutRepoUnresolved: the GitHub repository identity did not resolve.
	ReasonCloseoutRepoUnresolved = "repository-unresolved"
	// ReasonCloseoutProbeUnknown: the merged reprobe could not be established.
	ReasonCloseoutProbeUnknown = "merge-probe-unknown"
	// ReasonCloseoutNotMerged: the pull request is not merged; there is nothing to
	// close out.
	ReasonCloseoutNotMerged = "pr-not-merged"
	// ReasonCloseoutUnverifiedMerge: the merged facts carry no usable merge commit
	// or merge date.
	ReasonCloseoutUnverifiedMerge = "merge-unverified"
	// ReasonCloseoutDestinationProbe: the destination ref could not be fetched or
	// the reachability proof could not run.
	ReasonCloseoutDestinationProbe = "destination-probe-failed"
	// ReasonCloseoutUnreachable: the reported merge commit is not reachable from the
	// integration tip; the merge is not verified on this destination.
	ReasonCloseoutUnreachable = "merge-commit-unreachable"
	// ReasonCloseoutDestinationMismatch: the merge landed on a branch that is
	// neither the integration branch nor the change's live parent branch.
	ReasonCloseoutDestinationMismatch = "destination-mismatch"
	// ReasonCloseoutChildUnproven: a descendant's carry into the root could not be
	// proven; the root stays recoverable.
	ReasonCloseoutChildUnproven = "descendant-carry-unproven"
	// ReasonCloseoutBacklinkPending: the metadata transaction landed (the change is
	// done) but the follow-up integration-ref backlink leg did not; a retryable
	// health/maintenance finding.
	ReasonCloseoutBacklinkPending = "terminal-backlink-pending"
	// ReasonCloseoutNotesFrozen: the change is already terminal and the request
	// carries notes that differ from the terminal record; refused — a terminal
	// record is never rewritten.
	ReasonCloseoutNotesFrozen = "terminal-notes-frozen"
)

// closeoutBacklinkArtifactHeadings is the marker heading set the closeout record
// splice manages when it clears a stale finalize-blocked section from a record it
// is terminating. It reuses finalize_block.go's finalizeBlockedSectionHeading.
var closeoutBlockedHeadingSet = []string{finalizeBlockedSectionHeading}

// CloseoutResult is the protocol-v1 document `finalize closeout` returns. It
// names identity, the closed disposition, the root archive path and any carried
// descendant ids on a terminal archive, and — on a refusal — a stable reason and
// message. Findings carries validation diagnostics and the retryable
// terminal-backlink-pending finding. It leaks no authored artifact bytes.
type CloseoutResult struct {
	Envelope
	ID          int             `json:"id,omitempty"`
	Disposition string          `json:"disposition,omitempty"`
	ArchivePath string          `json:"archive_path,omitempty"`
	CarriedIDs  []int           `json:"carried_ids,omitempty"`
	Revision    string          `json:"committed_revision,omitempty"`
	Reason      string          `json:"reason,omitempty"`
	Message     string          `json:"message,omitempty"`
	Findings    []StatusFinding `json:"findings"`
}

// HumanText renders a one-line summary naming identity and disposition only —
// never an authored document body.
func (r CloseoutResult) HumanText() string {
	if r.Result == ResultApplied || r.Result == ResultNoOp {
		s := fmt.Sprintf("%s: change %04d %s", r.Operation, r.ID, r.Disposition)
		if r.ArchivePath != "" {
			s += " (" + r.ArchivePath + ")"
		}
		return s
	}
	if r.Reason != "" {
		return fmt.Sprintf("%s: %s (%s)", r.Operation, r.Result, r.Reason)
	}
	return fmt.Sprintf("%s: %s", r.Operation, r.Result)
}

// newCloseoutResult stamps the envelope for the finalize.closeout operation and
// normalizes the findings collection so a nil never leaks into the document.
func newCloseoutResult(result Result, out CloseoutResult) CloseoutResult {
	out.Envelope = NewEnvelope(OperationFinalizeCloseout, result)
	if out.Findings == nil {
		out.Findings = []StatusFinding{}
	}
	return out
}

// closeoutRefusal builds a refusing result carrying a stable reason, message, and
// disposition.
func closeoutRefusal(result Result, disposition, reason, message string, id int) CloseoutResult {
	return newCloseoutResult(result, CloseoutResult{
		ID: id, Disposition: disposition, Reason: reason, Message: message,
	})
}

// closeoutReceipt is the canonical receipt persisted with a closeout commit. Its
// keys are alphabetical so json.Marshal emits the canonical sorted-key compact
// form the engine's receipt validator requires.
type closeoutReceipt struct {
	ArchiveDate string `json:"archive_date"`
	IDs         []int  `json:"ids"`
	Notes       string `json:"notes,omitempty"`
	Op          string `json:"op"`
	Root        int    `json:"root"`
}

// closeoutBacklinkReceipt is the receipt the docket-mode integration-ref backlink
// leg persists — the stable closeout request trailer that keys a lost-response
// recovery alongside the remote block bytes.
type closeoutBacklinkReceipt struct {
	ArchiveDate string `json:"archive_date"`
	Op          string `json:"op"`
	Root        int    `json:"root"`
}

// closeoutTarget is one change the closeout transaction terminates: its resolved
// id, its active record path, its slug, and the archive path it relocates to.
type closeoutTarget struct {
	id          int
	activePath  string
	slug        string
	archivePath string
}

// FinalizeCloseout reloads the metadata, reprobes the recorded PR and its merge
// destination, and applies the one verified terminal shape the destination
// selects. It never asserts done without a merge-commit reachability proof, never
// leaves a remotely partial metadata outcome, and (in docket mode) retargets the
// merged plan/results backlinks in an isolated retryable follow-up leg.
func FinalizeCloseout(ctx context.Context, deps FinalizeDeps, repoDir string, id int, notes CloseoutNotes) CloseoutResult {
	if id <= 0 {
		return newCloseoutResult(ResultInvalidInput, CloseoutResult{
			ID: id, Findings: []StatusFinding{lifecycleFinding("invalid-id", "id must be a positive change id")},
		})
	}

	// Normalize and validate the whole authored-notes set before any external
	// probe or mutation: invalid notes produce no probe and no metadata write.
	notes, noteFindings := normalizeCloseoutNotes(notes)
	if len(noteFindings) != 0 {
		return newCloseoutResult(ResultInvalidInput, CloseoutResult{ID: id, Disposition: CloseoutDispBlocked, Findings: noteFindings})
	}

	cc, refusal := loadCloseoutContext(ctx, deps, repoDir, id)
	if refusal != nil {
		return *refusal
	}

	// Terminal-state short circuits keyed on the promised state (the canonical
	// archive record), never a local proxy. A done change is already closed out; a
	// killed/other terminal record is an illegal source.
	switch cc.change.Status() {
	case domain.StatusDone:
		// Replay is proven against the terminal record's own bytes: the writer is
		// the reader. Empty notes replay any terminal record; identical notes are a
		// byte-level no-op; different notes cannot rewrite a frozen terminal record.
		if match, err := closeoutNotesMatchTerminal(cc.body, notes); err != nil {
			return closeoutRefusal(ResultInvalidState, CloseoutDispBlocked, ReasonCloseoutNotesFrozen, err.Error(), id)
		} else if !match {
			return closeoutRefusal(ResultInvalidState, CloseoutDispBlocked, ReasonCloseoutNotesFrozen,
				fmt.Sprintf("change %04d is already terminal; a retry carrying different notes is not a replay and cannot rewrite the terminal record", id), id)
		}
		return newCloseoutResult(ResultNoOp, CloseoutResult{
			ID: id, Disposition: CloseoutDispAlready, ArchivePath: cc.change.Path(),
			Message: fmt.Sprintf("change %04d is already done and archived", id),
		})
	case domain.StatusImplemented, domain.StatusStackedMerged:
		// Legal closeout sources; fall through.
	default:
		return closeoutRefusal(ResultInvalidState, CloseoutDispBlocked, ReasonCloseoutIllegalSource,
			fmt.Sprintf("change %04d is %q; only an implemented or stacked-merged change may be closed out", id, cc.change.RawStatus()), id)
	}

	// Reprobe the canonical PR authoritatively. Its absence is not-finalizable.
	canonicalN, ok := parsePRNumber(cc.change.PR().Value)
	if !finalizeHasPRRef(cc.change) || !ok {
		return closeoutRefusal(ResultBlocked, CloseoutDispBlocked, ReasonCloseoutNotFinalizable,
			fmt.Sprintf("change %04d carries no canonical pull-request reference to close out", id), id)
	}
	ghRepo, err := deps.GitHub.DiscoverRepository(ctx, repoDir)
	if err != nil {
		return newCloseoutResult(ResultExternalFailed, CloseoutResult{
			ID: id, Disposition: CloseoutDispUnknown, Reason: ReasonCloseoutRepoUnresolved, Message: err.Error(),
		})
	}
	facts, refusal := reprobeMerged(ctx, deps, ghRepo, id, canonicalN)
	if refusal != nil {
		return *refusal
	}

	// Route by the verified merge destination.
	integrationBranch := cc.integrationBranch
	if facts.BaseRef == integrationBranch {
		return closeoutIntegrationDestination(ctx, deps, cc, ghRepo, canonicalN, facts, notes)
	}

	// A stacked change whose destination is its live parent's branch takes the
	// in-place stacked-merged path.
	parent, pout := domain.StackParent(cc.snap, cc.change)
	if pout == domain.LookupFound && !parent.Status().Terminal() {
		parentBranch := domain.BranchForSlug(parent.Slug())
		if facts.BaseRef == parentBranch {
			return closeoutStacked(ctx, deps, cc, parentBranch, facts, notes)
		}
	}

	return closeoutRefusal(ResultBlocked, CloseoutDispBlocked, ReasonCloseoutDestinationMismatch,
		fmt.Sprintf("change %04d merged into %q, which is neither the integration branch nor a live parent branch", id, facts.BaseRef), id)
}

// closeoutNotesMatchTerminal reports whether the terminal record already
// carries exactly the promise this request makes: splicing the request's notes
// into the terminal bytes is a byte-level no-op. Empty notes match any
// terminal record (the pre-notes replay). The comparison uses the same splice
// that writes, so reader and writer can never disagree.
func closeoutNotesMatchTerminal(body []byte, notes CloseoutNotes) (bool, error) {
	if notes.Empty() {
		return true, nil
	}
	respliced, err := spliceCloseoutNotes(body, notes)
	if err != nil {
		return false, err
	}
	return string(respliced) == string(body), nil
}

// closeoutContext is the fresh reload every closeout decision reads from.
type closeoutContext struct {
	pin               StatusPin
	eff               config.Effective
	snap              domain.Snapshot
	change            domain.Change
	version           string
	body              []byte
	blobVersions      map[string]string // record path -> exact blob version
	sources           map[string][]byte // record path -> exact record bytes
	repo              gitcli.Repository
	integrationBranch string
	inline            bool
	link              render.LinkContext
	changesDir        string
}

// loadCloseoutContext pins once, runs the capability preflight before any
// external effect, reads the corpus once, and resolves the change from that one
// authoritative copy (decide-and-act-on-the-same-copy).
func loadCloseoutContext(ctx context.Context, deps FinalizeDeps, repoDir string, id int) (*closeoutContext, *CloseoutResult) {
	reader := deps.Planning.Reader
	pin, err := reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		r := closeoutRefusal(result, CloseoutDispBlocked, reason, err.Error(), id)
		return nil, &r
	}
	if decision := config.PreflightMutation(&pin.Config); !decision.Allowed {
		r := newCloseoutResult(ResultUnsupportedConfig, CloseoutResult{
			ID: id, Reason: ReasonDeferredCapRequested,
			Message: "configuration actively requests a deferred capability docket does not ship in this version (" +
				strings.Join(blockerPaths(decision.Blockers), ", ") + "); withdraw it before any mutation",
		})
		return nil, &r
	}
	eff := pin.Config.Effective

	inline, err := fenceBoardSurface(eff)
	if err != nil {
		if pe, ok := asPlanningError(err); ok {
			r := closeoutRefusal(pe.Result, CloseoutDispBlocked, pe.Reason, pe.Message, id)
			return nil, &r
		}
		r := closeoutRefusal(ResultInternalError, CloseoutDispBlocked, ReasonStatusInternalError, err.Error(), id)
		return nil, &r
	}

	blobs, err := reader.ReadCorpus(ctx, pin)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		r := closeoutRefusal(result, CloseoutDispBlocked, reason, err.Error(), id)
		return nil, &r
	}
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: eff, Documents: inputs})
	if err != nil {
		r := closeoutRefusal(ResultInternalError, CloseoutDispBlocked, ReasonStatusInternalError, err.Error(), id)
		return nil, &r
	}
	snap := build.Snapshot

	c, out := snap.Change(domain.ChangeID(id))
	if out != domain.LookupFound {
		reason, result := ReasonCloseoutUnknownChange, ResultInvalidInput
		msg := fmt.Sprintf("no change %04d is present in the corpus", id)
		if out == domain.LookupAmbiguous {
			reason, result = ReasonCloseoutAmbiguousID, ResultInvalidState
			msg = fmt.Sprintf("more than one record claims change id %04d; refusing to choose", id)
		}
		r := closeoutRefusal(result, CloseoutDispBlocked, reason, msg, id)
		return nil, &r
	}

	blobVersions := make(map[string]string, len(blobs))
	sources := make(map[string][]byte, len(blobs))
	var version string
	var body []byte
	for _, b := range blobs {
		blobVersions[b.Path] = b.Version
		sources[b.Path] = b.Data
		if b.Path == c.Path() {
			version = b.Version
			body = b.Data
		}
	}

	repo, err := deps.Planning.Client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})
	if err != nil {
		result, reason := classifyStatusError(ctx, classifyGitFailure(err))
		r := closeoutRefusal(result, CloseoutDispBlocked, reason, err.Error(), id)
		return nil, &r
	}

	integration := pin.IntegrationBranch
	if integration == "" {
		integration = pin.DefaultBranch
	}

	return &closeoutContext{
		pin: pin, eff: eff, snap: snap, change: c, version: version, body: body,
		blobVersions: blobVersions, sources: sources, repo: repo, integrationBranch: integration,
		inline:     inline,
		link:       linkContextOf(pin),
		changesDir: eff.ChangesDir.Value,
	}, nil
}

// reprobeMerged reprobes one PR authoritatively and validates the merged facts
// carry a usable merge commit and merge date. A probe error is unknown; a
// cleanly-not-merged PR is blocked; a merged PR without a usable commit/date is
// unverified.
func reprobeMerged(ctx context.Context, deps FinalizeDeps, ghRepo githubcli.Repository, id, number int) (githubcli.MergedFacts, *CloseoutResult) {
	outcome, facts, err := deps.GitHub.ProbeMerged(ctx, ghRepo, number)
	if err != nil {
		r := newCloseoutResult(ResultExternalFailed, CloseoutResult{
			ID: id, Disposition: CloseoutDispUnknown, Reason: ReasonCloseoutProbeUnknown, Message: err.Error(),
		})
		return githubcli.MergedFacts{}, &r
	}
	if outcome != githubcli.MergeMerged && outcome != githubcli.MergeAlreadyMerged {
		r := closeoutRefusal(ResultBlocked, CloseoutDispBlocked, ReasonCloseoutNotMerged,
			fmt.Sprintf("pull request #%d is not merged; there is nothing to close out", number), id)
		return githubcli.MergedFacts{}, &r
	}
	if !validFullObjectID(facts.MergeCommit) || strings.TrimSpace(facts.MergedAtUTC) == "" {
		r := newCloseoutResult(ResultExternalFailed, CloseoutResult{
			ID: id, Disposition: CloseoutDispUnknown, Reason: ReasonCloseoutUnverifiedMerge,
			Message: "the merged facts carry no usable merge commit or merge date; retained, no closeout",
		})
		return githubcli.MergedFacts{}, &r
	}
	return facts, nil
}

// closeoutIntegrationDestination handles a merge whose destination is the
// integration branch: it proves reachability, derives the stack-root closeout
// set, and drives the archive transaction (ordinary or root carry) plus the
// docket-mode backlink leg.
func closeoutIntegrationDestination(ctx context.Context, deps FinalizeDeps, cc *closeoutContext, ghRepo githubcli.Repository, canonicalN int, facts githubcli.MergedFacts, notes CloseoutNotes) CloseoutResult {
	id := int(cc.change.ID())

	// Merge-commit reachability from the freshly-fetched integration tip. A fetch
	// or ancestry probe error is unknown; a clean unreachable answer is contended.
	rev, err := deps.Planning.Client.FetchBranch(ctx, cc.repo, originRemote, gitcli.RefName(branchRefPrefix+cc.integrationBranch))
	if err != nil {
		return newCloseoutResult(ResultExternalFailed, CloseoutResult{
			ID: id, Disposition: CloseoutDispUnknown, Reason: ReasonCloseoutDestinationProbe, Message: err.Error(),
		})
	}
	reachable, err := deps.Planning.Client.IsAncestor(ctx, cc.repo, gitcli.ObjectID(facts.MergeCommit), rev.Commit)
	if err != nil {
		return newCloseoutResult(ResultExternalFailed, CloseoutResult{
			ID: id, Disposition: CloseoutDispUnknown, Reason: ReasonCloseoutDestinationProbe, Message: err.Error(),
		})
	}
	if !reachable {
		return closeoutRefusal(ResultContended, CloseoutDispContended, ReasonCloseoutUnreachable,
			"the reported merge commit is not reachable from the integration tip; the merge is not verified on this destination", id)
	}

	// Derive the descendant closeout set from the authoritative graph and live PR
	// facts. A descendant probe error is unknown (retain).
	descFacts, err := probeDescendantFacts(ctx, deps, ghRepo, cc.snap, cc.change.ID())
	if err != nil {
		return newCloseoutResult(ResultExternalFailed, CloseoutResult{
			ID: id, Disposition: CloseoutDispUnknown, Reason: ReasonCloseoutProbeUnknown, Message: err.Error(),
		})
	}
	set, polFail := domain.DeriveRootCloseoutSet(cc.snap, cc.change.ID(), descFacts)
	if polFail != nil {
		return closeoutRefusal(ResultInvalidState, CloseoutDispBlocked, polFail.Reason,
			fmt.Sprintf("change %04d cannot root a closeout set: %s", id, polFail.Reason), id)
	}
	if !domain.RootCloseoutProven(set) {
		disp, reason := closeoutUnprovenDisposition(set)
		return closeoutRefusal(ResultBlocked, disp, reason,
			fmt.Sprintf("change %04d has a descendant whose carry into the root is not proven; the root stays recoverable", id), id)
	}

	archiveDate, ok := archiveDateFromMerge(facts.MergedAtUTC)
	if !ok {
		return newCloseoutResult(ResultExternalFailed, CloseoutResult{
			ID: id, Disposition: CloseoutDispUnknown, Reason: ReasonCloseoutUnverifiedMerge,
			Message: "the merge date did not parse as an RFC3339 timestamp; retained, no closeout",
		})
	}

	// Assemble the ordered target set: root first, then each proven descendant.
	targets := []closeoutTarget{closeoutTargetFor(cc.change, cc.changesDir, archiveDate)}
	for _, d := range set {
		dc, out := cc.snap.Change(d.ID)
		if out != domain.LookupFound {
			continue
		}
		targets = append(targets, closeoutTargetFor(dc, cc.changesDir, archiveDate))
	}

	disposition := CloseoutDispDoneArchived
	var carried []int
	if len(targets) > 1 {
		disposition = CloseoutDispRootArchived
		for _, tg := range targets[1:] {
			carried = append(carried, tg.id)
		}
	}

	res := runCloseoutArchiveTransaction(ctx, deps, cc, targets, archiveDate, disposition, carried, notes)
	if res.Result != ResultApplied {
		return res
	}

	// Docket-mode follow-up: retarget the merged plan/results backlinks on the
	// integration ref. A failed/contended leg leaves the change truthfully done and
	// emits a retryable finding.
	if cc.pin.Mode == "docket" {
		if finding := runCloseoutBacklinkLeg(ctx, deps, cc, targets, archiveDate); finding != nil {
			res.Findings = append(res.Findings, *finding)
		}
	}
	return res
}

// closeoutTargetFor builds a target for one change: its archive path uses the
// supplied (root) merge date so every filename in a root carry shares one date.
func closeoutTargetFor(c domain.Change, changesDir, archiveDate string) closeoutTarget {
	return closeoutTarget{
		id:         int(c.ID()),
		activePath: c.Path(),
		slug:       c.Slug(),
		archivePath: path.Join(changesDir, "archive",
			fmt.Sprintf("%s-%04d-%s.md", archiveDate, int(c.ID()), c.Slug())),
	}
}

// closeoutUnprovenDisposition maps the first descendant refusal token to a
// closeout disposition. A descendant that is not yet stacked-merged needs
// retargeting/finishing (children-retarget-required); an unknown PR is unknown;
// every other structural break is blocked.
func closeoutUnprovenDisposition(set []domain.CarriedDescendant) (string, string) {
	for _, d := range set {
		switch d.Proof {
		case "":
			continue
		case "not-stacked-merged":
			return CloseoutDispChildrenRetargetRequired, ReasonCloseoutChildUnproven
		case "pr-unknown":
			return CloseoutDispUnknown, ReasonCloseoutProbeUnknown
		default:
			return CloseoutDispBlocked, ReasonCloseoutChildUnproven
		}
	}
	return CloseoutDispBlocked, ReasonCloseoutChildUnproven
}

// probeDescendantFacts reprobes the live PR of every transitive stack descendant
// so DeriveRootCloseoutSet reasons over authoritative facts. A descendant with no
// PR reference contributes no facts (DeriveRootCloseoutSet reads that as
// pr-unknown); a probe error is returned so the caller retains it as unknown.
func probeDescendantFacts(ctx context.Context, deps FinalizeDeps, ghRepo githubcli.Repository, snap domain.Snapshot, root domain.ChangeID) (map[domain.ChangeID]domain.PRFacts, error) {
	facts := map[domain.ChangeID]domain.PRFacts{}
	for _, id := range domain.StackDescendantsParentFirst(snap, root) {
		d, out := snap.Change(id)
		if out != domain.LookupFound {
			continue
		}
		number, ok := parsePRNumber(d.PR().Value)
		if !finalizeHasPRRef(d) || !ok {
			continue
		}
		outcome, mf, err := deps.GitHub.ProbeMerged(ctx, ghRepo, number)
		if err != nil {
			return nil, err
		}
		if outcome == githubcli.MergeMerged || outcome == githubcli.MergeAlreadyMerged {
			facts[id] = domain.PRFacts{
				Number: itoa(number), State: "merged", HeadOID: mf.HeadOID, BaseRef: mf.BaseRef,
				MergedAtUTC: mf.MergedAtUTC, MergeCommit: mf.MergeCommit,
			}
			continue
		}
		facts[id] = domain.PRFacts{Number: itoa(number), State: "closed"}
	}
	return facts, nil
}

// closeoutStacked applies MarkStackedMerged in place, clears any stale
// finalize-blocked marker, and rerenders the board — never archiving. A change
// already stacked-merged whose destination is still its parent's branch is a
// verified no-op.
func closeoutStacked(ctx context.Context, deps FinalizeDeps, cc *closeoutContext, parentBranch string, facts githubcli.MergedFacts, notes CloseoutNotes) CloseoutResult {
	id := int(cc.change.ID())
	if cc.change.Status() == domain.StatusStackedMerged {
		// Replay against the terminal in-place record's own bytes: identical notes
		// (or none) are a byte-level no-op; different notes cannot rewrite it.
		if match, err := closeoutNotesMatchTerminal(cc.body, notes); err != nil {
			return closeoutRefusal(ResultInvalidState, CloseoutDispBlocked, ReasonCloseoutNotesFrozen, err.Error(), id)
		} else if !match {
			return closeoutRefusal(ResultInvalidState, CloseoutDispBlocked, ReasonCloseoutNotesFrozen,
				fmt.Sprintf("change %04d is already stacked-merged; a retry carrying different notes is not a replay and cannot rewrite the terminal record", id), id)
		}
		return newCloseoutResult(ResultNoOp, CloseoutResult{
			ID: id, Disposition: CloseoutDispAlready,
			Message: fmt.Sprintf("change %04d is already stacked-merged into %q", id, parentBranch),
		})
	}

	op := closeoutStackedOp{
		id:           id,
		parentBranch: parentBranch,
		destination:  facts.BaseRef,
		notes:        notes,
		eff:          cc.eff,
		inline:       cc.inline,
		changesDir:   cc.changesDir,
	}
	res, execErr := deps.Planning.Engine.Execute(ctx, transaction.Request{
		Repository: cc.repo,
		Remote:     originRemote,
		TargetRef:  gitcli.RefName(branchRefPrefix + metadataBranchOf(cc.pin)),
		Expected: []transaction.EntityExpectation{{
			Path:    gitcli.RepoPath(cc.change.Path()),
			Version: transaction.ExpectedVersion{Kind: transaction.VersionBlob, ObjectID: gitcli.ObjectID(cc.version)},
		}},
		Loader:    newPlanningLoader(cc.eff),
		Operation: op,
	})
	result, _ := mapOutcome(res, execErr, ResultInvalidState)
	out := CloseoutResult{ID: id, Findings: findingsToStatus(res.Findings)}
	switch {
	case res.Disposition == transaction.DispositionFailed:
		out.Disposition = CloseoutDispFailed
	case result == ResultApplied:
		out.Disposition = CloseoutDispStackedMerged
		out.Revision = string(res.AppliedCommit)
	case result == ResultNoOp:
		out.Disposition = CloseoutDispAlready
	case result == ResultContended:
		out.Disposition = CloseoutDispContended
	default:
		out.Disposition = CloseoutDispBlocked
	}
	r := newCloseoutResult(result, out)
	r.Failure = failureStatus(res, execErr)
	return r
}

// runCloseoutArchiveTransaction drives the metadata transaction that marks every
// target done, relocates it into the archive, retargets its metadata-resident
// backlinks, and rerenders the board — all-or-nothing.
func runCloseoutArchiveTransaction(ctx context.Context, deps FinalizeDeps, cc *closeoutContext, targets []closeoutTarget, archiveDate, disposition string, carried []int, notes CloseoutNotes) CloseoutResult {
	id := int(cc.change.ID())

	expectations := make([]transaction.EntityExpectation, 0, len(targets))
	for _, tg := range targets {
		version := cc.blobVersions[tg.activePath]
		expectations = append(expectations, transaction.EntityExpectation{
			Path:    gitcli.RepoPath(tg.activePath),
			Version: transaction.ExpectedVersion{Kind: transaction.VersionBlob, ObjectID: gitcli.ObjectID(version)},
		})
	}

	op := closeoutArchiveOp{
		rootID:      id,
		targets:     targets,
		archiveDate: archiveDate,
		notes:       notes,
		eff:         cc.eff,
		inline:      cc.inline,
		link:        cc.link,
		changesDir:  cc.changesDir,
	}
	res, execErr := deps.Planning.Engine.Execute(ctx, transaction.Request{
		Repository: cc.repo,
		Remote:     originRemote,
		TargetRef:  gitcli.RefName(branchRefPrefix + metadataBranchOf(cc.pin)),
		Expected:   expectations,
		Loader:     newPlanningLoader(cc.eff),
		Operation:  op,
	})
	result, _ := mapOutcome(res, execErr, ResultInvalidState)
	out := CloseoutResult{ID: id, CarriedIDs: carried, Findings: findingsToStatus(res.Findings)}
	if result == ResultApplied {
		out.Disposition = disposition
		out.ArchivePath = targets[0].archivePath
		out.Revision = string(res.AppliedCommit)
	} else {
		switch {
		case res.Disposition == transaction.DispositionFailed:
			out.Disposition = CloseoutDispFailed
		case result == ResultContended:
			out.Disposition = CloseoutDispContended
		default:
			out.Disposition = CloseoutDispBlocked
		}
	}
	r := newCloseoutResult(result, out)
	r.Failure = failureStatus(res, execErr)
	return r
}

// runCloseoutBacklinkLeg retargets the merged plan/results backlinks on the
// integration ref in docket mode. It returns a terminal-backlink-pending finding
// when the leg did not land — the change stays truthfully done and the sweep
// retries the leg — or nil when the leg landed (or had nothing to do).
func runCloseoutBacklinkLeg(ctx context.Context, deps FinalizeDeps, cc *closeoutContext, targets []closeoutTarget, archiveDate string) *StatusFinding {
	backlinkTargets, err := closeoutBacklinkTargets(cc, targets)
	if err != nil {
		return &StatusFinding{Code: ReasonCloseoutBacklinkPending, Severity: string(domain.SeverityWarning), Message: err.Error()}
	}
	if len(backlinkTargets) == 0 {
		return nil
	}

	op := closeoutBacklinkOp{
		rootID:      cc.rootIDOf(targets),
		archiveDate: archiveDate,
		targets:     backlinkTargets,
	}
	res, execErr := deps.Planning.Engine.Execute(ctx, transaction.Request{
		Repository: cc.repo,
		Remote:     originRemote,
		TargetRef:  gitcli.RefName(branchRefPrefix + cc.integrationBranch),
		Loader:     newBacklinkArtifactLoader(backlinkTargets),
		Operation:  op,
	})
	// Best-effort secondary leg: the change stays truthfully done and the sweep
	// retries. The finding carries the transaction's typed cause — the failure's
	// stage/kind/detail, or a refusal's finding codes and paths — so a stuck leg
	// is self-diagnosing (change 0337); after the scoped loader, an in-scope
	// artifact-level problem is the only refusal left, and this names it.
	result, _ := mapOutcome(res, execErr, ResultInvalidState)
	if result == ResultApplied || result == ResultNoOp {
		return nil
	}
	msg := fmt.Sprintf("the change is done, but the integration-ref backlink leg did not land (%s)", result)
	if d := backlinkLegDetail(res, execErr); d != "" {
		msg += ": " + d
	}
	msg += "; the sweep will retry it"
	return &StatusFinding{
		Code:     ReasonCloseoutBacklinkPending,
		Severity: string(domain.SeverityWarning),
		Message:  msg,
	}
}

// rootIDOf returns the root target's id.
func (cc *closeoutContext) rootIDOf(targets []closeoutTarget) int {
	if len(targets) > 0 {
		return targets[0].id
	}
	return 0
}

// closeoutBacklinkTargets renders, per target, the archived backlink interior and
// the plan/results artifact paths that carry a backlink block. It is computed in
// the app layer so the integration-ref operation carries only the exact interior
// bytes and the exact paths, never a metadata record.
func closeoutBacklinkTargets(cc *closeoutContext, targets []closeoutTarget) ([]closeoutBacklinkTarget, error) {
	out := make([]closeoutBacklinkTarget, 0, len(targets))
	for _, tg := range targets {
		c, out2 := cc.snap.Change(domain.ChangeID(tg.id))
		if out2 != domain.LookupFound {
			continue
		}
		var paths []string
		if p := c.Plan().Value; p != "" {
			paths = append(paths, p)
		}
		if p := c.Results().Value; p != "" {
			paths = append(paths, p)
		}
		if len(paths) == 0 {
			continue
		}
		src, ok := cc.sources[tg.activePath]
		if !ok {
			return nil, fmt.Errorf("closeout backlink: no source bytes for change %04d at %q", tg.id, tg.activePath)
		}
		interior, err := archivedBacklinkInterior(cc.eff, tg.archivePath, src, cc.link)
		if err != nil {
			return nil, err
		}
		out = append(out, closeoutBacklinkTarget{artifactPaths: paths, interior: interior})
	}
	return out, nil
}

// archivedBacklinkInterior renders the docket:backlink interior a merged
// artifact must carry after closeout: it points at the change's ARCHIVE path. It
// builds a one-record snapshot at the archive path from the record bytes already
// in hand so render.BacklinkContent renders the canonical line — the exact form
// the metadata-ref spec retarget uses, so both legs stay consistent.
func archivedBacklinkInterior(eff config.Effective, archivePath string, srcBytes []byte, link render.LinkContext) (string, error) {
	doc, err := document.Parse(srcBytes)
	if err != nil {
		return "", fmt.Errorf("closeout backlink: parsing record for %q: %w", archivePath, err)
	}
	build, err := repository.BuildSnapshot(repository.BuildInput{
		Config: eff,
		Documents: []repository.InputDocument{{
			Kind: repository.KindChange, Location: repository.LocationArchive, Path: archivePath, Document: doc,
		}},
	})
	if err != nil {
		return "", fmt.Errorf("closeout backlink: building record snapshot for %q: %w", archivePath, err)
	}
	for _, c := range build.Snapshot.Changes() {
		if c.Path() == archivePath {
			block, err := render.BacklinkContent(c, link)
			if err != nil {
				return "", fmt.Errorf("closeout backlink: rendering backlink for %q: %w", archivePath, err)
			}
			return backlinkInterior(block), nil
		}
	}
	return "", fmt.Errorf("closeout backlink: archived record absent from its own one-record snapshot (%q)", archivePath)
}

// closeoutBacklinkTarget is one change's share of the integration-ref backlink
// leg: the exact plan/results paths to patch and the exact backlink interior to
// write into each.
type closeoutBacklinkTarget struct {
	artifactPaths []string
	interior      string
}

// archiveDateFromMerge parses an RFC3339 mergedAt into a UTC YYYY-MM-DD date.
func archiveDateFromMerge(mergedAt string) (string, bool) {
	tm, err := time.Parse(time.RFC3339, mergedAt)
	if err != nil {
		return "", false
	}
	return tm.UTC().Format("2006-01-02"), true
}

// itoa is a tiny non-allocating-intent integer formatter used where strconv would
// be noise.
func itoa(n int) string { return fmt.Sprintf("%d", n) }

// --- closeoutArchiveOp (metadata transaction) -----------------------------

// closeoutArchiveOp is the SemanticOperation that marks every target done,
// relocates it into the archive, retargets its metadata-resident backlinks, and
// rerenders the board — as one validated atomic plan.
type closeoutArchiveOp struct {
	rootID      int
	targets     []closeoutTarget
	archiveDate string
	notes       CloseoutNotes
	eff         config.Effective
	inline      bool
	link        render.LinkContext
	changesDir  string
}

func (o closeoutArchiveOp) Key() transaction.OperationKey {
	return transaction.OperationKey(OperationFinalizeCloseout)
}

func (o closeoutArchiveOp) Plan(ctx context.Context, st transaction.AttemptState) (transaction.MutationPlan, transaction.OperationResult, error) {
	snap := st.State.Snapshot

	// Phase A: per target, gate the transition through the domain and produce the
	// intermediate (marker-cleared, field-patched) record bytes.
	type prepared struct {
		tg           closeoutTarget
		intermediate []byte
	}
	preps := make([]prepared, 0, len(o.targets))
	relocations := make([]relocation, 0, len(o.targets))
	for _, tg := range o.targets {
		c, out := snap.Change(domain.ChangeID(tg.id))
		if out != domain.LookupFound {
			return refuseCloseout("not-found", fmt.Sprintf("change %04d is not present in the current corpus", tg.id))
		}
		result, fail := domain.MarkDone(c, domain.DoneFacts{ReachableFromIntegration: true})
		if fail != nil {
			return refuseCloseoutPolicy(fail)
		}
		src, ok := st.State.Sources[tg.activePath]
		if !ok {
			return refuseCloseout("path-mismatch", fmt.Sprintf("no record source loaded at %q for change %04d", tg.activePath, tg.id))
		}
		cleared, err := clearFinalizeBlockedSection(src)
		if err != nil {
			return refuseCloseout("marker-clear-failed", err.Error())
		}
		// Notes land ONLY on the root (the explicit change); descendants ride
		// through untouched, so root notes never propagate and a descendant's own
		// authored `## Closeout notes` survives root archival.
		if tg.id == o.rootID {
			cleared, err = spliceCloseoutNotes(cleared, o.notes)
			if err != nil {
				return refuseCloseout("notes-splice-failed", err.Error())
			}
		}
		doc1, err := document.Parse(cleared)
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("closeout: reparsing record %04d: %w", tg.id, err)
		}
		var ps document.PatchSet
		for _, fc := range result.Changed {
			ps.SetField(fc.Field, lifecycleFieldValue(fc.To))
		}
		upsertField(&ps, doc1, "updated", document.String(o.archiveDate))
		intermediate, err := doc1.Apply(ps)
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("closeout: patching record %04d: %w", tg.id, err)
		}
		preps = append(preps, prepared{tg: tg, intermediate: intermediate})
		relocations = append(relocations, relocation{activePath: tg.activePath, archivePath: tg.archivePath, bytes: intermediate})
	}

	// Phase B: build the candidate snapshot reflecting every relocation at once.
	candidate, err := buildCloseoutCandidate(o.eff, st.State.Documents, relocations)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, err
	}

	// Phase C: per target, render the artifact block from the candidate, write the
	// archive record, delete the active path, and retarget metadata-resident
	// backlinks.
	var files []transaction.FileMutation
	for _, p := range preps {
		gc, gout := candidate.Change(domain.ChangeID(p.tg.id))
		if gout != domain.LookupFound {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("closeout: archived record %04d absent from candidate snapshot", p.tg.id)
		}
		body, err := render.ArtifactBlockContent(gc, candidate, o.link)
		if err != nil {
			return refuseCloseout("artifact-render-failed", err.Error())
		}
		doc2, err := document.Parse(p.intermediate)
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("closeout: reparsing patched record %04d: %w", p.tg.id, err)
		}
		var ps2 document.PatchSet
		ps2.ReplaceBlock("artifacts", body)
		finalBytes, err := doc2.Apply(ps2)
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("closeout: writing artifact block for %04d: %w", p.tg.id, err)
		}
		files = append(files,
			transaction.FileMutation{Path: gitcli.RepoPath(p.tg.archivePath), Kind: transaction.MutationCreate, Bytes: finalBytes},
			transaction.FileMutation{Path: gitcli.RepoPath(p.tg.activePath), Kind: transaction.MutationDelete},
		)
		files, err = retargetArtifactBacklinks(ctx, st.Tree, gc, o.link, files)
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, err
		}
	}

	// Phase D: one board over the final population.
	if o.inline {
		boardBytes, err := render.Board(render.BoardInput{Snapshot: candidate})
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("closeout: rendering board: %w", err)
		}
		boardPath := path.Join(o.changesDir, "BOARD.md")
		kind, err := boardMutationKind(ctx, st.Tree, boardPath)
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, err
		}
		files = append(files, transaction.FileMutation{Path: gitcli.RepoPath(boardPath), Kind: kind, Bytes: boardBytes})
	}

	ids := make([]int, 0, len(o.targets))
	for _, tg := range o.targets {
		ids = append(ids, tg.id)
	}
	receipt, err := json.Marshal(closeoutReceipt{ArchiveDate: o.archiveDate, IDs: ids, Notes: closeoutNotesDigest(o.notes), Op: OperationFinalizeCloseout, Root: o.rootID})
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("closeout: encoding receipt: %w", err)
	}

	subject := fmt.Sprintf("change %04d closed out (done, archived)", o.rootID)
	if len(o.targets) > 1 {
		subject = fmt.Sprintf("change %04d stack root closed out (%d records archived)", o.rootID, len(o.targets))
	}
	return transaction.MutationPlan{Files: files, CommitSubject: subject, Receipt: receipt}, transaction.OperationResult{}, nil
}

// --- closeoutStackedOp (in-place) -----------------------------------------

// closeoutStackedOp marks a change stacked-merged in place, clears any stale
// finalize-blocked marker, and rerenders the board. It archives nothing.
type closeoutStackedOp struct {
	id           int
	parentBranch string
	destination  string
	notes        CloseoutNotes
	eff          config.Effective
	inline       bool
	changesDir   string
}

func (o closeoutStackedOp) Key() transaction.OperationKey {
	return transaction.OperationKey(OperationFinalizeCloseout)
}

func (o closeoutStackedOp) Plan(ctx context.Context, st transaction.AttemptState) (transaction.MutationPlan, transaction.OperationResult, error) {
	snap := st.State.Snapshot
	c, out := snap.Change(domain.ChangeID(o.id))
	if out != domain.LookupFound {
		return refuseCloseout("not-found", fmt.Sprintf("change %04d is not present in the current corpus", o.id))
	}
	result, fail := domain.MarkStackedMerged(c, o.parentBranch, domain.MergeFacts{VerifiedDestination: o.destination})
	if fail != nil {
		return refuseCloseoutPolicy(fail)
	}
	src, ok := st.State.Sources[c.Path()]
	if !ok {
		return refuseCloseout("path-mismatch", fmt.Sprintf("no record source loaded at %q for change %04d", c.Path(), o.id))
	}
	cleared, err := clearFinalizeBlockedSection(src)
	if err != nil {
		return refuseCloseout("marker-clear-failed", err.Error())
	}
	cleared, err = spliceCloseoutNotes(cleared, o.notes)
	if err != nil {
		return refuseCloseout("notes-splice-failed", err.Error())
	}
	doc, err := document.Parse(cleared)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("closeout stacked: reparsing record %04d: %w", o.id, err)
	}
	var ps document.PatchSet
	for _, fc := range result.Changed {
		ps.SetField(fc.Field, lifecycleFieldValue(fc.To))
	}
	edited, err := doc.Apply(ps)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("closeout stacked: patching record %04d: %w", o.id, err)
	}

	files := []transaction.FileMutation{
		{Path: gitcli.RepoPath(c.Path()), Kind: transaction.MutationReplace, Bytes: edited},
	}

	// The stacked-merged status is board-visible, so the board must render from a
	// candidate reflecting the edit — not the before-snapshot planInlineBoard reads.
	if o.inline {
		candidate, err := buildInPlaceCandidate(o.eff, st.State.Documents, c.Path(), edited)
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, err
		}
		boardBytes, err := render.Board(render.BoardInput{Snapshot: candidate})
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("closeout stacked: rendering board: %w", err)
		}
		boardPath := path.Join(o.changesDir, "BOARD.md")
		kind, err := boardMutationKind(ctx, st.Tree, boardPath)
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, err
		}
		files = append(files, transaction.FileMutation{Path: gitcli.RepoPath(boardPath), Kind: kind, Bytes: boardBytes})
	}

	receipt, err := json.Marshal(closeoutReceipt{IDs: []int{o.id}, Notes: closeoutNotesDigest(o.notes), Op: OperationFinalizeCloseout, Root: o.id})
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("closeout stacked: encoding receipt: %w", err)
	}
	return transaction.MutationPlan{
		Files:         files,
		CommitSubject: fmt.Sprintf("change %04d stacked-merged into %s", o.id, o.parentBranch),
		Receipt:       receipt,
	}, transaction.OperationResult{}, nil
}

// --- closeoutBacklinkOp (integration-ref leg) -----------------------------

// closeoutBacklinkOp patches the docket:backlink block of each merged plan/results
// artifact on the integration ref to point at the archive path. It declares a
// file only when its block actually changes, so a replay against already-retargeted
// remote bytes is an empty-plan no-op (idempotency keyed on the promised state).
type closeoutBacklinkOp struct {
	rootID      int
	archiveDate string
	targets     []closeoutBacklinkTarget
}

func (o closeoutBacklinkOp) Key() transaction.OperationKey {
	return transaction.OperationKey(OperationFinalizeCloseoutBacklink)
}

func (o closeoutBacklinkOp) Plan(ctx context.Context, st transaction.AttemptState) (transaction.MutationPlan, transaction.OperationResult, error) {
	var files []transaction.FileMutation
	for _, tg := range o.targets {
		for _, p := range tg.artifactPaths {
			original, present, err := readTreeBlob(ctx, st.Tree, p)
			if err != nil {
				return transaction.MutationPlan{}, transaction.OperationResult{}, err
			}
			if !present {
				// The merged artifact is not on the integration ref; nothing to patch.
				continue
			}
			doc, err := document.Parse(original)
			if err != nil {
				return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("closeout backlink: parsing artifact %q: %w", p, err)
			}
			if _, ok := doc.Block(backlinkBlockName); !ok {
				// No managed block to retarget; the operation never conjures one.
				continue
			}
			var ps document.PatchSet
			ps.ReplaceBlock(backlinkBlockName, tg.interior)
			updated, err := doc.Apply(ps)
			if err != nil {
				return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("closeout backlink: retargeting artifact %q: %w", p, err)
			}
			if string(updated) == string(original) {
				continue // already points at the archive path: no-op (promised state)
			}
			files = append(files, transaction.FileMutation{Path: gitcli.RepoPath(p), Kind: transaction.MutationReplace, Bytes: updated})
		}
	}
	if len(files) == 0 {
		return transaction.MutationPlan{}, transaction.OperationResult{}, nil
	}
	receipt, err := json.Marshal(closeoutBacklinkReceipt{ArchiveDate: o.archiveDate, Op: OperationFinalizeCloseoutBacklink, Root: o.rootID})
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("closeout backlink: encoding receipt: %w", err)
	}
	return transaction.MutationPlan{
		Files:         files,
		CommitSubject: fmt.Sprintf("change %04d terminal backlinks retargeted to archive", o.rootID),
		Receipt:       receipt,
	}, transaction.OperationResult{}, nil
}

// --- shared helpers -------------------------------------------------------

// relocation is one record's move from an active path to an archive path with
// its new bytes.
type relocation struct {
	activePath  string
	archivePath string
	bytes       []byte
}

// buildCloseoutCandidate rebuilds the complete snapshot the attempt would see
// after every relocation lands: each relocated record removed from its active
// path and re-placed at its archive path with an archive location; every other
// corpus document reclassified by its path (mirroring the planning loader).
func buildCloseoutCandidate(eff config.Effective, docs map[string]document.Document, relocations []relocation) (domain.Snapshot, error) {
	moved := make(map[string]relocation, len(relocations))
	for _, r := range relocations {
		moved[r.activePath] = r
	}

	paths := make([]string, 0, len(docs))
	for p := range docs {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	inputs := make([]repository.InputDocument, 0, len(docs)+len(relocations))
	for _, p := range paths {
		if _, relocated := moved[p]; relocated {
			continue // dropped; re-added at the archive path below
		}
		kind, loc, ok := classifyCorpusPath(eff, p)
		if !ok {
			continue
		}
		inputs = append(inputs, repository.InputDocument{Kind: kind, Location: loc, Path: p, Document: docs[p]})
	}
	for _, r := range relocations {
		doc, err := document.Parse(r.bytes)
		if err != nil {
			return domain.Snapshot{}, fmt.Errorf("closeout: parsing relocated record %q: %w", r.archivePath, err)
		}
		inputs = append(inputs, repository.InputDocument{
			Kind: repository.KindChange, Location: repository.LocationArchive, Path: r.archivePath, Document: doc,
		})
	}

	build, err := repository.BuildSnapshot(repository.BuildInput{Config: eff, Documents: inputs})
	if err != nil {
		return domain.Snapshot{}, fmt.Errorf("closeout: building candidate snapshot: %w", err)
	}
	return build.Snapshot, nil
}

// buildInPlaceCandidate rebuilds the snapshot with the record at editPath
// replaced by editBytes (its location unchanged) and every other corpus document
// reclassified by its path — the state the stacked-merged board renders from.
func buildInPlaceCandidate(eff config.Effective, docs map[string]document.Document, editPath string, editBytes []byte) (domain.Snapshot, error) {
	paths := make([]string, 0, len(docs))
	for p := range docs {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	inputs := make([]repository.InputDocument, 0, len(docs))
	for _, p := range paths {
		kind, loc, ok := classifyCorpusPath(eff, p)
		if !ok {
			continue
		}
		doc := docs[p]
		if p == editPath {
			parsed, err := document.Parse(editBytes)
			if err != nil {
				return domain.Snapshot{}, fmt.Errorf("closeout: parsing in-place record %q: %w", editPath, err)
			}
			doc = parsed
		}
		inputs = append(inputs, repository.InputDocument{Kind: kind, Location: loc, Path: p, Document: doc})
	}

	build, err := repository.BuildSnapshot(repository.BuildInput{Config: eff, Documents: inputs})
	if err != nil {
		return domain.Snapshot{}, fmt.Errorf("closeout: building in-place candidate snapshot: %w", err)
	}
	return build.Snapshot, nil
}

// retargetArtifactBacklinks retargets, on the metadata tree, the docket:backlink
// block of each of the archived change's spec/plan/results artifacts that is
// present ON that tree and carries a block. A metadata-resident artifact (always
// the spec; in main mode the plan and results too) is retargeted here; an
// integration-resident one (docket-mode plan/results) is absent from st.Tree and
// left to the follow-up leg. It never conjures a block a hand-authored artifact
// lacks, matching the kill path's spec-retarget contract.
func retargetArtifactBacklinks(ctx context.Context, tree transaction.Tree, gc domain.Change, link render.LinkContext, files []transaction.FileMutation) ([]transaction.FileMutation, error) {
	backlink, err := render.BacklinkContent(gc, link)
	if err != nil {
		return nil, fmt.Errorf("closeout: rendering backlink for %04d: %w", int(gc.ID()), err)
	}
	interior := backlinkInterior(backlink)

	for _, p := range artifactPathsOf(gc) {
		bytesAt, present, err := readTreeBlob(ctx, tree, p)
		if err != nil {
			return nil, err
		}
		if !present {
			continue
		}
		doc, err := document.Parse(bytesAt)
		if err != nil {
			return nil, fmt.Errorf("closeout: parsing linked artifact %q: %w", p, err)
		}
		if _, ok := doc.Block(backlinkBlockName); !ok {
			continue
		}
		var ps document.PatchSet
		ps.ReplaceBlock(backlinkBlockName, interior)
		updated, err := doc.Apply(ps)
		if err != nil {
			return nil, fmt.Errorf("closeout: retargeting backlink in %q: %w", p, err)
		}
		if string(updated) == string(bytesAt) {
			continue
		}
		files = append(files, transaction.FileMutation{Path: gitcli.RepoPath(p), Kind: transaction.MutationReplace, Bytes: updated})
	}
	return files, nil
}

// artifactPathsOf returns the change's spec/plan/results pointer paths, empties
// omitted, in the fixed spec, plan, results order.
func artifactPathsOf(c domain.Change) []string {
	var out []string
	if p := c.Spec().Value; p != "" {
		out = append(out, p)
	}
	if p := c.Plan().Value; p != "" {
		out = append(out, p)
	}
	if p := c.Results().Value; p != "" {
		out = append(out, p)
	}
	return out
}

// clearFinalizeBlockedSection removes a stale "## Finalize blocked" section from a
// record body when present, leaving every other byte untouched. A record with no
// such section is returned byte-identical.
func clearFinalizeBlockedSection(src []byte) ([]byte, error) {
	if !namedSectionPresent(src, finalizeBlockedSectionHeading) {
		return src, nil
	}
	return render.ApplySectionEdits(src, closeoutBlockedHeadingSet,
		[]render.SectionEdit{{Heading: finalizeBlockedSectionHeading, Intent: render.SectionRemove}})
}

// refuseCloseout builds a refusing (plan, OperationResult) pair carrying one
// state-shaped finding for the closeout operations' Plan closures.
func refuseCloseout(code, msg string) (transaction.MutationPlan, transaction.OperationResult, error) {
	return transaction.MutationPlan{}, transaction.OperationResult{
		Refused: true,
		Findings: []domain.Finding{{
			Code: code, Severity: domain.SeverityError,
			Entity: domain.EntityRef{Kind: domain.EntityChange},
			Detail: map[string]string{"message": msg},
		}},
	}, nil
}

// refuseCloseoutPolicy folds a domain PolicyFailure into a state-shaped refusal.
func refuseCloseoutPolicy(fail *domain.PolicyFailure) (transaction.MutationPlan, transaction.OperationResult, error) {
	return refuseCloseout(fail.Reason, fmt.Sprintf("change %04d: %s", int(fail.Change), fail.Reason))
}
