package workspace

// This file owns PublishRewrite: the narrow, receipt-scoped publication of a
// rewritten (rebased) feature head onto the exact remote feature ref. It is the
// ONE place in the workspace package that may publish a non-fast-forward update,
// and it may do so only through the exact old-value lease the owned rebase
// recorded — never an arbitrary force. The general PublishHead stays unweakened:
// it refuses every non-fast-forward as contended.
//
// The order (spec §"Rebase rewrite publication"):
//
//	1. validate the request and take the per-workspace operation lock;
//	2. read the on-disk receipt beside the manifest; a missing or unreadable
//	   receipt is a refusal, and the on-disk receipt must equal the caller's
//	   expected receipt (decide and act on the SAME copy) and must belong to this
//	   workspace's manifest (matching repo identity and change id) — any mismatch
//	   is a refusal BEFORE any push;
//	3. probe the authoritative remote feature ref; an unobservable remote is
//	   `unknown` (retain — the effect may land later, never forced on a probe that
//	   could not establish the state);
//	4. the remote already at NewHead is `noop` (keyed on the PROMISED state — the
//	   exact rewritten commit at the exact ref — never on the receipt's origin or a
//	   local proxy; learnings: idempotency-keying);
//	5. the remote exactly at the receipt's OrigRemoteHead is force-updated to
//	   NewHead under --force-with-lease=<ref>:<OrigRemoteHead>, then reprobed to
//	   equality: exact NewHead is `published`, an unobservable reprobe is
//	   `unknown`, any other observed commit is `contended`;
//	6. a remote at any other commit (moved off OrigRemoteHead and not at NewHead)
//	   is `contended` with the remote UNTOUCHED — no push is issued, so the lease
//	   is never widened past the exact recorded old value.

import (
	"context"
	"path/filepath"
	"strconv"

	"github.com/danielhanold/docket/internal/gitcli"
)

// rewriteOp is the Op recorded on every PublishRewrite Failure.
const rewriteOp = "publish-rewrite"

// rewriteRemote is the remote a rewrite publishes to. The workspace package
// treats origin as the single publication remote throughout (Prepare and
// PublishHead take it explicitly); a rewrite derives it from that same
// convention because the receipt and manifest carry no remote of their own.
const rewriteRemote gitcli.RemoteName = "origin"

// RewriteOutcome is the closed set of outcomes for a rewrite publication.
type RewriteOutcome string

const (
	RewritePublished RewriteOutcome = "published"
	RewriteNoop      RewriteOutcome = "noop"
	RewriteContended RewriteOutcome = "contended"
	RewriteUnknown   RewriteOutcome = "unknown"
)

// RewriteRequest names the workspace metadata directory (which holds the manifest
// and the receipt beside it), the caller's expected receipt, and the exact
// rewritten head to publish.
type RewriteRequest struct {
	Dir     string
	Receipt RebaseReceipt
	NewHead string
}

// PublishRewrite publishes NewHead onto the exact remote feature ref under the
// receipt's exact old-value lease, idempotently. published, noop, contended, and
// unknown are value outcomes returned with a nil error; every refusal (a missing
// or mismatched receipt, an invalid request, or an inconsistent manifest) and a
// definite push failure are error returns with the remote untouched by the
// refusal.
func (s *Service) PublishRewrite(ctx context.Context, req RewriteRequest) (RewriteOutcome, error) {
	if req.Dir == "" || !filepath.IsAbs(req.Dir) {
		return "", &Failure{Op: rewriteOp, Stage: "validate", Kind: KindInvalidInput, Detail: "workspace directory is not an absolute path"}
	}
	if !validObjectID(gitcli.ObjectID(req.NewHead)) {
		return "", &Failure{Op: rewriteOp, Stage: "validate", Kind: KindInvalidInput, Detail: "new head is not a full object id"}
	}
	newHead := gitcli.ObjectID(req.NewHead)

	// Serialize against concurrent Prepare/Inspect/Publish/Cleanup on this one
	// workspace.
	releaseOp, err := acquireOperationLock(req.Dir)
	if err != nil {
		return "", &Failure{Op: rewriteOp, Stage: "lock", Kind: KindExternal, Detail: "acquiring workspace operation lock", Err: err}
	}
	defer releaseOp()

	// Read the authoritative on-disk receipt: the decision and the action read the
	// SAME copy. A missing or unreadable receipt is a refusal.
	onDisk, found, err := s.ReadRebaseReceipt(ctx, req.Dir)
	if err != nil {
		return "", err
	}
	if !found {
		return "", &Failure{Op: rewriteOp, Stage: "verify", Kind: KindInvalidState, Detail: "no rebase receipt to authorize a rewrite publication"}
	}
	// The caller's expected receipt must match the authoritative on-disk copy, so a
	// rewrite is never published against a receipt that changed under the caller.
	if onDisk != req.Receipt {
		return "", &Failure{Op: rewriteOp, Stage: "verify", Kind: KindInvalidState, Detail: "on-disk receipt does not match the expected receipt"}
	}

	// The receipt must belong to this workspace's manifest.
	m, present, cerr := loadManifest(req.Dir)
	if cerr != nil {
		return "", &Failure{Op: rewriteOp, Stage: "inventory", Kind: KindExternal, Detail: "workspace manifest is unreadable", Err: cerr}
	}
	if !present {
		return "", &Failure{Op: rewriteOp, Stage: "verify", Kind: KindInvalidState, Detail: "no workspace manifest beside the receipt"}
	}
	if onDisk.RepoIdentity != m.CommonDir {
		return "", &Failure{Op: rewriteOp, Stage: "verify", Kind: KindInvalidState, Detail: "receipt repo identity does not match the workspace"}
	}
	if onDisk.ChangeID != strconv.Itoa(int(m.ChangeID)) {
		return "", &Failure{Op: rewriteOp, Stage: "verify", Kind: KindInvalidState, Detail: "receipt change id does not match the workspace"}
	}
	if !validObjectID(gitcli.ObjectID(onDisk.OrigRemoteHead)) {
		return "", &Failure{Op: rewriteOp, Stage: "verify", Kind: KindInvalidState, Detail: "receipt orig remote head is not a full object id"}
	}
	origRemoteHead := gitcli.ObjectID(onDisk.OrigRemoteHead)

	// Resolve the canonical repository from the workspace checkout so the push runs
	// against the repository's shared object store and configured remote.
	repo, err := s.git.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: m.Path})
	if err != nil {
		return "", mapGitFailure(rewriteOp, "inventory", err)
	}
	ref := m.FeatureRef

	// Probe the authoritative remote feature ref. An unobservable remote is
	// unknown — never a clean absence, never a forced overwrite.
	rr, err := s.git.ProbeRemoteBranch(ctx, repo, rewriteRemote, ref)
	if err != nil {
		return RewriteUnknown, nil
	}
	if rr.State == gitcli.RemoteRefFound && rr.Commit == newHead {
		// Keyed on the promised state: the exact rewritten commit already reached the
		// exact ref. Nothing to do; no push.
		return RewriteNoop, nil
	}
	if rr.State != gitcli.RemoteRefFound || rr.Commit != origRemoteHead {
		// The remote is absent, or holds a commit other than the recorded old value
		// and not our new head: divergence. Refuse and push nothing — the lease is
		// never widened past OrigRemoteHead.
		return RewriteContended, nil
	}

	// The remote is exactly at OrigRemoteHead: force-update it to NewHead under the
	// exact recorded lease, then reprobe to equality before reporting published.
	out, perr := s.git.PushLease(ctx, repo, rewriteRemote, ref, newHead, origRemoteHead)
	if perr != nil {
		return s.reprobeRewrite(ctx, repo, ref, newHead, origRemoteHead)
	}
	switch out.Disposition {
	case gitcli.PushApplied:
		return s.reprobeRewrite(ctx, repo, ref, newHead, origRemoteHead)
	case gitcli.PushLeaseLost:
		return RewriteContended, nil
	default:
		// A non-conclusive push: re-derive the remote state from a fresh probe.
		return s.reprobeRewrite(ctx, repo, ref, newHead, origRemoteHead)
	}
}

// reprobeRewrite re-derives the remote state after a rewrite push, within the
// remaining caller budget. An unobservable remote is `unknown` (the effect may
// have landed but cannot be established — never a clean absence); the exact
// intended head is `published`; the remote still exactly at OrigRemoteHead (or
// cleanly absent) means the push did not land — a definite failed error, never a
// false published; any other observed commit is `contended` (the remote moved
// under the lease). It never mutates.
func (s *Service) reprobeRewrite(ctx context.Context, repo gitcli.Repository, ref gitcli.RefName, newHead, origRemoteHead gitcli.ObjectID) (RewriteOutcome, error) {
	rr, err := s.git.ProbeRemoteBranch(ctx, repo, rewriteRemote, ref)
	if err != nil {
		return RewriteUnknown, nil
	}
	if rr.State == gitcli.RemoteRefFound && rr.Commit == newHead {
		return RewritePublished, nil
	}
	if rr.State != gitcli.RemoteRefFound || rr.Commit == origRemoteHead {
		return "", &Failure{Op: rewriteOp, Stage: "push", Kind: KindExternal, Detail: "rewrite push did not land and the remote is unchanged"}
	}
	return RewriteContended, nil
}
