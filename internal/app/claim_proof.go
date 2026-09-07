package app

import (
	"context"
	"encoding/base64"
	"encoding/json"

	"github.com/danielhanold/docket/internal/gitcli"
)

// This file is the verdict path's read-only seam onto committed change.claim
// proof (change 0407). A keyed gate-verdict must resolve ownership from the
// durable, authoritative proof a successful claim leaves behind — the committed
// claim receipt on the metadata branch — rather than inferring it from a
// before-set/epoch snapshot. The scanner reads those receipts back through the
// same engine trailer grammar the idempotency scan uses; a later task consumes
// its output to bind a dispatch to exactly its own claim.

// The change.claim trailer keys this scanner reads back. They mirror the
// engine-authored spellings in engineTrailers (internal/repository/transaction/
// idempotency.go) — the engine's spellings are authoritative, so a drift there
// reddens the real-git scanner test that commits these literals by hand.
const (
	claimProofOperationTrailer = "Docket-Operation"
	claimProofRequestIDTrailer = "Docket-Request-ID"
	claimProofResultTrailer    = "Docket-Result"
)

// ClaimProof is one committed change.claim applied receipt read back from the
// metadata branch's engine trailer blocks — the durable, authoritative proof of
// a successful claim (spec: "The receipt is authoritative proof of a successful
// claim"). Proofs are returned newest-first (git log order), so the FIRST proof
// naming a change id is that id's newest claim.
type ClaimProof struct {
	RequestID       string // Docket-Request-ID trailer
	ChangeID        int    // receipt id
	GateContextHash string // receipt gate_context_hash ("" = ungated claim)
	Revision        string // commit hash carrying the receipt
}

// ClaimProofScanner is the verdict path's read-only seam onto committed claim
// proofs; tests inject fakes, production scans real commit trailers.
type ClaimProofScanner interface {
	ScanClaimProofs(ctx context.Context, repoDir string) ([]ClaimProof, error)
}

// gitClaimProofScanner is the production ClaimProofScanner: it re-syncs the
// authoritative metadata revision (the same fresh-origin pin every gate read
// uses) and scans that history's engine trailer blocks for change.claim
// receipts.
type gitClaimProofScanner struct {
	deps PlanningDeps
}

// NewClaimProofScanner builds the production scanner over the planning seams.
func NewClaimProofScanner(deps PlanningDeps) ClaimProofScanner {
	return gitClaimProofScanner{deps: deps}
}

// claimProofReceipt decodes the fields of a committed change.claim receipt this
// scanner reports. It reads gate_context_hash, which the canonical
// changeClaimReceipt itself grows in a later seam (change 0407); a receipt
// predating that field simply decodes gate_context_hash to "" (an ungated
// claim), so the decode is forward- and backward-compatible.
type claimProofReceipt struct {
	changeClaimReceipt
	GateContextHash string `json:"gate_context_hash"`
}

// ScanClaimProofs re-pins the authoritative metadata revision, then walks that
// history's engine trailer blocks and returns one ClaimProof per committed
// change.claim receipt, newest-first (git log order). A commit whose block is
// not a change.claim receipt, carries no Docket-Result, presents no request id,
// or whose receipt does not decode is skipped — such a commit is another
// operation's receipt or corrupt history, never a reason to fail the whole scan.
func (s gitClaimProofScanner) ScanClaimProofs(ctx context.Context, repoDir string) ([]ClaimProof, error) {
	pin, err := s.deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		return nil, err
	}
	repo, err := s.deps.Client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})
	if err != nil {
		return nil, err
	}
	scanned, err := s.deps.Client.ScanCommitTrailers(ctx, repo,
		gitcli.ObjectID(pin.MetadataRevision), []string{claimProofRequestIDTrailer})
	if err != nil {
		return nil, err
	}

	var proofs []ClaimProof
	for _, ct := range scanned {
		var op, requestID, result string
		var haveResult bool
		for _, tr := range ct.Trailers {
			switch tr.Key {
			case claimProofOperationTrailer:
				op = tr.Value
			case claimProofRequestIDTrailer:
				requestID = tr.Value
			case claimProofResultTrailer:
				result = tr.Value
				haveResult = true
			}
		}
		if op != OperationChangeClaim || !haveResult || requestID == "" {
			continue
		}
		decoded, derr := base64.RawURLEncoding.DecodeString(result)
		if derr != nil {
			continue
		}
		var rec claimProofReceipt
		if uerr := json.Unmarshal(decoded, &rec); uerr != nil {
			continue
		}
		proofs = append(proofs, ClaimProof{
			RequestID:       requestID,
			ChangeID:        rec.ID,
			GateContextHash: rec.GateContextHash,
			Revision:        string(ct.Commit),
		})
	}
	return proofs, nil
}
