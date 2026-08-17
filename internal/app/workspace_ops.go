package app

import (
	"context"
	"fmt"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/workspace"
)

// This file is the `workspace prepare`, `workspace inspect`, and
// `workspace publish` operations: the thin app-layer wiring that reloads the
// authoritative snapshot, resolves the change's effective base through the
// domain, builds a validated workspace.Target, and delegates the Git mechanics
// to the landed workspace.Service. The app layer decides NO base-selection or
// stack policy of its own — every base comes from domain.ResolveEffectiveBase,
// every disposition comes straight from the service, and `contended`/`unknown`
// pass through verbatim (no force, no retry-with-force).
//
// Prepare additionally gates the delegation on the change being in-progress at
// the exact submitted version — the claim happens first, so a losing claimant
// creates no workspace. Publish reinspects the workspace's current head and
// refuses when it differs from the caller's expected head, before any push.
//
// The service is injected through WorkspaceDeps so unit tests drive a fake
// seam; the real-git integration coverage exercises workspace.Service itself.

// The operation keys the three workspace operations record in their envelopes.
const (
	OperationWorkspacePrepare = "workspace.prepare"
	OperationWorkspaceInspect = "workspace.inspect"
	OperationWorkspacePublish = "workspace.publish"
)

// The stable machine reasons the workspace operations report for their typed
// refusals. Message text is explanatory and must not be parsed.
const (
	// ReasonWorkspaceUnknownChange is returned when an id names no record.
	ReasonWorkspaceUnknownChange = "unknown-change"
	// ReasonWorkspaceAmbiguousID is returned when an id is claimed by more than
	// one record: the operation refuses to choose.
	ReasonWorkspaceAmbiguousID = "ambiguous-change"
	// ReasonWorkspaceNotInProgress is returned by prepare when the change is not
	// in-progress: the claim must land first, so a workspace is never allocated
	// for an unclaimed change.
	ReasonWorkspaceNotInProgress = "not-in-progress"
	// ReasonWorkspaceVersionMismatch is returned by prepare when the record no
	// longer carries the submitted version — the caller lost a race and must not
	// overwrite; it maps to a contended outcome.
	ReasonWorkspaceVersionMismatch = "version-mismatch"
	// ReasonWorkspaceUnresolvedBase is returned when the change's effective base
	// does not resolve to a single branch (a killed/missing/cyclic parent, or a
	// live parent whose remote branch is absent).
	ReasonWorkspaceUnresolvedBase = "unresolved-base"
	// ReasonWorkspaceMalformedTarget is returned when the resolved identity and
	// base do not form a valid workspace.Target.
	ReasonWorkspaceMalformedTarget = "malformed-target"
	// ReasonWorkspaceHeadMismatch is returned by publish when the workspace's
	// reinspected head differs from the caller's expected head: nothing is pushed.
	ReasonWorkspaceHeadMismatch = "head-mismatch"
)

// WorkspaceIDRequest is the closed request for prepare and inspect. Version is
// required by prepare (the exact record blob the claim receipt reported) and is
// ignored by inspect.
type WorkspaceIDRequest struct {
	ID      int    `json:"id"`
	Version string `json:"version,omitempty"`
}

// WorkspacePublishRequest is the closed request for publish: the change and the
// exact local head the caller expects the ready workspace to carry.
type WorkspacePublishRequest struct {
	ID   int    `json:"id"`
	Head string `json:"head"`
}

// WorkspaceService is the seam the workspace operations delegate their Git
// mechanics to. *workspace.Service satisfies it; unit tests inject a fake.
type WorkspaceService interface {
	Prepare(ctx context.Context, req workspace.PrepareRequest) (workspace.Workspace, error)
	Inspect(ctx context.Context, req workspace.InspectRequest) (workspace.Inspection, error)
	PublishHead(ctx context.Context, req workspace.PublishRequest) (workspace.PublishResult, error)
}

// WorkspaceDeps carries the workspace-service seam, kept separate from
// PlanningDeps so the operation composes the read-only planning seams with the
// workspace engine without folding one into the other.
type WorkspaceDeps struct {
	Service WorkspaceService
}

// WorkspaceOpResult is the protocol-v1 document the three workspace operations
// return. It reports the canonical workspace facts (path, refs, base commit,
// head) and the closed disposition/state kind straight from the service; a typed
// refusal carries a stable reason and explanatory message instead.
type WorkspaceOpResult struct {
	Envelope
	ID          int    `json:"id,omitempty"`
	Path        string `json:"path,omitempty"`
	FeatureRef  string `json:"feature_ref,omitempty"`
	BaseRef     string `json:"base_ref,omitempty"`
	BaseCommit  string `json:"base_commit,omitempty"`
	Head        string `json:"head,omitempty"`
	RemoteHead  string `json:"remote_head,omitempty"`
	Disposition string `json:"disposition,omitempty"`
	State       string `json:"state,omitempty"`
	Reason      string `json:"reason,omitempty"`
	Message     string `json:"message,omitempty"`
}

// HumanText renders the one-line human summary. It names identity, disposition,
// and the canonical facts only — never an authored document body.
func (r WorkspaceOpResult) HumanText() string {
	if r.Result == ResultApplied || r.Result == ResultNoOp {
		disp := r.Disposition
		if disp == "" {
			disp = r.State
		}
		return fmt.Sprintf("%s: change %04d %s at %s (base %s@%s head %s)",
			r.Operation, r.ID, disp, r.Path, r.BaseRef, shortCommit(r.BaseCommit), shortCommit(r.Head))
	}
	if r.Reason != "" {
		return fmt.Sprintf("%s: %s (%s)", r.Operation, r.Result, r.Reason)
	}
	return fmt.Sprintf("%s: %s", r.Operation, r.Result)
}

// newWorkspaceResult stamps the envelope for opKey.
func newWorkspaceResult(opKey string, result Result, out WorkspaceOpResult) WorkspaceOpResult {
	out.Envelope = NewEnvelope(opKey, result)
	return out
}

// workspaceContext is everything a workspace operation reads before it can build
// a target and delegate: the resolved change, its exact record version, the
// effective base, and the discovered repository.
type workspaceContext struct {
	change  domain.Change
	version string
	base    domain.EffectiveBase
	repo    gitcli.Repository
}

// loadWorkspaceContext performs the shared read every workspace operation needs:
// pin authoritative context once, read and build the snapshot, resolve the named
// change (a typed unknown/ambiguous refusal otherwise), fetch the branch facts
// effective-base resolution consults, resolve the base, and discover the
// repository. It returns a non-nil result pointer for every pre-delegation
// refusal. The base is returned as its tagged value: a caller that requires a
// resolved base checks its kind.
func loadWorkspaceContext(ctx context.Context, deps PlanningDeps, repoDir string, id int, opKey string) (workspaceContext, *WorkspaceOpResult) {
	pin, err := deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		r := newWorkspaceResult(opKey, result, WorkspaceOpResult{ID: id, Reason: reason, Message: err.Error()})
		return workspaceContext{}, &r
	}
	eff := pin.Config.Effective

	blobs, err := deps.Reader.ReadCorpus(ctx, pin)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		r := newWorkspaceResult(opKey, result, WorkspaceOpResult{ID: id, Reason: reason, Message: err.Error()})
		return workspaceContext{}, &r
	}
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: eff, Documents: inputs})
	if err != nil {
		r := newWorkspaceResult(opKey, ResultInternalError, WorkspaceOpResult{ID: id, Reason: ReasonStatusInternalError, Message: err.Error()})
		return workspaceContext{}, &r
	}
	snap := build.Snapshot

	c, out := snap.Change(domain.ChangeID(id))
	if out != domain.LookupFound {
		reason, result := ReasonWorkspaceUnknownChange, ResultInvalidInput
		msg := fmt.Sprintf("no change %04d is present in the corpus", id)
		if out == domain.LookupAmbiguous {
			reason, result = ReasonWorkspaceAmbiguousID, ResultInvalidState
			msg = fmt.Sprintf("more than one record claims change id %04d; refusing to choose", id)
		}
		r := newWorkspaceResult(opKey, result, WorkspaceOpResult{ID: id, Reason: reason, Message: msg})
		return workspaceContext{}, &r
	}

	version := ""
	for _, b := range blobs {
		if b.Path == c.Path() {
			version = b.Version
			break
		}
	}

	facts, err := deps.Reader.BranchFacts(ctx, pin, stackBranches(snap))
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		r := newWorkspaceResult(opKey, result, WorkspaceOpResult{ID: id, Reason: reason, Message: err.Error()})
		return workspaceContext{}, &r
	}

	repo, err := deps.Client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})
	if err != nil {
		result, reason := classifyStatusError(ctx, classifyGitFailure(err))
		r := newWorkspaceResult(opKey, result, WorkspaceOpResult{ID: id, Reason: reason, Message: err.Error()})
		return workspaceContext{}, &r
	}

	return workspaceContext{
		change:  c,
		version: version,
		base:    domain.ResolveEffectiveBase(snap, c, facts),
		repo:    repo,
	}, nil
}

// resolveWorkspaceTarget builds the validated workspace.Target from a resolved
// change context, refusing when the effective base did not resolve to a single
// branch or the identity/base do not form a valid target.
func resolveWorkspaceTarget(opKey string, wc workspaceContext) (workspace.Target, *WorkspaceOpResult) {
	if wc.base.Kind != domain.BaseResolved {
		r := newWorkspaceResult(opKey, ResultInvalidState, WorkspaceOpResult{
			ID:      int(wc.change.ID()),
			Reason:  ReasonWorkspaceUnresolvedBase,
			Message: fmt.Sprintf("the change's effective base did not resolve to a branch (kind %q)", wc.base.Kind),
		})
		return workspace.Target{}, &r
	}
	target, err := workspace.NewTarget(wc.change.ID(), wc.change.Slug(), wc.base)
	if err != nil {
		r := newWorkspaceResult(opKey, ResultInvalidInput, WorkspaceOpResult{
			ID:      int(wc.change.ID()),
			Reason:  ReasonWorkspaceMalformedTarget,
			Message: err.Error(),
		})
		return workspace.Target{}, &r
	}
	return target, nil
}

// WorkspacePrepare resolves the change's workspace target and delegates to the
// service's ownership-safe, idempotent Prepare. It refuses before any Git work
// unless the change is in-progress at the exact submitted version — the claim
// lands first, so a losing claimant never allocates a workspace.
func WorkspacePrepare(ctx context.Context, deps PlanningDeps, wdeps WorkspaceDeps, repoDir string, req WorkspaceIDRequest) WorkspaceOpResult {
	wc, refusal := loadWorkspaceContext(ctx, deps, repoDir, req.ID, OperationWorkspacePrepare)
	if refusal != nil {
		return *refusal
	}

	// The claim must have landed first: the change is in-progress at the exact
	// version the caller pinned. A non-in-progress change is a state refusal; a
	// version drift is a lost race, reported as contended so the caller stops.
	if wc.change.Status() != domain.StatusInProgress {
		return newWorkspaceResult(OperationWorkspacePrepare, ResultInvalidState, WorkspaceOpResult{
			ID:      req.ID,
			Reason:  ReasonWorkspaceNotInProgress,
			Message: fmt.Sprintf("change %04d is %q, not in-progress; claim it before preparing a workspace", req.ID, wc.change.RawStatus()),
		})
	}
	if wc.version != req.Version {
		return newWorkspaceResult(OperationWorkspacePrepare, ResultContended, WorkspaceOpResult{
			ID:          req.ID,
			Disposition: string(workspace.PrepareContended),
			Reason:      ReasonWorkspaceVersionMismatch,
			Message:     "the change record moved since the submitted version; re-read authoritative context before preparing",
		})
	}

	target, tRefusal := resolveWorkspaceTarget(OperationWorkspacePrepare, wc)
	if tRefusal != nil {
		return *tRefusal
	}

	ws, err := wdeps.Service.Prepare(ctx, workspace.PrepareRequest{
		Repository: wc.repo,
		Remote:     originRemote,
		Target:     target,
	})
	if err != nil {
		return mapWorkspaceFailure(OperationWorkspacePrepare, req.ID, err)
	}
	return prepareResult(OperationWorkspacePrepare, req.ID, ws)
}

// WorkspaceInspect resolves the change's workspace target and returns the
// service's read-only classification. It is read-only: it never fetches,
// repairs, or mutates.
func WorkspaceInspect(ctx context.Context, deps PlanningDeps, wdeps WorkspaceDeps, repoDir string, req WorkspaceIDRequest) WorkspaceOpResult {
	wc, refusal := loadWorkspaceContext(ctx, deps, repoDir, req.ID, OperationWorkspaceInspect)
	if refusal != nil {
		return *refusal
	}
	target, tRefusal := resolveWorkspaceTarget(OperationWorkspaceInspect, wc)
	if tRefusal != nil {
		return *tRefusal
	}

	insp, err := wdeps.Service.Inspect(ctx, workspace.InspectRequest{
		Repository: wc.repo,
		Target:     target,
	})
	if err != nil {
		return mapWorkspaceFailure(OperationWorkspaceInspect, req.ID, err)
	}
	return newWorkspaceResult(OperationWorkspaceInspect, ResultApplied, WorkspaceOpResult{
		ID:          req.ID,
		Path:        insp.Path,
		FeatureRef:  string(target.FeatureRef),
		BaseRef:     string(target.BaseRef),
		BaseCommit:  string(insp.BaseCommit),
		Head:        string(insp.HeadCommit),
		State:       string(insp.Kind),
		Disposition: string(insp.Kind),
	})
}

// WorkspacePublish reinspects the workspace's current head and refuses when it
// differs from the caller's expected head, then delegates to the service's
// idempotent PublishHead. The service's disposition passes through verbatim —
// contended and unknown are never forced or retried-with-force.
func WorkspacePublish(ctx context.Context, deps PlanningDeps, wdeps WorkspaceDeps, repoDir string, req WorkspacePublishRequest) WorkspaceOpResult {
	wc, refusal := loadWorkspaceContext(ctx, deps, repoDir, req.ID, OperationWorkspacePublish)
	if refusal != nil {
		return *refusal
	}
	target, tRefusal := resolveWorkspaceTarget(OperationWorkspacePublish, wc)
	if tRefusal != nil {
		return *tRefusal
	}

	// Reinspect and require the workspace's current head to equal the caller's
	// expected head before publishing; a mismatch is refused, and no push runs.
	insp, err := wdeps.Service.Inspect(ctx, workspace.InspectRequest{
		Repository: wc.repo,
		Target:     target,
	})
	if err != nil {
		return mapWorkspaceFailure(OperationWorkspacePublish, req.ID, err)
	}
	if string(insp.HeadCommit) != req.Head {
		return newWorkspaceResult(OperationWorkspacePublish, ResultInvalidState, WorkspaceOpResult{
			ID:      req.ID,
			Head:    req.Head,
			State:   string(insp.Kind),
			Reason:  ReasonWorkspaceHeadMismatch,
			Message: "the workspace head differs from the expected head; publish nothing",
		})
	}

	res, err := wdeps.Service.PublishHead(ctx, workspace.PublishRequest{
		Repository: wc.repo,
		Remote:     originRemote,
		Target:     target,
	})
	if err != nil {
		return mapWorkspaceFailure(OperationWorkspacePublish, req.ID, err)
	}
	return publishResult(OperationWorkspacePublish, req.ID, target, res)
}

// prepareResult maps a prepare disposition onto the protocol result taxonomy,
// carrying the verified workspace facts. created/resumed are applied work;
// existing is a no-op; blocked is an invalid-state collision; contended is a
// lost race — all returned with the service's disposition verbatim.
func prepareResult(opKey string, id int, ws workspace.Workspace) WorkspaceOpResult {
	var result Result
	switch ws.Disposition {
	case workspace.PrepareCreated, workspace.PrepareResumed:
		result = ResultApplied
	case workspace.PrepareExisting:
		result = ResultNoOp
	case workspace.PrepareBlocked:
		result = ResultInvalidState
	case workspace.PrepareContended:
		result = ResultContended
	default:
		result = ResultInternalError
	}
	return newWorkspaceResult(opKey, result, WorkspaceOpResult{
		ID:          id,
		Path:        ws.Path,
		FeatureRef:  string(ws.FeatureRef),
		BaseRef:     string(ws.BaseRef),
		BaseCommit:  string(ws.BaseCommit),
		Head:        string(ws.HeadCommit),
		Disposition: string(ws.Disposition),
	})
}

// publishResult maps a publish disposition onto the protocol result taxonomy,
// carrying the intended head and observed remote head. published is applied;
// already-published is a no-op; contended is a lost race; unknown is an
// unverified external effect (external-failed) that a caller must reprobe — all
// with the service's disposition verbatim.
func publishResult(opKey string, id int, target workspace.Target, res workspace.PublishResult) WorkspaceOpResult {
	var result Result
	switch res.Disposition {
	case workspace.PublishPublished:
		result = ResultApplied
	case workspace.PublishAlreadyPublished:
		result = ResultNoOp
	case workspace.PublishContended:
		result = ResultContended
	case workspace.PublishUnknown:
		result = ResultExternalFailed
	default:
		result = ResultInternalError
	}
	return newWorkspaceResult(opKey, result, WorkspaceOpResult{
		ID:          id,
		FeatureRef:  string(target.FeatureRef),
		BaseRef:     string(target.BaseRef),
		Head:        string(res.Head),
		RemoteHead:  string(res.Remote),
		Disposition: string(res.Disposition),
	})
}

// mapWorkspaceFailure folds a workspace.Failure onto the protocol taxonomy. The
// Failure's Kind is the stable reason and its Detail is already bounded and
// redacted, so it rides through as the message.
func mapWorkspaceFailure(opKey string, id int, err error) WorkspaceOpResult {
	result := ResultInternalError
	reason := ReasonStatusInternalError
	message := err.Error()
	if f, ok := workspace.AsFailure(err); ok {
		reason = string(f.Kind)
		message = f.Error()
		switch f.Kind {
		case workspace.KindInvalidInput:
			result = ResultInvalidInput
		case workspace.KindInvalidState:
			result = ResultInvalidState
		case workspace.KindExternal, workspace.KindInvalidOutput, workspace.KindTimedOut:
			result = ResultExternalFailed
		case workspace.KindCancelled:
			result = ResultInterrupted
		}
	}
	return newWorkspaceResult(opKey, result, WorkspaceOpResult{ID: id, Reason: reason, Message: message})
}
