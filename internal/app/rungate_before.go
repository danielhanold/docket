package app

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/danielhanold/docket/internal/domain"
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
)

// RunGateBeforeResult is the protocol-v1 document `run gate-before` returns. On
// an armed gate Result is applied and Key names the durable record; on a
// gate-unarmed report Result is still applied (the report line exits 0) and
// Reason carries the stable token. A usage error (bad target) carries a
// non-applied Result and no report line. It never carries authored document
// bodies.
type RunGateBeforeResult struct {
	Envelope
	Armed   bool   `json:"armed"`
	Key     string `json:"key,omitempty"`
	Target  string `json:"target,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// HumanText renders the one report line. An armed gate prints `gate-armed
// <key>`; a gate-unarmed report prints `gate-unarmed <reason-token>`; a usage
// error (a non-applied result) names its reason instead of a report line.
func (r RunGateBeforeResult) HumanText() string {
	if r.Result == ResultApplied {
		if r.Armed {
			return "gate-armed " + r.Key
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
// claim set, captures the dispatch epoch after that read, mints the durable
// record, and returns `gate-armed <key>` — degrading any arming failure to a
// `gate-unarmed <reason>` report line that still exits 0.
func RunGateBefore(ctx context.Context, deps PlanningDeps, repoDir string, target string) RunGateBeforeResult {
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
	// source, through the same corpus read + parse the claim path uses.
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
	for _, c := range build.Snapshot.Changes() {
		if c.Status() == domain.StatusInProgress {
			beforeIDs = append(beforeIDs, int(c.ID()))
		}
	}
	sort.Ints(beforeIDs)

	// (3) Capture the dispatch epoch AFTER the before-read.
	dispatchEpoch := time.Now().Unix()

	// (4) Mint the durable record. Schema and Repo are stamped by the store.
	key, err := MintGateRecord(repoDir, GateRecord{
		Target:        gateBeforeStoredTarget,
		CreatedAt:     createdAt,
		DispatchEpoch: dispatchEpoch,
		BeforeIDs:     beforeIDs,
		Retry:         RetryUnused,
		Disposition:   "gate-armed",
	})
	if err != nil {
		return gateUnarmed(ReasonGateMintFailed)
	}

	// (5) Report the armed gate.
	return newRunGateBeforeResult(ResultApplied, RunGateBeforeResult{
		Armed:  true,
		Key:    key,
		Target: gateBeforeStoredTarget,
	})
}
