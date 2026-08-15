package transaction

// This file owns the request-ID idempotency machinery: the engine-authored trailer
// vocabulary, the block builder every commit stamps, and the pre-allocation
// ancestry scan that turns a lost response into a replayed receipt instead of a
// second allocation. The scan reads Git's own trailer grammar (via Task 4's
// ScanCommitTrailers) over the FULL ancestry of the freshly fetched base — no
// depth window — and validates each candidate block's multiplicity, shape, and
// receipt canonicality before trusting it. It never chooses a winner by commit
// order: duplicate, malformed, or contradictory history is invalid-state, not a
// silent pick.

import (
	"context"
	"encoding/base64"

	"github.com/danielhanold/docket/internal/gitcli"
)

// The engine-authored trailer keys. Docket-Transaction-ID, Docket-Operation, and
// Docket-Result are present on every transaction commit; the two request trailers
// are present together only on a keyed commit, and absent together otherwise.
const (
	trailerTransactionID = "Docket-Transaction-ID"
	trailerOperation     = "Docket-Operation"
	trailerRequestID     = "Docket-Request-ID"
	trailerRequestDigest = "Docket-Request-Digest"
	trailerResult        = "Docket-Result" // unpadded base64url of the canonical JSON receipt
)

// engineTrailers builds the exactly-one engine-authored block for a commit: the
// three always-present trailers (transaction id, operation, base64url receipt),
// plus both request trailers, in the spec's order, when key != nil. Docket-Result
// is always last so the request pair — when present — sits between the operation
// and the receipt, matching the spec's block layout.
func engineTrailers(txnID string, op OperationKey, key *IdempotencyKey, receipt []byte) []gitcli.Trailer {
	trailers := make([]gitcli.Trailer, 0, 5)
	trailers = append(trailers,
		gitcli.Trailer{Key: trailerTransactionID, Value: txnID},
		gitcli.Trailer{Key: trailerOperation, Value: string(op)},
	)
	if key != nil {
		trailers = append(trailers,
			gitcli.Trailer{Key: trailerRequestID, Value: key.RequestID},
			gitcli.Trailer{Key: trailerRequestDigest, Value: string(key.Digest)},
		)
	}
	trailers = append(trailers,
		gitcli.Trailer{Key: trailerResult, Value: base64.RawURLEncoding.EncodeToString(receipt)},
	)
	return trailers
}

// replayKind classifies the ancestry scan's verdict.
type replayKind int

const (
	replayNone         replayKind = iota // no commit carries this request id — proceed
	replayFound                          // exactly one match, same digest — already applied
	replayIDReused                       // the id exists with a different digest — invalid-input
	replayInvalidState                   // duplicate, malformed, or contradictory history
)

// replayOutcome is the scan's result. commit/op/receipt are populated only on
// replayFound; receipt is already base64url-decoded and receipt-validated.
type replayOutcome struct {
	kind    replayKind
	commit  gitcli.ObjectID
	op      OperationKey
	receipt []byte
}

// engineBlock is one commit's validated engine trailer block: its operation key,
// its request digest, and its decoded, validated receipt.
type engineBlock struct {
	op      OperationKey
	digest  string
	receipt []byte
}

// scanForRequest searches the full ancestry reachable from `from` for the exact
// request id in key, using Git's own trailer grammar so a trailer-looking body
// line never matches and no history-depth window can expire idempotency. Every
// commit bearing our request id must present a well-formed engine block
// (multiplicity, shape, decoded receipt); a malformed one is invalid-state. The
// verdict is by count and digest, never by commit order:
//   - zero matches → replayNone;
//   - exactly one match with the same digest → replayFound;
//   - exactly one match with a different digest → replayIDReused;
//   - two or more matches → replayInvalidState.
func (e *Engine) scanForRequest(ctx context.Context, repo gitcli.Repository,
	from gitcli.ObjectID, key *IdempotencyKey) (replayOutcome, error) {

	scanned, err := e.client.ScanCommitTrailers(ctx, repo, from, []string{trailerRequestID})
	if err != nil {
		return replayOutcome{}, err
	}

	type match struct {
		commit gitcli.ObjectID
		block  engineBlock
	}
	var matches []match
	for _, ct := range scanned {
		if !commitHasRequestID(ct.Trailers, key.RequestID) {
			continue // a different request's commit — not ours
		}
		block, ok := parseEngineBlock(ct.Trailers)
		if !ok {
			// A commit bearing our request id but with a malformed engine block is a
			// contradiction the scan must never trust.
			return replayOutcome{kind: replayInvalidState}, nil
		}
		matches = append(matches, match{commit: ct.Commit, block: block})
	}

	switch len(matches) {
	case 0:
		return replayOutcome{kind: replayNone}, nil
	case 1:
		m := matches[0]
		if m.block.digest != string(key.Digest) {
			return replayOutcome{kind: replayIDReused}, nil
		}
		return replayOutcome{
			kind:    replayFound,
			commit:  m.commit,
			op:      m.block.op,
			receipt: m.block.receipt,
		}, nil
	default:
		return replayOutcome{kind: replayInvalidState}, nil
	}
}

// commitHasRequestID reports whether any Docket-Request-ID trailer on the commit
// carries the exact request id. Multiplicity and shape are the block validator's
// job; this only decides whether the commit is a candidate for our request.
func commitHasRequestID(trailers []gitcli.Trailer, requestID string) bool {
	for _, tr := range trailers {
		if tr.Key == trailerRequestID && tr.Value == requestID {
			return true
		}
	}
	return false
}

// parseEngineBlock validates a commit's engine trailer block and, on success,
// returns its operation key, request digest, and decoded receipt. It requires each
// of the five engine keys exactly once (the request pair present together, since
// this is only called for a commit bearing a request id), a well-formed operation
// key and request id/digest, and a Docket-Result that base64url-decodes to a valid
// canonical receipt. Any violation returns ok=false — a malformed block the scan
// must treat as invalid-state.
func parseEngineBlock(trailers []gitcli.Trailer) (engineBlock, bool) {
	var txnID, op, reqID, digest, result []string
	for _, tr := range trailers {
		switch tr.Key {
		case trailerTransactionID:
			txnID = append(txnID, tr.Value)
		case trailerOperation:
			op = append(op, tr.Value)
		case trailerRequestID:
			reqID = append(reqID, tr.Value)
		case trailerRequestDigest:
			digest = append(digest, tr.Value)
		case trailerResult:
			result = append(result, tr.Value)
		}
	}
	// Multiplicity: every engine key exactly once. The request pair must both be
	// present exactly once, which also enforces "both present or both absent" for a
	// commit that reached this validator via a request-id trailer.
	if len(txnID) != 1 || len(op) != 1 || len(reqID) != 1 || len(digest) != 1 || len(result) != 1 {
		return engineBlock{}, false
	}
	if txnID[0] == "" {
		return engineBlock{}, false
	}
	// Shape: operation key and the request id/digest grammar.
	if err := validateOperationKey(OperationKey(op[0])); err != nil {
		return engineBlock{}, false
	}
	if err := validateIdempotencyKey(&IdempotencyKey{RequestID: reqID[0], Digest: RequestDigest(digest[0])}); err != nil {
		return engineBlock{}, false
	}
	// Receipt: base64url decode then full receipt validation (canonical, bounded).
	decoded, err := base64.RawURLEncoding.DecodeString(result[0])
	if err != nil {
		return engineBlock{}, false
	}
	if err := validateReceipt(decoded); err != nil {
		return engineBlock{}, false
	}
	return engineBlock{op: OperationKey(op[0]), digest: digest[0], receipt: decoded}, true
}
