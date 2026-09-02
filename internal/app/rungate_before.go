package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/repository"
)

// This file is the `docket run gate-before` operation (change 0334, Task 2): it
// ARMS the implement-next run gate. It re-syncs the metadata worktree to fresh
// origin, reads the current in-progress claim set, captures a dispatch epoch
// AFTER that read, and mints a durable gate record under the git common dir
// (rungate_store.go). Its whole contract is the printed report line:
// `gate-armed <key>` on success, `gate-unarmed <reason-token>` on any failure —
// both exit 0 (learning exit-code-encodes-a-non-failure). Only `implement-next`
// is an accepted target; anything else is a usage error that exits non-zero.
//
// It writes NO metadata: the fresh-origin re-sync and the change-file parse are
// the SAME plumbing the claim path uses (PinContext advances the remote-tracking
// ref; ReadCorpus + parseCorpus + repository.BuildSnapshot yield the snapshot),
// reused rather than reimplemented (learning duplicated-gate-copies-the-whole-
// predicate). The only durable write is the gate record.

// OperationRunGateBefore is the operation key `run gate-before` records in its
// envelope.
const OperationRunGateBefore = "run.gate-before"

// gateBeforeAcceptedTarget is the sole accepted target argument (the workflow
// name, not the agent name); gateBeforeStoredTarget is the canonical agent name
// the durable record stores as its Target — the thing a dispatch actually
// launches. Attribution downstream keys on the workflow the gate brackets, so
// the two spellings are pinned here, not derived.
const (
	gateBeforeAcceptedTarget = "implement-next"
	gateBeforeStoredTarget   = "docket-implement-next"
)

// The stable reason tokens a gate-unarmed line carries. Each names the arming
// step that failed; a consumer keys on the token, never the prose.
const (
	// ReasonGateInvalidTarget: the target argument was not `implement-next`. This
	// is a usage error (non-zero exit), not a gate-unarmed report line.
	ReasonGateInvalidTarget = "invalid-target"
	// ReasonGateSyncFailed: the fresh-origin metadata re-sync (PinContext) failed,
	// so the before-read could not be taken from authoritative state.
	ReasonGateSyncFailed = "sync-failed"
	// ReasonGateChangesUnreadable: the changes corpus could not be read or parsed
	// into a snapshot, so the in-progress claim set is unknown.
	ReasonGateChangesUnreadable = "changes-unreadable"
	// ReasonGateMintFailed: the durable gate record could not be minted (the git
	// common dir was unresolvable, or the write failed).
	ReasonGateMintFailed = "mint-failed"
	// ReasonGateResumeUnverified: a --resume id could not be verified as an
	// already-in-progress change with a valid workspace identity, so the gate
	// refuses to pre-bind attribution to it. No record is minted (change 0359).
	ReasonGateResumeUnverified = "resume-unverified"
	// ReasonGateScopeFailed: the outer recovery scope could not be prepared, so no
	// dispatch context exists to hand the child. No record is minted (change 0359).
	ReasonGateScopeFailed = "scope-failed"
)

// GateScopeDeps carries the outer-scope preparation seam gate-before composes
// so unit tests can fake the durable scope mint. Production wiring (internal/cli/
// run.go) composes gatedrive.OpenStore(<git-common-dir>).PrepareScope; a unit
// test injects a fake that records the request and returns a canned grant.
type GateScopeDeps struct {
	Prepare func(gatedrive.ScopeRequest) (gatedrive.ScopeGrant, error)
}

// gateHashToken returns the sha256 of a raw token as lowercase hex — the single
// boundary at which the printed dispatch context becomes the stored
// ChildContextHash, so the persisted record links to a nested drive's
// GateContextHash without carrying the raw capability. It matches gatedrive's own
// capHash so the two hashes compare equal.
func gateHashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// RunGateBeforeResult is the protocol-v1 document `run gate-before` returns. On
// an armed gate Result is applied and Key names the durable record; on a
// gate-unarmed report Result is still applied (the report line exits 0) and
// Reason carries the stable token. A usage error (bad target) carries a
// non-applied Result and no report line. It never carries authored document
// bodies.
type RunGateBeforeResult struct {
	Envelope
	Armed bool   `json:"armed"`
	Key   string `json:"key,omitempty"`
	// DispatchContext is the outer scope's ChildCapability the parent copies into
	// the implement-next dispatch prompt; a nested drive carries its hash as the
	// GateContextHash. It is NOT secret from the child (change 0359). The parent
	// capability is deliberately absent from this result — it lives only in the
	// 0600-private gate record.
	DispatchContext string `json:"dispatch_context,omitempty"`
	Target          string `json:"target,omitempty"`
	Reason          string `json:"reason,omitempty"`
	Message         string `json:"message,omitempty"`
}

// HumanText renders the one report line. An armed gate prints `gate-armed <key>
// <dispatch-context>`; a gate-unarmed report prints `gate-unarmed
// <reason-token>`; a usage error (a non-applied result) names its reason instead
// of a report line. The parent capability never appears here — only the child
// dispatch context, which is meant for the child.
func (r RunGateBeforeResult) HumanText() string {
	if r.Result == ResultApplied {
		if r.Armed {
			return "gate-armed " + r.Key + " " + r.DispatchContext
		}
		return "gate-unarmed " + r.Reason
	}
	if r.Reason != "" {
		return fmt.Sprintf("%s: %s (%s)", r.Operation, r.Result, r.Reason)
	}
	return fmt.Sprintf("%s: %s", r.Operation, r.Result)
}

// newRunGateBeforeResult stamps the envelope for the operation.
func newRunGateBeforeResult(result Result, out RunGateBeforeResult) RunGateBeforeResult {
	out.Envelope = NewEnvelope(OperationRunGateBefore, result)
	return out
}

// gateUnarmed builds a gate-unarmed report line: a success-shaped envelope
// (exit 0) carrying the stable reason token. It never mints a record.
func gateUnarmed(reason string) RunGateBeforeResult {
	return newRunGateBeforeResult(ResultApplied, RunGateBeforeResult{Armed: false, Reason: reason})
}

// RunGateBefore arms the implement-next run gate. On a bad target it returns a
// usage error (non-zero exit); otherwise it re-syncs, reads the in-progress
// claim set, captures the dispatch epoch after that read, optionally verifies an
// explicit resume id, prepares the OUTER recovery scope (change 0359), mints the
// durable record, and returns `gate-armed <key> <dispatch-context>` — degrading
// any arming failure to a `gate-unarmed <reason>` report line that still exits 0.
//
// resumeID (0 = none) requests explicit resume attribution: the id is pre-bound
// as the record's AttributedID ONLY when it is a verified in-progress change with
// a valid WorkspaceInspect identity — never by a timestamp game. A resumed change
// still sits in the fresh BeforeIDs and that is correct: attribution is already
// bound, so the verdict path never re-derives it.
func RunGateBefore(ctx context.Context, deps PlanningDeps, wdeps WorkspaceDeps, sdeps GateScopeDeps, repoDir string, target string, resumeID int) RunGateBeforeResult {
	if target != gateBeforeAcceptedTarget {
		return newRunGateBeforeResult(ResultInvalidInput, RunGateBeforeResult{
			Reason:  ReasonGateInvalidTarget,
			Message: fmt.Sprintf("unsupported target %q; only %q is an accepted gate target", target, gateBeforeAcceptedTarget),
		})
	}

	// CreatedAt is stamped at the start of the arm; DispatchEpoch is captured
	// AFTER the before-read below, so a claim landing at or after the dispatch is
	// distinguishable from one already present. Both are real wall-clock stamps
	// (never the injected transaction clock): downstream attribution compares
	// DispatchEpoch against a claim's real claimed_at time.
	createdAt := time.Now().Unix()

	// (1) Re-sync the metadata worktree to fresh origin. PinContext advances the
	// remote-tracking ref through a targeted fetch — the same fresh-origin re-sync
	// the claim path performs — and pins the metadata revision.
	pin, err := deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		return gateUnarmed(ReasonGateSyncFailed)
	}

	// (2) Read the in-progress claim set (ids only) from the pinned metadata
	// source, through the same corpus read + parse the claim path uses. The full
	// per-id status is retained so a resume can verify the resume id is genuinely
	// in-progress (never a proposed or implemented id).
	blobs, err := deps.Reader.ReadCorpus(ctx, pin)
	if err != nil {
		return gateUnarmed(ReasonGateChangesUnreadable)
	}
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: pin.Config.Effective, Documents: inputs})
	if err != nil {
		return gateUnarmed(ReasonGateChangesUnreadable)
	}
	var beforeIDs []int
	statusByID := map[int]domain.Status{}
	for _, c := range build.Snapshot.Changes() {
		id := int(c.ID())
		statusByID[id] = c.Status()
		if c.Status() == domain.StatusInProgress {
			beforeIDs = append(beforeIDs, id)
		}
	}
	sort.Ints(beforeIDs)

	// (3) Capture the dispatch epoch AFTER the before-read. A resume never touches
	// this ordering: the resumed change stays in BeforeIDs and DispatchEpoch stays
	// post-read, because attribution is bound by verified identity below, not by a
	// timestamp comparison.
	dispatchEpoch := time.Now().Unix()

	// (4) Verify an explicit resume id, if requested. Attribution is pre-bound ONLY
	// for a change that is genuinely in-progress AND resolves a valid workspace
	// identity (WorkspaceInspect applied). Anything else — a proposed/implemented
	// id, or a failed inspect — is resume-unverified: no record is minted.
	var (
		attributedID  int
		scopeChangeID string
		branch        string
		worktree      string
	)
	if resumeID != 0 {
		if statusByID[resumeID] != domain.StatusInProgress {
			return gateUnarmed(ReasonGateResumeUnverified)
		}
		insp := WorkspaceInspect(ctx, deps, wdeps, repoDir, WorkspaceIDRequest{ID: resumeID})
		if insp.Result != ResultApplied {
			return gateUnarmed(ReasonGateResumeUnverified)
		}
		attributedID = resumeID
		scopeChangeID = strconv.Itoa(resumeID)
		branch = insp.FeatureRef
		worktree = insp.Path
	}

	// (5) Prepare the OUTER recovery scope. The grant's ChildCapability becomes the
	// dispatch context the parent hands the child; its hash links every nested
	// drive to this outer gate. The ParentCapability is retained in the 0600
	// record and never printed. When resuming, the scope's ChangeID is pre-bound to
	// the verified id, and its Branch/Worktree carry the resumed change's identity.
	grant, serr := sdeps.Prepare(gatedrive.ScopeRequest{
		ChangeID: scopeChangeID,
		Branch:   branch,
		Worktree: worktree,
	})
	if serr != nil {
		return gateUnarmed(ReasonGateScopeFailed)
	}

	// (6) Mint the durable record. Schema and Repo are stamped by the store. The
	// ParentCap is persisted (0600) but never surfaces in the result or a report
	// line; ChildContextHash is the sha256 of the printed dispatch context.
	key, err := MintGateRecord(repoDir, GateRecord{
		Target:           gateBeforeStoredTarget,
		CreatedAt:        createdAt,
		DispatchEpoch:    dispatchEpoch,
		BeforeIDs:        beforeIDs,
		AttributedID:     attributedID,
		Retry:            RetryUnused,
		Disposition:      "gate-armed",
		ScopeID:          grant.ScopeID,
		ParentCap:        grant.ParentCapability,
		ChildContextHash: gateHashToken(grant.ChildCapability),
	})
	if err != nil {
		return gateUnarmed(ReasonGateMintFailed)
	}

	// (7) Report the armed gate with its dispatch context.
	return newRunGateBeforeResult(ResultApplied, RunGateBeforeResult{
		Armed:           true,
		Key:             key,
		Target:          gateBeforeStoredTarget,
		DispatchContext: grant.ChildCapability,
	})
}
