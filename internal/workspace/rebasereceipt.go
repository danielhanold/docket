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
	"strconv"
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
// string so the whole value is comparable, the gate pair included, letting a
// publishing run assert the on-disk receipt is byte-for-byte the one it expects.
// RepoIdentity is the repository's canonical common directory; ChangeID is the
// decimal change id; OrigHead is the pre-rebase local head; OrigRemoteHead is the
// remote feature head the rewrite lease is keyed to; BaseRef/BaseHead are the
// rebase target; Attempt is an opaque token distinguishing one rewrite attempt
// from another.
type RebaseReceipt struct {
	RepoIdentity   string `json:"repo_identity"`
	ChangeID       string `json:"change_id"`
	OrigHead       string `json:"orig_head"`
	OrigRemoteHead string `json:"orig_remote_head"`
	BaseRef        string `json:"base_ref"`
	BaseHead       string `json:"base_head"`
	Attempt        string `json:"attempt"`
	// GateDriveID / GateOwnerGeneration persist the finalize local gate's
	// continuation across WAITING slices, so a bare re-entry of the identical
	// finalize.rebase invocation advances the SAME drive (change 0396). The
	// owner generation is receipt-private by design (ADR-0098: only the exact
	// owner advances a drive); it never appears in any CLI document. The pair
	// rule is both-empty (no live drive) or both-set (a WAITING drive to
	// resume); a half-set pair is malformed on write and on read alike.
	GateDriveID         string `json:"gate_drive_id,omitempty"`
	GateOwnerGeneration string `json:"gate_owner_generation,omitempty"`
	// PublishCheckpoint* persist the COMPLETED-gate publish checkpoint (change
	// 0408): when finalize's local gate reaches PASSED for a real (rewritten)
	// rebase, the tested head, the effective base head, the byte-exact resolved
	// finalize.test_command, the gate policy, the open PR number, and the green
	// evidence block are recorded so a resume after a denied publish can reuse
	// the still-valid evidence instead of re-running the suite. The field rule
	// is all-empty (no checkpoint) or all-set (a completed gate to reuse); a
	// partially-set checkpoint is malformed on write and on read alike, and a
	// checkpoint never coexists with a live gate-continuation pair — a terminal
	// and a live drive are mutually exclusive.
	PublishCheckpointHead     string `json:"publish_checkpoint_head,omitempty"`
	PublishCheckpointBaseHead string `json:"publish_checkpoint_base_head,omitempty"`
	PublishCheckpointCommand  string `json:"publish_checkpoint_command,omitempty"`
	PublishCheckpointGate     string `json:"publish_checkpoint_gate,omitempty"`
	PublishCheckpointPRNumber string `json:"publish_checkpoint_pr_number,omitempty"`
	PublishCheckpointEvidence string `json:"publish_checkpoint_evidence,omitempty"`
	CreatedUTC                string `json:"created_utc"`

	// Resolver-budget group (change 0349). All six empty == legacy receipt (a
	// pre-budget binary wrote it): recognizable, never corrupt, never freshly
	// budgeted. When present: ResolverBudgetVersion is "1";
	// ResolverLimit/ResolverUsed are decimal ints with limit >= 1 and
	// 0 <= used <= limit. ResolverReservationToken/ResolverReservationStopped
	// record the outstanding reservation (both-empty or both-set, with the
	// stopped commit a full object id); ResolverContinuationStarted ("" or "1")
	// may be set only while a reservation is outstanding. Every field stays a
	// scalar string so the whole receipt stays comparable and byte-comparable.
	ResolverBudgetVersion       string `json:"resolver_budget_version,omitempty"`
	ResolverLimit               string `json:"resolver_limit,omitempty"`
	ResolverUsed                string `json:"resolver_used,omitempty"`
	ResolverReservationToken    string `json:"resolver_reservation_token,omitempty"`
	ResolverReservationStopped  string `json:"resolver_reservation_stopped,omitempty"`  // full object id of the stopped commit
	ResolverContinuationStarted string `json:"resolver_continuation_started,omitempty"` // "" | "1"
}

// HasResolverBudget reports whether the receipt carries the resolver-budget group
// (change 0349). A legacy receipt written by a pre-budget binary has the whole
// group absent, so the version field alone distinguishes budgeted from legacy.
func (r RebaseReceipt) HasResolverBudget() bool {
	return r.ResolverBudgetVersion != ""
}

// ResolverBudget decodes the stored limit and used counts. Only call it when
// HasResolverBudget reports true; on a legacy receipt the empty fields fail to
// parse and it returns an error rather than a misleading zero budget.
func (r RebaseReceipt) ResolverBudget() (limit, used int, err error) {
	limit, err = strconv.Atoi(r.ResolverLimit)
	if err != nil {
		return 0, 0, fmt.Errorf("decoding resolver limit: %w", err)
	}
	used, err = strconv.Atoi(r.ResolverUsed)
	if err != nil {
		return 0, 0, fmt.Errorf("decoding resolver used: %w", err)
	}
	return limit, used, nil
}

// validateResolverBudget enforces the whole-group rule: the six fields are either
// all empty (a legacy receipt — valid, never budgeted) or a well-formed budget
// group. It is a conjunct of validateRebaseReceipt, so the same rule gates both
// the write and the read.
func validateResolverBudget(r RebaseReceipt) error {
	if r.ResolverBudgetVersion == "" && r.ResolverLimit == "" && r.ResolverUsed == "" &&
		r.ResolverReservationToken == "" && r.ResolverReservationStopped == "" &&
		r.ResolverContinuationStarted == "" {
		return nil // legacy receipt: whole group absent
	}
	if r.ResolverBudgetVersion != "1" {
		return fmt.Errorf("unsupported resolver budget version %q", r.ResolverBudgetVersion)
	}
	limit, err := strconv.Atoi(r.ResolverLimit)
	if err != nil {
		return fmt.Errorf("resolver limit is not a decimal integer: %q", r.ResolverLimit)
	}
	used, err := strconv.Atoi(r.ResolverUsed)
	if err != nil {
		return fmt.Errorf("resolver used is not a decimal integer: %q", r.ResolverUsed)
	}
	if limit < 1 {
		return fmt.Errorf("resolver limit must be >= 1, got %d", limit)
	}
	if used < 0 || used > limit {
		return fmt.Errorf("resolver used out of range: got %d, want 0..%d", used, limit)
	}
	if (r.ResolverReservationToken == "") != (r.ResolverReservationStopped == "") {
		return fmt.Errorf("half-set resolver reservation: token and stopped commit must both be empty or both be set")
	}
	if r.ResolverReservationStopped != "" && !validObjectID(gitcli.ObjectID(r.ResolverReservationStopped)) {
		return fmt.Errorf("invalid resolver reservation stopped commit")
	}
	switch r.ResolverContinuationStarted {
	case "", "1":
	default:
		return fmt.Errorf("invalid resolver continuation marker %q: must be \"\" or \"1\"", r.ResolverContinuationStarted)
	}
	if r.ResolverContinuationStarted == "1" && r.ResolverReservationToken == "" {
		return fmt.Errorf("resolver continuation started without an outstanding reservation")
	}
	return nil
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
	if (r.GateDriveID == "") != (r.GateOwnerGeneration == "") {
		return fmt.Errorf("half-set gate continuation pair: drive id and owner generation must both be empty or both be set")
	}
	cpFields := []string{
		r.PublishCheckpointHead, r.PublishCheckpointBaseHead, r.PublishCheckpointCommand,
		r.PublishCheckpointGate, r.PublishCheckpointPRNumber, r.PublishCheckpointEvidence,
	}
	set := 0
	for _, f := range cpFields {
		if f != "" {
			set++
		}
	}
	if set != 0 && set != len(cpFields) {
		return fmt.Errorf("partially-set publish checkpoint: all checkpoint fields must be empty or all set")
	}
	if set != 0 {
		if !validObjectID(gitcli.ObjectID(r.PublishCheckpointHead)) {
			return fmt.Errorf("invalid publish checkpoint head")
		}
		if !validObjectID(gitcli.ObjectID(r.PublishCheckpointBaseHead)) {
			return fmt.Errorf("invalid publish checkpoint base head")
		}
		if n, err := strconv.Atoi(r.PublishCheckpointPRNumber); err != nil || n <= 0 {
			return fmt.Errorf("invalid publish checkpoint pr number")
		}
		if r.GateDriveID != "" {
			return fmt.Errorf("publish checkpoint cannot coexist with a live gate continuation")
		}
	}
	if _, err := time.Parse(time.RFC3339, r.CreatedUTC); err != nil {
		return fmt.Errorf("invalid created_utc")
	}
	if err := validateResolverBudget(r); err != nil {
		return err
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
