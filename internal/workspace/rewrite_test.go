package workspace

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/gitcli"
)

// This file drives PublishRewrite: the narrow, receipt-scoped force-with-lease
// publication of a rewritten (rebased) feature head. PublishRewrite refuses
// without a matching receipt, pushes exactly the caller's NewHead onto the exact
// remote feature ref under a --force-with-lease keyed on the receipt's
// OrigRemoteHead, and reprobes to equality before it reports published. The
// idempotency key is the remote state (learnings: idempotency-keying): a remote
// already at NewHead is a noop, and a remote moved off OrigRemoteHead is
// contended with the remote left untouched — no force beyond the exact lease.

// receiptFor builds a receipt that matches repo/tgt, recording origRemote as both
// the pre-rebase head and the remote head the lease is keyed to, and base as the
// rebase target.
func receiptFor(repo gitcli.Repository, tgt Target, origRemote, base gitcli.ObjectID, attempt string) RebaseReceipt {
	return RebaseReceipt{
		RepoIdentity:   repo.CommonDir,
		ChangeID:       strconv.Itoa(int(tgt.ChangeID)),
		OrigHead:       string(origRemote),
		OrigRemoteHead: string(origRemote),
		BaseRef:        string(tgt.BaseRef),
		BaseHead:       string(base),
		Attempt:        attempt,
		CreatedUTC:     time.Now().UTC().Format(time.RFC3339),
	}
}

// rewriteWorkspaceHead amends the workspace tip into a divergent new commit and
// returns it. The new head shares the base parent but is neither an ancestor nor
// a descendant of the pre-amend head — a genuine history rewrite that only a
// force update can publish.
func rewriteWorkspaceHead(t *testing.T, ws string) gitcli.ObjectID {
	t.Helper()
	writeWorktreeFile(t, ws, "feature.txt", "rewritten feature work\n")
	gitOut(t, ws, "add", "feature.txt")
	gitOut(t, ws, "commit", "-q", "--amend", "--no-edit")
	return gitcli.ObjectID(gitOut(t, ws, "rev-parse", "HEAD"))
}

// TestPublishRewriteLease proves the happy path: with the remote at the receipt's
// OrigRemoteHead, PublishRewrite lands exactly NewHead under the lease and
// reprobes to equality, reporting published.
func TestPublishRewriteLease(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	prepareOK(t, svc, repo, tgt)
	ws := wsPathOf(repo)
	base := gitcli.ObjectID(gitOut(t, ws, "rev-parse", "HEAD"))
	head1 := commitInWorkspace(t, ws, "feature.txt", "feature work\n")

	// Establish the remote feature ref at head1.
	if res, err := publishHead(t, svc, repo, tgt); err != nil || res.Disposition != PublishPublished {
		t.Fatalf("seed publish = %q err=%v; want published", res.Disposition, err)
	}

	// Rewrite the local branch to a divergent new head.
	newHead := rewriteWorkspaceHead(t, ws)
	if newHead == head1 {
		t.Fatalf("fixture: rewrite did not change the head")
	}

	dir := metaDirOf(repo, tgt)
	rec := receiptFor(repo, tgt, head1, base, "attempt-01")
	if err := svc.WriteRebaseReceipt(context.Background(), dir, rec); err != nil {
		t.Fatalf("WriteRebaseReceipt: %v", err)
	}

	outcome, err := svc.PublishRewrite(context.Background(), RewriteRequest{Dir: dir, Receipt: rec, NewHead: string(newHead)})
	if err != nil {
		t.Fatalf("PublishRewrite: %v", err)
	}
	if outcome != RewritePublished {
		t.Errorf("outcome = %q; want published", outcome)
	}
	if got, ok := originFeatCommit(t, r); !ok || got != newHead {
		t.Errorf("origin feat ref = %q (ok=%v); want rewritten head %q", got, ok, newHead)
	}
}

// TestPublishRewriteLeaseWithGatePair proves publish still authorizes a rewrite
// from a receipt that carries the gate-continuation pair: the on-disk receipt and
// the caller's expected receipt are the same value, pair included, so the
// equality gate holds (every field is a scalar string; the whole value compares).
func TestPublishRewriteLeaseWithGatePair(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	prepareOK(t, svc, repo, tgt)
	ws := wsPathOf(repo)
	base := gitcli.ObjectID(gitOut(t, ws, "rev-parse", "HEAD"))
	head1 := commitInWorkspace(t, ws, "feature.txt", "feature work\n")

	// Establish the remote feature ref at head1.
	if res, err := publishHead(t, svc, repo, tgt); err != nil || res.Disposition != PublishPublished {
		t.Fatalf("seed publish = %q err=%v; want published", res.Disposition, err)
	}

	// Rewrite the local branch to a divergent new head.
	newHead := rewriteWorkspaceHead(t, ws)
	if newHead == head1 {
		t.Fatalf("fixture: rewrite did not change the head")
	}

	dir := metaDirOf(repo, tgt)
	rec := receiptFor(repo, tgt, head1, base, "attempt-01")
	rec.GateDriveID = "drive-01"
	rec.GateOwnerGeneration = "gen-01"
	if err := svc.WriteRebaseReceipt(context.Background(), dir, rec); err != nil {
		t.Fatalf("WriteRebaseReceipt: %v", err)
	}

	outcome, err := svc.PublishRewrite(context.Background(), RewriteRequest{Dir: dir, Receipt: rec, NewHead: string(newHead)})
	if err != nil {
		t.Fatalf("PublishRewrite: %v", err)
	}
	if outcome != RewritePublished {
		t.Errorf("outcome = %q; want published", outcome)
	}
	if got, ok := originFeatCommit(t, r); !ok || got != newHead {
		t.Errorf("origin feat ref = %q (ok=%v); want rewritten head %q", got, ok, newHead)
	}
}

// TestPublishRewriteNoop proves the idempotency key is the remote state: a remote
// already holding NewHead (a completed rewrite / adopted lost response) is a noop
// with no push issued, even though the remote no longer sits at OrigRemoteHead.
func TestPublishRewriteNoop(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	prepareOK(t, svc, repo, tgt)
	ws := wsPathOf(repo)
	base := gitcli.ObjectID(gitOut(t, ws, "rev-parse", "HEAD"))
	head1 := commitInWorkspace(t, ws, "feature.txt", "feature work\n")
	newHead := rewriteWorkspaceHead(t, ws)

	// The rewrite already reached the remote out of band; the local branch is at
	// newHead, so a force push publishes exactly it.
	gitOut(t, ws, "push", "-f", "-q", "origin", "feat/"+prepSlug)
	before, ok := originFeatCommit(t, r)
	if !ok || before != newHead {
		t.Fatalf("fixture: origin feat ref = %q (ok=%v); want %q", before, ok, newHead)
	}

	dir := metaDirOf(repo, tgt)
	rec := receiptFor(repo, tgt, head1, base, "attempt-01")
	if err := svc.WriteRebaseReceipt(context.Background(), dir, rec); err != nil {
		t.Fatalf("WriteRebaseReceipt: %v", err)
	}

	outcome, err := svc.PublishRewrite(context.Background(), RewriteRequest{Dir: dir, Receipt: rec, NewHead: string(newHead)})
	if err != nil {
		t.Fatalf("PublishRewrite: %v", err)
	}
	if outcome != RewriteNoop {
		t.Errorf("outcome = %q; want noop", outcome)
	}
	if after, _ := originFeatCommit(t, r); after != before {
		t.Errorf("origin feat ref changed on a noop: before=%q after=%q", before, after)
	}
}

// TestPublishRewriteContention proves a remote moved off OrigRemoteHead (and not
// at NewHead) is contended with the remote untouched: no push is issued, so the
// interloper survives — the lease is never widened past the exact old value.
func TestPublishRewriteContention(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	prepareOK(t, svc, repo, tgt)
	ws := wsPathOf(repo)
	base := gitcli.ObjectID(gitOut(t, ws, "rev-parse", "HEAD"))
	head1 := commitInWorkspace(t, ws, "feature.txt", "feature work\n")
	if res, err := publishHead(t, svc, repo, tgt); err != nil || res.Disposition != PublishPublished {
		t.Fatalf("seed publish = %q err=%v; want published", res.Disposition, err)
	}
	newHead := rewriteWorkspaceHead(t, ws)

	// A third party force-pushes a divergent commit onto the feature ref.
	gitOut(t, r.Writer, "checkout", "-q", "-B", "feat/"+prepSlug, "origin/main")
	writeWorktreeFile(t, r.Writer, "divergent.txt", "divergent work\n")
	gitOut(t, r.Writer, "add", "-A")
	gitOut(t, r.Writer, "commit", "-q", "-m", "divergent")
	divergent := gitcli.ObjectID(gitOut(t, r.Writer, "rev-parse", "HEAD"))
	gitOut(t, r.Writer, "push", "-f", "-q", "origin", "feat/"+prepSlug)
	gitOut(t, r.Writer, "checkout", "-q", "main")

	dir := metaDirOf(repo, tgt)
	rec := receiptFor(repo, tgt, head1, base, "attempt-01")
	if err := svc.WriteRebaseReceipt(context.Background(), dir, rec); err != nil {
		t.Fatalf("WriteRebaseReceipt: %v", err)
	}

	outcome, err := svc.PublishRewrite(context.Background(), RewriteRequest{Dir: dir, Receipt: rec, NewHead: string(newHead)})
	if err != nil {
		t.Fatalf("PublishRewrite: %v", err)
	}
	if outcome != RewriteContended {
		t.Errorf("outcome = %q; want contended", outcome)
	}
	if got, _ := originFeatCommit(t, r); got != divergent {
		t.Errorf("origin feat ref = %q; want unchanged divergent %q", got, divergent)
	}
}

// TestPublishRewriteRefusesWithoutReceipt proves the receipt gate: a missing
// receipt, a receipt whose repo identity does not match the workspace, and a
// receipt whose change id does not match the workspace are each refused with an
// error before any push, leaving the remote uncreated.
func TestPublishRewriteRefusesWithoutReceipt(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	prepareOK(t, svc, repo, tgt)
	ws := wsPathOf(repo)
	base := gitcli.ObjectID(gitOut(t, ws, "rev-parse", "HEAD"))
	head1 := commitInWorkspace(t, ws, "feature.txt", "feature work\n")
	newHead := rewriteWorkspaceHead(t, ws)
	dir := metaDirOf(repo, tgt)
	ctx := context.Background()

	// (a) No receipt on disk at all.
	rec := receiptFor(repo, tgt, head1, base, "attempt-01")
	if _, err := svc.PublishRewrite(ctx, RewriteRequest{Dir: dir, Receipt: rec, NewHead: string(newHead)}); err == nil {
		t.Errorf("missing receipt: PublishRewrite = nil error; want refusal")
	}

	// (b) Receipt present but its repo identity is a different repository.
	wrongRepo := receiptFor(repo, tgt, head1, base, "attempt-01")
	wrongRepo.RepoIdentity = filepath.Join(t.TempDir(), "other-common.git")
	if err := svc.WriteRebaseReceipt(ctx, dir, wrongRepo); err != nil {
		t.Fatalf("WriteRebaseReceipt(wrongRepo): %v", err)
	}
	if _, err := svc.PublishRewrite(ctx, RewriteRequest{Dir: dir, Receipt: wrongRepo, NewHead: string(newHead)}); err == nil {
		t.Errorf("wrong repo identity: PublishRewrite = nil error; want refusal")
	}

	// (c) Receipt present but its change id is a different change.
	wrongChange := receiptFor(repo, tgt, head1, base, "attempt-01")
	wrongChange.ChangeID = "999"
	if err := svc.WriteRebaseReceipt(ctx, dir, wrongChange); err != nil {
		t.Fatalf("WriteRebaseReceipt(wrongChange): %v", err)
	}
	if _, err := svc.PublishRewrite(ctx, RewriteRequest{Dir: dir, Receipt: wrongChange, NewHead: string(newHead)}); err == nil {
		t.Errorf("wrong change id: PublishRewrite = nil error; want refusal")
	}

	// (d) On-disk receipt does not match the caller's expected receipt.
	onDisk := receiptFor(repo, tgt, head1, base, "attempt-on-disk")
	if err := svc.WriteRebaseReceipt(ctx, dir, onDisk); err != nil {
		t.Fatalf("WriteRebaseReceipt(onDisk): %v", err)
	}
	mismatched := onDisk
	mismatched.Attempt = "attempt-expected"
	if _, err := svc.PublishRewrite(ctx, RewriteRequest{Dir: dir, Receipt: mismatched, NewHead: string(newHead)}); err == nil {
		t.Errorf("receipt/caller mismatch: PublishRewrite = nil error; want refusal")
	}

	if _, ok := originFeatCommit(t, r); ok {
		t.Errorf("origin feat ref created despite every refusal; no push may have been issued")
	}
}

// TestPublishRewriteUnknownRetains proves an unobservable remote yields unknown
// with no mutation: the effect is retained for a later attempt, never forced on a
// probe that could not establish the remote state.
func TestPublishRewriteUnknownRetains(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	prepareOK(t, svc, repo, tgt)
	ws := wsPathOf(repo)
	base := gitcli.ObjectID(gitOut(t, ws, "rev-parse", "HEAD"))
	head1 := commitInWorkspace(t, ws, "feature.txt", "feature work\n")
	newHead := rewriteWorkspaceHead(t, ws)

	dir := metaDirOf(repo, tgt)
	rec := receiptFor(repo, tgt, head1, base, "attempt-01")
	if err := svc.WriteRebaseReceipt(context.Background(), dir, rec); err != nil {
		t.Fatalf("WriteRebaseReceipt: %v", err)
	}

	// Break the origin URL so the remote probe cannot establish the ref state.
	gitOut(t, r.Primary, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "nonexistent.git"))

	outcome, err := svc.PublishRewrite(context.Background(), RewriteRequest{Dir: dir, Receipt: rec, NewHead: string(newHead)})
	if err != nil {
		t.Fatalf("PublishRewrite: %v", err)
	}
	if outcome != RewriteUnknown {
		t.Errorf("outcome = %q; want unknown", outcome)
	}
}

// TestGeneralPublishStillRefusesRewrite proves the general PublishHead is not
// weakened by the rewrite path: a divergent local head (a rewrite of the
// published remote head) is still refused as contended, never force-published —
// only the receipt-scoped PublishRewrite may force, and only under its lease.
func TestGeneralPublishStillRefusesRewrite(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	prepareOK(t, svc, repo, tgt)
	ws := wsPathOf(repo)
	head1 := commitInWorkspace(t, ws, "feature.txt", "feature work\n")
	if res, err := publishHead(t, svc, repo, tgt); err != nil || res.Disposition != PublishPublished {
		t.Fatalf("seed publish = %q err=%v; want published", res.Disposition, err)
	}
	before, ok := originFeatCommit(t, r)
	if !ok || before != head1 {
		t.Fatalf("fixture: origin feat ref = %q (ok=%v); want %q", before, ok, head1)
	}

	// Rewrite the local branch; the general publish must refuse, not force.
	rewriteWorkspaceHead(t, ws)
	res, err := publishHead(t, svc, repo, tgt)
	if err != nil {
		t.Fatalf("PublishHead: %v", err)
	}
	if res.Disposition != PublishContended {
		t.Errorf("Disposition = %q; want contended (general publish must never force a rewrite)", res.Disposition)
	}
	if after, _ := originFeatCommit(t, r); after != before {
		t.Errorf("origin feat ref changed: before=%q after=%q; general publish force-published a rewrite", before, after)
	}
}
