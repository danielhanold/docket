package app

// open.go holds `docket open`'s argument grammar and its result document: the
// six targets it accepts, the reasons a refusal carries, the notes a success
// may attach, and OpenResult, the one document every outcome renders.

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/danielhanold/docket/internal/gitcli"
)

// OperationOpen is the operation id of `docket open`.
const OperationOpen = "open"

// OpenTargets lists what `docket open` can open, in usage order. Every target
// but board takes an optional change id.
var OpenTargets = []string{"board", "change", "spec", "plan", "results", "pr"}

// Target kinds an OpenResult reports: a URL or a local file path.
const (
	OpenTargetKindURL  = "url"
	OpenTargetKindFile = "file"
)

// Reasons an OpenResult carries when it refuses or fails.
const (
	ReasonOpenInvalidTarget      = "invalid-target"
	ReasonOpenUnknownChange      = "unknown-change"
	ReasonOpenAmbiguousChange    = "ambiguous-change"
	ReasonOpenArtifactUnset      = "artifact-unset"
	ReasonOpenTrivialNoSpec      = "trivial-no-spec"
	ReasonOpenBoardDisabled      = "board-disabled"
	ReasonOpenPRNotURL           = "pr-not-url"
	ReasonOpenPathInvalid        = "artifact-path-invalid"
	ReasonOpenHeadDetached       = "head-detached"
	ReasonOpenBranchUnmatched    = "branch-unmatched"
	ReasonOpenBranchAmbiguous    = "branch-ambiguous"
	ReasonOpenCheckoutUnreadable = "checkout-unreadable"
	ReasonOpenFileMissing        = "file-missing"
	ReasonOpenNoOpener           = "no-opener"
	ReasonOpenOpenerFailed       = "opener-failed"
)

// openNoteNotGitHub is the note attached when a GitHub page was wanted but the
// origin is not a GitHub remote, so the local file was opened instead.
const openNoteNotGitHub = "origin is not a GitHub remote — opened the local file instead"

// openNoteStale is the note attached when preparing the metadata checkout
// failed with code: the local file opened anyway, but may be out of date.
func openNoteStale(code string) string {
	return "metadata checkout not synced (" + code + ") — file may be stale"
}

// OpenOptions is one `docket open` request.
type OpenOptions struct {
	RepoDir string
	What    string
	ID      int // 0 = infer from the checked-out branch
	Print   bool
}

// OpenDeps is what Open reads and runs, injected so tests start no real git
// and launch no real application.
type OpenDeps struct {
	Reader   StatusReader
	Checkout func(ctx context.Context, dir string) (gitcli.CheckoutState, error)
	Prepare  func(ctx context.Context, repoDir string) RepositoryPrepareResult
	Opener   Opener
}

// OpenResult is the document `docket open` emits for every outcome. ChangeID is
// omitted for the board; Notes is always a list, possibly empty.
type OpenResult struct {
	Envelope
	What       string   `json:"what"`
	ChangeID   int      `json:"change_id,omitempty"`
	Target     string   `json:"target,omitempty"`
	TargetKind string   `json:"target_kind,omitempty"`
	Launched   bool     `json:"launched"`
	Notes      []string `json:"notes"`
	Reason     string   `json:"reason,omitempty"`
	Message    string   `json:"message,omitempty"`
}

// HumanText renders the result for a terminal: the target alone on success
// (so `--print` composes in command substitution), the resolved target then
// the failure message when a failure follows resolution, or the message alone.
func (r OpenResult) HumanText() string {
	if r.Message == "" {
		return r.Target
	}
	if r.Target != "" {
		return r.Target + "\n" + r.Message
	}
	return r.Message
}

// HumanNotes returns the notes a human renderer writes apart from HumanText.
func (r OpenResult) HumanNotes() []string { return r.Notes }

// openFailure is a refusal or failure on the way to a target, carrying the
// result class, reason, and message the OpenResult reports.
type openFailure struct {
	result  Result
	reason  string
	message string
}

// openIDPattern admits plain decimal ids, leading zeros included; a sign, a
// "#", or a hex prefix is refused.
var openIDPattern = regexp.MustCompile(`^[0-9]+$`)

// ParseOpenArgs parses `open <target> [id]` into the target and the change id,
// 0 when none was given. Every error is a one-line usage message.
func ParseOpenArgs(args []string) (string, int, error) {
	targets := strings.Join(OpenTargets, ", ")
	switch {
	case len(args) == 0:
		return "", 0, fmt.Errorf("missing target — choose one of %s", targets)
	case len(args) > 2:
		return "", 0, fmt.Errorf("too many arguments — usage: open <target> [id], where <target> is one of %s", targets)
	}
	what := args[0]
	if !slices.Contains(OpenTargets, what) {
		return "", 0, fmt.Errorf("unknown target %q — choose one of %s", what, targets)
	}
	if len(args) == 1 {
		return what, 0, nil
	}
	if what == "board" {
		return "", 0, fmt.Errorf("board takes no change id — the targets are %s, and only the last five take an id", targets)
	}
	raw := args[1]
	n, err := strconv.Atoi(raw)
	if !openIDPattern.MatchString(raw) || err != nil || n <= 0 {
		return "", 0, fmt.Errorf("invalid change id %s", raw)
	}
	return what, n, nil
}
