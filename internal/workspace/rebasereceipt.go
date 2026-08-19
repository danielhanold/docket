package workspace

// This file owns the rebase receipt: a small ownership-scoped effect record
// written beside the manifest that proves which owned rewrite may be continued,
// aborted, or published. It is an EFFECT RECEIPT, never a phase machine — it
// records the exact object identities (pre-rebase head, the remote head the
// eventual lease is keyed to, the base ref and its head) and an opaque attempt
// token, so a resuming or publishing run can prove it is acting on the same
// rewrite it began rather than a different head.
//
// The read is strictly three-outcome (found / cleanly absent / error): a
// not-exist on the exact receipt path is the only clean absence; a decode or
// validation failure, or any other read error, is an error — a corrupt or
// unreadable receipt NEVER reads as a clean absence (learnings:
// probe-error-is-not-clean-absence). The write is crash-safe: a same-directory
// temp file, fsync, chmod, atomic rename, and directory fsync, mirroring
// writeManifest's discipline so the receipt lands whole or not at all and never
// leaves a stray sibling behind.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/danielhanold/docket/internal/gitcli"
)

const (
	// rebaseReceiptOp is the Op recorded on every receipt Failure.
	rebaseReceiptOp = "rebase-receipt"
	// rebaseReceiptFileName is the receipt file published beside the manifest.
	rebaseReceiptFileName = "rebase-receipt.json"
)

// RebaseReceipt is the owned rebase's effect record. Every field is a scalar
// string so the whole value is comparable, letting a publishing run assert the
// on-disk receipt is byte-for-byte the one it expects. RepoIdentity is the
// repository's canonical common directory; ChangeID is the decimal change id;
// OrigHead is the pre-rebase local head; OrigRemoteHead is the remote feature
// head the rewrite lease is keyed to; BaseRef/BaseHead are the rebase target;
// Attempt is an opaque token distinguishing one rewrite attempt from another.
type RebaseReceipt struct {
	RepoIdentity   string `json:"repo_identity"`
	ChangeID       string `json:"change_id"`
	OrigHead       string `json:"orig_head"`
	OrigRemoteHead string `json:"orig_remote_head"`
	BaseRef        string `json:"base_ref"`
	BaseHead       string `json:"base_head"`
	Attempt        string `json:"attempt"`
	CreatedUTC     string `json:"created_utc"`
}

// validateRebaseReceipt rejects every malformed field so an invalid receipt is
// never written and a corrupt one is never returned as valid. The heads are full
// object ids, BaseRef is a qualified branch ref, RepoIdentity is an absolute
// path, and ChangeID/Attempt are non-empty. It is the single gate both the write
// (before publishing) and the read (after decoding) pass through, so a receipt
// that reaches disk and one that reads back obey identical rules.
func validateRebaseReceipt(r RebaseReceipt) error {
	if r.RepoIdentity == "" || !filepath.IsAbs(r.RepoIdentity) {
		return fmt.Errorf("repo identity is not an absolute path")
	}
	if r.ChangeID == "" {
		return fmt.Errorf("empty change id")
	}
	if !validObjectID(gitcli.ObjectID(r.OrigHead)) {
		return fmt.Errorf("invalid orig head")
	}
	if !validObjectID(gitcli.ObjectID(r.OrigRemoteHead)) {
		return fmt.Errorf("invalid orig remote head")
	}
	if err := validBranchRef(gitcli.RefName(r.BaseRef)); err != nil {
		return fmt.Errorf("invalid base ref: %w", err)
	}
	if !validObjectID(gitcli.ObjectID(r.BaseHead)) {
		return fmt.Errorf("invalid base head")
	}
	if r.Attempt == "" {
		return fmt.Errorf("empty attempt token")
	}
	if _, err := time.Parse(time.RFC3339, r.CreatedUTC); err != nil {
		return fmt.Errorf("invalid created_utc")
	}
	return nil
}

// WriteRebaseReceipt publishes r into dir crash-safely, beside the manifest. It
// validates first, ensures the directory exists at 0700, writes a same-directory
// temp file, chmods it to 0600, fsyncs it, renames it over the receipt, and
// fsyncs the directory so the rename is durable. Any exit before the rename
// removes the temp file, so a refused or failed write leaves no stray sibling and
// no partially written receipt.
func (s *Service) WriteRebaseReceipt(ctx context.Context, dir string, r RebaseReceipt) error {
	if err := validateRebaseReceipt(r); err != nil {
		return &Failure{Op: rebaseReceiptOp, Stage: "validate", Kind: KindInvalidInput, Detail: "refusing to write invalid rebase receipt", Err: err}
	}
	if dir == "" || !filepath.IsAbs(dir) {
		return &Failure{Op: rebaseReceiptOp, Stage: "validate", Kind: KindInvalidInput, Detail: "receipt directory is not an absolute path"}
	}
	if err := ensureDir(dir); err != nil {
		return &Failure{Op: rebaseReceiptOp, Stage: "manifest", Kind: KindExternal, Detail: "preparing receipt directory", Err: err}
	}

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return &Failure{Op: rebaseReceiptOp, Stage: "manifest", Kind: KindInvalidOutput, Detail: "encoding rebase receipt", Err: err}
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, ".rebase-receipt.json.*.tmp")
	if err != nil {
		return &Failure{Op: rebaseReceiptOp, Stage: "manifest", Kind: KindExternal, Detail: "staging rebase receipt", Err: err}
	}
	tmpName := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return &Failure{Op: rebaseReceiptOp, Stage: "manifest", Kind: KindExternal, Detail: "writing rebase receipt", Err: err}
	}
	if err := tmp.Chmod(workspaceFileMode); err != nil {
		_ = tmp.Close()
		return &Failure{Op: rebaseReceiptOp, Stage: "manifest", Kind: KindExternal, Detail: "setting mode on rebase receipt temp", Err: err}
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return &Failure{Op: rebaseReceiptOp, Stage: "manifest", Kind: KindExternal, Detail: "syncing rebase receipt", Err: err}
	}
	if err := tmp.Close(); err != nil {
		return &Failure{Op: rebaseReceiptOp, Stage: "manifest", Kind: KindExternal, Detail: "closing rebase receipt temp", Err: err}
	}
	if err := os.Rename(tmpName, filepath.Join(dir, rebaseReceiptFileName)); err != nil {
		return &Failure{Op: rebaseReceiptOp, Stage: "manifest", Kind: KindExternal, Detail: "publishing rebase receipt", Err: err}
	}
	if err := syncDir(dir); err != nil {
		return &Failure{Op: rebaseReceiptOp, Stage: "manifest", Kind: KindExternal, Detail: "syncing receipt directory", Err: err}
	}
	committed = true

	// Round-trip guard: the bytes just written must read back valid and equal.
	reloaded, present, rerr := s.ReadRebaseReceipt(ctx, dir)
	if rerr != nil {
		return &Failure{Op: rebaseReceiptOp, Stage: "manifest", Kind: KindExternal, Detail: "verifying written rebase receipt", Err: rerr}
	}
	if !present || reloaded != r {
		return &Failure{Op: rebaseReceiptOp, Stage: "manifest", Kind: KindInvalidOutput, Detail: "written rebase receipt did not round-trip equal"}
	}
	return nil
}

// ReadRebaseReceipt reads the receipt in dir with a strict three-outcome
// contract:
//   - (r, true, nil)     present and valid;
//   - (zero, false, nil) cleanly absent — os.IsNotExist on the exact receipt path;
//   - (zero, false, err) anything else: unreadable, truncated, undecodable, or
//     any field violation.
//
// A corrupt or unreadable receipt NEVER reads as absent (learnings:
// probe-error-is-not-clean-absence): only a not-exist error on the receipt path
// is clean absence; a permission error, a decode error, or a validation failure
// is an error.
func (s *Service) ReadRebaseReceipt(ctx context.Context, dir string) (RebaseReceipt, bool, error) {
	_ = ctx
	if dir == "" || !filepath.IsAbs(dir) {
		return RebaseReceipt{}, false, &Failure{Op: rebaseReceiptOp, Stage: "validate", Kind: KindInvalidInput, Detail: "receipt directory is not an absolute path"}
	}
	data, err := os.ReadFile(filepath.Join(dir, rebaseReceiptFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return RebaseReceipt{}, false, nil
		}
		return RebaseReceipt{}, false, &Failure{Op: rebaseReceiptOp, Stage: "manifest", Kind: KindExternal, Detail: "reading rebase receipt", Err: err}
	}
	var r RebaseReceipt
	if err := json.Unmarshal(data, &r); err != nil {
		return RebaseReceipt{}, false, &Failure{Op: rebaseReceiptOp, Stage: "manifest", Kind: KindInvalidOutput, Detail: "decoding rebase receipt", Err: err}
	}
	if err := validateRebaseReceipt(r); err != nil {
		return RebaseReceipt{}, false, &Failure{Op: rebaseReceiptOp, Stage: "manifest", Kind: KindInvalidOutput, Detail: "invalid rebase receipt", Err: err}
	}
	return r, true, nil
}

// ClearRebaseReceipt removes the receipt in dir. It is idempotent: a not-exist
// receipt is already cleared, so removing an absent receipt is a no-op, not an
// error. Any other removal failure is an external error.
func (s *Service) ClearRebaseReceipt(ctx context.Context, dir string) error {
	_ = ctx
	if dir == "" || !filepath.IsAbs(dir) {
		return &Failure{Op: rebaseReceiptOp, Stage: "validate", Kind: KindInvalidInput, Detail: "receipt directory is not an absolute path"}
	}
	if err := os.Remove(filepath.Join(dir, rebaseReceiptFileName)); err != nil && !os.IsNotExist(err) {
		return &Failure{Op: rebaseReceiptOp, Stage: "remove", Kind: KindExternal, Detail: "removing rebase receipt", Err: err}
	}
	return nil
}
