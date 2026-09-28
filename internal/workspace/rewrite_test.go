package workspace

import (
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
