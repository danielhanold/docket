package githubcli

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"strconv"
)

// State is the closed set of pull-request lifecycle states, normalized to lower
// case from GitHub's uppercase enum.
type State string

const (
	StateOpen   State = "open"
	StateClosed State = "closed"
	StateMerged State = "merged"
)

// PullRequest is the typed snapshot the adapter returns. Version is a local
// optimistic-concurrency token over the mutable snapshot (see computeVersion);
// it contains no body bytes and is recomputed only from the latest
// authoritative response.
type PullRequest struct {
	Number     int
	URL        string
	State      State
	Draft      bool
	HeadBranch string
	HeadCommit string // full GitHub-reported object id, validated 40/64 lowercase hex
	BaseBranch string
	Title      string
	Body       string
	Approved   bool   // reviewDecision == APPROVED on an enriched exact view; always false from standard-field reads
	Version    string // "sha256:" + 64 hex over the exact mutable snapshot
}

// prViewJSON is the documented nested shape `gh pr view/create --json ...`
// emits. isDraft and the *Ref* fields are gh's field names verbatim; decoding
// keys on these, never on display text, a jq fragment, or stderr. Pointer/typed
// fields let the decoder tell "absent" from a legitimate zero (e.g. a genuinely
// empty body vs a missing headRefOid).
type prViewJSON struct {
	Number      int     `json:"number"`
	URL         string  `json:"url"`
	State       *string `json:"state"`
	IsDraft     bool    `json:"isDraft"`
	HeadRefName string  `json:"headRefName"`
	HeadRefOid  string  `json:"headRefOid"`
	BaseRefName string  `json:"baseRefName"`
	Title       string  `json:"title"`
	Body        string  `json:"body"`
	// ReviewDecision is requested only by the exact-number view
	// (prViewJSONFields). Absent (standard field set) and JSON null (no
	// decision yet) are both nil and both map to unapproved.
	ReviewDecision *string `json:"reviewDecision"`
}

// decodePullRequest decodes ONE PR object from gh's JSON and validates every
// required field. Missing required fields or a malformed full object id are
// invalid-output; an unrecognized state enum is invalid-state. It never returns
// a partially-populated zero value alongside an error.
func decodePullRequest(op string, data []byte) (PullRequest, error) {
	var raw prViewJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return PullRequest{}, newFailure(op, StageDecode, KindInvalidOutput, "pull-request output is not valid JSON", err)
	}
	return raw.toPullRequest(op)
}

// decodeSolePullRequest decodes a JSON ARRAY of PRs and requires exactly one.
// Zero elements are invalid-output (nothing to decode); more than one is
// invalid-state (ambiguous — a single-PR view resolved to several candidates).
func decodeSolePullRequest(op string, data []byte) (PullRequest, error) {
	var rawList []prViewJSON
	if err := json.Unmarshal(data, &rawList); err != nil {
		return PullRequest{}, newFailure(op, StageDecode, KindInvalidOutput, "pull-request list output is not valid JSON", err)
	}
	switch len(rawList) {
	case 0:
		return PullRequest{}, newFailure(op, StageDecode, KindInvalidOutput, "expected one pull request, found none", nil)
	case 1:
		return rawList[0].toPullRequest(op)
	default:
		return PullRequest{}, newFailure(op, StageDecode, KindInvalidState, "expected one pull request, found several (ambiguous)", nil)
	}
}

// toPullRequest validates the decoded fields and computes the version. Every
// required field is checked; the state enum is mapped from GitHub's uppercase
// form; the head object id is validated as full lowercase hex.
func (raw prViewJSON) toPullRequest(op string) (PullRequest, error) {
	if raw.Number <= 0 {
		return PullRequest{}, newFailure(op, StageDecode, KindInvalidOutput, "pull request has no positive number", nil)
	}
	if raw.URL == "" {
		return PullRequest{}, newFailure(op, StageDecode, KindInvalidOutput, "pull request has no url", nil)
	}
	if raw.HeadRefName == "" {
		return PullRequest{}, newFailure(op, StageDecode, KindInvalidOutput, "pull request has no head branch", nil)
	}
	if raw.BaseRefName == "" {
		return PullRequest{}, newFailure(op, StageDecode, KindInvalidOutput, "pull request has no base branch", nil)
	}
	if err := validateFullObjectID(raw.HeadRefOid); err != nil {
		return PullRequest{}, newFailure(op, StageDecode, KindInvalidOutput, "pull request head oid invalid: "+err.Error(), err)
	}
	state, err := normalizeState(raw.State)
	if err != nil {
		return PullRequest{}, newFailure(op, StageDecode, KindInvalidState, err.Error(), err)
	}
	approved, err := normalizeReviewDecision(raw.ReviewDecision)
	if err != nil {
		return PullRequest{}, newFailure(op, StageDecode, KindInvalidState, err.Error(), err)
	}
	pr := PullRequest{
		Number:     raw.Number,
		URL:        raw.URL,
		State:      state,
		Draft:      raw.IsDraft,
		HeadBranch: raw.HeadRefName,
		HeadCommit: raw.HeadRefOid,
		BaseBranch: raw.BaseRefName,
		Title:      raw.Title,
		Body:       raw.Body,
		Approved:   approved,
	}
	pr.Version = computeVersion(pr)
	return pr, nil
}

// normalizeState maps GitHub's uppercase state enum to the typed lowercase
// State. A missing or unrecognized value is rejected rather than defaulted.
func normalizeState(raw *string) (State, error) {
	if raw == nil {
		return "", errEnum("pull request has no state")
	}
	switch *raw {
	case "OPEN":
		return StateOpen, nil
	case "CLOSED":
		return StateClosed, nil
	case "MERGED":
		return StateMerged, nil
	default:
		return "", errEnum("unrecognized pull-request state enum")
	}
}

// normalizeReviewDecision maps GitHub's nullable reviewDecision enum to the
// Approved boolean. Only APPROVED is an affirmative decision; REVIEW_REQUIRED,
// CHANGES_REQUESTED, and null/absent are false — null never becomes true merely
// because a repository has no required-review rule. GitHub also reports "no
// decision" as an empty STRING (not JSON null) when branch protection requires
// a PR but zero approvals, so empty string is equally no-decision, never an
// invalid enum. Unknown non-empty vocabulary is invalid external state and is
// rejected, never folded into either outcome.
func normalizeReviewDecision(raw *string) (bool, error) {
	if raw == nil || *raw == "" {
		return false, nil
	}
	switch *raw {
	case "APPROVED":
		return true, nil
	case "REVIEW_REQUIRED", "CHANGES_REQUESTED":
		return false, nil
	default:
		return false, errEnum("unrecognized pull-request reviewDecision enum")
	}
}

// validateFullObjectID requires a non-empty, all-lowercase-hex id of length 40
// or 64 (SHA-1 and SHA-256 full representation; never truncated). githubcli
// keeps this local rather than importing gitcli — head commits arrive as plain
// validated strings across the package boundary.
func validateFullObjectID(id string) error {
	if id == "" {
		return errEnum("empty object id")
	}
	if len(id) != 40 && len(id) != 64 {
		return errEnum("object id must be 40 or 64 hex chars")
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return errEnum("object id has non-lowercase-hex char")
		}
	}
	return nil
}

// computeVersion is the local optimistic-concurrency token: sha256 over a
// length-prefixed canonical concatenation of the mutable snapshot fields
// (Number, State, draft flag, HeadBranch, HeadCommit, BaseBranch, Title, Body).
// URL is server-assigned and excluded. Each field is preceded by its 8-byte big
// endian length so no rearrangement of field boundaries — e.g. Title="ab",
// Body="c" versus Title="a", Body="bc" — can collide. It contains no body bytes,
// only their digest.
//
// Approved is deliberately excluded too: review state is view-only, read-only
// gate evidence — the standard list/create/edit snapshots never request it, and
// including it would give the same PR incompatible tokens depending on which
// read shape produced the snapshot. Finalize reloads review state directly
// before effects rather than authorizing a review mutation through this token.
func computeVersion(pr PullRequest) string {
	fields := []string{
		strconv.Itoa(pr.Number),
		string(pr.State),
		boolFlag(pr.Draft),
		pr.HeadBranch,
		pr.HeadCommit,
		pr.BaseBranch,
		pr.Title,
		pr.Body,
	}
	h := sha256.New()
	var lenbuf [8]byte
	for _, f := range fields {
		binary.BigEndian.PutUint64(lenbuf[:], uint64(len(f)))
		h.Write(lenbuf[:])
		h.Write([]byte(f))
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// boolFlag renders a bool as the single-byte "t"/"f" token the version snapshot
// uses.
func boolFlag(b bool) string {
	if b {
		return "t"
	}
	return "f"
}

// enumError is a small typed error carrying a stable, redaction-safe message for
// decode-time validation failures.
type enumError struct{ msg string }

func (e enumError) Error() string { return e.msg }

func errEnum(msg string) error { return enumError{msg: msg} }
